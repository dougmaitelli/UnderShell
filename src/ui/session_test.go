package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

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

func TestGameRenderSanitizesViewportByConstruction(t *testing.T) {
	model := newGameModel(nil, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", X: 10, Y: 10,
	})
	model.phase = phasePlaying
	model.width, model.height = 50, 15
	model.snapshot = world.Snapshot{Players: []world.Player{
		{ID: 1, Name: "Aria", X: 10, Y: 10},
		{ID: 2, Name: "Rowan", X: 11, Y: 10},
	}}
	output := model.gameView()
	if !strings.Contains(output, "@") || !strings.Contains(output, "●") {
		t.Fatalf("players not rendered: %q", output)
	}
	if !strings.Contains(output, "Players: 2") {
		t.Fatalf("player count not rendered: %q", output)
	}
	if !strings.Contains(output, "Nearby: Rowan") {
		t.Fatalf("nearby player name not rendered: %q", output)
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
	model := newGameModel(nil, nil, nil, Identity{}, nil)
	model.width, model.height = 80, 24
	output := model.welcomeView()
	if width, height := lipgloss.Size(output); width != 80 || height != 24 {
		t.Fatalf("welcome size = %dx%d, want 80x24", width, height)
	}
}
