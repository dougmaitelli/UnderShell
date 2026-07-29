package persistence

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"sshrpg/src/persistence/entity"
)

func TestCharacterSchemaFormatsForPostgreSQL(t *testing.T) {
	sqlDB := sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithDSN("postgres://test:test@localhost/test?sslmode=disable"),
	))
	defer sqlDB.Close()
	db := bun.NewDB(sqlDB, pgdialect.New())
	defer db.Close()

	query := db.NewCreateTable().
		Model((*entity.Character)(nil)).
		String()
	if strings.Contains(query, "COLLATE NOCASE") {
		t.Fatalf("PostgreSQL character schema contains SQLite collation: %s", query)
	}
	if !strings.Contains(query, "BIGSERIAL") {
		t.Fatalf("PostgreSQL character schema lacks generated ID: %s", query)
	}
}
