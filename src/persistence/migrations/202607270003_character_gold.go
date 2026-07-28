package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addProgressColumn(
				ctx,
				db,
				"gold",
				"INTEGER NOT NULL DEFAULT 100",
			)
		},
		nil,
	)
}
