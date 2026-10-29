package dax

import (
	"encoding/binary"
	"por/pal"
	"sort"
)

// Amiga Pool of Radiance archives: u16 BE index size, 10-byte BE entries
// (u16 id, u32 offset, u16 stored size, u16 raw size), then ByteKiller
// crunched blocks (decruncher at 0x63346 of the Amiga "program").

// OpenAmiga indexes an Amiga .dax file held in b; blocks come out raw.
func OpenAmiga(b []byte) *File {
	n := int(binary.BigEndian.Uint16(b))
	f := &File{data: b[2+n:], cache: map[int][]byte{}}
	for i := 2; i+10 <= 2+n; i += 10 {
		e := Entry{ID: int(binary.BigEndian.Uint16(b[i:])), Offset: int32(binary.BigEndian.Uint32(b[i+2:])), Stored: binary.BigEndian.Uint16(b[i+6:])}
		f.Entries = append(f.Entries, e)
		f.cache[e.ID] = Unbytekill(f.data[e.Offset : int(e.Offset)+int(e.Stored)])
	}
	return f
}

// Unbytekill decrunches a ByteKiller stream, read backwards from its end:
// u32 raw size, u32 checksum, then the bit stream; output is filled from
// the end too. Returns nil on a malformed stream.
func Unbytekill(src []byte) (out []byte) {
	defer func() {
		if recover() != nil {
			out = nil
		}
	}()
	p := len(src)
	long := func() uint32 { p -= 4; return binary.BigEndian.Uint32(src[p:]) }
	out = make([]byte, long())
	long() // checksum
	d0 := long()
	bit := func() uint32 {
		c := d0 & 1
		d0 >>= 1
		if d0 == 0 {
			d0 = long()
			c = d0 & 1
			d0 = d0>>1 | 0x80000000
		}
		return c
	}
	bits := func(n int) int {
		v := 0
		for ; n > 0; n-- {
			v = v<<1 | int(bit())
		}
		return v
	}
	q := len(out)
	copyBack := func(nbits, count int) {
		off := bits(nbits)
		for ; count > 0; count-- {
			q--
			out[q] = out[q+off]
		}
	}
	literal := func(count int) {
		for ; count > 0; count-- {
			q--
			out[q] = byte(bits(8))
		}
	}
	for q > 0 {
		if bit() == 0 {
			if bit() == 1 {
				copyBack(8, 2)
			} else {
				literal(bits(3) + 1)
			}
			continue
		}
		switch d := bits(2); d {
		case 0, 1:
			copyBack(9+d, d+3)
		case 2:
			copyBack(12, bits(8)+1)
		case 3:
			literal(bits(8) + 9)
		}
	}
	return out
}

// Amiga image header (BE): u16 height, u16 width cells, u16 x, u16 y,
// u16 frame count, u16 plane size; then per frame four bitplanes of
// height rows padded to whole 16-pixel words, plane 0 the low bit (tile
// sheets: plane 0 of every frame, then plane 1 of every frame, ...).

// chunky converts one frame of planes to DOS packed nibbles (w cells wide).
func chunky(src []byte, h, w, stride, plane int) []byte {
	out := make([]byte, h*w*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w*8; x++ {
			var c byte
			for p := 0; p < 4; p++ {
				i := p*plane + y*stride + x/8
				if i < len(src) && src[i]&(0x80>>(x%8)) != 0 {
					c |= 1 << p
				}
			}
			if x%2 == 0 {
				out[(y*w*8+x)/2] = c << 4
			} else {
				out[(y*w*8+x)/2] |= c
			}
		}
	}
	return out
}

func be16(b []byte, i int) int { return int(binary.BigEndian.Uint16(b[i:])) }

