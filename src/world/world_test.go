package world

import (
	"fmt"
	"testing"
	"time"

	"sshrpg/src/enemy"
	"sshrpg/src/item"
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
	areas := testAreas(t)
	if err := areas.SetDefaultSpawn("cavern", Point{X: 1, Y: 1}); err != nil {
		t.Fatal(err)
	}
	manager := New(areas, nil, nil)
	defer manager.Close()

	session := manager.Join(Player{ID: 1, Name: "Aria", AreaID: "missing", X: 99, Y: 99})
	snapshot := receiveSnapshot(t, session.Updates)
	assertPlayer(t, snapshot, 1, "cavern", 1, 1)
	if snapshot.Players[0].Health != playerMaxHealth ||
		snapshot.Players[0].MaxHealth != playerMaxHealth {
		t.Fatalf("new player health = %d/%d", snapshot.Players[0].Health, snapshot.Players[0].MaxHealth)
	}
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
		ID: "slime", Name: "Slime", Visual: []string{"(s)"}, Health: 3, Experience: 1,
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
		ID: "slime", Name: "Slime", Visual: []string{"(s)"}, Health: 3, Experience: 125,
		Drops: []enemy.Drop{{ItemID: "slime_gel", Chance: 1}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := item.NewItems([]item.Definition{{
		ID: "slime_gel", Name: "Slime Gel", MaxStack: 10,
	}})
	if err != nil {
		t.Fatal(err)
	}
	manager := New(areas, items, enemies)
	defer manager.Close()
	session := manager.Join(Player{
		ID: 1, Name: "Aria", AreaID: "meadow", X: 1, Y: 1, Attack: 1,
	})
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
	if len(snapshot.Enemies) != 0 || len(snapshot.Drops) != 1 {
		t.Fatalf("enemy death snapshot = enemies %#v, drops %#v", snapshot.Enemies, snapshot.Drops)
	}
	if len(snapshot.Players) != 1 ||
		snapshot.Players[0].Level != 2 ||
		snapshot.Players[0].Experience != 25 ||
		snapshot.Players[0].SkillPoints != 1 {
		t.Fatalf("enemy experience was not awarded: %#v", snapshot.Players)
	}
	combatEvent := receiveEvent(t, session.Events)
	if combatEvent.Kind != EventCombat || combatEvent.Message != "Defeated Slime" {
		t.Fatalf("combat event = %#v", combatEvent)
	}
	xpEvent := receiveEvent(t, session.Events)
	if xpEvent.Kind != EventProgression || xpEvent.Message != "Gained 125 XP" {
		t.Fatalf("XP event = %#v", xpEvent)
	}
	levelEvent := receiveEvent(t, session.Events)
	if levelEvent.Kind != EventProgression ||
		levelEvent.Message != "Level up! Reached level 2" {
		t.Fatalf("level event = %#v", levelEvent)
	}
	pointEvent := receiveEvent(t, session.Events)
	if pointEvent.Kind != EventProgression ||
		pointEvent.Message != "Gained 1 skill point" {
		t.Fatalf("skill-point event = %#v", pointEvent)
	}
	dropID := snapshot.Drops[0].ID
	if result := manager.Pickup(1, "wrong token"); result.Found {
		t.Fatalf("unauthenticated pickup succeeded: %#v", result)
	}
	pickedUp := manager.Pickup(1, session.Token)
	if !pickedUp.Found || pickedUp.Item.ID != dropID || pickedUp.Item.ItemID != "slime_gel" {
		t.Fatalf("pickup = %#v", pickedUp)
	}
	snapshot = receiveSnapshot(t, session.Updates)
	if len(snapshot.Drops) != 0 {
		t.Fatalf("picked-up drop remains in snapshot: %#v", snapshot.Drops)
	}
}

func TestExperienceCurveSupportsMultipleLevels(t *testing.T) {
	if got := ExperienceToNextLevel(1); got != 100 {
		t.Fatalf("level 1 requirement = %d, want 100", got)
	}
	if got := ExperienceToNextLevel(4); got != 1600 {
		t.Fatalf("level 4 requirement = %d, want 1600", got)
	}
	player := Player{Level: 1}
	grantExperience(&player, 750)
	if player.Level != 3 || player.Experience != 250 || player.SkillPoints != 2 {
		t.Fatalf("progress after 750 XP = level %d, XP %d, SP %d",
			player.Level, player.Experience, player.SkillPoints)
	}
}

func TestSkillPointsUpgradePlayerAttributes(t *testing.T) {
	manager := New(testAreas(t), nil, nil)
	defer manager.Close()
	session := manager.Join(Player{
		ID: 1, Name: "Aria", AreaID: "meadow", X: 1, Y: 1,
		Level: 4, SkillPoints: 3,
	})
	receiveSnapshot(t, session.Updates)

	if invalid := manager.SpendSkillPoint(1, session.Token, "unknown"); invalid.ID != 0 {
		t.Fatalf("unknown skill was accepted: %#v", invalid)
	}
	attack := manager.SpendSkillPoint(1, session.Token, "attack")
	if attack.Attack != 1 || attack.SkillPoints != 2 {
		t.Fatalf("attack upgrade = %#v", attack)
	}
	defense := manager.SpendSkillPoint(1, session.Token, "defense")
	if defense.Defense != 1 || defense.SkillPoints != 1 {
		t.Fatalf("defense upgrade = %#v", defense)
	}
	vitality := manager.SpendSkillPoint(1, session.Token, "vitality")
	if vitality.Vitality != 1 || vitality.SkillPoints != 0 ||
		vitality.Health != 15 || vitality.MaxHealth != 15 {
		t.Fatalf("vitality upgrade = %#v", vitality)
	}
	if exhausted := manager.SpendSkillPoint(1, session.Token, "attack"); exhausted.ID != 0 {
		t.Fatalf("upgrade without points was accepted: %#v", exhausted)
	}
	select {
	case event := <-session.Events:
		t.Fatalf("spending a skill point emitted an event: %#v", event)
	default:
	}
}

func TestGlobalChatBroadcastsAndPreloadsLatestTenMessages(t *testing.T) {
	manager := New(testAreas(t), nil, nil)
	defer manager.Close()
	first := manager.Join(Player{ID: 1, Name: "Aria", AreaID: "meadow", X: 1, Y: 1})
	second := manager.Join(Player{ID: 2, Name: "Rowan", AreaID: "cavern", X: 1, Y: 1})
	receiveSnapshot(t, first.Updates)
	receiveSnapshot(t, second.Updates)

	if manager.Chat(1, "wrong token", "nope") {
		t.Fatal("unauthenticated chat message was accepted")
	}
	if !manager.Chat(1, first.Token, "hello realm") {
		t.Fatal("valid chat message was rejected")
	}
	for _, session := range []Session{first, second} {
		message := receiveChat(t, session.Chats)
		if message.PlayerName != "Aria" || message.Message != "hello realm" {
			t.Fatalf("broadcast chat = %#v", message)
		}
	}
	for index := 0; index < 11; index++ {
		if !manager.Chat(2, second.Token, fmt.Sprintf("message-%02d", index)) {
			t.Fatal("valid chat message was rejected")
		}
	}
	third := manager.Join(Player{ID: 3, Name: "Mira", AreaID: "meadow", X: 1, Y: 1})
	receiveSnapshot(t, third.Updates)
	for index := 1; index < 11; index++ {
		message := receiveChat(t, third.Chats)
		expected := fmt.Sprintf("message-%02d", index)
		if message.Message != expected {
			t.Fatalf("history message %d = %q, want %q", index, message.Message, expected)
		}
	}
}

func TestEnemyPursuesAndAttacksNearestPlayer(t *testing.T) {
	areas, err := NewAreas([]AreaDefinition{{
		ID: "meadow", Name: "Meadow",
		Layout: []string{"#######", "#.....#", "#######"},
		Spawn:  Point{X: 1, Y: 1},
		EnemySpawns: []EnemySpawn{{
			EnemyID: "slime", X: 1, Y: 1, Width: 5, Height: 1,
			MaxEnemies: 1, RespawnSeconds: 10,
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := areas.SetDefaultSpawn("meadow", Point{X: 4, Y: 1}); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{areas: areas}
	player := &activePlayer{Player: Player{
		ID: 1, AreaID: "meadow", X: 1, Y: 1,
		Health: 10, MaxHealth: 10, Defense: 1,
	}, events: make(chan Event, 10)}
	players := map[int64]*activePlayer{1: player}
	live := map[uint64]*Enemy{1: {
		ID: 1, Name: "Cave Bat", AreaID: "meadow", X: 5, Y: 1,
		Damage: 2, spawnIndex: 0,
	}}
	now := time.Now()

	for expectedX := 4; expectedX >= 2; expectedX-- {
		if !manager.updateEnemies(players, live, now) {
			t.Fatal("enemy pursuit did not change world state")
		}
		if live[1].X != expectedX || player.Health != 10 {
			t.Fatalf("pursuit state = enemy x %d, player health %d", live[1].X, player.Health)
		}
	}
	manager.updateEnemies(players, live, now)
	if player.Health != 9 {
		t.Fatalf("first enemy attack left player health at %d, want 9", player.Health)
	}
	manager.updateEnemies(players, live, now.Add(time.Second))
	if player.Health != 9 {
		t.Fatalf("enemy ignored attack cooldown; health = %d", player.Health)
	}
	manager.updateEnemies(players, live, now.Add(enemyAttackInterval))
	if player.Health != 8 {
		t.Fatalf("second enemy attack left player health at %d, want 8", player.Health)
	}
	player.Health = 1
	player.X = 3
	manager.updateEnemies(players, live, now.Add(2*enemyAttackInterval))
	if player.Health != playerMaxHealth || player.MaxHealth != playerMaxHealth ||
		player.AreaID != "meadow" || player.X != 4 || player.Y != 1 {
		t.Fatalf("dead player did not respawn at full health: %#v", player.Player)
	}
	expectedEvents := []Event{
		{Kind: EventDamage, Message: "Took 1 damage from Cave Bat"},
		{Kind: EventDamage, Message: "Took 1 damage from Cave Bat"},
		{Kind: EventDamage, Message: "Took 1 damage from Cave Bat"},
		{Kind: EventDeath, Message: "You were defeated"},
		{Kind: EventRespawn, Message: "Respawned in Meadow"},
	}
	for index, expected := range expectedEvents {
		if actual := <-player.events; actual != expected {
			t.Fatalf("event %d = %#v, want %#v", index, actual, expected)
		}
	}
}

func TestPeacefulEnemyDoesNotAggroOrAttackPlayer(t *testing.T) {
	areas, err := NewAreas([]AreaDefinition{{
		ID: "meadow", Name: "Meadow",
		Layout: []string{"#######", "#.....#", "#######"},
		Spawn:  Point{X: 1, Y: 1},
		EnemySpawns: []EnemySpawn{{
			EnemyID: "deer", X: 1, Y: 1, Width: 5, Height: 1,
			MaxEnemies: 1, RespawnSeconds: 10,
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	manager := &Manager{areas: areas}
	player := &activePlayer{Player: Player{
		ID: 1, AreaID: "meadow", X: 1, Y: 1, Health: 10, MaxHealth: 10,
	}}
	live := map[uint64]*Enemy{1: {
		ID: 1, AreaID: "meadow", X: 2, Y: 1, Damage: 0, spawnIndex: 0,
	}}
	for tick := 0; tick < 20; tick++ {
		manager.updateEnemies(
			map[int64]*activePlayer{1: player}, live,
			time.Now().Add(time.Duration(tick)*enemyAttackInterval),
		)
	}
	if player.Health != 10 {
		t.Fatalf("peaceful enemy damaged player; health = %d", player.Health)
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

func receiveEvent(t *testing.T, events <-chan Event) Event {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
		return Event{}
	}
}

func receiveChat(t *testing.T, messages <-chan ChatMessage) ChatMessage {
	t.Helper()
	select {
	case message := <-messages:
		return message
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for chat message")
		return ChatMessage{}
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
