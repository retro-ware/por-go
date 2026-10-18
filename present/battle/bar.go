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
	texts []string
}

// has is the key of the button named s (0: none).
func (b *bar) has(s string) byte {
	for i, t := range b.texts {
		if t == s && i < len(b.rects) {
			return b.keys[i]
		}
	}
	return 0
}

// overBar tells whether screen pixel (x, y) is on the bar.
func (p *Presenter) overBar(x, y int) bool {
	for _, r := range p.bar.rects {
		if image.Pt(x, y).In(image.Rect(r.Min.X-r.Dx()/4, r.Min.Y-r.Dy()/2, r.Max.X+r.Dx()/4, r.Max.Y+r.Dy()/2)) {
			return true
		}
	}
	return false
}

// shownItems are the menu's items as the bar shows them: on the main
// combat menu Move goes (a click on the map walks) and Aim is Attack.
func shownItems(m *engine.Menu) (items []engine.MenuItem, sel int) {
	main := false
	for _, it := range m.Items {
		main = main || it.Text == "Done"
	}
	sel = -1
	for i, it := range m.Items {
		switch {
		case main && it.Text == "Move":
			continue
		case main && it.Text == "Aim":
			it.Text = "Attack"
		}
		if i == m.Sel {
			sel = len(items)
		}
		items = append(items, it)
	}
	return items, sel
}

// DrawBar draws menu m as the action bar at scale k, its bottom at bottom,
// in the game's font (glyph).
func (p *Presenter) DrawBar(dst *gpu.Image, glyph func(byte) [8]byte, m *engine.Menu, k, bottom int) {
	b := &p.bar
	items, sel := shownItems(m)
	prompt := m.Prompt
	if prompt == "" {
		prompt = p.hint
	}
	w, h := max(len(items)*slot, len(prompt)*8), barH+promptH
	id := prompt
	for i, it := range items {
		id += "|" + it.Text
		if i == sel {
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
		b.img.WritePixels(barPixels(glyph, prompt, items, sel, w, h).Pix)
	}
	x0, y0 := dst.Bounds().Dx()/2-w*k/2, bottom-h*k
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(float64(k), float64(k))
	op.GeoM.Translate(float64(x0), float64(y0))
	dst.DrawImage(b.img, op)
	b.rects, b.keys, b.texts = b.rects[:0], b.keys[:0], b.texts[:0]
	x1 := (w - len(items)*slot) / 2
	for i, it := range items {
		r := image.Rect(x1+i*slot+(slot-button)/2, promptH, i*slot+(slot+button)/2, promptH+button)
		b.rects = append(b.rects, image.Rectangle{r.Min.Mul(k), r.Max.Mul(k)}.Add(image.Pt(x0, y0)))
		b.keys, b.texts = append(b.keys, it.Key), append(b.texts, it.Text)
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

func barPixels(glyph func(byte) [8]byte, prompt string, items []engine.MenuItem, sel, w, h int) *image.RGBA {
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
	if prompt != "" {
		text(prompt, max(0, (w-len(prompt)*8)/2), 0, 1, rgb(15))
	}
	x1 := (w - len(items)*slot) / 2
	for i, it := range items {
		r := image.Rect(x1+i*slot+(slot-button)/2, promptH, x1+i*slot+(slot+button)/2, promptH+button)
		edge := rgb(8)
		if i == sel {
			edge = color.RGBA{0xFF, 0xCC, 0x33, 0xFF}
		}
		fill(r, edge)
		fill(r.Inset(2), color.RGBA{0x18, 0x20, 0x38, 0xFF})
		text(string(it.Key), r.Min.X+(button-16)/2, r.Min.Y+(button-16)/2, 2, rgb(15))
		if i < 10 {
			text(string(rune('0'+(i+1)%10)), r.Min.X+3, r.Min.Y+3, 1, rgb(7)) // its number key
		}
		word := it.Text
		if len(word) > slot/8 {
			word = word[:slot/8]
		}
		text(word, x1+i*slot+(slot-len(word)*8)/2, r.Max.Y+4, 1, rgb(10))
	}
	return im
}
