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
			if _, err := db.NewCreateTable().
				Model((*entity.CharacterEquipment)(nil)).
				IfNotExists().
				ForeignKey(
					"(character_id, inventory_slot) REFERENCES inventory_items (character_id, slot) ON DELETE CASCADE",
				).
				Exec(ctx); err != nil {
				return fmt.Errorf("create character equipment: %w", err)
			}
			return nil
		},
		nil,
	)
}
