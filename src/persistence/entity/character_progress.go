package entity

import "github.com/uptrace/bun"

// CharacterProgress stores persistent leveling state independently of identity.
type CharacterProgress struct {
	bun.BaseModel `bun:"table:character_progress,alias:character_progress"`

	CharacterID int64 `bun:"character_id,pk"`
	Level       int   `bun:"level,notnull"`
	Experience  int64 `bun:"experience,notnull"`
	SkillPoints int   `bun:"skill_points,notnull"`
}
