package ui

import (
	"charm.land/lipgloss/v2"

	"sshrpg/src/domain"
	"sshrpg/src/world"
)

type ViewState struct {
	Phase         phase
	Width         int
	Height        int
	Input         string
	Message       string
	Creating      bool
	Character     *domain.Character
	Snapshot      world.Snapshot
	InventoryOpen bool
	Inventory     *domain.Inventory
	AttackFrame   int
	FacingX       int
	FacingY       int
}

type Renderer struct {
	welcome   WelcomeRenderer
	game      GameRenderer
	inventory InventoryRenderer
}

func NewRenderer() Renderer {
	return Renderer{
		welcome:   WelcomeRenderer{},
		game:      GameRenderer{},
		inventory: InventoryRenderer{},
	}
}

func (r Renderer) Render(state ViewState) string {
	switch state.Phase {
	case phaseOnboarding:
		return r.welcome.Render(state)
	case phaseJoining:
		return lipgloss.Place(
			state.Width,
			state.Height,
			lipgloss.Center,
			lipgloss.Center,
			mutedStyle.Render("Entering the realm…"),
		)
	case phasePlaying:
		game := r.game.Render(state)
		if state.InventoryOpen {
			return r.inventory.RenderOver(game, state.Width, state.Height, state.Inventory)
		}
		return game
	default:
		return ""
	}
}

var (
	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8"))
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FB7185"))
)
