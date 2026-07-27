package item

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.json")
	if err := os.WriteFile(path, []byte(`{
		"items": [
			{"id":"health_potion","name":"Health Potion","description":"Restores health.","max_stack":10}
		]
	}`), 0600); err != nil {
		t.Fatal(err)
	}

	items, err := LoadItems(path)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := items.Item("health_potion")
	if !ok || definition.Name != "Health Potion" || definition.MaxStack != 10 {
		t.Fatalf("unexpected item definition: %#v", definition)
	}
}

func TestItemsRejectDuplicateIDs(t *testing.T) {
	_, err := NewItems([]Definition{
		{ID: "potion", Name: "Potion", MaxStack: 10},
		{ID: "potion", Name: "Other Potion", MaxStack: 10},
	})
	if err == nil {
		t.Fatal("expected duplicate item ID to fail validation")
	}
}

func TestBundledItemsAreValid(t *testing.T) {
	items, err := LoadItems(filepath.Join("..", "..", "items", "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	if items.Len() == 0 {
		t.Fatal("bundled item definitions is empty")
	}
}
