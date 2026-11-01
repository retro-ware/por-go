package remaster

import (
	"image/color"

	"gpu"

	"por/engine"
	"por/pal"
	"por/present/input"
)

// The character sheet (View.Sheet): the page is the engine's; its View
// menu is the presenter's.
func (p *Presenter) sheetItems(v engine.View) []menuRow {
	var it []menuRow
	it = append(it, menuRow{"Items", 'I', func() { p.send(engine.ActInventory, -1) }}) // also Tab
	for _, pw := range v.Sheet.Powers {
		it = append(it, menuRow{pw.Name, pw.Name[0], func() { p.send(pw.Kind, -1) }})
	}
	return append(it, menuRow{"Exit", 'E', func() { p.send(engine.ActClose, 0) }})
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

// menuRow draws a horizontal menu on the page's last row, as the
// inventory's: the words centred, their hotkeys white, the selected one on
// the text selection; their rectangles go to winRects (the clicks, hover).
func (p *Presenter) menuRow(dst *gpu.Image, words []string, keys []byte, sel int) {
	s := &p.scr
	cols, row := s.pw/8, s.ph/8-1
	n := 0
	for _, w := range words {
		n += len(w) + 1
	}
	x := max(0, (cols-n+1)/2)
	if gpu.Sized(&p.menuImg, cols*8, 8) {
		p.menuBuf = make([]byte, cols*8*8*4)
	}
	clear(p.menuBuf)
	set := func(x, y int, c color.RGBA) {
		if x < cols*8 {
			o := (y*cols*8 + x) * 4
			p.menuBuf[o], p.menuBuf[o+1], p.menuBuf[o+2], p.menuBuf[o+3] = c.R, c.G, c.B, 255
		}
	}
	for i, w := range words {
		t, hk := hotText(w, keys[i])
		if i == sel {
			for y := range 8 {
				for px := (x-1)*8 + 4; px < (x+len(t))*8+4; px++ {
					set(max(px, 0), y, pal.EGA(1))
				}
			}
		}
		for j := 0; j < len(t); j++ {
			c := pal.EGA(10)
			if j == hk {
				c = pal.EGA(15)
			}
			g := p.glyph(t[j])
			for y := range 8 {
				for b := range 8 {
					if g[y]&(0x80>>b) != 0 {
						set((x+j)*8+b, y, c)
					}
				}
			}
		}
		p.winRects = append(p.winRects, p.cellRect(engine.CellRect{X1: x, Y1: row, X2: x + len(t) - 1, Y2: row}))
		x += len(t) + 1
	}
	p.menuImg.WritePixels(p.menuBuf)
	at := p.cellRect(engine.CellRect{X1: 0, Y1: row, X2: cols - 1, Y2: row})
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(float64(at.Dx())/float64(cols*8), float64(at.Dy())/8)
	op.GeoM.Translate(float64(at.Min.X), float64(at.Min.Y))
	dst.DrawImage(p.menuImg, op)
}
