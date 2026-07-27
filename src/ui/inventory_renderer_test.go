package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestInventoryRendererCompositesWindowOverGame(t *testing.T) {
	width, height := 50, 14
	background := make([]string, height)
	for row := range background {
		background[row] = strings.Repeat(string(rune('A'+row)), width)
	}

	output := ansi.Strip(InventoryRenderer{}.RenderOver(strings.Join(background, "\n"), width, height, nil))
	rows := strings.Split(output, "\n")
	if len(rows) != height {
		t.Fatalf("rendered height = %d, want %d", len(rows), height)
	}
	if rows[0] != background[0] || rows[height-1] != background[height-1] {
		t.Fatal("inventory window replaced game content outside its bounds")
	}
	if !strings.Contains(output, "INVENTORY") {
		t.Fatal("inventory window was not composited over the game")
	}
}

func TestInventoryRendererUsesLatestGameFrame(t *testing.T) {
	renderer := InventoryRenderer{}
	firstGame := "first frame" + strings.Repeat("\n", 23)
	secondGame := "second frame" + strings.Repeat("\n", 23)
	first := ansi.Strip(renderer.RenderOver(firstGame, 80, 24, nil))
	second := ansi.Strip(renderer.RenderOver(secondGame, 80, 24, nil))

	if !strings.Contains(first, "first frame") {
		t.Fatal("first game frame is missing")
	}
	if !strings.Contains(second, "second frame") || strings.Contains(second, "first frame") {
		t.Fatal("inventory did not render over the latest game frame")
	}
}
