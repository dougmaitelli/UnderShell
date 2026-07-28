package persistence

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestOpenMigratesLegacyProgressSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open(sqliteshim.ShimName, path)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	if _, err := legacy.Exec(`
		CREATE TABLE character_progress (
			character_id INTEGER PRIMARY KEY,
			level INTEGER NOT NULL,
			experience INTEGER NOT NULL,
			skill_points INTEGER NOT NULL
		);
		INSERT INTO character_progress (
			character_id,
			level,
			experience,
			skill_points
		) VALUES (7, 4, 125, 2);
		CREATE TABLE character_quests (
			character_id INTEGER NOT NULL,
			quest_id TEXT NOT NULL,
			giver_id TEXT NOT NULL,
			giver_name TEXT NOT NULL,
			status TEXT NOT NULL,
			accepted_at TEXT NOT NULL,
			completed_at TEXT,
			PRIMARY KEY (character_id, quest_id)
		);
		INSERT INTO character_quests (
			character_id,
			quest_id,
			giver_id,
			giver_name,
			status,
			accepted_at
		) VALUES (7, 'legacy_quest', 'legacy_giver', 'Legacy Giver', 'active', 'now');
	`); err != nil {
		legacy.Close()
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	database, err := Open(path)
	if err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}

	var (
		level      int
		experience int64
		points     int
		attack     int
		defense    int
		vitality   int
		gold       int
	)
	if err := database.sql.QueryRow(`
		SELECT
			level,
			experience,
			skill_points,
			attack,
			defense,
			vitality,
			gold
		FROM character_progress
		WHERE character_id = 7
	`).Scan(
		&level,
		&experience,
		&points,
		&attack,
		&defense,
		&vitality,
		&gold,
	); err != nil {
		database.Close()
		t.Fatalf("read migrated progress: %v", err)
	}
	if level != 4 || experience != 125 || points != 2 {
		database.Close()
		t.Fatalf(
			"legacy progress changed: level=%d experience=%d points=%d",
			level,
			experience,
			points,
		)
	}
	if attack != 0 || defense != 0 || vitality != 0 || gold != 100 {
		database.Close()
		t.Fatalf(
			"unexpected migration defaults: attack=%d defense=%d vitality=%d gold=%d",
			attack,
			defense,
			vitality,
			gold,
		)
	}
	var copiedGiverColumns int
	if err := database.sql.QueryRow(`
		SELECT COUNT(*)
		FROM pragma_table_info('character_quests')
		WHERE name IN ('giver_name', 'giver_area_id')
	`).Scan(&copiedGiverColumns); err != nil {
		database.Close()
		t.Fatalf("inspect normalized quest giver columns: %v", err)
	}
	if copiedGiverColumns != 0 {
		database.Close()
		t.Fatalf("copied quest giver columns remaining = %d", copiedGiverColumns)
	}

	var migrationCount int
	if err := database.sql.QueryRow(
		"SELECT COUNT(*) FROM bun_migrations",
	).Scan(&migrationCount); err != nil {
		database.Close()
		t.Fatalf("count applied migrations: %v", err)
	}
	if migrationCount != 7 {
		database.Close()
		t.Fatalf("applied migrations = %d, want 7", migrationCount)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close migrated database: %v", err)
	}

	database, err = Open(path)
	if err != nil {
		t.Fatalf("reopen migrated database: %v", err)
	}
	defer database.Close()

	if err := database.sql.QueryRow(
		"SELECT COUNT(*) FROM bun_migrations",
	).Scan(&migrationCount); err != nil {
		t.Fatalf("count migrations after reopen: %v", err)
	}
	if migrationCount != 7 {
		t.Fatalf("applied migrations after reopen = %d, want 7", migrationCount)
	}
}
