// Package world manages live multiplayer state.
package world

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"sshrpg/src/domain"
	"sshrpg/src/enemy"
	"sshrpg/src/item"
	"sshrpg/src/npc"
	"sshrpg/src/quest"
)

// Manager is the public facade for the serialized world runtime.
type Manager struct {
	areas     *Areas
	items     *item.Items
	enemies   *enemy.Enemies
	quests    *quest.Quests
	events    chan any
	done      chan struct{}
	closeOnce sync.Once
}

var ErrWorldClosed = errors.New("world is closed")

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

func (m *Manager) Close() { m.closeOnce.Do(func() { close(m.done) }) }

func request[T any](m *Manager, build func(chan T) any) (T, bool) {
	select {
	case <-m.done:
		var zero T
		return zero, false
	default:
	}
	reply := make(chan T, 1)
	select {
	case m.events <- build(reply):
	case <-m.done:
		var zero T
		return zero, false
	}
	select {
	case result := <-reply:
		return result, true
	case <-m.done:
		var zero T
		return zero, false
	}
}

func (m *Manager) Join(player Player) Session {
	result, _ := request(m, func(reply chan Session) any {
		return joinRequest{player: player, reply: reply}
	})
	return result
}

func (m *Manager) Move(id int64, token string, dx, dy int) Player {
	result, _ := request(m, func(reply chan Player) any {
		return moveRequest{id: id, token: token, dx: dx, dy: dy, reply: reply}
	})
	return result
}

func (m *Manager) Attack(id int64, token string) AttackResult {
	result, _ := request(m, func(reply chan AttackResult) any {
		return attackRequest{id: id, token: token, reply: reply}
	})
	return result
}

func (m *Manager) Pickup(id int64, token string) PickupResult {
	result, _ := request(m, func(reply chan PickupResult) any {
		return pickupRequest{id: id, token: token, reply: reply}
	})
	return result
}

// RestorePickup returns a claimed item to the world when inventory persistence
// fails, preventing a transient database error from destroying loot.
func (m *Manager) RestorePickup(id int64, token string, item GroundItem) bool {
	result, _ := request(m, func(reply chan bool) any {
		return restorePickupRequest{id: id, token: token, item: item, reply: reply}
	})
	return result
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
	result, _ := request(m, func(reply chan ConsumableResult) any {
		return useConsumableRequest{id: id, token: token, definition: definition, reply: reply}
	})
	return result
}

func (m *Manager) UpdateEquipment(
	id int64,
	token string,
	stats item.EquipmentStats,
) Player {
	result, _ := request(m, func(reply chan Player) any {
		return updateEquipmentRequest{id: id, token: token, stats: stats, reply: reply}
	})
	return result
}

func (m *Manager) SpendSkillPoint(id int64, token, skill string) Player {
	result, _ := request(m, func(reply chan Player) any {
		return spendSkillRequest{id: id, token: token, skill: skill, reply: reply}
	})
	return result
}

func (m *Manager) Chat(id int64, token, message string) bool {
	result, _ := request(m, func(reply chan bool) any {
		return chatRequest{id: id, token: token, message: message, reply: reply}
	})
	return result
}

func (m *Manager) ServerMessage(message string) bool {
	result, _ := request(m, func(reply chan bool) any {
		return serverChatRequest{message: message, reply: reply}
	})
	return result
}

func (m *Manager) AuthenticatedRole(
	id int64,
	token string,
) (domain.CharacterRole, bool) {
	result, open := request(m, func(reply chan adminAuthorizeResult) any {
		return adminAuthorizeRequest{id: id, token: token, reply: reply}
	})
	if !open {
		return "", false
	}
	return result.role, result.ok
}

func (m *Manager) FindOnlinePlayer(name string) (Player, error) {
	result, open := request(m, func(reply chan adminPlayerResult) any {
		return adminFindPlayerRequest{name: name, reply: reply}
	})
	if !open {
		return Player{}, ErrWorldClosed
	}
	return result.player, result.err
}

func (m *Manager) GrantExperience(name string, amount int64) (Player, error) {
	result, open := request(m, func(reply chan adminPlayerResult) any {
		return adminGrantExperienceRequest{name: name, amount: amount, reply: reply}
	})
	if !open {
		return Player{}, ErrWorldClosed
	}
	return result.player, result.err
}

func (m *Manager) GrantLevels(name string, amount int) (Player, error) {
	result, open := request(m, func(reply chan adminPlayerResult) any {
		return adminGrantLevelsRequest{name: name, amount: amount, reply: reply}
	})
	if !open {
		return Player{}, ErrWorldClosed
	}
	return result.player, result.err
}

func (m *Manager) TeleportToArea(name, area string) (Player, error) {
	result, open := request(m, func(reply chan adminPlayerResult) any {
		return adminTeleportAreaRequest{name: name, area: area, reply: reply}
	})
	if !open {
		return Player{}, ErrWorldClosed
	}
	return result.player, result.err
}

func (m *Manager) TeleportToPlayer(name, destination string) (Player, error) {
	result, open := request(m, func(reply chan adminPlayerResult) any {
		return adminTeleportPlayerRequest{name: name, destination: destination, reply: reply}
	})
	if !open {
		return Player{}, ErrWorldClosed
	}
	return result.player, result.err
}

func (m *Manager) NotifyPlayer(id int64, message string, inventoryChanged bool) bool {
	result, _ := request(m, func(reply chan bool) any {
		return adminNotifyRequest{id: id, message: message, inventoryChanged: inventoryChanged, reply: reply}
	})
	return result
}

func (m *Manager) SetPlayerRole(
	name string,
	role domain.CharacterRole,
) (Player, error) {
	result, open := request(m, func(reply chan adminPlayerResult) any {
		return adminSetRoleRequest{name: name, role: role, reply: reply}
	})
	if !open {
		return Player{}, ErrWorldClosed
	}
	return result.player, result.err
}

func (m *Manager) KickPlayer(name, reason string) (Player, error) {
	result, open := request(m, func(reply chan adminPlayerResult) any {
		return adminKickRequest{name: name, reason: reason, reply: reply}
	})
	if !open {
		return Player{}, ErrWorldClosed
	}
	return result.player, result.err
}

func (m *Manager) Leave(id int64, token string) {
	select {
	case m.events <- leaveRequest{id: id, token: token}:
	case <-m.done:
	}
}

// DefeatEnemy removes a live enemy. Its owning spawn begins its respawn timer.
func (m *Manager) DefeatEnemy(id uint64) bool {
	result, _ := request(m, func(reply chan bool) any {
		return defeatEnemyRequest{id: id, reply: reply}
	})
	return result
}

func (m *Manager) run() {
	newRuntimeState(m).run(m.events, m.done)
}

func newToken() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	return hex.EncodeToString(value[:])
}
