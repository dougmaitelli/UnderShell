package ui

import (
	tea "charm.land/bubbletea/v2"

	"sshrpg/src/npc"
	"sshrpg/src/world"
)

type worldConnection struct {
	session  world.Session
	joined   bool
	snapshot world.Snapshot
}

func (m *gameModel) updateWorldJoined(msg worldJoinedMsg) (tea.Model, tea.Cmd) {
	m.connection.session = msg.session
	m.connection.joined = true
	m.phase = phasePlaying
	return m, tea.Batch(
		waitForSnapshot(msg.session.Updates),
		waitForWorldEvent(msg.session.Events),
		waitForChatMessage(msg.session.Chats),
		waitForKick(msg.session.Kicked),
	)
}

func (m *gameModel) updateWorldEvent(msg worldEventMsg) (tea.Model, tea.Cmd) {
	if !msg.ok {
		return m, nil
	}
	expiry := m.addEvent(EventView{Kind: msg.event.Kind, Message: msg.event.Message})
	return m, tea.Batch(expiry, waitForWorldEvent(m.connection.session.Events))
}

func (m *gameModel) updateChatMessage(msg chatMessageMsg) (tea.Model, tea.Cmd) {
	if !msg.ok {
		return m, nil
	}
	m.chat.receive(msg.message)
	return m, tea.Batch(
		m.nameShimmer.setNeeded(
			m.connection.snapshot.Players, m.chat.messages,
		),
		waitForChatMessage(m.connection.session.Chats),
	)
}

func (m *gameModel) updateWorldSnapshot(msg worldSnapshotMsg) (tea.Model, tea.Cmd) {
	if !msg.ok {
		return m, tea.Quit
	}
	m.connection.snapshot = msg.snapshot
	commands := []tea.Cmd{waitForSnapshot(m.connection.session.Updates)}
	if shimmer := m.nameShimmer.setNeeded(
		msg.snapshot.Players, m.chat.messages,
	); shimmer != nil {
		commands = append(commands, shimmer)
	}
	for _, player := range msg.snapshot.Players {
		if player.ID != m.character.ID {
			continue
		}
		locationChanged := m.character.AreaID != player.AreaID ||
			m.character.X != player.X ||
			m.character.Y != player.Y
		progressChanged := m.character.Level != player.Level ||
			m.character.Experience != player.Experience ||
			m.character.SkillPoints != player.SkillPoints ||
			m.character.Attack != player.Attack ||
			m.character.Defense != player.Defense ||
			m.character.Vitality != player.Vitality
		m.character.AreaID = player.AreaID
		m.character.X, m.character.Y = player.X, player.Y
		m.character.Level = player.Level
		m.character.Experience = player.Experience
		m.character.SkillPoints = player.SkillPoints
		m.character.Attack = player.Attack
		m.character.Defense = player.Defense
		m.character.Vitality = player.Vitality
		if locationChanged {
			commands = append(commands, m.savePosition())
		}
		if progressChanged {
			commands = append(commands, m.saveProgress())
		}
		break
	}
	if m.mode == inputModeShop {
		nearby := m.nearbyNPC()
		if nearby == nil || nearby.Type != npc.TypeShop ||
			m.shop.npc == nil || nearby.ID != m.shop.npc.ID {
			m.shop.close()
			m.mode = inputModeGame
		}
	}
	if m.mode == inputModeQuestDialogue {
		nearby := m.nearbyNPC()
		if nearby == nil || nearby.Type != npc.TypeQuestGiver ||
			m.quests.dialogue.giver == nil ||
			nearby.ID != m.quests.dialogue.giver.ID {
			m.quests.closeDialogue()
			m.mode = inputModeGame
		}
	}
	return m, tea.Batch(commands...)
}
