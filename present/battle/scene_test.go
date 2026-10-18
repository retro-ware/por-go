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
	m := &engine.Menu{Items: []engine.MenuItem{{Text: "Move", Key: 'M'}, {Text: "View", Key: 'V'}, {Text: "Aim", Key: 'A'}, {Text: "Quick", Key: 'Q'}, {Text: "Done", Key: 'D'}}}
	p.bar.items, _ = shownItems(m, []engine.MenuItem{{Text: "Wait", Key: 'W'}})
	p.bar.rects = make([]image.Rectangle, len(p.bar.items))
	str := func(ks []uint16) string {
		s := ""
		for _, k := range ks {
			s += string(rune(k))
		}
		return s
	}
	for k, want := range map[uint16]string{'1': "A", '2': "W", '3': "DQ", '4': "Q", '5': "", '0': "", 8: "DQ", 'w': "", 'Q': ""} { // Quick last
		if got, taken := p.Key(k); str(got) != want || !taken {
			t.Errorf("key %#x: %q %v, want %q taken", k, str(got), taken, want)
		}
	}
	for k, want := range map[uint16]string{'u': "Q", 'p': "W", 'g': ""} { // the remaster's letters; no Guard on this bar
		if got, _ := p.Key(k); str(got) != want {
			t.Errorf("hotkey %c: %q, want %q", k, str(got), want)
		}
	}
	if got, taken := p.Key(0x4800); got != nil || !taken {
		t.Error("arrows pan the camera with the bar up")
	}
	p.bar.rects = nil
	if _, taken := p.Key(0x4800); taken {
		t.Error("arrows step in Move (no bar)")
	}
	p.bar.rects = make([]image.Rectangle, len(p.bar.items))
	if _, taken := p.Key(13); taken {
		t.Error("Enter off the aim bar belongs to the game")
	}
	if p.yawTo == 0 {
		t.Error("Q did not turn the camera")
	}
	aim := &engine.Menu{Prompt: "Aim:", Items: []engine.MenuItem{{Text: "Next", Key: 'N'}, {Text: "Prev", Key: 'P'}, {Text: "Manual", Key: 'M'}, {Text: "Attack", Key: 'A'}, {Text: "Center", Key: 'C'}, {Text: "Exit", Key: 'E'}}}
	p.bar.items, _ = shownItems(aim, nil)
	p.bar.rects, p.aiming = make([]image.Rectangle, len(p.bar.items)), true
	if len(p.bar.items) != 3 || p.bar.items[0].Text != "Free" || p.bar.items[2].Text != "Cancel" {
		t.Errorf("aim bar: %+v", p.bar.items)
	}
	if got, _ := p.Key(13); str(got) != "A" {
		t.Errorf("Enter aiming: %q, want A", str(got))
	}
	if got, _ := p.Key(9); str(got) != "N" {
		t.Errorf("Tab aiming: %q, want N", str(got))
	}
	p.bar.items = []barItem{{'Q', "Quit", []uint16{'Q'}}} // the Done menu: Backspace is its Quit
	p.bar.rects = make([]image.Rectangle, 1)
	if got, _ := p.Key(8); str(got) != "Q" {
		t.Errorf("Backspace on the Done menu: %q", str(got))
	}
}

func TestButtonRects(t *testing.T) {
	r := buttonRect(400, 3, 2, 2) // a hint wider than the buttons centres them
	if bw := face + 8; r.Dx() != bw || r.Min.X != (400-3*(bw+8)+8)/2+2*(bw+8) {
		t.Errorf("button 2 of 3 on a 400 wide bar: %v", r)
	}
}
