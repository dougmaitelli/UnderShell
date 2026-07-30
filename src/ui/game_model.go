package ui

import (
	"context"
	"errors"
	"log/slog"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"sshrpg/src/admin"
	"sshrpg/src/domain"
	"sshrpg/src/repository"
	"sshrpg/src/world"
)

type phase uint8

const (
	phaseOnboarding phase = iota
	phaseJoining
	phasePlaying
)

type gameModel struct {
	repositories Repositories
	world        *world.Manager
	admin        *admin.Handler
	log          *slog.Logger
	identity     Identity

	phase     phase
	input     textinput.Model
	message   string
	creating  bool
	character *domain.Character
	inventory *domain.Inventory

	connection    worldConnection
	movement      movementState
	actions       actionState
	inventoryMenu inventoryState
	skills        skillsState
	shop          shopState
	quests        questState
	chat          chatPanelState
	eventFeed     eventFeed
	nameShimmer   playerNameShimmerState
	mode          inputMode
	width         int
	height        int
	renderer      Renderer
	lastView      string
	renderDirty   bool
}

type characterCreatedMsg struct {
	character *domain.Character
	inventory *domain.Inventory
	err       error
}

type worldJoinedMsg struct {
	session world.Session
}

type worldSnapshotMsg struct {
	snapshot world.Snapshot
	ok       bool
}
type worldEventMsg struct {
	event world.Event
	ok    bool
}
type chatMessageMsg struct {
	message world.ChatMessage
	ok      bool
}
type worldKickedMsg struct {
	reason string
	ok     bool
}

type playerMovedMsg struct {
	player world.Player
	moved  bool
}

type positionSavedMsg struct {
	err error
}
type progressSavedMsg struct {
	err error
}

type movementTickMsg struct{}
type walkAnimationDoneMsg struct{ generation uint64 }
type attackAnimationMsg struct{ frame int }
type attackResultMsg struct{ result world.AttackResult }
type pickupResultMsg struct{ result world.PickupResult }
type itemStoredMsg struct {
	inventory *domain.Inventory
	itemName  string
	err       error
}
type skillSpentMsg struct{ player world.Player }
type eventExpiredMsg struct{ id uint64 }
type chatSentMsg struct{ ok bool }
type adminCommandMsg struct {
	message string
	err     error
}
type inventoryReloadedMsg struct {
	inventory *domain.Inventory
	err       error
}

func newGameModel(
	repositories Repositories,
	worldManager *world.Manager,
	log *slog.Logger,
	identity Identity,
	char *domain.Character,
	inventory *domain.Inventory,
) *gameModel {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "Your name"
	input.CharLimit = 20
	input.SetWidth(20)
	input.Validate = func(value string) error {
		_, err := domain.ValidateCharacterName(value)
		return err
	}
	currentPhase := phaseOnboarding
	if char != nil {
		currentPhase = phaseJoining
	}
	questState := newQuestState(nil)
	if worldManager != nil {
		questState = newQuestState(worldManager.Quests())
	}
	return &gameModel{
		repositories: repositories,
		world:        worldManager, log: log, identity: identity,
		phase: currentPhase, input: input,
		character: char, inventory: inventory,
		width: 80, height: 24,
		movement:    newMovementState(),
		chat:        newChatPanelState(),
		quests:      questState,
		renderer:    NewRenderer(),
		renderDirty: true,
	}
}

func (m *gameModel) Init() tea.Cmd {
	if m.character != nil {
		return m.joinWorld()
	}
	return m.input.Focus()
}

