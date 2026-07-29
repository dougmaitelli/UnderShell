package ui

import (
	"testing"

	"sshrpg/src/domain"
	"sshrpg/src/item"
	"sshrpg/src/npc"
	"sshrpg/src/repository"
	"sshrpg/src/world"
)

func TestShopSellsAnyInventoryItemWithAnItemSellPrice(t *testing.T) {
	items, err := item.NewItems([]item.Definition{
		{
			ID: "potion", Name: "Potion",
			Type: item.TypeConsumable, SellPrice: 5,
			Effects:  []item.Effect{{Type: item.EffectRestoreHealth, Amount: 5}},
			MaxStack: 10,
		},
		{
			ID: "slime_gel", Name: "Slime Gel",
			Type: item.TypeMaterial, SellPrice: 2, MaxStack: 50,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	potion, _ := items.Item("potion")
	shop := shopState{npc: &npc.Definition{
		ID: "merchant", Name: "Mira", Type: npc.TypeShop,
		Stock: []npc.ShopItem{{Item: potion, BuyPrice: 10}},
	}}
	inventory := &domain.Inventory{
		CharacterID: 1,
		Items: []domain.InventoryItem{{
			Slot: 1, ItemKey: "slime_gel", Quantity: 4,
		}},
	}
	entries := shop.sellEntries(inventory, items)
	if len(entries) != 1 ||
		entries[0].Name != "Slime Gel" ||
		entries[0].SellPrice != 2 ||
		entries[0].Item.Quantity != 4 {
		t.Fatalf("sell entries = %#v", entries)
	}
}

func TestSuccessfulShopTradesAddEvents(t *testing.T) {
	model := newGameModel(
		Repositories{}, nil, nil, Identity{},
		&domain.Character{ID: 1, Gold: 100},
		&domain.Inventory{CharacterID: 1},
	)
	inventory := &domain.Inventory{CharacterID: 1}

	_, command := model.updateShopTrade(shopTradeMsg{
		result: repository.TradeResult{
			Inventory: inventory, Gold: 90,
		},
		itemName: "Health Potion",
		buying:   true,
	})
	if command == nil {
		t.Fatal("purchase event did not get an expiry command")
	}
	events := model.eventFeed.views()
	if len(events) != 1 ||
		events[0].Kind != world.EventTrade ||
		events[0].Message != "Bought Health Potion" {
		t.Fatalf("purchase events = %#v", events)
	}

	model.shop.tab = shopTabSell
	_, command = model.updateShopTrade(shopTradeMsg{
		result: repository.TradeResult{
			Inventory: inventory, Gold: 95,
		},
		itemName: "Health Potion",
	})
	if command == nil {
		t.Fatal("sale event did not get an expiry command")
	}
	events = model.eventFeed.views()
	if len(events) != 2 ||
		events[1].Kind != world.EventTrade ||
		events[1].Message != "Sold Health Potion" {
		t.Fatalf("sale events = %#v", events)
	}
}
