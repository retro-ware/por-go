package battle

import (
	"image"
	"image/color"
	"image/draw"
	"strconv"

	"gpu"

	"por/ega"
	"por/engine"
)

// The action bar, XCOM style: one button per menu item along the bottom
// centre showing its icon (else its hotkey, big) at native size; the word
// shows under the button the mouse is on. The bar is drawn in screen
// pixels, its text at the bar's scale.
const (
	face    = 64 // an icon's side: drawn 1:1
	promptH = 12 // the prompt line over the buttons, at scale 1
	labelH  = 14 // the hovered button's word under them, at scale 1
)

type bar struct {
	img   *gpu.Image
	id    string            // what img shows
	rects []image.Rectangle // the buttons on screen, as last drawn
	items []barItem
}

// barItem is one button: its letter, its word, the keys it sends.
type barItem struct {
	Key  byte
	Text string
	Send []uint16
}

// hotkeys are the remaster's letters for the bar's buttons, by word, and
// what the button shows for them (the number keys work too, unshown).
var hotkeys = map[string]struct {
	key   uint16
	label string
}{"Attack": {'f', "F"}, "Guard": {'g', "G"}, "End": {8, "\x1b"}, "Wait": {'p', "P"}, "Cast": {'c', "C"},
	"Quick": {'u', "U"}, "Items": {'i', "I"}, "Turn": {'t', "T"}, "Bandage": {'b', "B"}} // Cancel: Esc, unshown

// leftArrow is the glyph drawn for "\x1b" (Backspace): the game's font has none.
var leftArrow = [8]byte{0x00, 0x10, 0x30, 0x7E, 0x30, 0x10, 0x00, 0x00}

// hotkey is the keys of the button whose hotkey is k (nil: none).
func (b *bar) hotkey(k uint16) []uint16 {
	for i, it := range b.items {
		if h, ok := hotkeys[it.Text]; ok && h.key == k && i < len(b.rects) {
			return it.Send
		}
	}
	return nil
}

