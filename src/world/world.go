// Package world manages live multiplayer state.
package world

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand/v2"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"sshrpg/src/enemy"
	"sshrpg/src/item"
)

type Player struct {
	ID          int64
	Name        string
	AreaID      string
	X           int
	Y           int
	Health      int
	MaxHealth   int
	Level       int
	Experience  int64
	SkillPoints int
	Attack      int
	Defense     int
	Vitality    int
}

type Snapshot struct {
	Area    *Area
	Players []Player
	Enemies []Enemy
	Drops   []GroundItem
}

type Enemy struct {
	ID           uint64
	DefinitionID string
	Name         string
	Visual       []string
	Health       int
	MaxHealth    int
	Damage       int
	Experience   int64
	AreaID       string
	X            int
	Y            int
	spawnIndex   int
	nextAttack   time.Time
}

type GroundItem struct {
	ID     uint64
	ItemID string
	Name   string
	AreaID string
	X      int
	Y      int
}

type Session struct {
	Token   string
	Updates <-chan Snapshot
	Events  <-chan Event
	Chats   <-chan ChatMessage
	Kicked  <-chan struct{}
}

type ChatMessage struct {
	PlayerID   int64
	PlayerName string
	Message    string
}

type EventKind string

const (
	EventPickup      EventKind = "pickup"
	EventProgression EventKind = "progression"
	EventCombat      EventKind = "combat"
	EventDamage      EventKind = "damage"
	EventDeath       EventKind = "death"
	EventRespawn     EventKind = "respawn"
)

