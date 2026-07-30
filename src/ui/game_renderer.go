package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"sshrpg/src/domain"
	"sshrpg/src/npc"
	"sshrpg/src/world"
)

type cellStyle uint8
type playerNamePalette uint8
type terrainTheme uint8

const (
	cellStylePlain cellStyle = iota
	cellStyleWall
	cellStyleTerrainAccent
	cellStyleTerrainWoodwork
	cellStyleTerrainVegetation
	cellStyleTerrainWater
	cellStyleTerrainLava
	cellStyleTerrainIce
	cellStyleTerrainFire
	cellStyleTerrainWell
	cellStyleTerrainLandmark
	cellStyleWaypoint
	cellStyleGroundItem
	cellStyleEnemy
	cellStyleNPC
	cellStylePlayerBody
	cellStyleAttack
	cellStylePlayerName
)

const (
	playerNamePaletteUser playerNamePalette = iota
	playerNamePaletteModerator
	playerNamePaletteAdmin
)

const (
	terrainThemeStone terrainTheme = iota
	terrainThemeVerdant
	terrainThemeRedwood
	terrainThemeMarsh
	terrainThemeCoastal
	terrainThemeFrost
	terrainThemeEmber
	terrainThemeSunlit
	terrainThemeCrystal
	terrainThemeAstral
	terrainThemeVillage
	terrainThemeIron
)

type gameCell struct {
	glyph       rune
	style       cellStyle
	terrain     terrainTheme
	namePalette playerNamePalette
	nameStyle   uint8
}

type gameGrid struct {
	width  int
	height int
	cells  []gameCell
}

type terrainCache struct {
	area        *world.Area
	left        int
	top         int
	width       int
	height      int
	cells       []gameCell
	initialized bool
}

type GameRenderer struct {
	terrain terrainCache
}

func (renderer *GameRenderer) Render(state ViewState) string {
	if state.Width < 40 || state.Height < 10 {
		return lipgloss.Place(
			max(state.Width, 1), max(state.Height, 1),
			lipgloss.Center, lipgloss.Center,
			errorStyle.Render("Please resize your terminal to at least 40×10."),
		)
	}

	mapHeight := max(state.Height-3, 1)
	self := snapshotPlayer(state.Snapshot, state.Character)
	left, top := self.X-state.Width/2, self.Y-mapHeight/2
	grid := renderer.terrainGrid(
		state.Snapshot.Area, left, top, state.Width, mapHeight,
	)

	nearby := make([]string, 0, len(state.Snapshot.Players))
	visiblePlayers := make([]world.Player, 0, len(state.Snapshot.Players))
	for _, player := range state.Snapshot.Players {
		x, y := player.X-left, player.Y-top
		if !grid.inBounds(x, y) {
			continue
		}
		visiblePlayers = append(visiblePlayers, player)
		if player.ID != state.Character.ID {
			nearby = append(nearby, player.Name)
		}
	}
	sort.SliceStable(visiblePlayers, func(i, j int) bool {
		return visiblePlayers[i].ID != state.Character.ID &&
			visiblePlayers[j].ID == state.Character.ID
	})

	visibleDrops := append([]world.GroundItem(nil), state.Snapshot.Drops...)
	sort.Slice(visibleDrops, func(i, j int) bool {
		return visibleDrops[i].ID < visibleDrops[j].ID
	})
	for _, drop := range visibleDrops {
		x, y := drop.X-left, drop.Y-top
		if grid.inBounds(x, y) {
			drawCentered(grid, x, y, []rune("◆"), cellStyleGroundItem)
		}
	}

	visibleEnemies := append([]world.Enemy(nil), state.Snapshot.Enemies...)
	sort.Slice(visibleEnemies, func(i, j int) bool {
		return visibleEnemies[i].ID < visibleEnemies[j].ID
	})
	for _, enemy := range visibleEnemies {
		x, y := enemy.X-left, enemy.Y-top
		if !grid.inBounds(x, y) {
			continue
		}
		drawEnemy(
			grid, x, y,
			fmt.Sprintf(
				"%s [%d/%d]",
				enemy.Definition.Name, enemy.Health, enemy.Definition.Health,
			),
			enemy.Definition.Visual,
		)
	}

	if state.Snapshot.Area != nil {
		for _, definition := range state.Snapshot.Area.NPCs {
			x, y := definition.X-left, definition.Y-top
			if grid.inBounds(x, y) {
				drawNPC(grid, x, y, definition)
			}
		}
	}

	// Draw players last so the local character remains visible when entities overlap.
	for _, player := range visiblePlayers {
		marker := "○"
		walkFrame, facingX, facingY := 0, 0, 0
		if player.ID == state.Character.ID {
			marker = "@"
			walkFrame = state.WalkFrame
			facingX, facingY = state.FacingX, state.FacingY
		}
		drawPlayer(
			grid, player.X-left, player.Y-top,
			marker, player.Name, walkFrame, facingX, facingY,
			player.Role, state.PlayerNameShimmer,
		)
	}
	if state.AttackFrame > 0 {
		drawSlash(
			grid, self.X-left, self.Y-top,
			state.AttackDirection, state.AttackFrame,
		)
	}
	sort.Strings(nearby)

	progression := fmt.Sprintf(
		"Lv %d • XP %d/%d", self.Level, self.Experience,
		world.ExperienceToNextLevel(self.Level),
	)
	if self.SkillPoints > 0 {
		progression += fmt.Sprintf(" • SP %d", self.SkillPoints)
	}
	header := headerStyle.Render(fmt.Sprintf(
		" %s • %s • HP %d/%d • Gold %d • %s  (%d, %d)  Players here: %d • Enemies: %d",
		self.Name, progression, self.Health, self.MaxHealth,
		state.Character.Gold,
		areaName(state.Snapshot.Area), self.X, self.Y,
		len(state.Snapshot.Players), len(state.Snapshot.Enemies),
	))
	footer := " F1: help • Ctrl+C: quit"
	if len(nearby) > 0 {
		footer += " • Nearby: " + strings.Join(nearby, ", ")
	}
	return header + "\n" + grid.render() + "\n" + mutedStyle.Render(footer)
}

