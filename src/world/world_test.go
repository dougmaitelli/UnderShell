package world

import (
	"testing"
	"time"
)

func TestPlayersSeeMovementAndReconnectReplacesSession(t *testing.T) {
	manager := New(10, 10)
	defer manager.Close()

	first := manager.Join(Player{ID: 1, Name: "Aria", X: 2, Y: 2})
	second := manager.Join(Player{ID: 2, Name: "Rowan", X: 4, Y: 4})
	receiveSnapshot(t, first.Updates)
	receiveSnapshot(t, second.Updates)

	moved := manager.Move(1, first.Token, 1, 0)
	if moved.X != 3 || moved.Y != 2 {
		t.Fatalf("unexpected position: %#v", moved)
	}
	snapshot := receiveSnapshot(t, second.Updates)
	assertPlayer(t, snapshot, 1, 3, 2)

	replacement := manager.Join(Player{ID: 1, Name: "Aria", X: 3, Y: 2})
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

func TestMovementIsClampedToWorld(t *testing.T) {
	manager := New(3, 3)
	defer manager.Close()
	session := manager.Join(Player{ID: 1, Name: "Aria"})
	receiveSnapshot(t, session.Updates)

	player := manager.Move(1, session.Token, -1, -1)
	if player.X != 0 || player.Y != 0 {
		t.Fatalf("position escaped lower boundary: %#v", player)
	}
	for range 5 {
		player = manager.Move(1, session.Token, 1, 1)
	}
	if player.X != 2 || player.Y != 2 {
		t.Fatalf("position escaped upper boundary: %#v", player)
	}
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

func assertPlayer(t *testing.T, snapshot Snapshot, id int64, x, y int) {
	t.Helper()
	for _, player := range snapshot.Players {
		if player.ID == id {
			if player.X != x || player.Y != y {
				t.Fatalf("unexpected position: %#v", player)
			}
			return
		}
	}
	t.Fatalf("player %d missing from %#v", id, snapshot)
}
