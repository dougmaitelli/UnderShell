package enemy

import (
	"os"
	"path/filepath"
	"testing"

	"sshrpg/src/item"
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
	items, err := item.LoadItems(filepath.Join("..", "..", "items", "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := enemies.ValidateDrops(items); err != nil {
		t.Fatal(err)
	}
}

func TestLoadEnemiesRejectsInvalidVisualAndUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enemies.json")
	if err := os.WriteFile(path, []byte(`{
		"enemies":[{"id":"slime","name":"Slime","description":"","health":1,"visual":[""]}]
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

func TestEnemiesAllowPeacefulButRejectNegativeDamage(t *testing.T) {
	if _, err := NewEnemies([]Definition{{
		ID: "deer", Name: "Deer", Health: 2, Damage: 0, Visual: []string{"d"},
	}}); err != nil {
		t.Fatalf("peaceful enemy was rejected: %v", err)
	}
	if _, err := NewEnemies([]Definition{{
		ID: "broken", Name: "Broken", Health: 2, Damage: -1, Visual: []string{"b"},
	}}); err == nil {
		t.Fatal("expected negative damage to fail")
	}
}
