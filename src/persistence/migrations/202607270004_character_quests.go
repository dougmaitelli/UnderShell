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
				Model((*entity.CharacterQuest)(nil)).
				IfNotExists().
				Exec(ctx); err != nil {
				return fmt.Errorf("create character quest schema: %w", err)
			}
			return nil
		},
		nil,
	)
}
