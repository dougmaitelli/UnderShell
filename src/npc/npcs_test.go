package npc

import (
	"testing"

	"sshrpg/src/item"
	"sshrpg/src/quest"
)

func TestShopDefinitionValidatesItemReferences(t *testing.T) {
	definitions := []Definition{{
		ID: "merchant", Name: "Mira", Type: TypeShop, X: 2, Y: 3,
		Stock: []ShopItem{{
			ItemID: "potion", BuyPrice: 10, SellPrice: 5,
		}},
	}}
	if err := Validate(definitions); err != nil {
		t.Fatal(err)
	}
	items, err := item.NewItems([]item.Definition{{
		ID: "potion", Name: "Potion", Description: "Restores health.", MaxStack: 10,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := ResolveItems(definitions, items); err != nil {
		t.Fatal(err)
	}
	canonical, _ := items.Item("potion")
	if definitions[0].Stock[0].Item != canonical {
		t.Fatalf("unexpected stock: %#v", definitions[0].Stock[0])
	}
}

func TestShopDefinitionRejectsInvalidEconomy(t *testing.T) {
	definitions := []Definition{{
		ID: "merchant", Name: "Mira", Type: TypeShop,
		Stock: []ShopItem{{
			ItemID: "potion", BuyPrice: 5, SellPrice: 10,
		}},
	}}
	if err := Validate(definitions); err == nil {
		t.Fatal("expected sell price above buy price to fail")
	}
}

func TestQuestGiverResolvesConfiguredQuests(t *testing.T) {
	definitions := []Definition{{
		ID: "orin", Name: "Orin", Type: TypeQuestGiver,
		QuestIDs: []string{"slime_supplies"},
	}}
	if err := Validate(definitions); err != nil {
		t.Fatal(err)
	}
	quests, err := quest.NewQuests([]quest.Definition{{
		ID: "slime_supplies", Name: "Slime Supplies",
		Objective: quest.Objective{ItemID: "slime_gel", Quantity: 5},
		Dialogue: quest.Dialogue{
			Offer: "Please help.", InProgress: "Keep looking.",
			Ready: "You found them.", Completed: "Thank you.",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := ResolveQuests(definitions, quests); err != nil {
		t.Fatal(err)
	}
}

func TestCloneCopiesStockStorage(t *testing.T) {
	original := []Definition{{
		ID:    "merchant",
		Stock: []ShopItem{{ItemID: "potion"}},
	}}
	cloned := Clone(original)
	cloned[0].Stock[0].ItemID = "sword"
	if original[0].Stock[0].ItemID != "potion" {
		t.Fatal("clone shares stock storage with original")
	}
}
