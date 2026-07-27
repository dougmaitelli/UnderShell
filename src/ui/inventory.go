package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"sshrpg/src/domain"
)

func (m *gameModel) updateInventoryInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "i", "esc":
		m.mode = inputModeGame
	}
	return m, nil
}

type InventoryRenderer struct{}

func (InventoryRenderer) RenderOver(
	game string,
	width, height int,
	inventory *domain.Inventory,
) string {
	contents := mutedStyle.Render("Your inventory is empty.")
	if inventory != nil && len(inventory.Items) > 0 {
		rows := make([]string, len(inventory.Items))
		for index, item := range inventory.Items {
			rows[index] = fmt.Sprintf("%2d. %s ×%d", item.Slot, item.ItemKey, item.Quantity)
		}
		contents = lipgloss.JoinVertical(lipgloss.Left, rows...)
	}
	body := lipgloss.JoinVertical(
		lipgloss.Center,
		inventoryTitleStyle.Render("INVENTORY"),
		"",
		contents,
		"",
		mutedStyle.Render("I or Esc to close"),
	)
	window := inventoryWindowStyle.Render(body)
	windowWidth, windowHeight := lipgloss.Size(window)

	gameLayer := lipgloss.NewLayer(game).X(0).Y(0).Z(0)
	inventoryLayer := lipgloss.NewLayer(window).
		X(max((width-windowWidth)/2, 0)).
		Y(max((height-windowHeight)/2, 0)).
		Z(1)

	return lipgloss.NewCompositor(gameLayer, inventoryLayer).Render()
}

var (
	inventoryTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FBBF24"))
	inventoryWindowStyle = lipgloss.NewStyle().
				Width(34).
				Align(lipgloss.Center).
				Padding(1, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#64748B"))
)
