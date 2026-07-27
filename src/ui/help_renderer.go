package ui

import "charm.land/lipgloss/v2"

type HelpRenderer struct{}

func (HelpRenderer) RenderOver(game string, width, height int) string {
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		helpTitleStyle.Render("HELP"),
		"",
		"WASD / arrows   Move",
		"X               Attack nearby enemies",
		"E               Pick up nearby item",
		"I               Open inventory",
		"K               Open skills",
		"T               Focus chat",
		"Enter           Send focused chat message",
		"Esc             Close menu or cancel chat",
		"F1              Open or close help",
		"Ctrl+C          Disconnect",
		"",
		mutedStyle.Render("F1 or Esc to close"),
	)
	window := helpWindowStyle.Render(body)
	windowWidth, windowHeight := lipgloss.Size(window)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(game).X(0).Y(0).Z(0),
		lipgloss.NewLayer(window).
			X(max((width-windowWidth)/2, 0)).
			Y(max((height-windowHeight)/2, 0)).
			Z(1),
	).Render()
}

var (
	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#38BDF8"))
	helpWindowStyle = lipgloss.NewStyle().
			Width(48).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#64748B"))
)
