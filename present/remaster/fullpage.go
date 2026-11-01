package remaster

import (
	"image"
	"image/color"
	"image/draw"

	"gpu"

	"por/pal"
)

// fullPage draws a screen of its own over the whole window, as the
// bestiary and the inventory: a canvas of whole cells at the inventory's
// scale, body's text and pictures on black, one window round it with its
// title on the top border, the toast on the bottom row. body gets the
// canvas and its size in cells; fullPage returns the window pixels a
// canvas pixel takes, across and down.
func (p *Presenter) fullPage(dst *gpu.Image, sc *sheetCanvas, note Note, title string, body func(m *image.RGBA, cw, ch int)) (kx, ky float64) {
	b := dst.Bounds()
	k := max(1, min(b.Dx()/invW, b.Dy()/invH))
	cw, ch := max(invW, b.Dx()/k)/8, max(invH, b.Dy()/k)/8
	if w, h := cw*8, ch*8; gpu.Sized(&sc.img, w, h) {
		sc.w, sc.h, sc.px = w, h, image.NewRGBA(image.Rect(0, 0, w, h))
	}
	draw.Draw(sc.px, sc.px.Rect, image.Black, image.Point{}, draw.Src)
	body(sc.px, cw, ch)
	if note.Show && note.Text != "" {
		noteLine(sc.px, note, (ch-1)*8)
	}
	sc.img.WritePixels(sc.px.Pix)
	window(sc.img, p.border.tiles, gpu.GeoM{}, cw, ch, nil)
	if title != "" {
		onBorder(sc.img, note.Glyph, (cw*8-8*(len(title)+2))/2, 0, true, []string{title}, []color.RGBA{white})
	}
	dst.Fill(color.Black)
	return p.fit(dst, sc.img, sc.w, sc.h)
}

// textAt writes s at cell x, y of m in EGA colour c.
func textAt(m *image.RGBA, note Note, s string, x, y, c int) {
	drawText(m, note.Glyph, s, x*8, y*8, pal.EGA(c))
}
