package enemy

import (
	"os"
	"path/filepath"
	"testing"

	"sshrpg/src/item"
)

func TestLoadBundledEnemies(t *testing.T) {
	items, err := item.LoadItems(filepath.Join("..", "..", "items", "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	enemies, err := LoadEnemies(
		filepath.Join("..", "..", "enemies", "enemies.json"), items,
	)
	if err != nil {
		t.Fatal(err)
	}
	if enemies.Len() < 2 {
		t.Fatalf("enemy count = %d, want at least 2", enemies.Len())
	}
	if slime, ok := enemies.Enemy("slime"); !ok || len(slime.Visual) != 2 {
		t.Fatalf("unexpected slime definition: %#v, %v", slime, ok)
	}
	first, _ := enemies.Enemy("slime")
	second, _ := enemies.Enemy("slime")
	if first != second {
		t.Fatal("enemy registry did not return a stable canonical reference")
	}
	slime, _ := enemies.Enemy("slime")
	for _, drop := range slime.Drops {
		canonical, _ := items.Item(drop.Item.ID)
		if drop.Item != canonical {
			t.Fatal("enemy drop did not retain its item reference")
		}
	}
}

func TestLoadEnemiesRejectsInvalidVisualAndUnknownFields(t *testing.T) {
	items, err := item.NewItems([]item.Definition{{
		ID: "item", Name: "Item", Type: item.TypeMaterial, MaxStack: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "enemies.json")
	if err := os.WriteFile(path, []byte(`{
		"enemies":[{"id":"slime","name":"Slime","description":"","health":1,"experience":1,"visual":[""]}]
	}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEnemies(path, items); err == nil {
		t.Fatal("expected invalid visual to fail")
	}

	if err := os.WriteFile(path, []byte(`{"enemies":[],"extra":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEnemies(path, items); err == nil {
		t.Fatal("expected unknown field to fail")
	}
}

func TestEnemiesAllowPeacefulButRejectNegativeDamage(t *testing.T) {
	if _, err := NewEnemies([]Definition{{
		ID: "deer", Name: "Deer", Health: 2, Damage: 0, Experience: 1, Visual: []string{"d"},
	}}); err != nil {
		t.Fatalf("peaceful enemy was rejected: %v", err)
	}
	if _, err := NewEnemies([]Definition{{
		ID: "broken", Name: "Broken", Health: 2, Damage: -1, Experience: 1, Visual: []string{"b"},
	}}); err == nil {
		t.Fatal("expected negative damage to fail")
	}
}
