package item

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.json")
	if err := os.WriteFile(path, []byte(`{
		"items": [
			{"id":"health_potion","name":"Health Potion","description":"Restores health.","max_stack":10}
		]
	}`), 0600); err != nil {
		t.Fatal(err)
	}

	catalog, err := LoadCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := catalog.Item("health_potion")
	if !ok || definition.Name != "Health Potion" || definition.MaxStack != 10 {
		t.Fatalf("unexpected item definition: %#v", definition)
	}
}

func TestCatalogRejectsDuplicateIDs(t *testing.T) {
	_, err := NewCatalog([]Definition{
		{ID: "potion", Name: "Potion", MaxStack: 10},
		{ID: "potion", Name: "Other Potion", MaxStack: 10},
	})
	if err == nil {
		t.Fatal("expected duplicate item ID to fail validation")
	}
}

func TestBundledCatalogIsValid(t *testing.T) {
	catalog, err := LoadCatalog(filepath.Join("..", "..", "items", "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Len() == 0 {
		t.Fatal("bundled item catalog is empty")
	}
}
