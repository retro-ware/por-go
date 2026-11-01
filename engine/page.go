package engine

import (
	"image"
	"image/color"

	"por/dax"
	"por/pal"
)

// The page (extra): what the original drew on its 320x200 screen, kept as
// typed layers (pictures, symbols and fills in drawing order) under the text
// cells, so a presenter draws the screen from the View.

// Picture is one thing drawn on the page, bottom first: frame Frame of
// block Block of the DAX file File (a picture, an animation's frame, a
// symbol, a tile) decoded in the page's palette with its mask pixels
// transparent, its top left at At.Min and cut to At; or, Fill set, a fill
// of Colour over At.
type Picture struct {
	File         string // "" for pixels the engine made itself (the full map)
	Block, Frame int
	Img          image.Image // nil for a fill
	At           image.Rectangle
	Fill         bool
	Colour       color.RGBA
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
	src    *dax.Image // what d is a frame of (nil: the engine's own pixels)
	frame  int        // its frame
}

type pageState struct {
	layers  []layer
	held    int      // Hold depth: the page is not published meanwhile
	frameUp bool     // DrawMainFrame's boxes are still whole
	pal     pal.Regs // the palette the layers' images are in
	ids     uint64   // layers drawn
}

// draw records l over the page: layers it hides go, and text cells it
// touches with them.
func (e *Engine) draw(l layer) {
	pg := &e.pg
	r := l.at.Intersect(image.Rect(0, 0, textCols*8, textRows*8))
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
			e.cells()[y][x].set = false
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

// picture records a blit of w by h pixels d, frame frame of src, at x0,
// y0 (mask as blit's).
func (e *Engine) picture(x0, y0, w, h int, d []byte, mask int, src *dax.Image, frame int) {
	d = d[:w*h]
	opaque := true
	for _, p := range d {
		if int(p) == mask || p > 16 && p < pal.AmigaBase {
			opaque = false
			break
		}
	}
	e.draw(layer{at: image.Rect(x0, y0, x0+w, y0+h), d: d, w: w, mask: mask, opaque: opaque, src: src, frame: frame})
}

// clearPage starts an empty page (ClearScreen).
func (e *Engine) clearPage() {
	clear(e.pg.layers)
	e.pg.layers = e.pg.layers[:0]
	e.pg.frameUp = false
	e.newPage()
	e.ext.cells = newCells()
}

// newPage ends what belongs to the page being replaced: the sheet's
// page (its portrait box), the icon's drop.
func (e *Engine) newPage() {
	e.snap.portrait, e.snap.iconDrop, e.snap.iconBox, e.snap.sheet = CellRect{}, nil, CellRect{}, nil
}

// resizePage gives the page cols by rows cells (40x25 at least): what is
// drawn stays where it is, then the screen draws itself up again at the
// new size (ext.redraw); a question is the presenter's to lay out.
func (e *Engine) resizePage(cols, rows int) {
	cols, rows = max(cols, minCols), max(rows, minRows)
	if cols == textCols && rows == textRows {
		return
	}
	old, oldPrompt := e.cells(), promptRow
	setPageSize(cols, rows)
	e.ext.cells = newCells()
	for y := range min(len(old), textRows) {
		copy(e.ext.cells[y], old[y])
	}
	e.glowOn(nil)
	if e.ext.redraw != nil {
		e.ext.redraw()
	}
	if oldPrompt < textRows { // the bar's old row would stay inside the page
		e.DisplaySpaces(0, oldPrompt, colBlack, len(old[0]))
	}
}

// redrawWith sets f as the screen's redraw (resizePage) until the func
// returned puts the one before back.
func (e *Engine) redrawWith(f func()) func() {
	was := e.ext.redraw
	e.ext.redraw = f
	return func() { e.ext.redraw = was }
}

// pageView fills v's page from the layers and cells.
func (e *Engine) pageView(v *View) {
	pg := &e.pg
	stale := pg.pal != e.pal
	pg.pal = e.pal
	v.Pictures = make([]Picture, 0, len(pg.layers))
	for i := range pg.layers {
		l := &pg.layers[i]
		if l.d == nil {
			v.Pictures = append(v.Pictures, Picture{At: l.at, Fill: true, Colour: e.pal.RGBA(l.fill)})
			continue
		}
		if l.img == nil || stale {
			l.img = e.layerRGBA(*l)
		}
		s := Picture{Img: l.img, At: l.at}
		if l.src != nil {
			s.File, s.Block, s.Frame = l.src.File, l.src.Block, l.src.Base+l.frame
		}
		if !l.clip.Empty() {
			s.At = l.at.Intersect(l.clip)
			s.Img = l.img.(*image.RGBA).SubImage(s.At.Sub(l.at.Min))
		}
		v.Pictures = append(v.Pictures, s)
	}
	v.Cells = image.Pt(textCols, textRows)
	v.Lines = make([][]PaneRun, textRows)
	for y, row := range e.cells() {
		v.Lines[y] = e.runs(row)
	}
	if pg.frameUp && !e.modal() {
		v.Pane = e.pane()
	}
}

// pane is the text pane's cells as runs.
func (e *Engine) pane() *Pane {
	p := &Pane{}
	cells := e.cells()
	for i := range p.Lines {
		p.Lines[i] = e.runs(cells[TextPane.Y1+i][TextPane.X1 : TextPane.X2+1])
	}
	return p
}

// runs is a row of text cells as runs of one colour, X from the row's start.
func (e *Engine) runs(row []textCell) []PaneRun {
	var l []PaneRun
	for x, c := range row {
		if !c.set {
			continue
		}
		fg, bg := e.pal.RGBA(c.colour), e.pal.RGBA(c.bg)
		if n := len(l) - 1; n >= 0 && l[n].X+len(l[n].Text) == x && l[n].Fg == fg && l[n].Bg == bg && l[n].Upcase == c.upcase {
			l[n].Text += string(c.c)
			continue
		}
		l = append(l, PaneRun{X: x, Text: string(c.c), Fg: fg, Bg: bg, Upcase: c.upcase})
	}
	return l
}

// layerRGBA converts l's pixels in the current palette, as blit draws them.
func (e *Engine) layerRGBA(l layer) image.Image {
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
