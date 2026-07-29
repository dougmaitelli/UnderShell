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

func TestEnvWithoutFallback(t *testing.T) {
	t.Setenv("OPTIONAL_CONFIG", "")
	if got := env("OPTIONAL_CONFIG"); got != "" {
		t.Fatalf("optional config = %q, want empty value", got)
	}

	t.Setenv("OPTIONAL_CONFIG", "configured")
	if got := env("OPTIONAL_CONFIG"); got != "configured" {
		t.Fatalf("optional config = %q, want configured value", got)
	}
}

func TestEnvWithFallback(t *testing.T) {
	t.Setenv("DEFAULTED_CONFIG", "")
	if got := env("DEFAULTED_CONFIG", "fallback"); got != "fallback" {
		t.Fatalf("defaulted config = %q, want fallback", got)
	}

	t.Setenv("DEFAULTED_CONFIG", "configured")
	if got := env("DEFAULTED_CONFIG", "fallback"); got != "configured" {
		t.Fatalf("defaulted config = %q, want configured value", got)
	}
}
