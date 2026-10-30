package remaster

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"

	xdraw "golang.org/x/image/draw"

	"gpu"

	"por/engine"
	"por/pal"
)

// Rect is where the page lands on the window, in device pixels.
type Rect struct{ X, Y, W, H float64 }

// Note is a toast shown on the bottom line of the page (row 24, in place
// of the prompt), in the game's current font.
type Note struct {
	Text  string
	Show  bool
	Glyph func(byte) [8]byte
}

// page is the engine's screen (pageSize: 320x200 at least) drawn from the
// View's Page and Text.
type page struct {
	img   *gpu.Image
	px    *image.RGBA
	seq   uint64
	noted bool
	drawn bool
	page  []engine.Sprite      // what img shows: the View's page...
	text  [][]engine.Cell      // ...and text
	scans map[image.Image]scan // the overlays on the GPU
	fit   *gpu.Image           // fit's whole-scale copy
}

// pageSize is v's page in pixels: its text cells, 8 by 8 (the original's
// 320x200 for a View without any).
func pageSize(v engine.View) (w, h int) {
	if len(v.Text) == 0 || len(v.Text[0]) == 0 {
		return pal.W, pal.H
	}
	return len(v.Text[0]) * 8, len(v.Text) * 8
}

// grid is the text cells the window holds: 40x25 at least, more at the
// largest whole scale the original's screen fits (scale), so a larger
// window shows more rather than bigger cells; the engine lays its screens
// out to it (ActResize).
func grid(dst *gpu.Image) (cols, rows int) {
	b, k := dst.Bounds(), int(scale(dst))
	return max(40, b.Dx()/(8*k)), max(25, b.Dy()/(8*k))
}

// fit draws img, w by h unscaled pixels, over the whole of dst: nearest
// neighbour at the largest whole scale that fits (sharp pixels), then
// stretched the rest of the way to the window's edges, under a cell on
// each axis when the page is the window's grid. No black bars, ever:
// every screen fills the window. It returns the scale on each axis.
func (p *Presenter) fit(dst, img *gpu.Image, w, h int) (kx, ky float64) {
	b := dst.Bounds()
	k := max(1, min(b.Dx()/w, b.Dy()/h))
	kx, ky = float64(b.Dx())/float64(w), float64(b.Dy())/float64(h)
	op := &gpu.DrawImageOptions{}
	if kx == float64(k) && ky == float64(k) {
		op.GeoM.Scale(kx, ky)
		dst.DrawImage(img, op)
		return
	}
	pg := &p.pg
	if pg.fit == nil || pg.fit.Bounds().Dx() != w*k || pg.fit.Bounds().Dy() != h*k {
		if pg.fit != nil {
			pg.fit.Deallocate()
		}
		pg.fit = gpu.NewImage(w*k, h*k)
	}
	op.GeoM.Scale(float64(k), float64(k))
	pg.fit.DrawImage(img, op)
	op = &gpu.DrawImageOptions{Filter: gpu.FilterLinear}
	op.GeoM.Scale(kx/float64(k), ky/float64(k))
	dst.DrawImage(pg.fit, op)
	return
}

// scan is an overlay on the GPU: the picture itself (w, h 0), or a copy
// scaled up to w by h window pixels (Catmull-Rom: sharper than the GPU's
// bilinear).
type scan struct {
	im   *gpu.Image
	w, h int
}

// pageImage draws v's page (only when a new View or the note changed) and
// returns it.
func (p *Presenter) pageImage(v engine.View, note Note) *gpu.Image {
	pg := &p.pg
	w, h := pageSize(v)
	if pg.img == nil || pg.px.Rect.Dx() != w || pg.px.Rect.Dy() != h {
		if pg.img != nil {
			pg.img.Deallocate()
		}
		pg.img, pg.px, pg.drawn = gpu.NewImage(w, h), image.NewRGBA(image.Rect(0, 0, w, h)), false
	}
	if pg.drawn && !note.Show && !pg.noted && (v.Seq == pg.seq || samePage(v, pg)) { // the View changed, the page not: no redraw, no upload
		pg.seq = v.Seq
		return pg.img
	}
	pg.drawn, pg.seq, pg.noted = true, v.Seq, note.Show
	pg.page, pg.text = append(pg.page[:0], v.Page...), v.Text
	m := pg.px
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
				glyphAt(m, c.Glyph, x*8, y*8, c.Fg, c.Bg)
			}
		}
	}
	if note.Show { // yellow, centred on the bottom line in place of the prompt (as the inventory's menu row)
		draw.Draw(m, image.Rect(0, h-8, w, h), image.Black, image.Point{}, draw.Src)
		x0 := (w - 8*len(note.Text)) / 2
		for i := 0; i < len(note.Text); i++ {
			glyphAt(m, note.Glyph(note.Text[i]), x0+i*8, h-8, pal.EGA(14), color.RGBA{A: 255})
		}
	}
	pg.img.WritePixels(m.Pix)
	return pg.img
}

// glyphAt draws glyph g at x, y: set bits fg, the rest bg.
func glyphAt(m *image.RGBA, g [8]byte, x, y int, fg, bg color.RGBA) {
	for r := 0; r < 8; r++ {
		for b := 0; b < 8; b++ {
			c := bg
			if g[r]&(0x80>>b) != 0 {
				c = fg
			}
			m.SetRGBA(x+b, y+r, c)
		}
	}
}

