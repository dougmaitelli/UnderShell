package ui

import (
	tea "charm.land/bubbletea/v2"
)

type actionState struct {
	attackInFlight bool
	pickupInFlight bool
	attackFrame    int
}

func (s *actionState) beginAttack() bool {
	if s.attackInFlight {
		return false
	}
	s.attackInFlight = true
	s.attackFrame = 1
	return true
}

func (s *actionState) advanceAttack(frame int) bool {
	if frame > 2 {
		s.attackFrame = 0
		s.attackInFlight = false
		return false
	}
	s.attackFrame = frame
	return true
}

func (s *actionState) beginPickup() bool {
	if s.pickupInFlight {
		return false
	}
	s.pickupInFlight = true
	return true
}

func (s *actionState) finishPickup() {
	s.pickupInFlight = false
}

func (m *gameModel) updateActionInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "x":
		if m.actions.beginAttack() {
			return m, tea.Batch(m.attack(), attackAnimationTick(2))
		}
	case "e":
		if shop := m.nearbyShop(); shop != nil {
			return m.openShop(shop)
		}
		if m.actions.beginPickup() {
			return m, m.pickup()
		}
	}
	return m, nil
}
