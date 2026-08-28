package entity

import "github.com/uptrace/bun"

// CharacterProgress stores persistent leveling state independently of identity.
type CharacterProgress struct {
	bun.BaseModel `bun:"table:character_progress,alias:character_progress"`

	CharacterID int64 `bun:"character_id,pk"`
	Level       int   `bun:"level,notnull,type:BIGINT CHECK (level >= 1)"`
	Experience  int64 `bun:"experience,notnull,type:BIGINT CHECK (experience >= 0)"`
	SkillPoints int   `bun:"skill_points,notnull,type:BIGINT CHECK (skill_points >= 0)"`
	Attack      int   `bun:"attack,notnull,default:0,type:BIGINT CHECK (attack >= 0)"`
	Defense     int   `bun:"defense,notnull,default:0,type:BIGINT CHECK (defense >= 0)"`
	Vitality    int   `bun:"vitality,notnull,default:0,type:BIGINT CHECK (vitality >= 0)"`
	Gold        int   `bun:"gold,notnull,default:100,type:BIGINT CHECK (gold >= 0)"`
}
