package dax

import "encoding/binary"

// Image is a decoded picture block: Count frames of Height x Width*8 pixels,
// one byte per pixel (EGA colour 0..15). A masked load (0464:0111 with the
// mask flag) does what the original does: pixels of the mask colour go into
// a separate mask and read as colour 0. Here they are stored as Clear (16)
// and Mask is set to Clear: masked draws skip them, plain draws show black.
//
// Block header: u16 height, u16 width (8-pixel cells), u16 x, u16 y, u8 count,
// 8 unused bytes, then count frames of height*width*4 bytes, two 4-bit
// pixels per byte, high nibble first.
type Image struct {
	Height, Width int // Width in 8-pixel cells
	X, Y          int
	Count         int
	BPP           int // bytes (pixels) per frame
	Data          []byte
	Mask          int // Clear for a masked load, -1 = none
}

// Clear marks a masked-out pixel; the page draws it as colour 0.
const Clear = 16

// NewImage allocates a blank image.
func NewImage(count, width, height int) *Image {
	im := &Image{Height: height, Width: width, Count: count, BPP: height * width * 8, Mask: -1}
	im.Data = make([]byte, count*im.BPP)
	return im
}

// ParseImage decodes a picture block. If masked, maskColour becomes the
// image's transparent colour (the original's "masked load").
func ParseImage(b []byte, masked bool, maskColour byte) *Image {
	if len(b) < 17 {
		return nil
	}
	im := &Image{
		Height: int(binary.LittleEndian.Uint16(b[0:])),
		Width:  int(binary.LittleEndian.Uint16(b[2:])),
		X:      int(binary.LittleEndian.Uint16(b[4:])),
		Y:      int(binary.LittleEndian.Uint16(b[6:])),
		Count:  int(b[8]),
		Mask:   -1,
	}
	if masked {
		im.Mask = Clear
	}
	im.BPP = im.Height * im.Width * 8
	im.Data = make([]byte, im.Count*im.BPP)
	src := b[17:]
	n := im.Count * im.BPP / 2
	if n > len(src) {
		n = len(src)
	}
	for i := 0; i < n; i++ {
		im.Data[i*2] = src[i] >> 4
		im.Data[i*2+1] = src[i] & 15
	}
	if masked {
		clearMask(im.Data, maskColour)
	}
	amigaTag(b[9:], im.Data)
	return im
}

// Frame returns the pixel slice for frame i.
func (im *Image) Frame(i int) []byte { return im.Data[i*im.BPP : (i+1)*im.BPP] }

// Clone deep-copies the image.
func (im *Image) Clone() *Image {
	c := *im
	c.Data = append([]byte(nil), im.Data...)
	return &c
}

// FlipLeftToRight mirrors every frame horizontally.
func (im *Image) FlipLeftToRight() {
	w := im.Width * 8
	for f := 0; f < im.Count; f++ {
		d := im.Frame(f)
		for y := 0; y < im.Height; y++ {
			row := d[y*w : (y+1)*w]
			for i, j := 0, w-1; i < j; i, j = i+1, j-1 {
				row[i], row[j] = row[j], row[i]
			}
		}
	}
}

// SpriteFrame is one frame of an animated sprite block (PIC*/SPRIT*).
type SpriteFrame struct {
	Delay int // ticks (tenths of a second)
	Image *Image
}

// ParseSprite decodes an animation block: u8 frame count, then per frame a
// 21-byte header (u32 delay, u16 height, u16 width cells, u16 x, u16 y, 9
// unused) followed by its pixels. With xor set (PIC*, FINAL*) frames after
// the first are stored XORed against frame 0 and are reconstructed here.
func ParseSprite(b []byte, masked bool, maskColour byte, xor bool) []SpriteFrame {
	if len(b) < 1 {
		return nil
	}
	n := int(b[0])
	var frames []SpriteFrame
	p := 1
	var first []byte
	for f := 0; f < n; f++ {
		if p+21 > len(b) {
			break
		}
		delay := int(binary.LittleEndian.Uint32(b[p:]))
		h := int(binary.LittleEndian.Uint16(b[p+4:]))
		w := int(binary.LittleEndian.Uint16(b[p+6:]))
		x := int(binary.LittleEndian.Uint16(b[p+8:]))
		y := int(binary.LittleEndian.Uint16(b[p+10:]))
		p += 21
		size := h * w * 4
		if p+size > len(b) {
			size = len(b) - p
		}
		raw := append([]byte(nil), b[p:p+size]...)
		p += size
		if f == 0 {
			first = raw
		} else if xor && string(b[p-size-9:p-size-7]) != "AM" { // Amiga frames are imported whole
			for i := range raw {
				if i < len(first) {
					raw[i] ^= first[i]
				}
			}
		}
		im := &Image{Height: h, Width: w, X: x, Y: y, Count: 1, BPP: h * w * 8, Mask: -1}
		im.Data = make([]byte, im.BPP)
		for i := 0; i < len(raw) && i*2+1 < len(im.Data); i++ {
			im.Data[i*2] = raw[i] >> 4
			im.Data[i*2+1] = raw[i] & 15
		}
		if masked {
			im.Mask = Clear
			clearMask(im.Data, maskColour)
		}
		amigaTag(b[p-size-9:], im.Data)
		frames = append(frames, SpriteFrame{Delay: delay, Image: im})
	}
	return frames
}

func clearMask(d []byte, colour byte) {
	for i, p := range d {
		if p == colour {
			d[i] = Clear
		}
	}
}
