package ui

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/ssh"

	"sshrpg/src/domain"
	"sshrpg/src/repository"
	"sshrpg/src/world"
)

type Identity struct {
	Fingerprint string
	KeyType     string
	PublicKey   string
}

type Runner struct {
	characters  repository.CharacterRepository
	inventories repository.InventoryRepository
	world       *world.Manager
	log         *slog.Logger
}

func New(
	characters repository.CharacterRepository,
	inventories repository.InventoryRepository,
	worldManager *world.Manager,
	log *slog.Logger,
) *Runner {
	return &Runner{
		characters: characters, inventories: inventories,
		world: worldManager, log: log,
	}
}

func (r *Runner) Run(session ssh.Session, identity Identity) {
	pty, resize, ok := session.Pty()
	if !ok {
		_, _ = io.WriteString(session, "An interactive terminal is required. Try: ssh -t <host>\n")
		return
	}

	char, err := r.characters.FindByFingerprint(session.Context(), identity.Fingerprint)
	if err != nil {
		r.log.Error("load character", "error", err)
		_, _ = io.WriteString(session, "The game could not load your character. Please try again.\n")
		return
	}
	var inventory *domain.Inventory
	if char != nil {
		inventory, err = r.inventories.FindOrCreate(session.Context(), char.ID)
		if err != nil {
			r.log.Error("load inventory", "character_id", char.ID, "error", err)
			_, _ = io.WriteString(session, "The game could not load your inventory. Please try again.\n")
			return
		}
	}

	model := newGameModel(r.characters, r.inventories, r.world, r.log, identity, char, inventory)
	program := tea.NewProgram(
		model,
		tea.WithContext(session.Context()),
		tea.WithInput(session),
		tea.WithOutput(session),
		tea.WithEnvironment(append(session.Environ(), "TERM="+pty.Term)),
		tea.WithColorProfile(colorprofile.TrueColor),
		tea.WithoutSignalHandler(),
	)

	go forwardWindowSizes(program, resize, pty.Window)
	finalModel, runErr := program.Run()
	if final, ok := finalModel.(*gameModel); ok {
		final.leaveWorld()
	}
	if runErr != nil &&
		!errors.Is(runErr, context.Canceled) &&
		!errors.Is(runErr, tea.ErrProgramKilled) &&
		!errors.Is(runErr, tea.ErrInterrupted) {
		r.log.Error("terminal program failed", "error", runErr)
	}
}

func forwardWindowSizes(program *tea.Program, resize <-chan ssh.Window, initial ssh.Window) {
	program.Send(tea.WindowSizeMsg{Width: initial.Width, Height: initial.Height})
	for window := range resize {
		program.Send(tea.WindowSizeMsg{Width: window.Width, Height: window.Height})
	}
}

type phase uint8

const (
	phaseOnboarding phase = iota
	phaseJoining
	phasePlaying
)

