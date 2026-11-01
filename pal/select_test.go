package pal

import (
	"image"
	"image/color"
	"testing"
)

func TestSelect(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 6, 6))
	fill := func(r image.Rectangle, c color.RGBA) {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				m.SetRGBA(x, y, c)
			}
		}
	}
	SelectOutline(image.Rect(1, 1, 5, 5), 1, fill)
	if m.RGBAAt(1, 1) != EGA(10) || m.RGBAAt(4, 4) != EGA(10) || m.RGBAAt(2, 2) != (color.RGBA{}) || m.RGBAAt(0, 0) != (color.RGBA{}) {
		t.Error("outline: a green frame inside r only")
	}
	SelectText(image.Rect(2, 2, 4, 4), fill)
	if m.RGBAAt(2, 3) != EGA(1) || m.RGBAAt(1, 3) != EGA(10) {
		t.Error("text: r filled blue")
	}
}
