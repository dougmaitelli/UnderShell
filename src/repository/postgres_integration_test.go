package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"sshrpg/src/domain"
	"sshrpg/src/item"
	"sshrpg/src/persistence"
	"sshrpg/src/quest"
)

func TestPostgreSQLConcurrentEconomicTransitions(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DATABASE_URL is not set")
	}
	database, err := persistence.Open(dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	closeDatabase(t, database)
	if _, err := database.ORM().ExecContext(
		context.Background(), "TRUNCATE TABLE characters CASCADE",
	); err != nil {
		t.Fatalf("reset PostgreSQL test data: %v", err)
	}

	t.Run("sale", func(t *testing.T) {
		ctx := context.Background()
		character := createIntegrationCharacter(t, database, "sale")
		inventories := NewInventoryRepository(database.ORM())
		const sales = 20
		inventory, err := inventories.AddItems(
			ctx, character.ID, "slime_gel", sales, sales,
		)
		if err != nil {
			t.Fatal(err)
		}
		slot := inventory.Items[0].Slot
		shops := NewShopRepository(database.ORM())
		runConcurrently(t, sales, func() error {
			_, err := shops.SellItem(ctx, character.ID, slot, "slime_gel", 3)
			return err
		})

		stored, err := inventories.FindOrCreate(ctx, character.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(stored.Items) != 0 {
			t.Fatalf("concurrent sales left inventory: %#v", stored.Items)
		}
		gold, err := characterGold(ctx, database.ORM(), character.ID)
		if err != nil {
			t.Fatal(err)
		}
		wantGold := domain.DefaultStartingGold + sales*3
		if gold != wantGold {
			t.Fatalf("gold = %d, want %d", gold, wantGold)
		}
	})

	t.Run("consume stack", func(t *testing.T) {
		ctx := context.Background()
		character := createIntegrationCharacter(t, database, "consume")
		inventories := NewInventoryRepository(database.ORM())
		inventory, err := inventories.AddItems(ctx, character.ID, "health_potion", 10, 2)
		if err != nil {
			t.Fatal(err)
		}
		slot := inventory.Items[0].Slot
		remaining, err := inventories.ConsumeItem(ctx, character.ID, slot, "health_potion")
		if err != nil {
			t.Fatal(err)
		}
		if len(remaining.Items) != 1 || remaining.Items[0].Quantity != 1 {
			t.Fatalf("consumed stack = %#v", remaining.Items)
		}
		empty, err := inventories.ConsumeItem(ctx, character.ID, slot, "health_potion")
		if err != nil {
			t.Fatal(err)
		}
		if len(empty.Items) != 0 {
			t.Fatalf("last consumed item remained: %#v", empty.Items)
		}
	})

	t.Run("quest completion", func(t *testing.T) {
		ctx := context.Background()
		character := createIntegrationCharacter(t, database, "quest")
		inventories := NewInventoryRepository(database.ORM())
		if _, err := inventories.AddItems(
			ctx, character.ID, "slime_gel", 10, 2,
		); err != nil {
			t.Fatal(err)
		}
		quests := NewQuestRepository(database.ORM())
		if _, err := quests.Accept(ctx, AcceptQuestParams{
			CharacterID: character.ID,
			QuestID:     "concurrent_quest",
			GiverID:     "tester",
		}); err != nil {
			t.Fatal(err)
		}
		definition := &quest.Definition{
			ID: "concurrent_quest",
			Objective: quest.Objective{
				Item: &item.Definition{ID: "slime_gel"}, Quantity: 2,
			},
			Reward: quest.Reward{Gold: 40},
		}
		var completed atomic.Int32
		runConcurrently(t, 2, func() error {
			_, err := quests.Complete(ctx, character.ID, definition)
			if err == nil {
				completed.Add(1)
				return nil
			}
			if errors.Is(err, ErrQuestNotActive) {
				return nil
			}
			return err
		})
		if completed.Load() != 1 {
			t.Fatalf("successful completion claims = %d, want 1", completed.Load())
		}
		gold, err := characterGold(ctx, database.ORM(), character.ID)
		if err != nil {
			t.Fatal(err)
		}
		if gold != domain.DefaultStartingGold+40 {
			t.Fatalf("quest reward gold = %d", gold)
		}
	})

	t.Run("inventory mutation", func(t *testing.T) {
		ctx := context.Background()
		character := createIntegrationCharacter(t, database, "inventory")
		inventories := NewInventoryRepository(database.ORM())
		const additions = 40
		const maxStack = 7
		runConcurrently(t, additions, func() error {
			_, err := inventories.AddItem(
				ctx, character.ID, "slime_gel", maxStack,
			)
			return err
		})
		stored, err := inventories.FindOrCreate(ctx, character.ID)
		if err != nil {
			t.Fatal(err)
		}
		total := 0
		for _, stack := range stored.Items {
			if stack.Quantity < 1 || stack.Quantity > maxStack {
				t.Fatalf("invalid concurrent stack: %#v", stack)
			}
			total += stack.Quantity
		}
		if total != additions {
			t.Fatalf("stored quantity = %d, want %d", total, additions)
		}
	})
}

func createIntegrationCharacter(
	t *testing.T,
	database *persistence.Database,
	suffix string,
) *domain.Character {
	t.Helper()
	key := fmt.Sprintf("%s-%s", t.Name(), suffix)
	character, err := NewCharacterRepository(database.ORM()).Create(
		context.Background(),
		CreateCharacterParams{
			KeyFingerprint: "SHA256:" + key,
			PublicKeyType:  "ssh-ed25519",
			PublicKey:      "key-" + key,
			Name:           integrationCharacterName(suffix),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return character
}

func integrationCharacterName(suffix string) string {
	switch suffix {
	case "sale":
		return "PgSeller"
	case "quest":
		return "PgQuester"
	default:
		return "PgKeeper"
	}
}

func runConcurrently(t *testing.T, count int, operation func() error) {
	t.Helper()
	start := make(chan struct{})
	errs := make(chan error, count)
	var workers sync.WaitGroup
	for range count {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errs <- operation()
		}()
	}
	close(start)
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent operation: %v", err)
		}
	}
}
