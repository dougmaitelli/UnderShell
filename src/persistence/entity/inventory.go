package entity

import "github.com/uptrace/bun"

// Inventory is the persistence-owned inventory container for one character.
type Inventory struct {
	bun.BaseModel `bun:"table:inventories,alias:inventory"`

	CharacterID int64  `bun:"character_id,pk"`
	CreatedAt   string `bun:"created_at,notnull"`
}
