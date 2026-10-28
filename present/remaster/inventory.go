package remaster

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"
	"strings"
	"time"

	"gpu"

	"blobber/ui"

	"por/ega"
	"por/engine"
	"por/present/classic"
)

// The inventory screen (F1-F8, Items): the member's Eye of the Beholder
// paper doll on the left, the party inventory on the right, in the
// original's font, colours and box borders. Everything is drawn 1:1 on one
// canvas that is scaled whole to the window, so all pixels are the same
// size. It draws engine.View.Inventory in both modes and only sends keys:
// KeyItem picks a line, the menu's own keys act on it; the menu row and
// the boxes the game shows over the list (Look, Drop) are the original's.

var (
	//go:embed doll/doll.png
	dollPNG []byte
	//go:embed items/atlas.png
	atlasPNG []byte
	//go:embed items/items.rec
	itemsRec string
)

// Layout, in canvas pixels. The canvas is at least invW x invH; the list
// takes what the window gives beyond that.
const (
	invW, invH = 400, 250
	leftCells  = 17 // the doll box's width in cells, with its border
	rowH       = 20 // a party inventory line: a whole icon high
	listTop    = 24 // the first line, under the box's title
	atlasCell  = 32
	atlasCols  = 24
	wheelPause = 60 * time.Millisecond // the quickest the wheel scrolls a line
)

// outer is the items screen's own border (the game's DrawBorder round the
// screen): the box tiles come from it.
var outer = engine.OuterBox.Framed()

func col(i int) color.RGBA {
	c := ega.Palette[i]
	return color.RGBA{c[0], c[1], c[2], 255}
}

var (
	green, white, cyan = col(10), col(15), col(11)
	blue, gray, dark   = col(1), col(7), col(8)
	red                = col(12)
)

// dollBoxes are the doll's slot boxes (EOB's frames, 18x18 with the frame;
// the rings' 10x10) in doll pixels, by engine doll slot.
var dollBoxes = [engine.DollSlots]image.Rectangle{
	engine.DollWeapon:    image.Rect(13, 63, 31, 81),
	engine.DollShield:    image.Rect(61, 63, 79, 81),
	engine.DollArmor:     image.Rect(7, 23, 25, 41),
	engine.DollGauntlets: image.Rect(8, 43, 26, 61),
	engine.DollHelm:      image.Rect(81, 2, 99, 20),
	engine.DollBelt:      image.Rect(83, 41, 101, 59),
	engine.DollRobe:      image.Rect(83, 59, 101, 77),
	engine.DollCloak:     image.Rect(70, 22, 88, 40),
	engine.DollBoots:     image.Rect(60, 84, 78, 102),
	engine.DollRing1:     image.Rect(11, 83, 21, 93),
	engine.DollRing2:     image.Rect(23, 83, 33, 93),
	engine.DollArrows:    image.Rect(8, 3, 26, 21),
	engine.DollBolts:     image.Rect(83, 77, 101, 95),
	engine.DollBracers:   image.Rect(4, 95, 22, 113),
}

var slotNames = [engine.DollSlots]string{"Weapon", "Shield", "Armour", "Gauntlets", "Helm", "Belt", "Robe", "Cloak",
	"Boots", "Ring", "Ring", "Arrows", "Bolts", "Bracers"}

// inventory is the screen's state between frames.
type inventory struct {
	doll, atlas image.Image
	icons       map[string]int // items.rec: icon key → atlas cell
	px          *image.RGBA    // the canvas, drawn on the CPU
	img, tip    *gpu.Image     // the canvas and the tooltip on the GPU
	glyph       func(byte) [8]byte

	shown  bool            // the last frame was the inventory
	busy   bool            // the game asks something over it (no item menu)
	k      int             // canvas scale
	at     image.Point     // canvas top left on screen
	w, h   int             // canvas size
	top    int             // first party inventory line shown
	rows   int             // party inventory lines shown
	n      int             // party inventory lines
	lines  []hit           // the items' rectangles (canvas)
	bar    image.Rectangle // the menu row (40 cells)
	scroll image.Rectangle // the scroll bar
	dollAt image.Rectangle
	arrows [2]image.Rectangle
	hover  image.Point // the pointer (canvas)
	member int         // View.Inventory.Member
	count  int         // party members
	seen   int         // the Sel the list last scrolled to

	held      bool        // the left button was down last frame
	press     *hit        // the item the button went down on
	pressAt   image.Point // where
	dragging  bool        // it moved off: a drag
	scrolling bool        // the button went down on the scroll bar
	wheel     float64     // wheel movement not yet a line
	wheeled   time.Time   // the last line the wheel scrolled
}

