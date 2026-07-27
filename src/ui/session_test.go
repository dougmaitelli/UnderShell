package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"sshrpg/src/domain"
	"sshrpg/src/world"
)

func TestMovementKeys(t *testing.T) {
	tests := map[string][2]int{
		"w": {0, -1}, "W": {0, -1}, "up": {0, -1},
		"s": {0, 1}, "a": {-1, 0}, "d": {1, 0},
		"x": {0, 0},
	}
	for key, expected := range tests {
		dx, dy := movement(key)
		if dx != expected[0] || dy != expected[1] {
			t.Fatalf("%q: got (%d,%d), want %v", key, dx, dy, expected)
		}
	}
}

func TestEnhancedMovementKeepsEarlierDirectionHeld(t *testing.T) {
	model := newGameModel(nil, nil, nil, nil, Identity{}, &domain.Character{ID: 1}, nil)
	model.phase = phasePlaying
	model.enhancedKeyboard = true

	model.handleMovementPress("a")
	model.moveInFlight = false
	model.handleMovementPress("s")
	if dx, dy := heldMovement(model.heldDirections); dx != -1 || dy != 1 {
		t.Fatalf("held movement = (%d,%d), want (-1,1)", dx, dy)
	}

	_, _ = model.Update(tea.KeyReleaseMsg(tea.Key{Text: "s", Code: 's'}))
	if dx, dy := heldMovement(model.heldDirections); dx != -1 || dy != 0 {
		t.Fatalf("movement after releasing S = (%d,%d), want (-1,0)", dx, dy)
	}
}

func TestInventoryCanBeOpenedAndClosed(t *testing.T) {
	model := newGameModel(nil, nil, nil, nil, Identity{}, &domain.Character{ID: 1, Name: "Aria"}, nil)
	model.phase = phasePlaying
	model.width, model.height = 80, 24
	model.heldDirections["left"] = true

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	if !model.inventoryOpen {
		t.Fatal("inventory did not open")
	}
	if len(model.heldDirections) != 0 {
		t.Fatal("opening inventory did not clear held movement")
	}
	output := ansi.Strip(model.View().Content)
	if !strings.Contains(output, "INVENTORY") || !strings.Contains(output, "Your inventory is empty.") {
		t.Fatalf("inventory window not rendered: %q", output)
	}

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if model.inventoryOpen {
		t.Fatal("inventory did not close with Escape")
	}

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	if model.inventoryOpen {
		t.Fatal("inventory did not close with I")
	}
}

func TestSkillsAndInventoryMenusAreMutuallyExclusive(t *testing.T) {
	model := newGameModel(nil, nil, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", Level: 2, SkillPoints: 2, Attack: 1,
	}, nil)
	model.phase = phasePlaying
	model.width, model.height = 80, 24

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "k", Code: 'k'}))
	if !model.skillsOpen || model.inventoryOpen {
		t.Fatalf("skills did not open exclusively: skills %v, inventory %v",
			model.skillsOpen, model.inventoryOpen)
	}
	plain := ansi.Strip(model.View().Content)
	if !strings.Contains(plain, "SKILLS") ||
		!strings.Contains(plain, "Unspent points: 2") ||
		!strings.Contains(plain, "[1] Attack   1") {
		t.Fatalf("skills menu not rendered: %q", plain)
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	if !model.skillsOpen || model.inventoryOpen {
		t.Fatal("inventory opened over skills")
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "k", Code: 'k'}))
	if !model.inventoryOpen || model.skillsOpen {
		t.Fatal("skills opened over inventory")
	}
}

func TestChatFocusCapturesTypingUntilEnter(t *testing.T) {
	model := newGameModel(nil, nil, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", Level: 1,
	}, nil)
	model.phase = phasePlaying
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}))
	if !model.chatFocused {
		t.Fatal("T did not focus chat")
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	if model.inventoryOpen || model.chatInput.Value() != "i" {
		t.Fatalf("chat did not capture gameplay key: inventory %v, input %q",
			model.inventoryOpen, model.chatInput.Value())
	}
	_, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if model.chatFocused || command == nil {
		t.Fatalf("Enter did not send and return focus: focused %v, command %v",
			model.chatFocused, command != nil)
	}
}

