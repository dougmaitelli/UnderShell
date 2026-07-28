package entity

import "github.com/uptrace/bun"

// CharacterProgress stores persistent leveling state independently of identity.
type CharacterProgress struct {
	bun.BaseModel `bun:"table:character_progress,alias:character_progress"`

	CharacterID int64 `bun:"character_id,pk"`
	Level       int   `bun:"level,notnull"`
	Experience  int64 `bun:"experience,notnull"`
	SkillPoints int   `bun:"skill_points,notnull"`
	Attack      int   `bun:"attack,notnull,default:0"`
	Defense     int   `bun:"defense,notnull,default:0"`
	Vitality    int   `bun:"vitality,notnull,default:0"`
	Gold        int   `bun:"gold,notnull,default:100"`
}
