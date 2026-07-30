package ui

import (
	"testing"

	"sshrpg/src/world"
)

func TestTerrainThemesGiveRegionsDistinctWallColors(t *testing.T) {
	meadow := terrainCell(
		'#', terrainThemeForArea(&world.Area{Palette: "verdant"}),
	)
	ember := terrainCell(
		'#', terrainThemeForArea(&world.Area{Palette: "ember"}),
	)

	meadowWall := renderCellRun(meadow, string(meadow.glyph))
	emberWall := renderCellRun(ember, string(ember.glyph))
	if meadowWall == emberWall {
		t.Fatal("verdant and ember regions use the same wall color")
	}
}

func TestTerrainTilesUseSemanticColors(t *testing.T) {
	theme := terrainThemeVillage
	tests := []struct {
		name  string
		tile  rune
		style cellStyle
	}{
		{name: "wall", tile: '#', style: cellStyleWall},
		{name: "bridge or door", tile: '=', style: cellStyleTerrainWoodwork},
		{name: "tree", tile: 'T', style: cellStyleTerrainVegetation},
		{name: "water", tile: '~', style: cellStyleTerrainWater},
		{name: "lava", tile: '≈', style: cellStyleTerrainLava},
		{name: "ice or crystal", tile: '*', style: cellStyleTerrainIce},
		{name: "campfire", tile: 'f', style: cellStyleTerrainFire},
		{name: "well", tile: 'W', style: cellStyleTerrainWell},
		{name: "landmark", tile: '^', style: cellStyleTerrainLandmark},
		{name: "detail", tile: 'o', style: cellStyleTerrainAccent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cell := terrainCell(test.tile, theme)
			if cell.style != test.style {
				t.Fatalf("style = %d, want %d", cell.style, test.style)
			}
			if renderCellRun(cell, string(cell.glyph)) == string(cell.glyph) {
				t.Fatal("terrain tile did not emit ANSI styling")
			}
		})
	}
}

func TestSemanticTerrainColorsAreDistinct(t *testing.T) {
	rendered := make(map[string]rune)
	for _, tile := range []rune{'#', '=', 'T', '~', '≈', '*', 'f', 'W', '^'} {
		cell := terrainCell(tile, terrainThemeVillage)
		style := renderCellRun(cell, "X")
		if other, exists := rendered[style]; exists {
			t.Fatalf("tiles %q and %q use the same color", other, tile)
		}
		rendered[style] = tile
	}
}

func TestUnknownTerrainPaletteFallsBackToStone(t *testing.T) {
	theme := terrainThemeForArea(&world.Area{Palette: "unknown"})
	if theme != terrainThemeStone {
		t.Fatalf("theme = %d, want stone theme", theme)
	}
}

func TestTerrainGroundRemainsAnUnstyledSpace(t *testing.T) {
	cell := terrainCell('.', terrainThemeVerdant)
	if cell != (gameCell{}) {
		t.Fatalf("ground cell = %#v, want zero-value cell", cell)
	}
	if renderCellRun(cell, " ") != " " {
		t.Fatal("ground cell should remain unstyled")
	}
}
