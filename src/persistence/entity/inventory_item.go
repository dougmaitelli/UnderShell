package entity

import "github.com/uptrace/bun"

// InventoryItem stores an item stack in a stable inventory slot.
type InventoryItem struct {
	bun.BaseModel `bun:"table:inventory_items,alias:inventory_item"`

	CharacterID int64  `bun:"character_id,pk"`
	Slot        int    `bun:"slot,pk"`
	ItemKey     string `bun:"item_key,notnull"`
	Quantity    int    `bun:"quantity,notnull"`
}
