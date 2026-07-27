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

func TestAddItemStacksToLimitThenUsesNextSlot(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	character, err := NewCharacterRepository(database.ORM()).Create(ctx, CreateCharacterParams{
		KeyFingerprint: "SHA256:items",
		PublicKeyType:  "ssh-ed25519", PublicKey: "key-items", Name: "Collector",
	})
	if err != nil {
		t.Fatal(err)
	}
	inventories := NewInventoryRepository(database.ORM())
	if _, err := inventories.FindOrCreate(ctx, character.ID); err != nil {
		t.Fatal(err)
	}
	first, err := inventories.AddItem(ctx, character.ID, "slime_gel", 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := inventories.AddItem(ctx, character.ID, "slime_gel", 2)
	if err != nil {
		t.Fatal(err)
	}
	third, err := inventories.AddItem(ctx, character.ID, "slime_gel", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.Items[0].Quantity != 1 {
		t.Fatalf("first pickup = %#v", first.Items)
	}
	if len(second.Items) != 1 || second.Items[0].Quantity != 2 {
		t.Fatalf("second pickup = %#v", second.Items)
	}
	if len(third.Items) != 2 ||
		third.Items[0].Quantity != 2 ||
		third.Items[1].Quantity != 1 ||
		third.Items[1].Slot != 2 {
		t.Fatalf("third pickup = %#v", third.Items)
	}
}
