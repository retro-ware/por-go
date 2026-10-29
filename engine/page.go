package engine

import (
	"image"
	"image/color"

	"por/pal"
)

// The page (extra): what the original drew on its 320x200 screen, kept as
// typed layers (pictures, symbols and fills in drawing order) under the text
// cells, so a presenter draws the screen from the View.

// Sprite is one thing drawn on the page: Img with its top left at At.Min
// (a fill: a uniform colour over At).
type Sprite struct {
	Img image.Image
	At  image.Rectangle
}

// Cell is a text cell: the glyph's set bits in Fg, the rest in Bg.
type Cell struct {
	Glyph  [8]byte
	Fg, Bg color.RGBA
	Set    bool // the cell holds text
}

// Overlay is a picture the window draws over the page at its own
// resolution: the part Src of Img (empty: all of it) scaled into the
// page rectangle At (the journal's scans).
type Overlay struct {
	Img     image.Image
	Src, At image.Rectangle
}

// Part is the rectangle of Img the overlay shows.
func (o Overlay) Part() image.Rectangle {
	if o.Src.Empty() {
		return o.Img.Bounds()
	}
	return o.Src
}

// layer is one drawing call: w by h pixels d at at (mask: the value left
// out, -1 none), or a fill of colour fill (d nil).
type layer struct {
	at     image.Rectangle
	d      []byte
	w      int
	mask   int
	fill   uint8
	opaque bool            // it hides everything under it
	img    image.Image     // d in the colours of pageState.pal (nil: not converted yet)
	clip   image.Rectangle // drawn only inside (empty: all of it)
	id     uint64
}

type pageState struct {
	layers   []layer
	overlays []Overlay
	held     int      // Hold depth: the page is not published meanwhile
	frameUp  bool     // DrawMainFrame's boxes are still whole
	pal      pal.Regs // the palette the layers' images are in
	ids      uint64   // layers drawn
	under    uint64   // the sprite whose layer restoreUnder takes away
}

// draw records l over the page: layers it hides go, and text cells it
// touches with them.
func (e *Engine) draw(l layer) {
	pg := &e.pg
	r := l.at.Intersect(image.Rect(0, 0, pal.W, pal.H))
	if r.Empty() {
		return
	}
	if l.opaque {
		keep := pg.layers[:0]
		for _, o := range pg.layers {
			if !o.at.Intersect(r).Eq(o.at) {
				keep = append(keep, o)
			}
		}
		clear(pg.layers[len(keep):])
		pg.layers = keep
	}
	pg.ids++
	l.id = pg.ids
	pg.layers = append(pg.layers, l)
	for y := r.Min.Y / 8; y <= (r.Max.Y-1)/8; y++ {
		for x := r.Min.X / 8; x <= (r.Max.X-1)/8; x++ {
			e.ext.cells[y][x].set = false
		}
	}
	if pg.frameUp {
		for _, b := range []CellRect{ViewWindow, TextPane} {
			f := b.Framed().Pixels()
			if r.Overlaps(f) && !r.In(b.Pixels()) {
				pg.frameUp = false
			}
		}
	}
}

// fill paints a rectangle [x0,x1) x [y0,y1) in colour c (16 is black,
// 17..AmigaBase-1 transparent: nothing).
func (e *Engine) fill(x0, y0, x1, y1 int, c uint8) {
	if c == 16 {
		c = 0
	}
	if c >= 16 && c < pal.AmigaBase {
		return
	}
	e.draw(layer{at: image.Rect(x0, y0, x1, y1), fill: c, opaque: true})
}

// picture records a blit of w by h pixels d at x0, y0 (mask as blit's).
func (e *Engine) picture(x0, y0, w, h int, d []byte, mask int) {
	d = d[:w*h]
	opaque := true
	for _, p := range d {
		if int(p) == mask || p > 16 && p < pal.AmigaBase {
			opaque = false
			break
		}
	}
	e.draw(layer{at: image.Rect(x0, y0, x0+w, y0+h), d: d, w: w, mask: mask, opaque: opaque})
}

// clearPage starts an empty page (ClearScreen).
func (e *Engine) clearPage() {
	clear(e.pg.layers)
	e.pg.layers = e.pg.layers[:0]
	e.pg.frameUp = false
	e.pg.overlays = nil
	e.snap.portrait, e.snap.sheet = CellRect{}, false
	e.ext.cells = [25][40]textCell{}
}

// pageView fills v's page from the layers and cells.
func (e *Engine) pageView(v *View) {
	pg := &e.pg
	stale := pg.pal != e.pal
	pg.pal = e.pal
	v.Page = make([]Sprite, 0, len(pg.layers))
	for i := range pg.layers {
		l := &pg.layers[i]
		if l.img == nil || stale {
			l.img = e.layerRGBA(*l)
		}
		s := Sprite{l.img, l.at}
		if !l.clip.Empty() {
			s.At = l.at.Intersect(l.clip)
			s.Img = l.img.(*image.RGBA).SubImage(s.At.Sub(l.at.Min))
		}
		v.Page = append(v.Page, s)
	}
	for y := range e.ext.cells {
		for x, c := range e.ext.cells[y] {
			if c.set {
				v.Text[y][x] = Cell{Glyph: c.glyph, Fg: e.pal.RGBA(c.colour), Bg: e.pal.RGBA(c.bg), Set: true}
				v.TextShown = v.TextShown || image.Pt(x, y).In(TextPane.rect())
			}
		}
	}
	v.Overlays = pg.overlays
	v.MainFrame = pg.frameUp
}

// rect is r as an image.Rectangle of cells.
func (r CellRect) rect() image.Rectangle { return image.Rect(r.X1, r.Y1, r.X2+1, r.Y2+1) }

// layerRGBA converts l's pixels in the current palette, as blit draws them.
func (e *Engine) layerRGBA(l layer) image.Image {
	if l.d == nil {
		return image.NewUniform(e.pal.RGBA(l.fill))
	}
	h := len(l.d) / l.w
	out := image.NewRGBA(image.Rect(0, 0, l.w, h))
	for i, p := range l.d {
		switch {
		case int(p) == l.mask, p > 16 && p < pal.AmigaBase:
		case p == 16:
			out.SetRGBA(i%l.w, i/l.w, e.pal.RGBA(0))
		default:
			out.SetRGBA(i%l.w, i/l.w, e.pal.RGBA(p))
		}
	}
	return out
}
