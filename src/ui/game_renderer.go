package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"sshrpg/src/world"
)

type GameRenderer struct{}

func (GameRenderer) Render(state ViewState) string {
	if state.Width < 40 || state.Height < 10 {
		return lipgloss.Place(
			max(state.Width, 1), max(state.Height, 1),
			lipgloss.Center, lipgloss.Center,
			errorStyle.Render("Please resize your terminal to at least 40×10."),
		)
	}

	mapHeight := max(state.Height-3, 1)
	grid := make([][]string, mapHeight)
	for y := range grid {
		grid[y] = make([]string, state.Width)
		for x := range grid[y] {
			grid[y][x] = " "
		}
	}

	self := world.Player{
		ID: state.Character.ID, Name: state.Character.Name, AreaID: state.Character.AreaID,
		X: state.Character.X, Y: state.Character.Y,
	}
	for _, player := range state.Snapshot.Players {
		if player.ID == state.Character.ID {
			self = player
			break
		}
	}
	left, top := self.X-state.Width/2, self.Y-mapHeight/2
	if state.Snapshot.Area != nil {
		for screenY := 0; screenY < mapHeight; screenY++ {
			for screenX := 0; screenX < state.Width; screenX++ {
				point := world.Point{X: left + screenX, Y: top + screenY}
				if !state.Snapshot.Area.InBounds(point) {
					continue
				}
				grid[screenY][screenX] = renderTile(state.Snapshot.Area.Tile(point))
				if _, ok := state.Snapshot.Area.Waypoint(point); ok {
					grid[screenY][screenX] = waypointStyle.Render("◇")
				}
			}
		}
	}

	nearby := make([]string, 0, len(state.Snapshot.Players))
	visiblePlayers := make([]world.Player, 0, len(state.Snapshot.Players))
	for _, player := range state.Snapshot.Players {
		x, y := player.X-left, player.Y-top
		if x < 0 || y < 0 || x >= state.Width || y >= mapHeight {
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
	sort.Slice(visibleDrops, func(i, j int) bool { return visibleDrops[i].ID < visibleDrops[j].ID })
	for _, drop := range visibleDrops {
		x, y := drop.X-left, drop.Y-top
		if x < 0 || y < 0 || x >= state.Width || y >= mapHeight {
			continue
		}
		drawCentered(grid, x, y, []rune("◆"), groundItemStyle)
	}
	visibleEnemies := append([]world.Enemy(nil), state.Snapshot.Enemies...)
	sort.Slice(visibleEnemies, func(i, j int) bool { return visibleEnemies[i].ID < visibleEnemies[j].ID })
	for _, enemy := range visibleEnemies {
		x, y := enemy.X-left, enemy.Y-top
		if x < 0 || y < 0 || x >= state.Width || y >= mapHeight {
			continue
		}
		drawEnemy(
			grid, x, y,
			fmt.Sprintf("%s [%d/%d]", enemy.Name, enemy.Health, enemy.MaxHealth),
			enemy.Visual, enemyStyle,
		)
	}
	// Draw players last so the local character remains visible when entities overlap.
	for _, player := range visiblePlayers {
		style, marker := otherPlayerStyle, "○"
		walkFrame, facingX, facingY := 0, 0, 0
		if player.ID == state.Character.ID {
			style, marker = selfPlayerStyle, "@"
			walkFrame = state.WalkFrame
			facingX, facingY = state.FacingX, state.FacingY
		}
		drawPlayer(
			grid, player.X-left, player.Y-top,
			marker, player.Name, walkFrame, facingX, facingY, style,
		)
	}
	if state.AttackFrame > 0 {
		drawSlash(
			grid, self.X-left, self.Y-top,
			state.FacingX, state.FacingY, state.AttackFrame,
		)
	}
	sort.Strings(nearby)

	rows := make([]string, mapHeight)
	for y, row := range grid {
		rows[y] = strings.Join(row, "")
	}
	progression := fmt.Sprintf(
		"Lv %d • XP %d/%d", self.Level, self.Experience,
		world.ExperienceToNextLevel(self.Level),
	)
	if self.SkillPoints > 0 {
		progression += fmt.Sprintf(" • SP %d", self.SkillPoints)
	}
	header := headerStyle.Render(fmt.Sprintf(
		" %s • %s • HP %d/%d • %s  (%d, %d)  Players here: %d • Enemies: %d",
		self.Name, progression, self.Health, self.MaxHealth,
		areaName(state.Snapshot.Area), self.X, self.Y,
		len(state.Snapshot.Players), len(state.Snapshot.Enemies),
	))
	footer := " F1: help • Ctrl+C: quit"
	if len(nearby) > 0 {
		footer += " • Nearby: " + strings.Join(nearby, ", ")
	}
	return header + "\n" + strings.Join(rows, "\n") + "\n" + mutedStyle.Render(footer)
}

func areaName(area *world.Area) string {
	if area == nil {
		return "Unknown Area"
	}
	return area.Name
}

func renderTile(tile rune) string {
	switch tile {
	case '#':
		return wallStyle.Render("█")
	case '.':
		return " "
	default:
		return string(tile)
	}
}

func drawPlayer(
	grid [][]string,
	x, baseY int,
	marker, name string,
	walkFrame, facingX, facingY int,
	style lipgloss.Style,
) {
	drawCentered(grid, x, baseY-3, terminalCellRunes(marker+" "+name), style)
	drawCentered(grid, x, baseY-2, []rune("O"), style)
	drawCentered(grid, x, baseY-1, []rune("/|\\"), style)
	drawCentered(grid, x, baseY, []rune(playerLegs(walkFrame, facingX, facingY)), style)
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

func drawEnemy(grid [][]string, x, baseY int, name string, visual []string, style lipgloss.Style) {
	drawCentered(grid, x, baseY-len(visual), terminalCellRunes(name), style)
	for rowIndex, row := range visual {
		y := baseY - len(visual) + 1 + rowIndex
		drawCentered(grid, x, y, terminalCellRunes(row), style)
	}
}

func drawSlash(grid [][]string, x, baseY, dx, dy, frame int) {
	if dx == 0 && dy == 0 {
		dx = 1
	}
	slashX, slashY := x+dx*2, baseY-1+dy*2
	glyph := "/"
	if frame == 2 {
		if dx != 0 {
			glyph = "─"
		} else {
			glyph = "│"
		}
	} else if dx < 0 || dy > 0 {
		glyph = "\\"
	}
	drawCentered(grid, slashX, slashY, []rune(glyph), attackStyle)
}

func drawCentered(grid [][]string, centerX, y int, content []rune, style lipgloss.Style) {
	if y < 0 || y >= len(grid) {
		return
	}
	startX := centerX - len(content)/2
	for offset, cell := range content {
		x := startX + offset
		if x < 0 || x >= len(grid[y]) || cell == ' ' {
			continue
		}
		grid[y][x] = style.Render(string(cell))
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
	selfPlayerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FBBF24"))
	otherPlayerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#38BDF8"))
	enemyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FB7185"))
	attackStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FDE68A"))
	groundItemStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4ADE80"))
	wallStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#334155"))
	waypointStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A78BFA"))
)
