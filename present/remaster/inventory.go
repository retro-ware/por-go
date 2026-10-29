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
	"slices"
	"strconv"
	"strings"
	"time"

	"gpu"

	"por/engine"
	"por/pal"
)

// The inventory screen (F1-F8, Items): the member's Eye of the Beholder
// paper doll on the left (the shop's Browse: its goods, half the screen
// each), the party inventory on the right, in the
// original's font, colours and box borders. Everything is drawn 1:1 on one
// canvas that is scaled whole to the window, so all pixels are the same
// size. It draws engine.View.Inventory and acts through engine.Action: the
// menu row, the Look box and the Drop and Sell questions are its own.

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

func col(i int) color.RGBA {
	c := pal.Palette[i]
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
	sel    int             // the highlighted item (InvItem.Index), -1 for none
	look   *engine.InvItem // the Look box is up for it
	ask    *engine.Action  // a Drop or Sell waits for Yes
	view   *engine.Inventory
	acts   []engine.Action // to send (Presenter.Actions)
	words  []word          // the menu row's words, or the question's Yes and No
	k      int             // canvas scale
	at     image.Point     // canvas top left on screen
	w, h   int             // canvas size
	list   pane            // the party inventory
	wares  pane            // the shop's goods (Browse)
	ware   int             // the highlighted ware (InvItem.Index), -1 for none
	lines  []hit           // the items' rectangles (canvas)
	dollAt image.Rectangle
	arrows [2]image.Rectangle
	hover  image.Point // the pointer (canvas)
	member int         // View.Inventory.Member
	count  int         // party members

	held      bool        // the left button was down last frame
	press     *hit        // the item the button went down on
	pressAt   image.Point // where
	dragging  bool        // it moved off: a drag
	scrolling *pane       // the button went down on its scroll bar
	wheel     float64     // wheel movement not yet a line
	wheeled   time.Time   // the last line the wheel scrolled
}

type hit struct {
	r     image.Rectangle
	item  engine.InvItem
	doll  bool // on the doll (readied)
	ware  bool // one of the shop's goods
	whole bool // its name shows whole
}

// pane is a scrolled list of items: the party inventory or the shop's goods.
type pane struct {
	top, rows, n int             // the first line shown, the lines shown, the lines
	seen         int             // the highlight the list last scrolled to
	at           image.Rectangle // the lines' area
	scroll       image.Rectangle // the scroll bar
}

// clamp keeps the lines shown inside the list.
func (pn *pane) clamp() { pn.top = max(0, min(pn.top, pn.n-pn.rows)) }

// drawPane draws items as lines of icon and name (and price) from x0 to x1,
// listTop to bottom, the highlighted one (sel) in blue, with a scroll bar
// when they do not fit.
func (inv *inventory) drawPane(pn *pane, items []engine.InvItem, sel int, ware, price, busy bool, x0, x1, bottom int) {
	pn.n, pn.rows = len(items), max(1, (bottom-listTop)/rowH)
	for i, it := range items { // keep a newly highlighted line in sight (the wheel scrolls freely)
		if it.Index == sel && sel != pn.seen {
			pn.top = min(pn.top, i)
			pn.top = max(pn.top, i-pn.rows+1)
		}
	}
	pn.seen = sel
	pn.clamp()
	tr := x1 // the lines' right edge: the scroll bar's left
	if pn.n > pn.rows {
		tr -= 10
	}
	pn.at = image.Rect(x0, listTop, tr, listTop+pn.rows*rowH)
	pw := 0 // the prices' column
	if price {
		for _, it := range items {
			pw = max(pw, 8*len(shortMoney(it.Price))+8)
		}
	}
	chars := (tr - x0 - rowH - 4 - pw) / 8
	for i := pn.top; i < pn.n && i < pn.top+pn.rows; i++ {
		it := &items[i]
		y := listTop + (i-pn.top)*rowH
		r := image.Rect(x0, y, tr, y+rowH)
		switch {
		case it.Index == sel:
			inv.fill(r, blue)
		case inv.hover.In(r) && !busy:
			inv.outline(r, dark)
		}
		inv.drawIcon(it, image.Rect(x0, y, x0+rowH, y+rowH))
		tc := green
		if it.Wrong {
			tc = red
		}
		inv.text(upTo(it.Name, chars), x0+rowH+4, y+(rowH-8)/2, tc)
		if price {
			m := shortMoney(it.Price)
			inv.text(m, tr-4-8*len(m), y+(rowH-8)/2, col(14))
		}
		inv.lines = append(inv.lines, hit{r: r, item: *it, ware: ware, whole: len(strings.TrimSpace(it.Name)) <= chars})
	}
	if pn.n == 0 {
		inv.text("Nothing", x0+rowH+4, listTop+(rowH-8)/2, dark)
	}
	pn.scroll = image.Rectangle{}
	if pn.n > pn.rows { // a groove with a bevelled thumb
		pn.scroll = image.Rect(x1-6, listTop, x1, listTop+pn.rows*rowH)
		inv.fill(image.Rect(x1-4, listTop, x1-2, pn.scroll.Max.Y), dark)
		h := pn.scroll.Dy()
		t := image.Rect(x1-6, listTop+h*pn.top/pn.n, x1, listTop+h*(pn.top+pn.rows)/pn.n)
		inv.fill(t, gray)
		inv.fill(image.Rect(t.Min.X, t.Min.Y, t.Max.X, t.Min.Y+1), white)
		inv.fill(image.Rect(t.Min.X, t.Max.Y-1, t.Max.X, t.Max.Y), dark)
	}
}

