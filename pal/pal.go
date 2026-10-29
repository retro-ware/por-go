// Package pal is the game's colours: the 16-colour EGA set the DOS build
// drew in, and the Amiga palettes its art may carry.
package pal

import "image/color"

// W and H are the original screen's size in pixels.
const (
	W = 320
	H = 200
)

// Palette is the standard 16-colour EGA/CGA palette (RGB).
var Palette = [16][3]uint8{
	{0, 0, 0}, {0, 0, 173}, {0, 173, 0}, {0, 173, 173},
	{173, 0, 0}, {173, 0, 173}, {173, 82, 0}, {173, 173, 173},
	{82, 82, 82}, {82, 82, 255}, {82, 255, 82}, {82, 255, 255},
	{255, 82, 82}, {255, 82, 255}, {255, 255, 82}, {255, 255, 255},
}

// EGA is colour i of Palette.
func EGA(i int) color.RGBA { c := Palette[i&15]; return color.RGBA{c[0], c[1], c[2], 255} }

// Amiga holds the Amiga Pool of Radiance palettes (12-bit RGB, from the
// program's data at 0x54080, 0x54040 and 0x54020). Pixel values from AmigaBase up
// are colour AmigaBase+16*slot+index of these, drawn in their own RGB.
var Amiga = [3][16]uint16{
	AmigaView:     {0x000, 0x124, 0x953, 0xb97, 0xecb, 0x710, 0xa42, 0xdb0, 0xff5, 0x074, 0x053, 0x369, 0x555, 0x777, 0xaaa, 0xfff},
	AmigaPortrait: {0x000, 0xdb9, 0xb97, 0x964, 0x521, 0x742, 0xa84, 0xcb6, 0xff9, 0x007, 0x44b, 0x444, 0x555, 0x777, 0xaaa, 0xfff},
	AmigaTerrain:  {0x000, 0x06b, 0x096, 0x0aa, 0x000, 0x940, 0xb44, 0xfaa, 0xfe9, 0x59f, 0x0d8, 0x5ff, 0x555, 0x777, 0xaaa, 0xfff},
}

// Amiga palette slots: PIC/FINAL pictures; portraits; the area map,
// combat ground tiles and combat icons.
const (
	AmigaView = iota
	AmigaPortrait
	AmigaTerrain
	AmigaBase = 32
)

// AmigaSaturation scales the Amiga colours' distance from grey, to sit
// closer to the vivid EGA set (1 = the Amiga's own values).
var AmigaSaturation = 1.4

// Regs are the EGA palette registers: register -> Palette colour.
type Regs [16]uint8

// Identity is the registers as the BIOS sets them.
var Identity = Regs{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

// RGBA is the displayed colour of pixel value i under the registers r.
func (r *Regs) RGBA(i uint8) color.RGBA {
	if i >= AmigaBase {
		c := Amiga[int(i-AmigaBase)/16%len(Amiga)][i%16]
		return saturate(float64(c>>8&15)*17, float64(c>>4&15)*17, float64(c&15)*17)
	}
	return EGA(int(r[i&15]))
}

func saturate(r, g, b float64) color.RGBA {
	l := 0.299*r + 0.587*g + 0.114*b
	f := func(v float64) uint8 { return uint8(max(0, min(255, l+(v-l)*AmigaSaturation)) + 0.5) }
	return color.RGBA{f(r), f(g), f(b), 255}
}
