package remaster

import (
	"image"
	"image/color"
	"strings"

	"gpu"

	"por/ega"
	"por/engine"
)

// drawMenu draws a vertical menu in the original's look: a box with the
// text pane's red border, the items in the game's font, green on black with
// their hotkeys white, the selected one on blue. It goes at scale k, centred on cx, its top at top.
func (p *Presenter) drawMenu(dst, frame *gpu.Image, glyph func(byte) [8]byte, title string, items []string, keys []byte, sel int, k float64, cx, top int) {
	rows, hot := items, keys
	if title != "" {
		rows, hot = append([]string{title, ""}, items...), append([]byte{0, 0}, keys...)
		sel += 2
	}
	w := 0
	for _, r := range rows {
		w = max(w, len(r))
	}
	cw, ch := w+4, len(rows)+2 // cells: a border and a blank column each side
	if p.menuImg == nil || p.menuImg.Bounds().Dx() != cw*8 || p.menuImg.Bounds().Dy() != ch*8 {
		if p.menuImg != nil {
			p.menuImg.Deallocate()
		}
		p.menuImg, p.menuBuf = gpu.NewImage(cw*8, ch*8), make([]byte, cw*8*ch*8*4)
	}
	rgb := func(i int) color.RGBA { c := ega.Palette[i]; return color.RGBA{c[0], c[1], c[2], 255} }
	green, blue, white := rgb(10), rgb(1), rgb(15)
	clear(p.menuBuf)
	for i := 3; i < len(p.menuBuf); i += 4 {
		p.menuBuf[i] = 255
	}
	set := func(x, y int, c color.RGBA) {
		o := (y*cw*8 + x) * 4
		p.menuBuf[o], p.menuBuf[o+1], p.menuBuf[o+2] = c.R, c.G, c.B
	}
	for r, s := range rows {
		y0 := (r + 1) * 8
		if r == sel {
			for y := 0; y < 8; y++ {
				for x := 8; x < (cw-1)*8; x++ {
					set(x, y0+y, blue)
				}
			}
		}
		hk := -1 // the hotkey: white, as on the original's bar
		if r < len(hot) && hot[r] != 0 {
			hk = strings.IndexByte(strings.ToUpper(s), hot[r])
		}
		for i := 0; i < len(s); i++ {
			g := glyph(s[i])
			c := green
			if i == hk {
				c = white
			}
			for y := 0; y < 8; y++ {
				for b := 0; b < 8; b++ {
					if g[y]&(0x80>>b) != 0 {
						set((i+2)*8+b, y0+y, c)
					}
				}
			}
		}
	}
	p.menuImg.WritePixels(p.menuBuf)

	drawBorder(p.menuImg, frame, engine.TextPane.Framed(), gpu.GeoM{}, cw, ch)

	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(cx)-k*float64(cw*8)/2, float64(top))
	dst.DrawImage(p.menuImg, op)
	p.menuRects = p.menuRects[:0]
	for r := len(rows) - len(items); r < len(rows); r++ {
		x0, y0 := float64(cx)-k*float64(cw*8)/2, float64(top)+k*float64((r+1)*8)
		p.menuRects = append(p.menuRects, image.Rect(int(x0+k*8), int(y0), int(x0+k*float64((cw-1)*8)), int(y0+k*8)))
	}
}
