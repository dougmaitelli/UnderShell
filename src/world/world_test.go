package world

import (
	"testing"
	"time"
)

func TestPlayersOnlySeeOthersInTheirArea(t *testing.T) {
	manager := New(testAreas(t))
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
	manager := New(testAreas(t))
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
	manager := New(testAreas(t))
	defer manager.Close()

	session := manager.Join(Player{ID: 1, Name: "Aria", AreaID: "missing", X: 99, Y: 99})
	snapshot := receiveSnapshot(t, session.Updates)
	assertPlayer(t, snapshot, 1, "meadow", 1, 1)
}

func testAreas(t *testing.T) *AreaSet {
	t.Helper()
	areas, err := NewAreaSet([]AreaDefinition{
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