// shortMoney is a price as "12g 5c".
func shortMoney(s string) string {
	return strings.NewReplacer(" gold and ", "g ", " gold", "g", " copper", "c").Replace(s)
}

// wareAt is the shop's ware with Index i, nil if none.
func wareAt(in *engine.Inventory, i int) *engine.InvItem {
	for k := range in.Wares {
		if in.Wares[k].Index == i {
			return &in.Wares[k]
		}
	}
	return nil
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

// word is a choice on the menu row (or a question's Yes and No): its
// text, the action it sends and where it is (canvas).
type word struct {
	text string
	act  engine.ActKind
	on   bool // it does something now
	r    image.Rectangle
}

// itemAt is the inventory's item with Index i, nil if none.
func itemAt(in *engine.Inventory, i int) *engine.InvItem {
	for _, it := range in.Doll {
		if it != nil && it.Index == i {
			return it
		}
	}
	for k := range in.Pool {
		if in.Pool[k].Index == i {
			return &in.Pool[k]
		}
	}
	return nil
}

// menu is the menu row for the highlighted item.
func (inv *inventory) menu(in *engine.Inventory) []word {
	it := itemAt(in, inv.sel)
	if inv.ask != nil {
		return []word{{text: "Yes", act: inv.ask.Kind, on: true}, {text: "No", act: engine.ActNone, on: true}}
	}
	if in.Wares != nil { // the shop: buy a ware, sell an item
		return []word{{text: "Buy", act: engine.ActBuy, on: wareAt(in, inv.ware) != nil}, {text: "Sell", act: engine.ActSell, on: it != nil && in.Sell},
			{text: "Look", act: actLook, on: it != nil || wareAt(in, inv.ware) != nil}, {text: "Exit", act: engine.ActClose, on: true}}
	}
	w := []word{{text: "Ready", act: engine.ActReady, on: it != nil && it.Slot}}
	if in.Throw {
		w = append(w, word{text: "Throw", act: engine.ActThrow, on: it != nil && it.Throw})
	}
	if in.Use {
		w = append(w, word{text: "Use", act: engine.ActUse, on: it != nil && it.Use})
	}
	w = append(w, word{text: "Drop", act: engine.ActDrop, on: it != nil})
	if in.Sell {
		w = append(w, word{text: "Sell", act: engine.ActSell, on: it != nil})
	}
	w = append(w, word{text: "Look", act: actLook, on: it != nil})
	for _, pw := range in.Powers {
		w = append(w, word{text: pw.Name, act: pw.Kind, on: true})
	}
	return append(w, word{text: "Exit", act: engine.ActClose, on: true})
}

// actLook is the menu's Look, which the screen shows itself.
const actLook engine.ActKind = -1

// Inventory draws the inventory screen when the game shows it, and
// reports whether it did.
func (p *Presenter) Inventory(dst *gpu.Image, note Note, v engine.View) bool {
	inv := &p.inv
	inv.shown = false
	in := v.Inventory
	if in == nil {
		inv.look, inv.ask, inv.view = nil, nil, nil
		return false
	}
	inv.load()
	if inv.view == nil || in.Member != inv.member {
		inv.sel, inv.list.top = -1, 0 // a new member: nothing highlighted
	}
	if inv.view == nil || (inv.view.Wares == nil) != (in.Wares == nil) {
		inv.ware, inv.wares.top = -1, 0
	}
	inv.shown, inv.member, inv.count, inv.glyph, inv.view = true, in.Member, len(v.Party), note.Glyph, in
	if itemAt(in, inv.sel) == nil {
		inv.sel = -1
	}
	if wareAt(in, inv.ware) == nil {
		inv.ware = -1
	}
	shop := in.Wares != nil
	split := leftCells // the left box's width in cells, with its border
	busy := inv.look != nil || inv.ask != nil
	p.setup(note, v)
	t := p.border.tiles

	b := dst.Bounds()
	inv.k = max(1, min(b.Dx()/invW, b.Dy()/invH))
	cw, ch := max(invW, b.Dx()/inv.k)/8, max(invH, b.Dy()/inv.k)/8
	if shop { // the goods and the party inventory share the screen
		split = (cw + 1) / 2
	}
	if w, h := cw*8, ch*8; w != inv.w || h != inv.h {
		if inv.img != nil {
			inv.img.Deallocate()
		}
		inv.w, inv.h, inv.px, inv.img = w, h, image.NewRGBA(image.Rect(0, 0, w, h)), gpu.NewImage(w, h)
	}
	inv.fill(inv.px.Rect, color.RGBA{A: 255})
	inv.lines = inv.lines[:0]
	bottom := (ch - 2) * 8 // the boxes' inner bottom

	// the member: name, portrait between the arrows, hit points (the shop: its goods)
	lx, lw := 8, (leftCells-2)*8
	inv.arrows = [2]image.Rectangle{}
	inv.dollAt = image.Rectangle{}
	if shop {
		inv.text("Shop", lx, 8, white)
		inv.drawPane(&inv.wares, in.Wares, inv.ware, true, true, busy, lx, (split-1)*8, bottom)
	} else {
		p.drawMember(in, v, lx, lw, bottom, busy)
	}

	// the party inventory: its title and fill, the lines, the scroll bar
	rx, rr := split*8, (cw-1)*8 // a cell inside the shared divider
	inv.text("Party Items", rx, 8, white)
	fillc := green
	if len(in.Pool) >= in.Cap {
		fillc = red
	}
	s := strconv.Itoa(len(in.Pool)) + "/" + strconv.Itoa(in.Cap)
	inv.text(s, rr-8*len(s), 8, fillc)
	inv.drawPane(&inv.list, in.Pool, inv.sel, false, shop, busy, rx, rr, bottom)

	// an item dragged under the pointer
	if inv.dragging && inv.press != nil {
		inv.drawIcon(&inv.press.item, image.Rectangle{inv.hover, inv.hover}.Inset(-rowH/2))
	}

	// the menu row under the boxes: a note, else the choices
	rowY := (ch-1)*8 + 0
	inv.words = inv.menu(in)
	if note.Show && note.Text != "" {
		inv.words = nil
		inv.text(upTo(note.Text, cw), (inv.w-8*min(len(note.Text), cw))/2, rowY, col(14))
	}
	x := 0
	for _, w := range inv.words {
		x += 8 * (len(w.text) + 1)
	}
	x = (inv.w - x + 8) / 2
	for i := range inv.words {
		w := &inv.words[i]
		w.r = image.Rect(x-4, rowY-2, x+8*len(w.text)+4, rowY+10)
		c, hc := green, white
		switch {
		case !w.on:
			c, hc = dark, dark
		case inv.hover.In(w.r):
			inv.fill(w.r, blue)
		}
		inv.text(w.text[:1], x, rowY, hc)
		inv.text(w.text[1:], x+8, rowY, c)
		x += 8 * (len(w.text) + 1)
	}

	// the Look box or the question over the list
	var box []string
	switch it := itemAt(in, inv.sel); {
	case inv.look != nil:
		box = append([]string{strings.TrimSpace(inv.look.Name), ""}, inv.look.Look...)
	case inv.ask != nil && it != nil && inv.ask.Kind == engine.ActDrop:
		box = []string{"Your " + strings.TrimSpace(it.Name), "will be gone forever.", "", "Drop it?"}
	case inv.ask != nil && it != nil:
		box = []string{"I'll give you " + it.Price, "for your " + strings.TrimSpace(it.Name) + ".", "", "Is it a deal?"}
	}
	var br image.Rectangle
	if box != nil {
		bw := 0
		for _, l := range box {
			bw = max(bw, len(l))
		}
		bw = min(bw, cw-6)
		br = image.Rect(0, 0, (bw+2)*8, (len(box)+2)*8)
		br = br.Add(image.Pt((inv.w-br.Dx())/2/8*8, (inv.h-br.Dy())/2/8*8))
		inv.fill(br, color.RGBA{A: 255})
		for i, l := range box {
			c := green
			if i == 0 {
				c = cyan
			}
			inv.text(upTo(l, bw), br.Min.X+8, br.Min.Y+8+8*i, c)
		}
	}

	// onto the GPU: the boxes' borders, then the box's
	inv.img.WritePixels(inv.px.Pix)
	drawBorder(inv.img, t, gpu.GeoM{}, split, ch-1)
	var g gpu.GeoM
	g.Translate(float64(split-1)*8, 0) // the boxes share the divider
	drawBorder(inv.img, t, g, cw-split+1, ch-1)
	if shop {
		inv.money(in, rr)
	}
	if box != nil {
		var g gpu.GeoM
		g.Translate(float64(br.Min.X), float64(br.Min.Y))
		drawBorder(inv.img, t, g, br.Dx()/8, br.Dy()/8)
	} else if !inv.dragging {
		inv.tooltip(t, in)
	}

	// onto the screen at the whole scale
	inv.at = image.Pt((b.Dx()-inv.k*inv.w)/2, (b.Dy()-inv.k*inv.h)/2)
	dst.Fill(color.Black)
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(float64(inv.k), float64(inv.k))
	op.GeoM.Translate(float64(inv.at.X), float64(inv.at.Y))
	dst.DrawImage(inv.img, op)
	return true
}

// drawMember draws the member's name, portrait, hit points and paper doll
// with the readied items in the left box.
func (p *Presenter) drawMember(in *engine.Inventory, v engine.View, lx, lw, bottom int, busy bool) {
	inv := &p.inv

	oy := max(0, (bottom-84-inv.doll.Bounds().Dy())/3) // the portrait and doll down the box, a little above its middle
	if in.Member >= 0 && in.Member < len(v.Party) {
		m := v.Party[in.Member]
		inv.text(m.Name, lx+(lw-8*len(m.Name))/2, 8, white)
		for i, cl := range strings.Split(m.Class, "/") { // a class a line: three don't fit one
			inv.text(cl, lx+(lw-8*len(cl))/2, 18+10*i, green)
		}
		if f := p.face(m.Head); f != nil {
			r := image.Rect(lx+(lw-portraitSize)/2, 20+oy, lx+(lw+portraitSize)/2, 20+oy+portraitSize)
			draw.Draw(inv.px, r, f, f.Bounds().Min, draw.Src)
		}
		hp := "HP " + strconv.Itoa(m.HP) + "/" + strconv.Itoa(m.MaxHP)
		inv.text(hp, lx+(lw-8*len(hp))/2, 70+oy, green)
	}
	ab := image.Rect(-8, -8, 24, 16) // "<-" and "->" with room round them for the pointer
	inv.arrows = [2]image.Rectangle{ab.Add(image.Pt(lx+4, 38+oy)), ab.Add(image.Pt(lx+lw-20, 38+oy))}
	if inv.count > 1 && in.Switch {
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
		case it != nil && it.Index == inv.sel:
			inv.outline(box, col(14)) // yellow: the boxes are white-edged
		case inv.hover.In(box) && !busy:
			inv.outline(box, cyan)
		}
	}
}

