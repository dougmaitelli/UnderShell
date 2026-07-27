// Package world manages live multiplayer state.
package world

import (
	"crypto/rand"
	"encoding/hex"

	"sshrpg/src/item"
)

type Player struct {
	ID     int64
	Name   string
	AreaID string
	X      int
	Y      int
}

type Snapshot struct {
	Area    *Area
	Players []Player
}

type Session struct {
	Token   string
	Updates <-chan Snapshot
	Kicked  <-chan struct{}
}

type Manager struct {
	areas  *AreaSet
	items  *item.Catalog
	events chan any
	done   chan struct{}
}

type activePlayer struct {
	Player
	token   string
	updates chan Snapshot
	kicked  chan struct{}
}

type joinRequest struct {
	player Player
	reply  chan Session
}
type moveRequest struct {
	id    int64
	token string
	dx    int
	dy    int
	reply chan Player
}
type leaveRequest struct {
	id    int64
	token string
}

func New(areas *AreaSet, items *item.Catalog) *Manager {
	m := &Manager{
		areas: areas, items: items,
		events: make(chan any), done: make(chan struct{}),
	}
	go m.run()
	return m
}

func (m *Manager) Items() *item.Catalog {
	return m.items
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

func (m *Manager) Leave(id int64, token string) {
	select {
	case m.events <- leaveRequest{id: id, token: token}:
	case <-m.done:
	}
}

func (m *Manager) run() {
	players := make(map[int64]*activePlayer)
	for {
		select {
		case event := <-m.events:
			switch e := event.(type) {
			case joinRequest:
				if previous := players[e.player.ID]; previous != nil {
					close(previous.kicked)
					close(previous.updates)
				}
				m.placePlayer(&e.player)
				p := &activePlayer{
					Player: e.player, token: newToken(),
					updates: make(chan Snapshot, 1), kicked: make(chan struct{}),
				}
				players[p.ID] = p
				e.reply <- Session{Token: p.token, Updates: p.updates, Kicked: p.kicked}
				m.broadcast(players)
			case moveRequest:
				p := players[e.id]
				if p != nil && p.token == e.token {
					m.move(p, e.dx, e.dy)
					e.reply <- p.Player
					m.broadcast(players)
				} else {
					e.reply <- Player{}
				}
			case leaveRequest:
				if p := players[e.id]; p != nil && p.token == e.token {
					delete(players, e.id)
					close(p.kicked)
					close(p.updates)
					m.broadcast(players)
				}
			}
		case <-m.done:
			for _, p := range players {
				close(p.kicked)
				close(p.updates)
			}
			return
		}
	}
}

func (m *Manager) placePlayer(player *Player) {
	area, ok := m.areas.Area(player.AreaID)
	if !ok {
		area = m.areas.Default()
		player.AreaID = area.ID
		player.X, player.Y = area.Spawn.X, area.Spawn.Y
		return
	}
	if !area.Walkable(Point{X: player.X, Y: player.Y}) {
		player.X, player.Y = area.Spawn.X, area.Spawn.Y
	}
}

func (m *Manager) move(player *activePlayer, dx, dy int) {
	area, ok := m.areas.Area(player.AreaID)
	if !ok {
		m.placePlayer(&player.Player)
		return
	}
	target := Point{X: player.X + dx, Y: player.Y + dy}
	if !area.Walkable(target) {
		return
	}
	player.X, player.Y = target.X, target.Y

	if waypoint, ok := area.Waypoint(target); ok {
		player.AreaID = waypoint.DestinationArea
		player.X, player.Y = waypoint.DestinationX, waypoint.DestinationY
	}
}

func (m *Manager) broadcast(players map[int64]*activePlayer) {
	for _, recipient := range players {
		area, _ := m.areas.Area(recipient.AreaID)
		snapshot := Snapshot{Area: area, Players: make([]Player, 0, len(players))}
		for _, player := range players {
			if player.AreaID == recipient.AreaID {
				snapshot.Players = append(snapshot.Players, player.Player)
			}
		}
		select {
		case <-recipient.updates:
		default:
		}
		select {
		case recipient.updates <- snapshot:
		default:
		}
	}
}

func newToken() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	return hex.EncodeToString(value[:])
}
