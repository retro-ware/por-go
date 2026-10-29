package amiga

import (
	"encoding/binary"
	"image"
	"image/color"
)

// Title decodes the painted title (the golden dragon over the hero) from
// disk 1's "intro": a PowerPacker-crunched executable whose decrunched hunk
// 5 starts with the picture, 4 bitplanes of 320x200 one after the other,
// then its 16 colours as 0x0RGB words. Nil if intro is not that file.
func Title(intro []byte) (img image.Image) {
	defer func() {
		if recover() != nil {
			img = nil
		}
	}()
	h := hunks(ppDecrunch(hunks(intro)[1]))
	if len(h) < 6 {
		return nil
	}
	const w, ht, planes, plane = 320, 200, 4, 320 * 200 / 8
	d := h[5]
	var p color.Palette
	for i := 0; i < 1<<planes; i++ {
		v := binary.BigEndian.Uint16(d[planes*plane+2*i:])
		p = append(p, color.RGBA{uint8(v>>8&15) * 17, uint8(v>>4&15) * 17, uint8(v&15) * 17, 255})
	}
	out := image.NewPaletted(image.Rect(0, 0, w, ht), p)
	for y := 0; y < ht; y++ {
		for x := 0; x < w; x++ {
			var c uint8
			for b := 0; b < planes; b++ {
				if d[b*plane+y*w/8+x/8]&(0x80>>(x%8)) != 0 {
					c |= 1 << b
				}
			}
			out.Pix[y*w+x] = c
		}
	}
	return out
}

// hunks returns the code and data hunks' contents of an executable (BSS as nil).
func hunks(exe []byte) [][]byte {
	p := 0
	u := func() int { v := binary.BigEndian.Uint32(exe[p:]); p += 4; return int(v & 0x3fffffff) }
	if u() != 0x3f3 {
		return nil
	}
	for u() != 0 {
	}
	u()
	first, last := u(), u()
	p += 4 * (last - first + 1)
	var out [][]byte
	for p < len(exe) {
		switch u() {
		case 0x3e9, 0x3ea:
			l := u() * 4
			out = append(out, exe[p:p+l])
			p += l
		case 0x3eb:
			u()
			out = append(out, nil)
		case 0x3ec:
			for c := u(); c != 0; c = u() {
				p += 4 + 4*c
			}
		case 0x3f2:
		default:
			return out
		}
	}
	return out
}

// ppDecrunch unpacks PowerPacker data (efficiency 9,9,9,9, as intro's
// decruncher has it): read backwards, the last long the output length
// (24 bits) and the bits to skip.
func ppDecrunch(src []byte) []byte {
	eff := [4]int{9, 9, 9, 9}
	last := binary.BigEndian.Uint32(src[len(src)-4:])
	out := make([]byte, last>>8)
	o, p := len(out), len(src)-4
	var buf uint32
	n := 0
	bit := func() int {
		if n == 0 {
			p -= 4
			buf, n = binary.BigEndian.Uint32(src[p:]), 32
		}
		b := int(buf & 1)
		buf >>= 1
		n--
		return b
	}
	bits := func(k int) int {
		v := 0
		for ; k > 0; k-- {
			v = v<<1 | bit()
		}
		return v
	}
	bits(int(last & 0xff))
	for o > 0 {
		if bit() == 0 { // a literal run
			t := 1
			for x := 3; x == 3; t += x {
				x = bits(2)
			}
			for ; t > 0 && o > 0; t-- {
				o--
				out[o] = byte(bits(8))
			}
			if o <= 0 {
				break
			}
		}
		x := bits(2)
		ob, t, off := eff[x], x+2, 0
		if x == 3 {
			if bit() == 0 {
				ob = 7
			}
			off = bits(ob)
			for y := 7; y == 7; t += y {
				y = bits(3)
			}
		} else {
			off = bits(ob)
		}
		for ; t > 0 && o > 0; t-- {
			o--
			out[o] = out[o+off+1]
		}
	}
	return out
}
