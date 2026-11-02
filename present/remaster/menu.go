package remaster

import (
	"bytes"
	"image"
	"image/color"
	"por/engine"
	"por/pal"
	"por/present/input"
	"slices"
	"strings"

	"gpu"
)

// menuRow is one line of every menu: its text, its hotkey (0 none) and what
// picking it does.
type menuRow struct {
	text string
	key  byte
	do   func()
}

// rowKeys are the rows' hotkeys: a row that can be picked and has no
// letter gets a digit (digitKeys).
func rowKeys(rows []menuRow) []byte {
	k := make([]byte, len(rows))
	for i, r := range rows {
		k[i] = r.key
	}
	return digitKeys(k, func(i int) bool { return rows[i].do != nil })
}

// digitKeys gives the rows i of keys with no hotkey and on(i) the digits
// 1-9 in order (those another row has already left out), then none.
func digitKeys(keys []byte, on func(int) bool) []byte {
	d := byte('1')
	for i := range keys {
		if keys[i] != 0 || !on(i) {
			continue
		}
		for d <= '9' && bytes.IndexByte(keys, d) >= 0 {
			d++
		}
		if d > '9' {
			break
		}
		keys[i], d = d, d+1
	}
	return keys
}

// menuNav is the one key handler of every menu over rows with hotkeys keys:
// Up/Down (wrapping), PgUp/PgDn, Home/End move *sel; Enter, Space and pad A
// pick it, a hotkey (either case) picks its row; Esc is back and never
// picks. It returns the row picked (-1 none), back, and whether k was taken.
func menuNav(k uint16, sel *int, keys []byte) (pick int, back, ok bool) {
	n := len(keys)
	a := actOf(k)
	if n == 0 {
		return -1, a == input.Esc, a == input.Esc
	}
	s := min(max(*sel, 0), n-1)
	switch {
	case a == input.Up:
		s = (s + n - 1) % n
	case a == input.Down:
		s = (s + 1) % n
	case a == input.PgUp:
		s = max(0, s-10)
	case a == input.PgDn:
		s = min(n-1, s+10)
	case a == input.Home:
		s = 0
	case a == input.End:
		s = n - 1
	case a == input.Enter || a == input.Space || a == input.PadA:
		*sel = s
		return s, false, true
	case a == input.Esc:
		return -1, true, true
	case k > ' ' && k < 128:
		h := byte(k)
		if h >= 'a' && h <= 'z' {
			h -= 'a' - 'A'
		}
		if i := bytes.IndexByte(keys, h); i >= 0 {
			*sel = i
			return i, false, true
		}
		return -1, false, false
	default:
		return -1, false, false
	}
	*sel = s
	return -1, false, true
}

// hotText is row s as drawn with hotkey key: the column of its white letter
// (-1 none); a key not in the text, and every digit (a row's digit in its
// text is a price, not the key), is shown in front, as "[E] Camp".
func hotText(s string, key byte) (string, int) {
	if key == 0 {
		return s, -1
	}
	if i := strings.IndexByte(strings.ToUpper(s), key); i >= 0 && (key < '0' || key > '9') {
		return s, i
	}
	return "[" + string(key) + "] " + s, 1
}

// drawMenu draws a vertical menu in the original's look: a box with the
// text pane's red border, the items in the game's font, green on black with
// their hotkeys white, the rows off[i] (nothing to pick) grey (EGA 8), the
// selected one on the text selection (a blue EGA 1 ground). It goes at scale k,
// centred on cx, its top at top.
func (p *Presenter) drawMenu(dst *gpu.Image, glyph func(byte) [8]byte, title string, items []string, keys []byte, off []bool, sel int, k float64, cx, top int) {
	p.drawMenuIcons(dst, glyph, title, items, keys, off, nil, sel, k, cx, top)
}

// rowsOff are the rows with nothing to pick (do nil).
func rowsOff(rows []menuRow) []bool {
	off := make([]bool, len(rows))
	for i, r := range rows {
		off[i] = r.do == nil
	}
	return off
}

// iconRow is the cells an item row with its 20x20 atlas icon takes: as
// tall as wide, the text in the middle line, after the icon.
const iconRow = 3