type gameModel struct {
	characters  repository.CharacterRepository
	inventories repository.InventoryRepository
	world       *world.Manager
	log         *slog.Logger
	identity    Identity

	phase     phase
	input     textinput.Model
	chatInput textinput.Model
	message   string
	creating  bool
	character *domain.Character
	inventory *domain.Inventory

	worldSession world.Session
	joined       bool
	snapshot     world.Snapshot
	width        int
	height       int

	enhancedKeyboard   bool
	heldDirections     map[string]bool
	movementLoop       bool
	moveInFlight       bool
	attackInFlight     bool
	pickupInFlight     bool
	attackFrame        int
	facingX            int
	facingY            int
	inventoryOpen      bool
	skillsOpen         bool
	skillSpendInFlight bool
	renderer           Renderer
	events             []timedEvent
	nextEventID        uint64
	chatFocused        bool
	chatMessages       []world.ChatMessage
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

type timedEvent struct {
	id uint64
	EventView
}

const eventLifetime = 6 * time.Second

func newGameModel(
	characters repository.CharacterRepository,
	inventories repository.InventoryRepository,
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
	chatInput := textinput.New()
	chatInput.Prompt = ""
	chatInput.Placeholder = "Message"
	chatInput.CharLimit = 200
	chatInput.SetWidth(28)

	currentPhase := phaseOnboarding
	if char != nil {
		currentPhase = phaseJoining
	}
	return &gameModel{
		characters: characters, inventories: inventories,
		world: worldManager, log: log, identity: identity,
		phase: currentPhase, input: input, chatInput: chatInput,
		character: char, inventory: inventory,
		width: 80, height: 24, heldDirections: make(map[string]bool),
		facingX:  1,
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
		m.enhancedKeyboard = msg.SupportsEventTypes()
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.phase == phaseOnboarding {
			return m.updateOnboarding(msg)
		}
		if m.phase == phasePlaying {
			if m.chatFocused {
				return m.updateChat(msg)
			}
			switch strings.ToLower(msg.String()) {
			case "i":
				if m.skillsOpen {
					return m, nil
				}
				m.inventoryOpen = !m.inventoryOpen
				clear(m.heldDirections)
				m.movementLoop = false
				return m, nil
			case "k":
				if m.inventoryOpen {
					return m, nil
				}
				m.skillsOpen = !m.skillsOpen
				clear(m.heldDirections)
				m.movementLoop = false
				return m, nil
			case "t":
				if m.inventoryOpen || m.skillsOpen {
					return m, nil
				}
				m.chatFocused = true
				clear(m.heldDirections)
				m.movementLoop = false
				return m, m.chatInput.Focus()
			case "esc":
				if m.inventoryOpen {
					m.inventoryOpen = false
					return m, nil
				}
				if m.skillsOpen {
					m.skillsOpen = false
					return m, nil
				}
			case "x":
				if !m.inventoryOpen && !m.skillsOpen && !m.attackInFlight {
					m.attackInFlight = true
					m.attackFrame = 1
					return m, tea.Batch(m.attack(), attackAnimationTick(2))
				}
				return m, nil
			case "e":
				if !m.inventoryOpen && !m.skillsOpen && !m.pickupInFlight {
					m.pickupInFlight = true
					return m, m.pickup()
				}
				return m, nil
			case "1", "2", "3":
				if m.skillsOpen && !m.skillSpendInFlight && m.character.SkillPoints > 0 {
					m.skillSpendInFlight = true
					return m, m.spendSkill(msg.String())
				}
				return m, nil
			}
			if m.inventoryOpen || m.skillsOpen {
				return m, nil
			}
			return m, m.handleMovementPress(msg.String())
		}
	case tea.KeyReleaseMsg:
		if m.phase == phasePlaying && m.enhancedKeyboard {
			delete(m.heldDirections, directionKey(msg.String()))
		}
		return m, nil
	case movementTickMsg:
		if m.inventoryOpen || m.skillsOpen || m.chatFocused {
			m.movementLoop = false
			return m, nil
		}
		return m, m.handleMovementTick()
	case attackAnimationMsg:
		if msg.frame > 2 {
			m.attackFrame = 0
			m.attackInFlight = false
			return m, nil
		}
		m.attackFrame = msg.frame
		return m, attackAnimationTick(msg.frame + 1)
	case attackResultMsg:
		return m, nil
	case pickupResultMsg:
		if !msg.result.Found {
			m.pickupInFlight = false
			return m, nil
		}
		return m, m.storePickup(msg.result.Item)
	case itemStoredMsg:
		m.pickupInFlight = false
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
		m.skillSpendInFlight = false
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
				if existing, err := m.characters.FindByFingerprint(context.Background(), m.identity.Fingerprint); err == nil && existing != nil {
					m.character = existing
					inventory, inventoryErr := m.inventories.FindOrCreate(context.Background(), existing.ID)
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
		m.worldSession = msg.session
		m.joined = true
		m.phase = phasePlaying
		return m, tea.Batch(
			waitForSnapshot(msg.session.Updates),
			waitForWorldEvent(msg.session.Events),
			waitForChatMessage(msg.session.Chats),
			waitForKick(msg.session.Kicked),
		)
	case worldEventMsg:
		if !msg.ok {
			return m, nil
		}
		expiry := m.addEvent(EventView{Kind: msg.event.Kind, Message: msg.event.Message})
		return m, tea.Batch(expiry, waitForWorldEvent(m.worldSession.Events))
	case chatMessageMsg:
		if !msg.ok {
			return m, nil
		}
		m.chatMessages = append(m.chatMessages, msg.message)
		if len(m.chatMessages) > chatMessageLimit {
			m.chatMessages = m.chatMessages[len(m.chatMessages)-chatMessageLimit:]
		}
		return m, waitForChatMessage(m.worldSession.Chats)
	case chatSentMsg:
		return m, nil
	case eventExpiredMsg:
		for index, event := range m.events {
			if event.id == msg.id {
				m.events = append(m.events[:index], m.events[index+1:]...)
				break
			}
		}
		return m, nil
	case worldSnapshotMsg:
		if !msg.ok {
			return m, tea.Quit
		}
		m.snapshot = msg.snapshot
		commands := []tea.Cmd{waitForSnapshot(m.worldSession.Updates)}
		for _, player := range msg.snapshot.Players {
			if player.ID != m.character.ID {
				continue
			}
			locationChanged := m.character.AreaID != player.AreaID ||
				m.character.X != player.X ||
				m.character.Y != player.Y
			progressChanged := m.character.Level != player.Level ||
				m.character.Experience != player.Experience ||
				m.character.SkillPoints != player.SkillPoints ||
				m.character.Attack != player.Attack ||
				m.character.Defense != player.Defense ||
				m.character.Vitality != player.Vitality
			m.character.AreaID = player.AreaID
			m.character.X, m.character.Y = player.X, player.Y
			m.character.Level = player.Level
			m.character.Experience = player.Experience
			m.character.SkillPoints = player.SkillPoints
			m.character.Attack = player.Attack
			m.character.Defense = player.Defense
			m.character.Vitality = player.Vitality
			if locationChanged {
				commands = append(commands, m.savePosition())
			}
			if progressChanged {
				commands = append(commands, m.saveProgress())
			}
			break
		}
		return m, tea.Batch(commands...)
	case worldKickedMsg:
		m.message = "This character connected from another session."
		return m, tea.Quit
	case playerMovedMsg:
		m.moveInFlight = false
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

func (m *gameModel) updateOnboarding(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		if m.creating {
			return m, nil
		}
		name, err := domain.ValidateCharacterName(m.input.Value())
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.message = ""
		m.creating = true
		return m, m.createCharacter(name)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Err != nil {
		m.message = m.input.Err.Error()
	} else {
		m.message = ""
	}
	return m, cmd
}

func (m *gameModel) updateChat(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		message := strings.TrimSpace(m.chatInput.Value())
		m.chatInput.SetValue("")
		m.chatInput.Blur()
		m.chatFocused = false
		if message == "" {
			return m, nil
		}
		return m, m.sendChat(message)
	case "esc":
		m.chatInput.SetValue("")
		m.chatInput.Blur()
		m.chatFocused = false
		return m, nil
	}
	var command tea.Cmd
	m.chatInput, command = m.chatInput.Update(msg)
	return m, command
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
		Snapshot:      m.snapshot,
		InventoryOpen: m.inventoryOpen,
		Inventory:     m.inventory,
		SkillsOpen:    m.skillsOpen,
		Events:        m.eventViews(),
		ChatMessages:  append([]world.ChatMessage(nil), m.chatMessages...),
		ChatFocused:   m.chatFocused,
		ChatInput:     m.chatInput.View(),
		AttackFrame:   m.attackFrame,
		FacingX:       m.facingX,
		FacingY:       m.facingY,
	}
}

func (m *gameModel) createCharacter(name string) tea.Cmd {
	return func() tea.Msg {
		char, err := m.characters.Create(
			context.Background(),
			repository.CreateCharacterParams{
				KeyFingerprint: m.identity.Fingerprint,
				PublicKeyType:  m.identity.KeyType,
				PublicKey:      m.identity.PublicKey,
				Name:           name,
			},
		)
		if err != nil {
			return characterCreatedMsg{err: err}
		}
		inventory, err := m.inventories.FindOrCreate(context.Background(), char.ID)
		return characterCreatedMsg{character: char, inventory: inventory, err: err}
	}
}

func (m *gameModel) joinWorld() tea.Cmd {
	return func() tea.Msg {
		session := m.world.Join(world.Player{
			ID: m.character.ID, Name: m.character.Name,
			AreaID: m.character.AreaID, X: m.character.X, Y: m.character.Y,
			Level: m.character.Level, Experience: m.character.Experience,
			SkillPoints: m.character.SkillPoints,
			Attack:      m.character.Attack, Defense: m.character.Defense, Vitality: m.character.Vitality,
		})
		return worldJoinedMsg{session: session}
	}
}

func (m *gameModel) movePlayer(dx, dy int) tea.Cmd {
	return func() tea.Msg {
		return playerMovedMsg{player: m.world.Move(
			m.character.ID, m.worldSession.Token, dx, dy,
		)}
	}
}

func (m *gameModel) attack() tea.Cmd {
	return func() tea.Msg {
		return attackResultMsg{result: m.world.Attack(m.character.ID, m.worldSession.Token)}
	}
}

func (m *gameModel) pickup() tea.Cmd {
	return func() tea.Msg {
		return pickupResultMsg{result: m.world.Pickup(m.character.ID, m.worldSession.Token)}
	}
}

func (m *gameModel) spendSkill(key string) tea.Cmd {
	skills := map[string]string{"1": "attack", "2": "defense", "3": "vitality"}
	skill := skills[key]
	return func() tea.Msg {
		return skillSpentMsg{player: m.world.SpendSkillPoint(
			m.character.ID, m.worldSession.Token, skill,
		)}
	}
}

func (m *gameModel) sendChat(message string) tea.Cmd {
	return func() tea.Msg {
		return chatSentMsg{ok: m.world.Chat(
			m.character.ID, m.worldSession.Token, message,
		)}
	}
}

func (m *gameModel) storePickup(drop world.GroundItem) tea.Cmd {
	return func() tea.Msg {
		definition, ok := m.world.Items().Item(drop.ItemID)
		if !ok {
			return itemStoredMsg{itemName: drop.Name, err: errors.New("unknown picked up item")}
		}
		inventory, err := m.inventories.AddItem(
			context.Background(), m.character.ID, definition.ID, definition.MaxStack,
		)
		return itemStoredMsg{inventory: inventory, itemName: definition.Name, err: err}
	}
}

func (m *gameModel) handleMovementPress(key string) tea.Cmd {
	direction := directionKey(key)
	if direction == "" {
		return nil
	}
	m.setFacing(movement(key))
	if !m.enhancedKeyboard {
		dx, dy := movement(key)
		if m.moveInFlight {
			return nil
		}
		m.moveInFlight = true
		return m.movePlayer(dx, dy)
	}

	m.heldDirections[direction] = true
	commands := make([]tea.Cmd, 0, 2)
	if !m.moveInFlight {
		dx, dy := heldMovement(m.heldDirections)
		m.moveInFlight = true
		commands = append(commands, m.movePlayer(dx, dy))
	}
	if !m.movementLoop {
		m.movementLoop = true
		commands = append(commands, movementTick())
	}
	return tea.Batch(commands...)
}

func (m *gameModel) setFacing(dx, dy int) {
	if dx != 0 || dy != 0 {
		m.facingX, m.facingY = dx, dy
	}
}

func (m *gameModel) handleMovementTick() tea.Cmd {
	if !m.enhancedKeyboard || len(m.heldDirections) == 0 {
		m.movementLoop = false
		return nil
	}
	commands := []tea.Cmd{movementTick()}
	if !m.moveInFlight {
		dx, dy := heldMovement(m.heldDirections)
		if dx != 0 || dy != 0 {
			m.moveInFlight = true
			commands = append(commands, m.movePlayer(dx, dy))
		}
	}
	return tea.Batch(commands...)
}

func movementTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return movementTickMsg{}
	})
}