func (renderer *GameRenderer) terrainGrid(
	area *world.Area,
	left, top, width, height int,
) *gameGrid {
	cache := &renderer.terrain
	if cache.initialized &&
		cache.area == area &&
		cache.left == left &&
		cache.top == top &&
		cache.width == width &&
		cache.height == height {
		return &gameGrid{
			width: width, height: height,
			cells: append([]gameCell(nil), cache.cells...),
		}
	}

	grid := newGameGrid(width, height)
	if area != nil {
		theme := terrainThemeForArea(area)
		for screenY := 0; screenY < height; screenY++ {
			for screenX := 0; screenX < width; screenX++ {
				point := world.Point{X: left + screenX, Y: top + screenY}
				if !area.InBounds(point) {
					continue
				}
				grid.set(
					screenX, screenY,
					terrainCell(area.Tile(point), theme),
				)
				if _, ok := area.Waypoint(point); ok {
					grid.set(screenX, screenY, gameCell{
						glyph: '◇', style: cellStyleWaypoint,
					})
				}
			}
		}
	}
	cache.area = area
	cache.left = left
	cache.top = top
	cache.width = width
	cache.height = height
	cache.cells = append(cache.cells[:0], grid.cells...)
	cache.initialized = true
	return grid
}

func terrainCell(tile rune, theme terrainTheme) gameCell {
	switch tile {
	case '#':
		return gameCell{
			glyph: '█', style: cellStyleWall, terrain: theme,
		}
	case '.':
		// Ground stays blank so entities remain readable and large open areas
		// remain cheap for terminals to draw.
		return gameCell{}
	case '=':
		return gameCell{glyph: tile, style: cellStyleTerrainWoodwork}
	case 'T':
		return gameCell{glyph: tile, style: cellStyleTerrainVegetation}
	case '~':
		return gameCell{glyph: tile, style: cellStyleTerrainWater}
	case '≈':
		return gameCell{glyph: tile, style: cellStyleTerrainLava}
	case '*':
		return gameCell{glyph: tile, style: cellStyleTerrainIce}
	case 'f':
		return gameCell{glyph: '▲', style: cellStyleTerrainFire}
	case 'W':
		return gameCell{glyph: '◉', style: cellStyleTerrainWell}
	case '^':
		return gameCell{glyph: tile, style: cellStyleTerrainLandmark}
	default:
		return gameCell{
			glyph: tile, style: cellStyleTerrainAccent, terrain: theme,
		}
	}
}

