package ui

import "charm.land/lipgloss/v2"

type WelcomeRenderer struct{}

func (WelcomeRenderer) Render(state ViewState) string {
	if state.Width < 46 || state.Height < 14 {
		return lipgloss.Place(
			max(state.Width, 1), max(state.Height, 1),
			lipgloss.Center, lipgloss.Center,
			errorStyle.Render("Please resize your terminal to at least 46×14."),
		)
	}

	status := "Enter to begin • Ctrl+C to leave"
	if state.Creating {
		status = "Creating character…"
	}
	if state.Message != "" {
		status = errorStyle.Render(state.Message)
	} else {
		status = mutedStyle.Render(status)
	}

	field := lipgloss.JoinHorizontal(
		lipgloss.Center,
		labelStyle.Render("Character name: "),
		state.Input,
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
	return lipgloss.Place(state.Width, state.Height, lipgloss.Center, lipgloss.Center, box)
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7DD3FC"))
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0"))
	welcomeBoxStyle = lipgloss.NewStyle().
			Width(40).
			Align(lipgloss.Center).
			Padding(1, 2).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#38BDF8"))
)
