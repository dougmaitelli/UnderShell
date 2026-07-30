package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"sshrpg/src/domain"
	"sshrpg/src/world"
)

const playerNameShimmerInterval = 140 * time.Millisecond

type playerNameShimmerState struct {
	active     bool
	frame      int
	generation uint64
}

type playerNameShimmerMsg struct {
	generation uint64
}

func (s *playerNameShimmerState) setNeeded(
	players []world.Player,
	messages []world.ChatMessage,
) tea.Cmd {
	needed := false
	for _, player := range players {
		if hasPlayerNameShimmer(player.Role) {
			needed = true
			break
		}
	}
	if !needed {
		for _, message := range messages {
			if hasPlayerNameShimmer(message.PlayerRole) {
				needed = true
				break
			}
		}
	}
	if needed == s.active {
		return nil
	}
	s.active = needed
	s.generation++
	if !needed {
		s.frame = 0
		return nil
	}
	return playerNameShimmerTick(s.generation)
}

func renderPlayerName(
	name string,
	role domain.CharacterRole,
	frame int,
) string {
	var rendered strings.Builder
	for index, character := range []rune(name) {
		rendered.WriteString(
			playerNameStyle(role, frame, index).Render(string(character)),
		)
	}
	return rendered.String()
}

func (s *playerNameShimmerState) advance(generation uint64) tea.Cmd {
	if !s.active || generation != s.generation {
		return nil
	}
	s.frame = (s.frame + 1) % len(adminPlayerNameStyles)
	return playerNameShimmerTick(s.generation)
}

func playerNameShimmerTick(generation uint64) tea.Cmd {
	return tea.Tick(playerNameShimmerInterval, func(time.Time) tea.Msg {
		return playerNameShimmerMsg{generation: generation}
	})
}

func playerNameStyle(
	role domain.CharacterRole,
	frame, characterIndex int,
) lipgloss.Style {
	styles := playerNameShimmerStyles(role)
	if len(styles) == 0 {
		return userPlayerNameStyle
	}
	index := (frame + characterIndex) % len(styles)
	return styles[index]
}

func hasPlayerNameShimmer(role domain.CharacterRole) bool {
	return len(playerNameShimmerStyles(role)) > 0
}

func playerNameShimmerStyles(role domain.CharacterRole) []lipgloss.Style {
	switch role {
	case domain.CharacterRoleAdmin:
		return adminPlayerNameStyles
	case domain.CharacterRoleModerator:
		return moderatorPlayerNameStyles
	default:
		return nil
	}
}

var (
	userPlayerNameStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#38BDF8"))
	adminPlayerNameStyles = []lipgloss.Style{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B91C1C")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B91C1C")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B91C1C")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B91C1C")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF3333")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#DC2626")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B91C1C")),
	}
	moderatorPlayerNameStyles = []lipgloss.Style{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A16207")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A16207")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A16207")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A16207")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FACC15")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EAB308")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#CA8A04")),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A16207")),
	}
)