func terrainThemeForArea(area *world.Area) terrainTheme {
	if area == nil {
		return terrainThemeStone
	}
	switch area.Palette {
	case "verdant":
		return terrainThemeVerdant
	case "redwood":
		return terrainThemeRedwood
	case "marsh":
		return terrainThemeMarsh
	case "coastal":
		return terrainThemeCoastal
	case "frost":
		return terrainThemeFrost
	case "ember":
		return terrainThemeEmber
	case "sunlit":
		return terrainThemeSunlit
	case "crystal":
		return terrainThemeCrystal
	case "astral":
		return terrainThemeAstral
	case "village":
		return terrainThemeVillage
	case "iron":
		return terrainThemeIron
	default:
		return terrainThemeStone
	}
}

func newGameGrid(width, height int) *gameGrid {
	return &gameGrid{
		width: width, height: height,
		cells: make([]gameCell, max(width, 0)*max(height, 0)),
	}
}

func (grid *gameGrid) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < grid.width && y < grid.height
}

func (grid *gameGrid) set(x, y int, cell gameCell) {
	if grid.inBounds(x, y) {
		grid.cells[y*grid.width+x] = cell
	}
}

func (grid *gameGrid) cell(x, y int) gameCell {
	if !grid.inBounds(x, y) {
		return gameCell{}
	}
	return grid.cells[y*grid.width+x]
}

func (grid *gameGrid) render() string {
	var rendered strings.Builder
	rendered.Grow(grid.width * grid.height)
	for y := 0; y < grid.height; y++ {
		if y > 0 {
			rendered.WriteByte('\n')
		}
		row := grid.cells[y*grid.width : (y+1)*grid.width]
		for start := 0; start < len(row); {
			appearance := row[start]
			end := start + 1
			for end < len(row) && sameCellStyle(appearance, row[end]) {
				end++
			}
			var run strings.Builder
			run.Grow(end - start)
			for _, cell := range row[start:end] {
				glyph := cell.glyph
				if glyph == 0 {
					glyph = ' '
				}
				run.WriteRune(glyph)
			}
			rendered.WriteString(renderCellRun(appearance, run.String()))
			start = end
		}
	}
	return rendered.String()
}

func (grid *gameGrid) renderedCell(x, y int) string {
	cell := grid.cell(x, y)
	glyph := cell.glyph
	if glyph == 0 {
		glyph = ' '
	}
	return renderCellRun(cell, string(glyph))
}

func sameCellStyle(left, right gameCell) bool {
	if left.style != right.style {
		return false
	}
	if left.style == cellStyleWall ||
		left.style == cellStyleTerrainAccent {
		return left.terrain == right.terrain
	}
	if left.style != cellStylePlayerName {
		return true
	}
	return left.namePalette == right.namePalette &&
		left.nameStyle == right.nameStyle
}

