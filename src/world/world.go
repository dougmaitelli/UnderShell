// Package world manages live multiplayer state.
package world

import (
	"crypto/rand"
	"encoding/hex"
)

type Player struct {
	ID   int64
	Name string
	X    int
	Y    int
}

type Snapshot struct {
	Players []Player
}

type Session struct {
	Token   string
	Updates <-chan Snapshot
	Kicked  <-chan struct{}
}

type Manager struct {
	width  int
	height int
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

func New(width, height int) *Manager {
	m := &Manager{width: width, height: height, events: make(chan any), done: make(chan struct{})}
	go m.run()
	return m
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
				e.player.X = clamp(e.player.X, 0, m.width-1)
				e.player.Y = clamp(e.player.Y, 0, m.height-1)
				p := &activePlayer{
					Player: e.player, token: newToken(),
					updates: make(chan Snapshot, 1), kicked: make(chan struct{}),
				}
				players[p.ID] = p
				e.reply <- Session{Token: p.token, Updates: p.updates, Kicked: p.kicked}
				broadcast(players)
			case moveRequest:
				p := players[e.id]
				if p != nil && p.token == e.token {
					p.X = clamp(p.X+e.dx, 0, m.width-1)
					p.Y = clamp(p.Y+e.dy, 0, m.height-1)
					e.reply <- p.Player
					broadcast(players)
				} else {
					e.reply <- Player{}
				}
			case leaveRequest:
				if p := players[e.id]; p != nil && p.token == e.token {
					delete(players, e.id)
					close(p.kicked)
					close(p.updates)
					broadcast(players)
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

func broadcast(players map[int64]*activePlayer) {
	snapshot := Snapshot{Players: make([]Player, 0, len(players))}
	for _, p := range players {
		snapshot.Players = append(snapshot.Players, p.Player)
	}
	for _, p := range players {
		select {
		case <-p.updates:
		default:
		}
		select {
		case p.updates <- snapshot:
		default:
		}
	}
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func newToken() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	return hex.EncodeToString(value[:])
}
