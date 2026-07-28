package ui

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	"sshrpg/src/repository"
	"sshrpg/src/world"
)

func (m *gameModel) createCharacter(name string) tea.Cmd {
	return func() tea.Msg {
		char, err := m.repositories.Characters.Create(
			context.Background(),
			repository.CreateCharacterParams{
				KeyFingerprint: m.identity.Fingerprint,
				PublicKeyType:  m.identity.KeyType,
				PublicKey:      m.identity.PublicKey,
				Name:           name,
			},
		)
		if err != nil {
			return characterCreatedMsg{err: err}
		}
		inventory, err := m.repositories.Inventories.FindOrCreate(context.Background(), char.ID)
		return characterCreatedMsg{character: char, inventory: inventory, err: err}
	}
}

func (m *gameModel) joinWorld() tea.Cmd {
	return func() tea.Msg {
		session := m.world.Join(world.Player{
			ID: m.character.ID, Name: m.character.Name, Role: m.character.Role,
			AreaID: m.character.AreaID, X: m.character.X, Y: m.character.Y,
			Level: m.character.Level, Experience: m.character.Experience,
			SkillPoints: m.character.SkillPoints,
			Attack:      m.character.Attack, Defense: m.character.Defense,
			Vitality:       m.character.Vitality,
			EquipmentStats: m.equipmentStats(m.inventory),
		})
		return worldJoinedMsg{session: session}
	}
}

func (m *gameModel) movePlayer(dx, dy int) tea.Cmd {
	return func() tea.Msg {
		return playerMovedMsg{player: m.world.Move(
			m.character.ID, m.connection.session.Token, dx, dy,
		)}
	}
}

func (m *gameModel) attack() tea.Cmd {
	return func() tea.Msg {
		return attackResultMsg{result: m.world.Attack(m.character.ID, m.connection.session.Token)}
	}
}

func (m *gameModel) pickup() tea.Cmd {
	return func() tea.Msg {
		return pickupResultMsg{result: m.world.Pickup(m.character.ID, m.connection.session.Token)}
	}
}

func (m *gameModel) spendSkill(key string) tea.Cmd {
	skills := map[string]string{"1": "attack", "2": "defense", "3": "vitality"}
	skill := skills[key]
	return func() tea.Msg {
		return skillSpentMsg{player: m.world.SpendSkillPoint(
			m.character.ID, m.connection.session.Token, skill,
		)}
	}
}

func (m *gameModel) sendChat(message string) tea.Cmd {
	return func() tea.Msg {
		return chatSentMsg{ok: m.world.Chat(
			m.character.ID, m.connection.session.Token, message,
		)}
	}
}

func (m *gameModel) storePickup(drop world.GroundItem) tea.Cmd {
	return func() tea.Msg {
		if drop.Item == nil {
			return itemStoredMsg{
				itemName: "",
				err:      errors.New("unknown picked up item"),
			}
		}
		inventory, err := m.repositories.Inventories.AddItem(
			context.Background(), m.character.ID, drop.Item.ID, drop.Item.MaxStack,
		)
		return itemStoredMsg{
			inventory: inventory, itemName: drop.Item.Name, err: err,
		}
	}
}

func (m *gameModel) savePosition() tea.Cmd {
	id, areaID, x, y := m.character.ID, m.character.AreaID, m.character.X, m.character.Y
	return func() tea.Msg {
		return positionSavedMsg{err: m.repositories.Characters.UpdateLocation(
			context.Background(), id, areaID, x, y,
		)}
	}
}

func (m *gameModel) saveProgress() tea.Cmd {
	id := m.character.ID
	level, experience, skillPoints := m.character.Level, m.character.Experience, m.character.SkillPoints
	attack, defense, vitality := m.character.Attack, m.character.Defense, m.character.Vitality
	return func() tea.Msg {
		return progressSavedMsg{err: m.repositories.Characters.UpdateProgress(
			context.Background(), id, level, experience, skillPoints,
			attack, defense, vitality,
		)}
	}
}

func waitForSnapshot(updates <-chan world.Snapshot) tea.Cmd {
	return func() tea.Msg {
		snapshot, ok := <-updates
		return worldSnapshotMsg{snapshot: snapshot, ok: ok}
	}
}

func waitForKick(kicked <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-kicked
		return worldKickedMsg{}
	}
}

func waitForWorldEvent(events <-chan world.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		return worldEventMsg{event: event, ok: ok}
	}
}

func waitForChatMessage(messages <-chan world.ChatMessage) tea.Cmd {
	return func() tea.Msg {
		message, ok := <-messages
		return chatMessageMsg{message: message, ok: ok}
	}
}

func (m *gameModel) leaveWorld() {
	if m.connection.joined {
		m.world.Leave(m.character.ID, m.connection.session.Token)
		m.connection.joined = false
	}
}