func renderCellRun(cell gameCell, run string) string {
	switch cell.style {
	case cellStyleWall:
		return terrainStylesFor(cell.terrain).wall.Render(run)
	case cellStyleTerrainAccent:
		return terrainStylesFor(cell.terrain).accent.Render(run)
	case cellStyleTerrainWoodwork:
		return terrainWoodworkStyle.Render(run)
	case cellStyleTerrainVegetation:
		return terrainVegetationStyle.Render(run)
	case cellStyleTerrainWater:
		return terrainWaterStyle.Render(run)
	case cellStyleTerrainLava:
		return terrainLavaStyle.Render(run)
	case cellStyleTerrainIce:
		return terrainIceStyle.Render(run)
	case cellStyleTerrainFire:
		return terrainFireStyle.Render(run)
	case cellStyleTerrainWell:
		return terrainWellStyle.Render(run)
	case cellStyleTerrainLandmark:
		return terrainLandmarkStyle.Render(run)
	case cellStyleWaypoint:
		return waypointStyle.Render(run)
	case cellStyleGroundItem:
		return groundItemStyle.Render(run)
	case cellStyleEnemy:
		return enemyStyle.Render(run)
	case cellStyleNPC:
		return npcStyle.Render(run)
	case cellStylePlayerBody:
		return playerBodyStyle.Render(run)
	case cellStyleAttack:
		return attackStyle.Render(run)
	case cellStylePlayerName:
		return playerNameCellStyle(cell).Render(run)
	default:
		return run
	}
}

type terrainStyles struct {
	wall   lipgloss.Style
	accent lipgloss.Style
}

func terrainStylesFor(theme terrainTheme) terrainStyles {
	if int(theme) >= len(terrainThemeStyles) {
		return terrainThemeStyles[terrainThemeStone]
	}
	return terrainThemeStyles[theme]
}

func drawNPC(grid *gameGrid, x, baseY int, definition npc.Definition) {
	label := definition.Name
	body := "/|\\"
	switch definition.Type {
	case npc.TypeShop:
		label += " [Shop]"
		body = "/$\\"
	case npc.TypeQuestGiver:
		label += " [Quest]"
		body = "/?\\"
	}
	drawCentered(grid, x, baseY-2, terminalCellRunes(label), cellStyleNPC)
	drawCentered(grid, x, baseY-1, []rune("O"), cellStyleNPC)
	drawCentered(grid, x, baseY, []rune(body), cellStyleNPC)
}

func areaName(area *world.Area) string {
	if area == nil {
		return "Unknown Area"
	}
	return area.Name
}

func drawPlayer(
	grid *gameGrid,
	x, baseY int,
	marker, name string,
	walkFrame, facingX, facingY int,
	role domain.CharacterRole,
	shimmerFrame int,
) {
	drawPlayerName(
		grid, x, baseY-3,
		terminalCellRunes(marker+" "+name),
		role, shimmerFrame,
	)
	drawCentered(grid, x, baseY-2, []rune("O"), cellStylePlayerBody)
	drawCentered(grid, x, baseY-1, []rune("/|\\"), cellStylePlayerBody)
	drawCentered(
		grid, x, baseY,
		[]rune(playerLegs(walkFrame, facingX, facingY)),
		cellStylePlayerBody,
	)
}

func drawPlayerName(
	grid *gameGrid,
	centerX, y int,
	content []rune,
	role domain.CharacterRole,
	shimmerFrame int,
) {
	if y < 0 || y >= grid.height {
		return
	}
	startX := centerX - len(content)/2
	for offset, cell := range content {
		x := startX + offset
		if !grid.inBounds(x, y) || cell == ' ' {
			continue
		}
		grid.set(
			x, y,
			playerNameCell(cell, role, shimmerFrame, offset),
		)
	}
}

func playerNameCell(
	glyph rune,
	role domain.CharacterRole,
	frame, offset int,
) gameCell {
	cell := gameCell{
		glyph: glyph, style: cellStylePlayerName,
		namePalette: playerNamePaletteUser,
	}
	styles := playerNameShimmerStyles(role)
	if len(styles) == 0 {
		return cell
	}
	if role == domain.CharacterRoleAdmin {
		cell.namePalette = playerNamePaletteAdmin
	} else {
		cell.namePalette = playerNamePaletteModerator
	}
	cell.nameStyle = uint8((frame + offset) % len(styles))
	return cell
}

func playerNameCellStyle(cell gameCell) lipgloss.Style {
	switch cell.namePalette {
	case playerNamePaletteAdmin:
		return adminPlayerNameStyles[int(cell.nameStyle)%len(adminPlayerNameStyles)]
	case playerNamePaletteModerator:
		index := int(cell.nameStyle) % len(moderatorPlayerNameStyles)
		return moderatorPlayerNameStyles[index]
	default:
		return userPlayerNameStyle
	}
}

