package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"sshrpg/src/domain"
	"sshrpg/src/item"
	"sshrpg/src/npc"
	"sshrpg/src/quest"
	"sshrpg/src/world"
)

func TestMovementKeys(t *testing.T) {
	tests := map[string][2]int{
		"w": {0, -1}, "W": {0, -1}, "up": {0, -1},
		"s": {0, 1}, "a": {-1, 0}, "d": {1, 0},
		"x": {0, 0},
	}
	for key, expected := range tests {
		dx, dy := movement(key)
		if dx != expected[0] || dy != expected[1] {
			t.Fatalf("%q: got (%d,%d), want %v", key, dx, dy, expected)
		}
	}
}

func TestMovementIntervalsAccountForTerminalAxis(t *testing.T) {
	if interval := movementInterval(1, 0); interval != horizontalMovementInterval {
		t.Fatalf("horizontal movement interval = %s", interval)
	}
	if interval := movementInterval(0, 1); interval != verticalMovementInterval {
		t.Fatalf("vertical movement interval = %s", interval)
	}
	if interval := movementInterval(1, 1); interval != verticalMovementInterval {
		t.Fatalf("diagonal movement interval = %s", interval)
	}
	if verticalMovementInterval <= horizontalMovementInterval {
		t.Fatal("vertical movement must be slower than horizontal movement")
	}
}

func TestEnhancedMovementKeepsEarlierDirectionHeld(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{ID: 1}, nil)
	model.phase = phasePlaying
	model.movement.enhanced = true

	model.handleMovementPress("a")
	model.movement.inFlight = false
	model.handleMovementPress("s")
	if dx, dy := heldMovement(model.movement.held); dx != -1 || dy != 1 {
		t.Fatalf("held movement = (%d,%d), want (-1,1)", dx, dy)
	}

	_, _ = model.Update(tea.KeyReleaseMsg(tea.Key{Text: "s", Code: 's'}))
	if dx, dy := heldMovement(model.movement.held); dx != -1 || dy != 0 {
		t.Fatalf("movement after releasing S = (%d,%d), want (-1,0)", dx, dy)
	}
}

func TestEnhancedMovementIgnoresTerminalKeyRepeats(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{ID: 1}, nil)
	model.phase = phasePlaying
	model.movement.enhanced = true

	_, initialCommand := model.Update(tea.KeyPressMsg(tea.Key{
		Text: "s",
		Code: 's',
	}))
	if initialCommand == nil {
		t.Fatal("initial movement press did not start movement")
	}
	model.movement.inFlight = false
	walkSteps := model.movement.walkSteps

	_, repeatCommand := model.Update(tea.KeyPressMsg(tea.Key{
		Text:     "s",
		Code:     's',
		IsRepeat: true,
	}))
	if repeatCommand != nil {
		t.Fatal("enhanced movement accepted a terminal key-repeat event")
	}
	if model.movement.inFlight {
		t.Fatal("terminal key repeat started an extra movement request")
	}
	if model.movement.walkSteps != walkSteps {
		t.Fatal("terminal key repeat advanced the walking animation")
	}
}

func TestBasicMovementUsesTerminalKeyRepeats(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{ID: 1}, nil)
	model.phase = phasePlaying

	_, command := model.Update(tea.KeyPressMsg(tea.Key{
		Text:     "s",
		Code:     's',
		IsRepeat: true,
	}))
	if command == nil || !model.movement.inFlight {
		t.Fatal("basic movement did not accept a terminal key-repeat event")
	}
	_ = model.View()
	model.movement.inFlight = false
	_, command = model.Update(tea.KeyPressMsg(tea.Key{
		Text:     "s",
		Code:     's',
		IsRepeat: true,
	}))
	if command != nil || model.movement.inFlight || model.renderDirty {
		t.Fatal("movement repeat bypassed the client movement rate limit")
	}
}

func TestWalkingAnimationAlternatesAndReturnsToStanding(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{ID: 1}, nil)

	model.movement.step()
	firstGeneration := model.movement.walkGeneration
	if model.movement.walkFrame != 1 {
		t.Fatalf("first walk frame = %d, want 1", model.movement.walkFrame)
	}

	model.movement.step()
	if model.movement.walkFrame != 1 {
		t.Fatalf("second walk frame = %d, want held frame 1", model.movement.walkFrame)
	}
	model.movement.step()
	if model.movement.walkFrame != 2 {
		t.Fatalf("third walk frame = %d, want 2", model.movement.walkFrame)
	}
	model.movement.finishStep(firstGeneration)
	if model.movement.walkFrame != 2 {
		t.Fatal("an older animation timer stopped the current step")
	}
	model.movement.finishStep(model.movement.walkGeneration)
	if model.movement.walkFrame != 0 {
		t.Fatalf("finished walk frame = %d, want standing frame 0", model.movement.walkFrame)
	}
}

