package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"

	"sshrpg/src/persistence/entity"
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
	if err := database.createSchema(context.Background()); err != nil {
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

func (d *Database) createSchema(ctx context.Context) error {
	if _, err := d.orm.NewCreateTable().
		Model((*entity.Character)(nil)).
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("create character schema: %w", err)
	}
	if _, err := d.orm.NewCreateTable().
		Model((*entity.CharacterLocation)(nil)).
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("create character location schema: %w", err)
	}
	if _, err := d.orm.NewCreateTable().
		Model((*entity.Inventory)(nil)).
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("create inventory schema: %w", err)
	}
	if _, err := d.orm.NewCreateTable().
		Model((*entity.InventoryItem)(nil)).
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("create inventory item schema: %w", err)
	}
	return nil
}