func attackAnimationTick(frame int) tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg {
		return attackAnimationMsg{frame: frame}
	})
}

func (m *gameModel) savePosition() tea.Cmd {
	id, areaID, x, y := m.character.ID, m.character.AreaID, m.character.X, m.character.Y
	return func() tea.Msg {
		return positionSavedMsg{err: m.characters.UpdateLocation(context.Background(), id, areaID, x, y)}
	}
}

func (m *gameModel) saveProgress() tea.Cmd {
	id := m.character.ID
	level, experience, skillPoints := m.character.Level, m.character.Experience, m.character.SkillPoints
	attack, defense, vitality := m.character.Attack, m.character.Defense, m.character.Vitality
	return func() tea.Msg {
		return progressSavedMsg{err: m.characters.UpdateProgress(
			context.Background(), id, level, experience, skillPoints,
			attack, defense, vitality,
		)}
	}
}

func waitForSnapshot(updates <-chan world.Snapshot) tea.Cmd {
	return func() tea.Msg {
		snapshot, ok := <-updates
		return worldSnapshotMsg{snapshot: snapshot, ok: ok}
	}
}

func waitForKick(kicked <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-kicked
		return worldKickedMsg{}
	}
}

func waitForWorldEvent(events <-chan world.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		return worldEventMsg{event: event, ok: ok}
	}
}