// drawPage shows the whole page over the whole window (fit), with the
// overlays at the window's own resolution; it returns where. A page not yet
// of the window's grid (the engine has the resize still to do) is shown
// at a whole scale in the middle meanwhile, so nothing stretches.
func (p *Presenter) drawPage(dst *gpu.Image, v engine.View, note Note) Rect {
	img := p.pageImage(v, note)
	dst.Fill(color.Black)
	b := dst.Bounds()
	w, h := pageSize(v)
	var r Rect
	if cols, rows := grid(dst); cols*8 == w && rows*8 == h {
		kx, ky := p.fit(dst, img, w, h)
		r = Rect{0, 0, kx * float64(w), ky * float64(h)}
	} else {
		k := math.Max(1, math.Floor(math.Min(float64(b.Dx())/float64(w), float64(b.Dy())/float64(h))))
		r = Rect{(float64(b.Dx()) - k*float64(w)) / 2, (float64(b.Dy()) - k*float64(h)) / 2, k * float64(w), k * float64(h)}
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(r.X, r.Y)
		dst.DrawImage(img, op)
	}
	p.drawOverlays(dst, v.Overlays, r, w, h)
	return r
}

// drawOverlays draws the overlays (journal scans) over the page of w by h
// pixels at r.
func (p *Presenter) drawOverlays(dst *gpu.Image, os []engine.Overlay, r Rect, w, h int) {
	pg := &p.pg
	for img, im := range pg.scans { // drop the copies of scans no longer shown
		keep := false
		for _, o := range os {
			keep = keep || o.Img == img
		}
		if !keep {
			im.im.Deallocate()
			delete(pg.scans, img)
		}
	}
	kx, ky := r.W/float64(w), r.H/float64(h)
	for _, o := range os {
		b := o.Part()
		tw, th := int(float64(o.At.Dx())*kx), int(float64(o.At.Dy())*ky) // its place in window pixels
		x, y := r.X+float64(o.At.Min.X)*kx, r.Y+float64(o.At.Min.Y)*ky
		// a picture smaller than its place (a portrait) is scaled up by whole
		// pixels when that nearly fills the place (crisp pixel art, on its
		// foot), else by Catmull-Rom to the exact size; a larger one (a journal
		// scan) shrinks bilinear on the GPU
		s := wholeScale(b.Dx(), b.Dy(), tw, th)
		sw, sh := 0, 0
		if s == 0 && tw >= b.Dx() && th >= b.Dy() {
			sw, sh = tw, th
		}
		sc, ok := pg.scans[o.Img]
		if !ok || sc.w != sw || sc.h != sh {
			if ok {
				sc.im.Deallocate()
			}
			if pg.scans == nil {
				pg.scans = map[image.Image]scan{}
			}
			src := o.Img
			if sw > 0 {
				m := image.NewRGBA(image.Rect(0, 0, sw, sh))
				xdraw.CatmullRom.Scale(m, m.Bounds(), o.Img, b, draw.Src, nil)
				src = m
			}
			sc = scan{gpu.NewImageFromImage(src), sw, sh}
			pg.scans[o.Img] = sc
		}
		im := sc.im
		op := &gpu.DrawImageOptions{Filter: gpu.FilterLinear}
		switch {
		case s > 0:
			im = im.SubImage(b).(*gpu.Image)
			op.Filter = gpu.FilterNearest
			op.GeoM.Scale(float64(s), float64(s))
			x += float64(tw-s*b.Dx()) / 2
			y += float64(th - s*b.Dy())
		case sw > 0:
			op.Filter = gpu.FilterNearest
		default:
			im = im.SubImage(b).(*gpu.Image) // drawn from its own top left
			op.GeoM.Scale(float64(tw)/float64(b.Dx()), float64(th)/float64(b.Dy()))
		}
		op.GeoM.Translate(x, y)
		dst.DrawImage(im, op)
	}
}

// samePage reports whether v's page and text are what pg.img shows. The
// engine publishes a View every tick, most of them with the page unchanged;
// a layer's picture is the same when it is the same pixels.
func samePage(v engine.View, pg *page) bool {
	if len(v.Page) != len(pg.page) || len(v.Text) != len(pg.text) {
		return false
	}
	for y := range v.Text {
		if !slices.Equal(v.Text[y], pg.text[y]) {
			return false
		}
	}
	for i, s := range v.Page {
		if s.At != pg.page[i].At || !sameImage(s.Img, pg.page[i].Img) {
			return false
		}
	}
	return true
}

func sameImage(a, b image.Image) bool {
	switch a := a.(type) {
	case *image.RGBA:
		b, ok := b.(*image.RGBA)
		return ok && a.Rect == b.Rect && a.Stride == b.Stride && len(a.Pix) == len(b.Pix) && (len(a.Pix) == 0 || &a.Pix[0] == &b.Pix[0])
	case *image.Uniform:
		b, ok := b.(*image.Uniform)
		return ok && a.C == b.C
	}
	return false
}
