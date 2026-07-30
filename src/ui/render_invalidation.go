package ui

import (
	"sshrpg/src/domain"
	"sshrpg/src/world"
)

type viewport struct {
	left   int
	top    int
	width  int
	height int
}

func snapshotAffectsView(
	previous world.Snapshot,
	next world.Snapshot,
	character *domain.Character,
	width, height int,
) bool {
	if character == nil || previous.Area != next.Area {
		return true
	}

	previousSelf := snapshotPlayer(previous, character)
	nextSelf := snapshotPlayer(next, character)
	if previousSelf != nextSelf ||
		len(previous.Players) != len(next.Players) ||
		len(previous.Enemies) != len(next.Enemies) {
		return true
	}

	previousViewport := playerViewport(previousSelf, width, height)
	nextViewport := playerViewport(nextSelf, width, height)
	if previousViewport != nextViewport {
		return true
	}

	return !renderedPlayersEqual(
		previous, next, previousViewport,
	) || !renderedEnemiesEqual(
		previous, next, previousViewport,
	) || !renderedDropsEqual(
		previous, next, previousViewport,
	)
}

func visibleSnapshotPlayers(
	snapshot world.Snapshot,
	character *domain.Character,
	width, height int,
) []world.Player {
	if character == nil {
		return nil
	}
	bounds := playerViewport(snapshotPlayer(snapshot, character), width, height)
	players := make([]world.Player, 0, len(snapshot.Players))
	for _, player := range snapshot.Players {
		if bounds.contains(player.X, player.Y) {
			players = append(players, player)
		}
	}
	return players
}

func snapshotPlayer(snapshot world.Snapshot, character *domain.Character) world.Player {
	for _, player := range snapshot.Players {
		if player.ID == character.ID {
			return player
		}
	}
	return world.Player{
		ID: character.ID, Name: character.Name, Role: character.Role,
		AreaID: character.AreaID, X: character.X, Y: character.Y,
		Level: character.Level, Experience: character.Experience,
		SkillPoints: character.SkillPoints, Attack: character.Attack,
		Defense: character.Defense, Vitality: character.Vitality,
	}
}

func playerViewport(player world.Player, width, height int) viewport {
	mapHeight := max(height-3, 1)
	return viewport{
		left: player.X - width/2, top: player.Y - mapHeight/2,
		width: width, height: mapHeight,
	}
}

func (v viewport) contains(x, y int) bool {
	return x >= v.left && y >= v.top &&
		x < v.left+v.width && y < v.top+v.height
}

func renderedPlayersEqual(
	previous world.Snapshot,
	next world.Snapshot,
	bounds viewport,
) bool {
	visible := 0
	for _, player := range previous.Players {
		if !bounds.contains(player.X, player.Y) {
			continue
		}
		visible++
		found := false
		for _, candidate := range next.Players {
			if candidate.ID != player.ID ||
				!bounds.contains(candidate.X, candidate.Y) {
				continue
			}
			found = candidate.Name == player.Name &&
				candidate.Role == player.Role &&
				candidate.X == player.X &&
				candidate.Y == player.Y
			break
		}
		if !found {
			return false
		}
	}
	return visible == visiblePlayerCount(next, bounds)
}

func visiblePlayerCount(snapshot world.Snapshot, bounds viewport) int {
	count := 0
	for _, player := range snapshot.Players {
		if bounds.contains(player.X, player.Y) {
			count++
		}
	}
	return count
}

func renderedEnemiesEqual(
	previous world.Snapshot,
	next world.Snapshot,
	bounds viewport,
) bool {
	visible := 0
	for _, enemy := range previous.Enemies {
		if !bounds.contains(enemy.X, enemy.Y) {
			continue
		}
		visible++
		found := false
		for _, candidate := range next.Enemies {
			if candidate.ID != enemy.ID ||
				!bounds.contains(candidate.X, candidate.Y) {
				continue
			}
			found = candidate.Definition == enemy.Definition &&
				candidate.Health == enemy.Health &&
				candidate.X == enemy.X &&
				candidate.Y == enemy.Y
			break
		}
		if !found {
			return false
		}
	}
	return visible == visibleEnemyCount(next, bounds)
}

func visibleEnemyCount(snapshot world.Snapshot, bounds viewport) int {
	count := 0
	for _, enemy := range snapshot.Enemies {
		if bounds.contains(enemy.X, enemy.Y) {
			count++
		}
	}
	return count
}

func renderedDropsEqual(
	previous world.Snapshot,
	next world.Snapshot,
	bounds viewport,
) bool {
	visible := 0
	for _, drop := range previous.Drops {
		if !bounds.contains(drop.X, drop.Y) {
			continue
		}
		visible++
		found := false
		for _, candidate := range next.Drops {
			if candidate.ID != drop.ID ||
				!bounds.contains(candidate.X, candidate.Y) {
				continue
			}
			found = candidate.X == drop.X && candidate.Y == drop.Y
			break
		}
		if !found {
			return false
		}
	}
	return visible == visibleDropCount(next, bounds)
}

func visibleDropCount(snapshot world.Snapshot, bounds viewport) int {
	count := 0
	for _, drop := range snapshot.Drops {
		if bounds.contains(drop.X, drop.Y) {
			count++
		}
	}
	return count
}
