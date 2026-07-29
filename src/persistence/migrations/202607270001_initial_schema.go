package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"sshrpg/src/persistence/entity"
)

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			models := []any{
				(*entity.Character)(nil),
				(*entity.CharacterLocation)(nil),
				(*entity.CharacterProgress)(nil),
				(*entity.Inventory)(nil),
				(*entity.InventoryItem)(nil),
				(*entity.CharacterQuest)(nil),
			}
			for _, model := range models {
				if _, err := db.NewCreateTable().
					Model(model).
					IfNotExists().
					Exec(ctx); err != nil {
					return fmt.Errorf("create initial schema: %w", err)
				}
			}
			if _, err := db.NewCreateTable().
				Model((*entity.CharacterEquipment)(nil)).
				IfNotExists().
				ForeignKey(
					"(character_id, inventory_slot) REFERENCES inventory_items (character_id, slot) ON DELETE CASCADE",
				).
				Exec(ctx); err != nil {
				return fmt.Errorf("create initial character equipment schema: %w", err)
			}
			if _, err := db.NewCreateIndex().
				Index("characters_name_ci_unique").
				Table("characters").
				ColumnExpr("LOWER(name)").
				Unique().
				IfNotExists().
				Exec(ctx); err != nil {
				return fmt.Errorf(
					"create case-insensitive character name index: %w",
					err,
				)
			}
			return nil
		},
		nil,
	)
}
