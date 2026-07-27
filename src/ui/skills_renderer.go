package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"sshrpg/src/domain"
)

type SkillsRenderer struct{}

func (SkillsRenderer) RenderOver(
	game string,
	width, height int,
	character *domain.Character,
) string {
	level, points, attack, defense, vitality := 1, 0, 0, 0, 0
	if character != nil {
		level = character.Level
		points = character.SkillPoints
		attack, defense, vitality = character.Attack, character.Defense, character.Vitality
	}
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		skillsTitleStyle.Render("SKILLS"),
		"",
		fmt.Sprintf("Level: %d", level),
		fmt.Sprintf("Unspent points: %d", points),
		"",
		fmt.Sprintf("[1] Attack   %d  (+1 damage)", attack),
		fmt.Sprintf("[2] Defense  %d  (-1 damage taken)", defense),
		fmt.Sprintf("[3] Vitality %d  (+5 maximum health)", vitality),
		"",
		mutedStyle.Render("1–3 to spend • K or Esc to close"),
	)
	window := skillsWindowStyle.Render(body)
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
	skillsTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#A78BFA"))
	skillsWindowStyle = lipgloss.NewStyle().
				Width(42).
				Padding(1, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#64748B"))
)