func TestRejectedMovementReusesPreviousRender(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", AreaID: "meadow", X: 1, Y: 1,
	}, nil)
	model.phase = phasePlaying
	initial := model.View().Content

	_, command := model.Update(playerMovedMsg{
		player: world.Player{
			ID: 1, Name: "Aria", AreaID: "meadow", X: 1, Y: 1,
		},
	})
	if command != nil || model.renderDirty {
		t.Fatal("rejected movement scheduled work or invalidated the render")
	}
	if rendered := model.View().Content; rendered != initial {
		t.Fatal("rejected movement changed the rendered frame")
	}
}

func TestInventoryCanBeOpenedAndClosed(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{ID: 1, Name: "Aria"}, nil)
	model.phase = phasePlaying
	model.width, model.height = 80, 24
	model.movement.held["left"] = true

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	if model.mode != inputModeInventory {
		t.Fatal("inventory did not open")
	}
	if len(model.movement.held) != 0 {
		t.Fatal("opening inventory did not clear held movement")
	}
	output := ansi.Strip(model.View().Content)
	if !strings.Contains(output, "INVENTORY") || !strings.Contains(output, "Your inventory is empty.") {
		t.Fatalf("inventory window not rendered: %q", output)
	}

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if model.mode == inputModeInventory {
		t.Fatal("inventory did not close with Escape")
	}

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	if model.mode == inputModeInventory {
		t.Fatal("inventory did not close with I")
	}
}

func TestSkillsAndInventoryMenusAreMutuallyExclusive(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", Level: 2, SkillPoints: 2, Attack: 1,
	}, nil)
	model.phase = phasePlaying
	model.width, model.height = 80, 24

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "k", Code: 'k'}))
	if model.mode != inputModeSkills {
		t.Fatalf("skills did not open exclusively: mode %v", model.mode)
	}
	plain := ansi.Strip(model.View().Content)
	if !strings.Contains(plain, "SKILLS") ||
		!strings.Contains(plain, "Unspent points: 2") ||
		!strings.Contains(plain, "[1] Attack   1") {
		t.Fatalf("skills menu not rendered: %q", plain)
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	if model.mode != inputModeSkills {
		t.Fatal("inventory opened over skills")
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "k", Code: 'k'}))
	if model.mode != inputModeInventory {
		t.Fatal("skills opened over inventory")
	}
}

func TestChatFocusCapturesTypingUntilEnter(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", Level: 1,
	}, nil)
	model.phase = phasePlaying
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}))
	if model.mode != inputModeChat {
		t.Fatal("T did not focus chat")
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	if model.mode == inputModeInventory || model.chat.input.Value() != "i" {
		t.Fatalf("chat did not capture gameplay key: inventory %v, input %q",
			model.mode == inputModeInventory, model.chat.input.Value())
	}
	_, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if model.mode == inputModeChat || command == nil {
		t.Fatalf("Enter did not send and return focus: focused %v, command %v",
			model.mode == inputModeChat, command != nil)
	}
}

func TestAttackKeyStartsSlashAnimation(t *testing.T) {
	for name, key := range map[string]tea.Key{
		"x":     {Text: "x", Code: 'x'},
		"space": {Code: tea.KeySpace},
	} {
		t.Run(name, func(t *testing.T) {
			model := newGameModel(
				Repositories{}, nil, nil, Identity{},
				&domain.Character{
					ID: 1, Name: "Aria", AreaID: "meadow", X: 2, Y: 1,
				},
				nil,
			)
			model.phase = phasePlaying
			model.movement.setFacing(-1, 0)
			model.movement.setFacing(0, -1)
			_, command := model.Update(tea.KeyPressMsg(key))
			if command == nil || !model.actions.attackInFlight ||
				model.actions.attackFrame != 1 ||
				model.actions.attackDirection != -1 {
				t.Fatalf(
					"attack state = in-flight %v, frame %d, direction %d, command %v",
					model.actions.attackInFlight, model.actions.attackFrame,
					model.actions.attackDirection, command != nil,
				)
			}
		})
	}
}

func TestPickupKeyRequestsNearbyDrop(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", AreaID: "meadow", X: 2, Y: 1,
	}, nil)
	model.phase = phasePlaying
	_, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	if command == nil || !model.actions.pickupInFlight {
		t.Fatalf("pickup state = in-flight %v, command %v", model.actions.pickupInFlight, command != nil)
	}
}

