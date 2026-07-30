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

const (
	cellStylePlain cellStyle = iota
	cellStyleWall
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

type gameCell struct {
	glyph       rune
	style       cellStyle
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
		for screenY := 0; screenY < height; screenY++ {
			for screenX := 0; screenX < width; screenX++ {
				point := world.Point{X: left + screenX, Y: top + screenY}
				if !area.InBounds(point) {
					continue
				}
				switch tile := area.Tile(point); tile {
				case '#':
					grid.set(screenX, screenY, gameCell{
						glyph: '█', style: cellStyleWall,
					})
				case '.':
				// The zero-value cell is already an unstyled space.
				default:
					grid.set(screenX, screenY, gameCell{glyph: tile})
				}
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
	if left.style != cellStylePlayerName {
		return true
	}
	return left.namePalette == right.namePalette &&
		left.nameStyle == right.nameStyle
}

func renderCellRun(cell gameCell, run string) string {
	switch cell.style {
	case cellStyleWall:
		return wallStyle.Render(run)
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
	wallStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#334155"))
	waypointStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A78BFA"))
)
