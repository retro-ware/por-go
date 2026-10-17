package battle

import (
	"testing"

	"por/engine"
)

func TestMapMesh(t *testing.T) {
	var g [25][50]engine.Square
	for z := range g {
		for x := range g[z] {
			g[z][x].Tile = 22
		}
	}
	g[0][0].Tile = 34 // a lone rock: not the floor
	g[5][5], g[5][6] = engine.Square{Tile: 1, Wall: true}, engine.Square{Tile: 2, Wall: true}
	g[9][9].Tile = 4 // a painted wall side on a walkable square
	if f := plainFloor(&g); f != 22 {
		t.Fatalf("plainFloor %d, want 22", f)
	}
	vs := mapMesh(&g)
	// 1248 floors + two blocks: 2 tops, 6 outer sides (the shared side is gone)
	if got, want := len(vs), (1248+2+6)*6; got != want {
		t.Fatalf("%d vertices, want %d", got, want)
	}
	// faces before (9,9): its 9*50+9 squares, the two blocks having 4 faces each
	if uv := tileUV(22); vs[(9*50+9-2+8)*6].U != uv[0][0] {
		t.Errorf("painted side at (9,9) not shown as the floor")
	}
}