func (m *gameModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	// Prove clean, not dirty: unknown messages redraw by default. Individual
	// cases may reuse the cached frame only when they know no visible state
	// changed.
	m.renderDirty = true
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		if m.width == msg.Width && m.height == msg.Height {
			m.reuseLastView()
			return m, nil
		}
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyboardEnhancementsMsg:
		m.movement.enhanced = msg.SupportsEventTypes()
		m.reuseLastView()
		return m, nil
	case tea.KeyPressMsg:
		return m.updateKeyPress(msg)
	case tea.KeyReleaseMsg:
		if m.phase == phasePlaying && m.movement.enhanced {
			delete(m.movement.held, directionKey(msg.String()))
		}
		m.reuseLastView()
		return m, nil
	case movementTickMsg:
		if m.mode != inputModeGame {
			m.movement.looping = false
			m.reuseLastView()
			return m, nil
		}
		return m, m.handleMovementTick()
	case walkAnimationDoneMsg:
		if !m.movement.finishStep(msg.generation) {
			m.reuseLastView()
		}
		return m, nil
	case attackAnimationMsg:
		if !m.actions.advanceAttack(msg.frame) {
			return m, nil
		}
		return m, attackAnimationTick(msg.frame + 1)
	case playerNameShimmerMsg:
		if !m.nameShimmer.active ||
			msg.generation != m.nameShimmer.generation {
			m.reuseLastView()
			return m, nil
		}
		return m, m.nameShimmer.advance(msg.generation)
	case attackResultMsg:
		m.reuseLastView()
		return m, nil
	case pickupResultMsg:
		if !msg.result.Found {
			m.actions.finishPickup()
			m.reuseLastView()
			return m, nil
		}
		return m, m.storePickup(msg.result.Item)
	case itemStoredMsg:
		m.actions.finishPickup()
		if msg.err != nil {
			m.log.Error("store picked up item", "character_id", m.character.ID, "error", msg.err)
			m.message = "Could not add that item to your inventory."
			return m, nil
		}
		m.inventory = msg.inventory
		m.message = "Picked up " + msg.itemName + "."
		return m, m.addEvent(EventView{
			Kind: world.EventPickup, Message: "Picked up " + msg.itemName,
		})
	case inventoryEquipmentMsg:
		return m.updateInventoryEquipment(msg)
	case inventoryConsumableMsg:
		return m.updateInventoryConsumable(msg)
	case skillSpentMsg:
		m.skills.finishSpend()
		if msg.player.ID == 0 {
			m.reuseLastView()
			return m, nil
		}
		m.character.SkillPoints = msg.player.SkillPoints
		m.character.Attack = msg.player.Attack
		m.character.Defense = msg.player.Defense
		m.character.Vitality = msg.player.Vitality
		return m, m.saveProgress()
	case characterCreatedMsg:
		m.creating = false
		if msg.err != nil {
			if errors.Is(msg.err, repository.ErrCharacterKeyExists) {
				if existing, err := m.repositories.Characters.FindByFingerprint(
					context.Background(), m.identity.Fingerprint,
				); err == nil && existing != nil {
					if existing.Banned {
						m.message = bannedAccountMessage
						return m, tea.Quit
					}
					m.character = existing
					inventory, inventoryErr := m.repositories.Inventories.FindOrCreate(
						context.Background(), existing.ID,
					)
					if inventoryErr != nil {
						m.message = inventoryErr.Error()
						return m, nil
					}
					m.inventory = inventory
					m.phase = phaseJoining
					return m, m.joinWorld()
				}
			}
			m.message = msg.err.Error()
			return m, nil
		}
		m.character = msg.character
		m.inventory = msg.inventory
		m.phase = phaseJoining
		m.input.Blur()
		return m, m.joinWorld()
	case worldJoinedMsg:
		return m.updateWorldJoined(msg)
	case worldEventMsg:
		return m.updateWorldEvent(msg)
	case chatMessageMsg:
		return m.updateChatMessage(msg)
	case chatSentMsg:
		m.reuseLastView()
		return m, nil
	case adminCommandMsg:
		if msg.err != nil {
			m.message = msg.err.Error()
			return m, nil
		}
		m.message = msg.message
		return m, nil
	case inventoryReloadedMsg:
		if msg.err != nil {
			m.log.Error(
				"reload inventory after admin command",
				"character_id", m.character.ID,
				"error", msg.err,
			)
			m.message = "Could not refresh your inventory."
			return m, nil
		}
		m.inventory = msg.inventory
		return m, nil
	case shopTradeMsg:
		return m.updateShopTrade(msg)
	case questInteractionMsg:
		return m.updateQuestInteraction(msg)
	case eventExpiredMsg:
		if !m.eventFeed.expire(msg.id) {
			m.reuseLastView()
		}
		return m, nil
	case worldSnapshotMsg:
		return m.updateWorldSnapshot(msg)
	case worldKickedMsg:
		if !msg.ok {
			m.reuseLastView()
			return m, nil
		}
		m.message = msg.reason
		return m, tea.Quit
	case playerMovedMsg:
		m.movement.inFlight = false
		if msg.player.ID == 0 {
			m.reuseLastView()
			return m, nil
		}
		if !msg.moved {
			m.reuseLastView()
			return m, nil
		}
		m.character.AreaID = msg.player.AreaID
		m.character.X, m.character.Y = msg.player.X, msg.player.Y
		return m, tea.Batch(m.savePosition(), m.movement.step())
	case positionSavedMsg:
		m.reuseLastView()
		if msg.err != nil {
			m.log.Error("save position", "character_id", m.character.ID, "error", msg.err)
		}
	case progressSavedMsg:
		m.reuseLastView()
		if msg.err != nil {
			m.log.Error("save progress", "character_id", m.character.ID, "error", msg.err)
		}
	}
	return m, nil
}

func (m *gameModel) View() tea.View {
	content := m.lastView
	if content == "" || m.renderDirty {
		content = m.renderer.Render(m.viewState())
		m.lastView = content
	}
	m.renderDirty = false
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "UnderShell"
	view.KeyboardEnhancements.ReportEventTypes = true
	return view
}

func (m *gameModel) reuseLastView() {
	if m.lastView != "" {
		m.renderDirty = false
	}
}

func (m *gameModel) viewState() ViewState {
	state := ViewState{
		Phase:             m.phase,
		Width:             m.width,
		Height:            m.height,
		Input:             m.input.View(),
		Message:           m.message,
		Creating:          m.creating,
		Character:         m.character,
		Snapshot:          m.connection.snapshot,
		InventoryOpen:     m.mode == inputModeInventory,
		Inventory:         m.inventory,
		SkillsOpen:        m.mode == inputModeSkills,
		Events:            m.eventFeed.views(),
		ChatMessages:      append([]world.ChatMessage(nil), m.chat.messages...),
		ChatFocused:       m.mode == inputModeChat,
		ChatInput:         m.chat.input.View(),
		HelpOpen:          m.mode == inputModeHelp,
		ShopOpen:          m.mode == inputModeShop,
		JournalOpen:       m.mode == inputModeJournal,
		QuestDialogueOpen: m.mode == inputModeQuestDialogue,
		AttackFrame:       m.actions.attackFrame,
		AttackDirection:   m.actions.attackDirection,
		WalkFrame:         m.movement.walkFrame,
		FacingX:           m.movement.facingX,
		FacingY:           m.movement.facingY,
		PlayerNameShimmer: m.nameShimmer.frame,
	}
	if state.InventoryOpen {
		state.InventoryView = m.inventoryView()
	}
	if state.ShopOpen {
		state.Shop = m.shopView()
	}
	if state.JournalOpen {
		state.Journal = m.quests.journalView(m.inventory, m.questGiver)
	}
	if state.QuestDialogueOpen {
		state.QuestDialogue = m.quests.dialogueView()
	}
	return state
}
