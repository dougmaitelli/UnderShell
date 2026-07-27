package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"sshrpg/src/enemy"
)

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Waypoint struct {
	X               int    `json:"x"`
	Y               int    `json:"y"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	DestinationArea string `json:"destination_area"`
	DestinationX    int    `json:"destination_x"`
	DestinationY    int    `json:"destination_y"`
}

type AreaDefinition struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Layout      []string     `json:"layout"`
	Width       int          `json:"width"`
	Height      int          `json:"height"`
	Default     string       `json:"default_tile"`
	Border      string       `json:"border_tile"`
	Features    []TileRect   `json:"features"`
	Spawn       Point        `json:"spawn"`
	Waypoints   []Waypoint   `json:"waypoints"`
	EnemySpawns []EnemySpawn `json:"enemy_spawns"`
}

type EnemySpawn struct {
	EnemyID        string `json:"enemy_id"`
	X              int    `json:"x"`
	Y              int    `json:"y"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	MaxEnemies     int    `json:"max_enemies"`
	RespawnSeconds int    `json:"respawn_seconds"`
}

type TileRect struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Tile   string `json:"tile"`
}

type Area struct {
	ID          string
	Name        string
	Layout      []string
	Width       int
	Height      int
	Spawn       Point
	EnemySpawns []EnemySpawn
	waypoints   map[Point]Waypoint
}

type Areas struct {
	areas        map[string]*Area
	defaultID    string
	defaultPoint Point
}

func LoadAreas(directory string) (*Areas, error) {
	paths, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("find area files: %w", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no JSON area files found in %s", directory)
	}

	definitions := make([]AreaDefinition, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read area %s: %w", path, err)
		}
		var definition AreaDefinition
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&definition); err != nil {
			return nil, fmt.Errorf("decode area %s: %w", path, err)
		}
		definitions = append(definitions, definition)
	}
	return NewAreas(definitions)
}

func NewAreas(definitions []AreaDefinition) (*Areas, error) {
	if len(definitions) == 0 {
		return nil, errors.New("at least one area is required")
	}

	set := &Areas{areas: make(map[string]*Area, len(definitions))}
	for _, definition := range definitions {
		area, err := buildArea(definition)
		if err != nil {
			return nil, fmt.Errorf("area %q: %w", definition.ID, err)
		}
		if _, exists := set.areas[area.ID]; exists {
			return nil, fmt.Errorf("duplicate area ID %q", area.ID)
		}
		set.areas[area.ID] = area
		if set.defaultID == "" {
			set.defaultID = area.ID
			set.defaultPoint = area.Spawn
		}
	}

	for _, area := range set.areas {
		for point, waypoint := range area.waypoints {
			destination, ok := set.areas[waypoint.DestinationArea]
			if !ok {
				return nil, fmt.Errorf(
					"area %q waypoint at (%d,%d) targets unknown area %q",
					area.ID, point.X, point.Y, waypoint.DestinationArea,
				)
			}
			target := Point{X: waypoint.DestinationX, Y: waypoint.DestinationY}
			if !destination.Walkable(target) {
				return nil, fmt.Errorf(
					"area %q waypoint at (%d,%d) has blocked destination (%d,%d) in %q",
					area.ID, point.X, point.Y, target.X, target.Y, destination.ID,
				)
			}
		}
	}
	return set, nil
}

func (s *Areas) Area(id string) (*Area, bool) {
	area, ok := s.areas[id]
	return area, ok
}

func (s *Areas) SetDefaultSpawn(areaID string, point Point) error {
	area, ok := s.areas[areaID]
	if !ok {
		return fmt.Errorf("default spawn references unknown area %q", areaID)
	}
	if !area.Walkable(point) {
		return fmt.Errorf(
			"default spawn (%d,%d) in area %q must be on a walkable tile",
			point.X, point.Y, areaID,
		)
	}
	s.defaultID, s.defaultPoint = areaID, point
	return nil
}

func (s *Areas) DefaultSpawn() (*Area, Point) {
	return s.areas[s.defaultID], s.defaultPoint
}

func (s *Areas) ValidateEnemySpawns(enemies *enemy.Enemies) error {
	if enemies == nil {
		return errors.New("enemies are required")
	}
	for _, area := range s.areas {
		for index, spawn := range area.EnemySpawns {
			if _, ok := enemies.Enemy(spawn.EnemyID); !ok {
				return fmt.Errorf(
					"area %q enemy spawn %d references unknown enemy %q",
					area.ID, index, spawn.EnemyID,
				)
			}
		}
	}
	return nil
}

func (a *Area) Tile(point Point) rune {
	if !a.InBounds(point) {
		return '#'
	}
	return []rune(a.Layout[point.Y])[point.X]
}

func (a *Area) InBounds(point Point) bool {
	return point.Y >= 0 && point.Y < a.Height && point.X >= 0 && point.X < a.Width
}

func (a *Area) Walkable(point Point) bool {
	return a.InBounds(point) && a.Tile(point) != '#'
}

func (a *Area) Waypoint(point Point) (Waypoint, bool) {
	waypoint, ok := a.waypoints[point]
	return waypoint, ok
}