func TestInteractOpensNearbyShopBeforePickup(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", AreaID: "market", X: 2, Y: 1, Gold: 100,
	}, &domain.Inventory{CharacterID: 1})
	model.phase = phasePlaying
	model.width, model.height = 80, 24
	items, err := item.NewItems([]item.Definition{{
		ID: "potion", Name: "Potion",
		Type: item.TypeConsumable,
		Effects: []item.Effect{{
			Type: item.EffectRestoreHealth, Amount: 5,
		}},
		MaxStack: 10,
	}})
	if err != nil {
		t.Fatal(err)
	}
	areas, err := world.NewAreas([]world.AreaDefinition{{
		ID: "market", Name: "Market",
		Layout: []string{"########", "#......#", "########"},
		Spawn:  world.Point{X: 1, Y: 1},
		NPCs: []npc.Config{{
			ID: "merchant", Name: "Mira", Type: npc.TypeShop, X: 3, Y: 1,
			Stock: []npc.ShopItemConfig{{
				ItemID: "potion", BuyPrice: 10,
			}},
		}},
	}}, world.References{Items: items})
	if err != nil {
		t.Fatal(err)
	}
	area, _ := areas.Area("market")
	model.connection.snapshot = world.Snapshot{
		Area: area,
		Players: []world.Player{{
			ID: 1, Name: "Aria", AreaID: "market", X: 2, Y: 1,
		}},
		Drops: []world.GroundItem{{
			ID: 1, AreaID: "market", X: 2, Y: 1,
		}},
	}
	potion, _ := items.Item("potion")
	model.connection.snapshot.Drops[0].Item = potion

	_, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	if command != nil || model.mode != inputModeShop || model.actions.pickupInFlight {
		t.Fatalf(
			"interaction = mode %v, pickup %v, command %v",
			model.mode, model.actions.pickupInFlight, command != nil,
		)
	}
	plain := ansi.Strip(model.View().Content)
	for _, expected := range []string{"Mira's Shop", "[BUY]", "Potion", "10 gold", "Gold: 100"} {
		if !strings.Contains(plain, expected) {
			t.Fatalf("shop is missing %q: %q", expected, plain)
		}
	}
	_, command = model.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	if command == nil || model.mode != inputModeShop || model.shop.npc == nil {
		t.Fatal("E did not trade while keeping the shop open")
	}
	model.shop.inFlight = false
	_, command = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
	if command == nil || model.mode != inputModeShop || model.shop.npc == nil {
		t.Fatal("Space did not trade while keeping the shop open")
	}
	model.shop.inFlight = false
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	if model.shop.tab != shopTabSell {
		t.Fatal("Tab did not switch the shop to selling")
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if model.mode != inputModeGame || model.shop.npc != nil {
		t.Fatal("Escape did not close the shop")
	}
}

