package remaster

import (
	"image"
	"image/color"
	"image/draw"
	"math"

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

// page is the original's 320x200 screen drawn from the View's Page and Text.
type page struct {
	img   *gpu.Image
	px    *image.RGBA
	seq   uint64
	noted bool
	drawn bool
	scans map[image.Image]scan // the overlays on the GPU
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
	if pg.img == nil {
		pg.img, pg.px = gpu.NewImage(pal.W, pal.H), image.NewRGBA(image.Rect(0, 0, pal.W, pal.H))
	}
	if pg.drawn && v.Seq == pg.seq && !note.Show && !pg.noted {
		return pg.img
	}
	pg.drawn, pg.seq, pg.noted = true, v.Seq, note.Show
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
		draw.Draw(m, image.Rect(0, pal.H-8, pal.W, pal.H), image.Black, image.Point{}, draw.Src)
		x0 := (pal.W - 8*len(note.Text)) / 2
		for i := 0; i < len(note.Text); i++ {
			glyphAt(m, note.Glyph(note.Text[i]), x0+i*8, pal.H-8, pal.EGA(14), color.RGBA{A: 255})
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

// drawPage shows the whole page on dst, as large as fits at a whole
// scale, with the overlays at the window's own resolution; it returns where.
func (p *Presenter) drawPage(dst *gpu.Image, v engine.View, note Note) Rect {
	img := p.pageImage(v, note)
	dst.Fill(color.Black)
	b := dst.Bounds()
	k := math.Max(1, math.Floor(math.Min(float64(b.Dx())/pal.W, float64(b.Dy())/pal.H)))
	r := Rect{(float64(b.Dx()) - k*pal.W) / 2, (float64(b.Dy()) - k*pal.H) / 2, k * pal.W, k * pal.H}
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(r.X, r.Y)
	dst.DrawImage(img, op)
	p.drawOverlays(dst, v.Overlays, r)
	return r
}

// drawOverlays draws the overlays (journal scans) over the page at r.
func (p *Presenter) drawOverlays(dst *gpu.Image, os []engine.Overlay, r Rect) {
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
	k := r.W / pal.W
	for _, o := range os {
		b := o.Part()
		tw, th := o.At.Dx()*int(k), o.At.Dy()*int(k) // its place in window pixels
		x, y := r.X+float64(o.At.Min.X)*k, r.Y+float64(o.At.Min.Y)*k
		// a picture smaller than its place (a portrait) is scaled up by whole
		// pixels when that nearly fills the place (crisp pixel art, on its
		// foot), else by Catmull-Rom to the exact size; a larger one (a journal
		// scan) shrinks bilinear on the GPU
		s := min(tw/b.Dx(), th/b.Dy())
		whole := s >= 1 && s*b.Dx() >= tw*9/10 && s*b.Dy() >= th*9/10
		sw, sh := 0, 0
		if s >= 1 && !whole {
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
		case whole:
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
