package repository

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"sshrpg/src/persistence"
)

func TestInventoryIsCreatedAndLoadedEmpty(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	closeDatabase(t, database)

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
	closeDatabase(t, database)
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

func TestInventoryStackIncrementFormatsForPostgreSQL(t *testing.T) {
	sqlDB := sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithDSN("postgres://test:test@localhost/test?sslmode=disable"),
	))
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	query := incrementInventoryStack(db, 7, 3, 2).String()
	for _, expected := range []string{
		`UPDATE "inventory_items"`,
		`SET quantity = quantity + 2`,
		`character_id = 7`,
		`slot = 3`,
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("PostgreSQL stack increment lacks %q: %s", expected, query)
		}
	}
}

func TestAddItemsDistributesQuantityAcrossStacks(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	closeDatabase(t, database)
	ctx := context.Background()
	character, err := NewCharacterRepository(database.ORM()).Create(
		ctx,
		CreateCharacterParams{
			KeyFingerprint: "SHA256:item-quantity",
			PublicKeyType:  "ssh-ed25519",
			PublicKey:      "key-item-quantity",
			Name:           "Quartermaster",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := NewInventoryRepository(database.ORM()).AddItems(
		ctx, character.ID, "slime_gel", 2, 5,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Items) != 3 ||
		inventory.Items[0].Quantity != 2 ||
		inventory.Items[1].Quantity != 2 ||
		inventory.Items[2].Quantity != 1 {
		t.Fatalf("bulk-added items = %#v", inventory.Items)
	}
}

func TestEquipmentAssignmentsArePersistedReplacedAndRemoved(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	closeDatabase(t, database)
	ctx := context.Background()
	character, err := NewCharacterRepository(database.ORM()).Create(
		ctx,
		CreateCharacterParams{
			KeyFingerprint: "SHA256:equipment",
			PublicKeyType:  "ssh-ed25519",
			PublicKey:      "key-equipment",
			Name:           "Armorer",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	inventories := NewInventoryRepository(database.ORM())
	first, err := inventories.AddItem(ctx, character.ID, "rusty_sword", 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := inventories.AddItem(ctx, character.ID, "iron_sword", 1)
	if err != nil {
		t.Fatal(err)
	}

	equipped, err := inventories.Equip(
		ctx, character.ID, first.Items[0].Slot, "rusty_sword", "weapon",
	)
	if err != nil {
		t.Fatal(err)
	}
	if slot, ok := equipped.EquippedInventorySlot("weapon"); !ok ||
		slot != first.Items[0].Slot {
		t.Fatalf("first equipment assignment = %#v", equipped.Equipment)
	}

	replaced, err := inventories.Equip(
		ctx, character.ID, second.Items[1].Slot, "iron_sword", "weapon",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(replaced.Equipment) != 1 ||
		replaced.Equipment[0].InventorySlot != second.Items[1].Slot ||
		replaced.IsEquipped(first.Items[0].Slot) {
		t.Fatalf("replaced equipment assignment = %#v", replaced.Equipment)
	}

	unequipped, err := inventories.Unequip(ctx, character.ID, "weapon")
	if err != nil {
		t.Fatal(err)
	}
	if len(unequipped.Equipment) != 0 {
		t.Fatalf("equipment remained after unequip: %#v", unequipped.Equipment)
	}
}

func TestConsumeItemDecrementsThenRemovesOwnedStack(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	closeDatabase(t, database)
	ctx := context.Background()
	character, err := NewCharacterRepository(database.ORM()).Create(
		ctx,
		CreateCharacterParams{
			KeyFingerprint: "SHA256:consume",
			PublicKeyType:  "ssh-ed25519",
			PublicKey:      "key-consume",
			Name:           "Alchemist",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	inventories := NewInventoryRepository(database.ORM())
	if _, err := inventories.AddItem(
		ctx, character.ID, "health_potion", 10,
	); err != nil {
		t.Fatal(err)
	}
	stacked, err := inventories.AddItem(
		ctx, character.ID, "health_potion", 10,
	)
	if err != nil {
		t.Fatal(err)
	}

	remaining, err := inventories.ConsumeItem(
		ctx, character.ID, stacked.Items[0].Slot, "health_potion",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining.Items) != 1 || remaining.Items[0].Quantity != 1 {
		t.Fatalf("consumed stack = %#v", remaining.Items)
	}
	empty, err := inventories.ConsumeItem(
		ctx, character.ID, remaining.Items[0].Slot, "health_potion",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Items) != 0 {
		t.Fatalf("last consumed item remained: %#v", empty.Items)
	}
	if _, err := inventories.ConsumeItem(
		ctx, character.ID, remaining.Items[0].Slot, "health_potion",
	); !errors.Is(err, ErrItemNotOwned) {
		t.Fatalf("missing consumed item error = %v", err)
	}
}
