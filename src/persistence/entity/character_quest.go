package entity

import "github.com/uptrace/bun"

// CharacterQuest stores persistent acceptance and completion state.
type CharacterQuest struct {
	bun.BaseModel `bun:"table:character_quests,alias:character_quest"`

	CharacterID int64  `bun:"character_id,pk"`
	QuestID     string `bun:"quest_id,pk"`
	GiverID     string `bun:"giver_id,notnull"`
	Status      string `bun:"status,notnull"`
	AcceptedAt  string `bun:"accepted_at,notnull"`
	CompletedAt string `bun:"completed_at,nullzero"`
}
