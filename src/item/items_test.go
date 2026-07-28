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
			{"id":"health_potion","name":"Health Potion","description":"Restores health.","type":"consumable","max_stack":10}
		]
	}`), 0600); err != nil {
		t.Fatal(err)
	}

	items, err := LoadItems(path)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := items.Item("health_potion")
	if !ok || definition.Name != "Health Potion" ||
		definition.Type != TypeConsumable || definition.MaxStack != 10 {
		t.Fatalf("unexpected item definition: %#v", definition)
	}
	again, _ := items.Item("health_potion")
	if definition != again {
		t.Fatal("item lookup did not return the stable canonical definition")
	}
}

func TestItemsRejectDuplicateIDs(t *testing.T) {
	_, err := NewItems([]Definition{
		{ID: "potion", Name: "Potion", Type: TypeConsumable, MaxStack: 10},
		{ID: "potion", Name: "Other Potion", Type: TypeConsumable, MaxStack: 10},
	})
	if err == nil {
		t.Fatal("expected duplicate item ID to fail validation")
	}
}

func TestEquipmentRequiresSupportedSlotAndSingleStack(t *testing.T) {
	for _, slot := range []EquipmentSlot{
		SlotHelmet, SlotWeapon, SlotArmor,
		SlotBoots, SlotGloves, SlotLegs,
	} {
		_, err := NewItems([]Definition{{
			ID: "equipment", Name: "Equipment",
			Type: TypeEquipment, EquipmentSlot: slot, MaxStack: 1,
		}})
		if err != nil {
			t.Fatalf("slot %q was rejected: %v", slot, err)
		}
	}
	for name, definition := range map[string]Definition{
		"missing slot": {
			ID: "equipment", Name: "Equipment",
			Type: TypeEquipment, MaxStack: 1,
		},
		"unsupported slot": {
			ID: "equipment", Name: "Equipment", Type: TypeEquipment,
			EquipmentSlot: "cape", MaxStack: 1,
		},
		"stacked equipment": {
			ID: "equipment", Name: "Equipment", Type: TypeEquipment,
			EquipmentSlot: SlotArmor, MaxStack: 2,
		},
		"slot on consumable": {
			ID: "potion", Name: "Potion", Type: TypeConsumable,
			EquipmentSlot: SlotHelmet, MaxStack: 10,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewItems([]Definition{definition}); err == nil {
				t.Fatal("expected invalid item definition to fail")
			}
		})
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