// has is the keys of the button named s (nil: none).
func (b *bar) has(s string) []uint16 {
	for i, it := range b.items {
		if it.Text == s && i < len(b.rects) {
			return it.Send
		}
	}
	return nil
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

// shownItems are the menu's items as the bar shows them, XCOM 2 style:
// on the main combat menu Move and View go (a click on the map walks, on
// one's own figure opens the sheet), Aim is Attack, the turn enders
// (actions) sit before End turn (Done, Quit) but Guard, second after
// Attack, and Quick goes last; aiming keeps Attack, Free
// aim and Cancel (Tab cycles the targets; the camera stays put).
func shownItems(m *engine.Menu, actions []engine.MenuItem) (items []barItem, sel int) {
	kind := ""
	for _, it := range m.Items {
		switch it.Text {
		case "Done":
			kind = "main"
		case "Next", "Center":
			kind = "aim"
		}
	}
	sel = -1
	add := func(it barItem, i int) {
		if i == m.Sel {
			sel = len(items)
		}
		items = append(items, it)
	}
	quick, quickAt := barItem{}, -2 // the main menu's Quick goes last, rightmost
	for i, it := range m.Items {
		b := barItem{it.Key, it.Text, []uint16{uint16(it.Key)}}
		switch {
		case kind == "main" && (it.Text == "Move" || it.Text == "View"):
			continue
		case kind == "main" && it.Text == "Quick":
			quick, quickAt = b, i
			continue
		case kind == "main" && it.Text == "Aim":
			b.Text = "Attack"
			add(b, i)
			for _, a := range actions { // Guard right after it
				if a.Text == "Guard" {
					add(barItem{a.Key, a.Text, []uint16{uint16(a.Key)}}, -1)
				}
			}
			continue
		case kind == "main" && it.Text == "Done":
			for _, a := range actions {
				if a.Text != "Guard" {
					add(barItem{a.Key, a.Text, []uint16{uint16(a.Key)}}, -1)
				}
			}
			b.Text, b.Send = "End", []uint16{'D', 'Q'}
		case kind == "aim" && (it.Text == "Next" || it.Text == "Prev" || it.Text == "Center"):
			continue
		case kind == "aim" && it.Text == "Manual":
			b.Text = "Free"
		case kind == "aim" && it.Text == "Exit":
			b.Text = "Cancel"
		}
		add(b, i)
	}
	if quickAt != -2 {
		add(quick, quickAt)
	}
	return items, sel
}

// DrawBar draws menu m as the action bar at scale k, its bottom at bottom,
// in the game's font (glyph).
func (p *Presenter) DrawBar(dst *gpu.Image, glyph func(byte) [8]byte, m *engine.Menu, k, bottom int) {
	b := &p.bar
	items, sel := shownItems(m, p.actions)
	p.aiming = false
	for _, it := range m.Items {
		p.aiming = p.aiming || it.Text == "Next" || it.Text == "Center"
	}
	prompt := m.Prompt
	if p.hint != "" {
		prompt = p.hint
	}
	hover := -1
	cx, cy := gpu.CursorPosition()
	for i, r := range b.rects {
		if i < len(items) && image.Pt(cx, cy).In(r) {
			hover = i
		}
	}
	n, bw := len(items), face+4*k
	w, h := max(n*(bw+4*k), len(prompt)*8*k+8*k, 7*8*k+bw), (promptH+labelH)*k+bw
	id := prompt + "|" + strconv.Itoa(k) + "|" + strconv.Itoa(hover)
	for i, it := range items {
		id += "|" + it.Text
		if i == sel {
			id += "*"
		}
	}
	if b.img == nil || b.img.Bounds() != image.Rect(0, 0, w, h) {
		if b.img != nil {
			b.img.Deallocate()
		}
		b.img, b.id = gpu.NewImage(w, h), ""
	}
	if id != b.id {
		b.id = id
		b.img.WritePixels(barPixels(glyph, prompt, items, sel, hover, w, h, k).Pix)
	}
	x0, y0 := dst.Bounds().Dx()/2-w/2, bottom-h
	op := &gpu.DrawImageOptions{}
	op.GeoM.Translate(float64(x0), float64(y0))
	dst.DrawImage(b.img, op)
	b.rects, b.items = b.rects[:0], items
	for i := range items {
		r := buttonRect(w, n, i, k)
		b.rects = append(b.rects, r.Add(image.Pt(x0, y0)))
	}
}

// buttonRect is button i of n on a bar image w wide at scale k.
func buttonRect(w, n, i, k int) image.Rectangle {
	bw, gap := face+4*k, 4*k
	x := (w-n*(bw+gap)+gap)/2 + i*(bw+gap)
	return image.Rect(x, promptH*k, x+bw, promptH*k+bw)
}

// Click is the keys of the button at screen pixel (x, y), if any.
func (p *Presenter) Click(x, y int) ([]uint16, bool) {
	for i, r := range p.bar.rects {
		if image.Pt(x, y).In(r) {
			return p.bar.items[i].Send, true
		}
	}
	return nil, false
}

// HideBar forgets the buttons (no bar on screen: clicks hit nothing).
func (p *Presenter) HideBar() { p.bar.rects = p.bar.rects[:0] }

// barPixels is the bar's picture at scale k (hover: the button whose word shows, -1 none).
func barPixels(glyph func(byte) [8]byte, prompt string, items []barItem, sel, hover, w, h, k int) *image.RGBA {
	rgb := func(i int) color.RGBA { c := ega.Palette[i]; return color.RGBA{c[0], c[1], c[2], 255} }
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	text := func(s string, x, y, scale int, c color.RGBA) {
		for i := 0; i < len(s); i++ {
			g := glyph(s[i])
			if s[i] == 0x1b {
				g = leftArrow
			}
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
		x := max(0, (w-len(prompt)*8*k)/2)
		fill(image.Rect(x-4*k, 0, x+(len(prompt)*8+4)*k, 10*k).Intersect(im.Bounds()), color.RGBA{0, 0, 0, 0xB0}) // translucent black: the text reads over any wall
		text(prompt, x, k, k, rgb(15))
	}
	for i, it := range items {
		r := buttonRect(w, len(items), i, k)
		edge := rgb(8)
		if i == sel || i == hover {
			edge = color.RGBA{0xFF, 0xCC, 0x33, 0xFF}
		}
		fill(r, edge)
		in := r.Inset(2 * k) // face pixels square: the icon 1:1
		fill(in, color.RGBA{0x18, 0x20, 0x38, 0xFF})
		if ic := iconPicture(it.Text); ic != nil {
			draw.Draw(im, in, ic, ic.Bounds().Min, draw.Over)
		} else {
			text(string(it.Key), in.Min.X+(in.Dx()-16*k)/2, in.Min.Y+(in.Dy()-16*k)/2, 2*k, rgb(15))
		}
		if h, ok := hotkeys[it.Text]; ok { // its hotkey, bottom right
			lw := len(h.label) * 8 * k
			at := image.Pt(in.Max.X-lw-2*k, in.Max.Y-10*k)
			draw.Draw(im, image.Rect(at.X-k, at.Y-k, at.X+lw+k, at.Y+9*k), image.NewUniform(color.RGBA{0, 0, 0, 0xB0}), image.Point{}, draw.Over) // darkens the icon under it
			text(h.label, at.X, at.Y, k, rgb(15))
		}
		if i != hover {
			continue
		}
		word := it.Text
		wx := min(max(0, r.Min.X+(r.Dx()-len(word)*8*k)/2), w-len(word)*8*k)
		fill(image.Rect(wx-3*k, r.Max.Y+3*k, wx+(len(word)*8+3)*k, r.Max.Y+13*k).Intersect(im.Bounds()), color.RGBA{0, 0, 0, 0xB0}) // as the prompt's
		text(word, wx, r.Max.Y+4*k, k, rgb(10))
	}
	return im
}
