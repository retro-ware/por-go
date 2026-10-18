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
	tile := image.NewRGBA(image.Rect(0, 0, tilePx, tilePx))
	for i := range tilePx * tilePx {
		tile.SetRGBA(i%tilePx, i/tilePx, color.RGBA{uint8(i), 50, 60, 255})
	}
	tile.SetRGBA(20, 1, color.RGBA{255, 255, 255, 255}) // in the bevel
	tile.SetRGBA(5, 5, color.RGBA{})                    // mortar
	mortar := color.RGBA{1, 2, 3, 255}
	got := stoneTexture([]image.Image{nil, nil, nil, tile}, mortar)
	if got.RGBAAt(20, 1) != tile.RGBAAt(1, 20) {
		t.Errorf("bevel pixel %v, want the mirrored stone %v", got.RGBAAt(20, 1), tile.RGBAAt(1, 20))
	}
	if got.RGBAAt(5, 5) != mortar {
		t.Errorf("mortar %v, want the ground colour", got.RGBAAt(5, 5))
	}
}

func TestKeys(t *testing.T) {
	p := &Presenter{}
	p.bar.rects = make([]image.Rectangle, 3)
	p.bar.keys = []byte{'M', 'A', 'D'}
	for k, want := range map[uint16]uint16{'1': 'M', '3': 'D', '4': 0, '0': 0, 8: 'D', 'w': 0, 'Q': 0} {
		if got, taken := p.Key(k); got != want || !taken {
			t.Errorf("key %#x: %#x %v, want %#x taken", k, got, taken, want)
		}
	}
	if _, taken := p.Key(0x4800); taken {
		t.Error("arrows belong to the game")
	}
	if p.yawTo == 0 {
		t.Error("Q did not turn the camera")
	}
}
