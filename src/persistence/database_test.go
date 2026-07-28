package persistence

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"

	"sshrpg/src/persistence/entity"
)

type legacyCharacter struct {
	bun.BaseModel `bun:"table:characters,alias:character"`

	ID             int64  `bun:"id,pk,autoincrement"`
	KeyFingerprint string `bun:"key_fingerprint,notnull,unique"`
	PublicKeyType  string `bun:"public_key_type,notnull"`
	PublicKey      string `bun:"public_key,notnull"`
	Name           string `bun:"name,notnull,unique,type:TEXT COLLATE NOCASE"`
	CreatedAt      string `bun:"created_at,notnull"`
	LastSeenAt     string `bun:"last_seen_at,notnull"`
}

type legacyCharacterProgress struct {
	bun.BaseModel `bun:"table:character_progress,alias:character_progress"`

	CharacterID int64 `bun:"character_id,pk"`
	Level       int   `bun:"level,notnull"`
	Experience  int64 `bun:"experience,notnull"`
	SkillPoints int   `bun:"skill_points,notnull"`
}

type legacyCharacterQuest struct {
	bun.BaseModel `bun:"table:character_quests,alias:character_quest"`

	CharacterID int64  `bun:"character_id,pk"`
	QuestID     string `bun:"quest_id,pk"`
	GiverID     string `bun:"giver_id,notnull"`
	GiverName   string `bun:"giver_name,notnull"`
	Status      string `bun:"status,notnull"`
	AcceptedAt  string `bun:"accepted_at,notnull"`
	CompletedAt *string
}

func TestOpenMigratesLegacyProgressSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open(sqliteshim.ShimName, path)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	legacyORM := bun.NewDB(legacy, sqlitedialect.New())
	for _, model := range []any{
		(*legacyCharacter)(nil),
		(*legacyCharacterProgress)(nil),
		(*legacyCharacterQuest)(nil),
	} {
		if _, err := legacyORM.NewCreateTable().Model(model).Exec(ctx); err != nil {
			legacyORM.Close()
			t.Fatalf("create legacy schema: %v", err)
		}
	}
	if _, err := legacyORM.NewInsert().Model(&legacyCharacter{
		ID: 7, KeyFingerprint: "SHA256:legacy",
		PublicKeyType: "ssh-ed25519", PublicKey: "legacy-key",
		Name: "Legacy", CreatedAt: "now", LastSeenAt: "now",
	}).Exec(ctx); err != nil {
		legacyORM.Close()
		t.Fatalf("insert legacy character: %v", err)
	}
	if _, err := legacyORM.NewInsert().Model(&legacyCharacterProgress{
		CharacterID: 7, Level: 4, Experience: 125, SkillPoints: 2,
	}).Exec(ctx); err != nil {
		legacyORM.Close()
		t.Fatalf("insert legacy progress: %v", err)
	}
	if _, err := legacyORM.NewInsert().Model(&legacyCharacterQuest{
		CharacterID: 7, QuestID: "legacy_quest",
		GiverID: "legacy_giver", GiverName: "Legacy Giver",
		Status: "active", AcceptedAt: "now",
	}).Exec(ctx); err != nil {
		legacyORM.Close()
		t.Fatalf("insert legacy quest: %v", err)
	}
	if err := legacyORM.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	database, err := Open(path)
	if err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}

	progress := new(entity.CharacterProgress)
	if err := database.orm.NewSelect().
		Model(progress).
		Where("character_id = ?", 7).
		Scan(ctx); err != nil {
		database.Close()
		t.Fatalf("read migrated progress: %v", err)
	}
	if progress.Level != 4 ||
		progress.Experience != 125 ||
		progress.SkillPoints != 2 {
		database.Close()
		t.Fatalf(
			"legacy progress changed: level=%d experience=%d points=%d",
			progress.Level,
			progress.Experience,
			progress.SkillPoints,
		)
	}
	if progress.Attack != 0 ||
		progress.Defense != 0 ||
		progress.Vitality != 0 ||
		progress.Gold != 100 {
		database.Close()
		t.Fatalf(
			"unexpected migration defaults: attack=%d defense=%d vitality=%d gold=%d",
			progress.Attack,
			progress.Defense,
			progress.Vitality,
			progress.Gold,
		)
	}
	var copiedGiverColumns int
	if err := database.orm.NewRaw(`
		SELECT COUNT(*)
		FROM pragma_table_info('character_quests')
		WHERE name IN ('giver_name', 'giver_area_id')
	`).Scan(ctx, &copiedGiverColumns); err != nil {
		database.Close()
		t.Fatalf("inspect normalized quest giver columns: %v", err)
	}
	if copiedGiverColumns != 0 {
		database.Close()
		t.Fatalf("copied quest giver columns remaining = %d", copiedGiverColumns)
	}
	character := new(entity.Character)
	if err := database.orm.NewSelect().
		Model(character).
		Where("id = ?", 7).
		Scan(ctx); err != nil {
		database.Close()
		t.Fatalf("read migrated character role: %v", err)
	}
	if character.Role != "user" {
		database.Close()
		t.Fatalf("migrated character role = %q, want user", character.Role)
	}

	var migrationCount int
	if err := database.orm.NewSelect().
		Table("bun_migrations").
		ColumnExpr("COUNT(*)").
		Scan(ctx, &migrationCount); err != nil {
		database.Close()
		t.Fatalf("count applied migrations: %v", err)
	}
	if migrationCount != 8 {
		database.Close()
		t.Fatalf("applied migrations = %d, want 8", migrationCount)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close migrated database: %v", err)
	}

	database, err = Open(path)
	if err != nil {
		t.Fatalf("reopen migrated database: %v", err)
	}
	defer database.Close()

	if err := database.orm.NewSelect().
		Table("bun_migrations").
		ColumnExpr("COUNT(*)").
		Scan(ctx, &migrationCount); err != nil {
		t.Fatalf("count migrations after reopen: %v", err)
	}
	if migrationCount != 8 {
		t.Fatalf("applied migrations after reopen = %d, want 8", migrationCount)
	}
}
