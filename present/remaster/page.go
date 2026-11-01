package remaster

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"
	"strconv"
	"strings"

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
	Font  func(ch byte, upcase bool) [8]byte // a pane run's glyphs (engine.PaneRun)
}

// page is the engine's screen (pageSize: 320x200 at least) drawn from the
// View's Page and Text.
type page struct {
	img   *gpu.Image
	px    *image.RGBA
	seq   uint64
	noted bool
	drawn bool
	page  []engine.Picture     // what img shows: the View's page...
	text  [][]engine.PaneRun   // ...and text
	overs []overlay            // the screens' portraits over the page
	scans map[image.Image]scan // the overlays on the GPU
	fit   *gpu.Image           // fit's whole-scale copy
	pane  *gpu.Image           // the text pane (paneImage)
	pic   *gpu.Image           // the picture window (pictureBox)
	toast *gpu.Image           // the note's line over the 3D view
}

// pageSize is v's page in pixels: its text cells, 8 by 8 (the original's
// 320x200 for a View without any).
func pageSize(v engine.View) (w, h int) {
	if v.Cells.X == 0 || v.Cells.Y == 0 {
		return pal.W, pal.H
	}
	return v.Cells.X * 8, v.Cells.Y * 8
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
	if pg.drawn && !note.Show && !pg.noted && (v.Seq == pg.seq || !modal(v) && !ownPage(v) && samePage(v, pg)) { // the View changed, the page not: no redraw, no upload
		pg.seq = v.Seq
		return pg.img
	}
	pg.drawn, pg.seq, pg.noted = true, v.Seq, note.Show
	pg.page, pg.text = append(pg.page[:0], v.Pictures...), v.Lines
	m := pg.px
	c := screenPage(v)
	c.draw(m, v, note)
	pg.overs = c.overs
	drawModal(m, v, note)
	if note.Show {
		noteLine(m, note, h-8)
	}
	pg.img.WritePixels(m.Pix)
	return pg.img
}

// ownPage: v shows a screen the presenter lays out from its data
// (screenPage), drawn anew with each View.
func ownPage(v engine.View) bool {
	return v.SheetPage != nil || v.Picker != nil || v.Slots != nil || v.IconEditor != nil
}

// modal: v shows a screen over the whole page (help, the journal, the
// full map, the overview), drawn from its data over the page under it.
func modal(v engine.View) bool { return v.Pages != nil || v.FullMap != nil || v.Overview }

// drawModal draws v's screen over the whole page (modal) into m as the
// game draws such a page: a box round it, text in the game's font.
func drawModal(m *image.RGBA, v engine.View, note Note) {
	if !modal(v) {
		return
	}
	cols, rows := m.Rect.Dx()/8, m.Rect.Dy()/8
	run := func(r engine.PaneRun, y int) {
		for i := 0; i < len(r.Text); i++ {
			glyphAt(m, note.Font(r.Text[i], r.Upcase), (r.X+i)*8, y*8, r.Fg, r.Bg)
		}
	}
	str := func(x, y, c int, s string) { // DisplayString
		run(engine.PaneRun{X: x, Text: s[:max(0, min(len(s), cols-x))], Fg: col(c), Bg: col(0), Upcase: true}, y)
	}
	fill := func(r image.Rectangle, c color.RGBA) { draw.Draw(m, r, image.NewUniform(c), image.Point{}, draw.Src) }
	box := func(x2, y2 int) { // DrawBoxTitled(1, 1, x2, y2): the inside cleared, the border's cells round it
		fill(image.Rect(8, 8, x2*8+8, y2*8+8), col(0))
		tile := func(i, x, y int) {
			if t := v.Border[i]; t != nil {
				draw.Draw(m, image.Rect(x*8, y*8, x*8+8, y*8+8), t, t.Bounds().Min, draw.Over)
			}
		}
		for _, c := range [][2]int{{0, 0}, {x2 + 1, 0}, {0, y2 + 1}, {x2 + 1, y2 + 1}} {
			tile(0, c[0], c[1])
		}
		for x := 1; x <= x2; x++ {
			tile(2, x, 0)
			tile(2, x, y2+1)
		}
		for y := 1; y <= y2; y++ {
			tile(1, 0, y)
			tile(1, x2+1, y)
		}
	}
	prompt := func(s string, c int) { // the prompt row cleared, s on it
		str(0, rows-1, 0, strings.Repeat(" ", cols))
		str(0, rows-1, c, s)
	}
	switch {
	case v.Pages != nil:
		pg := v.Pages
		box(cols-2, rows-2)
		run(pg.Title, 1)
		for _, s := range pg.Scans { // the paper; the scan goes over the window (drawOverlays)
			fill(s.At, s.Paper)
		}
		for i, l := range pg.Lines {
			for _, r := range l {
				run(r, pg.Top+i)
			}
		}
		run(pg.Hint, rows-2)
	case v.FullMap != nil:
		fm := v.FullMap
		lx := fm.LegendAt.X
		fill(m.Rect, col(0))
		str(0, 0, 15, fm.Name[:min(len(fm.Name), lx-1)])
		str(lx, 0, 15, "Labels")
		for i, l := range fm.Legend {
			str(lx, fm.LegendAt.Y+i, l.Colour, string(l.Letter))
			str(lx+2, fm.LegendAt.Y+i, 10, l.Text[:min(len(l.Text), 0x28-lx-2)])
		}
		str(1, rows-2, 11, fm.Info[:min(len(fm.Info), 0x26)])
		draw.Draw(m, fm.Map.Bounds().Add(fm.At), fm.Map, fm.Map.Bounds().Min, draw.Over)
		n, c := fm.Cell, fm.At.Add(fm.Cursor.Mul(fm.Cell))
		for _, r := range []image.Rectangle{image.Rect(0, 0, n, 1), image.Rect(0, n-1, n, n), image.Rect(0, 0, 1, n), image.Rect(n-1, 0, n, n)} {
			fill(r.Add(c), col(14))
		}
		if fm.Label != nil {
			prompt("Label: ", 13)
			str(7, rows-1, 15, *fm.Label+"_")
		}
	case v.Overview:
		box(cols-2, rows-3)
		const title = "Party"
		str((cols-1-len(title))/2, 1, 15, title)
		y := 3
		str(2, y, 15, "Name")
		str(18, y, 15, "HP")
		str(26, y, 15, "AC")
		str(30, y, 15, "Status")
		y += 2
		for _, c := range v.Party {
			if y > 0x14 {
				break
			}
			colour := 11
			if c.Selected {
				colour = 15
			}
			str(2, y, colour, c.Name[:min(len(c.Name), 15)])
			hpc := 10
			switch {
			case c.HP <= 0:
				hpc = 12
			case c.HP*2 < c.MaxHP:
				hpc = 14
			}
			str(18, y, hpc, strconv.Itoa(c.HP)+"/"+strconv.Itoa(c.MaxHP))
			str(26, y, 10, strconv.Itoa(c.AC))
			str(30, y, 10, c.Status)
			w := 0 // the hit point bar under the name, 32 steps
			if c.MaxHP > 0 && c.HP > 0 {
				w = min((c.HP*0x20+c.MaxHP-1)/c.MaxHP, 0x20)
			}
			px, py := 2*8, (y+1)*8+2
			fill(image.Rect(px, py, px+0x20*4, py+3), col(8))
			fill(image.Rect(px, py, px+w*4, py+3), col(hpc))
			y += 2
		}
		prompt("press any key to continue", 15)
	}
}