type hit struct {
	r     image.Rectangle
	item  engine.InvItem
	doll  bool // on the doll (readied)
	whole bool // its name shows whole
}

func decodePNG(b []byte) image.Image {
	im, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return im
}

// readIcons parses items.rec: records of "field: value" lines between
// blank lines, comments starting with #.
func readIcons(rec string) map[string]int {
	m := map[string]int{}
	name := ""
	s := bufio.NewScanner(strings.NewReader(rec))
	for s.Scan() {
		k, v, ok := strings.Cut(s.Text(), ":")
		v = strings.TrimSpace(v)
		switch {
		case !ok || strings.HasPrefix(k, "#"):
		case k == "internal_name":
			name = v
		case k == "icon":
			if n, err := strconv.Atoi(v); err == nil && name != "" {
				m[name] = n
			}
		}
	}
	return m
}

func (inv *inventory) load() {
	if inv.doll != nil {
		return
	}
	inv.doll, inv.atlas = decodePNG(dollPNG), decodePNG(atlasPNG)
	inv.icons = readIcons(itemsRec)
}

// icon is the atlas cell of an item: the key "TT:word" for one of its
// name words (or a word of one), else "TT" for its type; -1 for none.
func (inv *inventory) icon(it *engine.InvItem) int {
	t := fmt.Sprintf("%02X", it.Type)
	for _, w := range it.Words {
		if n, ok := inv.icons[t+":"+w]; ok {
			return n
		}
		for _, f := range strings.Fields(w) {
			if n, ok := inv.icons[t+":"+f]; ok {
				return n
			}
		}
	}
	if n, ok := inv.icons[t]; ok {
		return n
	}
	return -1
}

// drawIcon draws the middle of the item's atlas cell, unscaled, in r (the
// icons are about 16 pixels in the middle of their 32 pixel cells).
func (inv *inventory) drawIcon(it *engine.InvItem, r image.Rectangle) {
	n := inv.icon(it)
	if n < 0 {
		inv.text("?", r.Min.X+r.Dx()/2-4, r.Min.Y+r.Dy()/2-4, gray)
		return
	}
	mid := image.Pt(n%atlasCols*atlasCell+atlasCell/2, n/atlasCols*atlasCell+atlasCell/2)
	draw.Draw(inv.px, r, inv.atlas, mid.Sub(image.Pt(r.Dx()/2, r.Dy()/2)), draw.Over)
}

// text draws s in the game's font at (x, y).
func (inv *inventory) text(s string, x, y int, c color.RGBA) { drawText(inv.px, inv.glyph, s, x, y, c) }

func drawText(dst *image.RGBA, glyph func(byte) [8]byte, s string, x, y int, c color.RGBA) {
	for i := 0; i < len(s); i++ {
		g := glyph(s[i])
		for gy := 0; gy < 8; gy++ {
			for b := 0; b < 8; b++ {
				if g[gy]&(0x80>>b) != 0 {
					dst.SetRGBA(x+i*8+b, y+gy, c)
				}
			}
		}
	}
}

func (inv *inventory) fill(r image.Rectangle, c color.RGBA) {
	draw.Draw(inv.px, r, image.NewUniform(c), image.Point{}, draw.Src)
}

// outline is a one pixel frame around r.
func (inv *inventory) outline(r image.Rectangle, c color.RGBA) {
	inv.fill(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), c)
	inv.fill(image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), c)
	inv.fill(image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), c)
	inv.fill(image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), c)
}

// upTo is s in at most n characters.
func upTo(s string, n int) string { return s[:min(len(s), max(0, n))] }

