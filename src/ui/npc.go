package ui

import npcconfig "sshrpg/src/npc"

const npcInteractionRange = 2

func (m *gameModel) nearbyNPC() *npcconfig.Definition {
	if m.character == nil || m.connection.snapshot.Area == nil {
		return nil
	}
	area := m.connection.snapshot.Area
	if area.ID != m.character.AreaID {
		return nil
	}
	var nearby *npcconfig.Definition
	nearbyDistance := npcInteractionRange + 1
	for index := range area.NPCs {
		definition := &area.NPCs[index]
		distance := max(
			absolute(definition.X-m.character.X),
			absolute(definition.Y-m.character.Y),
		)
		if distance <= npcInteractionRange && distance < nearbyDistance {
			nearby = definition
			nearbyDistance = distance
		}
	}
	return nearby
}

func absolute(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
