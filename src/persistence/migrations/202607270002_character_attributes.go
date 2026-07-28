package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			for _, column := range []string{"attack", "defense", "vitality"} {
				if err := addProgressColumn(ctx, db, column, "INTEGER NOT NULL DEFAULT 0"); err != nil {
					return err
				}
			}
			return nil
		},
		nil,
	)
}

func addProgressColumn(
	ctx context.Context,
	db *bun.DB,
	name string,
	definition string,
) error {
	var count int
	if err := db.NewRaw(
		"SELECT COUNT(*) FROM pragma_table_info('character_progress') WHERE name = ?",
		name,
	).Scan(ctx, &count); err != nil {
		return fmt.Errorf("inspect character progress column %s: %w", name, err)
	}
	if count > 0 {
		return nil
	}

	if _, err := db.NewAddColumn().
		Table("character_progress").
		ColumnExpr(name + " " + definition).
		Exec(ctx); err != nil {
		return fmt.Errorf("add character progress column %s: %w", name, err)
	}
	return nil
}