// money writes the party's gold and copper on the top border, at the
// right box's right end.
func (inv *inventory) money(in *engine.Inventory, rr int) {
	g, c := strconv.Itoa(in.Gold)+"g", strconv.Itoa(in.Copper)+"c"
	w := 8 * (len(g) + len(c) + 3)
	px := image.NewRGBA(image.Rect(0, 0, w, 8))
	draw.Draw(px, px.Rect, image.Black, image.Point{}, draw.Src)
	drawText(px, inv.glyph, g, 8, 0, col(14))
	drawText(px, inv.glyph, c, 8*(len(g)+2), 0, col(6))
	m := gpu.NewImage(w, 8)
	defer m.Deallocate()
	m.WritePixels(px.Pix)
	op := &gpu.DrawImageOptions{}
	op.GeoM.Translate(float64(rr-w), 0)
	inv.img.DrawImage(m, op)
}

// tooltip names what the pointer is on when the screen does not show it
// whole: a doll slot (its item or what goes there), a cut list line.
func (inv *inventory) tooltip(t [3]*gpu.Image, in *engine.Inventory) {
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
	drawBorder(inv.tip, t, gpu.GeoM{}, cw, 3)
	x := min(inv.hover.X+8, inv.w-w)
	y := inv.hover.Y + 12
	if y+h > inv.h {
		y = inv.hover.Y - h - 4
	}
	op := &gpu.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	inv.img.DrawImage(inv.tip, op)
}

