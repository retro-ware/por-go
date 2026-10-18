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
	p.bar.keys = []byte{'A', 'Q', 'D'}
	p.bar.texts = []string{"Attack", "Quick", "Done"}
	for k, want := range map[uint16]string{'1': "A", '3': "D", '4': "", '0': "", 8: "DQ", 'w': "", 'Q': "", 13: "A"} {
		got, taken := p.Key(k)
		s := ""
		for _, k := range got {
			s += string(rune(k))
		}
		if s != want || !taken {
			t.Errorf("key %#x: %q %v, want %q taken", k, s, taken, want)
		}
	}
	if _, taken := p.Key(0x4800); taken {
		t.Error("arrows belong to the game")
	}
	if p.yawTo == 0 {
		t.Error("Q did not turn the camera")
	}
	p.bar.texts[1] = "Quit" // the Done menu: Backspace is its Quit
	if got, _ := p.Key(8); len(got) != 1 || got[0] != 'Q' {
		t.Errorf("Backspace on the Done menu: %v", got)
	}
}

func TestShownItems(t *testing.T) {
	m := &engine.Menu{Sel: 0, Items: []engine.MenuItem{{Text: "Move", Key: 'M'}, {Text: "Aim", Key: 'A'}, {Text: "Done", Key: 'D'}}}
	items, sel := shownItems(m)
	if len(items) != 2 || items[0].Text != "Attack" || sel != -1 {
		t.Errorf("main menu: %+v sel %d", items, sel)
	}
}
