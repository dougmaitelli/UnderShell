// Package migrations contains the ordered Bun schema migrations.
package migrations

import "github.com/uptrace/bun/migrate"

// Migrations is the schema history applied when the database opens.
var Migrations = migrate.NewMigrations()
