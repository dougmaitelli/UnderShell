package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

type movementState struct {
	enhanced bool
	held     map[string]bool
	looping  bool
	inFlight bool
	facingX  int
	facingY  int
}

func newMovementState() movementState {
	return movementState{held: make(map[string]bool), facingX: 1}
}

func (s *movementState) stop() {
	clear(s.held)
	s.looping = false
}

func (s *movementState) setFacing(dx, dy int) {
	if dx != 0 || dy != 0 {
		s.facingX, s.facingY = dx, dy
	}
}

func (m *gameModel) handleMovementPress(key string) tea.Cmd {
	direction := directionKey(key)
	if direction == "" {
		return nil
	}
	m.movement.setFacing(movement(key))
	if !m.movement.enhanced {
		dx, dy := movement(key)
		if m.movement.inFlight {
			return nil
		}
		m.movement.inFlight = true
		return m.movePlayer(dx, dy)
	}

	m.movement.held[direction] = true
	commands := make([]tea.Cmd, 0, 2)
	if !m.movement.inFlight {
		dx, dy := heldMovement(m.movement.held)
		m.movement.inFlight = true
		commands = append(commands, m.movePlayer(dx, dy))
	}
	if !m.movement.looping {
		m.movement.looping = true
		commands = append(commands, movementTick())
	}
	return tea.Batch(commands...)
}

func (m *gameModel) handleMovementTick() tea.Cmd {
	if !m.movement.enhanced || len(m.movement.held) == 0 {
		m.movement.looping = false
		return nil
	}
	commands := []tea.Cmd{movementTick()}
	if !m.movement.inFlight {
		dx, dy := heldMovement(m.movement.held)
		if dx != 0 || dy != 0 {
			m.movement.inFlight = true
			commands = append(commands, m.movePlayer(dx, dy))
		}
	}
	return tea.Batch(commands...)
}

func movementTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return movementTickMsg{}
	})
}

func attackAnimationTick(frame int) tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg {
		return attackAnimationMsg{frame: frame}
	})
}

func movement(key string) (int, int) {
	switch strings.ToLower(key) {
	case "w", "up":
		return 0, -1
	case "s", "down":
		return 0, 1
	case "a", "left":
		return -1, 0
	case "d", "right":
		return 1, 0
	default:
		return 0, 0
	}
}

func directionKey(key string) string {
	switch strings.ToLower(key) {
	case "w", "up":
		return "up"
	case "s", "down":
		return "down"
	case "a", "left":
		return "left"
	case "d", "right":
		return "right"
	default:
		return ""
	}
}

func heldMovement(held map[string]bool) (int, int) {
	var dx, dy int
	if held["left"] {
		dx--
	}
	if held["right"] {
		dx++
	}
	if held["up"] {
		dy--
	}
	if held["down"] {
		dy++
	}
	return dx, dy
}
