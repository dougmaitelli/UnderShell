package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"sshrpg/src/world"
)

const chatMessageLimit = 10
const chatTextWidth = 30

type ChatRenderer struct{}

func (ChatRenderer) RenderOver(
	game string,
	width, height int,
	messages []world.ChatMessage,
	focused bool,
	input string,
) string {
	if len(messages) == 0 && !focused {
		return game
	}
	if len(messages) > chatMessageLimit {
		messages = messages[len(messages)-chatMessageLimit:]
	}
	rows := make([]string, 0, len(messages)+3)
	rows = append(rows, chatTitleStyle.Render("CHAT"))
	for _, message := range messages {
		text := fmt.Sprintf("[%s] %s", message.PlayerName, message.Message)
		rows = append(rows, chatMessageStyle.Render(truncateChatLine(text, chatTextWidth)))
	}
	if focused {
		rows = append(rows, chatInputStyle.Render("> "+input))
	}
	window := chatWindowStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	_, windowHeight := lipgloss.Size(window)
	x := 1
	y := max(height-windowHeight-3, 1)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(game).X(0).Y(0).Z(0),
		lipgloss.NewLayer(window).X(x).Y(y).Z(1),
	).Render()
}

func truncateChatLine(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

var (
	chatTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E2E8F0"))
	chatWindowStyle = lipgloss.NewStyle().
			Width(chatTextWidth).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#475569"))
	chatMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CBD5E1"))
	chatInputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#38BDF8"))
)
