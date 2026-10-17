package battle

import (
	"image"
	"image/color"

	"gpu"

	"por/ega"
	"por/engine"
)

// The action bar, XCOM style: one button per menu item along the bottom
// centre, its hotkey big on the button and its word under it.
// ponytail: the hotkey letter is the icon; draw real icon art when there is some.
const (
	button  = 32 // a button's side at scale 1
	slot    = 48 // the width each item takes
	barH    = button + 4 + 8 + 4
	promptH = 12 // the prompt line over the buttons
)

type bar struct {
	img   *gpu.Image
	id    string            // what img shows
	rects []image.Rectangle // the buttons on screen, as last drawn
	keys  []byte
}

// DrawBar draws menu m as the action bar at scale k, its bottom at bottom,
// in the game's font (glyph).
func (p *Presenter) DrawBar(dst *gpu.Image, glyph func(byte) [8]byte, m *engine.Menu, k, bottom int) {
	b := &p.bar
	w, h := len(m.Items)*slot, barH+promptH
	id := m.Prompt
	for i, it := range m.Items {
		id += "|" + it.Text
		if i == m.Sel {
			id += "*"
		}
	}
	if b.img == nil || b.img.Bounds().Dx() != w {
		if b.img != nil {
			b.img.Deallocate()
		}
		b.img, b.id = gpu.NewImage(w, h), ""
	}
	if id != b.id {
		b.id = id
		b.img.WritePixels(barPixels(glyph, m, w, h).Pix)
	}
	x0, y0 := dst.Bounds().Dx()/2-w*k/2, bottom-h*k
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(float64(k), float64(k))
	op.GeoM.Translate(float64(x0), float64(y0))
	dst.DrawImage(b.img, op)
	b.rects, b.keys = b.rects[:0], b.keys[:0]
	for i, it := range m.Items {
		r := image.Rect(i*slot+(slot-button)/2, promptH, i*slot+(slot+button)/2, promptH+button)
		b.rects = append(b.rects, image.Rectangle{r.Min.Mul(k), r.Max.Mul(k)}.Add(image.Pt(x0, y0)))
		b.keys = append(b.keys, it.Key)
	}
}

// Click is the hotkey of the button at screen pixel (x, y), if any.
func (p *Presenter) Click(x, y int) (byte, bool) {
	for i, r := range p.bar.rects {
		if image.Pt(x, y).In(r) {
			return p.bar.keys[i], true
		}
	}
	return 0, false
}

// HideBar forgets the buttons (no bar on screen: clicks hit nothing).
func (p *Presenter) HideBar() { p.bar.rects = p.bar.rects[:0] }

func barPixels(glyph func(byte) [8]byte, m *engine.Menu, w, h int) *image.RGBA {
	rgb := func(i int) color.RGBA { c := ega.Palette[i]; return color.RGBA{c[0], c[1], c[2], 255} }
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	text := func(s string, x, y, scale int, c color.RGBA) {
		for i := 0; i < len(s); i++ {
			g := glyph(s[i])
			for gy := range 8 {
				for gx := range 8 {
					if g[gy]&(0x80>>gx) == 0 {
						continue
					}
					for d := range scale * scale {
						im.SetRGBA(x+(i*8+gx)*scale+d%scale, y+gy*scale+d/scale, c)
					}
				}
			}
		}
	}
	fill := func(r image.Rectangle, c color.RGBA) {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				im.SetRGBA(x, y, c)
			}
		}
	}
	if m.Prompt != "" {
		text(m.Prompt, max(0, (w-len(m.Prompt)*8)/2), 0, 1, rgb(15))
	}
	for i, it := range m.Items {
		r := image.Rect(i*slot+(slot-button)/2, promptH, i*slot+(slot+button)/2, promptH+button)
		edge := rgb(8)
		if i == m.Sel {
			edge = color.RGBA{0xFF, 0xCC, 0x33, 0xFF}
		}
		fill(r, edge)
		fill(r.Inset(2), color.RGBA{0x18, 0x20, 0x38, 0xFF})
		text(string(it.Key), r.Min.X+(button-16)/2, r.Min.Y+(button-16)/2, 2, rgb(15))
		word := it.Text
		if len(word) > slot/8 {
			word = word[:slot/8]
		}
		text(word, i*slot+(slot-len(word)*8)/2, r.Max.Y+4, 1, rgb(10))
	}
	return im
}