// drawPictures draws pics on black into m, whose top left is page pixel
// at: a fill as its colour, a picture over what is under it.
func drawPictures(m *image.RGBA, pics []engine.Picture, at image.Point) {
	draw.Draw(m, m.Bounds(), image.Black, image.Point{}, draw.Src)
	for _, s := range pics {
		r := s.At.Sub(at).Add(m.Rect.Min)
		if s.Fill {
			draw.Draw(m, r, image.NewUniform(s.Colour), image.Point{}, draw.Src)
			continue
		}
		draw.Draw(m, r, s.Img, s.Img.Bounds().Min, draw.Over)
	}
}

// pictureBox draws the pictures in page rectangle r (the picture window)
// into *img.
func pictureBox(img **gpu.Image, pics []engine.Picture, r image.Rectangle) *gpu.Image {
	m := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	drawPictures(m, pics, r.Min)
	return upload(img, m)
}

// noteLine draws the note yellow, centred on a black line at y of m (the
// bottom line in place of the prompt, as the inventory's menu row).
func noteLine(m *image.RGBA, note Note, y int) {
	w := m.Rect.Dx()
	draw.Draw(m, image.Rect(0, y, w, y+8), image.Black, image.Point{}, draw.Src)
	x0 := (w - 8*len(note.Text)) / 2
	for i := 0; i < len(note.Text); i++ {
		glyphAt(m, note.Glyph(note.Text[i]), x0+i*8, y, pal.EGA(14), color.RGBA{A: 255})
	}
}

// paneImage draws the text pane's runs on black in *img (made or remade
// at the pane's size): what drawBox shows inside the pane's border.
func paneImage(img **gpu.Image, pn *engine.Pane, note Note) *gpu.Image {
	r := engine.TextPane.Pixels()
	m := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(m, m.Rect, image.Black, image.Point{}, draw.Src)
	for y, l := range pn.Lines {
		for _, run := range l {
			for i := 0; i < len(run.Text); i++ {
				glyphAt(m, note.Font(run.Text[i], run.Upcase), (run.X+i)*8, y*8, run.Fg, run.Bg)
			}
		}
	}
	return upload(img, m)
}

// upload writes m to *img, made or remade at m's size.
func upload(img **gpu.Image, m *image.RGBA) *gpu.Image {
	if *img == nil || (*img).Bounds().Size() != m.Rect.Size() {
		if *img != nil {
			(*img).Deallocate()
		}
		*img = gpu.NewImage(m.Rect.Dx(), m.Rect.Dy())
	}
	(*img).WritePixels(m.Pix)
	return *img
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
	os := append([]overlay(nil), p.pg.overs...)
	if v.Pages != nil {
		for _, s := range v.Pages.Scans {
			os = append(os, overlay{Img: s.Img, Src: s.Src, At: s.At})
		}
	}
	p.drawOverlays(dst, os, r, w, h)
	return r
}

// drawOverlays draws the overlays (journal scans) over the page of w by h
// pixels at r.
func (p *Presenter) drawOverlays(dst *gpu.Image, os []overlay, r Rect, w, h int) {
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
	if len(v.Pictures) != len(pg.page) || len(v.Lines) != len(pg.text) {
		return false
	}
	for y := range v.Lines {
		if !slices.Equal(v.Lines[y], pg.text[y]) {
			return false
		}
	}
	for i, s := range v.Pictures {
		if o := pg.page[i]; s.At != o.At || s.Fill != o.Fill || s.Colour != o.Colour || !sameImage(s.Img, o.Img) {
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
	case nil:
		return b == nil
	}
	return false
}
