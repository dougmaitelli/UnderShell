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
		if player.Role == domain.CharacterRoleAdmin {
			needed = true
			break
		}
	}
	if !needed {
		for _, message := range messages {
			if message.PlayerRole == domain.CharacterRoleAdmin {
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
	if role != domain.CharacterRoleAdmin {
		return userPlayerNameStyle
	}
	index := (frame + characterIndex) % len(adminPlayerNameStyles)
	return adminPlayerNameStyles[index]
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
)
