package ui

import (
	"context"
	"errors"
	"log/slog"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

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
	log          *slog.Logger
	identity     Identity

	phase     phase
	input     textinput.Model
	message   string
	creating  bool
	character *domain.Character
	inventory *domain.Inventory

	connection worldConnection
	movement   movementState
	actions    actionState
	skills     skillsState
	chat       chatPanelState
	eventFeed  eventFeed
	mode       inputMode
	width      int
	height     int
	renderer   Renderer
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
type worldKickedMsg struct{}

type playerMovedMsg struct {
	player world.Player
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
	return &gameModel{
		repositories: repositories,
		world:        worldManager, log: log, identity: identity,
		phase: currentPhase, input: input,
		character: char, inventory: inventory,
		width: 80, height: 24,
		movement: newMovementState(),
		chat:     newChatPanelState(),
		renderer: NewRenderer(),
	}
}

func (m *gameModel) Init() tea.Cmd {
	if m.character != nil {
		return m.joinWorld()
	}
	return m.input.Focus()
}

func (m *gameModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyboardEnhancementsMsg:
		m.movement.enhanced = msg.SupportsEventTypes()
		return m, nil
	case tea.KeyPressMsg:
		return m.updateKeyPress(msg)
	case tea.KeyReleaseMsg:
		if m.phase == phasePlaying && m.movement.enhanced {
			delete(m.movement.held, directionKey(msg.String()))
		}
		return m, nil
	case movementTickMsg:
		if m.mode != inputModeGame {
			m.movement.looping = false
			return m, nil
		}
		return m, m.handleMovementTick()
	case walkAnimationDoneMsg:
		m.movement.finishStep(msg.generation)
		return m, nil
	case attackAnimationMsg:
		if !m.actions.advanceAttack(msg.frame) {
			return m, nil
		}
		return m, attackAnimationTick(msg.frame + 1)
	case attackResultMsg:
		return m, nil
	case pickupResultMsg:
		if !msg.result.Found {
			m.actions.finishPickup()
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
	case skillSpentMsg:
		m.skills.finishSpend()
		if msg.player.ID == 0 {
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
		return m, nil
	case eventExpiredMsg:
		m.eventFeed.expire(msg.id)
		return m, nil
	case worldSnapshotMsg:
		return m.updateWorldSnapshot(msg)
	case worldKickedMsg:
		m.message = "This character connected from another session."
		return m, tea.Quit
	case playerMovedMsg:
		m.movement.inFlight = false
		if msg.player.ID == 0 {
			return m, nil
		}
		m.character.AreaID = msg.player.AreaID
		m.character.X, m.character.Y = msg.player.X, msg.player.Y
		return m, m.savePosition()
	case positionSavedMsg:
		if msg.err != nil {
			m.log.Error("save position", "character_id", m.character.ID, "error", msg.err)
		}
	case progressSavedMsg:
		if msg.err != nil {
			m.log.Error("save progress", "character_id", m.character.ID, "error", msg.err)
		}
	}
	return m, nil
}

func (m *gameModel) View() tea.View {
	view := tea.NewView(m.renderer.Render(m.viewState()))
	view.AltScreen = true
	view.WindowTitle = "SSH Realms"
	view.KeyboardEnhancements.ReportEventTypes = true
	return view
}

func (m *gameModel) viewState() ViewState {
	return ViewState{
		Phase:         m.phase,
		Width:         m.width,
		Height:        m.height,
		Input:         m.input.View(),
		Message:       m.message,
		Creating:      m.creating,
		Character:     m.character,
		Snapshot:      m.connection.snapshot,
		InventoryOpen: m.mode == inputModeInventory,
		Inventory:     m.inventory,
		SkillsOpen:    m.mode == inputModeSkills,
		Events:        m.eventFeed.views(),
		ChatMessages:  append([]world.ChatMessage(nil), m.chat.messages...),
		ChatFocused:   m.mode == inputModeChat,
		ChatInput:     m.chat.input.View(),
		HelpOpen:      m.mode == inputModeHelp,
		AttackFrame:   m.actions.attackFrame,
		WalkFrame:     m.movement.walkFrame,
		FacingX:       m.movement.facingX,
		FacingY:       m.movement.facingY,
	}
}
