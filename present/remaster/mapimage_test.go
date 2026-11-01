package remaster

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"por/engine"
)

// The maps (the minimap, View.Map, the full map) were pictures the engine
// drew; testdata/map_*.png are those pictures, recorded with their typed
// data (map_*.json) at a few squares, the map's edges among them. The
// presenter's mapImage of the data must give them pixel for pixel.
func TestMapImage(t *testing.T) {
	var ink [16]color.RGBA
	for i := range ink {
		ink[i] = col(i)
	}
	e, err := engine.New("../../data")
	if err != nil {
		t.Skip("no game files")
	}
	e.Init() // the font
	files, _ := filepath.Glob("testdata/map_*.json")
	if len(files) == 0 {
		t.Fatal("no recorded maps")
	}
	for _, f := range files {
		var m engine.MapSquares
		b, _ := os.ReadFile(f)
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		r, err := os.Open(strings.TrimSuffix(f, ".json") + ".png")
		if err != nil {
			t.Fatal(err)
		}
		want, err := png.Decode(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		cell, glyph := want.Bounds().Dx()/16, e.FontGlyph
		if strings.HasSuffix(f, "_mini.json") {
			glyph = nil
		}
		got := mapImage(&m, cell, ink, glyph)
		for y := range want.Bounds().Dy() {
			for x := range want.Bounds().Dx() {
				if w := color.RGBAModel.Convert(want.At(x, y)); got.RGBAAt(x, y) != w {
					t.Fatalf("%s: pixel %v is %v, want %v", f, image.Pt(x, y), got.RGBAAt(x, y), w)
				}
			}
		}
	}
}
