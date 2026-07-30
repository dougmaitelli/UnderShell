package world

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"sshrpg/src/content"
	"sshrpg/src/enemy"
	"sshrpg/src/item"
	"sshrpg/src/npc"
	"sshrpg/src/quest"
)

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type WaypointDefinition struct {
	X               int    `json:"x"`
	Y               int    `json:"y"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	DestinationArea string `json:"destination_area"`
	DestinationX    int    `json:"destination_x"`
	DestinationY    int    `json:"destination_y"`
}

type Waypoint struct {
	X            int
	Y            int
	Width        int
	Height       int
	Destination  *Area
	DestinationX int
	DestinationY int
}

type AreaDefinition struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Palette     string                 `json:"palette"`
	Layout      []string               `json:"layout"`
	Width       int                    `json:"width"`
	Height      int                    `json:"height"`
	Border      string                 `json:"border_tile"`
	Features    []TileRect             `json:"features"`
	Objects     []MapObjectPlacement   `json:"objects"`
	Spawn       Point                  `json:"spawn"`
	Waypoints   []WaypointDefinition   `json:"waypoints"`
	EnemySpawns []EnemySpawnDefinition `json:"enemy_spawns"`
	NPCs        []npc.Config           `json:"npcs"`
}

type EnemySpawnDefinition struct {
	EnemyID        string `json:"enemy_id"`
	X              int    `json:"x"`
	Y              int    `json:"y"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	MaxEnemies     int    `json:"max_enemies"`
	RespawnSeconds int    `json:"respawn_seconds"`
}

type EnemySpawn struct {
	Enemy          *enemy.Definition
	X              int
	Y              int
	Width          int
	Height         int
	MaxEnemies     int
	RespawnSeconds int
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
	Palette     string
	Layout      []string
	Width       int
	Height      int
	Spawn       Point
	EnemySpawns []EnemySpawn
	NPCs        []npc.Definition
	waypoints   map[Point]*Waypoint
}

type npcLocation struct {
	definition *npc.Definition
	area       *Area
}

type Areas struct {
	areas        map[string]*Area
	npcs         map[string]npcLocation
	defaultArea  *Area
	defaultPoint Point
}

type References struct {
	Items   *item.Items
	Enemies *enemy.Enemies
	Quests  *quest.Quests
	Objects *MapObjects
}

func LoadAreas(
	directory string,
	references ...References,
) (*Areas, error) {
	definitions, err := content.LoadDefinitions[AreaDefinition](directory, "area")
	if err != nil {
		return nil, err
	}
	return NewAreas(definitions, references...)
}