func TestAttackKeyStartsSlashAnimation(t *testing.T) {
	model := newGameModel(nil, nil, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", AreaID: "meadow", X: 2, Y: 1,
	}, nil)
	model.phase = phasePlaying
	_, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}))
	if command == nil || !model.attackInFlight || model.attackFrame != 1 {
		t.Fatalf(
			"attack state = in-flight %v, frame %d, command %v",
			model.attackInFlight, model.attackFrame, command != nil,
		)
	}
}

func TestPickupKeyRequestsNearbyDrop(t *testing.T) {
	model := newGameModel(nil, nil, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", AreaID: "meadow", X: 2, Y: 1,
	}, nil)
	model.phase = phasePlaying
	_, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	if command == nil || !model.pickupInFlight {
		t.Fatalf("pickup state = in-flight %v, command %v", model.pickupInFlight, command != nil)
	}
}

func TestGameRenderSanitizesViewportByConstruction(t *testing.T) {
	model := newGameModel(nil, nil, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", AreaID: "meadow", X: 2, Y: 1,
	}, nil)
	areas, err := world.NewAreas([]world.AreaDefinition{{
		ID: "meadow", Name: "Meadow",
		Layout: []string{"######", "#....#", "######"},
		Spawn:  world.Point{X: 1, Y: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	area, _ := areas.Area("meadow")
	model.phase = phasePlaying
	model.width, model.height = 50, 15
	model.snapshot = world.Snapshot{Area: area, Players: []world.Player{
		{
			ID: 1, Name: "Aria", AreaID: "meadow", X: 2, Y: 1,
			Level: 2, Experience: 25, SkillPoints: 1, Health: 8, MaxHealth: 10,
		},
		{ID: 2, Name: "Rowan", AreaID: "meadow", X: 12, Y: 1},
	}}
	output := model.renderer.game.Render(model.viewState())
	plain := ansi.Strip(output)
	if !strings.Contains(plain, "Aria") ||
		!strings.Contains(plain, "Rowan") ||
		!strings.Contains(plain, "○ Rowan") ||
		!strings.Contains(plain, "/|\\") ||
		!strings.Contains(plain, "/ \\") {
		t.Fatalf("players not rendered: %q", output)
	}
	if !strings.Contains(output, "Players here: 2") {
		t.Fatalf("player count not rendered: %q", output)
	}
	if !strings.Contains(output, "Nearby: Rowan") {
		t.Fatalf("nearby player name not rendered: %q", output)
	}
	if !strings.Contains(output, "F1: help") {
		t.Fatalf("help control not rendered: %q", output)
	}
	if !strings.Contains(plain, "Lv 2 • XP 25/400 • SP 1 • HP 8/10") {
		t.Fatalf("player progression not rendered: %q", plain)
	}
}

func TestHelpWindowListsCommandsAndBlocksGameplayInput(t *testing.T) {
	model := newGameModel(nil, nil, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", Level: 1,
	}, nil)
	model.phase = phasePlaying
	model.width, model.height = 80, 24
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	if !model.helpOpen {
		t.Fatal("F1 did not open help")
	}
	plain := ansi.Strip(model.View().Content)
	for _, command := range []string{
		"HELP", "WASD / arrows", "Attack nearby", "Open inventory",
		"Open skills", "Focus chat", "Ctrl+C",
	} {
		if !strings.Contains(plain, command) {
			t.Fatalf("help is missing %q: %q", command, plain)
		}
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	if model.inventoryOpen {
		t.Fatal("gameplay shortcut opened inventory over help")
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if model.helpOpen {
		t.Fatal("Escape did not close help")
	}
}

func TestGroundItemsUseOneGenericMarker(t *testing.T) {
	model := newGameModel(nil, nil, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", AreaID: "meadow", X: 10, Y: 5,
	}, nil)
	model.phase = phasePlaying
	model.width, model.height = 40, 14
	model.snapshot = world.Snapshot{
		Players: []world.Player{{ID: 1, Name: "Aria", AreaID: "meadow", X: 10, Y: 5}},
		Drops: []world.GroundItem{
			{ID: 1, ItemID: "slime_gel", Name: "Slime Gel", AreaID: "meadow", X: 3, Y: 5},
			{ID: 2, ItemID: "health_potion", Name: "Health Potion", AreaID: "meadow", X: 17, Y: 5},
		},
	}
	plain := ansi.Strip(model.renderer.game.Render(model.viewState()))
	if count := strings.Count(plain, "◆"); count != 2 {
		t.Fatalf("generic ground item marker count = %d, want 2: %q", count, plain)
	}
	if strings.Contains(plain, "SP 0") {
		t.Fatalf("zero skill points should not be rendered: %q", plain)
	}
}

func TestPlayerBaseIsAnchoredAtWorldCoordinate(t *testing.T) {
	grid := make([][]string, 8)
	for y := range grid {
		grid[y] = make([]string, 12)
		for x := range grid[y] {
			grid[y][x] = " "
		}
	}
	drawPlayer(grid, 6, 6, "@", "Aria", lipgloss.NewStyle())

	if grid[6][5] != "/" || grid[6][6] != " " || grid[6][7] != "\\" {
		t.Fatalf("feet are not centered on base coordinate: %#v", grid[6][4:9])
	}
	if strings.Join(grid[3], "") != "   @ Aria   " {
		t.Fatalf("name is not centered above the figure: %q", strings.Join(grid[3], ""))
	}
}

func TestSlashFramesAreDirectional(t *testing.T) {
	grid := make([][]string, 8)
	for y := range grid {
		grid[y] = make([]string, 12)
		for x := range grid[y] {
			grid[y][x] = " "
		}
	}
	drawSlash(grid, 5, 5, 1, 0, 1)
	if ansi.Strip(grid[4][7]) != "/" {
		t.Fatalf("first right-facing slash frame = %q", grid[4][7])
	}
	drawSlash(grid, 5, 5, 1, 0, 2)
	if ansi.Strip(grid[4][7]) != "─" {
		t.Fatalf("second right-facing slash frame = %q", grid[4][7])
	}
}

func TestPlayerStylesEmitANSIColors(t *testing.T) {
	local := selfPlayerStyle.Render("@ Aria")
	remote := otherPlayerStyle.Render("○ Rowan")
	if local == ansi.Strip(local) {
		t.Fatal("local player style did not emit ANSI styling")
	}
	if remote == ansi.Strip(remote) {
		t.Fatal("remote player style did not emit ANSI styling")
	}
}

func TestWelcomeBorderRowsHaveEqualWidth(t *testing.T) {
	box := welcomeBoxStyle.Render(strings.Join([]string{
		titleStyle.Render("SSH REALMS"),
		"",
		"Your SSH key has no character yet.",
		"",
		"Character name: Aria",
		"",
		"Enter to begin • Ctrl+C to leave",
	}, "\n"))
	width := lipgloss.Width(box)
	for _, line := range strings.Split(box, "\n") {
		if lineWidth := lipgloss.Width(line); lineWidth != width {
			t.Errorf("welcome row width = %d, want %d: %q", lineWidth, width, line)
		}
	}
}

func TestWelcomeViewUsesTerminalDimensions(t *testing.T) {
	model := newGameModel(nil, nil, nil, nil, Identity{}, nil, nil)
	model.width, model.height = 80, 24
	output := model.renderer.welcome.Render(model.viewState())
	if width, height := lipgloss.Size(output); width != 80 || height != 24 {
		t.Fatalf("welcome size = %dx%d, want 80x24", width, height)
	}
}
