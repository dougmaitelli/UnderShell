package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/migrate"

	"sshrpg/src/persistence/migrations"
)

type Database struct {
	sql *sql.DB
	orm *bun.DB
}

// Open connects to PostgreSQL when source is a postgres:// or postgresql://
// URL. Every other source is treated as a SQLite filesystem path.
func Open(source string) (*Database, error) {
	if source == "" {
		return nil, errors.New("database source is required")
	}
	if isPostgreSQLURL(source) {
		return openPostgreSQL(source)
	}
	if strings.Contains(source, "://") {
		return nil, fmt.Errorf("unsupported database URL scheme %q", databaseScheme(source))
	}
	return openSQLite(source)
}

func openSQLite(path string) (*Database, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create SQLite data directory: %w", err)
	}
	sqlDB, err := sql.Open(
		sqliteshim.ShimName,
		path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
	)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		return nil, errors.Join(
			fmt.Errorf("ping sqlite: %w", err),
			sqlDB.Close(),
		)
	}

	database := &Database{
		sql: sqlDB,
		orm: bun.NewDB(sqlDB, sqlitedialect.New()),
	}
	if err := database.applyMigrations(context.Background()); err != nil {
		return nil, errors.Join(err, database.Close())
	}
	return database, nil
}

func openPostgreSQL(dsn string) (*Database, error) {
	sqlDB := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if err := sqlDB.Ping(); err != nil {
		return nil, errors.Join(
			fmt.Errorf("ping PostgreSQL: %w", err),
			sqlDB.Close(),
		)
	}

	database := &Database{
		sql: sqlDB,
		orm: bun.NewDB(sqlDB, pgdialect.New()),
	}
	if err := database.applyMigrations(context.Background()); err != nil {
		return nil, errors.Join(err, database.Close())
	}
	return database, nil
}

func isPostgreSQLURL(source string) bool {
	scheme := databaseScheme(source)
	return scheme == "postgres" || scheme == "postgresql"
}

func databaseScheme(source string) string {
	parsed, err := url.Parse(source)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Scheme)
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
