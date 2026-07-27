package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	characters repository.CharacterRepository
	world      *world.Manager
	log        *slog.Logger
}

func New(characters repository.CharacterRepository, worldManager *world.Manager, log *slog.Logger) *Runner {
	return &Runner{characters: characters, world: worldManager, log: log}
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

	model := newGameModel(r.characters, r.world, r.log, identity, char)
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
	characters repository.CharacterRepository
	world      *world.Manager
	log        *slog.Logger
	identity   Identity

	phase     phase
	input     textinput.Model
	message   string
	creating  bool
	character *domain.Character

	worldSession world.Session
	joined       bool
	snapshot     world.Snapshot
	width        int
	height       int

	enhancedKeyboard bool
	heldDirections   map[string]bool
	movementLoop     bool
	moveInFlight     bool
}

type characterCreatedMsg struct {
	character *domain.Character
	err       error
}

type worldJoinedMsg struct {
	session world.Session
}

type worldSnapshotMsg struct {
	snapshot world.Snapshot
	ok       bool
}
type worldKickedMsg struct{}

type playerMovedMsg struct {
	player world.Player
}

type positionSavedMsg struct {
	err error
}

type movementTickMsg struct{}

func newGameModel(
	characters repository.CharacterRepository,
	worldManager *world.Manager,
	log *slog.Logger,
	identity Identity,
	char *domain.Character,
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
		characters: characters, world: worldManager, log: log, identity: identity,
		phase: currentPhase, input: input, character: char,
		width: 80, height: 24, heldDirections: make(map[string]bool),
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
			return m, m.handleMovementPress(msg.String())
		}
	case tea.KeyReleaseMsg:
		if m.phase == phasePlaying && m.enhancedKeyboard {
			delete(m.heldDirections, directionKey(msg.String()))
		}
		return m, nil
	case movementTickMsg:
		return m, m.handleMovementTick()
	case characterCreatedMsg:
		m.creating = false
		if msg.err != nil {
			if errors.Is(msg.err, repository.ErrCharacterKeyExists) {
				if existing, err := m.characters.FindByFingerprint(context.Background(), m.identity.Fingerprint); err == nil && existing != nil {
					m.character = existing
					m.phase = phaseJoining
					return m, m.joinWorld()
				}
			}
			m.message = msg.err.Error()
			return m, nil
		}
		m.character = msg.character
		m.phase = phaseJoining
		m.input.Blur()
		return m, m.joinWorld()
	case worldJoinedMsg:
		m.worldSession = msg.session
		m.joined = true
		m.phase = phasePlaying
		return m, tea.Batch(waitForSnapshot(msg.session.Updates), waitForKick(msg.session.Kicked))
	case worldSnapshotMsg:
		if !msg.ok {
			return m, tea.Quit
		}
		m.snapshot = msg.snapshot
		for _, player := range msg.snapshot.Players {
			if player.ID != m.character.ID {
				continue
			}
			locationChanged := m.character.AreaID != player.AreaID ||
				m.character.X != player.X ||
				m.character.Y != player.Y
			m.character.AreaID = player.AreaID
			m.character.X, m.character.Y = player.X, player.Y
			if locationChanged {
				return m, tea.Batch(
					waitForSnapshot(m.worldSession.Updates),
					m.savePosition(),
				)
			}
			break
		}
		return m, waitForSnapshot(m.worldSession.Updates)
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

func (m *gameModel) View() tea.View {
	var content string
	switch m.phase {
	case phaseOnboarding:
		content = m.welcomeView()
	case phaseJoining:
		content = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, mutedStyle.Render("Entering the realm…"))
	case phasePlaying:
		content = m.gameView()
	}
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "SSH Realms"
	view.KeyboardEnhancements.ReportEventTypes = true
	return view
}

