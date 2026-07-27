package entity

import "github.com/uptrace/bun"

// CharacterLocation stores the current world location separately from account
// identity so area persistence can evolve independently.
type CharacterLocation struct {
	bun.BaseModel `bun:"table:character_locations,alias:character_location"`

	CharacterID int64  `bun:"character_id,pk"`
	AreaID      string `bun:"area_id,notnull"`
	X           int    `bun:"x,notnull"`
	Y           int    `bun:"y,notnull"`
	UpdatedAt   string `bun:"updated_at,notnull"`
}
