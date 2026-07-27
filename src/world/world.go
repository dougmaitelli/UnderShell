// Package world manages live multiplayer state.
package world

import (
	"crypto/rand"
	"encoding/hex"

	"sshrpg/src/enemy"
	"sshrpg/src/item"
)

// Manager is the public facade for the serialized world runtime.
type Manager struct {
	areas   *Areas
	items   *item.Items
	enemies *enemy.Enemies
	events  chan any
	done    chan struct{}
}

func New(areas *Areas, items *item.Items, enemies *enemy.Enemies) *Manager {
	m := &Manager{
		areas: areas, items: items, enemies: enemies,
		events: make(chan any), done: make(chan struct{}),
	}
	go m.run()
	return m
}

func (m *Manager) Items() *item.Items { return m.items }

func (m *Manager) Enemies() *enemy.Enemies { return m.enemies }

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
