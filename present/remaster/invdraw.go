// invdraw.go draws the inventory screen: the menu row's words, the doll, the member panel, tooltips and borders.

package remaster

import (
	"gpu"
	"image"
	"image/color"
	"image/draw"
	"por/engine"
	"por/pal"
	"slices"
	"strconv"
	"strings"
)

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
	if inv.ask != nil { // the question has its Yes and No
		return nil
	}
	if in.Loot { // the loot: take a piece of treasure
		return []word{{text: "Take", act: engine.ActBuy, on: wareAt(in, inv.ware) != nil}, {text: "All", act: engine.ActTakeAll, on: len(in.Wares) > 0}, {text: "Exit", act: engine.ActClose, on: true}}
	}
	if in.Wares != nil { // the shop: buy a ware, sell an item
		return []word{{text: "Buy", act: engine.ActBuy, on: wareAt(in, inv.ware) != nil}, {text: "Sell", act: engine.ActSell, on: it != nil && in.Sell},
			{text: "Exit", act: engine.ActClose, on: true}}
	}
	w := []word{{text: "Ready", act: engine.ActReady, on: it != nil && it.Slot}}
	if in.Use {
		w = append(w, word{text: "Use", act: engine.ActUse, on: it != nil && it.Use})
	}
	w = append(w, word{text: "Drop", act: engine.ActDrop, on: it != nil})
	if in.Sell {
		w = append(w, word{text: "Sell", act: engine.ActSell, on: it != nil})
	}
	w = append(w, word{text: "View", act: engine.ActSheet, on: true}) // the sheet (Tab)
	for _, pw := range in.Powers {
		w = append(w, word{text: pw.Name, act: pw.Kind, on: true})
	}
	return append(w, word{text: "Exit", act: engine.ActClose, on: true})
}

