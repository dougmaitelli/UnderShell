package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"sshrpg/src/world"
)

const chatMessageLimit = 10
const chatTextWidth = 30

type chatPanelState struct {
	input    textinput.Model
	messages []world.ChatMessage
}

func newChatPanelState() chatPanelState {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "Message"
	input.CharLimit = 200
	input.SetWidth(28)
	return chatPanelState{input: input}
}

func (s *chatPanelState) receive(message world.ChatMessage) {
	s.messages = append(s.messages, message)
	if len(s.messages) > chatMessageLimit {
		s.messages = s.messages[len(s.messages)-chatMessageLimit:]
	}
}

func (m *gameModel) updateChatInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		message := strings.TrimSpace(m.chat.input.Value())
		m.chat.input.SetValue("")
		m.chat.input.Blur()
		m.mode = inputModeGame
		if message == "" {
			return m, nil
		}
		return m, m.sendChat(message)
	case "esc":
		m.chat.input.SetValue("")
		m.chat.input.Blur()
		m.mode = inputModeGame
		return m, nil
	}
	var command tea.Cmd
	m.chat.input, command = m.chat.input.Update(msg)
	return m, command
}

type ChatRenderer struct{}

func (ChatRenderer) RenderOver(
	game string,
	width, height int,
	messages []world.ChatMessage,
	focused bool,
	input string,
	shimmerFrame ...int,
) string {
	if len(messages) == 0 && !focused {
		return game
	}
	if len(messages) > chatMessageLimit {
		messages = messages[len(messages)-chatMessageLimit:]
	}
	rows := make([]string, 0, len(messages)+3)
	rows = append(rows, chatTitleStyle.Render("CHAT"))
	frame := 0
	if len(shimmerFrame) > 0 {
		frame = shimmerFrame[0]
	}
	for _, message := range messages {
		rows = append(rows, renderChatMessage(message, frame))
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

func renderChatMessage(message world.ChatMessage, shimmerFrame int) string {
	text := truncateChatLine(
		fmt.Sprintf("[%s] %s", message.PlayerName, message.Message),
		chatTextWidth,
	)
	runes := []rune(text)
	if len(runes) < 2 {
		return chatMessageStyle.Render(text)
	}
	nameLength := min(len([]rune(message.PlayerName)), len(runes)-1)
	var rendered strings.Builder
	rendered.WriteString(chatMessageStyle.Render(string(runes[:1])))
	rendered.WriteString(renderPlayerName(
		string(runes[1:1+nameLength]),
		message.PlayerRole,
		shimmerFrame,
	))
	if 1+nameLength < len(runes) {
		rendered.WriteString(
			chatMessageStyle.Render(string(runes[1+nameLength:])),
		)
	}
	return rendered.String()
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