// drawMenuIcons is drawMenu with an icon before each row of icons that is
// not nil (the inventory's row: the icon, then the text): unscaled, or
// shrunk with its aspect kept to the row's height.
func (p *Presenter) drawMenuIcons(dst *gpu.Image, glyph func(byte) [8]byte, title string, items []string, keys []byte, off []bool, icons []image.Image, sel int, k float64, cx, top int) {
	items = slices.Clone(items)
	grey := make([]bool, len(items))
	copy(grey, off)
	hk := make([]int, len(items)) // each row's white letter
	for i := range items {
		var key byte
		if i < len(keys) {
			key = keys[i]
		}
		items[i], hk[i] = hotText(items[i], key)
	}
	title, foot, _ := strings.Cut(title, string(rune(inkFoot)))
	rows, hot, icon := items, hk, make([]image.Image, len(items))
	copy(icon, icons)
	if title != "" { // a long question on two lines, so the box stays narrow
		t := append(titleLines(title), "")
		rows, hot, icon = append(t, items...), append(slices.Repeat([]int{-1}, len(t)), hk...), append(make([]image.Image, len(t)), icon...)
		grey = append(make([]bool, len(t)), grey...)
		if sel >= 0 {
			sel += len(t)
		}
	}
	w, ch := 0, 2                // cells: a border and a blank column each side
	at := make([]int, len(rows)) // each row's first line, in cells
	rh := func(r int) int {
		if icon[r] != nil {
			return iconRow
		}
		return 1
	}
	for r, s := range rows {
		x := visLen(s)
		if icon[r] != nil {
			x += iconRow
		}
		w, at[r], ch = max(w, x), ch-2, ch+rh(r)
	}
	w = max(w, len(foot)) // the footer sits on the bottom border, inside its corners
	pad := 2              // a border and a blank column each side
	if p.menuTight {
		pad = 1 // the border only (the creation's drop-downs, on the sheet)
	}
	cw := w + 2*pad
	if gpu.Sized(&p.menuImg, cw*8, ch*8) {
		p.menuBuf = make([]byte, cw*8*ch*8*4)
	}
	rgb := pal.EGA
	green, white, dim := rgb(10), rgb(15), rgb(8)
	if p.logGrey {
		green = rgb(7) // the battle Log's light grey
	}
	clear(p.menuBuf)
	for i := 3; i < len(p.menuBuf); i += 4 {
		p.menuBuf[i] = 255
	}
	set := func(x, y int, c color.RGBA) {
		o := (y*cw*8 + x) * 4
		p.menuBuf[o], p.menuBuf[o+1], p.menuBuf[o+2] = c.R, c.G, c.B
	}
	ink := green // a colour span goes on over a wrapped line
	for r, s := range rows {
		y0, x0 := at[r]*8+8, pad
		if r == sel {
			for y := y0; y < y0+rh(r)*8; y++ { // the text selection: a blue ground
				for x := 8; x < (cw-1)*8; x++ {
					set(x, y, pal.EGA(1))
				}
			}
		}
		if ic := icon[r]; ic != nil { // the icon, then the text in the middle line
			b, h := ic.Bounds(), iconRow*8
			f := min(1, float64(h)/float64(b.Dx()), float64(h)/float64(b.Dy()))
			w, ih := int(f*float64(b.Dx())), int(f*float64(b.Dy()))
			for y := range ih {
				for x := range w {
					if c := color.RGBAModel.Convert(ic.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/ih)).(color.RGBA); c.A > 0x80 {
						set(16+(h-w)/2+x, y0+(h-ih)/2+y, c)
					}
				}
			}
			y0, x0 = y0+8, 2+iconRow
		}
		hk := hot[r] // the hotkey: white, as on the original's bar
		col := 0
		for i := 0; i < len(s); i++ {
			if s[i] == inkGold || s[i] == inkRed || s[i] == inkHead { // a span in colour, to the next mark
				switch {
				case ink != green:
					ink = green
				case s[i] == inkGold:
					ink = rgb(14)
				case s[i] == inkHead:
					ink = rgb(11)
				default:
					ink = rgb(12)
				}
				continue
			}
			g := glyph(s[i])
			c := ink
			switch {
			case grey[r]:
				c = dim
			case i == hk:
				c = white
			}
			for y := 0; y < 8; y++ {
				for b := 0; b < 8; b++ {
					if g[y]&(0x80>>b) != 0 {
						set((col+x0)*8+b, y0+y, c)
					}
				}
			}
			col++
		}
	}
	p.menuImg.WritePixels(p.menuBuf)

	tiles := p.border.tiles
	if p.menuGrey {
		tiles = p.border.grey
	}
	window(p.menuImg, tiles, gpu.GeoM{}, cw, ch, nil)
	if foot != "" { // gold on black, right on the bottom border
		fw := len(foot) * 8
		if gpu.Sized(&p.footImg, fw, 8) {
			p.footBuf = make([]byte, fw*8*4)
		}
		for i := 0; i < len(p.footBuf); i += 4 {
			p.footBuf[i], p.footBuf[i+1], p.footBuf[i+2], p.footBuf[i+3] = 0, 0, 0, 255
		}
		gold := rgb(14)
		for n := 0; n < len(foot); n++ {
			g := glyph(foot[n])
			for y := 0; y < 8; y++ {
				for b := 0; b < 8; b++ {
					if g[y]&(0x80>>b) != 0 {
						o := (y*fw + n*8 + b) * 4
						p.footBuf[o], p.footBuf[o+1], p.footBuf[o+2] = gold.R, gold.G, gold.B
					}
				}
			}
		}
		p.footImg.WritePixels(p.footBuf)
		fo := &gpu.DrawImageOptions{}
		fo.GeoM.Translate(float64((cw-1)*8-fw), float64((ch-1)*8))
		p.menuImg.DrawImage(p.footImg, fo)
	}

	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(cx)-k*float64(cw*8)/2, float64(top))
	dst.DrawImage(p.menuImg, op)
	p.menuRects = p.menuRects[:0]
	for r := len(rows) - len(items); r < len(rows); r++ {
		x0, y0 := float64(cx)-k*float64(cw*8)/2, float64(top)+k*float64((at[r]+1)*8)
		p.menuRects = append(p.menuRects, image.Rect(int(x0+k*8), int(y0), int(x0+k*float64((cw-1)*8)), int(y0+k*float64(rh(r)*8))))
	}
}

// Colour marks in a question's title: the text from one to the next is
// gold (inkGold) or red (inkRed).
const (
	inkGold = '\x01'
	inkRed  = '\x02'
	inkFoot = '\x03' // the rest of the title goes on the bottom border, in gold
	inkHead = '\x04' // cyan: a submenu's heading
)

// visLen is s's length in cells, its colour marks left out.
func visLen(s string) int {
	return len(s) - strings.Count(s, string(rune(inkGold))) - strings.Count(s, string(rune(inkRed))) - strings.Count(s, string(rune(inkHead)))
}

// titleLines are a menu title's lines: its own (\n) and blank ones kept,
// each wrapped at 24 cells, so the box stays narrow.
func titleLines(title string) []string {
	var out []string
	for _, l := range strings.Split(title, "\n") {
		if w := engine.WrapWords(l, 24); len(w) > 0 {
			out = append(out, w...)
		} else {
			out = append(out, "")
		}
	}
	return out
}