// Inventory draws the inventory screen when the game shows it, and
// reports whether it did.
func (p *Presenter) Inventory(dst *gpu.Image, note Note, v engine.View) bool {
	inv := &p.inv
	inv.shown = false
	in := v.Inventory
	if choosing(v) { // a member pick (Heal whom?): the view and the cards instead (Draw)
		return false
	}
	if in == nil {
		inv.ask, inv.view = nil, nil
		return false
	}
	inv.load()
	if inv.view == nil || in.Member != inv.member {
		inv.sel, inv.list.top, inv.carry = -1, 0, nil // a new member: nothing highlighted or on the cursor
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
	busy := inv.ask != nil
	p.setup(note, v)
	t := p.border.tiles

	b := dst.Bounds()
	inv.k = max(1, min(b.Dx()/invW, b.Dy()/invH))
	cw, ch := max(invW, b.Dx()/inv.k)/8, max(invH, b.Dy()/inv.k)/8
	if shop { // the goods and the party inventory share the screen
		split = (cw + 1) / 2
	}
	if w, h := cw*8, ch*8; gpu.Sized(&inv.img, w, h) {
		inv.w, inv.h, inv.px = w, h, image.NewRGBA(image.Rect(0, 0, w, h))
	}
	inv.fill(inv.px.Rect, color.RGBA{A: 255})
	inv.lines = inv.lines[:0]
	bottom := (ch - 2) * 8 // the boxes' inner bottom: under them the menu row, the toast in its place while it shows

	// the member: name, portrait between the arrows, hit points (the shop: its goods)
	lx, lw := 8, (leftCells-2)*8
	inv.arrows = [2]image.Rectangle{}
	inv.dollAt = image.Rectangle{}
	inv.facePNG = nil
	if shop {
		inv.drawTabs(&inv.wares, lx, busy)
		inv.drawPane(&inv.wares, inv.wares.shown(in.Wares), inv.ware, true, true, busy, lx, (split-1)*8, bottom)
	} else {
		inv.wares.tabs = [len(tabNames)]image.Rectangle{} // no shop: no tabs to click
		p.drawMember(in, v, lx, lw, bottom, busy)
	}

	// the party inventory: its title and fill, the lines, the scroll bar
	rx, rr := split*8, (cw-1)*8 // a cell inside the shared divider
	inv.drawTabs(&inv.list, rx, busy)
	fillc := green
	if len(in.Pool) >= in.Cap {
		fillc = red
	}
	fill := strconv.Itoa(len(in.Pool)) + "/" + strconv.Itoa(in.Cap)
	inv.drawPane(&inv.list, inv.list.shown(in.Pool), inv.sel, false, shop && !in.Loot, busy, rx, rr, bottom) // the shop prices the party's items (it buys them); the loot does not

	// an item dragged, or the stack picked up, under the pointer
	if inv.carry != nil && inv.carriedItem() == nil { // gone (used, dropped, another member)
		inv.carry = nil
	}
	if inv.dragging && inv.press != nil {
		inv.drawIcon(&inv.press.item, image.Rectangle{inv.hover, inv.hover}.Inset(-rowH/2))
	} else if c := inv.carry; c != nil {
		inv.drawIcon(&c.icon, image.Rectangle{inv.hover, inv.hover}.Inset(-rowH/2))
		if c.n > 1 {
			inv.text(strconv.Itoa(c.n), inv.hover.X+rowH/2-8*len(strconv.Itoa(c.n))+2, inv.hover.Y+rowH/2-8, white)
		}
	}

	// the menu row under the boxes; the toast (noteLine) takes its place while it shows
	rowY := (ch - 1) * 8
	inv.words = inv.menu(in)
	toast := note.Show && note.Text != ""
	if toast {
		noteLine(inv.px, note, rowY)
	}
	x := 0
	for _, w := range inv.words {
		x += 8 * (len(w.text) + 1)
	}
	x = (inv.w - x + 8) / 2
	if shop && !toast && in.Member >= 0 && in.Member < len(v.Party) { // the shop: the choices left, the buyer right
		x = 8
		m := v.Party[in.Member]
		s := m.Name + " " + m.Class
		inv.text(m.Name, inv.w-8-8*len(s), rowY, white)
		inv.text(m.Class, inv.w-8-8*len(m.Class), rowY, gray)
	}
	inv.focus = min(inv.focus, len(inv.words))
	for i := range inv.words {
		w := &inv.words[i]
		if toast { // no words to see, none to click
			w.r = image.Rectangle{}
			continue
		}
		w.r = image.Rect(x-4, rowY-2, x+8*len(w.text)+4, rowY+10)
		c, hc := green, white
		switch {
		case !w.on:
			c, hc = dark, dark
		case inv.hover.In(w.r) || inv.focus == i+1:
			inv.fill(w.r, pal.EGA(1)) // the text selection
		}
		inv.text(w.text[:1], x, rowY, hc)
		inv.text(w.text[1:], x+8, rowY, c)
		x += 8 * (len(w.text) + 1)
	}

	// the question over the list
	var box []string
	switch it := itemAt(in, inv.sel); {
	case inv.ask != nil && it != nil && inv.ask.Kind == engine.ActDrop:
		box = []string{"Your " + strings.TrimSpace(it.Name), "will be gone forever.", "", "Drop it?"}
	case inv.ask != nil && inv.ask.Kind == engine.ActBuy:
		if w := wareAt(in, inv.ware); w != nil {
			box = []string{strings.TrimSpace(w.Name), "costs " + price(w.Price) + ".", "", "Buy it?"}
		}
	case inv.ask != nil && it != nil:
		box = []string{"I'll give you " + price(it.Price), "for your " + strings.TrimSpace(it.Name) + ".", "", "Is it a deal?"}
	}
	if it := itemAt(in, inv.sel); box != nil && it != nil && it.Scribe && inv.ask.Kind != engine.ActBuy { // ovr20:0D70's question
		box = append([]string{"Someone was going to", "scribe from it.", ""}, box...)
	}

	inv.panelAt = image.Rectangle{}
	// onto the GPU: the boxes' borders, then the box's
	inv.img.WritePixels(inv.px.Pix)
	window(inv.img, t, gpu.GeoM{}, split, ch-1, nil)
	var g gpu.GeoM
	g.Translate(float64(split-1)*8, 0) // the boxes share the divider
	window(inv.img, t, g, cw-split+1, ch-1, nil)
	switch { // the headings on the top border, the party's money on the shop's bottom one
	case in.Loot:
		inv.onBorder(lx, 0, true, []string{"Loot"}, []color.RGBA{white})
	case shop:
		inv.onBorder(lx, 0, true, []string{"Shop"}, []color.RGBA{white})
		inv.onBorder(rr, (ch-1)*8-8, false, []string{"party " + price(engine.Money{Gold: in.Gold, Copper: in.Copper})}, []color.RGBA{col(14)})
	}
	inv.onBorder(rx, 0, true, []string{"Party Items"}, []color.RGBA{white})
	inv.onBorder(rr, 0, false, []string{fill}, []color.RGBA{fillc})
	if box == nil && !inv.dragging {
		inv.tooltip(t, in)
	}

	// onto the whole screen
	inv.kx, inv.ky = p.fit(dst, inv.img, inv.w, inv.h)
	if inv.facePNG != nil { // the PNG portrait at the window's resolution, as the cards
		op := &gpu.DrawImageOptions{}
		op.GeoM.Translate(float64(inv.face.Min.X)*inv.kx, float64(inv.face.Min.Y)*inv.ky)
		dst.DrawImage(p.hiFace(inv.facePNG, portraitSize*inv.k), op)
		if r := inv.panelAt; r.Overlaps(inv.face) { // the tooltip or question stays on top
			op := &gpu.DrawImageOptions{}
			op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
			op.GeoM.Scale(inv.kx, inv.ky)
			dst.DrawImage(inv.img.SubImage(r).(*gpu.Image), op)
		}
	}
	inv.askAt = nil
	if box != nil { // the question: the standard menu, over everything
		p.keyWindow(dst, strings.Join(box, " "), []string{"Yes", "No"}, []byte{'Y', 'N'}, nil, inv.askSel, false)
		inv.askAt = slices.Clone(p.winRects)
	}
	return true
}

// drawMember draws the member's name, portrait, hit points and paper doll
// with the readied items in the left box.
func (p *Presenter) drawMember(in *engine.Inventory, v engine.View, lx, lw, bottom int, busy bool) {
	inv := &p.inv

	oy := max(0, bottom-84-inv.doll.Bounds().Dy()-4) // the portrait and doll on the box's bottom, away from the name and up to three classes
	if in.Member >= 0 && in.Member < len(v.Party) {
		m := v.Party[in.Member]
		inv.text(m.Name, lx+(lw-8*len(m.Name))/2, 8, white)
		for i, cl := range strings.Split(m.Class, "/") { // a class a line: three don't fit one
			inv.text(cl, lx+(lw-8*len(cl))/2, 18+10*i, green)
		}
		if f := p.portrait(m); f != nil {
			r := image.Rect(lx+(lw-portraitSize)/2, 20+oy, lx+(lw+portraitSize)/2, 20+oy+portraitSize)
			if m.Portrait != nil { // drawn at the window's resolution over the canvas (Draw)
				inv.face, inv.facePNG = r, m.Portrait
			} else {
				draw.Draw(inv.px, r, f, f.Bounds().Min, draw.Src)
			}
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
		if it != nil && it.Index == inv.sel || inv.hover.In(box) && !busy {
			pal.SelectOutline(box, 1, inv.fill)
		}
	}
}

// onBorder draws texts in their colours, a space apart on black, over
// the borders with the right end at x (left end when left), at row y.
func (inv *inventory) onBorder(x, y int, left bool, texts []string, cols []color.RGBA) {
	n := len(texts) + 1
	for _, t := range texts {
		n += len(t)
	}
	w := 8 * n
	px := image.NewRGBA(image.Rect(0, 0, w, 8))
	draw.Draw(px, px.Rect, image.Black, image.Point{}, draw.Src)
	at := 8
	for i, t := range texts {
		drawText(px, inv.glyph, t, at, 0, cols[i])
		at += 8 * (len(t) + 1)
	}
	m := gpu.NewImage(w, 8)
	defer m.Deallocate()
	m.WritePixels(px.Pix)
	if !left {
		x -= w
	}
	op := &gpu.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	inv.img.DrawImage(m, op)
}

// tooltip tells what the pointer is on: a doll slot's item (or what goes
// there), an item's name, what it does and what it costs or sells for.
func (inv *inventory) tooltip(t [3]*gpu.Image, in *engine.Inventory) {
	var lines []string
	describe := func(it *engine.InvItem, price string) {
		lines = []string{strings.TrimSpace(it.Name), ""}
		for _, l := range it.Look {
			if !strings.HasPrefix(l, "Price ") || price == "" { // the shop's own price instead of the list price
				lines = append(lines, l)
			}
		}
		if price != "" {
			lines = append(lines, price)
		}
	}
	for i, r := range dollBoxes {
		if inv.hover.In(r.Add(inv.dollAt.Min)) {
			lines = []string{slotNames[i]}
			if it := in.Doll[i]; it != nil {
				describe(it, "")
			}
		}
	}
	for _, l := range inv.lines {
		if l.doll || !inv.hover.In(l.r) {
			continue
		}
		switch {
		case l.ware && in.Loot:
			describe(&l.item, "")
		case l.ware:
			describe(&l.item, "Costs "+price(l.item.Price))
		case in.Sell:
			describe(&l.item, "Sells for "+price(l.item.Price))
		default:
			describe(&l.item, "")
		}
	}
	if len(lines) == 0 {
		return
	}
	inv.panel(t, lines, func(w, h int) image.Point { // beside the pointer, inside the screen
		x := max(0, min(inv.hover.X+8, inv.w-w))
		y := inv.hover.Y + 12
		if y+h > inv.h {
			y = max(0, inv.hover.Y-h-4)
		}
		return image.Pt(x, y)
	})
}

// panel draws lines in a box of the game's border over the canvas (over
// the boxes' borders), the first line cyan, at(w, h) its top left.
func (inv *inventory) panel(t [3]*gpu.Image, lines []string, at func(w, h int) image.Point) {
	cw := 0
	for _, l := range lines {
		cw = max(cw, len(l))
	}
	cw = min(cw, inv.w/8-2) + 2
	rows := len(lines) + 2
	w, h := cw*8, rows*8
	p := at(w, h)
	gpu.Sized(&inv.tip, w, h)
	px := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(px, px.Rect, image.Black, image.Point{}, draw.Src)
	for i, l := range lines {
		c := gray
		if i == 0 {
			c = cyan
		}
		l = upTo(l, cw-2)
		if j := strings.LastIndex(l, " ("); j >= 0 && strings.HasSuffix(l, ")") && (l[j+2] == '+' || l[j+2] == '-') { // the Look's difference to the readied item: green better, red worse
			dc := green
			if l[j+2] == '-' {
				dc = red
			}
			drawText(px, inv.glyph, l[j:], 8+8*j, 8+8*i, dc)
			l = l[:j]
		}
		drawText(px, inv.glyph, l, 8, 8+8*i, c)
	}
	inv.tip.WritePixels(px.Pix)
	window(inv.tip, t, gpu.GeoM{}, cw, rows, nil)
	op := &gpu.DrawImageOptions{}
	op.GeoM.Translate(float64(p.X), float64(p.Y))
	inv.img.DrawImage(inv.tip, op)
	inv.panelAt = image.Rectangle{Min: p, Max: p.Add(image.Pt(w, h))}
}
