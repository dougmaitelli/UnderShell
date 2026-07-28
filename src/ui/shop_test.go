package ui

import (
	"testing"

	"sshrpg/src/domain"
	"sshrpg/src/repository"
	"sshrpg/src/world"
)

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
