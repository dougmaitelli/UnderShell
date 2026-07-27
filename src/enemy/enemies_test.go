package enemy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBundledEnemies(t *testing.T) {
	enemies, err := LoadEnemies(filepath.Join("..", "..", "enemies", "enemies.json"))
	if err != nil {
		t.Fatal(err)
	}
	if enemies.Len() < 2 {
		t.Fatalf("enemy count = %d, want at least 2", enemies.Len())
	}
	if slime, ok := enemies.Enemy("slime"); !ok || len(slime.Visual) != 2 {
		t.Fatalf("unexpected slime definition: %#v, %v", slime, ok)
	}
}

func TestLoadEnemiesRejectsInvalidVisualAndUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enemies.json")
	if err := os.WriteFile(path, []byte(`{
		"enemies":[{"id":"slime","name":"Slime","description":"","visual":[""]}]
	}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEnemies(path); err == nil {
		t.Fatal("expected invalid visual to fail")
	}

	if err := os.WriteFile(path, []byte(`{"enemies":[],"extra":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEnemies(path); err == nil {
		t.Fatal("expected unknown field to fail")
	}
}
