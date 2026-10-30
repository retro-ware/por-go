package remaster

import (
	"image"
	"image/color"
	"math"

	"gpu"

	"por/engine"
	"por/pal"
)

// The credits (View.Credits) and the saved games (View.Slots) on a 4:3
// screen. The credits: the original's 25 rows spread over 30 (a 4:3
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

// tallKey is what a spread page was made from.
type tallKey struct {
	seq  uint64
	note bool
	text string
	from [tallRows]int
}

const creditBlank = 0xc // a blank row inside the big box, for the new rows

func (p *Presenter) drawCredits(dst *gpu.Image, v engine.View, note Note) Rect {
	var from [tallRows]int
	for _, y := range []int{7, 14, 21, 27, 28} {
		from[y] = creditBlank
	}
	for y := 0; y < 25; y++ {
		from[creditRow(y)] = y
	}
	return p.drawTall(dst, v, note, from, func(m *image.RGBA) {
		s := engine.RemasterCredit
		x0, y0, fg := (2*pal.W-len(s)*8)/2, 28*16+4, pal.EGA(0xe)
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
	})
}

// drawSlots draws the saved games (View.Slots) over 30 rows: the list's
// box grows by five blank rows (row 20, inside the box under the list)
// above the footer.
func (p *Presenter) drawSlots(dst *gpu.Image, v engine.View, note Note) Rect {
	var from [tallRows]int
	for y := range from {
		switch {
		case y <= 20:
			from[y] = y
		case y < 26:
			from[y] = 20
		default:
			from[y] = y - 5
		}
	}
	return p.drawTall(dst, v, note, from, nil)
}

// drawTallPage draws a page over 30 rows so it fills a 4:3 window: its
// blank row View.TallRow repeats five times (the sheet, the loot list).
func (p *Presenter) drawTallPage(dst *gpu.Image, v engine.View, note Note) Rect {
	var from [tallRows]int
	for y := range from {
		switch r := v.TallRow; {
		case y <= r:
			from[y] = y
		case y <= r+5:
			from[y] = r
		default:
			from[y] = y - 5
		}
	}
	return p.drawTall(dst, v, note, from, nil)
}

// tallRows is a 4:3 screen's height in text rows.
const tallRows = 30

// drawTall draws the page with row y of the screen taken from the page's
// row from[y], at twice the size (so extra can draw in half cells), at a
// whole number of page pixels; the rows take the pointer (Cell) as the
// page's.
func (p *Presenter) drawTall(dst *gpu.Image, v engine.View, note Note, from [tallRows]int, extra func(*image.RGBA)) Rect {
	p.pageImage(v, note)
	const w, h = 2 * pal.W, 2 * tallRows * 8
	if key := (tallKey{v.Seq, note.Show, note.Text, from}); p.tallImg == nil || key != p.tallKey { // spread the page again only when it changed: this is per pixel, every frame was most of a slow machine's time
		p.tallKey = key
		src := p.pg.px
		m := image.NewRGBA(image.Rect(0, 0, w, h))
		for to, y := range from {
			for dy := 0; dy < 16; dy++ {
				row := m.Pix[(to*16+dy)*m.Stride:]
				srow := src.Pix[(y*8+dy/2)*src.Stride:]
				for x := 0; x < w; x++ {
					copy(row[x*4:x*4+4], srow[x/2*4:x/2*4+4])
				}
			}
		}
		if extra != nil {
			extra(m)
		}
		if p.tallImg == nil {
			p.tallImg = gpu.NewImage(w, h)
		}
		p.tallImg.WritePixels(m.Pix)
	}
	dst.Fill(color.Black)
	b := dst.Bounds()
	k := math.Max(1, math.Floor(math.Min(float64(b.Dx())/pal.W, float64(b.Dy())/(h/2)))) / 2 // whole page pixels
	x0, y0 := (float64(b.Dx())-k*w)/2, (float64(b.Dy())-k*h)/2
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(x0, y0)
	dst.DrawImage(p.tallImg, op)
	p.drawOverlays(dst, v.Overlays, Rect{x0, y0, k * w, k * h}) // the sheet's dropped portrait: in rows the rows keep
	for to, y := range from {
		at := image.Rect(int(x0), int(y0+k*float64(to*16)), int(x0+k*w), int(y0+k*float64(to*16+16)))
		p.rows = append(p.rows, shownRows{at, image.Rect(0, y*8, pal.W, y*8+8)})
	}
	return Rect{}
}
