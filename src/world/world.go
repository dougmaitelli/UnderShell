// Package world manages live multiplayer state.
package world

import (
	"crypto/rand"
	"encoding/hex"
	mathrand "math/rand/v2"
	"time"

	"sshrpg/src/enemy"
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
	Enemies []Enemy
}

type Enemy struct {
	ID           uint64
	DefinitionID string
	Name         string
	Visual       []string
	AreaID       string
	X            int
	Y            int
	spawnIndex   int
}

type Session struct {
	Token   string
	Updates <-chan Snapshot
	Kicked  <-chan struct{}
}

type Manager struct {
	areas   *Areas
	items   *item.Items
	enemies *enemy.Enemies
	events  chan any
	done    chan struct{}
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
type defeatEnemyRequest struct {
	id    uint64
	reply chan bool
}

func New(areas *Areas, items *item.Items, enemies *enemy.Enemies) *Manager {
	m := &Manager{
		areas: areas, items: items, enemies: enemies,
		events: make(chan any), done: make(chan struct{}),
	}
	go m.run()
	return m
}

func (m *Manager) Items() *item.Items {
	return m.items
}

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

type spawnKey struct {
	areaID string
	index  int
}

type spawnState struct {
	count     int
	nextSpawn time.Time
}

func (m *Manager) run() {
	players := make(map[int64]*activePlayer)
	liveEnemies := make(map[uint64]*Enemy)
	spawns := make(map[spawnKey]*spawnState)
	var nextEnemyID uint64
	for _, area := range m.areas.areas {
		for index, spawn := range area.EnemySpawns {
			state := &spawnState{}
			spawns[spawnKey{areaID: area.ID, index: index}] = state
			for state.count < spawn.MaxEnemies {
				nextEnemyID++
				m.spawnEnemy(liveEnemies, area, index, nextEnemyID)
				state.count++
			}
		}
	}
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
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
				m.broadcastState(players, liveEnemies)
			case moveRequest:
				p := players[e.id]
				if p != nil && p.token == e.token {
					m.move(p, e.dx, e.dy)
					e.reply <- p.Player
					m.broadcastState(players, liveEnemies)
				} else {
					e.reply <- Player{}
				}
			case leaveRequest:
				if p := players[e.id]; p != nil && p.token == e.token {
					delete(players, e.id)
					close(p.kicked)
					close(p.updates)
					m.broadcastState(players, liveEnemies)
				}
			case defeatEnemyRequest:
				target := liveEnemies[e.id]
				if target == nil {
					e.reply <- false
					break
				}
				key := spawnKey{areaID: target.AreaID, index: target.spawnIndex}
				delete(liveEnemies, e.id)
				state := spawns[key]
				state.count--
				if state.nextSpawn.IsZero() {
					spawn := m.areas.areas[key.areaID].EnemySpawns[key.index]
					state.nextSpawn = time.Now().Add(time.Duration(spawn.RespawnSeconds) * time.Second)
				}
				e.reply <- true
				m.broadcastState(players, liveEnemies)
			}
		case now := <-ticker.C:
			changed := m.moveEnemies(liveEnemies)
			for key, state := range spawns {
				area := m.areas.areas[key.areaID]
				spawn := area.EnemySpawns[key.index]
				if state.count >= spawn.MaxEnemies || state.nextSpawn.IsZero() || now.Before(state.nextSpawn) {
					continue
				}
				nextEnemyID++
				m.spawnEnemy(liveEnemies, area, key.index, nextEnemyID)
				state.count++
				changed = true
				if state.count < spawn.MaxEnemies {
					state.nextSpawn = now.Add(time.Duration(spawn.RespawnSeconds) * time.Second)
				} else {
					state.nextSpawn = time.Time{}
				}
			}
			if changed {
				m.broadcastState(players, liveEnemies)
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

func (m *Manager) broadcastState(players map[int64]*activePlayer, enemies map[uint64]*Enemy) {
	for _, recipient := range players {
		area, _ := m.areas.Area(recipient.AreaID)
		snapshot := Snapshot{
			Area:    area,
			Players: make([]Player, 0, len(players)),
			Enemies: make([]Enemy, 0),
		}
		for _, player := range players {
			if player.AreaID == recipient.AreaID {
				snapshot.Players = append(snapshot.Players, player.Player)
			}
		}
		for _, enemy := range enemies {
			if enemy.AreaID == recipient.AreaID {
				snapshot.Enemies = append(snapshot.Enemies, *enemy)
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

func (m *Manager) spawnEnemy(live map[uint64]*Enemy, area *Area, spawnIndex int, id uint64) {
	spawn := area.EnemySpawns[spawnIndex]
	definition, ok := m.enemies.Enemy(spawn.EnemyID)
	if !ok {
		return
	}
	points := walkableSpawnPoints(area, spawn)
	point := points[mathrand.IntN(len(points))]
	live[id] = &Enemy{
		ID: id, DefinitionID: definition.ID, Name: definition.Name,
		Visual: append([]string(nil), definition.Visual...),
		AreaID: area.ID, X: point.X, Y: point.Y, spawnIndex: spawnIndex,
	}
}

func (m *Manager) moveEnemies(live map[uint64]*Enemy) bool {
	changed := false
	directions := [...]Point{{}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	for _, current := range live {
		area := m.areas.areas[current.AreaID]
		spawn := area.EnemySpawns[current.spawnIndex]
		direction := directions[mathrand.IntN(len(directions))]
		target := Point{X: current.X + direction.X, Y: current.Y + direction.Y}
		if target.X < spawn.X || target.X >= spawn.X+spawn.Width ||
			target.Y < spawn.Y || target.Y >= spawn.Y+spawn.Height ||
			!area.Walkable(target) {
			continue
		}
		if target.X != current.X || target.Y != current.Y {
			current.X, current.Y = target.X, target.Y
			changed = true
		}
	}
	return changed
}

func walkableSpawnPoints(area *Area, spawn EnemySpawn) []Point {
	points := make([]Point, 0, spawn.Width*spawn.Height)
	for y := spawn.Y; y < spawn.Y+spawn.Height; y++ {
		for x := spawn.X; x < spawn.X+spawn.Width; x++ {
			point := Point{X: x, Y: y}
			if area.Walkable(point) {
				points = append(points, point)
			}
		}
	}
	return points
}

func newToken() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	return hex.EncodeToString(value[:])
}
