package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

type inputMode uint8

const (
	inputModeGame inputMode = iota
	inputModeInventory
	inputModeSkills
	inputModeChat
	inputModeHelp
	inputModeShop
)

func (m *gameModel) updateKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.phase == phaseOnboarding {
		return m.updateOnboardingInput(msg)
	}
	if m.phase != phasePlaying {
		return m, nil
	}
	switch m.mode {
	case inputModeInventory:
		return m.updateInventoryInput(msg)
	case inputModeSkills:
		return m.updateSkillsInput(msg)
	case inputModeChat:
		return m.updateChatInput(msg)
	case inputModeHelp:
		return m.updateHelpInput(msg)
	case inputModeShop:
		return m.updateShopInput(msg)
	default:
		return m.updateGameplayInput(msg)
	}
}

func (m *gameModel) updateGameplayInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "i":
		return m.openInputMode(inputModeInventory)
	case "k":
		return m.openInputMode(inputModeSkills)
	case "t":
		m.mode = inputModeChat
		m.movement.stop()
		return m, m.chat.input.Focus()
	case "f1":
		return m.openInputMode(inputModeHelp)
	case "x", "e":
		return m.updateActionInput(msg)
	default:
		if m.movement.enhanced && msg.IsRepeat &&
			directionKey(msg.String()) != "" {
			return m, nil
		}
		return m, m.handleMovementPress(msg.String())
	}
}

func (m *gameModel) openInputMode(mode inputMode) (tea.Model, tea.Cmd) {
	m.mode = mode
	m.movement.stop()
	return m, nil
}
