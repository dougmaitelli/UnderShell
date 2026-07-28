package ui

import (
	tea "charm.land/bubbletea/v2"

	"sshrpg/src/npc"
)

type actionState struct {
	attackInFlight  bool
	pickupInFlight  bool
	attackFrame     int
	attackDirection int
}

func (s *actionState) beginAttack(direction int) bool {
	if s.attackInFlight {
		return false
	}
	if direction < 0 {
		s.attackDirection = -1
	} else {
		s.attackDirection = 1
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
		if m.actions.beginAttack(m.movement.horizontalFacing) {
			return m, tea.Batch(m.attack(), attackAnimationTick(2))
		}
	case "e":
		if definition := m.nearbyNPC(); definition != nil {
			switch definition.Type {
			case npc.TypeShop:
				return m.openShop(definition)
			case npc.TypeQuestGiver:
				return m.interactQuestGiver(definition)
			}
		}
		if m.actions.beginPickup() {
			return m, m.pickup()
		}
	}
	return m, nil
}