// InventoryShown: the last frame was the inventory screen (its mouse
// input goes to InventoryMouse, not the game's text cells).
func (p *Presenter) InventoryShown() bool { return p.inv.shown }

// Actions hands over the actions the screens made since the last call.
func (p *Presenter) Actions() []engine.Action {
	a := append(p.inv.acts, p.battle.Actions()...)
	p.inv.acts = nil
	return a
}

// do runs a menu choice on the highlighted item.
func (inv *inventory) do(k engine.ActKind) {
	it := itemAt(inv.view, inv.sel)
	switch {
	case k == engine.ActNone:
	case k == engine.ActClose, k >= engine.ActSpells && k <= engine.ActCure:
		inv.acts = append(inv.acts, engine.Action{Kind: k})
	case k == engine.ActBuy:
		if w := wareAt(inv.view, inv.ware); w != nil {
			inv.acts = append(inv.acts, engine.Action{Kind: k, N: w.Index})
		}
	case k == actLook && wareAt(inv.view, inv.ware) != nil:
		inv.look = wareAt(inv.view, inv.ware)
	case it == nil:
	case k == actLook:
		inv.look = it
	case inv.ask == nil && (k == engine.ActDrop || k == engine.ActSell):
		inv.ask = &engine.Action{Kind: k, N: it.Index}
		return
	default:
		inv.acts = append(inv.acts, engine.Action{Kind: k, N: it.Index})
	}
	inv.ask = nil
}

