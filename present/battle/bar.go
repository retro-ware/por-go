package battle

import (
	"image"
	"image/color"
	"image/draw"
	"slices"
	"strconv"

	xdraw "golang.org/x/image/draw"

	"gpu"

	"por/engine"
	"por/pal"
)

// The action bar, XCOM style: one button per menu item along the bottom
// centre showing its icon (else its hotkey, big) with its letter, scaled
// as one picture with the bar (64px, its native size, at scale 2); the word
// shows under the button the mouse is on. The bar is drawn in screen
// pixels, its text at the bar's scale. A menu's question shows on the
// bottom rows instead (remaster fight.go).
const (
	face    = 64 // an icon's side, and the button face's at scale 2
	promptH = 2  // a margin over the buttons, at scale 1
	labelH  = 14 // the hovered button's word under them, at scale 1
)

type bar struct {
	img   *gpu.Image
	id    string            // what img shows
	rects []image.Rectangle // the buttons on screen, as last drawn
	items []barItem
}

// barItem is one button: its letter, its word, the action it takes.
type barItem struct {
	Key  byte
	Text string
	Kind engine.ActKind
}

// hotkeys are the remaster's letters for the bar's buttons, by word, and
// what the button shows for them (the number keys work too, unshown).
var hotkeys = map[string]struct {
	key   uint16
	label string
}{"Attack": {'f', "F"}, "Guard": {'g', "G"}, "End": {8, "\x1b"}, "Wait": {'p', "P"}, "Cast": {'c', "C"},
	"Quick": {'u', "U"}, "Items": {'i', "I"}, "Turn": {'t', "T"}, "Bandage": {'b', "B"}, "Flee": {'r', "R"}, "Yes": {'y', "Y"}, "No": {'n', "N"}} // Cancel: Esc, unshown

// leftArrow is the glyph drawn for "\x1b" (Backspace): the game's font has none.
var leftArrow = [8]byte{0x00, 0x10, 0x30, 0x7E, 0x30, 0x10, 0x00, 0x00}

// actWords are the bar's words for the turn's actions.
var actWords = map[engine.ActKind]string{engine.ActAttack: "Attack", engine.ActGuard: "Guard", engine.ActItems: "Items",
	engine.ActCast: "Cast", engine.ActTurn: "Turn", engine.ActBandage: "Bandage", engine.ActWait: "Wait", engine.ActEnd: "End", engine.ActQuick: "Quick", engine.ActFlee: "Flee"}

