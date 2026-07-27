package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"sshrpg/src/persistence"
)

func TestCharacterIdentityAndPersistence(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
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
	if err := characters.UpdatePosition(ctx, created.ID, 7, 9); err != nil {
		t.Fatal(err)
	}

	found, err := characters.FindByFingerprint(ctx, "SHA256:first")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.Name != "Aria" || found.X != 7 || found.Y != 9 {
		t.Fatalf("unexpected character: %#v", found)
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
