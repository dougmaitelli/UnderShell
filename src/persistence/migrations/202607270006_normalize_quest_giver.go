package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			for _, column := range []string{"giver_name", "giver_area_id"} {
				var count int
				if err := db.NewRaw(
					"SELECT COUNT(*) FROM pragma_table_info('character_quests') WHERE name = ?",
					column,
				).Scan(ctx, &count); err != nil {
					return fmt.Errorf("inspect character quest column %s: %w", column, err)
				}
				if count == 0 {
					continue
				}
				if _, err := db.NewDropColumn().
					Table("character_quests").
					Column(column).
					Exec(ctx); err != nil {
					return fmt.Errorf("drop character quest column %s: %w", column, err)
				}
			}
			return nil
		},
		nil,
	)
}
