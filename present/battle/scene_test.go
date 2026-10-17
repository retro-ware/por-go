package battle

import (
	"image"
	"image/color"
	"testing"

	"por/engine"
)

func TestMapMesh(t *testing.T) {
	var g [25][50]engine.Square
	g[5][5], g[5][6] = engine.Square{Tile: 1, Wall: true}, engine.Square{Tile: 2, Wall: true}
	g[9][9].Tile = 4 // a dungeon piece on a walkable square: floor
	none := func(int) bool { return false }
	vs := mapMesh(&g, true, none, nil)
	// 1248 floors + two blocks: 2 tops, 6 outer sides (the shared side is gone)
	if got, want := len(vs), (1248+2+6)*6; got != want {
		t.Fatalf("%d vertices, want %d", got, want)
	}
	if k := kindOf(engine.Square{Tile: 30, Wall: true, Height: 2}, false, none); k != stand {
		t.Errorf("outdoor sight blocker is %v, want stand", k)
	}
	if k := kindOf(engine.Square{Tile: 30, Wall: true}, false, none); k != flat {
		t.Errorf("water is %v, want flat", k)
	}
}

func TestStoneTexture(t *testing.T) {
	stone := func(seed uint8) *image.RGBA {
		im := image.NewRGBA(image.Rect(0, 0, tilePx, tilePx))
		for i := range tilePx * tilePx {
			im.SetRGBA(i%tilePx, i/tilePx, color.RGBA{uint8(i) * 7, 50, 60, 255})
		}
		return im
	}
	tiles := make([]image.Image, 6)
	for i := range tiles {
		tiles[i] = stone(0)
	}
	bevel := tiles[3].(*image.RGBA)
	bevel.SetRGBA(20, 1, color.RGBA{255, 255, 255, 255}) // tile 3's own bevel pixel
	bevel.SetRGBA(0, 10, color.RGBA{})                    // a hole
	want := stone(0)
	got := stoneTexture(tiles)
	if got.RGBAAt(20, 1) != want.RGBAAt(20, 1) {
		t.Errorf("bevel pixel kept: %v", got.RGBAAt(20, 1))
	}
	if got.RGBAAt(0, 10).A != 255 {
		t.Errorf("hole not filled")
	}
}
