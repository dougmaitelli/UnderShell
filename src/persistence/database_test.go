package persistence

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"sshrpg/src/persistence/entity"
)

func TestEconomicConstraintsRejectInvalidDirectWrites(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	ctx := context.Background()
	if _, err := database.orm.NewInsert().Model(&entity.CharacterProgress{
		CharacterID: 2, Level: 0,
	}).Exec(ctx); err == nil {
		t.Fatal("database accepted invalid progress insert")
	}
	if _, err := database.orm.NewInsert().Model(&entity.InventoryItem{
		CharacterID: 2, Slot: 1, ItemKey: "test_item", Quantity: 0,
	}).Exec(ctx); err == nil {
		t.Fatal("database accepted invalid inventory insert")
	}

	validProgress := &entity.CharacterProgress{
		CharacterID: 1, Level: 1, Gold: 0,
	}
	if _, err := database.orm.NewInsert().Model(validProgress).Exec(ctx); err != nil {
		t.Fatalf("insert valid progress: %v", err)
	}
	invalidProgressUpdates := []string{
		"level = 0", "experience = -1", "skill_points = -1",
		"attack = -1", "defense = -1", "vitality = -1", "gold = -1",
	}
	for _, assignment := range invalidProgressUpdates {
		_, err := database.orm.NewUpdate().
			Model((*entity.CharacterProgress)(nil)).
			Set(assignment).
			Where("character_id = 1").
			Exec(ctx)
		if err == nil {
			t.Fatalf("database accepted invalid progress update %q", assignment)
		}
	}

	validItem := &entity.InventoryItem{
		CharacterID: 1, Slot: 1, ItemKey: "test_item", Quantity: 1,
	}
	if _, err := database.orm.NewInsert().Model(validItem).Exec(ctx); err != nil {
		t.Fatalf("insert valid inventory item: %v", err)
	}
	for _, quantity := range []int{0, -1} {
		_, err := database.orm.NewUpdate().
			Model((*entity.InventoryItem)(nil)).
			Set("quantity = ?", quantity).
			Where("character_id = 1 AND slot = 1").
			Exec(ctx)
		if err == nil {
			t.Fatalf("database accepted inventory quantity %d", quantity)
		}
	}

	// A rejected statement must not poison SQLite's connection or alter the
	// previously valid row.
	stored := new(entity.InventoryItem)
	if err := database.orm.NewSelect().Model(stored).
		Where("character_id = 1 AND slot = 1").Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if stored.Quantity != 1 {
		t.Fatalf("rejected writes changed quantity to %d", stored.Quantity)
	}
}

func TestOpenRejectsUnsupportedDatabaseURL(t *testing.T) {
	_, err := Open("mysql://localhost/game")
	if err == nil || !strings.Contains(err.Error(), "unsupported database URL scheme") {
		t.Fatalf("unsupported database error = %v", err)
	}
}

func TestOpenCreatesAndReopensCurrentSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "game.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("open fresh database: %v", err)
	}

	character := &entity.Character{
		KeyFingerprint: "SHA256:current-schema",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "current-schema-key",
		Name:           "Schema",
		Role:           "moderator",
		Banned:         true,
		CreatedAt:      "now",
		LastSeenAt:     "now",
	}
	if _, err := database.orm.NewInsert().Model(character).Exec(ctx); err != nil {
		t.Fatalf("insert current character schema: %v", err)
	}
	for _, model := range []any{
		&entity.CharacterLocation{
			CharacterID: character.ID,
			AreaID:      "meadow",
			X:           3,
			Y:           4,
			UpdatedAt:   "now",
		},
		&entity.CharacterProgress{
			CharacterID: character.ID,
			Level:       5,
			Experience:  125,
			SkillPoints: 2,
			Attack:      3,
			Defense:     2,
			Vitality:    1,
			Gold:        175,
		},
		&entity.Inventory{
			CharacterID: character.ID,
			CreatedAt:   "now",
		},
		&entity.InventoryItem{
			CharacterID: character.ID,
			Slot:        1,
			ItemKey:     "rusty_sword",
			Quantity:    1,
		},
		&entity.CharacterEquipment{
			CharacterID:   character.ID,
			EquipmentSlot: "weapon",
			InventorySlot: 1,
		},
		&entity.CharacterQuest{
			CharacterID: character.ID,
			QuestID:     "first_quest",
			GiverID:     "giver",
			Status:      "active",
			AcceptedAt:  "now",
		},
	} {
		if _, err := database.orm.NewInsert().Model(model).Exec(ctx); err != nil {
			if closeErr := database.Close(); closeErr != nil {
				t.Errorf("close database after insert failure: %v", closeErr)
			}
			t.Fatalf("insert current schema model %T: %v", model, err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close fresh database: %v", err)
	}

	database, err = Open(path)
	if err != nil {
		t.Fatalf("reopen current database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close reopened database: %v", err)
		}
	})

	reloadedCharacter := new(entity.Character)
	if err := database.orm.NewSelect().
		Model(reloadedCharacter).
		Where("id = ?", character.ID).
		Scan(ctx); err != nil {
		t.Fatalf("reload character: %v", err)
	}
	if reloadedCharacter.Role != "moderator" || !reloadedCharacter.Banned {
		t.Fatalf("reloaded character = %#v", reloadedCharacter)
	}
	reloadedProgress := new(entity.CharacterProgress)
	if err := database.orm.NewSelect().
		Model(reloadedProgress).
		Where("character_id = ?", character.ID).
		Scan(ctx); err != nil {
		t.Fatalf("reload progress: %v", err)
	}
	if reloadedProgress.Attack != 3 ||
		reloadedProgress.Defense != 2 ||
		reloadedProgress.Vitality != 1 ||
		reloadedProgress.Gold != 175 {
		t.Fatalf("reloaded progress = %#v", reloadedProgress)
	}
	reloadedEquipment := new(entity.CharacterEquipment)
	if err := database.orm.NewSelect().
		Model(reloadedEquipment).
		Where("character_id = ?", character.ID).
		Scan(ctx); err != nil {
		t.Fatalf("reload equipment: %v", err)
	}
	if reloadedEquipment.EquipmentSlot != "weapon" ||
		reloadedEquipment.InventorySlot != 1 {
		t.Fatalf("reloaded equipment = %#v", reloadedEquipment)
	}
	reloadedQuest := new(entity.CharacterQuest)
	if err := database.orm.NewSelect().
		Model(reloadedQuest).
		Where("character_id = ?", character.ID).
		Scan(ctx); err != nil {
		t.Fatalf("reload quest: %v", err)
	}
	if reloadedQuest.GiverID != "giver" ||
		reloadedQuest.Status != "active" {
		t.Fatalf("reloaded quest = %#v", reloadedQuest)
	}
}
