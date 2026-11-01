package remaster

import (
	"image"
	"image/color"

	"gpu"
	"gpu/vector"
)

// scrollBar is every scrolled list's bar (the inventory's look): n lines,
// rows of them shown from top, in r: a groove with a bevelled thumb.
type scrollBar struct {
	r            image.Rectangle
	n, rows, top int
}

// thumb is the part of r the shown lines take.
func (s scrollBar) thumb() image.Rectangle {
	h := s.r.Dy()
	return image.Rect(s.r.Min.X, s.r.Min.Y+h*s.top/s.n, s.r.Max.X, s.r.Min.Y+h*(s.top+s.rows)/s.n)
}

// to is the top that puts the thumb's middle at y, kept in range.
func (s scrollBar) to(y int) int {
	return min(max(0, (y-s.r.Min.Y)*s.n/max(1, s.r.Dy())-s.rows/2), max(0, s.n-s.rows))
}

// draw paints the bar with fill: the groove in r's middle third, the thumb
// grey with a light top and a dark bottom edge.
func (s scrollBar) draw(fill func(image.Rectangle, color.RGBA)) {
	w, e := s.r.Dx()/3, max(1, s.r.Dx()/6)
	fill(image.Rect(s.r.Min.X+w, s.r.Min.Y, s.r.Max.X-w, s.r.Max.Y), dark)
	t := s.thumb()
	fill(t, gray)
	fill(image.Rect(t.Min.X, t.Min.Y, t.Max.X, t.Min.Y+e), white)
	fill(image.Rect(t.Min.X, t.Max.Y-e, t.Max.X, t.Max.Y), dark)
}

// barHit is a bar drawn this frame and what moving its thumb does.
type barHit struct {
	bar scrollBar
	set func(top int)
}

// menuBar draws the scroll bar of a menu (drawMenu, at scale k) whose
// first rows lines (rs, its rows' rectangles) show n lines from *top, in
// its right blank column, and takes its clicks and drags: they move *top
// and keep *sel (nil: none) among the lines shown.
func (p *Presenter) menuBar(dst *gpu.Image, rs []image.Rectangle, n, rows int, top, sel *int, k float64) {
	if n <= rows || len(rs) < rows || rows == 0 {
		return
	}
	r0, r1 := rs[0], rs[rows-1]
	b := scrollBar{image.Rect(r0.Max.X-int(7*k), r0.Min.Y, r0.Max.X-int(k), r1.Max.Y), n, rows, *top}
	b.draw(func(r image.Rectangle, c color.RGBA) {
		vector.DrawFilledRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), c, false)
	})
	p.scrollBars = append(p.scrollBars, barHit{b, func(t int) {
		*top = t
		if sel != nil {
			*sel = min(max(*sel, t), t+rows-1)
		}
	}})
}

// barClick starts a drag on a scroll bar under (x, y): the thumb jumps there.
func (p *Presenter) barClick(x, y int) bool {
	for _, h := range p.scrollBars {
		if image.Pt(x, y).In(h.bar.r) {
			h.set(h.bar.to(y))
			p.barDrag = &h
			return true
		}
	}
	return false
}

// barHover drags the thumb taken by barClick while the button is down.
func (p *Presenter) barHover(y int, down bool) bool {
	h := p.barDrag
	if h == nil {
		return false
	}
	if !down {
		p.barDrag = nil
		return false
	}
	h.set(h.bar.to(y))
	return true
}
