package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

type movementState struct {
	enhanced         bool
	held             map[string]bool
	looping          bool
	inFlight         bool
	facingX          int
	facingY          int
	horizontalFacing int
	walkFrame        int
	walkSteps        uint64
	walkGeneration   uint64
	nextMove         time.Time
}

const (
	horizontalMovementInterval = 75 * time.Millisecond
	verticalMovementInterval   = 100 * time.Millisecond
)

func newMovementState() movementState {
	return movementState{
		held: make(map[string]bool), facingX: 1, horizontalFacing: 1,
	}
}

func (s *movementState) stop() {
	clear(s.held)
	s.looping = false
	s.walkFrame = 0
	s.walkGeneration++
}

func (s *movementState) setFacing(dx, dy int) {
	if dx != 0 || dy != 0 {
		s.facingX, s.facingY = dx, dy
	}
	if dx != 0 {
		s.horizontalFacing = dx
	}
}

func (s *movementState) step() tea.Cmd {
	s.walkSteps++
	s.walkFrame = int((s.walkSteps-1)/2%2) + 1
	s.walkGeneration++
	generation := s.walkGeneration
	return tea.Tick(240*time.Millisecond, func(time.Time) tea.Msg {
		return walkAnimationDoneMsg{generation: generation}
	})
}

func (s *movementState) finishStep(generation uint64) {
	if generation == s.walkGeneration {
		s.walkFrame = 0
	}
}

func (s *movementState) beginMove(now time.Time, dx, dy int) bool {
	if (dx == 0 && dy == 0) || s.inFlight || now.Before(s.nextMove) {
		return false
	}
	s.inFlight = true
	s.nextMove = now.Add(movementInterval(dx, dy))
	return true
}

func movementInterval(_, dy int) time.Duration {
	if dy != 0 {
		return verticalMovementInterval
	}
	return horizontalMovementInterval
}

func (m *gameModel) handleMovementPress(key string) tea.Cmd {
	direction := directionKey(key)
	if direction == "" {
		return nil
	}
	m.movement.setFacing(movement(key))
	if !m.movement.enhanced {
		dx, dy := movement(key)
		if !m.movement.beginMove(time.Now(), dx, dy) {
			m.skipRender = true
			return nil
		}
		m.skipRender = true
		return m.movePlayer(dx, dy)
	}

	m.movement.held[direction] = true
	commands := make([]tea.Cmd, 0, 2)
	dx, dy := heldMovement(m.movement.held)
	if m.movement.beginMove(time.Now(), dx, dy) {
		m.skipRender = true
		commands = append(commands, m.movePlayer(dx, dy))
	}
	if !m.movement.looping {
		m.movement.looping = true
		commands = append(commands, movementTick(movementInterval(dx, dy)))
	}
	return tea.Batch(commands...)
}

func (m *gameModel) handleMovementTick() tea.Cmd {
	if !m.movement.enhanced || len(m.movement.held) == 0 {
		m.movement.looping = false
		return nil
	}
	dx, dy := heldMovement(m.movement.held)
	commands := []tea.Cmd{movementTick(movementInterval(dx, dy))}
	if m.movement.beginMove(time.Now(), dx, dy) {
		if dx != 0 || dy != 0 {
			m.skipRender = true
			commands = append(commands, m.movePlayer(dx, dy))
		} else {
			m.movement.inFlight = false
		}
	}
	if len(commands) == 1 {
		m.skipRender = true
	}
	return tea.Batch(commands...)
}

func movementTick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg {
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
