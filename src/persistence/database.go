package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/migrate"

	"sshrpg/src/persistence/migrations"
)

type Database struct {
	sql *sql.DB
	orm *bun.DB
}

func Open(path string) (*Database, error) {
	sqlDB, err := sql.Open(
		sqliteshim.ShimName,
		path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
	)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	database := &Database{
		sql: sqlDB,
		orm: bun.NewDB(sqlDB, sqlitedialect.New()),
	}
	if err := database.applyMigrations(context.Background()); err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}

func (d *Database) ORM() bun.IDB {
	return d.orm
}

func (d *Database) Close() error {
	if d.orm != nil {
		return d.orm.Close()
	}
	if d.sql != nil {
		return d.sql.Close()
	}
	return nil
}

func (d *Database) applyMigrations(ctx context.Context) error {
	migrator := migrate.NewMigrator(
		d.orm,
		migrations.Migrations,
		migrate.WithMarkAppliedOnSuccess(true),
	)
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("initialize database migrations: %w", err)
	}
	if _, err := migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}
	return nil
}
