package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.json")
	if err := os.WriteFile(path, []byte(`{
		"default_spawn":{"area_id":"test_area","x":12,"y":34}
	}`), 0600); err != nil {
		t.Fatal(err)
	}

	game, err := LoadGame(path)
	if err != nil {
		t.Fatal(err)
	}
	if game.DefaultSpawn.AreaID != "test_area" ||
		game.DefaultSpawn.X != 12 ||
		game.DefaultSpawn.Y != 34 {
		t.Fatalf("unexpected default spawn: %#v", game.DefaultSpawn)
	}
}

func TestLoadGameRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.json")
	if err := os.WriteFile(path, []byte(`{
		"default_spawn":{"area_id":"meadow","x":1,"y":1},
		"unknown":true
	}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGame(path); err == nil {
		t.Fatal("expected unknown field to fail")
	}
}