// choose highlights ware i of the shop or item i of the member's.
func (inv *inventory) choose(ware bool, i int) {
	inv.sel, inv.ware = -1, -1
	if ware {
		inv.ware = i
	} else {
		inv.sel = i
	}
}

// step shows the member d places on (the arrows, Left and Right).
func (inv *inventory) step(d int) {
	if inv.count > 1 && inv.view.Switch {
		inv.acts = append(inv.acts, engine.Action{Kind: engine.ActMember, N: (inv.member + inv.count + d) % inv.count})
	}
}

// inventoryKey handles a key on the inventory screen; false leaves it to
// the game (F9, ?, Ctrl-F).
func (p *Presenter) inventoryKey(k uint16) bool {
	inv := &p.inv
	if inv.look != nil { // any key closes the Look box
		inv.look = nil
		return true
	}
	if inv.ask != nil {
		switch {
		case k == 'y' || k == 'Y' || k == 13:
			inv.do(inv.ask.Kind)
		default:
			inv.ask = nil
		}
		return true
	}
	type pick struct {
		ware bool
		i    int
	}
	var order []pick // the items as the screen shows them: the goods or the doll, then the list
	for _, it := range inv.view.Wares {
		order = append(order, pick{true, it.Index})
	}
	for _, it := range inv.view.Doll {
		if it != nil {
			order = append(order, pick{false, it.Index})
		}
	}
	for _, it := range inv.view.Pool {
		order = append(order, pick{false, it.Index})
	}
	step := func(d int) {
		if len(order) == 0 {
			return
		}
		i := slices.Index(order, pick{inv.ware >= 0, max(inv.ware, inv.sel)})
		if i < 0 && d < 0 {
			i = 0
		}
		inv.choose(order[(i+d+len(order))%len(order)].ware, order[(i+d+len(order))%len(order)].i)
	}
	switch {
	case k == 27:
		inv.do(engine.ActClose)
	case k == 0x4800:
		step(-1)
	case k == 0x5000:
		step(1)
	case k == 0x4B00:
		inv.step(-1)
	case k == 0x4D00:
		inv.step(1)
	case k == 13 && inv.ware >= 0:
		inv.do(engine.ActBuy)
	case k == 13:
		inv.do(engine.ActReady)
	case k >= KeyF1 && k <= KeyF1+7<<8 && k&0xFF == 0:
		if n := int(k-KeyF1) >> 8; n < inv.count && inv.view.Switch {
			inv.acts = append(inv.acts, engine.Action{Kind: engine.ActMember, N: n})
		}
	case k > ' ' && k < 128:
		for _, w := range inv.menu(inv.view) {
			if w.on && strings.EqualFold(w.text[:1], string(rune(k))) {
				inv.do(w.act)
			}
		}
	default:
		return false
	}
	return true
}