// Inventory draws the inventory screen when the game shows it, and
// reports whether it did.
func (p *Presenter) Inventory(dst *gpu.Image, scr *ega.Screen, note classic.Note, v engine.View) bool {
	inv := &p.inv
	inv.shown = false
	in := v.Inventory
	if in == nil {
		return false
	}
	inv.load()
	inv.shown, inv.member, inv.count, inv.glyph = true, in.Member, len(v.Party), note.Glyph
	inv.busy = v.Menu == nil || !hasKey(v.Menu, 'E') || in.Over != (engine.CellRect{})
	frame := p.Classic.Frame(scr, note)

	b := dst.Bounds()
	inv.k = max(1, min(b.Dx()/invW, b.Dy()/invH))
	cw, ch := max(invW, b.Dx()/inv.k)/8, max(invH, b.Dy()/inv.k)/8
	if w, h := cw*8, ch*8; w != inv.w || h != inv.h {
		if inv.img != nil {
			inv.img.Deallocate()
		}
		inv.w, inv.h, inv.px, inv.img = w, h, image.NewRGBA(image.Rect(0, 0, w, h)), gpu.NewImage(w, h)
	}
	inv.fill(inv.px.Rect, color.RGBA{A: 255})
	inv.lines = inv.lines[:0]
	bottom := (ch - 2) * 8 // the boxes' inner bottom

	// the member: name, portrait between the arrows, hit points
	lx, lw := 8, (leftCells-2)*8
	oy := max(0, (bottom-84-inv.doll.Bounds().Dy())/3)
	if len(v.Party) > in.Member && in.Member >= 0 {
		oy = max(oy, 10*strings.Count(v.Party[in.Member].Class, "/")+12) // under the classes
	} // the portrait and doll down the box, a little above its middle
	if in.Member >= 0 && in.Member < len(v.Party) {
		m := v.Party[in.Member]
		inv.text(m.Name, lx+(lw-8*len(m.Name))/2, 8, white)
		for i, cl := range strings.Split(m.Class, "/") { // a class a line: three don't fit one
			inv.text(cl, lx+(lw-8*len(cl))/2, 18+10*i, green)
		}
		if f := p.face(m.Head); f != nil {
			r := image.Rect(lx+(lw-ui.PortraitSize)/2, 20+oy, lx+(lw+ui.PortraitSize)/2, 20+oy+ui.PortraitSize)
			draw.Draw(inv.px, r, f, f.Bounds().Min, draw.Src)
		}
		hp := "HP " + strconv.Itoa(m.HP) + "/" + strconv.Itoa(m.MaxHP)
		inv.text(hp, lx+(lw-8*len(hp))/2, 70+oy, green)
	}
	ab := image.Rect(-8, -8, 24, 16) // "<-" and "->" with room round them for the pointer
	inv.arrows = [2]image.Rectangle{ab.Add(image.Pt(lx+4, 38+oy)), ab.Add(image.Pt(lx+lw-20, 38+oy))}
	if inv.count > 1 {
		inv.text("<-", lx+4, 38+oy, green)
		inv.text("->", lx+lw-20, 38+oy, green)
	}

	// the doll and the readied items
	db := inv.doll.Bounds()
	inv.dollAt = db.Add(image.Pt(lx+(lw-db.Dx())/2, 84+oy))
	draw.Draw(inv.px, inv.dollAt, inv.doll, db.Min, draw.Src)
	for s, it := range in.Doll {
		box := dollBoxes[s].Add(inv.dollAt.Min)
		if it != nil {
			inv.drawIcon(it, box.Inset(1))
			inv.lines = append(inv.lines, hit{r: box, item: *it, doll: true})
		}
		switch {
		case it != nil && it.Index == in.Sel:
			inv.outline(box, col(14)) // yellow: the boxes are white-edged
		case inv.hover.In(box) && !inv.busy:
			inv.outline(box, cyan)
		}
	}

	// the party inventory: its title and fill, the lines, the scroll bar
	rx, rr := leftCells*8+8, (cw-1)*8
	inv.text("Party Items", rx, 8, white)
	fillc := green
	if len(in.Pool) >= in.Cap {
		fillc = red
	}
	s := strconv.Itoa(len(in.Pool)) + "/" + strconv.Itoa(in.Cap)
	inv.text(s, rr-8*len(s), 8, fillc)
	inv.n, inv.rows = len(in.Pool), max(1, (bottom-listTop)/rowH)
	for i, it := range in.Pool { // keep a newly highlighted line in sight (the wheel scrolls freely)
		if it.Index == in.Sel && in.Sel != inv.seen {
			inv.top = min(inv.top, i)
			inv.top = max(inv.top, i-inv.rows+1)
		}
	}
	inv.seen = in.Sel
	inv.top = max(0, min(inv.top, inv.n-inv.rows))
	tr := rr // the lines' right edge: the scroll bar's left
	if inv.n > inv.rows {
		tr -= 10
	}
	chars := (tr - rx - rowH - 4) / 8
	for i := inv.top; i < inv.n && i < inv.top+inv.rows; i++ {
		it := &in.Pool[i]
		y := listTop + (i-inv.top)*rowH
		r := image.Rect(rx, y, tr, y+rowH)
		switch {
		case it.Index == in.Sel:
			inv.fill(r, blue)
		case inv.hover.In(r) && !inv.busy:
			inv.outline(r, dark)
		}
		inv.drawIcon(it, image.Rect(rx, y, rx+rowH, y+rowH))
		inv.text(upTo(it.Name, chars), rx+rowH+4, y+(rowH-8)/2, green)
		inv.lines = append(inv.lines, hit{r: r, item: *it, whole: len(strings.TrimSpace(it.Name)) <= chars})
	}
	if inv.n == 0 {
		inv.text("Nothing", rx+rowH+4, listTop+(rowH-8)/2, dark)
	}
	inv.scroll = image.Rectangle{}
	if inv.n > inv.rows { // a groove with a bevelled thumb
		inv.scroll = image.Rect(rr-6, listTop, rr, listTop+inv.rows*rowH)
		inv.fill(image.Rect(rr-4, listTop, rr-2, inv.scroll.Max.Y), dark)
		h := inv.scroll.Dy()
		t := image.Rect(rr-6, listTop+h*inv.top/inv.n, rr, listTop+h*(inv.top+inv.rows)/inv.n)
		inv.fill(t, gray)
		inv.fill(image.Rect(t.Min.X, t.Min.Y, t.Max.X, t.Min.Y+1), white)
		inv.fill(image.Rect(t.Min.X, t.Max.Y-1, t.Max.X, t.Max.Y), dark)
	}

	// an item dragged under the pointer
	if inv.dragging && inv.press != nil {
		inv.drawIcon(&inv.press.item, image.Rectangle{inv.hover, inv.hover}.Inset(-rowH/2))
	}

	// onto the GPU: the boxes' borders, the menu row, the game's boxes over it
	inv.img.WritePixels(inv.px.Pix)
	drawBorder(inv.img, frame, outer, gpu.GeoM{}, leftCells, ch-1)
	var g gpu.GeoM
	g.Translate(leftCells*8, 0)
	drawBorder(inv.img, frame, outer, g, cw-leftCells, ch-1)
	pr := engine.PromptRows.Pixels()
	inv.bar = pr.Sub(pr.Min).Add(image.Pt((inv.w-pr.Dx())/2, (ch-1)*8))
	op := &gpu.DrawImageOptions{}
	op.GeoM.Translate(float64(inv.bar.Min.X), float64(inv.bar.Min.Y))
	inv.img.DrawImage(frame.SubImage(pr).(*gpu.Image), op)
	if in.Over != (engine.CellRect{}) {
		drawBox(inv.img, frame, outer, in.Over, 1, inv.w/2, inv.h/2)
	} else if !inv.dragging && !inv.busy {
		inv.tooltip(frame, in)
	}

	// onto the screen at the whole scale
	inv.at = image.Pt((b.Dx()-inv.k*inv.w)/2, (b.Dy()-inv.k*inv.h)/2)
	dst.Fill(color.Black)
	op = &gpu.DrawImageOptions{}
	op.GeoM.Scale(float64(inv.k), float64(inv.k))
	op.GeoM.Translate(float64(inv.at.X), float64(inv.at.Y))
	dst.DrawImage(inv.img, op)
	return true
}

