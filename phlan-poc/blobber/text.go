package blobber

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"gpu"
)

// Glyph, if set, is the host's 8x8 font (a row a byte, bit 7 leftmost);
// the engine's text then draws in it twice its size. Without it, the plain
// built-in 7x13 face.
var Glyph func(byte) [8]byte

// LineH is the height of a line of text.
const LineH = 16

var texts = map[string]*gpu.Image{}

// Width is s's width in pixels.
func Width(s string) int {
	if Glyph != nil {
		return 16 * len(s)
	}
	return font.MeasureString(basicfont.Face7x13, s).Ceil()
}

// Print draws s in white with its top left at (x, y).
func Print(dst *gpu.Image, s string, x, y int) {
	if s == "" {
		return
	}
	img, ok := texts[s]
	if !ok {
		if len(texts) > 256 { // ponytail: drop the whole cache, an LRU if this ever shows up in profiles
			for _, t := range texts {
				t.Deallocate()
			}
			clear(texts)
		}
		img = gpu.NewImageFromImage(render(s))
		texts[s] = img
	}
	op := &gpu.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(img, op)
}

func render(s string) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, max(1, Width(s)), LineH))
	if Glyph == nil {
		d := font.Drawer{Dst: m, Src: image.NewUniform(color.White), Face: basicfont.Face7x13, Dot: fixed.P(0, 12)}
		d.DrawString(s)
		return m
	}
	for i := 0; i < len(s); i++ {
		g := Glyph(s[i])
		for y := range 8 {
			for b := range 8 {
				if g[y]&(0x80>>b) != 0 {
					draw.Draw(m, image.Rect(i*16+2*b, 2*y, i*16+2*b+2, 2*y+2), image.White, image.Point{}, draw.Src)
				}
			}
		}
	}
	return m
}
