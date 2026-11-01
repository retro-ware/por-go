package remaster

import (
	"image"
	"image/color"
	"image/draw"
	"strconv"
	"strings"

	"gpu"

	"por/engine"
	"por/pal"
	"por/present/input"
)

// The character sheet (View.Sheet): the page is the engine's; its View
// menu is the presenter's.
func (p *Presenter) sheetItems(v engine.View) []menuRow {
	var it []menuRow // the powers only: Tab is the inventory, Esc leaves
	for _, pw := range v.Sheet.Powers {
		it = append(it, menuRow{pw.Name, pw.Name[0], func() { p.send(pw.Kind, -1) }})
	}
	return it
}

// sheetUp: the sheet's menu is the page's question.
func sheetUp(v engine.View) bool { return v.Sheet != nil && askMenu(v) == nil && v.Inventory == nil }

// sheetKey takes key k on the sheet: the menu's keys, the member keys another member.
func (p *Presenter) sheetKey(k uint16, v engine.View) bool {
	items := p.sheetItems(v)
	if m, ok := memberKey(k, selectedIndex(v), len(v.Party), true); ok {
		p.send(engine.ActMember, m)
		return true
	}
	if actOf(k) == input.Minimap { // Tab, pad Y: the inventory (they toggle)
		p.send(engine.ActInventory, -1)
		return true
	}
	i, back, ok := menuNav(k, &p.sel, rowKeys(items))
	switch {
	case back:
		return false // ActClose (keyAct)
	case i >= 0:
		items[i].do()
	}
	return ok
}

// sheetCanvas is the sheet as the presenter draws it: at the inventory's
// scale and cells, the member on the left (name, classes, portrait, the
// HP and experience bars), the sheet's figures on the right, the powers
// on the bottom row (the toast in their place while it shows).
type sheetCanvas struct {
	img  *gpu.Image
	px   *image.RGBA
	w, h int
}

