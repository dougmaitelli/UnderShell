package world

import (
	"fmt"

	"sshrpg/src/npc"
)

func (a *Area) NPCAt(point Point) (*npc.Definition, bool) {
	for index := range a.NPCs {
		definition := &a.NPCs[index]
		if definition.X == point.X && definition.Y == point.Y {
			return definition, true
		}
	}
	return nil, false
}

func validateNPCPlacements(area *Area) error {
	positions := make(map[Point]bool, len(area.NPCs))
	for index := range area.NPCs {
		definition := &area.NPCs[index]
		position := Point{X: definition.X, Y: definition.Y}
		if !area.Walkable(position) {
			return fmt.Errorf("NPC %q must be on a walkable tile", definition.ID)
		}
		if positions[position] {
			return fmt.Errorf("multiple NPCs at (%d,%d)", definition.X, definition.Y)
		}
		if position == area.Spawn {
			return fmt.Errorf("NPC %q cannot occupy the area spawn", definition.ID)
		}
		if _, waypoint := area.Waypoint(position); waypoint {
			return fmt.Errorf("NPC %q cannot occupy a waypoint", definition.ID)
		}
		positions[position] = true
	}
	return nil
}
