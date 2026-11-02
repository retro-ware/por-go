package remaster

import (
	"image"
	"image/color"
	"image/draw"

	"por/engine"
	"por/pal"
)

// mapImage draws m at cell pixels a square in the colours ink: seen
// squares dark grey, walked ones blue, the walls (dim on squares not yet
// seen, doors not told apart there), the labels (their letter in glyph's
// font when glyph is set and the cell fits it, else a dot) and the party, a
// dot with a tick towards its facing.
func mapImage(m *engine.MapSquares, cell int, ink [16]color.RGBA, glyph func(byte, bool) [8]byte) *image.RGBA {
	n := 16 * cell
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		draw.Draw(img, image.Rect(x0, y0, x1, y1), image.NewUniform(c), image.Point{}, draw.Src)
	}
	fill(0, 0, n, n, ink[0])
	regs := pal.Identity
	door := regs.RGBA(engine.DoorColour)
	onMap := func(p image.Point) bool { return p.In(image.Rect(0, 0, 16, 16)) }
	for y, row := range m.Squares {
		for x, q := range row {
			px, py := x*cell, y*cell
			switch q.Seen {
			case engine.SeenFar:
				fill(px, py, px+cell, py+cell, ink[8])
			case engine.Visited:
				fill(px, py, px+cell, py+cell, ink[1])
			}
			for d, w := range q.Walls {
				c := ink[15]
				switch {
				case w == engine.NoWall:
					continue
				case q.Seen == engine.Unseen:
					c = ink[8]
				case w == engine.Door:
					c = door
				case w == engine.LockedDoor:
					c = ink[14]
				}
				switch d {
				case 0:
					fill(px, py, px+cell, py+1, c)
				case 1:
					fill(px+cell-1, py, px+cell, py+cell, c)
				case 2:
					fill(px, py+cell-1, px+cell, py+cell, c)
				case 3:
					fill(px, py, px+1, py+cell, c)
				}
			}
			switch {
			case q.Ink == 0 || image.Pt(x, y) == m.Party:
			case glyph != nil && cell >= 10:
				drawText(img, func(c byte) [8]byte { return glyph(c, true) }, string(q.Letter), px+(cell-8)/2, py+(cell-8)/2, ink[q.Ink]) // centred in the cell
			default:
				a, b := cell*2/7, cell*5/7
				fill(px+a, py+a, px+b, py+b, ink[q.Ink])
			}
		}
	}
	if onMap(m.Party) {
		r := max(2, cell/5)
		cx, cy := m.Party.X*cell+cell/2, m.Party.Y*cell+cell/2
		fill(cx-r+1, cy-r+1, cx+r, cy+r, ink[12])
		for i := r + 1; i <= r+2; i++ {
			t := image.Pt(cx, cy).Add(m.Ahead.Mul(i))
			fill(t.X, t.Y, t.X+1, t.Y+1, ink[14])
		}
	}
	return img
}
