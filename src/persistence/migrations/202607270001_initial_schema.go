package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"sshrpg/src/persistence/entity"
)

// characterProgressV1 preserves the original progress schema. Later migrations
// add attributes and gold, so a fresh database follows the same history as an
// existing save.
type characterProgressV1 struct {
	bun.BaseModel `bun:"table:character_progress,alias:character_progress"`

	CharacterID int64 `bun:"character_id,pk"`
	Level       int   `bun:"level,notnull"`
	Experience  int64 `bun:"experience,notnull"`
	SkillPoints int   `bun:"skill_points,notnull"`
}

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			models := []any{
				(*entity.Character)(nil),
				(*entity.CharacterLocation)(nil),
				(*characterProgressV1)(nil),
				(*entity.Inventory)(nil),
				(*entity.InventoryItem)(nil),
			}
			for _, model := range models {
				if _, err := db.NewCreateTable().
					Model(model).
					IfNotExists().
					Exec(ctx); err != nil {
					return fmt.Errorf("create initial schema: %w", err)
				}
			}
			return nil
		},
		nil,
	)
}
