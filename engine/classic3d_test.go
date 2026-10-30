package engine

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The original's 3D view renderer (ovr30), kept for the tests only: the
// remaster's comparison sheets (phlan-poc/compare/compare.py) render the
// original's view of a square through TestClassicViews. The game itself
// never draws it (geo.go Draw3DView).
//
// A wall value 1..15 indexes the 15 wall definitions of the 3 WALLDEF
// slots; each definition is 0x9C symbol ids laid out per view part.

const (
	walldefSize  = 0x9C     // one wall's symbol ids
	walldefSet   = 5 * 0x9C // 0x30C: one WALLDEF block chunk
	symWallBase  = 0x2D     // DS:272C: ids >= this are relocated per slot
	viewX, viewY = 3, 3     // 3D view origin in screen cells
)

var (
	classicWalldef [3 * walldefSet]byte // DS:6A58
	classicSky     [4]byte              // DS:6A60..6A63: sky, horizon, band, ground
)

// View part tables DS:0C8C..0CD1, indexed by part 0..9: first symbol in
// the wall definition, width and height in cells, and the view cell of the
// part's top-left corner.
var (
	partBase = [10]int{0, 2, 6, 10, 22, 38, 54, 110, 132, 154}
	partW    = [10]int{1, 1, 1, 3, 2, 2, 7, 2, 2, 1}
	partH    = [10]int{2, 4, 4, 4, 8, 8, 8, 11, 11, 2}
	partX    = [10]int{5, 4, 6, 4, 2, 7, 2, 0, 9, 5}
	partY    = [10]int{4, 3, 3, 3, 1, 1, 1, 0, 0, 4}
)

// classicFillRect = 0464:2632 (x cells, y rows, w cells, h rows, colour), in page coordinates.
func (e *Engine) classicFillRect(x, y, w, h int, colour byte) {
	e.fill(x*8+8, y+8, (x+w)*8+8, y+h+8, colour)
}

// classicSkyBands = ovr30:0174: the four colour bands behind the walls.
func (e *Engine) classicSkyBands() {
	for i, band := range [4][2]int{{0x10, 0x2B}, {0x3B, 1}, {0x3C, 2}, {0x3E, 0x2A}} {
		e.classicFillRect(viewX-1, band[0], 0xB, band[1], classicSky[i])
	}
}

// classicWallPart = ovr30:0211 (x, y, wall, part): blit one part of a wall
// definition at view cell (x,y), clipped to the 11x11 view, masked.
func (e *Engine) classicWallPart(x, y int, wall byte, part int) {
	s := partBase[part]
	for yy := y; yy <= y+partH[part]-1; yy++ {
		for xx := x; xx <= x+partW[part]-1; xx++ {
			if yy >= 0 && yy <= 10 && xx >= 0 && xx <= 10 {
				if id := classicWalldef[(int(wall)-1)*walldefSize+s]; id > 0 {
					e.Put8x8SymbolMasked(xx+viewX, yy+viewY, int(id))
				}
			}
			s++
		}
	}
}

// classicView = ovr30:05FD (x, y, dir): the first-person view from cell
// (x,y). Walls are drawn far to near: the row two cells ahead, the row one
// ahead, then the party's own cell, each scanned outwards from the centre.
func (e *Engine) classicView(x, y, dir int) {
	e.classicSkyBands()
	left, right, back := (dir+6)%8, (dir+2)%8, (dir+4)%8
	cx, cy := x+2*dirDX[dir], y+2*dirDY[dir]
	for dist := 2; dist >= 0; dist-- {
		switch dist {
		case 2:
			for _, side := range [2]struct{ dir, step, edge int }{{left, -2, 1}, {right, 2, -1}} {
				px, py, off := cx, cy, 0
				var prev byte
				for i := 0; i < 4; i++ {
					front := e.WallAt(px, py, dir)
					if !onMap(px, py) && e.WallAt(px, py, (side.dir+4)%8) == 0 {
						prev = 0
					}
					if front != 0 {
						if prev > 0 {
							e.classicWallPart(partX[9]+off+side.edge, partY[9], prev, 9)
						}
						prev = front
						e.classicWallPart(partX[0]+off, partY[0], front, 0)
					} else {
						if prev > 0 && e.WallAt(px-dirDX[side.dir], py-dirDY[side.dir], side.dir) != 0 {
							e.classicWallPart(partX[9]+off+side.edge, partY[9], prev, 9)
						}
						prev = 0
					}
					off += side.step
					px += dirDX[side.dir]
					py += dirDY[side.dir]
				}
			}
			for _, side := range [2]struct{ dir, step, part int }{{left, -2, 1}, {right, 2, 2}} {
				px, py, off := cx, cy, 0
				for i := 0; i < 3; i++ {
					if w := e.WallAt(px, py, side.dir); w != 0 {
						d := 0
						if i != 0 {
							d = side.step / 2
						}
						e.classicWallPart(partX[side.part]+off+d, partY[side.part], w, side.part)
					}
					off += side.step
					px += dirDX[side.dir]
					py += dirDY[side.dir]
				}
			}
		case 1:
			for _, side := range [2]struct{ from, toward, off, step, part int }{
				{left, right, -6, 3, 4}, {right, left, 6, -3, 5}} {
				px, py, off := cx+2*dirDX[side.from], cy+2*dirDY[side.from], side.off
				for i := 0; i < 3; i++ {
					if w := e.WallAt(px, py, dir); w != 0 {
						e.classicWallPart(partX[3]+off, partY[3], w, 3)
					}
					if w := e.WallAt(px, py, side.from); w != 0 {
						e.classicWallPart(partX[side.part]+off, partY[side.part], w, side.part)
					}
					off += side.step
					px += dirDX[side.toward]
					py += dirDY[side.toward]
				}
			}
		case 0:
			for _, side := range [2]struct{ from, toward, off, step, part int }{
				{left, right, -7, 7, 7}, {right, left, 7, -7, 8}} {
				px, py, off := cx+dirDX[side.from], cy+dirDY[side.from], side.off
				for i := 0; i < 2; i++ {
					if w := e.WallAt(px, py, dir); w != 0 {
						e.classicWallPart(partX[6]+off, partY[6], w, 6)
					}
					if w := e.WallAt(px, py, side.from); w != 0 {
						e.classicWallPart(partX[side.part]+off, partY[side.part], w, side.part)
					}
					off += side.step
					px += dirDX[side.toward]
					py += dirDY[side.toward]
				}
			}
		}
		cx += dirDX[back]
		cy += dirDY[back]
	}
}

