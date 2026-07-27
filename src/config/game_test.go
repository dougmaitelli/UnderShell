package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBundledGameConfig(t *testing.T) {
	game, err := LoadGame(filepath.Join("..", "..", "config", "game.json"))
	if err != nil {
		t.Fatal(err)
	}
	if game.DefaultSpawn.AreaID != "meadow" ||
		game.DefaultSpawn.X != 7 ||
		game.DefaultSpawn.Y != 32 {
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
