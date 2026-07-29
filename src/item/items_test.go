package item

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadItems(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "health-potion.json")
	if err := os.WriteFile(path, []byte(`{
		"id":"health_potion",
		"name":"Health Potion",
		"description":"Restores health.",
		"type":"consumable",
		"effects":[{"type":"restore_health","amount":5}],
		"max_stack":10
	}`), 0600); err != nil {
		t.Fatal(err)
	}

	items, err := LoadItems(directory)
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
		{
			ID: "potion", Name: "Potion", Type: TypeConsumable,
			Effects:  []Effect{{Type: EffectRestoreHealth, Amount: 5}},
			MaxStack: 10,
		},
		{
			ID: "potion", Name: "Other Potion", Type: TypeConsumable,
			Effects:  []Effect{{Type: EffectRestoreHealth, Amount: 5}},
			MaxStack: 10,
		},
	})
	if err == nil {
		t.Fatal("expected duplicate item ID to fail validation")
	}
}

func TestConsumablesRequireValidEffects(t *testing.T) {
	for name, definition := range map[string]Definition{
		"missing effect": {
			ID: "potion", Name: "Potion",
			Type: TypeConsumable, MaxStack: 10,
		},
		"unsupported effect": {
			ID: "potion", Name: "Potion", Type: TypeConsumable,
			Effects:  []Effect{{Type: "invisibility", Amount: 1}},
			MaxStack: 10,
		},
		"zero amount": {
			ID: "potion", Name: "Potion", Type: TypeConsumable,
			Effects:  []Effect{{Type: EffectRestoreHealth}},
			MaxStack: 10,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewItems([]Definition{definition}); err == nil {
				t.Fatal("expected invalid consumable effect to fail")
			}
		})
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

func TestEquipmentStatsMustBeNonNegativeAndEquipmentOnly(t *testing.T) {
	valid, err := NewItems([]Definition{{
		ID: "sword", Name: "Sword", Type: TypeEquipment,
		EquipmentSlot: SlotWeapon,
		Stats: EquipmentStats{
			Attack: 2, Defense: 1, Vitality: 1,
		},
		MaxStack: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	sword, _ := valid.Item("sword")
	if sword.Stats.Attack != 2 ||
		sword.Stats.Defense != 1 ||
		sword.Stats.Vitality != 1 {
		t.Fatalf("equipment stats = %#v", sword.Stats)
	}

	for name, definition := range map[string]Definition{
		"negative equipment stat": {
			ID: "sword", Name: "Sword", Type: TypeEquipment,
			EquipmentSlot: SlotWeapon,
			Stats:         EquipmentStats{Attack: -1},
			MaxStack:      1,
		},
		"stats on material": {
			ID: "ore", Name: "Ore", Type: TypeMaterial,
			Stats: EquipmentStats{Defense: 1}, MaxStack: 10,
		},
		"stats on consumable": {
			ID: "potion", Name: "Potion", Type: TypeConsumable,
			Stats: EquipmentStats{Vitality: 1},
			Effects: []Effect{{
				Type: EffectRestoreHealth, Amount: 5,
			}},
			MaxStack: 10,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewItems([]Definition{definition}); err == nil {
				t.Fatal("expected invalid item stats to fail")
			}
		})
	}
}

func TestBundledItemsAreValid(t *testing.T) {
	items, err := LoadItems(filepath.Join("..", "..", "content", "items"))
	if err != nil {
		t.Fatal(err)
	}
	if items.Len() == 0 {
		t.Fatal("bundled item definitions is empty")
	}
	sword, ok := items.Item("rusty_sword")
	if !ok || sword.Stats.Attack != 1 {
		t.Fatalf("rusty sword stats = %#v", sword)
	}
}
