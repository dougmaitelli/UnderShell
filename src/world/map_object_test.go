package world

import "testing"

func TestMapObjectPlacementStampsVisibleTilesOnly(t *testing.T) {
	objects, err := NewMapObjects([]MapObjectDefinition{{
		ID: "campfire",
		Layout: []string{
			" = ",
			"=f=",
			" = ",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	areas, err := NewAreas([]AreaDefinition{{
		ID: "camp", Name: "Camp", Width: 7, Height: 7,
		Spawn: Point{X: 1, Y: 1},
		Objects: []MapObjectPlacement{{
			ObjectID: "campfire", X: 2, Y: 2,
		}},
	}}, References{Objects: objects})
	if err != nil {
		t.Fatal(err)
	}
	area, _ := areas.Area("camp")

	if tile := area.Tile(Point{X: 2, Y: 2}); tile != '.' {
		t.Fatalf("transparent object cell replaced ground with %q", tile)
	}
	if tile := area.Tile(Point{X: 3, Y: 3}); tile != 'f' {
		t.Fatalf("campfire center = %q, want flame", tile)
	}
	if tile := area.Tile(Point{X: 2, Y: 3}); tile != '=' {
		t.Fatalf("campfire edge = %q, want log", tile)
	}
	if area.Walkable(Point{X: 3, Y: 3}) {
		t.Fatal("campfire center should block movement")
	}
	if !area.Walkable(Point{X: 2, Y: 3}) {
		t.Fatal("campfire logs should remain walkable")
	}
}

func TestMapObjectPlacementRequiresKnownInBoundsObject(t *testing.T) {
	objects, err := NewMapObjects([]MapObjectDefinition{{
		ID: "well", Layout: []string{"W"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		placement MapObjectPlacement
		refs      References
	}{
		{
			name: "missing definitions",
			placement: MapObjectPlacement{
				ObjectID: "well", X: 2, Y: 2,
			},
		},
		{
			name: "unknown object",
			placement: MapObjectPlacement{
				ObjectID: "missing", X: 2, Y: 2,
			},
			refs: References{Objects: objects},
		},
		{
			name: "outside layout",
			placement: MapObjectPlacement{
				ObjectID: "well", X: 5, Y: 2,
			},
			refs: References{Objects: objects},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewAreas([]AreaDefinition{{
				ID: "camp", Name: "Camp", Width: 5, Height: 5,
				Spawn:   Point{X: 1, Y: 1},
				Objects: []MapObjectPlacement{test.placement},
			}}, test.refs)
			if err == nil {
				t.Fatal("expected invalid map object placement to fail")
			}
		})
	}
}

func TestMapObjectDefinitionsRequireRectangularVisibleLayout(t *testing.T) {
	tests := []MapObjectDefinition{
		{ID: "empty"},
		{ID: "ragged", Layout: []string{"==", "="}},
		{ID: "transparent", Layout: []string{"   "}},
	}
	for _, definition := range tests {
		t.Run(definition.ID, func(t *testing.T) {
			if _, err := NewMapObjects([]MapObjectDefinition{definition}); err == nil {
				t.Fatal("expected invalid map object definition to fail")
			}
		})
	}
}
