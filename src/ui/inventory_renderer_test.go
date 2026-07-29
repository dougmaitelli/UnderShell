package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"sshrpg/src/domain"
	"sshrpg/src/item"
	"sshrpg/src/world"
)

func TestInventoryRendererCompositesWindowOverGame(t *testing.T) {
	width, height := 50, 14
	background := make([]string, height)
	for row := range background {
		background[row] = strings.Repeat(string(rune('A'+row)), width)
	}

	output := ansi.Strip(InventoryRenderer{}.RenderOver(strings.Join(background, "\n"), width, height, nil))
	rows := strings.Split(output, "\n")
	if len(rows) != height {
		t.Fatalf("rendered height = %d, want %d", len(rows), height)
	}
	if rows[0] != background[0] || rows[height-1] != background[height-1] {
		t.Fatal("inventory window replaced game content outside its bounds")
	}
	if !strings.Contains(output, "INVENTORY") {
		t.Fatal("inventory window was not composited over the game")
	}
}

func TestInventoryRendererShowsResolvedDetailsAndEquipment(t *testing.T) {
	definition := &item.Definition{
		ID: "rusty_sword", Name: "Rusty Sword",
		Description: "A worn but dependable blade.",
		Type:        item.TypeEquipment, EquipmentSlot: item.SlotWeapon,
		Stats: item.EquipmentStats{
			Attack: 1,
		},
		MaxStack: 1,
	}
	inventory := &domain.Inventory{
		CharacterID: 1,
		Items: []domain.InventoryItem{{
			Slot: 1, ItemKey: definition.ID, Quantity: 1,
		}},
		Equipment: []domain.EquippedItem{{
			EquipmentSlot: string(item.SlotWeapon), InventorySlot: 1,
		}},
	}
	view := InventoryView{
		Items: []InventoryItemView{{
			Item: inventory.Items[0], Definition: definition, Equipped: true,
		}},
		Equipment: []EquipmentSlotView{
			{Slot: item.SlotWeapon, ItemName: definition.Name},
		},
	}
	background := strings.Repeat(strings.Repeat(" ", 80)+"\n", 23) +
		strings.Repeat(" ", 80)
	rendered := InventoryRenderer{}.RenderOver(
		background, 80, 24, inventory, view,
	)
	if !strings.Contains(
		rendered,
		equipmentItemNameStyle.Render("Rusty Sword"),
	) {
		t.Fatalf("equipped item name is not yellow: %q", rendered)
	}
	output := ansi.Strip(rendered)
	for _, expected := range []string{
		"Rusty Sword", "Type: Equipment", "Slot: Weapon",
		"Status: Equipped", "Attack: +1", "Weapon: Rusty Sword",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("inventory missing %q: %q", expected, output)
		}
	}
}

func TestInventoryRendererShowsConsumableEffectAndUseAction(t *testing.T) {
	definition := &item.Definition{
		ID: "health_potion", Name: "Health Potion",
		Description: "Restores health.", Type: item.TypeConsumable,
		Effects: []item.Effect{{
			Type: item.EffectRestoreHealth, Amount: 5,
		}},
		MaxStack: 10,
	}
	inventory := &domain.Inventory{
		CharacterID: 1,
		Items: []domain.InventoryItem{{
			Slot: 1, ItemKey: definition.ID, Quantity: 2,
		}},
	}
	view := InventoryView{Items: []InventoryItemView{{
		Item: inventory.Items[0], Definition: definition,
	}}}
	background := strings.Repeat(strings.Repeat(" ", 80)+"\n", 23) +
		strings.Repeat(" ", 80)
	output := ansi.Strip(InventoryRenderer{}.RenderOver(
		background, 80, 24, inventory, view,
	))
	for _, expected := range []string{
		"Health Potion", "Type: Consumable",
		"Effect: Restore 5 health", "E/Space: use",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("inventory missing %q: %q", expected, output)
		}
	}
}

func TestEquipmentStatsAreDerivedFromEquippedInventoryReferences(t *testing.T) {
	items, err := item.NewItems([]item.Definition{
		{
			ID: "sword", Name: "Sword", Type: item.TypeEquipment,
			EquipmentSlot: item.SlotWeapon,
			Stats:         item.EquipmentStats{Attack: 2},
			MaxStack:      1,
		},
		{
			ID: "helmet", Name: "Helmet", Type: item.TypeEquipment,
			EquipmentSlot: item.SlotHelmet,
			Stats:         item.EquipmentStats{Defense: 3},
			MaxStack:      1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	areas, err := world.NewAreas([]world.AreaDefinition{{
		ID: "room", Name: "Room",
		Layout: []string{
			"#####",
			"#...#",
			"#####",
		},
		Spawn: world.Point{X: 1, Y: 1},
	}}, world.References{Items: items})
	if err != nil {
		t.Fatal(err)
	}
	manager := world.New(areas, items, nil, nil)
	defer manager.Close()
	inventory := &domain.Inventory{
		CharacterID: 1,
		Items: []domain.InventoryItem{
			{Slot: 1, ItemKey: "sword", Quantity: 1},
			{Slot: 2, ItemKey: "helmet", Quantity: 1},
		},
		Equipment: []domain.EquippedItem{{
			EquipmentSlot: string(item.SlotWeapon), InventorySlot: 1,
		}},
	}
	model := newGameModel(
		Repositories{}, manager, nil, Identity{},
		&domain.Character{ID: 1}, inventory,
	)
	if stats := model.equipmentStats(inventory); stats != (item.EquipmentStats{Attack: 2}) {
		t.Fatalf("derived equipment stats = %#v", stats)
	}
}

func TestInventoryRendererUsesLatestGameFrame(t *testing.T) {
	renderer := InventoryRenderer{}
	firstGame := "first frame" + strings.Repeat("\n", 23)
	secondGame := "second frame" + strings.Repeat("\n", 23)
	first := ansi.Strip(renderer.RenderOver(firstGame, 80, 24, nil))
	second := ansi.Strip(renderer.RenderOver(secondGame, 80, 24, nil))

	if !strings.Contains(first, "first frame") {
		t.Fatal("first game frame is missing")
	}
	if !strings.Contains(second, "second frame") || strings.Contains(second, "first frame") {
		t.Fatal("inventory did not render over the latest game frame")
	}
}