func playerLegs(walkFrame, facingX, facingY int) string {
	if walkFrame == 0 || walkFrame == 2 {
		return "/ \\"
	}
	switch {
	case facingX < 0:
		return " /| "
	case facingX > 0:
		return "  |\\"
	case facingY < 0:
		return "/| "
	case facingY > 0:
		return " |\\"
	default:
		return "/ \\"
	}
}

func drawEnemy(
	grid *gameGrid,
	x, baseY int,
	name string,
	visual []string,
) {
	drawCentered(
		grid, x, baseY-len(visual),
		terminalCellRunes(name), cellStyleEnemy,
	)
	for rowIndex, row := range visual {
		y := baseY - len(visual) + 1 + rowIndex
		drawCentered(
			grid, x, y, terminalCellRunes(row), cellStyleEnemy,
		)
	}
}

func drawSlash(grid *gameGrid, x, baseY, direction, frame int) {
	if direction < 0 {
		direction = -1
	} else {
		direction = 1
	}
	if frame == 2 {
		drawCentered(
			grid, x+direction*2, baseY-1,
			[]rune("───"), cellStyleAttack,
		)
		return
	}
	glyph := "/"
	if direction < 0 {
		glyph = "\\"
	}
	drawCentered(
		grid, x+direction*2, baseY-2,
		[]rune(glyph), cellStyleAttack,
	)
	drawCentered(
		grid, x+direction*3, baseY-1,
		[]rune(glyph), cellStyleAttack,
	)
}

func drawCentered(
	grid *gameGrid,
	centerX, y int,
	content []rune,
	style cellStyle,
) {
	if y < 0 || y >= grid.height {
		return
	}
	startX := centerX - len(content)/2
	for offset, cell := range content {
		x := startX + offset
		if !grid.inBounds(x, y) || cell == ' ' {
			continue
		}
		grid.set(x, y, gameCell{glyph: cell, style: style})
	}
}

func terminalCellRunes(value string) []rune {
	cells := make([]rune, 0, len(value))
	for _, cell := range value {
		if lipgloss.Width(string(cell)) == 1 {
			cells = append(cells, cell)
		} else {
			cells = append(cells, '?')
		}
	}
	return cells
}

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E2E8F0"))
	playerBodyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#CBD5E1"))
	enemyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FB7185"))
	attackStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FDE68A"))
	groundItemStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4ADE80"))
	npcStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F59E0B"))
	terrainWoodworkStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#D6A45B"))
	terrainVegetationStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#4ADE80"))
	terrainWaterStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#38BDF8"))
	terrainLavaStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FB5A3C"))
	terrainIceStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A5F3FC"))
	terrainFireStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FF713D"))
	terrainWellStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#94A3B8"))
	terrainLandmarkStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FDE68A"))
	waypointStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A78BFA"))
	terrainThemeStyles = [...]terrainStyles{
		terrainThemeStone: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#475569")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")),
		},
		terrainThemeVerdant: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#287052")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#86C96F")),
		},
		terrainThemeRedwood: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#714634")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#D18B5B")),
		},
		terrainThemeMarsh: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#316B62")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#79C6A3")),
		},
		terrainThemeCoastal: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#326B85")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#67D4E8")),
		},
		terrainThemeFrost: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#4D7896")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#9DE4F2")),
		},
		terrainThemeEmber: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#833E35")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#FB8351")),
		},
		terrainThemeSunlit: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#80633A")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#F2C66D")),
		},
		terrainThemeCrystal: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#55549A")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE2E5")),
		},
		terrainThemeAstral: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#5D4B8A")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#C4A7FF")),
		},
		terrainThemeVillage: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#76583B")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#62C7B7")),
		},
		terrainThemeIron: {
			wall:   lipgloss.NewStyle().Foreground(lipgloss.Color("#586474")),
			accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#E59A45")),
		},
	}
)