func waitForChatMessage(messages <-chan world.ChatMessage) tea.Cmd {
	return func() tea.Msg {
		message, ok := <-messages
		return chatMessageMsg{message: message, ok: ok}
	}
}

func (m *gameModel) addEvent(event EventView) tea.Cmd {
	m.nextEventID++
	id := m.nextEventID
	m.events = append(m.events, timedEvent{id: id, EventView: event})
	return tea.Tick(eventLifetime, func(time.Time) tea.Msg {
		return eventExpiredMsg{id: id}
	})
}

func (m *gameModel) eventViews() []EventView {
	events := make([]EventView, len(m.events))
	for index, event := range m.events {
		events[index] = event.EventView
	}
	return events
}

func (m *gameModel) leaveWorld() {
	if m.joined {
		m.world.Leave(m.character.ID, m.worldSession.Token)
		m.joined = false
	}
}

func movement(key string) (int, int) {
	switch strings.ToLower(key) {
	case "w", "up":
		return 0, -1
	case "s", "down":
		return 0, 1
	case "a", "left":
		return -1, 0
	case "d", "right":
		return 1, 0
	default:
		return 0, 0
	}
}

func directionKey(key string) string {
	switch strings.ToLower(key) {
	case "w", "up":
		return "up"
	case "s", "down":
		return "down"
	case "a", "left":
		return "left"
	case "d", "right":
		return "right"
	default:
		return ""
	}
}

func heldMovement(held map[string]bool) (int, int) {
	var dx, dy int
	if held["left"] {
		dx--
	}
	if held["right"] {
		dx++
	}
	if held["up"] {
		dy--
	}
	if held["down"] {
		dy++
	}
	return dx, dy
}
