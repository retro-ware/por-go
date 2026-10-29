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
	b := &engine.Battle{Turn: &engine.Turn{Can: []engine.ActKind{engine.ActAttack, engine.ActWait, engine.ActEnd, engine.ActQuick}}}
	p.b = b
	p.bar.items = shownItems(b)
	p.bar.rects = make([]image.Rectangle, len(p.bar.items))
	took := func(k uint16) []engine.Action { p.Key(k); return p.Actions() }
	for k, want := range map[uint16]engine.ActKind{'1': engine.ActAttack, '2': engine.ActWait, 8: engine.ActEnd, 'u': engine.ActQuick, 'p': engine.ActWait} {
		if got := took(k); len(got) != 1 || got[0].Kind != want {
			t.Errorf("key %#x: %+v, want kind %d", k, got, want)
		}
	}
	for _, k := range []uint16{'5', 'g', 'w', 'Q', 0x4800} {
		if got := took(k); len(got) != 0 {
			t.Errorf("key %#x: %+v, want nothing", k, got)
		}
	}
	if p.yawTo == 0 {
		t.Error("Q did not turn the camera")
	}
	b.Turn = &engine.Turn{Aim: true}
	b.Targets, b.Units, b.Cursor = []int{3, 5}, []engine.Unit{{ID: 3, X: 1, Y: 1}, {ID: 5, X: 4, Y: 2}}, image.Pt(1, 1)
	p.bar.items = shownItems(b)
	if len(p.bar.items) != 1 || p.bar.items[0].Text != "Cancel" {
		t.Errorf("aim bar: %+v", p.bar.items)
	}
	if got := took(9); len(got) != 1 || got[0] != (engine.Action{Kind: engine.ActPoint, X: 4, Y: 2}) {
		t.Errorf("Tab aiming: %+v, want the next target", got)
	}
	if got := took(13); len(got) != 1 || got[0] != (engine.Action{Kind: engine.ActAim, X: 1, Y: 1}) {
		t.Errorf("Enter aiming: %+v", got)
	}
	if got := took(27); len(got) != 1 || got[0].Kind != engine.ActClose {
		t.Errorf("Esc aiming: %+v", got)
	}
}

func TestButtonRects(t *testing.T) {
	r := buttonRect(400, 3, 2, 2) // a hint wider than the buttons centres them
	if bw := face + 8; r.Dx() != bw || r.Min.X != (400-3*(bw+8)+8)/2+2*(bw+8) {
		t.Errorf("button 2 of 3 on a 400 wide bar: %v", r)
	}
}

// TestNDC: ndc is pick's inverse: a square's centre projects to a screen
// point that picks the square again, and the eye's own square is out of sight.
func TestNDC(t *testing.T) {
	c := camera{eye: vec{20, 9, 20}, at: vec{14, 0, 8}, fovy: 0.8, aspect: 4.0 / 3}
	for _, w := range []vec{{14.5, 0, 8.5}, {10.5, 0, 6.5}, {17.5, 0, 12.5}} {
		nx, ny, ok := c.ndc(w)
		x, z, picked := c.pick(nx, ny)
		if !ok || !picked || x != int(w[0]) || z != int(w[2]) {
			t.Errorf("%v → (%.2f, %.2f) %v → %d,%d %v", w, nx, ny, ok, x, z, picked)
		}
	}
	if _, _, ok := c.ndc(vec{26, 0, 32}); ok {
		t.Error("a point behind the eye is in sight")
	}
	if _, _, ok := (camera{}).ndc(vec{1, 0, 1}); ok {
		t.Error("the zero camera sees")
	}
}