// classicWalldefLoad = ovr30:0E08 (block, slot): WALLDEF<n>.DAX block into
// wall slot 1..3. A block may hold several 5-wall chunks; they fill
// consecutive slots, and each slot gets its own symbol set (8X8D block
// block*10+k for multi-chunk blocks, else block itself). Symbol ids >=
// 0x2D are moved up to that slot's symbol range.
func (e *Engine) classicWalldefLoad(block, slot int) error {
	b := e.L.Block(e.set.file("WALLDEF"), block)
	if len(b) == 0 || slot+len(b)/walldefSet > 4 {
		return fmt.Errorf("no WALLDEF block %d for slot %d", block, slot)
	}
	reloc := byte(symFix[slot] - symFix[1])
	n := 0
	for o := 0; o+walldefSet <= len(b); o += walldefSet {
		s := slot + n
		copy(classicWalldef[(s-1)*walldefSet:s*walldefSet], b[o:])
		n++
	}
	for k := 1; k <= n; k++ {
		s := slot + k - 1
		if !validWallSlot(s) {
			continue
		}
		for i := (s - 1) * walldefSet; i < s*walldefSet; i++ {
			if classicWalldef[i] >= symWallBase {
				classicWalldef[i] += reloc
			}
		}
		if n > 1 {
			bl := block
			if bl == 0 {
				bl = 0xA
			}
			e.LoadSymbolSet(s, bl*10+k)
		} else {
			e.LoadSymbolSet(s, block)
		}
	}
	return nil
}

// pageRGBA rasterises the page as the presenter does: the layers, then the
// text cells over them.
func (e *Engine) pageRGBA() *image.RGBA {
	e.publish()
	v := e.View()
	m := image.NewRGBA(image.Rect(0, 0, textCols*8, textRows*8))
	draw.Draw(m, m.Bounds(), image.Black, image.Point{}, draw.Src)
	for _, s := range v.Page {
		op := draw.Over
		if _, ok := s.Img.(*image.Uniform); ok {
			op = draw.Src
		}
		draw.Draw(m, s.At, s.Img, s.Img.Bounds().Min, op)
	}
	for y, row := range v.Text {
		for x, c := range row {
			if c.Set {
				for r := 0; r < 8; r++ {
					for b := 0; b < 8; b++ {
						if c.Glyph[r]&(0x80>>b) != 0 {
							m.SetRGBA(x*8+b, y*8+r, c.Fg)
						} else {
							m.SetRGBA(x*8+b, y*8+r, c.Bg)
						}
					}
				}
			}
		}
	}
	return m
}

// TestClassicViews writes the original's 88x88 view of squares as PNGs for
// the remaster's comparison sheets. POR_CLASSIC names them:
// "SET GEO KIND WALLS VIEWS OUT", WALLS as block:slot,block:slot (the
// loads in the area script's order), VIEWS as x,y,D;x,y,D (D one of NESW),
// OUT the directory of the NN_por.png files, numbered from 1.
func TestClassicViews(t *testing.T) {
	spec := os.Getenv("POR_CLASSIC")
	if spec == "" {
		t.Skip("POR_CLASSIC not set")
	}
	f := strings.Fields(spec)
	if len(f) != 6 {
		t.Fatalf("POR_CLASSIC wants 6 fields: %q", spec)
	}
	e, err := New("../data")
	if err != nil {
		t.Fatal(err)
	}
	num := func(s string) int {
		n, err := strconv.Atoi(s)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	e.set = Set(num(f[0]))
	e.LoadSymbolSet(4, 202)
	e.LoadSymbolSet(0, 203)
	e.LoadGeo(num(f[1]))
	e.gs.areaKind = byte(num(f[2]))
	for _, w := range strings.Split(f[3], ",") {
		bs := strings.Split(w, ":")
		if err := e.classicWalldefLoad(num(bs[0]), num(bs[1])); err != nil {
			t.Fatal(err)
		}
	}
	classicSky = [4]byte{0xB, 0, 7, 6} // ovr30:0155 as the game starts
	for i, v := range strings.Split(f[4], ";") {
		p := strings.Split(v, ",")
		x, y, d := num(p[0]), num(p[1]), 2*strings.Index("NESW", p[2])
		if d < 0 {
			t.Fatalf("view %q", v)
		}
		e.clearPage()
		e.classicView(x, y, d)
		m := e.pageRGBA()
		r := image.Rect(viewX*8, viewY*8, viewX*8+88, viewY*8+88)
		out, err := os.Create(fmt.Sprintf("%s/%02d_por.png", f[5], i+1))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(out, m.SubImage(r)); err != nil {
			t.Fatal(err)
		}
		out.Close()
	}
}
