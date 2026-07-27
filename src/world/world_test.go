package world

import (
	"testing"
	"time"

	"sshrpg/src/enemy"
)

func TestPlayersOnlySeeOthersInTheirArea(t *testing.T) {
	manager := New(testAreas(t), nil, nil)
	defer manager.Close()

	first := manager.Join(Player{ID: 1, Name: "Aria", AreaID: "meadow", X: 1, Y: 1})
	second := manager.Join(Player{ID: 2, Name: "Rowan", AreaID: "meadow", X: 2, Y: 1})
	receiveSnapshot(t, first.Updates)
	snapshot := receiveSnapshot(t, second.Updates)
	if len(snapshot.Players) != 2 {
		t.Fatalf("players in meadow = %d, want 2", len(snapshot.Players))
	}

	moved := manager.Move(1, first.Token, 1, 0)
	if moved.AreaID != "cavern" || moved.X != 1 || moved.Y != 1 {
		t.Fatalf("waypoint did not teleport player: %#v", moved)
	}
	firstSnapshot := receiveSnapshot(t, first.Updates)
	if firstSnapshot.Area.ID != "cavern" || len(firstSnapshot.Players) != 1 {
		t.Fatalf("unexpected cavern snapshot: %#v", firstSnapshot)
	}
	secondSnapshot := receiveSnapshot(t, second.Updates)
	if secondSnapshot.Area.ID != "meadow" || len(secondSnapshot.Players) != 1 {
		t.Fatalf("unexpected meadow snapshot: %#v", secondSnapshot)
	}
}

func TestWallsBlockMovementAndReconnectReplacesSession(t *testing.T) {
	manager := New(testAreas(t), nil, nil)
	defer manager.Close()

	first := manager.Join(Player{ID: 1, Name: "Aria", AreaID: "meadow", X: 1, Y: 1})
	receiveSnapshot(t, first.Updates)

	blocked := manager.Move(1, first.Token, 0, -1)
	if blocked.X != 1 || blocked.Y != 1 {
		t.Fatalf("player walked through a wall: %#v", blocked)
	}

	replacement := manager.Join(Player{ID: 1, Name: "Aria", AreaID: "meadow", X: 1, Y: 1})
	select {
	case <-first.Kicked:
	case <-time.After(time.Second):
		t.Fatal("older session was not kicked")
	}
	receiveSnapshot(t, replacement.Updates)

	if stale := manager.Move(1, first.Token, 1, 0); stale.ID != 0 {
		t.Fatalf("stale session moved player: %#v", stale)
	}
}

func TestUnknownOrBlockedSavedLocationUsesDefaultSpawn(t *testing.T) {
	manager := New(testAreas(t), nil, nil)
	defer manager.Close()

	session := manager.Join(Player{ID: 1, Name: "Aria", AreaID: "missing", X: 99, Y: 99})
	snapshot := receiveSnapshot(t, session.Updates)
	assertPlayer(t, snapshot, 1, "meadow", 1, 1)
}

