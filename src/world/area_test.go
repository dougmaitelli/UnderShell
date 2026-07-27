package world

import (
	"os"
	"path/filepath"
	"testing"

	"sshrpg/src/enemy"
)

func TestLoadAreasValidatesInterAreaWaypoints(t *testing.T) {
	directory := t.TempDir()
	writeArea(t, directory, "one.json", `{
		"id":"one","name":"One","layout":["###","#.#","###"],
		"spawn":{"x":1,"y":1},
		"waypoints":[{"x":1,"y":1,"destination_area":"two","destination_x":1,"destination_y":1}]
	}`)
	writeArea(t, directory, "two.json", `{
		"id":"two","name":"Two","layout":["###","#.#","###"],
		"spawn":{"x":1,"y":1},"waypoints":[]
	}`)

	areas, err := LoadAreas(directory)
	if err != nil {
		t.Fatal(err)
	}
	one, ok := areas.Area("one")
	if !ok || one.Name != "One" || one.Width != 3 || one.Height != 3 {
		t.Fatalf("unexpected area: %#v", one)
	}
}

func TestLoadAreasRejectsUnknownDestination(t *testing.T) {
	directory := t.TempDir()
	writeArea(t, directory, "one.json", `{
		"id":"one","name":"One","layout":["###","#.#","###"],
		"spawn":{"x":1,"y":1},
		"waypoints":[{"x":1,"y":1,"destination_area":"missing","destination_x":1,"destination_y":1}]
	}`)
	if _, err := LoadAreas(directory); err == nil {
		t.Fatal("expected unknown waypoint destination to fail validation")
	}
}

func TestBundledAreasAreValid(t *testing.T) {
	areas, err := LoadAreas(filepath.Join("..", "..", "maps"))
	if err != nil {
		t.Fatal(err)
	}
	meadow, ok := areas.Area("meadow")
	if !ok {
		t.Fatal("bundled meadow area is missing")
	}
	if meadow.Width != 192 || meadow.Height != 64 {
		t.Fatalf("meadow size = %dx%d, want 192x64", meadow.Width, meadow.Height)
	}
	if _, ok := meadow.Waypoint(Point{X: 189, Y: 32}); !ok {
		t.Fatal("meadow waypoint does not cover its 3x3 center tile")
	}
	cavern, ok := areas.Area("cavern")
	if !ok {
		t.Fatal("bundled cavern area is missing")
	}
	if cavern.Width != 192 || cavern.Height != 64 {
		t.Fatalf("cavern size = %dx%d, want 192x64", cavern.Width, cavern.Height)
	}
	if _, ok := cavern.Waypoint(Point{X: 2, Y: 32}); !ok {
		t.Fatal("cavern waypoint does not cover its 3x3 center tile")
	}
	enemies, err := enemy.LoadEnemies(filepath.Join("..", "..", "enemies", "enemies.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := areas.ValidateEnemySpawns(enemies); err != nil {
		t.Fatal(err)
	}
}

func TestEnemySpawnValidation(t *testing.T) {
	areas, err := NewAreas([]AreaDefinition{{
		ID: "one", Name: "One", Layout: []string{"#####", "#...#", "#####"},
		Spawn: Point{X: 1, Y: 1},
		EnemySpawns: []EnemySpawn{{
			EnemyID: "missing", X: 1, Y: 1, Width: 3, Height: 1,
			MaxEnemies: 2, RespawnSeconds: 5,
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
	if err := areas.ValidateEnemySpawns(enemies); err == nil {
		t.Fatal("expected unknown enemy reference to fail")
	}
}

func TestDefaultSpawnMustBeWalkableInKnownArea(t *testing.T) {
	areas, err := NewAreas([]AreaDefinition{{
		ID: "one", Name: "One", Layout: []string{"###", "#.#", "###"},
		Spawn: Point{X: 1, Y: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := areas.SetDefaultSpawn("missing", Point{X: 1, Y: 1}); err == nil {
		t.Fatal("expected unknown default spawn area to fail")
	}
	if err := areas.SetDefaultSpawn("one", Point{X: 0, Y: 0}); err == nil {
		t.Fatal("expected blocked default spawn point to fail")
	}
	if err := areas.SetDefaultSpawn("one", Point{X: 1, Y: 1}); err != nil {
		t.Fatal(err)
	}
	area, point := areas.DefaultSpawn()
	if area.ID != "one" || point != (Point{X: 1, Y: 1}) {
		t.Fatalf("unexpected default spawn: %s %#v", area.ID, point)
	}
}

func writeArea(t *testing.T, directory, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}
