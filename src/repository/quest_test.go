package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"sshrpg/src/domain"
	"sshrpg/src/item"
	"sshrpg/src/persistence"
	"sshrpg/src/quest"
)

func TestQuestCompletionConsumesItemsAndRewardsGoldAtomically(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	character, err := NewCharacterRepository(database.ORM()).Create(ctx, CreateCharacterParams{
		KeyFingerprint: "SHA256:quest",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "key-quest",
		Name:           "Adventurer",
	})
	if err != nil {
		t.Fatal(err)
	}
	inventories := NewInventoryRepository(database.ORM())
	if _, err := inventories.FindOrCreate(ctx, character.ID); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if _, err := inventories.AddItem(ctx, character.ID, "slime_gel", 2); err != nil {
			t.Fatal(err)
		}
	}

	quests := NewQuestRepository(database.ORM())
	accepted, err := quests.Accept(ctx, AcceptQuestParams{
		CharacterID: character.ID, QuestID: "slime_supplies",
		GiverID: "orin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != domain.QuestActive || accepted.GiverID != "orin" {
		t.Fatalf("unexpected accepted quest: %#v", accepted)
	}
	completed, err := quests.Complete(ctx, character.ID, &quest.Definition{
		ID: "slime_supplies",
		Objective: quest.Objective{
			Item: questItem("slime_gel"), Quantity: 3,
		},
		Reward: quest.Reward{Gold: 30},
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Quest.Status != domain.QuestCompleted {
		t.Fatalf("quest status = %q, want completed", completed.Quest.Status)
	}
	if completed.Gold != domain.DefaultStartingGold+30 {
		t.Fatalf("gold = %d, want %d", completed.Gold, domain.DefaultStartingGold+30)
	}
	remaining := 0
	for _, stack := range completed.Inventory.Items {
		if stack.ItemKey == "slime_gel" {
			remaining += stack.Quantity
		}
	}
	if remaining != 2 {
		t.Fatalf("remaining slime gel = %d, want 2", remaining)
	}
	progress, err := quests.FindByCharacter(ctx, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(progress) != 1 || progress[0].Status != domain.QuestCompleted {
		t.Fatalf("unexpected persisted quests: %#v", progress)
	}
	if _, err := quests.Complete(
		ctx, character.ID, &quest.Definition{ID: "slime_supplies"},
	); !errors.Is(err, ErrQuestNotActive) {
		t.Fatalf("expected completed quest rejection, got %v", err)
	}
}

func TestQuestCompletionRollsBackWhenItemsAreIncomplete(t *testing.T) {
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	character, err := NewCharacterRepository(database.ORM()).Create(ctx, CreateCharacterParams{
		KeyFingerprint: "SHA256:incomplete-quest",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "key-incomplete-quest",
		Name:           "Gatherer",
	})
	if err != nil {
		t.Fatal(err)
	}
	inventories := NewInventoryRepository(database.ORM())
	if _, err := inventories.AddItem(ctx, character.ID, "slime_gel", 50); err != nil {
		t.Fatal(err)
	}
	quests := NewQuestRepository(database.ORM())
	if _, err := quests.Accept(ctx, AcceptQuestParams{
		CharacterID: character.ID, QuestID: "slime_supplies",
		GiverID: "orin",
	}); err != nil {
		t.Fatal(err)
	}

	_, err = quests.Complete(ctx, character.ID, &quest.Definition{
		ID: "slime_supplies",
		Objective: quest.Objective{
			Item: questItem("slime_gel"), Quantity: 2,
		},
		Reward: quest.Reward{Gold: 30},
	})
	if !errors.Is(err, ErrQuestItemsIncomplete) {
		t.Fatalf("expected incomplete items, got %v", err)
	}
	inventory, err := inventories.FindOrCreate(ctx, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Items) != 1 || inventory.Items[0].Quantity != 1 {
		t.Fatalf("failed completion changed inventory: %#v", inventory.Items)
	}
	progress, err := quests.FindByCharacter(ctx, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(progress) != 1 || progress[0].Status != domain.QuestActive {
		t.Fatalf("failed completion changed quest: %#v", progress)
	}
}

func TestQuestStateAndProgressPersistAcrossDatabaseReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.db")
	database, err := persistence.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	characters := NewCharacterRepository(database.ORM())
	character, err := characters.Create(ctx, CreateCharacterParams{
		KeyFingerprint: "SHA256:persistent-quest",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "key-persistent-quest",
		Name:           "Wayfarer",
	})
	if err != nil {
		t.Fatal(err)
	}
	inventories := NewInventoryRepository(database.ORM())
	for range 2 {
		if _, err := inventories.AddItem(
			ctx, character.ID, "slime_gel", 50,
		); err != nil {
			t.Fatal(err)
		}
	}
	quests := NewQuestRepository(database.ORM())
	if _, err := quests.Accept(ctx, AcceptQuestParams{
		CharacterID: character.ID, QuestID: "slime_supplies",
		GiverID: "orin",
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	database, err = persistence.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	quests = NewQuestRepository(database.ORM())
	progress, err := quests.FindByCharacter(ctx, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(progress) != 1 || progress[0].Status != domain.QuestActive ||
		progress[0].GiverID != "orin" {
		t.Fatalf("active quest did not persist: %#v", progress)
	}
	inventory, err := NewInventoryRepository(database.ORM()).
		FindOrCreate(ctx, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Items) != 1 || inventory.Items[0].Quantity != 2 {
		t.Fatalf("quest item progress did not persist: %#v", inventory.Items)
	}
	definition := quest.Definition{
		ID: "slime_supplies",
		Objective: quest.Objective{
			Item: questItem("slime_gel"), Quantity: 2,
		},
		Reward: quest.Reward{Gold: 30},
	}
	completed, err := quests.Complete(ctx, character.ID, &definition)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Gold != domain.DefaultStartingGold+30 {
		t.Fatalf("completed quest gold = %d", completed.Gold)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	database, err = persistence.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	quests = NewQuestRepository(database.ORM())
	progress, err = quests.FindByCharacter(ctx, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(progress) != 1 || progress[0].Status != domain.QuestCompleted ||
		progress[0].GiverID != "orin" {
		t.Fatalf("completed quest did not persist: %#v", progress)
	}
	if _, err := quests.Complete(
		ctx, character.ID, &definition,
	); !errors.Is(err, ErrQuestNotActive) {
		t.Fatalf("second completion was not rejected: %v", err)
	}
	reloaded, err := NewCharacterRepository(database.ORM()).
		FindByFingerprint(ctx, "SHA256:persistent-quest")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Gold != domain.DefaultStartingGold+30 {
		t.Fatalf("second completion changed gold to %d", reloaded.Gold)
	}
}

func questItem(id string) *item.Definition {
	return &item.Definition{ID: id, Name: id, MaxStack: 50}
}