// InventoryMouse handles the pointer at screen pixel (x, y) with the left
// button down or not, the right one just pressed and the wheel's movement.
// A click on an item highlights it; a click on a highlighted item with a
// slot readies or removes it, as does dragging it onto the doll or off it;
// a right click looks at it. The arrows are the previous and next member,
// the menu row acts on the highlighted item; the wheel and the scroll bar
// scroll.
func (p *Presenter) InventoryMouse(x, y int, down, right bool, wheel float64) {
	inv := &p.inv
	if inv.k == 0 || inv.view == nil {
		return
	}
	inv.hover = image.Pt((x-inv.at.X)/inv.k, (y-inv.at.Y)/inv.k)
	pressed, released := down && !inv.held, !down && inv.held
	inv.held = down

	// the wheel: a line a step, at most one a pause; the rest of a fling is dropped
	pn := &inv.list // the list under the pointer
	if inv.view.Wares != nil && inv.hover.X < inv.list.at.Min.X {
		pn = &inv.wares
	}
	if inv.wheel += wheel; (inv.wheel >= 1 || inv.wheel <= -1) && time.Since(inv.wheeled) >= wheelPause {
		if inv.wheel < 0 { // the touchpad's natural direction
			pn.top--
		} else {
			pn.top++
		}
		pn.clamp()
		inv.wheel, inv.wheeled = 0, time.Now()
	} else if time.Since(inv.wheeled) < wheelPause {
		inv.wheel = 0
	}

	if inv.look != nil || inv.ask != nil { // a box: its Yes and No, else any click closes it
		inv.press, inv.dragging, inv.scrolling = nil, false, nil
		if pressed || right {
			for _, w := range inv.words {
				if inv.hover.In(w.r) && pressed {
					inv.do(w.act)
					return
				}
			}
			inv.look, inv.ask = nil, nil
		}
		return
	}
	if sc := inv.scrolling; sc != nil && down && sc.scroll.Dy() > 0 {
		sc.top = (inv.hover.Y-sc.scroll.Min.Y)*sc.n/sc.scroll.Dy() - sc.rows/2
		sc.clamp()
	}
	if inv.press != nil && down && !inv.dragging {
		d := inv.hover.Sub(inv.pressAt)
		inv.dragging = d.X*d.X+d.Y*d.Y > 9
	}
	if right {
		for _, l := range inv.lines {
			if inv.hover.In(l.r) {
				inv.choose(l.ware, l.item.Index)
				inv.do(actLook)
				return
			}
		}
		inv.do(engine.ActClose)
		return
	}
	if pressed {
		for _, w := range inv.words {
			if inv.hover.In(w.r) && w.on {
				inv.do(w.act)
				return
			}
		}
		for _, sc := range []*pane{&inv.list, &inv.wares} {
			if inv.hover.In(sc.scroll) {
				inv.scrolling = sc
			}
		}
		for i, r := range inv.arrows {
			if inv.hover.In(r) {
				inv.step(2*i - 1)
				return
			}
		}
		for _, l := range inv.lines {
			if inv.hover.In(l.r) {
				inv.press, inv.pressAt = &l, inv.hover
			}
		}
		return
	}
	if !released {
		return
	}
	h, drag := inv.press, inv.dragging
	inv.press, inv.dragging, inv.scrolling = nil, false, nil
	if h == nil {
		return
	}
	if h.ware { // a click highlights a ware, a second one or a drag onto the party's items buys it
		was := inv.ware
		inv.choose(true, h.item.Index)
		if !drag && was == h.item.Index || drag && inv.hover.In(inv.list.at) {
			inv.do(engine.ActBuy)
		}
		return
	}
	was := inv.sel
	inv.choose(false, h.item.Index)
	if inv.view.Wares != nil { // the shop: a drag onto the goods sells
		if drag && inv.hover.In(inv.wares.at) {
			inv.do(engine.ActSell)
		}
		return
	}
	switch {
	case !drag && (was != h.item.Index || !h.item.Slot): // a click highlights it
	case !drag,
		h.doll && !inv.hover.In(inv.dollAt), // off the doll
		!h.doll && inv.hover.In(inv.dollAt): // onto it
		inv.do(engine.ActReady)
	}
}
