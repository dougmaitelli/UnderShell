package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"sshrpg/src/domain"
	"sshrpg/src/enemy"
	"sshrpg/src/world"
)

func TestOffscreenEnemyMovementReusesCachedView(t *testing.T) {
	definition := &enemy.Definition{
		Name: "Bat", Health: 5, Visual: []string{"(v)"},
	}
	model := newGameModel(
		Repositories{}, nil, nil, Identity{},
		&domain.Character{
			ID: 1, Name: "Aria", AreaID: "cavern", X: 100, Y: 100,
		},
		nil,
	)
	model.phase = phasePlaying
	model.width, model.height = 40, 14
	model.connection.snapshot = world.Snapshot{
		Players: []world.Player{{
			ID: 1, Name: "Aria", AreaID: "cavern", X: 100, Y: 100,
		}},
		Enemies: []world.Enemy{{
			ID: 1, Definition: definition, Health: 5,
			AreaID: "cavern", X: 5, Y: 5,
		}},
	}
	initial := model.View().Content

	next := model.connection.snapshot
	next.Enemies = append([]world.Enemy(nil), next.Enemies...)
	next.Enemies[0].X++
	_, _ = model.Update(worldSnapshotMsg{snapshot: next, ok: true})

	if model.renderDirty {
		t.Fatal("offscreen enemy movement invalidated the cached view")
	}
	if rendered := model.View().Content; rendered != initial {
		t.Fatal("offscreen enemy movement changed the rendered frame")
	}
}

func TestVisibleEnemyMovementInvalidatesView(t *testing.T) {
	definition := &enemy.Definition{
		Name: "Bat", Health: 5, Visual: []string{"(v)"},
	}
	character := &domain.Character{
		ID: 1, Name: "Aria", AreaID: "cavern", X: 100, Y: 100,
	}
	previous := world.Snapshot{
		Players: []world.Player{{
			ID: 1, Name: "Aria", AreaID: "cavern", X: 100, Y: 100,
		}},
		Enemies: []world.Enemy{{
			ID: 1, Definition: definition, Health: 5,
			AreaID: "cavern", X: 101, Y: 100,
		}},
	}
	next := previous
	next.Enemies = append([]world.Enemy(nil), next.Enemies...)
	next.Enemies[0].X++

	if !snapshotAffectsView(previous, next, character, 40, 14) {
		t.Fatal("visible enemy movement did not invalidate the view")
	}
}

func TestPlayerHealthAndAreaCountsInvalidateView(t *testing.T) {
	character := &domain.Character{
		ID: 1, Name: "Aria", AreaID: "cavern", X: 100, Y: 100,
	}
	previous := world.Snapshot{
		Players: []world.Player{{
			ID: 1, Name: "Aria", AreaID: "cavern", X: 100, Y: 100,
			Health: 10, MaxHealth: 10,
		}},
	}
	healthChanged := previous
	healthChanged.Players = append([]world.Player(nil), previous.Players...)
	healthChanged.Players[0].Health = 9
	if !snapshotAffectsView(previous, healthChanged, character, 40, 14) {
		t.Fatal("health change did not invalidate the header")
	}

	playerCountChanged := previous
	playerCountChanged.Players = append(
		append([]world.Player(nil), previous.Players...),
		world.Player{
			ID: 2, Name: "Rowan", AreaID: "cavern", X: 1, Y: 1,
		},
	)
	if !snapshotAffectsView(previous, playerCountChanged, character, 40, 14) {
		t.Fatal("area player count change did not invalidate the header")
	}
}

func TestUnknownGameplayKeyReusesCachedView(t *testing.T) {
	model := newGameModel(
		Repositories{}, nil, nil, Identity{},
		&domain.Character{ID: 1, Name: "Aria"},
		nil,
	)
	model.phase = phasePlaying
	initial := model.View().Content

	_, command := model.Update(tea.KeyPressMsg(tea.Key{
		Text: "q", Code: 'q',
	}))
	if command != nil || model.renderDirty {
		t.Fatal("unused gameplay key invalidated the view")
	}
	if rendered := model.View().Content; rendered != initial {
		t.Fatal("unused gameplay key changed the rendered frame")
	}
}

func TestPlayerNameShimmerTickInvalidatesRenderedView(t *testing.T) {
	character := &domain.Character{
		ID: 1, Name: "Aria", Role: domain.CharacterRoleAdmin,
		AreaID: "cavern", X: 100, Y: 100,
	}
	model := newGameModel(
		Repositories{}, nil, nil, Identity{}, character, nil,
	)
	model.phase = phasePlaying
	model.width, model.height = 40, 14
	model.connection.snapshot = world.Snapshot{
		Players: []world.Player{{
			ID: 1, Name: "Aria", Role: domain.CharacterRoleAdmin,
			AreaID: "cavern", X: 100, Y: 100,
		}},
	}
	if command := model.nameShimmer.setNeeded(
		model.connection.snapshot.Players, nil,
	); command == nil {
		t.Fatal("visible admin did not schedule a shimmer tick")
	}
	initial := model.View().Content

	_, command := model.Update(playerNameShimmerMsg{
		generation: model.nameShimmer.generation,
	})
	if command == nil {
		t.Fatal("shimmer tick did not schedule the next frame")
	}
	if !model.renderDirty {
		t.Fatal("shimmer tick did not invalidate the rendered view")
	}
	if rendered := model.View().Content; rendered == initial {
		t.Fatal("shimmer tick did not change the visible player name")
	}
}