// tooltip names what the pointer is on when the screen does not show it
// whole: a doll slot (its item or what goes there), a cut list line.
func (inv *inventory) tooltip(frame *gpu.Image, in *engine.Inventory) {
	s := ""
	for i, r := range dollBoxes {
		if inv.hover.In(r.Add(inv.dollAt.Min)) {
			s = slotNames[i]
			if it := in.Doll[i]; it != nil {
				s = it.Name
			}
		}
	}
	for _, l := range inv.lines {
		if !l.doll && !l.whole && inv.hover.In(l.r) {
			s = l.item.Name
		}
	}
	if s = strings.TrimSpace(s); s == "" {
		return
	}
	cw := min(len(s), inv.w/8-2) + 2
	w, h := cw*8, 24
	if inv.tip == nil || inv.tip.Bounds().Dx() != w {
		if inv.tip != nil {
			inv.tip.Deallocate()
		}
		inv.tip = gpu.NewImage(w, h)
	}
	px := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(px, px.Rect, image.Black, image.Point{}, draw.Src)
	drawText(px, inv.glyph, upTo(s, cw-2), 8, 8, cyan)
	inv.tip.WritePixels(px.Pix)
	drawBorder(inv.tip, frame, outer, gpu.GeoM{}, cw, 3)
	x := min(inv.hover.X+8, inv.w-w)
	y := inv.hover.Y + 12
	if y+h > inv.h {
		y = inv.hover.Y - h - 4
	}
	op := &gpu.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	inv.img.DrawImage(inv.tip, op)
}

