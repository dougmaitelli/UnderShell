package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"sshrpg/src/domain"
	"sshrpg/src/persistence"
)

func TestShopBuyAndSellUpdatesInventoryAndGoldAtomically(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	closeDatabase(t, database)
	ctx := context.Background()
	character, err := NewCharacterRepository(database.ORM()).Create(ctx, CreateCharacterParams{
		KeyFingerprint: "SHA256:shop",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "key-shop",
		Name:           "Merchant",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewInventoryRepository(database.ORM()).FindOrCreate(ctx, character.ID); err != nil {
		t.Fatal(err)
	}
	shops := NewShopRepository(database.ORM())
	bought, err := shops.BuyItem(ctx, character.ID, "health_potion", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if bought.Gold != domain.DefaultStartingGold-10 ||
		len(bought.Inventory.Items) != 1 ||
		bought.Inventory.Items[0].Quantity != 1 {
		t.Fatalf("unexpected purchase: %#v", bought)
	}
	characters := NewCharacterRepository(database.ORM())
	if err := characters.UpdateProgress(ctx, character.ID, 2, 25, 1, 1, 0, 0); err != nil {
		t.Fatal(err)
	}
	reloaded, err := characters.FindByFingerprint(ctx, "SHA256:shop")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Gold != bought.Gold {
		t.Fatalf("progress save changed gold from %d to %d", bought.Gold, reloaded.Gold)
	}
	sold, err := shops.SellItem(
		ctx, character.ID, bought.Inventory.Items[0].Slot, "health_potion", 5,
	)
	if err != nil {
		t.Fatal(err)
	}
	if sold.Gold != domain.DefaultStartingGold-5 || len(sold.Inventory.Items) != 0 {
		t.Fatalf("unexpected sale: %#v", sold)
	}
}

func TestShopRejectsPurchaseWithoutGold(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	closeDatabase(t, database)
	ctx := context.Background()
	character, err := NewCharacterRepository(database.ORM()).Create(ctx, CreateCharacterParams{
		KeyFingerprint: "SHA256:poor-shop",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "key-poor-shop",
		Name:           "Browser",
	})
	if err != nil {
		t.Fatal(err)
	}
	shops := NewShopRepository(database.ORM())
	_, err = shops.BuyItem(
		ctx, character.ID, "rusty_sword", 1, domain.DefaultStartingGold+1,
	)
	if !errors.Is(err, ErrInsufficientGold) {
		t.Fatalf("expected insufficient gold, got %v", err)
	}
	inventory, err := NewInventoryRepository(database.ORM()).FindOrCreate(ctx, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Items) != 0 {
		t.Fatalf("failed purchase changed inventory: %#v", inventory.Items)
	}
}

func TestShopRejectsSellingEquippedItem(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	closeDatabase(t, database)
	ctx := context.Background()
	character, err := NewCharacterRepository(database.ORM()).Create(
		ctx,
		CreateCharacterParams{
			KeyFingerprint: "SHA256:equipped-shop",
			PublicKeyType:  "ssh-ed25519",
			PublicKey:      "key-equipped-shop",
			Name:           "Knight",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	shops := NewShopRepository(database.ORM())
	bought, err := shops.BuyItem(ctx, character.ID, "rusty_sword", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	stack := bought.Inventory.Items[0]
	inventories := NewInventoryRepository(database.ORM())
	if _, err := inventories.Equip(
		ctx, character.ID, stack.Slot, stack.ItemKey, "weapon",
	); err != nil {
		t.Fatal(err)
	}

	_, err = shops.SellItem(
		ctx, character.ID, stack.Slot, stack.ItemKey, 5,
	)
	if !errors.Is(err, ErrItemEquipped) {
		t.Fatalf("expected equipped item rejection, got %v", err)
	}
	reloaded, err := inventories.FindOrCreate(ctx, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Items) != 1 || !reloaded.IsEquipped(stack.Slot) {
		t.Fatalf("rejected sale changed equipment: %#v", reloaded)
	}
}