func TestQuestGiverInteractionAndJournal(t *testing.T) {
	items, err := item.NewItems([]item.Definition{{
		ID: "slime_gel", Name: "Slime Gel",
		Type: item.TypeMaterial, MaxStack: 50,
	}})
	if err != nil {
		t.Fatal(err)
	}
	slimeGel, _ := items.Item("slime_gel")
	definitions, err := quest.NewQuests([]quest.Definition{{
		ID: "slime_supplies", Name: "Slime Supplies",
		Description: "Collect gel from meadow slimes.",
		Objective: quest.Objective{
			Item: slimeGel, Quantity: 5,
		},
		Reward: quest.Reward{Gold: 30},
		Dialogue: quest.Dialogue{
			Offer:      "The meadow slimes are spoiling my mixtures.",
			InProgress: "I still need more Slime Gel.",
			Ready:      "That is enough Slime Gel.",
			Completed:  "Thank you for your earlier help.",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", AreaID: "meadow", X: 2, Y: 1,
	}, &domain.Inventory{
		CharacterID: 1,
		Items: []domain.InventoryItem{{
			Slot: 1, ItemKey: "slime_gel", Quantity: 2,
		}},
	})
	model.phase = phasePlaying
	model.width, model.height = 80, 24
	model.quests = newQuestState(definitions)
	areas, err := world.NewAreas([]world.AreaDefinition{{
		ID: "meadow", Name: "Meadow",
		Layout: []string{"########", "#......#", "########"},
		Spawn:  world.Point{X: 1, Y: 1},
		NPCs: []npc.Config{{
			ID: "orin", Name: "Orin", Type: npc.TypeQuestGiver, X: 3, Y: 1,
			QuestIDs: []string{"slime_supplies"},
		}},
	}}, world.References{Items: items, Quests: definitions})
	if err != nil {
		t.Fatal(err)
	}
	area, _ := areas.Area("meadow")
	model.connection.snapshot = world.Snapshot{Area: area}

	_, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	if command != nil || model.mode != inputModeQuestDialogue ||
		model.quests.inFlight || model.actions.pickupInFlight {
		t.Fatal("E did not open quest dialogue before item pickup")
	}
	plain := ansi.Strip(model.View().Content)
	if !strings.Contains(plain, "The meadow slimes are spoiling my mixtures.") ||
		!strings.Contains(plain, "E/Space: accept quest") {
		t.Fatalf("offer dialogue was not rendered: %q", plain)
	}
	offerRow := -1
	for row, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "The meadow slimes are spoiling my mixtures.") {
			offerRow = row
			break
		}
	}
	if offerRow < model.height/2 {
		t.Fatalf("quest dialogue was not anchored near the bottom: row %d", offerRow)
	}
	_, command = model.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	if command == nil || !model.quests.inFlight || model.mode != inputModeGame {
		t.Fatal("E did not accept the quest from its dialogue")
	}

	model.quests.inFlight = false
	model.quests.progress["slime_supplies"] = domain.CharacterQuest{
		QuestID: "slime_supplies", GiverID: "orin",
		Status: domain.QuestActive,
	}
	_, command = model.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	if command != nil || model.mode != inputModeQuestDialogue ||
		model.quests.inFlight {
		t.Fatal("incomplete quest did not open progress dialogue")
	}
	plain = ansi.Strip(model.View().Content)
	if !strings.Contains(plain, "I still need more Slime Gel.") {
		t.Fatalf("progress dialogue was not rendered: %q", plain)
	}
	if strings.Contains(plain, "Slime Gel: 2/5") ||
		strings.Contains(plain, "Reward: 30 gold") {
		t.Fatalf("quest metadata leaked into NPC dialogue: %q", plain)
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	if model.mode != inputModeJournal {
		t.Fatal("J did not open the quest journal")
	}
	plain = ansi.Strip(model.View().Content)
	for _, expected := range []string{
		"QUEST JOURNAL", "> Slime Supplies", "Objective: Slime Gel",
		"In progress — 2 of 5",
		"Return to: Orin — Meadow", "Reward: 30 gold",
	} {
		if !strings.Contains(plain, expected) {
			t.Fatalf("journal is missing %q: %q", expected, plain)
		}
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if model.mode != inputModeGame {
		t.Fatal("Escape did not close the quest journal")
	}

	model.inventory.Items[0].Quantity = 5
	_, command = model.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	if command != nil || model.mode != inputModeQuestDialogue {
		t.Fatal("ready quest did not open turn-in dialogue")
	}
	plain = ansi.Strip(model.View().Content)
	if !strings.Contains(plain, "That is enough Slime Gel.") ||
		!strings.Contains(plain, "E/Space: hand over items") {
		t.Fatalf("turn-in dialogue was not rendered: %q", plain)
	}
	_, command = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
	if command == nil || !model.quests.inFlight || model.mode != inputModeGame {
		t.Fatal("Space did not submit the quest from its dialogue")
	}

	model.quests.inFlight = false
	progress := model.quests.progress["slime_supplies"]
	progress.Status = domain.QuestCompleted
	model.quests.progress["slime_supplies"] = progress
	_, command = model.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	if command != nil || model.mode != inputModeQuestDialogue {
		t.Fatal("completed quest did not open follow-up dialogue")
	}
	plain = ansi.Strip(model.View().Content)
	if !strings.Contains(plain, "Thank you for your earlier help.") {
		t.Fatalf("completed dialogue was not rendered: %q", plain)
	}
}

func TestQuestDialogueUsesAvailableTerminalWidth(t *testing.T) {
	message := "The cavern bats are elusive. Keep searching until you have three intact Bat Wings."
	background := strings.Repeat(strings.Repeat(" ", 100)+"\n", 23) +
		strings.Repeat(" ", 100)
	plain := ansi.Strip(QuestDialogueRenderer{}.RenderOver(
		background, 100, 24,
		QuestDialogueView{NPCName: "Orin", Text: message},
	))
	if !strings.Contains(plain, message) {
		t.Fatalf("dialogue wrapped despite fitting in the terminal: %q", plain)
	}
}

func TestJournalRendererUsesSelectedQuestForDetails(t *testing.T) {
	background := strings.Repeat(strings.Repeat(" ", 80)+"\n", 23) +
		strings.Repeat(" ", 80)
	output := ansi.Strip((JournalRenderer{}).RenderOver(
		background, 80, 24,
		JournalView{
			Selected: 1,
			Quests: []QuestView{
				{
					Name: "First Quest", Description: "First description.",
					ItemName: "First Item", Current: 1, Required: 2,
					GiverName: "First NPC",
				},
				{
					Name: "Second Quest",
					Description: "A deliberately long description that should wrap safely " +
						"inside the right pane continuation-marker.",
					ItemName: "Second Item", Current: 3, Required: 3,
					GiverName: "Second NPC", GiverArea: "Cavern", RewardGold: 20,
				},
			},
		},
	))
	for _, expected := range []string{
		"First Quest", "> Second Quest", "deliberately long description",
		"continuation-marker",
		"Objective: Second Item", "Ready to return",
		"Return to: Second NPC — Cavern", "Reward: 20 gold",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("journal is missing %q: %q", expected, output)
		}
	}
	if strings.Contains(output, "First description.") ||
		strings.Contains(output, "Objective: First Item") {
		t.Fatalf("journal showed details for an unselected quest: %q", output)
	}
	foundContinuation := false
	for _, line := range strings.Split(output, "\n") {
		if lipgloss.Width(line) > 80 {
			t.Fatalf("journal line exceeded terminal width: %q", line)
		}
		if !strings.Contains(line, "continuation-marker") {
			continue
		}
		foundContinuation = true
		pipes := make([]int, 0, 3)
		for index, character := range []rune(line) {
			if character == '│' {
				pipes = append(pipes, index)
			}
		}
		textColumn := len([]rune(strings.Split(line, "continuation-marker")[0]))
		if len(pipes) < 3 || textColumn <= pipes[1] {
			t.Fatalf("right-pane continuation crossed the divider: %q", line)
		}
	}
	if !foundContinuation {
		t.Fatal("wrapped journal continuation was not rendered")
	}
}

func TestJournalInputMovesAndWrapsSelection(t *testing.T) {
	dialogue := quest.Dialogue{
		Offer: "Offer.", InProgress: "Progress.",
		Ready: "Ready.", Completed: "Complete.",
	}
	definitions, err := quest.NewQuests([]quest.Definition{
		{
			ID: "first", Name: "First",
			Objective: quest.Objective{
				Item:     &item.Definition{ID: "first_item", Name: "First Item", MaxStack: 1},
				Quantity: 1,
			},
			Dialogue: dialogue,
		},
		{
			ID: "second", Name: "Second",
			Objective: quest.Objective{
				Item:     &item.Definition{ID: "second_item", Name: "Second Item", MaxStack: 1},
				Quantity: 1,
			},
			Dialogue: dialogue,
		},
		{
			ID: "finished", Name: "Finished Quest",
			Description: "A completed quest with retained details.",
			Objective: quest.Objective{
				Item: &item.Definition{
					ID: "finished_item", Name: "Finished Item", MaxStack: 1,
				},
				Quantity: 3,
			},
			Reward:   quest.Reward{Gold: 25},
			Dialogue: dialogue,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	model := newGameModel(
		Repositories{}, nil, nil, Identity{},
		&domain.Character{ID: 1}, &domain.Inventory{CharacterID: 1},
	)
	model.phase = phasePlaying
	model.mode = inputModeJournal
	model.quests = newQuestState(definitions)
	model.quests.setProgress([]domain.CharacterQuest{
		{QuestID: "first", Status: domain.QuestActive},
		{QuestID: "second", Status: domain.QuestActive},
		{
			QuestID: "finished", GiverID: "archivist",
			Status: domain.QuestCompleted,
		},
	})

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if model.quests.journalSelected != 1 {
		t.Fatalf("down selected %d, want 1", model.quests.journalSelected)
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "s", Code: 's'}))
	if model.quests.journalSelected != 0 {
		t.Fatalf("down wrap selected %d, want 0", model.quests.journalSelected)
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if model.quests.journalSelected != 1 {
		t.Fatalf("up wrap selected %d, want 1", model.quests.journalSelected)
	}

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	if model.quests.journalTab != journalTabCompleted ||
		model.quests.journalSelected != 0 {
		t.Fatalf(
			"completed tab state = tab %d selected %d",
			model.quests.journalTab,
			model.quests.journalSelected,
		)
	}
	journal := model.quests.journalView(model.inventory, model.questGiver)
	if len(journal.Quests) != 1 ||
		journal.Quests[0].Name != "Finished Quest" ||
		journal.Quests[0].Status != domain.QuestCompleted ||
		journal.Quests[0].Current != 3 ||
		journal.Quests[0].Required != 3 ||
		journal.Quests[0].RewardGold != 25 {
		t.Fatalf("completed journal = %#v", journal)
	}
	model.width, model.height = 80, 24
	plain := ansi.Strip(model.View().Content)
	for _, expected := range []string{
		"ACTIVE  [COMPLETED]",
		"> Finished Quest",
		"completed quest with retained details",
		"Completed — 3 of 3",
		"Quest giver: archivist",
		"Reward: 25 gold",
	} {
		if !strings.Contains(plain, expected) {
			t.Fatalf("completed journal is missing %q: %q", expected, plain)
		}
	}

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	if model.quests.journalTab != journalTabActive ||
		model.quests.journalSelected != 0 {
		t.Fatalf(
			"active tab state = tab %d selected %d",
			model.quests.journalTab,
			model.quests.journalSelected,
		)
	}

	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	if model.mode != inputModeGame {
		t.Fatal("J did not close the completed journal tab")
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	if model.mode != inputModeJournal ||
		model.quests.journalTab != journalTabActive ||
		model.quests.journalSelected != 0 {
		t.Fatalf(
			"reopened journal = mode %d tab %d selected %d",
			model.mode,
			model.quests.journalTab,
			model.quests.journalSelected,
		)
	}
}

func TestJournalWrappedDetailsStayInRightPaneAtSupportedWidths(t *testing.T) {
	for _, width := range []int{40, 50, 80} {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			background := strings.Repeat(strings.Repeat(" ", width)+"\n", 23) +
				strings.Repeat(" ", width)
			output := ansi.Strip((JournalRenderer{}).RenderOver(
				background, width, 24,
				JournalView{Quests: []QuestView{{
					Name:        "Quest",
					Description: "alpha beta gamma delta epsilon RIGHTMARKER",
					ItemName:    "Item", Current: 1, Required: 2,
					GiverName: "NPC", GiverArea: "Area",
				}}},
			))
			foundMarker := false
			for _, line := range strings.Split(output, "\n") {
				if lipgloss.Width(line) > width {
					t.Fatalf("line width exceeds %d: %q", width, line)
				}
				if !strings.Contains(line, "RIGHTMARKER") {
					continue
				}
				foundMarker = true
				pipes := make([]int, 0, 3)
				for index, character := range []rune(line) {
					if character == '│' {
						pipes = append(pipes, index)
					}
				}
				markerColumn := len([]rune(strings.Split(line, "RIGHTMARKER")[0]))
				if len(pipes) < 3 || markerColumn <= pipes[1] {
					t.Fatalf("wrapped detail crossed divider at width %d: %q", width, line)
				}
			}
			if !foundMarker {
				t.Fatalf("right-pane marker missing at width %d", width)
			}
		})
	}
}

func TestGameRenderSanitizesViewportByConstruction(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", AreaID: "meadow", X: 2, Y: 1,
	}, nil)
	areas, err := world.NewAreas([]world.AreaDefinition{{
		ID: "meadow", Name: "Meadow",
		Layout: []string{"######", "#....#", "######"},
		Spawn:  world.Point{X: 1, Y: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	area, _ := areas.Area("meadow")
	model.phase = phasePlaying
	model.width, model.height = 50, 15
	model.connection.snapshot = world.Snapshot{Area: area, Players: []world.Player{
		{
			ID: 1, Name: "Aria", AreaID: "meadow", X: 2, Y: 1,
			Level: 2, Experience: 25, SkillPoints: 1, Health: 8, MaxHealth: 10,
		},
		{ID: 2, Name: "Rowan", AreaID: "meadow", X: 12, Y: 1},
	}}
	output := model.renderer.game.Render(model.viewState())
	plain := ansi.Strip(output)
	if !strings.Contains(plain, "Aria") ||
		!strings.Contains(plain, "Rowan") ||
		!strings.Contains(plain, "○ Rowan") ||
		!strings.Contains(plain, "/|\\") ||
		!strings.Contains(plain, "/ \\") {
		t.Fatalf("players not rendered: %q", output)
	}
	if !strings.Contains(output, "Players here: 2") {
		t.Fatalf("player count not rendered: %q", output)
	}
	if !strings.Contains(output, "Nearby: Rowan") {
		t.Fatalf("nearby player name not rendered: %q", output)
	}
	if !strings.Contains(output, "F1: help") {
		t.Fatalf("help control not rendered: %q", output)
	}
	if !strings.Contains(plain, "Lv 2 • XP 25/400 • SP 1 • HP 8/10") {
		t.Fatalf("player progression not rendered: %q", plain)
	}
}

func TestHelpWindowListsCommandsAndBlocksGameplayInput(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", Level: 1,
	}, nil)
	model.phase = phasePlaying
	model.width, model.height = 80, 24
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	if model.mode != inputModeHelp {
		t.Fatal("F1 did not open help")
	}
	plain := ansi.Strip(model.View().Content)
	for _, command := range []string{
		"HELP", "WASD / arrows", "Attack nearby", "Open inventory",
		"Open skills", "Focus chat", "Switch shop or journal tab", "Ctrl+C",
	} {
		if !strings.Contains(plain, command) {
			t.Fatalf("help is missing %q: %q", command, plain)
		}
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	if model.mode == inputModeInventory {
		t.Fatal("gameplay shortcut opened inventory over help")
	}
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if model.mode == inputModeHelp {
		t.Fatal("Escape did not close help")
	}
}

func TestGroundItemsUseOneGenericMarker(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, &domain.Character{
		ID: 1, Name: "Aria", AreaID: "meadow", X: 10, Y: 5,
	}, nil)
	model.phase = phasePlaying
	model.width, model.height = 40, 14
	model.connection.snapshot = world.Snapshot{
		Players: []world.Player{{ID: 1, Name: "Aria", AreaID: "meadow", X: 10, Y: 5}},
		Drops: []world.GroundItem{
			{ID: 1, AreaID: "meadow", X: 3, Y: 5},
			{ID: 2, AreaID: "meadow", X: 17, Y: 5},
		},
	}
	plain := ansi.Strip(model.renderer.game.Render(model.viewState()))
	if count := strings.Count(plain, "◆"); count != 2 {
		t.Fatalf("generic ground item marker count = %d, want 2: %q", count, plain)
	}
	if strings.Contains(plain, "SP 0") {
		t.Fatalf("zero skill points should not be rendered: %q", plain)
	}
}

func TestPlayerBaseIsAnchoredAtWorldCoordinate(t *testing.T) {
	grid := newGameGrid(12, 8)
	drawPlayer(
		grid, 6, 6, "@", "Aria", 0, 1, 0,
		domain.CharacterRoleUser, 0,
	)

	if ansi.Strip(grid.renderedCell(5, 6)) != "/" ||
		grid.renderedCell(6, 6) != " " ||
		ansi.Strip(grid.renderedCell(7, 6)) != "\\" {
		t.Fatal("feet are not centered on base coordinate")
	}
	row := ansi.Strip(strings.Split(grid.render(), "\n")[3])
	if row != "   @ Aria   " {
		t.Fatalf("name is not centered above the figure: %q", row)
	}
}

func TestPlayerRoleColorOnlyAppliesToName(t *testing.T) {
	grid := newGameGrid(12, 8)
	adminName := playerNameStyle(domain.CharacterRoleAdmin, 0, 2)
	drawPlayer(
		grid, 6, 6, "@", "DougM", 0, 0, 0,
		domain.CharacterRoleAdmin, 0,
	)

	if grid.renderedCell(5, 3) != adminName.Render("D") {
		t.Fatalf("admin name cell has the wrong style: %q", grid.renderedCell(5, 3))
	}
	if grid.renderedCell(6, 4) != playerBodyStyle.Render("O") {
		t.Fatalf("player body does not use the neutral style: %q", grid.renderedCell(6, 4))
	}
	if grid.renderedCell(6, 4) == adminName.Render("O") {
		t.Fatal("admin name color leaked into the player body")
	}
}

func TestWalkingLegsExtendOnlyTowardMovement(t *testing.T) {
	if legs := playerLegs(1, 1, 0); legs != "  |\\" {
		t.Fatalf("right-facing legs = %q", legs)
	}
	if legs := playerLegs(1, -1, 0); legs != " /| " {
		t.Fatalf("left-facing legs = %q", legs)
	}
	if legs := playerLegs(1, 0, -1); len([]rune(legs)) != 3 {
		t.Fatalf("vertical legs are too wide: %q", legs)
	}
	if legs := playerLegs(2, 1, 0); legs != "/ \\" {
		t.Fatalf("return pose = %q", legs)
	}
}

func TestSlashFramesAreDirectional(t *testing.T) {
	grid := newGameGrid(12, 8)
	drawSlash(grid, 5, 5, 1, 1)
	if ansi.Strip(grid.renderedCell(7, 3)) != "/" ||
		ansi.Strip(grid.renderedCell(8, 4)) != "/" {
		t.Fatalf(
			"first right-facing slash frame = %q, %q",
			grid.renderedCell(7, 3), grid.renderedCell(8, 4),
		)
	}
	drawSlash(grid, 5, 5, 1, 2)
	for _, x := range []int{6, 7, 8} {
		if ansi.Strip(grid.renderedCell(x, 4)) != "─" {
			t.Fatalf("second right-facing slash frame at %d = %q", x, grid.renderedCell(x, 4))
		}
	}
	drawSlash(grid, 5, 5, -1, 1)
	if ansi.Strip(grid.renderedCell(3, 3)) != "\\" ||
		ansi.Strip(grid.renderedCell(2, 4)) != "\\" {
		t.Fatalf(
			"first left-facing slash frame = %q, %q",
			grid.renderedCell(3, 3), grid.renderedCell(2, 4),
		)
	}
}

func TestPlayerStylesEmitANSIColors(t *testing.T) {
	userLocal := playerNameStyle(
		domain.CharacterRoleUser, 0, 0,
	).Render("@ Aria")
	userRemote := playerNameStyle(
		domain.CharacterRoleUser, 4, 6,
	).Render("○ Rowan")
	admin := playerNameStyle(
		domain.CharacterRoleAdmin, 0, 0,
	).Render("@ DougM")
	if userLocal == ansi.Strip(userLocal) {
		t.Fatal("user player style did not emit ANSI styling")
	}
	if admin == ansi.Strip(admin) {
		t.Fatal("admin player style did not emit ANSI styling")
	}
	userLocalCodes := strings.Replace(userLocal, "@ Aria", "", 1)
	userRemoteCodes := strings.Replace(userRemote, "○ Rowan", "", 1)
	adminCodes := strings.Replace(admin, "@ DougM", "", 1)
	if userLocalCodes != userRemoteCodes {
		t.Fatal("local and remote users received different colors")
	}
	if adminCodes == userLocalCodes {
		t.Fatal("admin and user players received the same color")
	}
	userRed, userGreen, userBlue, _ :=
		playerNameStyle(
			domain.CharacterRoleUser, 0, 0,
		).GetForeground().RGBA()
	if userRed != 0x38*0x101 ||
		userGreen != 0xBD*0x101 ||
		userBlue != 0xF8*0x101 {
		t.Fatalf(
			"user color = (%x, %x, %x), want #38BDF8",
			userRed, userGreen, userBlue,
		)
	}
	adminColors := make(map[[3]uint32]bool)
	for index := range adminPlayerNameStyles {
		red, green, blue, _ := playerNameStyle(
			domain.CharacterRoleAdmin, 0, index,
		).GetForeground().RGBA()
		if red <= green || red <= blue {
			t.Fatalf(
				"admin shimmer color is not red: (%x, %x, %x)",
				red, green, blue,
			)
		}
		if green > 0x44*0x101 || blue > 0x44*0x101 {
			t.Fatalf(
				"admin shimmer color is too pale: (%x, %x, %x)",
				red, green, blue,
			)
		}
		adminColors[[3]uint32{red, green, blue}] = true
	}
	if len(adminColors) < 3 {
		t.Fatalf("admin shimmer has only %d distinct reds", len(adminColors))
	}
	firstRed, firstGreen, firstBlue, _ := playerNameStyle(
		domain.CharacterRoleAdmin, 0, 4,
	).GetForeground().RGBA()
	nextRed, nextGreen, nextBlue, _ := playerNameStyle(
		domain.CharacterRoleAdmin, 1, 4,
	).GetForeground().RGBA()
	if firstRed == nextRed &&
		firstGreen == nextGreen &&
		firstBlue == nextBlue {
		t.Fatal("admin shimmer did not advance between frames")
	}
	crest := adminPlayerNameStyles[4].GetForeground()
	if playerNameStyle(
		domain.CharacterRoleAdmin, 0, 4,
	).GetForeground() != crest ||
		playerNameStyle(
			domain.CharacterRoleAdmin, 1, 3,
		).GetForeground() != crest {
		t.Fatal("admin shimmer crest does not move from right to left")
	}
}

func TestPlayerNameShimmerRunsOnlyWhileAdminNameIsShown(t *testing.T) {
	state := playerNameShimmerState{}
	if command := state.setNeeded([]world.Player{{
		ID: 1, Role: domain.CharacterRoleUser,
	}}, nil); command != nil || state.active {
		t.Fatal("user-only snapshot started the admin shimmer")
	}
	command := state.setNeeded([]world.Player{{
		ID: 2, Role: domain.CharacterRoleAdmin,
	}}, nil)
	if command == nil || !state.active {
		t.Fatal("visible admin did not start the shimmer")
	}
	generation := state.generation
	if command := state.advance(generation); command == nil || state.frame != 1 {
		t.Fatal("active shimmer did not advance and reschedule")
	}
	for range len(adminPlayerNameStyles) - 2 {
		if command := state.advance(generation); command == nil {
			t.Fatal("shimmer stopped before completing one sweep")
		}
	}
	if command := state.advance(generation); command != nil {
		t.Fatal("shimmer continued scheduling redraws after one sweep")
	}
	if command := state.setNeeded([]world.Player{{
		ID: 1, Role: domain.CharacterRoleUser,
	}}, nil); command != nil || state.active || state.frame != 0 {
		t.Fatal("shimmer did not stop when the admin disappeared")
	}
	if command := state.advance(generation); command != nil || state.frame != 0 {
		t.Fatal("stale shimmer tick restarted the animation")
	}
	if command := state.setNeeded(nil, []world.ChatMessage{{
		PlayerRole: domain.CharacterRoleAdmin,
	}}); command == nil || !state.active {
		t.Fatal("admin chat message did not start the shimmer")
	}
}

func TestModeratorNameUsesYellowShimmer(t *testing.T) {
	colors := make(map[[3]uint32]bool)
	for index := range moderatorPlayerNameStyles {
		red, green, blue, _ := playerNameStyle(
			domain.CharacterRoleModerator, 0, index,
		).GetForeground().RGBA()
		if red <= blue || green <= blue {
			t.Fatalf(
				"moderator shimmer color is not yellow: (%x, %x, %x)",
				red, green, blue,
			)
		}
		colors[[3]uint32{red, green, blue}] = true
	}
	if len(colors) < 3 {
		t.Fatalf("moderator shimmer has only %d distinct yellows", len(colors))
	}
	crest := moderatorPlayerNameStyles[4].GetForeground()
	if playerNameStyle(
		domain.CharacterRoleModerator, 0, 4,
	).GetForeground() != crest ||
		playerNameStyle(
			domain.CharacterRoleModerator, 1, 3,
		).GetForeground() != crest {
		t.Fatal("moderator shimmer crest does not move from right to left")
	}

	state := playerNameShimmerState{}
	if command := state.setNeeded([]world.Player{{
		ID: 1, Role: domain.CharacterRoleModerator,
	}}, nil); command == nil || !state.active {
		t.Fatal("visible moderator did not start the shimmer")
	}
}

func TestWelcomeBorderRowsHaveEqualWidth(t *testing.T) {
	box := welcomeBoxStyle.Render(strings.Join([]string{
		titleStyle.Render("UnderShell"),
		"",
		"Your SSH key has no character yet.",
		"",
		"Character name: Aria",
		"",
		"Enter to begin • Ctrl+C to leave",
	}, "\n"))
	width := lipgloss.Width(box)
	for _, line := range strings.Split(box, "\n") {
		if lineWidth := lipgloss.Width(line); lineWidth != width {
			t.Errorf("welcome row width = %d, want %d: %q", lineWidth, width, line)
		}
	}
}

func TestWelcomeViewUsesTerminalDimensions(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, nil, nil)
	model.width, model.height = 80, 24
	output := model.renderer.welcome.Render(model.viewState())
	if width, height := lipgloss.Size(output); width != 80 || height != 24 {
		t.Fatalf("welcome size = %dx%d, want 80x24", width, height)
	}
}
