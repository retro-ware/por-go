package battle

import (
	"math"

	"gpu"

	"por/engine"
)

// XCOM-style health bars, shown while Shift is held: every bar is as wide
// as the next, cut into one segment per hit point (a few per segment for
// the hardiest, so a segment stays wide enough to see).

// maxPips is the most segments a bar is cut into.
const maxPips = 40

var (
	partyBar = [4]float32{0.75, 0.9, 0.88, 1} // XCOM's pale soldier teal
	enemyBar = [4]float32{0.9, 0.18, 0.14, 1}
	lostBar  = [4]float32{0.16, 0.16, 0.18, 0.85} // hit points lost
	barBack  = [4]float32{0, 0, 0, 0.6}
)

// head is where a figurine's bar goes: over its head, in the world.
type head struct {
	u   engine.Unit
	top vec
}

// pips is how a bar of maxHP is cut: segments, hit points a segment, and
// the segments hp fills.
func pips(hp, maxHP int) (n, per, full int) {
	per = (maxHP + maxPips - 1) / maxPips
	n = (maxHP + per - 1) / per
	full = (max(hp, 0) + per - 1) / per
	return n, per, min(full, n)
}

// project is world point v on dst in pixels; ok is false behind the eye.
func project(dst *gpu.Image, vp gpu.Mat4, v vec) (x, y float64, ok bool) {
	var c [4]float64
	for row := range 4 {
		c[row] = float64(vp[row])*v[0] + float64(vp[4+row])*v[1] + float64(vp[8+row])*v[2] + float64(vp[12+row])
	}
	if c[3] <= 0 {
		return 0, 0, false
	}
	r := dst.Bounds()
	return float64(r.Min.X) + (c[0]/c[3]+1)/2*float64(r.Dx()), float64(r.Min.Y) + (1-c[1]/c[3])/2*float64(r.Dy()), true
}

// drawHealth draws the bars over the figurines.
func (p *Presenter) drawHealth(dst *gpu.Image, vp gpu.Mat4, heads []head) {
	r := dst.Bounds()
	w := math.Round(float64(r.Dy()) / 14) // the same width for every bar
	h := math.Max(3, math.Round(w/10))
	gap := math.Max(1, math.Round(w/80))
	rect := func(x, y, rw, rh float64, c [4]float32) {
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(rw, rh)
		op.GeoM.Translate(x, y)
		op.ColorScale.Scale(c[0]*c[3], c[1]*c[3], c[2]*c[3], c[3])
		dst.DrawImage(p.white, op)
	}
	for _, hd := range heads {
		u := hd.u
		if u.MaxHP <= 0 {
			continue
		}
		cx, cy, ok := project(dst, vp, hd.top)
		if !ok {
			continue
		}
		x0, y0 := math.Round(cx-w/2), math.Round(cy-h)
		rect(x0-gap, y0-gap, w+2*gap, h+2*gap, barBack)
		col := enemyBar
		if u.Party {
			col = partyBar
		}
		n, _, full := pips(u.HP, u.MaxHP)
		pw := (w - gap*float64(n-1)) / float64(n)
		for i := range n {
			c := lostBar
			if i < full {
				c = col
			}
			a := math.Round(x0 + float64(i)*(pw+gap))
			b := math.Round(x0 + float64(i)*(pw+gap) + pw)
			rect(a, y0, math.Max(1, b-a), h, c)
		}
	}
}
