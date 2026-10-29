package remaster

import (
	"image"
	"image/color"
	"math"

	"gpu"

	"por/engine"
	"por/pal"
)

// The credits (View.Credits): the original's 25 rows spread over 30 (a 4:3
// screen), a blank row more between the sections, and the remaster's own
// line at the bottom of the big box, at half the cell size so it fits.

// creditRows maps the original's rows to the 30: the sections move down.
func creditRow(y int) int {
	switch {
	case y == 24:
		return 29 // the bottom border
	case y >= 19:
		return y + 3
	case y >= 13:
		return y + 2
	case y >= 7:
		return y + 1
	}
	return y
}

const creditBlank = 0xc // a blank row inside the big box, for the new rows

func (p *Presenter) drawCredits(dst *gpu.Image, v engine.View, note Note) Rect {
	p.pageImage(v, note)
	src := p.pg.px
	const w, h = 2 * pal.W, 2 * 30 * 8 // twice the size: the remaster's line in half cells
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	row := func(from, to int) {
		for y := 0; y < 16; y++ {
			for x := 0; x < w; x++ {
				m.SetRGBA(x, to*16+y, src.RGBAAt(x/2, from*8+y/2))
			}
		}
	}
	for _, y := range []int{7, 14, 21, 27, 28} {
		row(creditBlank, y)
	}
	for y := 0; y < 25; y++ {
		row(y, creditRow(y))
	}
	s := engine.RemasterCredit
	x0, y0, fg := (w-len(s)*8)/2, 28*16+4, pal.EGA(0xe)
	for i := 0; i < len(s); i++ {
		g := p.glyph(s[i])
		for r := 0; r < 8; r++ {
			for b := 0; b < 8; b++ {
				if g[r]&(0x80>>b) != 0 {
					m.SetRGBA(x0+i*8+b, y0+r, fg)
				}
			}
		}
	}
	if p.creditImg == nil {
		p.creditImg = gpu.NewImage(w, h)
	}
	p.creditImg.WritePixels(m.Pix)
	dst.Fill(color.Black)
	b := dst.Bounds()
	k := math.Max(1, math.Floor(math.Min(float64(b.Dx())/pal.W, float64(b.Dy())/(h/2)))) / 2 // whole page pixels
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate((float64(b.Dx())-k*w)/2, (float64(b.Dy())-k*h)/2)
	dst.DrawImage(p.creditImg, op)
	return Rect{}
}
