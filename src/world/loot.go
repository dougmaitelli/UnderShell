package world

import (
	mathrand "math/rand/v2"
)

type lootSystem struct {
	live   map[uint64]*GroundItem
	nextID uint64
}

func (item *GroundItem) isWithinPickupRange(player *activePlayer) bool {
	return player.isWithin(item.AreaID, item.X, item.Y, pickupRange)
}

func newLootSystem() lootSystem {
	return lootSystem{live: make(map[uint64]*GroundItem)}
}

func (s *lootSystem) rollDrops(target *Enemy) {
	for _, drop := range target.Definition.Drops {
		if mathrand.Float64() > drop.Chance {
			continue
		}
		if drop.Item == nil {
			continue
		}
		s.nextID++
		s.live[s.nextID] = &GroundItem{
			ID: s.nextID, Item: drop.Item,
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

func (s *lootSystem) restore(
	player *activePlayer,
	item GroundItem,
	broadcast func(),
) bool {
	if player == nil || item.ID == 0 || item.Item == nil {
		return false
	}
	if _, exists := s.live[item.ID]; exists {
		return false
	}
	s.live[item.ID] = &item
	broadcast()
	return true
}