func (m *gameModel) welcomeView() string {
	if m.width < 46 || m.height < 14 {
		return lipgloss.Place(
			max(m.width, 1), max(m.height, 1),
			lipgloss.Center, lipgloss.Center,
			errorStyle.Render("Please resize your terminal to at least 46×14."),
		)
	}

	status := "Enter to begin • Ctrl+C to leave"
	if m.creating {
		status = "Creating character…"
	}
	if m.message != "" {
		status = errorStyle.Render(m.message)
	} else {
		status = mutedStyle.Render(status)
	}

	field := lipgloss.JoinHorizontal(
		lipgloss.Center,
		labelStyle.Render("Character name: "),
		m.input.View(),
	)
	body := lipgloss.JoinVertical(
		lipgloss.Center,
		titleStyle.Render("SSH REALMS"),
		"",
		"Your SSH key has no character yet.",
		"",
		field,
		"",
		status,
	)
	box := welcomeBoxStyle.Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *gameModel) gameView() string {
	if m.width < 40 || m.height < 10 {
		return lipgloss.Place(
			max(m.width, 1), max(m.height, 1),
			lipgloss.Center, lipgloss.Center,
			errorStyle.Render("Please resize your terminal to at least 40×10."),
		)
	}

	mapHeight := max(m.height-3, 1)
	grid := make([][]string, mapHeight)
	for y := range grid {
		grid[y] = make([]string, m.width)
		for x := range grid[y] {
			grid[y][x] = " "
		}
	}

	self := world.Player{
		ID: m.character.ID, Name: m.character.Name, AreaID: m.character.AreaID,
		X: m.character.X, Y: m.character.Y,
	}
	for _, player := range m.snapshot.Players {
		if player.ID == m.character.ID {
			self = player
			break
		}
	}
	left, top := self.X-m.width/2, self.Y-mapHeight/2
	if m.snapshot.Area != nil {
		for screenY := 0; screenY < mapHeight; screenY++ {
			for screenX := 0; screenX < m.width; screenX++ {
				point := world.Point{X: left + screenX, Y: top + screenY}
				if !m.snapshot.Area.InBounds(point) {
					continue
				}
				tile := m.snapshot.Area.Tile(point)
				grid[screenY][screenX] = renderTile(tile)
				if _, ok := m.snapshot.Area.Waypoint(point); ok {
					grid[screenY][screenX] = waypointStyle.Render("◇")
				}
			}
		}
	}
	nearby := make([]string, 0, len(m.snapshot.Players))
	visiblePlayers := make([]world.Player, 0, len(m.snapshot.Players))
	for _, player := range m.snapshot.Players {
		x, y := player.X-left, player.Y-top
		if x < 0 || y < 0 || x >= m.width || y >= mapHeight {
			continue
		}
		visiblePlayers = append(visiblePlayers, player)
		if player.ID != m.character.ID {
			nearby = append(nearby, player.Name)
		}
	}
	sort.SliceStable(visiblePlayers, func(i, j int) bool {
		return visiblePlayers[i].ID != m.character.ID &&
			visiblePlayers[j].ID == m.character.ID
	})
	for _, player := range visiblePlayers {
		style := otherPlayerStyle
		marker := "○"
		if player.ID == m.character.ID {
			style = selfPlayerStyle
			marker = "@"
		}
		drawPlayer(
			grid,
			player.X-left,
			player.Y-top,
			marker,
			player.Name,
			style,
		)
	}
	sort.Strings(nearby)

	rows := make([]string, mapHeight)
	for y, row := range grid {
		rows[y] = strings.Join(row, "")
	}
	header := headerStyle.Render(fmt.Sprintf(
		" %s • %s  (%d, %d)  Players here: %d",
		self.Name, areaName(m.snapshot.Area), self.X, self.Y, len(m.snapshot.Players),
	))
	footer := " WASD/arrows: move • Ctrl+C: quit"
	if len(nearby) > 0 {
		footer += " • Nearby: " + strings.Join(nearby, ", ")
	}
	return header + "\n" + strings.Join(rows, "\n") + "\n" + mutedStyle.Render(footer)
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
		return characterCreatedMsg{character: char, err: err}
	}
}

func (m *gameModel) joinWorld() tea.Cmd {
	return func() tea.Msg {
		session := m.world.Join(world.Player{
			ID: m.character.ID, Name: m.character.Name,
			AreaID: m.character.AreaID, X: m.character.X, Y: m.character.Y,
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

func (m *gameModel) handleMovementPress(key string) tea.Cmd {
	direction := directionKey(key)
	if direction == "" {
		return nil
	}
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

func (m *gameModel) savePosition() tea.Cmd {
	id, areaID, x, y := m.character.ID, m.character.AreaID, m.character.X, m.character.Y
	return func() tea.Msg {
		return positionSavedMsg{err: m.characters.UpdateLocation(context.Background(), id, areaID, x, y)}
	}
}

func areaName(area *world.Area) string {
	if area == nil {
		return "Unknown Area"
	}
	return area.Name
}

func renderTile(tile rune) string {
	switch tile {
	case '#':
		return wallStyle.Render("█")
	case '.':
		return " "
	default:
		return string(tile)
	}
}

func drawPlayer(grid [][]string, x, baseY int, marker, name string, style lipgloss.Style) {
	drawCentered(grid, x, baseY-3, terminalCellRunes(marker+" "+name), style)
	drawCentered(grid, x, baseY-2, []rune("O"), style)
	drawCentered(grid, x, baseY-1, []rune("/|\\"), style)
	drawCentered(grid, x, baseY, []rune("/ \\"), style)
}

func drawCentered(grid [][]string, centerX, y int, content []rune, style lipgloss.Style) {
	if y < 0 || y >= len(grid) {
		return
	}
	startX := centerX - len(content)/2
	for offset, cell := range content {
		x := startX + offset
		if x < 0 || x >= len(grid[y]) || cell == ' ' {
			continue
		}
		grid[y][x] = style.Render(string(cell))
	}
}

func terminalCellRunes(value string) []rune {
	cells := make([]rune, 0, len(value))
	for _, cell := range value {
		if lipgloss.Width(string(cell)) == 1 {
			cells = append(cells, cell)
		} else {
			cells = append(cells, '?')
		}
	}
	return cells
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

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7DD3FC"))
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0"))
	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8"))
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FB7185"))
	welcomeBoxStyle = lipgloss.NewStyle().
			Width(40).
			Align(lipgloss.Center).
			Padding(1, 2).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#38BDF8"))
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E2E8F0"))
	selfPlayerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FBBF24"))
	otherPlayerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#38BDF8"))
	wallStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#334155"))
	waypointStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A78BFA"))
)
