package remaster

import (
	"image"
	"image/draw"

	"gpu"

	"por/engine"
	"por/pal"
)

// drawBestiary draws camp's bestiary over the whole window, at the
// inventory's scale and cells: the list on the left (the sort row, the
// creatures, Exit), the creature under the highlight on the right (its
// combat icon, crisp, on grey, every kind at the scale that makes the
// widest fill the pane; the picture window's when first met; its name,
// danger and description).
func (p *Presenter) drawBestiary(dst *gpu.Image, note Note, v engine.View) {
	sc, c := &p.beastPage, &p.camp
	b := dst.Bounds()
	k := max(1, min(b.Dx()/invW, b.Dy()/invH))
	cw, ch := max(invW, b.Dx()/k)/8, max(invH, b.Dy()/k)/8
	if w, h := cw*8, ch*8; gpu.Sized(&sc.img, w, h) {
		sc.w, sc.h, sc.px = w, h, image.NewRGBA(image.Rect(0, 0, w, h))
	}
	draw.Draw(sc.px, sc.px.Rect, image.Black, image.Point{}, draw.Src)
	text := func(s string, x, y int, col int) { drawText(sc.px, note.Glyph, s, x, y, pal.EGA(col)) }

	// the list: a row every 12 pixels, scrolling with the highlight
	_, items := p.campItems(v)
	beasts := p.beasts(v)
	c.sel = min(max(c.sel, 0), len(items)-1)
	if i := c.sel - 1; i >= 0 && i < len(beasts) {
		c.beast = beasts[i].Name
	}
	const rowH = 12
	lw := (leftCells - 2) * 8
	fit := max(1, (ch*8-24)/rowH)
	c.top = min(max(c.top, c.sel-fit+1), c.sel, max(0, len(items)-fit))
	var rows []image.Rectangle
	for i := c.top; i < min(len(items), c.top+fit); i++ {
		y := 10 + (i-c.top)*rowH
		r := image.Rect(6, y-2, 8+lw+2, y+10)
		if i == c.sel {
			draw.Draw(sc.px, r, image.NewUniform(pal.EGA(1)), image.Point{}, draw.Src)
		}
		col := inkGreen
		if i == 0 || i == len(items)-1 { // Sort and Exit
			col = inkWhite
		}
		text(upTo(items[i].text, lw/8), 8, y, col)
		rows = append(rows, r)
	}

	// the creature
	rx, rw := leftCells*8, (cw-leftCells-1)*8
	var bt *engine.Beast
	for i := range beasts {
		if beasts[i].Name == c.beast {
			bt = &beasts[i]
		}
	}
	c.beastPane = image.Rectangle{}
	if bt != nil { // the entry on a page of its own, shown from c.beastY down (PgUp/PgDn, the pad's shoulders, the wheel over it)
		if c.beastAt != bt.Name {
			c.beastAt, c.beastY = bt.Name, 0
		}
		lines := engine.WrapWords(bt.Text, rw/8-1)
		wide := 1
		for _, b := range beasts {
			if b.Icon != nil {
				wide = max(wide, b.Icon.Bounds().Dx())
			}
		}
		scale := func(im image.Image) float64 { return float64(rw-16) / float64(im.Bounds().Dx()) }
		tall := 36 + 10*len(lines) + 8
		if bt.Icon != nil {
			tall += int(float64(bt.Icon.Bounds().Dy())*float64(rw-16)/float64(wide)) + 10
		}
		if bt.Pic != nil {
			tall += int(float64(bt.Pic.Bounds().Dy())*scale(bt.Pic)) + 10
		}
		ent := image.NewRGBA(image.Rect(0, 0, rw, tall))
		draw.Draw(ent, ent.Rect, image.Black, image.Point{}, draw.Src) // opaque: what is not drawn stays black, not a hole
		etext := func(s string, x, y int, col int) { drawText(ent, note.Glyph, s, x, y, pal.EGA(col)) }
		etext(bt.Name, (rw-8*len(bt.Name))/2, 10, inkWhite)
		d := "Danger " + roman[bt.Danger]
		etext(d, (rw-8*len(d))/2, 22, inkYellow)
		y := 36
		blit := func(im image.Image, s float64, ground bool) { // nearest neighbour, centred, at y
			ib := im.Bounds()
			w, h := int(float64(ib.Dx())*s), int(float64(ib.Dy())*s)
			at := image.Rect((rw-w)/2, y, (rw+w)/2, y+h)
			if ground {
				draw.Draw(ent, at, image.NewUniform(pal.EGA(8)), image.Point{}, draw.Src) // on grey: its black parts show
			}
			for py := range h {
				for px := range w {
					if col := im.At(ib.Min.X+int(float64(px)/s), ib.Min.Y+int(float64(py)/s)); alpha(col) {
						ent.Set(at.Min.X+px, at.Min.Y+py, col)
					}
				}
			}
			y += h + 10
		}
		if im := bt.Icon; im != nil { // every kind at one scale: the widest icon met fills the pane's width
			blit(im, float64(rw-16)/float64(wide), true)
		}
		if im := bt.Pic; im != nil { // the picture window's, the pane's width
			blit(im, scale(im), false)
		}
		for _, l := range lines {
			etext(l, 4, y, inkGreen)
			y += 10
		}
		view := (ch - 2) * 8 // inside the frame: the entry never scrolls under it
		c.beastMax = max(0, tall-view)
		c.beastY = min(max(c.beastY, 0), c.beastMax)
		c.beastPane = image.Rect(rx, 8, rx+rw, 8+view)
		draw.Draw(sc.px, c.beastPane, ent, image.Pt(0, c.beastY), draw.Src)
	}
	if note.Show && note.Text != "" {
		noteLine(sc.px, note, (ch-1)*8)
	}

	sc.img.WritePixels(sc.px.Pix)
	t := p.border.tiles
	window(sc.img, t, gpu.GeoM{}, leftCells, ch, nil)
	var g gpu.GeoM
	g.Translate(float64(leftCells-1)*8, 0)
	window(sc.img, t, g, cw-leftCells+1, ch, nil)
	paneName(sc.img, note.Glyph, "Bestiary")
	kx, ky := p.fit(dst, sc.img, sc.w, sc.h)
	p.menuRects = p.menuRects[:0] // the rows on the window: hover and clicks as on every camp list
	c.beastPane = image.Rect(int(float64(c.beastPane.Min.X)*kx), int(float64(c.beastPane.Min.Y)*ky), int(float64(c.beastPane.Max.X)*kx), int(float64(c.beastPane.Max.Y)*ky))
	for _, r := range rows {
		p.menuRects = append(p.menuRects, image.Rect(int(float64(r.Min.X)*kx), int(float64(r.Min.Y)*ky), int(float64(r.Max.X)*kx), int(float64(r.Max.Y)*ky)))
	}
	p.campCtx = p.campCtx[:0]
}

// alpha: c is not see-through.
func alpha(c interface{ RGBA() (r, g, b, a uint32) }) bool {
	_, _, _, a := c.RGBA()
	return a > 0x8000
}
