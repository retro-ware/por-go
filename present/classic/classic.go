// Package classic shows the game as the original drew it: the 320x200 EGA
// screen, as large as fits the window at 4:3 like a CRT, with the engine's
// overlays (journal scans) at the window's own resolution.
package classic

import (
	"image"
	"image/color"
	"math"

	"gpu"

	"por/ega"
)

// Rect is where the picture lands on the window, in device pixels.
type Rect struct{ X, Y, W, H float64 }

// Note is a toast written into the top of the picture over the game's own
// drawing, in the game's current font.
type Note struct {
	Text  string
	Show  bool
	Glyph func(byte) [8]byte
}

// Presenter converts the screen to an image (only when it changed) and
// draws it. Its zero value is ready.
type Presenter struct {
	img *gpu.Image
	buf []byte

	// the frame last converted into buf: an unchanged one is not converted
	// or uploaded again
	last    [ega.W * ega.H]uint8
	lastPal [16]uint8
	lastSat float64
	lut     [256]color.RGBA // colour of each index under lastPal
	drawn   bool
	noted   bool // buf holds a note

	overlays []ega.Overlay                 // the engine's, copied each frame
	scans    map[image.Image]*gpu.Image // their GPU copies
}

// Draw shows scr on dst and returns where its picture went.
func (p *Presenter) Draw(dst *gpu.Image, scr *ega.Screen, note Note) Rect {
	p.Frame(scr, note)
	dst.Fill(color.Black)

	// The picture: as large as fits, 4:3 like a CRT.
	b := dst.Bounds()
	W, H := float64(b.Dx()), float64(b.Dy())
	scale := math.Min(W/4, H/3)
	pw, ph := scale*4, scale*3
	pic := Rect{(W - pw) / 2, (H - ph) / 2, pw, ph}
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(pw/ega.W, ph/ega.H)
	op.GeoM.Translate(pic.X, pic.Y)
	dst.DrawImage(p.img, op)
	p.drawOverlays(dst, pic)
	return pic
}

// Frame converts scr into a 320x200 image (only when it changed) and
// returns it; the remaster crops its windows from it. The image is reused.
func (p *Presenter) Frame(scr *ega.Screen, note Note) *gpu.Image {
	if p.img == nil {
		p.img, p.buf = gpu.NewImage(ega.W, ega.H), make([]byte, ega.W*ega.H*4)
	}
	scr.Lock()
	vis := scr.Visible()
	palChanged := scr.Pal != p.lastPal || ega.AmigaSaturation != p.lastSat
	upload := !p.drawn || palChanged || note.Show || p.noted || *vis != p.last
	if upload {
		if !p.drawn || palChanged {
			p.lastPal, p.lastSat = scr.Pal, ega.AmigaSaturation
			for i := range p.lut {
				p.lut[i] = scr.RGBA(uint8(i))
			}
		}
		p.last = *vis
		for i, c := range vis {
			rgb := p.lut[c]
			p.buf[i*4], p.buf[i*4+1], p.buf[i*4+2], p.buf[i*4+3] = rgb.R, rgb.G, rgb.B, 255
		}
		if note.Show {
			p.drawNote(note)
		}
		p.drawn, p.noted = true, note.Show
	}
	p.overlays = append(p.overlays[:0], scr.Overlays...)
	scr.Unlock()
	if upload {
		p.img.WritePixels(p.buf)
	}
	return p.img
}

// drawOverlays draws the engine's overlays (journal scans) at the window's
// own resolution over the picture at pic.
func (p *Presenter) drawOverlays(dst *gpu.Image, pic Rect) {
	pw, ph := pic.W, pic.H
	for img, im := range p.scans { // drop the copies of scans no longer shown
		keep := false
		for _, o := range p.overlays {
			keep = keep || o.Img == img
		}
		if !keep {
			im.Deallocate()
			delete(p.scans, img)
		}
	}
	for _, o := range p.overlays {
		im := p.scans[o.Img]
		if im == nil {
			if p.scans == nil {
				p.scans = map[image.Image]*gpu.Image{}
			}
			im = gpu.NewImageFromImage(o.Img)
			p.scans[o.Img] = im
		}
		b := o.Part()
		im = im.SubImage(b).(*gpu.Image) // drawn from its own top left
		op := &gpu.DrawImageOptions{Filter: gpu.FilterLinear}
		op.GeoM.Scale(float64(o.At.Dx())*pw/ega.W/float64(b.Dx()), float64(o.At.Dy())*ph/ega.H/float64(b.Dy()))
		op.GeoM.Translate(pic.X+float64(o.At.Min.X)*pw/ega.W, pic.Y+float64(o.At.Min.Y)*ph/ega.H)
		dst.DrawImage(im, op)
	}
}

// drawNote writes the note into the top of the picture, white on blue, over
// what the game drew (the screen stays untouched).
func (p *Presenter) drawNote(n Note) {
	s := " " + n.Text + " "
	if len(s) > 40 {
		s = s[:40]
	}
	x0 := (40 - len(s)) * 4
	bg, fg := p.lut[1], p.lut[15]
	for i := 0; i < len(s); i++ {
		gl := n.Glyph(s[i])
		for r := 0; r < 8; r++ {
			for b := 0; b < 8; b++ {
				c := bg
				if gl[r]&(0x80>>b) != 0 {
					c = fg
				}
				o := ((r+4)*ega.W + x0 + i*8 + b) * 4
				p.buf[o], p.buf[o+1], p.buf[o+2] = c.R, c.G, c.B
			}
		}
	}
}