// drawSheet draws the sheet (sheetUp) over the whole window.
func (p *Presenter) drawSheet(dst *gpu.Image, note Note, v engine.View) {
	sp, i := v.SheetPage, selectedIndex(v)
	if i < 0 {
		return
	}
	m, sc := v.Party[i], &p.sheet
	p.setup(note, v)
	p.pageMenu = true
	b := dst.Bounds()
	k := max(1, min(b.Dx()/invW, b.Dy()/invH)) // the inventory's scale and cells
	cw, ch := max(invW, b.Dx()/k)/8, max(invH, b.Dy()/k)/8
	if w, h := cw*8, ch*8; gpu.Sized(&sc.img, w, h) {
		sc.w, sc.h, sc.px = w, h, image.NewRGBA(image.Rect(0, 0, w, h))
	}
	draw.Draw(sc.px, sc.px.Rect, image.Black, image.Point{}, draw.Src)
	text := func(s string, x, y int, c color.RGBA) { drawText(sc.px, note.Glyph, s, x, y, c) }
	ink := func(i int) color.RGBA { return col(i) }

	// the member: name, classes, portrait, then the bars under it
	lx, lw := 8, (leftCells-2)*8
	centre := func(s string, y int, c color.RGBA) { text(s, lx+(lw-8*len(s))/2, y, c) }
	centre(m.Name, 12, ink(sp.NameColour))
	y := 24
	for _, cl := range strings.Split(m.Class, "/") {
		centre(cl, y, green)
		y += 10
	}
	face := image.Rect(lx+(lw-portraitSize)/2, y+4, lx+(lw+portraitSize)/2, y+4+portraitSize)
	var png image.Image
	if f := p.portrait(m); f != nil {
		if m.Portrait != nil {
			png = m.Portrait
		} else {
			draw.Draw(sc.px, face, f, f.Bounds().Min, draw.Src)
		}
	}
	y = face.Max.Y + 8
	hpY, expY := y+10, y+36
	hpc := green
	if m.HP < m.MaxHP {
		hpc = col(14)
	}
	text("HP", lx, y, white)
	hp := strconv.Itoa(m.HP) + "/" + strconv.Itoa(m.MaxHP)
	text(hp, lx+lw-8*len(hp), y, hpc)
	if !sp.Creating {
		text("Exp", lx, y+26, white)
		e := strconv.Itoa(sp.Exp)
		if sp.Next > 0 {
			e += "/" + strconv.Itoa(sp.Next)
		}
		text(e, lx+lw-8*len(e), y+26, green)
		text("Level "+sp.Levels, lx, y+52, white)
	}

	// the figures: who, then the abilities beside the combat figures
	rx := leftCells * 8
	row := func(r int) int { return 12 + 12*r }
	text(sp.Sex+" "+sp.Race+", age "+strconv.Itoa(sp.Age), rx, row(0), white)
	text(sp.Alignment, rx, row(1), white)
	if sp.Status != "" {
		text("Status", rx, row(2), white)
		text(sp.Status, rx+8*8, row(2), green)
	}
	for j, l := range []string{"STR", "INT", "WIS", "DEX", "CON", "CHA"} {
		text(l, rx, row(4+j), white)
		v := strconv.Itoa(sp.Stats[j])
		if j == 0 {
			v += sp.Str00
		}
		text(v, rx+5*8, row(4+j), green)
	}
	cx := rx + 16*8
	for j, f := range [][2]string{{"AC", strconv.Itoa(sp.AC)}, {"THAC0", strconv.Itoa(sp.THAC0)}, {"Damage", sp.Damage}, {"Move", strconv.Itoa(sp.Move)}} {
		text(f[0], cx, row(4+j), white)
		text(f[1], cx+8*8, row(4+j), green)
	}
	if sp.Effects != "" {
		text("Effects", rx, row(11), white)
		for j, l := range engine.WrapWords(sp.Effects, cw-leftCells-2) {
			text(l, rx, row(12+j), green)
		}
	}

	// the powers on the bottom row; the toast takes it while it shows
	items := p.sheetItems(v)
	rowY := (ch - 1) * 8
	p.winRects = p.winRects[:0]
	var words []image.Rectangle
	if note.Show && note.Text != "" {
		noteLine(sc.px, note, rowY)
	} else {
		n := 0
		for _, it := range items {
			n += 8 * (len(it.text) + 1)
		}
		x := (sc.w - n + 8) / 2
		p.sel = min(max(p.sel, 0), len(items)-1)
		for j, it := range items {
			r := image.Rect(x-4, rowY-2, x+8*len(it.text)+4, rowY+10)
			if j == p.sel {
				draw.Draw(sc.px, r, image.NewUniform(pal.EGA(1)), image.Point{}, draw.Src)
			}
			text(it.text[:1], x, rowY, white)
			text(it.text[1:], x+8, rowY, green)
			words = append(words, r)
			x += 8 * (len(it.text) + 1)
		}
	}

	sc.img.WritePixels(sc.px.Pix)
	t := p.border.tiles
	window(sc.img, t, gpu.GeoM{}, leftCells, ch-1, nil)
	var g gpu.GeoM
	g.Translate(float64(leftCells-1)*8, 0)
	window(sc.img, t, g, cw-leftCells+1, ch-1, nil)
	fill := greenFill
	if m.HP*4 < m.MaxHP {
		fill = redFill
	}
	p.bar(sc.img, lx-5, hpY, lw+10, m.HP, m.MaxHP, fill)
	if !sp.Creating && m.NextExp > 0 {
		p.bar(sc.img, lx-5, expY, lw+10, m.Exp-m.LevelExp, m.NextExp-m.LevelExp, goldFill)
	}
	kx, ky := p.fit(dst, sc.img, sc.w, sc.h)
	if png != nil { // the PNG portrait at the window's resolution, as the cards
		op := &gpu.DrawImageOptions{}
		op.GeoM.Translate(float64(face.Min.X)*kx, float64(face.Min.Y)*ky)
		dst.DrawImage(p.hiFace(png, int(float64(portraitSize)*kx)), op)
	}
	for _, r := range words { // the clicks, on the window
		p.winRects = append(p.winRects, image.Rect(int(float64(r.Min.X)*kx), int(float64(r.Min.Y)*ky), int(float64(r.Max.X)*kx), int(float64(r.Max.Y)*ky)))
	}
}