type Event struct {
	Kind    EventKind
	Message string
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
	events  chan Event
	chats   chan ChatMessage
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
type attackRequest struct {
	id    int64
	token string
	reply chan AttackResult
}
type pickupRequest struct {
	id    int64
	token string
	reply chan PickupResult
}
type spendSkillRequest struct {
	id    int64
	token string
	skill string
	reply chan Player
}
type chatRequest struct {
	id      int64
	token   string
	message string
	reply   chan bool
}

type AttackResult struct {
	HitIDs      []uint64
	DefeatedIDs []uint64
}

const attackRange = 2
const pickupRange = 2
const enemyAggroRange = 8
const playerMaxHealth = 10
const enemyAttackInterval = 1500 * time.Millisecond
const vitalityHealthPerRank = 5
const chatHistoryLimit = 10
const chatMessageLimit = 200

type PickupResult struct {
	Item  GroundItem
	Found bool
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
	groundItems := make(map[uint64]*GroundItem)
	spawns := make(map[spawnKey]*spawnState)
	var nextEnemyID uint64
	var nextGroundItemID uint64
	chatHistory := make([]ChatMessage, 0, chatHistoryLimit)
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
					close(previous.events)
					close(previous.chats)
				}
				m.placePlayer(&e.player)
				p := &activePlayer{
					Player: e.player, token: newToken(),
					updates: make(chan Snapshot, 1),
					events:  make(chan Event, 32),
					chats:   make(chan ChatMessage, 32), kicked: make(chan struct{}),
				}
				players[p.ID] = p
				for _, message := range chatHistory {
					p.chats <- message
				}
				m.broadcastState(players, liveEnemies, groundItems)
				e.reply <- Session{
					Token: p.token, Updates: p.updates, Events: p.events,
					Chats: p.chats, Kicked: p.kicked,
				}
			case moveRequest:
				p := players[e.id]
				if p != nil && p.token == e.token {
					m.move(p, e.dx, e.dy)
					m.broadcastState(players, liveEnemies, groundItems)
					e.reply <- p.Player
				} else {
					e.reply <- Player{}
				}
			case leaveRequest:
				if p := players[e.id]; p != nil && p.token == e.token {
					delete(players, e.id)
					close(p.kicked)
					close(p.updates)
					close(p.events)
					close(p.chats)
					m.broadcastState(players, liveEnemies, groundItems)
				}
			case defeatEnemyRequest:
				target := liveEnemies[e.id]
				if target == nil {
					e.reply <- false
					break
				}
				m.removeEnemy(
					liveEnemies, groundItems, spawns, target, &nextGroundItemID,
				)
				m.broadcastState(players, liveEnemies, groundItems)
				e.reply <- true
			case attackRequest:
				result := AttackResult{}
				player := players[e.id]
				if player == nil || player.token != e.token {
					e.reply <- result
					break
				}
				for _, target := range liveEnemies {
					if target.AreaID != player.AreaID ||
						abs(target.X-player.X) > attackRange ||
						abs(target.Y-player.Y) > attackRange {
						continue
					}
					target.Health -= 1 + player.Attack
					result.HitIDs = append(result.HitIDs, target.ID)
					if target.Health <= 0 {
						result.DefeatedIDs = append(result.DefeatedIDs, target.ID)
						sendEvent(player, Event{Kind: EventCombat, Message: "Defeated " + target.Name})
						sendEvent(player, Event{
							Kind:    EventProgression,
							Message: fmt.Sprintf("Gained %d XP", target.Experience),
						})
						previousLevel := player.Level
						grantExperience(&player.Player, target.Experience)
						for level := previousLevel + 1; level <= player.Level; level++ {
							sendEvent(player, Event{
								Kind:    EventProgression,
								Message: fmt.Sprintf("Level up! Reached level %d", level),
							})
							sendEvent(player, Event{
								Kind: EventProgression, Message: "Gained 1 skill point",
							})
						}
						m.removeEnemy(
							liveEnemies, groundItems, spawns, target, &nextGroundItemID,
						)
					}
				}
				if len(result.HitIDs) > 0 {
					m.broadcastState(players, liveEnemies, groundItems)
				}
				e.reply <- result
			case pickupRequest:
				result := PickupResult{}
				player := players[e.id]
				if player == nil || player.token != e.token {
					e.reply <- result
					break
				}
				for id, drop := range groundItems {
					if drop.AreaID != player.AreaID ||
						abs(drop.X-player.X) > pickupRange ||
						abs(drop.Y-player.Y) > pickupRange {
						continue
					}
					result.Item, result.Found = *drop, true
					delete(groundItems, id)
					break
				}
				if result.Found {
					m.broadcastState(players, liveEnemies, groundItems)
				}
				e.reply <- result
			case spendSkillRequest:
				player := players[e.id]
				if player == nil || player.token != e.token || player.SkillPoints < 1 {
					e.reply <- Player{}
					break
				}
				valid := true
				switch e.skill {
				case "attack":
					player.Attack++
				case "defense":
					player.Defense++
				case "vitality":
					player.Vitality++
					player.MaxHealth += vitalityHealthPerRank
					player.Health += vitalityHealthPerRank
				default:
					valid = false
				}
				if !valid {
					e.reply <- Player{}
					break
				}
				player.SkillPoints--
				m.broadcastState(players, liveEnemies, groundItems)
				e.reply <- player.Player
			case chatRequest:
				player := players[e.id]
				message, ok := validateChatMessage(e.message)
				if player == nil || player.token != e.token || !ok {
					e.reply <- false
					break
				}
				chat := ChatMessage{
					PlayerID: player.ID, PlayerName: player.Name, Message: message,
				}
				chatHistory = append(chatHistory, chat)
				if len(chatHistory) > chatHistoryLimit {
					chatHistory = chatHistory[len(chatHistory)-chatHistoryLimit:]
				}
				for _, recipient := range players {
					select {
					case recipient.chats <- chat:
					default:
					}
				}
				e.reply <- true
			}
		case now := <-ticker.C:
			changed := m.updateEnemies(players, liveEnemies, now)
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
				m.broadcastState(players, liveEnemies, groundItems)
			}
		case <-m.done:
			for _, p := range players {
				close(p.kicked)
				close(p.updates)
				close(p.events)
				close(p.chats)
			}
			return
		}
	}
}