func buildArea(definition AreaDefinition) (*Area, error) {
	definition.ID = strings.TrimSpace(definition.ID)
	definition.Name = strings.TrimSpace(definition.Name)
	if definition.ID == "" || definition.Name == "" {
		return nil, errors.New("id and name are required")
	}
	if len(definition.Layout) == 0 {
		layout, err := expandLayout(definition)
		if err != nil {
			return nil, err
		}
		definition.Layout = layout
	}

	width := len([]rune(definition.Layout[0]))
	if width == 0 {
		return nil, errors.New("layout rows cannot be empty")
	}
	for rowIndex, row := range definition.Layout {
		if len([]rune(row)) != width {
			return nil, fmt.Errorf("layout row %d has a different width", rowIndex)
		}
		for _, tile := range row {
			if !unicode.IsPrint(tile) {
				return nil, fmt.Errorf("layout row %d contains a non-printable tile", rowIndex)
			}
		}
	}

	area := &Area{
		ID: definition.ID, Name: definition.Name,
		Layout: append([]string(nil), definition.Layout...),
		Width:  width, Height: len(definition.Layout), Spawn: definition.Spawn,
		EnemySpawns: append([]EnemySpawn(nil), definition.EnemySpawns...),
		waypoints:   make(map[Point]Waypoint, len(definition.Waypoints)),
	}
	if !area.Walkable(area.Spawn) {
		return nil, errors.New("spawn point must be on a walkable tile")
	}
	for _, waypoint := range definition.Waypoints {
		if waypoint.Width == 0 {
			waypoint.Width = 1
		}
		if waypoint.Height == 0 {
			waypoint.Height = 1
		}
		if waypoint.Width < 1 || waypoint.Height < 1 {
			return nil, fmt.Errorf("waypoint at (%d,%d) must have positive dimensions", waypoint.X, waypoint.Y)
		}
		for y := waypoint.Y; y < waypoint.Y+waypoint.Height; y++ {
			for x := waypoint.X; x < waypoint.X+waypoint.Width; x++ {
				point := Point{X: x, Y: y}
				if !area.Walkable(point) {
					return nil, fmt.Errorf("waypoint tile at (%d,%d) must be walkable", point.X, point.Y)
				}
				if _, exists := area.waypoints[point]; exists {
					return nil, fmt.Errorf("overlapping waypoint at (%d,%d)", point.X, point.Y)
				}
				area.waypoints[point] = waypoint
			}
		}
	}
	for index, spawn := range area.EnemySpawns {
		spawn.EnemyID = strings.TrimSpace(spawn.EnemyID)
		if spawn.EnemyID == "" {
			return nil, fmt.Errorf("enemy spawn %d requires enemy_id", index)
		}
		if spawn.Width < 1 || spawn.Height < 1 {
			return nil, fmt.Errorf("enemy spawn %d must have positive dimensions", index)
		}
		if spawn.MaxEnemies < 1 {
			return nil, fmt.Errorf("enemy spawn %d max_enemies must be at least 1", index)
		}
		if spawn.RespawnSeconds < 1 {
			return nil, fmt.Errorf("enemy spawn %d respawn_seconds must be at least 1", index)
		}
		hasWalkableTile := false
		for y := spawn.Y; y < spawn.Y+spawn.Height; y++ {
			for x := spawn.X; x < spawn.X+spawn.Width; x++ {
				if area.Walkable(Point{X: x, Y: y}) {
					hasWalkableTile = true
				}
			}
		}
		if !hasWalkableTile {
			return nil, fmt.Errorf("enemy spawn %d must contain a walkable tile", index)
		}
		area.EnemySpawns[index] = spawn
	}
	return area, nil
}

func expandLayout(definition AreaDefinition) ([]string, error) {
	if definition.Width < 3 || definition.Height < 3 {
		return nil, errors.New("generated layouts require width and height of at least 3")
	}
	defaultTile, err := singleTile(definition.Default, '.')
	if err != nil {
		return nil, fmt.Errorf("default tile: %w", err)
	}
	borderTile, err := singleTile(definition.Border, '#')
	if err != nil {
		return nil, fmt.Errorf("border tile: %w", err)
	}

	grid := make([][]rune, definition.Height)
	for y := range grid {
		grid[y] = make([]rune, definition.Width)
		for x := range grid[y] {
			grid[y][x] = defaultTile
			if x == 0 || y == 0 || x == definition.Width-1 || y == definition.Height-1 {
				grid[y][x] = borderTile
			}
		}
	}
	for index, feature := range definition.Features {
		tile, err := singleTile(feature.Tile, '#')
		if err != nil {
			return nil, fmt.Errorf("feature %d tile: %w", index, err)
		}
		if feature.Width < 1 || feature.Height < 1 ||
			feature.X < 0 || feature.Y < 0 ||
			feature.X+feature.Width > definition.Width ||
			feature.Y+feature.Height > definition.Height {
			return nil, fmt.Errorf("feature %d is outside the generated layout", index)
		}
		for y := feature.Y; y < feature.Y+feature.Height; y++ {
			for x := feature.X; x < feature.X+feature.Width; x++ {
				grid[y][x] = tile
			}
		}
	}

	layout := make([]string, len(grid))
	for y, row := range grid {
		layout[y] = string(row)
	}
	return layout, nil
}

func singleTile(value string, fallback rune) (rune, error) {
	if value == "" {
		return fallback, nil
	}
	runes := []rune(value)
	if len(runes) != 1 || !unicode.IsPrint(runes[0]) {
		return 0, errors.New("must be exactly one printable character")
	}
	return runes[0], nil
}
