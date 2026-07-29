package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			if _, err := db.ExecContext(ctx, `
				CREATE TABLE characters_with_moderator (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					key_fingerprint TEXT NOT NULL UNIQUE,
					public_key_type TEXT NOT NULL,
					public_key TEXT NOT NULL,
					name TEXT COLLATE NOCASE NOT NULL UNIQUE,
					role TEXT NOT NULL DEFAULT 'user'
						CHECK (role IN ('user', 'moderator', 'admin')),
					created_at TEXT NOT NULL,
					last_seen_at TEXT NOT NULL
				)
			`); err != nil {
				return fmt.Errorf("create moderator-aware characters table: %w", err)
			}
			if _, err := db.ExecContext(ctx, `
				INSERT INTO characters_with_moderator (
					id, key_fingerprint, public_key_type, public_key,
					name, role, created_at, last_seen_at
				)
				SELECT
					id, key_fingerprint, public_key_type, public_key,
					name, role, created_at, last_seen_at
				FROM characters
			`); err != nil {
				return fmt.Errorf("copy characters for moderator role: %w", err)
			}
			if _, err := db.ExecContext(ctx, "DROP TABLE characters"); err != nil {
				return fmt.Errorf("replace characters for moderator role: %w", err)
			}
			if _, err := db.ExecContext(
				ctx,
				"ALTER TABLE characters_with_moderator RENAME TO characters",
			); err != nil {
				return fmt.Errorf("rename moderator-aware characters table: %w", err)
			}
			return nil
		},
		nil,
	)
}
