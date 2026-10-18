package classic

import (
	"image"
	"slices"

	"gpu"

	"por/ega"
	"por/engine"
)

// The combat map as the original lays it out: the view's squares are 3x3
// text cells from cell (2,1), 12 across and 7 down.
const (
	mapX, mapY   = 2 * 8, 1 * 8
	squarePx     = 24
	viewW, viewH = 12, 7
)

// the remaster's preview colours (present/battle), added over the picture
var (
	hoverTint   = [4]float32{0.3, 0.3, 0.3, 1}
	areaTint    = [4]float32{0.45, 0.2, 0.03, 1}
	targetTint  = [4]float32{0.95, 0.12, 0.08, 1}
	blockedTint = [4]float32{0.55, 0.03, 0.02, 1}
	reachTint   = [4]float32{0.03, 0.1, 0.28, 1}
	pathTint    = [4]float32{0.1, 0.35, 0.8, 1}
	freeTint    = [4]float32{0.9, 0.1, 0.05, 1}
)

var white *gpu.Image

// DrawBattle draws the remaster's targeting previews over the combat map
// of the picture at pic: where a click walks (blue), the walk to the
// square under the mouse at device pixel (mx, my) (red where leaving
// lets a foe strike), the foes a click attacks (red frames), a spell's
// area (orange) and an aim that cannot be taken (red).
func (p *Presenter) DrawBattle(dst *gpu.Image, pic Rect, b *engine.Battle, mx, my int) {
	if b == nil || pic.W <= 0 {
		return
	}
	if white == nil {
		white = gpu.NewImage(1, 1)
		white.WritePixels([]byte{255, 255, 255, 255})
	}
	sx, sy := pic.W/ega.W, pic.H/ega.H
	// rect lights the part of square q from (x0,y0) to (x1,y1), in its 24 pixels
	rect := func(q image.Point, x0, y0, x1, y1 float64, t [4]float32) {
		v := q.Sub(b.View)
		if v.X < 0 || v.Y < 0 || v.X >= viewW || v.Y >= viewH {
			return
		}
		op := &gpu.DrawImageOptions{Blend: gpu.BlendLighter}
		op.GeoM.Scale((x1-x0)*sx, (y1-y0)*sy)
		op.GeoM.Translate(pic.X+(float64(mapX+v.X*squarePx)+x0)*sx, pic.Y+(float64(mapY+v.Y*squarePx)+y0)*sy)
		op.ColorScale.Scale(t[0], t[1], t[2], 1)
		dst.DrawImage(white, op)
	}
	wash := func(q image.Point, t [4]float32) { rect(q, 1, 1, squarePx-1, squarePx-1, t) }
	frame := func(q image.Point, t [4]float32) {
		rect(q, 0, 0, squarePx, 1.5, t)
		rect(q, 0, squarePx-1.5, squarePx, squarePx, t)
		rect(q, 0, 1.5, 1.5, squarePx-1.5, t)
		rect(q, squarePx-1.5, 1.5, squarePx, squarePx-1.5, t)
	}

	for _, r := range b.Reach {
		wash(r, reachTint)
	}
	area := areaTint
	if b.AimWhy != "" {
		area = blockedTint
	}
	for _, a := range b.AreaAt {
		wash(a, area)
	}
	if b.AimWhy != "" && b.CursorOn {
		for _, c := range b.CursorAt {
			wash(c, blockedTint)
		}
	}
	for _, u := range b.Units {
		if slices.Contains(b.Targets, u.ID) {
			n := max(1, u.Size)
			for d := range n * n {
				frame(image.Pt(u.X+d%n, u.Y+d/n), targetTint)
			}
		}
	}
	px, py := (float64(mx)-pic.X)/sx-mapX, (float64(my)-pic.Y)/sy-mapY
	if px >= 0 && py >= 0 && px < viewW*squarePx && py < viewH*squarePx {
		q := b.View.Add(image.Pt(int(px)/squarePx, int(py)/squarePx))
		frame(q, hoverTint)
		for i, s := range b.Paths[q] {
			if i == 0 { // the mover's own square
				continue
			}
			t := pathTint
			if s.Free {
				t = freeTint
			}
			rect(s.Point, 7, 7, squarePx-7, squarePx-7, t)
		}
	}
}