func TestEnemiesSpawnToCapAndRespawnInsideTheirArea(t *testing.T) {
	areas, err := NewAreas([]AreaDefinition{{
		ID: "meadow", Name: "Meadow",
		Layout: []string{"#######", "#.....#", "#######"},
		Spawn:  Point{X: 1, Y: 1},
		EnemySpawns: []EnemySpawn{{
			EnemyID: "slime", X: 2, Y: 1, Width: 3, Height: 1,
			MaxEnemies: 2, RespawnSeconds: 1,
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	enemies, err := enemy.NewEnemies([]enemy.Definition{{
		ID: "slime", Name: "Slime", Visual: []string{"(s)"}, Health: 3,
	}})
	if err != nil {
		t.Fatal(err)
	}
	manager := New(areas, nil, enemies)
	defer manager.Close()

	session := manager.Join(Player{ID: 1, Name: "Aria", AreaID: "meadow", X: 1, Y: 1})
	snapshot := receiveSnapshot(t, session.Updates)
	assertEnemiesInSpawn(t, snapshot, 2)
	if len(snapshot.Enemies[0].Visual) != 1 || snapshot.Enemies[0].Visual[0] != "(s)" {
		t.Fatalf("enemy visual was not copied from its definition: %#v", snapshot.Enemies[0].Visual)
	}
	defeatedID := snapshot.Enemies[0].ID
	if !manager.DefeatEnemy(defeatedID) {
		t.Fatal("expected live enemy to be defeated")
	}
	snapshot = receiveSnapshot(t, session.Updates)
	assertEnemiesInSpawn(t, snapshot, 1)

	deadline := time.After(3 * time.Second)
	for {
		select {
		case snapshot = <-session.Updates:
			assertEnemiesInSpawn(t, snapshot, len(snapshot.Enemies))
			if len(snapshot.Enemies) == 2 {
				if snapshot.Enemies[0].ID == defeatedID || snapshot.Enemies[1].ID == defeatedID {
					t.Fatal("respawned enemy reused a live ID")
				}
				return
			}
		case <-deadline:
			t.Fatal("enemy did not respawn to the configured cap")
		}
	}
}

func TestAttackDamagesAndDefeatsNearbyEnemy(t *testing.T) {
	areas, err := NewAreas([]AreaDefinition{{
		ID: "meadow", Name: "Meadow",
		Layout: []string{"#####", "#...#", "#####"},
		Spawn:  Point{X: 1, Y: 1},
		EnemySpawns: []EnemySpawn{{
			EnemyID: "slime", X: 2, Y: 1, Width: 1, Height: 1,
			MaxEnemies: 1, RespawnSeconds: 10,
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	enemies, err := enemy.NewEnemies([]enemy.Definition{{
		ID: "slime", Name: "Slime", Visual: []string{"(s)"}, Health: 2,
	}})
	if err != nil {
		t.Fatal(err)
	}
	manager := New(areas, nil, enemies)
	defer manager.Close()
	session := manager.Join(Player{ID: 1, Name: "Aria", AreaID: "meadow", X: 1, Y: 1})
	snapshot := receiveSnapshot(t, session.Updates)
	enemyID := snapshot.Enemies[0].ID

	if result := manager.Attack(1, "wrong token"); len(result.HitIDs) != 0 {
		t.Fatalf("unauthenticated attack hit enemies: %#v", result)
	}
	first := manager.Attack(1, session.Token)
	if len(first.HitIDs) != 1 || first.HitIDs[0] != enemyID || len(first.DefeatedIDs) != 0 {
		t.Fatalf("first attack = %#v", first)
	}
	snapshot = receiveSnapshot(t, session.Updates)
	if len(snapshot.Enemies) != 1 || snapshot.Enemies[0].Health != 1 {
		t.Fatalf("enemy did not take damage: %#v", snapshot.Enemies)
	}

	second := manager.Attack(1, session.Token)
	if len(second.DefeatedIDs) != 1 || second.DefeatedIDs[0] != enemyID {
		t.Fatalf("second attack = %#v", second)
	}
	snapshot = receiveSnapshot(t, session.Updates)
	if len(snapshot.Enemies) != 0 {
		t.Fatalf("defeated enemy remains in snapshot: %#v", snapshot.Enemies)
	}
}

func assertEnemiesInSpawn(t *testing.T, snapshot Snapshot, count int) {
	t.Helper()
	if len(snapshot.Enemies) != count {
		t.Fatalf("enemy count = %d, want %d", len(snapshot.Enemies), count)
	}
	for _, current := range snapshot.Enemies {
		if current.AreaID != "meadow" || current.X < 2 || current.X > 4 || current.Y != 1 {
			t.Fatalf("enemy left its spawn area: %#v", current)
		}
	}
}

func testAreas(t *testing.T) *Areas {
	t.Helper()
	areas, err := NewAreas([]AreaDefinition{
		{
			ID: "meadow", Name: "Meadow",
			Layout: []string{"####", "#..#", "####"},
			Spawn:  Point{X: 1, Y: 1},
			Waypoints: []Waypoint{{
				X: 2, Y: 1, DestinationArea: "cavern",
				DestinationX: 1, DestinationY: 1,
			}},
		},
		{
			ID: "cavern", Name: "Cavern",
			Layout: []string{"####", "#..#", "####"},
			Spawn:  Point{X: 1, Y: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return areas
}

func receiveSnapshot(t *testing.T, updates <-chan Snapshot) Snapshot {
	t.Helper()
	select {
	case snapshot := <-updates:
		return snapshot
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for snapshot")
		return Snapshot{}
	}
}

func assertPlayer(t *testing.T, snapshot Snapshot, id int64, areaID string, x, y int) {
	t.Helper()
	for _, player := range snapshot.Players {
		if player.ID == id {
			if player.AreaID != areaID || player.X != x || player.Y != y {
				t.Fatalf("unexpected location: %#v", player)
			}
			return
		}
	}
	t.Fatalf("player %d missing from %#v", id, snapshot)
}
