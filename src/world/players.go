package world

import (
	"errors"
	"strings"

	"sshrpg/src/item"
)

var ErrPlayerNotOnline = errors.New("player is not online")

type activePlayer struct {
	Player
	token   string
	updates chan Snapshot
	events  chan Event
	chats   chan ChatMessage
	kicked  chan string
}

type playerSystem struct {
	areas *Areas
	live  map[int64]*activePlayer
}

func (s *playerSystem) activeAreas() map[string]bool {
	areas := make(map[string]bool, len(s.live))
	for _, player := range s.live {
		areas[player.AreaID] = true
	}
	return areas
}

func (s *playerSystem) join(
	player Player,
	chatHistory []ChatMessage,
	snapshot func(*activePlayer) Snapshot,
) Session {
	if previous := s.live[player.ID]; previous != nil {
		closePlayer(previous, "This character connected from another session.")
	}
	s.place(&player)
	active := &activePlayer{
		Player:  player,
		token:   newToken(),
		updates: make(chan Snapshot, 1),
		events:  make(chan Event, 32),
		chats:   make(chan ChatMessage, 32),
		kicked:  make(chan string, 1),
	}
	s.live[active.ID] = active
	for _, message := range chatHistory {
		active.chats <- message
	}
	s.broadcast(snapshot)
	return Session{
		Token: active.token, Updates: active.updates, Events: active.events,
		Chats: active.chats, Kicked: active.kicked,
	}
}

func (s *playerSystem) authenticated(id int64, token string) *activePlayer {
	player := s.live[id]
	if player == nil || player.token != token {
		return nil
	}
	return player
}

func (s *playerSystem) byName(name string) *activePlayer {
	name = strings.TrimSpace(name)
	for _, player := range s.live {
		if strings.EqualFold(player.Name, name) {
			return player
		}
	}
	return nil
}

func (s *playerSystem) move(
	id int64,
	token string,
	dx, dy int,
	broadcast func(),
) Player {
	player := s.authenticated(id, token)
	if player == nil {
		return Player{}
	}
	s.moveActive(player, dx, dy)
	broadcast()
	return player.Player
}

func (s *playerSystem) leave(id int64, token string) bool {
	player := s.authenticated(id, token)
	if player == nil {
		return false
	}
	delete(s.live, id)
	closePlayer(player, "")
	return true
}

func (s *playerSystem) kick(name, reason string) (Player, error) {
	player := s.byName(name)
	if player == nil {
		return Player{}, ErrPlayerNotOnline
	}
	delete(s.live, player.ID)
	result := player.Player
	closePlayer(player, reason)
	return result, nil
}

func (s *playerSystem) place(player *Player) {
	expectedMaxHealth := playerMaxHealth +
		(player.Vitality+player.EquipmentStats.Vitality)*vitalityHealthPerRank
	if player.MaxHealth < 1 {
		player.Health = expectedMaxHealth
		player.MaxHealth = expectedMaxHealth
	}
	if player.Level < 1 {
		player.Level = 1
	}
	area, ok := s.areas.Area(player.AreaID)
	if !ok {
		area, spawn := s.areas.DefaultSpawn()
		player.AreaID = area.ID
		player.X, player.Y = spawn.X, spawn.Y
		return
	}
	if !area.Walkable(Point{X: player.X, Y: player.Y}) {
		player.X, player.Y = area.Spawn.X, area.Spawn.Y
	}
}

func (s *playerSystem) moveActive(player *activePlayer, dx, dy int) {
	area, ok := s.areas.Area(player.AreaID)
	if !ok {
		s.place(&player.Player)
		return
	}
	target := Point{X: player.X + dx, Y: player.Y + dy}
	if !area.Walkable(target) {
		return
	}
	if _, occupied := area.NPCAt(target); occupied {
		return
	}
	player.X, player.Y = target.X, target.Y
	if waypoint, ok := area.Waypoint(target); ok {
		player.AreaID = waypoint.Destination.ID
		player.X, player.Y = waypoint.DestinationX, waypoint.DestinationY
	}
}

func (s *playerSystem) respawn(player *activePlayer) {
	spawnArea, spawn := s.areas.DefaultSpawn()
	player.respawn(spawnArea.ID, spawn)
}

func (p *activePlayer) respawn(areaID string, spawn Point) {
	p.AreaID = areaID
	p.X, p.Y = spawn.X, spawn.Y
	p.MaxHealth = playerMaxHealth +
		(p.Vitality+p.EquipmentStats.Vitality)*vitalityHealthPerRank
	p.Health = p.MaxHealth
}

func (p *activePlayer) setEquipmentStats(stats item.EquipmentStats) {
	previousMaxHealth := p.MaxHealth
	p.EquipmentStats = stats
	p.MaxHealth = playerMaxHealth +
		(p.Vitality+p.EquipmentStats.Vitality)*vitalityHealthPerRank
	if p.MaxHealth > previousMaxHealth {
		p.Health += p.MaxHealth - previousMaxHealth
	}
	p.Health = min(p.Health, p.MaxHealth)
}

func (s *playerSystem) snapshot(
	recipient *activePlayer,
	enemies map[uint64]*Enemy,
	drops map[uint64]*GroundItem,
) Snapshot {
	area, _ := s.areas.Area(recipient.AreaID)
	snapshot := Snapshot{
		Area: area, Players: make([]Player, 0, len(s.live)),
		Enemies: make([]Enemy, 0), Drops: make([]GroundItem, 0),
	}
	for _, player := range s.live {
		if player.AreaID == recipient.AreaID {
			snapshot.Players = append(snapshot.Players, player.Player)
		}
	}
	for _, enemy := range enemies {
		if enemy.AreaID == recipient.AreaID {
			snapshot.Enemies = append(snapshot.Enemies, *enemy)
		}
	}
	for _, drop := range drops {
		if drop.AreaID == recipient.AreaID {
			snapshot.Drops = append(snapshot.Drops, *drop)
		}
	}
	return snapshot
}

func (s *playerSystem) broadcast(snapshot func(*activePlayer) Snapshot) {
	for _, recipient := range s.live {
		next := snapshot(recipient)
		select {
		case <-recipient.updates:
		default:
		}
		select {
		case recipient.updates <- next:
		default:
		}
	}
}

func (s *playerSystem) closeAll() {
	for _, player := range s.live {
		closePlayer(player, "")
	}
}

func closePlayer(player *activePlayer, reason string) {
	if reason != "" {
		player.kicked <- reason
	}
	close(player.kicked)
	close(player.updates)
	close(player.events)
	close(player.chats)
}

func sendEvent(player *activePlayer, event Event) {
	select {
	case player.events <- event:
	default:
		// Keep the world loop responsive if a client stops consuming events.
	}
}
