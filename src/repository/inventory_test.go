package repository

import (
	"context"
	"path/filepath"
	"testing"

	"sshrpg/src/persistence"
)

func TestInventoryIsCreatedAndLoadedEmpty(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	character, err := NewCharacterRepository(database.ORM()).Create(ctx, CreateCharacterParams{
		KeyFingerprint: "SHA256:inventory",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "key-inventory",
		Name:           "Keeper",
	})
	if err != nil {
		t.Fatal(err)
	}

	inventories := NewInventoryRepository(database.ORM())
	created, err := inventories.FindOrCreate(ctx, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if created.CharacterID != character.ID || len(created.Items) != 0 {
		t.Fatalf("unexpected new inventory: %#v", created)
	}

	loaded, err := inventories.FindOrCreate(ctx, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CharacterID != character.ID || len(loaded.Items) != 0 {
		t.Fatalf("unexpected loaded inventory: %#v", loaded)
	}
}
