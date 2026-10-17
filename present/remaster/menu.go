package remaster

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"por/ega"
	"por/engine"
)

// drawMenu draws a vertical menu in the original's look: a box with the
// text pane's red border, the items in the game's font, green on black, the
// selected one on blue. It goes at scale k, centred on cx, its top at top.
func (p *Presenter) drawMenu(dst, frame *ebiten.Image, glyph func(byte) [8]byte, title string, items []string, sel int, k float64, cx, top int) {
	rows := items
	if title != "" {
		rows = append([]string{title, ""}, items...)
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
		p.menuImg, p.menuBuf = ebiten.NewImage(cw*8, ch*8), make([]byte, cw*8*ch*8*4)
	}
	rgb := func(i int) color.RGBA { c := ega.Palette[i]; return color.RGBA{c[0], c[1], c[2], 255} }
	green, blue := rgb(10), rgb(1)
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
		for i := 0; i < len(s); i++ {
			g := glyph(s[i])
			for y := 0; y < 8; y++ {
				for b := 0; b < 8; b++ {
					if g[y]&(0x80>>b) != 0 {
						set((i+2)*8+b, y0+y, green)
					}
				}
			}
		}
	}
	p.menuImg.WritePixels(p.menuBuf)

	// the border: the text pane's own cells, corners and edges
	f := engine.TextPane.Framed()
	cell := func(cx, cy, x, y int) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x*8), float64(y*8))
		p.menuImg.DrawImage(frame.SubImage(image.Rect(cx*8, cy*8, cx*8+8, cy*8+8)).(*ebiten.Image), op)
	}
	for x := 1; x < cw-1; x++ {
		cell(f.X1+1, f.Y1, x, 0)
		cell(f.X1+1, f.Y2, x, ch-1)
	}
	for y := 1; y < ch-1; y++ {
		cell(f.X1, f.Y1+1, 0, y)
		cell(f.X2, f.Y1+1, cw-1, y)
	}
	cell(f.X1, f.Y1, 0, 0)
	cell(f.X2, f.Y1, cw-1, 0)
	cell(f.X1, f.Y2, 0, ch-1)
	cell(f.X2, f.Y2, cw-1, ch-1)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(cx)-k*float64(cw*8)/2, float64(top))
	dst.DrawImage(p.menuImg, op)
}
