package remaster

import (
	"image"
	"image/color"
	"image/draw"

	"gpu"
	"gpu/vector"

	"por/engine"
	"por/pal"
)

// The shop's Id (View.Shop.Identified): what it revealed in a pop-up of
// its own over the dimmed screen, the icon large, the name and every line
// of what the item does in colour. Any key or click closes it.

// identified is the reveal still to show (nil: none).
func (p *Presenter) identified(v engine.View) *engine.InvItem {
	if s := v.Shop; s != nil && s.Identified != nil && s.IdentifiedN != p.idSeen {
		return s.Identified
	}
	return nil
}

// identDone closes the reveal's pop-up if one is up; it reports whether one was.
func (p *Presenter) identDone(v engine.View) bool {
	if p.identified(v) == nil {
		return false
	}
	p.idSeen = v.Shop.IdentifiedN
	return true
}

// drawIdentified draws the reveal's pop-up over dst.
func (p *Presenter) drawIdentified(dst *gpu.Image, glyph func(byte) [8]byte, it *engine.InvItem) {
	const title, hint, iconH = "* Identified! *", "Splendid!", 48
	w := max(len(title), len(it.Name), len(hint))
	for _, l := range it.Look {
		w = max(w, len(l)+2)
	}
	cw := w + 4
	ch := 2 + 2 + iconH/8 + 1 + 2 + len(it.Look) + 2 // the border, the title and a blank, the icon and a blank, the name and a blank, the lines, a blank and the hint
	m := image.NewRGBA(image.Rect(0, 0, cw*8, ch*8))
	draw.Draw(m, m.Rect, image.Black, image.Point{}, draw.Src)
	centre := func(s string) int { return (cw*8 - 8*len(s)) / 2 }
	sparkle := []int{13, 14, 11, 15, 10, 12} // the title's letters in turn: light magenta, yellow, cyan, white, green, red
	x0 := centre(title)
	for i := 0; i < len(title); i++ {
		drawText(m, glyph, title[i:i+1], x0+8*i, 8, pal.EGA(sparkle[i%len(sparkle)]))
	}
	y := 24
	if ic := p.inv.iconImage(it); ic != nil { // the icon, on a glow of dark magenta, its aspect kept
		b := ic.Bounds()
		f := min(float64(iconH)/float64(b.Dx()), float64(iconH)/float64(b.Dy()))
		iw, ih := int(f*float64(b.Dx())), int(f*float64(b.Dy()))
		at := image.Rect((cw*8-iw)/2, y, (cw*8+iw)/2, y+ih)
		draw.Draw(m, at.Inset(-4), image.NewUniform(pal.EGA(5)), image.Point{}, draw.Src)
		for py := range ih {
			for px := range iw {
				if c := ic.At(b.Min.X+px*b.Dx()/iw, b.Min.Y+py*b.Dy()/ih); alpha(c) {
					m.Set(at.Min.X+px, at.Min.Y+py, c)
				}
			}
		}
	}
	y += iconH + 8
	drawText(m, glyph, it.Name, centre(it.Name), y, pal.EGA(inkYellow))
	y += 16
	for _, l := range it.Look {
		drawText(m, glyph, "*", 16, y, pal.EGA(13))
		drawText(m, glyph, l, 32, y, pal.EGA(11))
		y += 8
	}
	drawText(m, glyph, hint, centre(hint), y+8, pal.EGA(inkWhite))

	img := upload(&p.idImg, m)
	window(img, p.border.tiles, gpu.GeoM{}, cw, ch, nil)
	b := dst.Bounds()
	vector.DrawFilledRect(dst, 0, 0, float32(b.Dx()), float32(b.Dy()), color.NRGBA{A: 0xA0})
	k := fitScale(scale(dst), 1, func(k float64) bool { // K, as far as it fits
		return k*float64(cw*8) <= float64(b.Dx()) && k*float64(ch*8) <= float64(b.Dy())
	})
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate((float64(b.Dx())-k*float64(cw*8))/2, (float64(b.Dy())-k*float64(ch*8))/2)
	dst.DrawImage(img, op)
}
