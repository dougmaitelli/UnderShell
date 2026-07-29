package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"sshrpg/src/domain"
	"sshrpg/src/persistence"
)

func TestCharacterIdentityAndPersistence(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	closeDatabase(t, database)
	characters := NewCharacterRepository(database.ORM())

	ctx := context.Background()
	created, err := characters.Create(ctx, CreateCharacterParams{
		KeyFingerprint: "SHA256:first",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "key-one",
		Name:           "Aria",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.AreaID != "" {
		t.Fatalf("new character should not have a persisted location yet: %#v", created)
	}
	if created.Role != domain.CharacterRoleUser {
		t.Fatalf("new character role = %q, want user", created.Role)
	}
	if created.Level != 1 || created.Experience != 0 || created.SkillPoints != 0 ||
		created.Gold != domain.DefaultStartingGold {
		t.Fatalf("unexpected initial progression: %#v", created)
	}
	if err := characters.UpdateLocation(ctx, created.ID, "cavern", 7, 9); err != nil {
		t.Fatal(err)
	}
	if err := characters.UpdateProgress(ctx, created.ID, 3, 50, 2, 4, 3, 2); err != nil {
		t.Fatal(err)
	}
	if err := characters.UpdateRole(
		ctx, created.ID, domain.CharacterRoleAdmin,
	); err != nil {
		t.Fatal(err)
	}
	banned, err := characters.SetBanned(ctx, "aRiA", true)
	if err != nil {
		t.Fatal(err)
	}
	if !banned.Banned || banned.ID != created.ID {
		t.Fatalf("banned character = %#v", banned)
	}

	found, err := characters.FindByFingerprint(ctx, "SHA256:first")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.Name != "Aria" ||
		found.Role != domain.CharacterRoleAdmin ||
		!found.Banned ||
		found.AreaID != "cavern" ||
		found.X != 7 || found.Y != 9 ||
		found.Level != 3 || found.Experience != 50 || found.SkillPoints != 2 ||
		found.Attack != 4 || found.Defense != 3 || found.Vitality != 2 ||
		found.Gold != domain.DefaultStartingGold {
		t.Fatalf("unexpected character: %#v", found)
	}
	if _, err := characters.SetBanned(ctx, "ARIA", false); err != nil {
		t.Fatal(err)
	}
	unbanned, err := characters.FindByFingerprint(ctx, "SHA256:first")
	if err != nil {
		t.Fatal(err)
	}
	if unbanned.Banned {
		t.Fatal("character remained banned after unban")
	}
	if err := characters.UpdateRole(ctx, created.ID, "owner"); !errors.Is(
		err, domain.ErrInvalidCharacterRole,
	) {
		t.Fatalf("invalid role update error = %v", err)
	}
	if err := characters.UpdateRole(
		ctx, created.ID+999, domain.CharacterRoleUser,
	); !errors.Is(err, ErrCharacterNotFound) {
		t.Fatalf("missing character role update error = %v", err)
	}

	_, err = characters.Create(ctx, CreateCharacterParams{
		KeyFingerprint: "SHA256:second",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "key-two",
		Name:           "aria",
	})
	if !errors.Is(err, ErrCharacterNameTaken) {
		t.Fatalf("expected ErrCharacterNameTaken, got %v", err)
	}
	_, err = characters.Create(ctx, CreateCharacterParams{
		KeyFingerprint: "SHA256:first",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "key-one",
		Name:           "Rowan",
	})
	if !errors.Is(err, ErrCharacterKeyExists) {
		t.Fatalf("expected ErrCharacterKeyExists, got %v", err)
	}
}

func closeDatabase(t *testing.T, database *persistence.Database) {
	t.Helper()
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
}
