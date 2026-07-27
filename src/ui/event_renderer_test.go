package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"sshrpg/src/domain"
	"sshrpg/src/world"
)

func TestEventOverlayShowsOnlyLastTenLines(t *testing.T) {
	events := make([]EventView, 13)
	for index := range events {
		events[index] = EventView{
			Kind: world.EventCombat, Message: fmt.Sprintf("event-%02d", index),
		}
	}
	game := strings.Repeat(strings.Repeat(" ", 80)+"\n", 20)
	plain := ansi.Strip(EventRenderer{}.RenderOver(game, 80, 24, events))
	for index := 0; index < 3; index++ {
		if strings.Contains(plain, fmt.Sprintf("event-%02d", index)) {
			t.Fatalf("old event %d remains visible: %q", index, plain)
		}
	}
	for index := 3; index < 13; index++ {
		if !strings.Contains(plain, fmt.Sprintf("event-%02d", index)) {
			t.Fatalf("recent event %d is missing: %q", index, plain)
		}
	}
}

func TestEventsExpireIndependently(t *testing.T) {
	model := newGameModel(Repositories{}, nil, nil, Identity{}, nil, nil)
	model.addEvent(EventView{Kind: world.EventPickup, Message: "first"})
	model.addEvent(EventView{Kind: world.EventProgression, Message: "second"})
	_, _ = model.Update(eventExpiredMsg{id: 1})
	if len(model.eventFeed.events) != 1 || model.eventFeed.events[0].Message != "second" {
		t.Fatalf("events after first expiry: %#v", model.eventFeed.events)
	}
}

func TestEventsRemainVisibleWithMenuOpen(t *testing.T) {
	state := ViewState{
		Phase: phasePlaying, Width: 80, Height: 24, InventoryOpen: true,
		Character: &domain.Character{ID: 1, Name: "Aria", Level: 1},
		Events:    []EventView{{Kind: world.EventPickup, Message: "Picked up Slime Gel"}},
	}
	plain := ansi.Strip(NewRenderer().Render(state))
	if !strings.Contains(plain, "EVENTS") ||
		!strings.Contains(plain, "Picked up Slime Gel") ||
		!strings.Contains(plain, "INVENTORY") {
		t.Fatalf("event and menu overlays were not both visible: %q", plain)
	}
}
