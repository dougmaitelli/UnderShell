package world

import (
	mathrand "math/rand/v2"

	"sshrpg/src/enemy"
	"sshrpg/src/item"
)

type lootSystem struct {
	defs   *item.Items
	live   map[uint64]*GroundItem
	nextID uint64
}

func (item *GroundItem) isWithinPickupRange(player *activePlayer) bool {
	return player.isWithin(item.AreaID, item.X, item.Y, pickupRange)
}

func newLootSystem(definitions *item.Items) lootSystem {
	return lootSystem{defs: definitions, live: make(map[uint64]*GroundItem)}
}

func (s *lootSystem) rollDrops(target *Enemy, definition enemy.Definition) {
	for _, drop := range definition.Drops {
		if mathrand.Float64() > drop.Chance {
			continue
		}
		itemDefinition, ok := s.defs.Item(drop.ItemID)
		if !ok {
			continue
		}
		s.nextID++
		s.live[s.nextID] = &GroundItem{
			ID: s.nextID, Item: itemDefinition,
			AreaID: target.AreaID, X: target.X, Y: target.Y,
		}
	}
}

func (s *lootSystem) pickup(player *activePlayer, broadcast func()) PickupResult {
	result := PickupResult{}
	if player == nil {
		return result
	}
	for id, drop := range s.live {
		if !drop.isWithinPickupRange(player) {
			continue
		}
		result.Item, result.Found = *drop, true
		delete(s.live, id)
		break
	}
	if result.Found {
		broadcast()
	}
	return result
}
