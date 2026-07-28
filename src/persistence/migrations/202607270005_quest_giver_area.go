package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(
		// Reserved for compatibility with development databases that applied
		// the earlier denormalized giver-area migration. The following
		// migration removes those copied fields.
		func(ctx context.Context, db *bun.DB) error {
			return nil
		},
		nil,
	)
}
