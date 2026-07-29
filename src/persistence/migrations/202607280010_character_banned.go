package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			var count int
			if err := db.NewRaw(
				"SELECT COUNT(*) FROM pragma_table_info('characters') WHERE name = ?",
				"banned",
			).Scan(ctx, &count); err != nil {
				return fmt.Errorf("inspect character banned column: %w", err)
			}
			if count > 0 {
				return nil
			}
			if _, err := db.NewAddColumn().
				Table("characters").
				ColumnExpr("banned BOOLEAN NOT NULL DEFAULT FALSE").
				Exec(ctx); err != nil {
				return fmt.Errorf("add character banned column: %w", err)
			}
			return nil
		},
		nil,
	)
}