// AmigaImage converts a plain picture block to the DOS layout ParseImage
// reads, tagged with the pal.Amiga palette slot its colours come from.
// split is for the tile sheets: each frame holds two side-by-side tiles,
// and each bitplane runs through all frames before the next one starts.
func AmigaImage(b []byte, split bool, slot byte) []byte {
	h, w, x, y, n, plane := be16(b, 0), be16(b, 2), be16(b, 4), be16(b, 6), be16(b, 8), be16(b, 10)
	var frames [][]byte
	for f := 0; f < n && 12+(f+1)*4*plane <= len(b); f++ {
		off, step := 12+f*4*plane, plane
		if split {
			off, step = 12+f*plane, plane*n
		}
		px := chunky(b[off:], h, w, (w+1)/2*2, step)
		if !split {
			frames = append(frames, px)
			continue
		}
		hw := w / 2 * 4 // bytes per half row
		l, r := make([]byte, 0, h*hw), make([]byte, 0, h*hw)
		for row := 0; row < h; row++ {
			l = append(l, px[row*w*4:row*w*4+hw]...)
			r = append(r, px[row*w*4+hw:row*w*4+2*hw]...)
		}
		frames = append(frames, l, r)
	}
	if split {
		w /= 2
	}
	out := binary.LittleEndian.AppendUint16(nil, uint16(h))
	out = binary.LittleEndian.AppendUint16(out, uint16(w))
	out = binary.LittleEndian.AppendUint16(out, uint16(x))
	out = binary.LittleEndian.AppendUint16(out, uint16(y))
	out = append(out, byte(len(frames)))
	out = append(out, 'A', 'M', slot, 0, 0, 0, 0, 0) // amigaTag in the unused bytes
	for _, f := range frames {
		out = append(out, f...)
	}
	return out
}

// AmigaSprite converts an animation block (u32 frame count, per frame u32
// delay in 50 Hz ticks and a one-frame picture) to the ParseSprite layout.
// xor stores frames after the first XORed against it, as PIC/FINAL expect.
func AmigaSprite(b []byte, xor bool, slot byte) []byte {
	n := int(binary.BigEndian.Uint32(b))
	out := []byte{byte(n)}
	var first []byte
	for f, p := 0, 4; f < n && p+16 <= len(b); f++ {
		delay := int(binary.BigEndian.Uint32(b[p:])) / 5
		h, w, x, y, plane := be16(b, p+4), be16(b, p+6), be16(b, p+8), be16(b, p+10), be16(b, p+14)
		p += 16
		if p+4*plane > len(b) {
			break
		}
		px := chunky(b[p:], h, w, (w+1)/2*2, plane)
		p += 4 * plane
		if f == 0 {
			first = px
		} else if xor {
			for i := range px {
				if i < len(first) {
					px[i] ^= first[i]
				}
			}
		}
		out = append(out, byte(delay), byte(delay>>8), 0, 0)
		out = binary.LittleEndian.AppendUint16(out, uint16(h))
		out = binary.LittleEndian.AppendUint16(out, uint16(w))
		out = binary.LittleEndian.AppendUint16(out, uint16(x))
		out = binary.LittleEndian.AppendUint16(out, uint16(y))
		out = append(out, 'A', 'M', slot, 0, 0, 0, 0, 0, 0)
		out = append(out, px...)
	}
	return out
}

// WriteDOS packs blocks (id -> decoded bytes) into a DOS .DAX archive with
// every block stored uncompressed (raw size 0).
func WriteDOS(blocks map[int][]byte) []byte {
	var ids []int
	for id := range blocks {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	idx, data := []byte{}, []byte{}
	for _, id := range ids {
		b := blocks[id]
		idx = append(idx, byte(id))
		idx = binary.LittleEndian.AppendUint32(idx, uint32(len(data)))
		idx = append(idx, 0, 0)
		idx = binary.LittleEndian.AppendUint16(idx, uint16(len(b)))
		data = append(data, b...)
	}
	out := binary.LittleEndian.AppendUint16(nil, uint16(len(idx)))
	return append(append(out, idx...), data...)
}

// amigaTag: an "AM" mark and palette slot in a picture header's unused
// bytes moves its colours to pal.AmigaBase+16*slot (masked pixels stay Clear).
func amigaTag(hdr, px []byte) {
	if len(hdr) < 3 || hdr[0] != 'A' || hdr[1] != 'M' {
		return
	}
	base := byte(pal.AmigaBase + 16*int(hdr[2]))
	for i, c := range px {
		if c < 16 {
			px[i] = base + c
		}
	}
}