func hasKey(m *engine.Menu, k byte) bool {
	for _, it := range m.Items {
		if it.Key == k {
			return true
		}
	}
	return false
}

// InventoryShown: the last frame was the inventory screen (its mouse
// input goes to InventoryMouse, not the game's text cells).
func (p *Presenter) InventoryShown() bool { return p.inv.shown }

// InventoryMouse is the keys for the pointer at screen pixel (x, y) with
// the left button down or not, the right one just pressed and the wheel's
// movement. A click on an item readies or removes it (picks it when it
// has no slot), dragging it onto the doll or off it does the same; a right
// click looks at it. The arrows are the previous and next member (F1-F8),
// the menu row is the game's own; the wheel and the scroll bar scroll.
func (p *Presenter) InventoryMouse(x, y int, down, right bool, wheel float64) []uint16 {
	inv := &p.inv
	if inv.k == 0 {
		return nil
	}
	inv.hover = image.Pt((x-inv.at.X)/inv.k, (y-inv.at.Y)/inv.k)
	pressed, released := down && !inv.held, !down && inv.held
	inv.held = down

	// the wheel: a line a step, at most one a pause; the rest of a fling is dropped
	if inv.wheel += wheel; (inv.wheel >= 1 || inv.wheel <= -1) && time.Since(inv.wheeled) >= wheelPause {
		if inv.wheel < 0 { // the touchpad's natural direction
			inv.top--
		} else {
			inv.top++
		}
		inv.top = max(0, min(inv.top, inv.n-inv.rows))
		inv.wheel, inv.wheeled = 0, time.Now()
	} else if time.Since(inv.wheeled) < wheelPause {
		inv.wheel = 0
	}

	cell := func() uint16 {
		return engine.MouseKey(engine.MouseLeft, (inv.hover.X-inv.bar.Min.X)/8, engine.PromptRows.Y1)
	}
	if inv.busy { // a question or a box of the game's: its row, else any click (press a key)
		inv.press, inv.dragging, inv.scrolling = nil, false, false
		switch {
		case pressed && inv.hover.In(inv.bar):
			return []uint16{cell()}
		case pressed:
			return []uint16{engine.MouseKey(engine.MouseLeft, 0, 0)}
		case right:
			return []uint16{engine.MouseKey(engine.MouseRight, 0, 0)}
		}
		return nil
	}
	if inv.scrolling && down && inv.scroll.Dy() > 0 {
		inv.top = (inv.hover.Y-inv.scroll.Min.Y)*inv.n/inv.scroll.Dy() - inv.rows/2
		inv.top = max(0, min(inv.top, inv.n-inv.rows))
	}
	if inv.press != nil && down && !inv.dragging {
		d := inv.hover.Sub(inv.pressAt)
		inv.dragging = d.X*d.X+d.Y*d.Y > 9
	}
	if right {
		for _, l := range inv.lines {
			if inv.hover.In(l.r) {
				return []uint16{engine.KeyItem + uint16(l.item.Index), 'L'}
			}
		}
		return []uint16{engine.MouseKey(engine.MouseRight, 0, 0)}
	}
	if pressed {
		switch {
		case inv.hover.In(inv.bar):
			return []uint16{cell()}
		case inv.hover.In(inv.scroll):
			inv.scrolling = true
		}
		for i, r := range inv.arrows {
			if inv.hover.In(r) && inv.count > 1 {
				n := (inv.member + inv.count + 2*i - 1) % inv.count
				return []uint16{engine.KeyF1 + uint16(n)<<8}
			}
		}
		for _, l := range inv.lines {
			if inv.hover.In(l.r) {
				inv.press, inv.pressAt = &l, inv.hover
			}
		}
		return nil
	}
	if !released {
		return nil
	}
	h, drag := inv.press, inv.dragging
	inv.press, inv.dragging, inv.scrolling = nil, false, false
	if h == nil {
		return nil
	}
	pick := engine.KeyItem + uint16(h.item.Index)
	switch {
	case !drag && !h.item.Slot: // nothing to ready: pick it for the menu
		return []uint16{pick}
	case !drag,
		h.doll && !inv.hover.In(inv.dollAt), // off the doll
		!h.doll && inv.hover.In(inv.dollAt): // onto it
		return []uint16{pick, 'R'}
	}
	return nil
}
