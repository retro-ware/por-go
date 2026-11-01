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
// transparent, its top left at At.Min and cut to At; or, Fill set (the
// presenter's own layouts; the engine draws none), a fill of Colour over At.
type Picture struct {
	File         string // "" for a fill (the presenter adds its own pictures, the minimap, so)
	Block, Frame int
	Img          image.Image // nil for a fill
	At           image.Rectangle
	Fill         bool
	Colour       color.RGBA
}

// layer is one drawing call: w by h pixels d at at (mask: the value left
// out, -1 none).
type layer struct {
	at     image.Rectangle
	d      []byte
	w      int
	mask   int
	opaque bool            // it hides everything under it
	img    image.Image     // d in the colours of pageState.pal (nil: not converted yet)
	clip   image.Rectangle // drawn only inside (empty: all of it)
	id     uint64
	src    *dax.Image // what d is a frame of (nil: the engine's own pixels)
	frame  int        // its frame
}

type pageState struct {
	layers []layer
	held   int      // Hold depth: the page is not published meanwhile
	pal    pal.Regs // the palette the layers' images are in
	ids    uint64   // layers drawn
	tx     texts    // the text the game wrote (text.go)
}

// draw records l over the page: layers it hides go, and the text it
// covers.
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
	e.unwrite(r)

}

// erase takes what is drawn wholly inside r off the page, and its text.
func (e *Engine) erase(r image.Rectangle) {
	pg := &e.pg
	keep := pg.layers[:0]
	for _, o := range pg.layers {
		if !o.at.In(r) {
			keep = append(keep, o)
		}
	}
	clear(pg.layers[len(keep):])
	pg.layers = keep
	e.unwrite(r)
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
	e.newPage()
	e.pg.tx = texts{}
}

// newPage ends what belongs to the page being replaced: the sheet's
// page (its portrait box), the icon's drop.
func (e *Engine) newPage() {
	e.snap.portrait, e.snap.iconDrop, e.snap.iconBox, e.snap.sheet, e.snap.results, e.snap.credits, e.snap.full = CellRect{}, nil, CellRect{}, nil, nil, false, false
}

// resizePage gives the page cols by rows cells (40x25 at least): what is
// drawn stays where it is, then the screen draws itself up again at the
// new size (ext.redraw); a question is the presenter's to lay out.
func (e *Engine) resizePage(cols, rows int) {
	cols, rows = max(cols, minCols), max(rows, minRows)
	if cols == textCols && rows == textRows {
		return
	}
	prompt := e.pg.tx.prompt // the prompt row moves with the page's bottom
	setPageSize(cols, rows)
	e.pg.tx.prompt = nil
	if len(prompt) > 0 {
		e.pg.tx.prompt = make([]textCell, textCols)
		copy(e.pg.tx.prompt, prompt) // a narrower page cuts the row
	}
	if e.ext.redraw != nil {
		e.ext.redraw()
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
	t := &pg.tx
	v.Adventure = e.adventure()
	if p := e.pane(); v.Adventure || p.Shown() {
		v.Pane = p
	}
	v.Prompt = e.runs(t.prompt)
	for y, row := range t.view {
		if l := e.runs(row); len(l) > 0 {
			for len(v.ViewText) <= y {
				v.ViewText = append(v.ViewText, nil)
			}
			v.ViewText[y] = l
		}
	}
}

// pane is the text pane's text as runs.
func (e *Engine) pane() *Pane {
	p := &Pane{}
	for i, row := range e.pg.tx.pane {
		if len(row) > TextPane.X1 {
			p.Lines[i] = e.runs(row[TextPane.X1:])
		}
	}
	return p
}

// runs is a row of text as runs of one colour, X from the row's start.
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

// adventure: the party is out in the world (not the party menu, the
// finale's pictures, or help, the journal, the maps over it).
func (e *Engine) adventure() bool {
	return e.beginFlag != 0 && e.beginFlag != 7 && !e.modal()
}
