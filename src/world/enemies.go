package world

import (
	"fmt"
	mathrand "math/rand/v2"
	"time"
)

type spawnKey struct {
	areaID string
	index  int
}

type spawnState struct {
	count     int
	nextSpawn time.Time
}

type enemySystem struct {
	areas  *Areas
	live   map[uint64]*Enemy
	spawns map[spawnKey]*spawnState
	nextID uint64
}

func (e *Enemy) distanceTo(player *activePlayer) int {
	return max(abs(player.X-e.X), abs(player.Y-e.Y))
}

func (e *Enemy) attack(player *activePlayer, now time.Time) (int, bool) {
	if now.Before(e.nextAttack) {
		return 0, false
	}
	actualDamage := player.takeDamage(e.Definition.Damage)
	e.nextAttack = now.Add(enemyAttackInterval)
	return actualDamage, true
}

func (e *Enemy) move(direction Point) {
	e.X += direction.X
	e.Y += direction.Y
}

func (e *Enemy) canMoveTo(area *Area, spawn EnemySpawn, target Point) bool {
	if _, occupied := area.NPCAt(target); occupied {
		return false
	}
	return target.X >= spawn.X && target.X < spawn.X+spawn.Width &&
		target.Y >= spawn.Y && target.Y < spawn.Y+spawn.Height &&
		area.Walkable(target)
}

func newEnemySystem(areas *Areas) enemySystem {
	return enemySystem{
		areas: areas,
		live:  make(map[uint64]*Enemy), spawns: make(map[spawnKey]*spawnState),
	}
}

func (s *enemySystem) populate() {
	for _, area := range s.areas.areas {
		for index, spawn := range area.EnemySpawns {
			state := &spawnState{}
			s.spawns[spawnKey{areaID: area.ID, index: index}] = state
			for state.count < spawn.MaxEnemies {
				s.nextID++
				s.spawn(area, index, s.nextID)
				state.count++
			}
		}
	}
}

func (s *enemySystem) enemy(id uint64) *Enemy { return s.live[id] }

func (s *enemySystem) spawn(area *Area, spawnIndex int, id uint64) {
	spawn := area.EnemySpawns[spawnIndex]
	if spawn.Enemy == nil {
		return
	}
	points := walkableSpawnPoints(area, spawn)
	point := points[mathrand.IntN(len(points))]
	s.live[id] = &Enemy{
		ID: id, Definition: spawn.Enemy, Health: spawn.Enemy.Health,
		AreaID: area.ID, X: point.X, Y: point.Y, spawnIndex: spawnIndex,
	}
}

func (s *enemySystem) remove(target *Enemy) {
	key := spawnKey{areaID: target.AreaID, index: target.spawnIndex}
	delete(s.live, target.ID)
	state := s.spawns[key]
	state.count--
	if state.nextSpawn.IsZero() {
		spawn := s.areas.areas[key.areaID].EnemySpawns[key.index]
		state.nextSpawn = time.Now().Add(time.Duration(spawn.RespawnSeconds) * time.Second)
	}
}

func (s *enemySystem) tick(players *playerSystem, now time.Time) bool {
	if len(players.live) == 0 {
		return false
	}
	activeAreas := players.activeAreas()
	changed := s.update(players, now)
	for key, state := range s.spawns {
		area := s.areas.areas[key.areaID]
		spawn := area.EnemySpawns[key.index]
		if state.count >= spawn.MaxEnemies || state.nextSpawn.IsZero() || now.Before(state.nextSpawn) {
			continue
		}
		s.nextID++
		s.spawn(area, key.index, s.nextID)
		state.count++
		changed = changed || activeAreas[key.areaID]
		if state.count < spawn.MaxEnemies {
			state.nextSpawn = now.Add(time.Duration(spawn.RespawnSeconds) * time.Second)
		} else {
			state.nextSpawn = time.Time{}
		}
	}
	return changed
}

func (s *enemySystem) update(players *playerSystem, now time.Time) bool {
	changed := false
	respawned := make(map[int64]bool)
	directions := [...]Point{{}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	activeAreas := players.activeAreas()
	for _, current := range s.live {
		if !activeAreas[current.AreaID] {
			continue
		}
		area := s.areas.areas[current.AreaID]
		spawn := area.EnemySpawns[current.spawnIndex]
		var targetPlayer *activePlayer
		distance := enemyAggroRange + 1
		if current.Definition.Damage > 0 {
			targetPlayer, distance = nearestPlayer(current, players.live, respawned)
		}
		if targetPlayer != nil && distance <= 1 {
			actualDamage, attacked := current.attack(targetPlayer, now)
			if !attacked {
				continue
			}
			if actualDamage > 0 {
				sendEvent(targetPlayer, Event{
					Kind: EventDamage,
					Message: fmt.Sprintf(
						"Took %d damage from %s",
						actualDamage, current.Definition.Name,
					),
				})
			}
			if targetPlayer.Health == 0 {
				sendEvent(targetPlayer, Event{Kind: EventDeath, Message: "You were defeated"})
				players.respawn(targetPlayer)
				spawnArea, _ := s.areas.Area(targetPlayer.AreaID)
				sendEvent(targetPlayer, Event{
					Kind: EventRespawn, Message: "Respawned in " + spawnArea.Name,
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
		if !current.canMoveTo(area, spawn, target) {
			if targetPlayer == nil {
				continue
			}
			direction = chaseFallbackDirection(current.X, current.Y, targetPlayer.X, targetPlayer.Y)
			target = Point{X: current.X + direction.X, Y: current.Y + direction.Y}
			if !current.canMoveTo(area, spawn, target) {
				continue
			}
		}
		if target.X == current.X && target.Y == current.Y {
			continue
		}
		current.move(direction)
		changed = true
	}
	return changed
}

// updateEnemies keeps the established internal test seam while delegating to enemySystem.
func (m *Manager) updateEnemies(
	players map[int64]*activePlayer,
	live map[uint64]*Enemy,
	now time.Time,
) bool {
	system := enemySystem{areas: m.areas, live: live}
	return system.update(&playerSystem{areas: m.areas, live: players}, now)
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
		distance := current.distanceTo(player)
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