// hotkey is the button whose hotkey is k (nil: none).
func (b *bar) hotkey(k uint16) *barItem {
	for i, it := range b.items {
		if h, ok := hotkeys[it.Text]; ok && h.key == k && i < len(b.rects) {
			return &b.items[i]
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

// shownItems are the buttons for b, XCOM 2 style: a question's Yes and
// No; aiming only Cancel (Tab cycles the targets, a click or Enter takes
// one); else the turn's actions in the menu's order (a click on the map
// walks or attacks, Attack aims first).
func shownItems(b *engine.Battle) (items []barItem) {
	switch t := b.Turn; {
	case b.Ask != "":
		return []barItem{{'Y', "Yes", engine.ActYes}, {'N', "No", engine.ActClose}}
	case t == nil:
	case t.Aim:
		return []barItem{{'E', "Cancel", engine.ActClose}}
	default:
		for _, k := range t.Can {
			w := actWords[k]
			items = append(items, barItem{w[0], w, k})
		}
	}
	return items
}

// DrawBar draws b's buttons as the action bar at scale k, its bottom at bottom,
// in the game's font (glyph).
func (p *Presenter) DrawBar(dst *gpu.Image, glyph func(byte) [8]byte, b *engine.Battle, k, bottom int) {
	br := &p.bar
	items := shownItems(b)
	if len(items) == 0 || p.Picking {
		p.HideBar()
		return
	}
	hover := -1
	cx, cy := gpu.CursorPosition()
	for i, r := range br.rects {
		if i < len(items) && image.Pt(cx, cy).In(r) {
			hover = i
		}
	}
	n, bw := len(items), face*k/2+4*k
	w, h := max(n*(bw+4*k), 7*8*k+bw), (promptH+labelH)*k+bw
	id := strconv.Itoa(k) + "|" + strconv.Itoa(hover)
	for _, it := range items {
		id += "|" + it.Text
	}
	if br.img == nil || br.img.Bounds() != image.Rect(0, 0, w, h) {
		if br.img != nil {
			br.img.Deallocate()
		}
		br.img, br.id = gpu.NewImage(w, h), ""
	}
	if id != br.id {
		br.id = id
		br.img.WritePixels(barPixels(glyph, items, hover, w, h, k).Pix)
	}
	x0, y0 := dst.Bounds().Dx()/2-w/2, bottom-h
	op := &gpu.DrawImageOptions{}
	op.GeoM.Translate(float64(x0), float64(y0))
	dst.DrawImage(br.img, op)
	br.rects, br.items = br.rects[:0], items
	for i := range items {
		r := buttonRect(w, n, i, k)
		br.rects = append(br.rects, r.Add(image.Pt(x0, y0)))
	}
}

// buttonRect is button i of n on a bar image w wide at scale k.
func buttonRect(w, n, i, k int) image.Rectangle {
	bw, gap := face*k/2+4*k, 4*k
	x := (w-n*(bw+gap)+gap)/2 + i*(bw+gap)
	return image.Rect(x, promptH*k, x+bw, promptH*k+bw)
}

// Click takes a click at screen pixel (x, y): a button of the bar, else
// a square of the map (aiming: the aim; a foe of the turn's targets: the
// attack; a square in reach: the walk). It reports whether it was taken.
func (p *Presenter) Click(x, y int) bool {
	for i, r := range p.bar.rects {
		if image.Pt(x, y).In(r) {
			p.press(p.bar.items[i])
			return true
		}
	}
	b := p.b
	sx, sy, ok := p.Pick(x, y)
	if b == nil || b.Turn == nil || b.Ask != "" || p.Picking || !ok {
		return false
	}
	sq := image.Pt(sx, sy)
	if b.Turn.Aim {
		p.act(engine.Action{Kind: engine.ActAim, X: sx, Y: sy})
		return true
	}
	for _, u := range b.Units {
		if slices.Contains(b.Targets, u.ID) && slices.Contains(u.Squares, sq) {
			p.act(engine.Action{Kind: engine.ActAttack, N: u.ID})
			return true
		}
	}
	if slices.Contains(b.Reach, sq) {
		p.act(engine.Action{Kind: engine.ActWalk, X: sx, Y: sy})
		return true
	}
	return false
}

// Point follows the pointer at screen pixel (x, y): aiming, the aim goes
// to the square under it.
func (p *Presenter) Point(x, y int) {
	b := p.b
	if b == nil || b.Turn == nil || !b.Turn.Aim || p.overBar(x, y) {
		return
	}
	if sx, sy, ok := p.Pick(x, y); ok && image.Pt(sx, sy) != p.pointed {
		p.pointed = image.Pt(sx, sy)
		p.act(engine.Action{Kind: engine.ActPoint, X: sx, Y: sy})
	}
}

// press takes button it: Cast opens the spell list, the others act.
func (p *Presenter) press(it barItem) {
	if it.Kind == engine.ActCast {
		p.Picking = true
		return
	}
	p.act(engine.Action{Kind: it.Kind})
}

// act queues a for Actions.
func (p *Presenter) act(a engine.Action) { p.out = append(p.out, a) }

// Actions hands over the actions queued since the last call.
func (p *Presenter) Actions() []engine.Action {
	a := p.out
	p.out = nil
	return a
}

// HideBar forgets the buttons (no bar on screen: clicks hit nothing).
func (p *Presenter) HideBar() { p.bar.rects = p.bar.rects[:0] }

// barPixels is the bar's picture at scale k (hover: the button whose word shows, -1 none).
func barPixels(glyph func(byte) [8]byte, items []barItem, hover, w, h, k int) *image.RGBA {
	rgb := pal.EGA
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	textOn := func(im *image.RGBA, s string, x, y, scale int, c color.RGBA) {
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
	text := func(s string, x, y, scale int, c color.RGBA) { textOn(im, s, x, y, scale, c) }
	fill := func(r image.Rectangle, c color.RGBA) {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				im.SetRGBA(x, y, c)
			}
		}
	}
	for i, it := range items {
		r := buttonRect(w, len(items), i, k)
		edge := rgb(8)
		if i == hover {
			edge = color.RGBA{0xFF, 0xCC, 0x33, 0xFF}
		}
		fill(r, edge)
		in := r.Inset(2 * k)
		// the face at the icon's size: the icon 1:1, the letters on it in
		// proportion; then scaled as one to the button (so their relation holds)
		f := image.NewRGBA(image.Rect(0, 0, face, face))
		draw.Draw(f, f.Bounds(), image.NewUniform(color.RGBA{0x18, 0x20, 0x38, 0xFF}), image.Point{}, draw.Src)
		if ic := iconPicture(it.Text); ic != nil {
			draw.Draw(f, f.Bounds(), ic, ic.Bounds().Min, draw.Over)
		} else {
			textOn(f, string(it.Key), (face-32)/2, (face-32)/2, 4, rgb(15))
		}
		if h, ok := hotkeys[it.Text]; ok { // its hotkey, bottom right
			const s = 2 // 16px letters on the 64px face
			lw := len(h.label) * 8 * s
			at := image.Pt(face-lw-2*s, face-10*s)
			draw.Draw(f, image.Rect(at.X-s, at.Y-s, at.X+lw+s, at.Y+9*s), image.NewUniform(color.RGBA{0, 0, 0, 0xB0}), image.Point{}, draw.Over) // darkens the icon under it
			textOn(f, h.label, at.X, at.Y, s, rgb(15))
		}
		var sc xdraw.Scaler = xdraw.NearestNeighbor // pixel art stays crisp growing
		if in.Dx() < face {
			sc = xdraw.ApproxBiLinear
		}
		sc.Scale(im, in, f, f.Bounds(), draw.Src, nil)
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
