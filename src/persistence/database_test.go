package persistence

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"sshrpg/src/persistence/entity"
)

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
			database.Close()
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
	defer database.Close()

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
