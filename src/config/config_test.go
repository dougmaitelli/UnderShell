package config

import "testing"

func TestDatabaseSourcePrefersPostgreSQLURL(t *testing.T) {
	cfg := Config{
		DatabaseURL:  "postgres://game:secret@database/game",
		DatabasePath: "/tmp/game.db",
	}
	if got := cfg.DatabaseSource(); got != cfg.DatabaseURL {
		t.Fatalf("database source = %q, want PostgreSQL URL", got)
	}
}

func TestDatabaseSourceFallsBackToSQLitePath(t *testing.T) {
	cfg := Config{DatabasePath: "/tmp/game.db"}
	if got := cfg.DatabaseSource(); got != cfg.DatabasePath {
		t.Fatalf("database source = %q, want SQLite path", got)
	}
}