func (m *Manager) placePlayer(player *Player) {
	expectedMaxHealth := playerMaxHealth + player.Vitality*vitalityHealthPerRank
	if player.MaxHealth < 1 {
		player.Health = expectedMaxHealth
		player.MaxHealth = expectedMaxHealth
	}
	if player.Level < 1 {
		player.Level = 1
	}
	area, ok := m.areas.Area(player.AreaID)
	if !ok {
		area, spawn := m.areas.DefaultSpawn()
		player.AreaID = area.ID
		player.X, player.Y = spawn.X, spawn.Y
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

func (m *Manager) broadcastState(
	players map[int64]*activePlayer,
	enemies map[uint64]*Enemy,
	drops map[uint64]*GroundItem,
) {
	for _, recipient := range players {
		area, _ := m.areas.Area(recipient.AreaID)
		snapshot := Snapshot{
			Area:    area,
			Players: make([]Player, 0, len(players)),
			Enemies: make([]Enemy, 0),
			Drops:   make([]GroundItem, 0),
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
		for _, drop := range drops {
			if drop.AreaID == recipient.AreaID {
				snapshot.Drops = append(snapshot.Drops, *drop)
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
		Health: definition.Health, MaxHealth: definition.Health,
		Damage: definition.Damage, Experience: definition.Experience,
		AreaID: area.ID, X: point.X, Y: point.Y, spawnIndex: spawnIndex,
	}
}

func (m *Manager) removeEnemy(
	live map[uint64]*Enemy,
	groundItems map[uint64]*GroundItem,
	spawns map[spawnKey]*spawnState,
	target *Enemy,
	nextGroundItemID *uint64,
) {
	key := spawnKey{areaID: target.AreaID, index: target.spawnIndex}
	delete(live, target.ID)
	if definition, ok := m.enemies.Enemy(target.DefinitionID); ok {
		for _, drop := range definition.Drops {
			if mathrand.Float64() > drop.Chance {
				continue
			}
			itemDefinition, ok := m.items.Item(drop.ItemID)
			if !ok {
				continue
			}
			*nextGroundItemID++
			groundItems[*nextGroundItemID] = &GroundItem{
				ID: *nextGroundItemID, ItemID: itemDefinition.ID, Name: itemDefinition.Name,
				AreaID: target.AreaID, X: target.X, Y: target.Y,
			}
		}
	}
	state := spawns[key]
	state.count--
	if state.nextSpawn.IsZero() {
		spawn := m.areas.areas[key.areaID].EnemySpawns[key.index]
		state.nextSpawn = time.Now().Add(time.Duration(spawn.RespawnSeconds) * time.Second)
	}
}

func (m *Manager) updateEnemies(
	players map[int64]*activePlayer,
	live map[uint64]*Enemy,
	now time.Time,
) bool {
	changed := false
	respawned := make(map[int64]bool)
	directions := [...]Point{{}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	for _, current := range live {
		area := m.areas.areas[current.AreaID]
		spawn := area.EnemySpawns[current.spawnIndex]
		var targetPlayer *activePlayer
		distance := enemyAggroRange + 1
		if current.Damage > 0 {
			targetPlayer, distance = nearestPlayer(current, players, respawned)
		}
		if targetPlayer != nil && distance <= 1 {
			if now.Before(current.nextAttack) {
				continue
			}
			damage := max(current.Damage-targetPlayer.Defense, 0)
			previousHealth := targetPlayer.Health
			targetPlayer.Health = max(targetPlayer.Health-damage, 0)
			current.nextAttack = now.Add(enemyAttackInterval)
			if actualDamage := previousHealth - targetPlayer.Health; actualDamage > 0 {
				sendEvent(targetPlayer, Event{
					Kind:    EventDamage,
					Message: fmt.Sprintf("Took %d damage from %s", actualDamage, current.Name),
				})
			}
			if targetPlayer.Health == 0 {
				sendEvent(targetPlayer, Event{Kind: EventDeath, Message: "You were defeated"})
				m.respawnPlayer(&targetPlayer.Player)
				spawnArea, _ := m.areas.Area(targetPlayer.AreaID)
				sendEvent(targetPlayer, Event{
					Kind:    EventRespawn,
					Message: "Respawned in " + spawnArea.Name,
				})
				respawned[targetPlayer.ID] = true
			}
			changed = true
			continue
		}

		direction := directions[mathrand.IntN(len(directions))]
		if targetPlayer != nil {
			direction = chaseDirection(current.X, current.Y, targetPlayer.X, targetPlayer.Y)
		}
		target := Point{X: current.X + direction.X, Y: current.Y + direction.Y}
		if !enemyCanMoveTo(area, spawn, target) {
			if targetPlayer == nil {
				continue
			}
			direction = chaseFallbackDirection(current.X, current.Y, targetPlayer.X, targetPlayer.Y)
			target = Point{X: current.X + direction.X, Y: current.Y + direction.Y}
			if !enemyCanMoveTo(area, spawn, target) {
				continue
			}
		}
		if target.X == current.X && target.Y == current.Y {
			continue
		}
		current.X, current.Y = target.X, target.Y
		changed = true
	}
	return changed
}

func (m *Manager) respawnPlayer(player *Player) {
	spawnArea, spawn := m.areas.DefaultSpawn()
	player.AreaID = spawnArea.ID
	player.X, player.Y = spawn.X, spawn.Y
	player.MaxHealth = playerMaxHealth + player.Vitality*vitalityHealthPerRank
	player.Health = player.MaxHealth
}

func nearestPlayer(
	current *Enemy,
	players map[int64]*activePlayer,
	excluded map[int64]bool,
) (*activePlayer, int) {
	var nearest *activePlayer
	nearestDistance := enemyAggroRange + 1
	for _, player := range players {
		if excluded[player.ID] || player.AreaID != current.AreaID || player.Health <= 0 {
			continue
		}
		distance := max(abs(player.X-current.X), abs(player.Y-current.Y))
		if distance <= enemyAggroRange && distance < nearestDistance {
			nearest, nearestDistance = player, distance
		}
	}
	return nearest, nearestDistance
}

func chaseDirection(fromX, fromY, toX, toY int) Point {
	dx, dy := toX-fromX, toY-fromY
	if abs(dx) >= abs(dy) && dx != 0 {
		return Point{X: sign(dx)}
	}
	return Point{Y: sign(dy)}
}

func chaseFallbackDirection(fromX, fromY, toX, toY int) Point {
	dx, dy := toX-fromX, toY-fromY
	if abs(dx) >= abs(dy) {
		return Point{Y: sign(dy)}
	}
	return Point{X: sign(dx)}
}

func enemyCanMoveTo(area *Area, spawn EnemySpawn, target Point) bool {
	return target.X >= spawn.X && target.X < spawn.X+spawn.Width &&
		target.Y >= spawn.Y && target.Y < spawn.Y+spawn.Height &&
		area.Walkable(target)
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}

func sendEvent(player *activePlayer, event Event) {
	select {
	case player.events <- event:
	default:
		// Keep the world loop responsive if a client stops consuming events.
	}
}

func validateChatMessage(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > chatMessageLimit {
		return "", false
	}
	for _, character := range value {
		if unicode.IsControl(character) || !unicode.IsPrint(character) {
			return "", false
		}
	}
	return value, true
}

// ExperienceToNextLevel returns the XP required to advance from level to level+1.
// The quadratic requirement is 100*level² and saturates only at integer capacity.
func ExperienceToNextLevel(level int) int64 {
	const maxExperience = int64(1<<63 - 1)
	if level < 1 {
		level = 1
	}
	const largestSafeLevel = 303700049
	if level > largestSafeLevel {
		return maxExperience
	}
	value := int64(level)
	return 100 * value * value
}

func grantExperience(player *Player, reward int64) {
	const maxExperience = int64(1<<63 - 1)
	if reward <= 0 {
		return
	}
	if player.Level < 1 {
		player.Level = 1
	}
	if reward > maxExperience-player.Experience {
		player.Experience = maxExperience
	} else {
		player.Experience += reward
	}
	for {
		requirement := ExperienceToNextLevel(player.Level)
		if player.Experience < requirement {
			return
		}
		player.Experience -= requirement
		player.Level++
		player.SkillPoints++
	}
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

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func newToken() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	return hex.EncodeToString(value[:])
}
