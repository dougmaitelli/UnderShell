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
		Model((*entity.CharacterProgress)(nil)).
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("create character progress schema: %w", err)
	}
	for _, column := range []string{"attack", "defense", "vitality"} {
		if err := d.ensureProgressColumn(ctx, column); err != nil {
			return err
		}
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

func (d *Database) ensureProgressColumn(ctx context.Context, column string) error {
	var count int
	if err := d.orm.NewRaw(
		"SELECT COUNT(*) FROM pragma_table_info('character_progress') WHERE name = ?",
		column,
	).Scan(ctx, &count); err != nil {
		return fmt.Errorf("inspect character progress column %s: %w", column, err)
	}
	if count > 0 {
		return nil
	}
	if _, err := d.orm.NewRaw(
		"ALTER TABLE character_progress ADD COLUMN " + column + " INTEGER NOT NULL DEFAULT 0",
	).Exec(ctx); err != nil {
		return fmt.Errorf("add character progress column %s: %w", column, err)
	}
	return nil
}
