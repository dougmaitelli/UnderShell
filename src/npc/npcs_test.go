package npc

import (
	"testing"

	"sshrpg/src/item"
)

func TestShopDefinitionValidationAndItemResolution(t *testing.T) {
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
	if definitions[0].Stock[0].Name != "Potion" ||
		definitions[0].Stock[0].MaxStack != 10 {
		t.Fatalf("unresolved stock: %#v", definitions[0].Stock[0])
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
