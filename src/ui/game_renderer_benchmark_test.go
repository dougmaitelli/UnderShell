package ui

import (
	"strings"
	"testing"

	"sshrpg/src/domain"
	"sshrpg/src/world"
)

func BenchmarkGameRenderer(b *testing.B) {
	for _, size := range []struct {
		name   string
		width  int
		height int
	}{
		{name: "80x24", width: 80, height: 24},
		{name: "240x80", width: 240, height: 80},
	} {
		b.Run(size.name, func(b *testing.B) {
			layout := make([]string, size.height*2)
			for row := range layout {
				layout[row] = strings.Repeat(".", size.width*2)
			}
			areas, err := world.NewAreas([]world.AreaDefinition{{
				ID: "field", Name: "Field", Layout: layout,
				Spawn: world.Point{X: size.width, Y: size.height},
			}})
			if err != nil {
				b.Fatal(err)
			}
			area, _ := areas.Area("field")
			character := &domain.Character{
				ID: 1, Name: "Aria", AreaID: "field",
				X: size.width, Y: size.height, Level: 1,
			}
			state := ViewState{
				Phase: phasePlaying, Width: size.width, Height: size.height,
				Character: character,
				Snapshot: world.Snapshot{
					Area: area,
					Players: []world.Player{{
						ID: 1, Name: "Aria", AreaID: "field",
						X: size.width, Y: size.height, Level: 1,
					}},
				},
			}
			renderer := &GameRenderer{}
			_ = renderer.Render(state)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				_ = renderer.Render(state)
			}
		})
	}
}
