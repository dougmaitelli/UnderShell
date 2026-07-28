package quest

import (
	"strings"
	"testing"

	"sshrpg/src/enemy"
	"sshrpg/src/item"
)

func TestQuestsResolveObjectiveItemReferences(t *testing.T) {
	quests, err := NewQuests([]Definition{{
		ID: "slime_supplies", Name: "Slime Supplies",
		Objective: Objective{ItemID: "slime_gel", Quantity: 5},
		Reward:    Reward{Gold: 30},
		Dialogue:  validDialogue(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := item.NewItems([]item.Definition{{
		ID: "slime_gel", Name: "Slime Gel", MaxStack: 50,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := quests.ResolveItems(items); err != nil {
		t.Fatal(err)
	}
	definition, _ := quests.Quest("slime_supplies")
	canonical, _ := items.Item("slime_gel")
	if definition.Objective.Item != canonical {
		t.Fatalf("unresolved objective: %#v", definition.Objective)
	}
	enemies, err := enemy.NewEnemies([]enemy.Definition{{
		ID: "slime", Name: "Slime", Visual: []string{"(s)"},
		Health: 1, Experience: 1,
		Drops: []enemy.Drop{{ItemID: "slime_gel", Chance: 1}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := quests.ValidateObjectives(enemies); err != nil {
		t.Fatal(err)
	}
}

func TestQuestsRejectObjectiveWithoutEnemyDrop(t *testing.T) {
	quests, err := NewQuests([]Definition{{
		ID: "slime_supplies", Name: "Slime Supplies",
		Objective: Objective{ItemID: "slime_gel", Quantity: 1},
		Dialogue:  validDialogue(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	enemies, err := enemy.NewEnemies([]enemy.Definition{{
		ID: "bat", Name: "Bat", Visual: []string{"v"},
		Health: 1, Experience: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := quests.ValidateObjectives(enemies); err == nil {
		t.Fatal("expected objective without an enemy drop to fail")
	}
}

func TestQuestsRejectInvalidDefinitions(t *testing.T) {
	_, err := NewQuests([]Definition{{
		ID: "slime_supplies", Name: "Slime Supplies",
		Objective: Objective{ItemID: "slime_gel"},
	}})
	if err == nil || !strings.Contains(err.Error(), "quantity") {
		t.Fatalf("expected invalid quantity, got %v", err)
	}
}

func TestQuestsRejectUnknownObjectiveItem(t *testing.T) {
	quests, err := NewQuests([]Definition{{
		ID: "slime_supplies", Name: "Slime Supplies",
		Objective: Objective{ItemID: "missing", Quantity: 1},
		Dialogue:  validDialogue(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := item.NewItems([]item.Definition{{
		ID: "slime_gel", Name: "Slime Gel", MaxStack: 50,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := quests.ResolveItems(items); err == nil {
		t.Fatal("expected unknown objective item to fail")
	}
}

func validDialogue() Dialogue {
	return Dialogue{
		Offer: "Please help.", InProgress: "Keep looking.",
		Ready: "You found them.", Completed: "Thank you.",
	}
}
