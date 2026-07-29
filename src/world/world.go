// Package world manages live multiplayer state.
package world

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"sshrpg/src/domain"
	"sshrpg/src/enemy"
	"sshrpg/src/item"
	"sshrpg/src/npc"
	"sshrpg/src/quest"
)

// Manager is the public facade for the serialized world runtime.
type Manager struct {
	areas   *Areas
	items   *item.Items
	enemies *enemy.Enemies
	quests  *quest.Quests
	events  chan any
	done    chan struct{}
}

func New(
	areas *Areas,
	items *item.Items,
	enemies *enemy.Enemies,
	quests *quest.Quests,
) *Manager {
	m := &Manager{
		areas: areas, items: items, enemies: enemies, quests: quests,
		events: make(chan any), done: make(chan struct{}),
	}
	go m.run()
	return m
}

func (m *Manager) Items() *item.Items { return m.items }

func (m *Manager) Enemies() *enemy.Enemies { return m.enemies }

func (m *Manager) Quests() *quest.Quests { return m.quests }

func (m *Manager) NPC(id string) (*npc.Definition, *Area, bool) {
	return m.areas.NPC(id)
}

func (m *Manager) Close() { close(m.done) }

func (m *Manager) Join(player Player) Session {
	reply := make(chan Session)
	m.events <- joinRequest{player: player, reply: reply}
	return <-reply
}

func (m *Manager) Move(id int64, token string, dx, dy int) Player {
	reply := make(chan Player)
	m.events <- moveRequest{id: id, token: token, dx: dx, dy: dy, reply: reply}
	return <-reply
}

func (m *Manager) Attack(id int64, token string) AttackResult {
	reply := make(chan AttackResult)
	select {
	case m.events <- attackRequest{id: id, token: token, reply: reply}:
		return <-reply
	case <-m.done:
		return AttackResult{}
	}
}

func (m *Manager) Pickup(id int64, token string) PickupResult {
	reply := make(chan PickupResult)
	select {
	case m.events <- pickupRequest{id: id, token: token, reply: reply}:
		return <-reply
	case <-m.done:
		return PickupResult{}
	}
}

func (m *Manager) UseConsumable(
	id int64,
	token string,
	itemID string,
) ConsumableResult {
	if m.items == nil {
		return ConsumableResult{}
	}
	definition, ok := m.items.Item(itemID)
	if !ok || definition.Type != item.TypeConsumable {
		return ConsumableResult{}
	}
	reply := make(chan ConsumableResult)
	select {
	case m.events <- useConsumableRequest{
		id: id, token: token, definition: definition, reply: reply,
	}:
		return <-reply
	case <-m.done:
		return ConsumableResult{}
	}
}

func (m *Manager) UpdateEquipment(
	id int64,
	token string,
	stats item.EquipmentStats,
) Player {
	reply := make(chan Player)
	select {
	case m.events <- updateEquipmentRequest{
		id: id, token: token, stats: stats, reply: reply,
	}:
		return <-reply
	case <-m.done:
		return Player{}
	}
}

func (m *Manager) SpendSkillPoint(id int64, token, skill string) Player {
	reply := make(chan Player)
	select {
	case m.events <- spendSkillRequest{id: id, token: token, skill: skill, reply: reply}:
		return <-reply
	case <-m.done:
		return Player{}
	}
}

func (m *Manager) Chat(id int64, token, message string) bool {
	reply := make(chan bool)
	select {
	case m.events <- chatRequest{id: id, token: token, message: message, reply: reply}:
		return <-reply
	case <-m.done:
		return false
	}
}

func (m *Manager) ServerMessage(message string) bool {
	reply := make(chan bool)
	select {
	case m.events <- serverChatRequest{message: message, reply: reply}:
		return <-reply
	case <-m.done:
		return false
	}
}

func (m *Manager) AuthenticatedRole(
	id int64,
	token string,
) (domain.CharacterRole, bool) {
	reply := make(chan adminAuthorizeResult)
	select {
	case m.events <- adminAuthorizeRequest{id: id, token: token, reply: reply}:
		result := <-reply
		return result.role, result.ok
	case <-m.done:
		return "", false
	}
}

func (m *Manager) FindOnlinePlayer(name string) (Player, error) {
	reply := make(chan adminPlayerResult)
	select {
	case m.events <- adminFindPlayerRequest{name: name, reply: reply}:
		result := <-reply
		return result.player, result.err
	case <-m.done:
		return Player{}, errors.New("world is closed")
	}
}

func (m *Manager) GrantExperience(name string, amount int64) (Player, error) {
	reply := make(chan adminPlayerResult)
	select {
	case m.events <- adminGrantExperienceRequest{name: name, amount: amount, reply: reply}:
		result := <-reply
		return result.player, result.err
	case <-m.done:
		return Player{}, errors.New("world is closed")
	}
}

func (m *Manager) GrantLevels(name string, amount int) (Player, error) {
	reply := make(chan adminPlayerResult)
	select {
	case m.events <- adminGrantLevelsRequest{name: name, amount: amount, reply: reply}:
		result := <-reply
		return result.player, result.err
	case <-m.done:
		return Player{}, errors.New("world is closed")
	}
}

func (m *Manager) TeleportToArea(name, area string) (Player, error) {
	reply := make(chan adminPlayerResult)
	select {
	case m.events <- adminTeleportAreaRequest{name: name, area: area, reply: reply}:
		result := <-reply
		return result.player, result.err
	case <-m.done:
		return Player{}, errors.New("world is closed")
	}
}

func (m *Manager) TeleportToPlayer(name, destination string) (Player, error) {
	reply := make(chan adminPlayerResult)
	select {
	case m.events <- adminTeleportPlayerRequest{
		name: name, destination: destination, reply: reply,
	}:
		result := <-reply
		return result.player, result.err
	case <-m.done:
		return Player{}, errors.New("world is closed")
	}
}

func (m *Manager) NotifyPlayer(id int64, message string, inventoryChanged bool) bool {
	reply := make(chan bool)
	select {
	case m.events <- adminNotifyRequest{
		id: id, message: message, inventoryChanged: inventoryChanged, reply: reply,
	}:
		return <-reply
	case <-m.done:
		return false
	}
}

func (m *Manager) SetPlayerRole(
	name string,
	role domain.CharacterRole,
) (Player, error) {
	reply := make(chan adminPlayerResult)
	select {
	case m.events <- adminSetRoleRequest{name: name, role: role, reply: reply}:
		result := <-reply
		return result.player, result.err
	case <-m.done:
		return Player{}, errors.New("world is closed")
	}
}

func (m *Manager) KickPlayer(name, reason string) (Player, error) {
	reply := make(chan adminPlayerResult)
	select {
	case m.events <- adminKickRequest{name: name, reason: reason, reply: reply}:
		result := <-reply
		return result.player, result.err
	case <-m.done:
		return Player{}, errors.New("world is closed")
	}
}

func (m *Manager) Leave(id int64, token string) {
	select {
	case m.events <- leaveRequest{id: id, token: token}:
	case <-m.done:
	}
}

// DefeatEnemy removes a live enemy. Its owning spawn begins its respawn timer.
func (m *Manager) DefeatEnemy(id uint64) bool {
	reply := make(chan bool)
	select {
	case m.events <- defeatEnemyRequest{id: id, reply: reply}:
		return <-reply
	case <-m.done:
		return false
	}
}

func (m *Manager) run() {
	newRuntimeState(m).run(m.events, m.done)
}

func newToken() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	return hex.EncodeToString(value[:])
}
