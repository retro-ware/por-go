// Package vector draws filled and stroked shapes, like Ebiten's.
package vector

import (
	"image/color"
	"math"

	"gpu"
)

// tris draws triangles of one solid colour (the antialias flags are ignored).
func tris(dst *gpu.Image, pts [][2]float32, is []uint16, c color.Color) {
	r, g, b, a := c.RGBA()
	k := float32(1) / 0xffff
	vs := make([]gpu.Vertex, len(pts))
	for i, p := range pts {
		vs[i] = gpu.Vertex{DstX: p[0], DstY: p[1], SrcX: 0.5, SrcY: 0.5,
			ColorR: float32(r) * k, ColorG: float32(g) * k, ColorB: float32(b) * k, ColorA: float32(a) * k}
	}
	dst.DrawTriangles(vs, is, nil, nil)
}

func quad(dst *gpu.Image, x, y, w, h float32, c color.Color) {
	tris(dst, [][2]float32{{x, y}, {x + w, y}, {x, y + h}, {x + w, y + h}}, []uint16{0, 1, 2, 1, 3, 2}, c)
}

// DrawFilledRect fills a rectangle.
func DrawFilledRect(dst *gpu.Image, x, y, w, h float32, c color.Color, _ bool) { quad(dst, x, y, w, h, c) }

// StrokeRect outlines a rectangle with a line sw wide, centred on its edges.
func StrokeRect(dst *gpu.Image, x, y, w, h, sw float32, c color.Color, _ bool) {
	o := sw / 2
	quad(dst, x-o, y-o, w+sw, sw, c)
	quad(dst, x-o, y+h-o, w+sw, sw, c)
	quad(dst, x-o, y+o, sw, h-sw, c)
	quad(dst, x+w-o, y+o, sw, h-sw, c)
}

// StrokeLine draws a line sw wide.
func StrokeLine(dst *gpu.Image, x0, y0, x1, y1, sw float32, c color.Color, _ bool) {
	dx, dy := x1-x0, y1-y0
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if l == 0 {
		return
	}
	nx, ny := -dy/l*sw/2, dx/l*sw/2
	tris(dst, [][2]float32{{x0 + nx, y0 + ny}, {x1 + nx, y1 + ny}, {x0 - nx, y0 - ny}, {x1 - nx, y1 - ny}}, []uint16{0, 1, 2, 1, 3, 2}, c)
}

// DrawFilledCircle fills a circle.
func DrawFilledCircle(dst *gpu.Image, cx, cy, r float32, c color.Color, _ bool) {
	const n = 32
	pts := [][2]float32{{cx, cy}}
	var is []uint16
	for i := 0; i <= n; i++ {
		t := float64(i) * 2 * math.Pi / n
		pts = append(pts, [2]float32{cx + r*float32(math.Cos(t)), cy + r*float32(math.Sin(t))})
		if i > 0 {
			is = append(is, 0, uint16(i), uint16(i+1))
		}
	}
	tris(dst, pts, is, c)
}
