package npc

import (
	"testing"

	"sshrpg/src/item"
	"sshrpg/src/quest"
)

func TestShopDefinitionValidatesItemReferences(t *testing.T) {
	configs := []Config{{
		ID: "merchant", Name: "Mira", Type: TypeShop, X: 2, Y: 3,
		Stock: []ShopItemConfig{{
			ItemID: "potion", BuyPrice: 10,
		}},
	}}
	items, err := item.NewItems([]item.Definition{{
		ID: "potion", Name: "Potion", Description: "Restores health.",
		Type: item.TypeConsumable,
		Effects: []item.Effect{{
			Type: item.EffectRestoreHealth, Amount: 5,
		}},
		MaxStack: 10,
	}})
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := Resolve(configs, items, nil)
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := items.Item("potion")
	if definitions[0].Stock[0].Item != canonical {
		t.Fatalf("unexpected stock: %#v", definitions[0].Stock[0])
	}
}

func TestShopDefinitionRejectsInvalidEconomy(t *testing.T) {
	configs := []Config{{
		ID: "merchant", Name: "Mira", Type: TypeShop,
		Stock: []ShopItemConfig{{
			ItemID: "ore", BuyPrice: 5,
		}},
	}}
	items, err := item.NewItems([]item.Definition{{
		ID: "ore", Name: "Ore", Type: item.TypeMaterial,
		SellPrice: 10, MaxStack: 20,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(configs, items, nil); err == nil {
		t.Fatal("expected buy price below item sell price to fail")
	}
}

func TestQuestGiverResolvesConfiguredQuests(t *testing.T) {
	configs := []Config{{
		ID: "orin", Name: "Orin", Type: TypeQuestGiver,
		QuestIDs: []string{"slime_supplies"},
	}}
	itemDefinition := &item.Definition{
		ID: "slime_gel", Name: "Slime Gel",
		Type: item.TypeMaterial, MaxStack: 50,
	}
	quests, err := quest.NewQuests([]quest.Definition{{
		ID: "slime_supplies", Name: "Slime Supplies",
		Objective: quest.Objective{Item: itemDefinition, Quantity: 5},
		Dialogue: quest.Dialogue{
			Offer: "Please help.", InProgress: "Keep looking.",
			Ready: "You found them.", Completed: "Thank you.",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := Resolve(configs, nil, quests)
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := quests.Quest("slime_supplies")
	if definitions[0].Quests[0] != canonical {
		t.Fatal("quest giver did not retain canonical quest reference")
	}
}

func TestCloneCopiesStockStorage(t *testing.T) {
	original := []Definition{{
		ID:    "merchant",
		Stock: []ShopItem{{Item: &item.Definition{ID: "potion"}}},
	}}
	cloned := Clone(original)
	cloned[0].Stock[0].Item = &item.Definition{ID: "sword"}
	if original[0].Stock[0].Item.ID != "potion" {
		t.Fatal("clone shares stock storage with original")
	}
}