func NewAreas(
	definitions []AreaDefinition,
	references ...References,
) (*Areas, error) {
	if len(definitions) == 0 {
		return nil, errors.New("at least one area is required")
	}
	var refs References
	if len(references) > 0 {
		refs = references[0]
	}

	set := &Areas{
		areas: make(map[string]*Area, len(definitions)),
		npcs:  make(map[string]npcLocation),
	}
	for _, definition := range definitions {
		area, err := buildArea(
			definition, refs.Items, refs.Enemies, refs.Quests, refs.Objects,
		)
		if err != nil {
			return nil, fmt.Errorf("area %q: %w", definition.ID, err)
		}
		if _, exists := set.areas[area.ID]; exists {
			return nil, fmt.Errorf("duplicate area ID %q", area.ID)
		}
		set.areas[area.ID] = area
		if set.defaultArea == nil {
			set.defaultArea = area
			set.defaultPoint = area.Spawn
		}
	}

	for _, area := range set.areas {
		for index := range area.NPCs {
			definition := &area.NPCs[index]
			if existing, exists := set.npcs[definition.ID]; exists {
				return nil, fmt.Errorf(
					"duplicate NPC ID %q in areas %q and %q",
					definition.ID, existing.area.ID, area.ID,
				)
			}
			set.npcs[definition.ID] = npcLocation{
				definition: definition,
				area:       area,
			}
		}
	}

	for _, definition := range definitions {
		area := set.areas[strings.TrimSpace(definition.ID)]
		for _, source := range definition.Waypoints {
			destinationID := strings.TrimSpace(source.DestinationArea)
			destination, ok := set.areas[destinationID]
			if !ok {
				return nil, fmt.Errorf(
					"area %q waypoint at (%d,%d) targets unknown area %q",
					area.ID, source.X, source.Y, destinationID,
				)
			}
			waypoint := area.waypoints[Point{X: source.X, Y: source.Y}]
			waypoint.Destination = destination
			target := Point{X: source.DestinationX, Y: source.DestinationY}
			if !destination.Walkable(target) {
				return nil, fmt.Errorf(
					"area %q waypoint at (%d,%d) has blocked destination (%d,%d) in %q",
					area.ID, source.X, source.Y,
					target.X, target.Y, destination.ID,
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

func (s *Areas) FindArea(value string) (*Area, bool) {
	value = strings.TrimSpace(value)
	if area, ok := s.areas[value]; ok {
		return area, true
	}
	for _, area := range s.areas {
		if strings.EqualFold(area.Name, value) {
			return area, true
		}
	}
	return nil, false
}

func (s *Areas) NPC(id string) (*npc.Definition, *Area, bool) {
	location, ok := s.npcs[id]
	if !ok {
		return nil, nil, false
	}
	return location.definition, location.area, true
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
	if npc, occupied := area.NPCAt(point); occupied {
		return fmt.Errorf("default spawn cannot overlap NPC %q", npc.ID)
	}
	s.defaultArea, s.defaultPoint = area, point
	return nil
}

func (s *Areas) DefaultSpawn() (*Area, Point) {
	return s.defaultArea, s.defaultPoint
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
	return a.InBounds(point) && !tileBlocksMovement(a.Tile(point))
}

func tileBlocksMovement(tile rune) bool {
	switch tile {
	case '#', 'T', '~', '≈', '*', 'f', 'W':
		return true
	default:
		return false
	}
}

func (a *Area) Waypoint(point Point) (*Waypoint, bool) {
	waypoint, ok := a.waypoints[point]
	return waypoint, ok
}

func buildArea(
	definition AreaDefinition,
	items *item.Items,
	enemies *enemy.Enemies,
	quests *quest.Quests,
	objects *MapObjects,
) (*Area, error) {
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
	stampedLayout, err := stampMapObjects(
		definition.Layout, definition.Objects, objects,
	)
	if err != nil {
		return nil, err
	}
	definition.Layout = stampedLayout

	npcs, err := npc.Resolve(definition.NPCs, items, quests)
	if err != nil {
		return nil, err
	}
	area := &Area{
		ID: definition.ID, Name: definition.Name, Palette: definition.Palette,
		Layout: append([]string(nil), definition.Layout...),
		Width:  width, Height: len(definition.Layout), Spawn: definition.Spawn,
		EnemySpawns: make([]EnemySpawn, len(definition.EnemySpawns)),
		NPCs:        npcs,
		waypoints:   make(map[Point]*Waypoint, len(definition.Waypoints)),
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
		resolvedWaypoint := Waypoint{
			X: waypoint.X, Y: waypoint.Y,
			Width: waypoint.Width, Height: waypoint.Height,
			DestinationX: waypoint.DestinationX,
			DestinationY: waypoint.DestinationY,
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
				area.waypoints[point] = &resolvedWaypoint
			}
		}
	}
	for index, source := range definition.EnemySpawns {
		enemyID := strings.TrimSpace(source.EnemyID)
		if enemyID == "" {
			return nil, fmt.Errorf("enemy spawn %d requires enemy_id", index)
		}
		if enemies == nil {
			return nil, fmt.Errorf("enemy spawn %d requires enemy definitions", index)
		}
		enemyDefinition, ok := enemies.Enemy(enemyID)
		if !ok {
			return nil, fmt.Errorf(
				"enemy spawn %d references unknown enemy %q",
				index, enemyID,
			)
		}
		spawn := EnemySpawn{
			Enemy: enemyDefinition,
			X:     source.X, Y: source.Y,
			Width: source.Width, Height: source.Height,
			MaxEnemies:     source.MaxEnemies,
			RespawnSeconds: source.RespawnSeconds,
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
	if err := validateNPCPlacements(area); err != nil {
		return nil, err
	}
	return area, nil
}

func expandLayout(definition AreaDefinition) ([]string, error) {
	if definition.Width < 3 || definition.Height < 3 {
		return nil, errors.New("generated layouts require width and height of at least 3")
	}
	borderTile, err := singleTile(definition.Border, '#')
	if err != nil {
		return nil, fmt.Errorf("border tile: %w", err)
	}

	grid := make([][]rune, definition.Height)
	for y := range grid {
		grid[y] = make([]rune, definition.Width)
		for x := range grid[y] {
			grid[y][x] = '.'
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
