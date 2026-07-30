package world

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"sshrpg/src/content"
)

type MapObjectDefinition struct {
	ID     string   `json:"id"`
	Layout []string `json:"layout"`
}

type MapObjectPlacement struct {
	ObjectID string `json:"object_id"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
}

type MapObject struct {
	ID     string
	Layout []string
	Width  int
	Height int
}

type MapObjects struct {
	objects map[string]*MapObject
}

func LoadMapObjects(directory string) (*MapObjects, error) {
	definitions, err := content.LoadDefinitions[MapObjectDefinition](
		directory, "map object",
	)
	if err != nil {
		return nil, err
	}
	return NewMapObjects(definitions)
}

func NewMapObjects(definitions []MapObjectDefinition) (*MapObjects, error) {
	if len(definitions) == 0 {
		return nil, errors.New("at least one map object is required")
	}
	set := &MapObjects{objects: make(map[string]*MapObject, len(definitions))}
	for _, definition := range definitions {
		object, err := buildMapObject(definition)
		if err != nil {
			return nil, fmt.Errorf("map object %q: %w", definition.ID, err)
		}
		if _, exists := set.objects[object.ID]; exists {
			return nil, fmt.Errorf("duplicate map object ID %q", object.ID)
		}
		set.objects[object.ID] = object
	}
	return set, nil
}

func (set *MapObjects) Object(id string) (*MapObject, bool) {
	if set == nil {
		return nil, false
	}
	object, ok := set.objects[strings.TrimSpace(id)]
	return object, ok
}

func buildMapObject(definition MapObjectDefinition) (*MapObject, error) {
	definition.ID = strings.TrimSpace(definition.ID)
	if definition.ID == "" {
		return nil, errors.New("id is required")
	}
	if len(definition.Layout) == 0 {
		return nil, errors.New("layout is required")
	}

	width := len([]rune(definition.Layout[0]))
	if width == 0 {
		return nil, errors.New("layout rows cannot be empty")
	}
	hasTile := false
	for rowIndex, row := range definition.Layout {
		if len([]rune(row)) != width {
			return nil, fmt.Errorf(
				"layout row %d has a different width", rowIndex,
			)
		}
		for _, tile := range row {
			if !unicode.IsPrint(tile) {
				return nil, fmt.Errorf(
					"layout row %d contains a non-printable tile", rowIndex,
				)
			}
			if tile != ' ' {
				hasTile = true
			}
		}
	}
	if !hasTile {
		return nil, errors.New("layout must contain at least one tile")
	}

	return &MapObject{
		ID:     definition.ID,
		Layout: append([]string(nil), definition.Layout...),
		Width:  width, Height: len(definition.Layout),
	}, nil
}

func stampMapObjects(
	layout []string,
	placements []MapObjectPlacement,
	objects *MapObjects,
) ([]string, error) {
	if len(placements) == 0 {
		return layout, nil
	}
	if objects == nil {
		return nil, errors.New("map object placements require object definitions")
	}

	grid := make([][]rune, len(layout))
	for y, row := range layout {
		grid[y] = []rune(row)
	}
	width := len(grid[0])
	for index, placement := range placements {
		object, ok := objects.Object(placement.ObjectID)
		if !ok {
			return nil, fmt.Errorf(
				"map object placement %d references unknown object %q",
				index, placement.ObjectID,
			)
		}
		if placement.X < 0 || placement.Y < 0 ||
			placement.X+object.Width > width ||
			placement.Y+object.Height > len(grid) {
			return nil, fmt.Errorf(
				"map object placement %d (%q) is outside the layout",
				index, object.ID,
			)
		}
		for objectY, row := range object.Layout {
			for objectX, tile := range []rune(row) {
				if tile == ' ' {
					continue
				}
				grid[placement.Y+objectY][placement.X+objectX] = tile
			}
		}
	}

	stamped := make([]string, len(grid))
	for y, row := range grid {
		stamped[y] = string(row)
	}
	return stamped, nil
}
