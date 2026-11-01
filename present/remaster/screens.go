package remaster

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"time"

	"gpu"

	"por/engine"
	"por/pal"
	"por/present/input"
)

// The page screens taking decisions whose questions are the presenter's:
// the creation sheet (View.Creation: its menu, the drop-downs, the typed
// lines), the grid pickers (View.Picker: the cursor, the frame, the
// pages), the icon editor's bar (View.IconDrop) and the save slots
// (View.Slots: the list, the name, Overwrite and Delete).

// Sub-screens of a page screen.
const (
	scrMain   = iota
	scrChange // the creation's parts, by name
	scrField  // a drop-down (scr.field)
	scrType   // a line being typed
	scrAsk    // a yes/no question
)

const scrRows = 14 // the lines a page screen's menu shows at a time

type screenUI struct {
	ptr     image.Point // the pointer's cell, as last seen: only a moved one takes the picker's cursor
	up      string      // the screen up (screenName): another one starts on its main menu
	sub     int
	sel     int
	top     int
	field   int
	prompt  string         // scrType: the question ...
	text    string         // ... the line typed ...
	caret   int            // ... its cursor ...
	max     int            // ... how long it may get ...
	act     engine.ActKind // ... and what it sends
	n       int
	ask     string // scrAsk: the question; yes on Yes, back to sub on No
	yes     func()
	back    int
	backSel int              // the row the question was asked from
	mainSel int              // the main menu's row that opened the submenu up: back restores it
	cur     int              // the picker's cursor
	at      Rect             // where the page went on the window ...
	pw, ph  int              // ... and its size in pixels
	anchor  *engine.CellRect // the drop-down's field: the menu opens on it
}

func screenName(v engine.View) string {
	switch {
	case v.Picker != nil:
		return "picker"
	case v.Creation != nil:
		return "creation"
	case v.Slots != nil:
		return "slots"
	case v.IconDrop && askMenu(v) == nil:
		return "icon"
	}
	return ""
}

// pageScreen: one of them is up.
func pageScreen(v engine.View) bool { return screenName(v) != "" }

func (p *Presenter) sendA(a engine.Action) { p.inv.acts = append(p.inv.acts, a) }

// screenItems are the lines of the page screen up and its title.
func (p *Presenter) screenItems(v engine.View) (string, []menuRow) {
	s := &p.scr
	act := func(k engine.ActKind, n int) func() { return func() { p.send(k, n) } }
	to := func(sub int) func() { return func() { s.open(sub) } }
	switch s.sub {
	case scrAsk:
		return s.ask, []menuRow{{"Yes", 'Y', func() { s.open(scrMain); s.yes() }}, {"No", 'N', func() { s.sub, s.sel = s.back, s.backSel }}}
	case scrType:
		return s.prompt, []menuRow{{caretLine(s.text, s.caret, s.max, p.blink()), 0, nil}}
	}
	switch screenName(v) {
	case "creation":
		cr := v.Creation
		switch s.sub {
		case scrChange:
			it := []menuRow{{"Name", 'N', func() { p.typeLine("Name", "", 15, engine.ActName, 0) }}}
			for f, fl := range cr.Fields {
				it = append(it, menuRow{fl.Name, 0, func() { p.openField(cr, f) }})
			}
			for i, name := range [6]string{"STR", "INT", "WIS", "DEX", "CON", "CHA"} {
				it = append(it, menuRow{name, 0, func() { p.typeLine(name, "", 5, engine.ActStat, i) }})
			}
			return "Change", append(it,
				menuRow{"Head", 'H', func() { s.open(scrMain); p.send(engine.ActPicker, engine.PickHead) }},
				menuRow{"Body", 'B', func() { s.open(scrMain); p.send(engine.ActPicker, engine.PickBody) }},
				menuRow{"Icon", 'I', func() { s.open(scrMain); p.send(engine.ActIconEdit, -1) }},
				menuRow{"Exit", 'E', to(scrMain)})
		case scrField:
			fl := cr.Fields[s.field]
			var it []menuRow
			for i, o := range fl.Options {
				t := o.Text
				if !o.Fits { // picking it changes what depends on it
					t = "(" + t + ")"
				}
				it = append(it, menuRow{t, 0, func() { s.open(scrMain); s.anchor = nil; p.sendX(engine.ActSetField, s.field, i) }})
			}
			return "", it
		}
		return "", []menuRow{
			{"Roll", 'R', act(engine.ActRoll, 0)},
			{"Minimal", 'M', func() {
				p.typeLine("Minimums (STR INT WIS DEX CON CHA)", cr.Minimums, 30, engine.ActRoll, 1)
			}},
			{"Change", 'C', to(scrChange)},
			{"Keep", 'K', act(engine.ActKeep, 0)},
			{"Exit", 'E', func() { p.askThen(cr.Abandon, func() { p.send(engine.ActClose, 0) }) }}}
	case "icon":
		return "", []menuRow{
			{"Head", 'H', act(engine.ActPicker, engine.PickIconHead)},
			{"Weapon", 'W', act(engine.ActPicker, engine.PickWeapon)},
			{"Size", 'S', act(engine.ActIconSize, 0)},
			{"Keep", 'K', act(engine.ActKeep, 0)},
			{"Exit", 'E', act(engine.ActClose, 0)}}
	case "slots":
		sl := v.Slots
		named := func(k engine.ActKind, name string) func() {
			return func() { s.open(scrMain); p.sendA(engine.Action{Kind: k, Text: name}) }
		}
		var it []menuRow
		if sl.CanSave {
			it = append(it, menuRow{"New Slot", 'N', func() { p.typeLine("Save as", "", 20, engine.ActSave, 0) }})
		}
		for i, name := range sl.Names {
			it = append(it, menuRow{fmt.Sprintf("%-21s %s", upTo(name, 21), sl.Times[i]), 0, func() {
				if !sl.CanSave {
					named(engine.ActLoad, name)()
					return
				}
				p.askThen("Overwrite "+name+"?", named(engine.ActSave, name))
			}})
		}
		if len(it) == 0 {
			it = append(it, menuRow{"No saved games yet", 0, nil})
		}
		return "Saved Games", append(it, menuRow{"Exit", 'E', act(engine.ActClose, 0)})
	}
	return "", nil
}

func (p *Presenter) blink() bool { return time.Now().UnixMilli()/400%2 == 0 }

// typeLine starts a typed line: prompt, starting as text, sending act N
// with the line on Enter.
func (p *Presenter) typeLine(prompt, text string, n int, act engine.ActKind, an int) {
	s := &p.scr
	s.open(scrType)
	s.back, s.sub, s.prompt, s.text, s.caret, s.max, s.act, s.n = scrMain, scrType, prompt, text, len(text), n, act, an
}

// askThen asks q; yes runs on Yes.
func (p *Presenter) askThen(q string, yes func()) {
	s := &p.scr
	if s.sub == scrMain {
		s.mainSel = s.sel
	}
	s.back, s.backSel, s.sub, s.ask, s.yes, s.sel = s.sub, s.sel, scrAsk, q, yes, 1
}

// openField opens field f's drop-down on its text, the value it holds highlighted.
func (p *Presenter) openField(cr *engine.Creation, f int) {
	s := &p.scr
	s.open(scrField)
	s.field, s.sel = f, cr.Fields[f].Cur
	at := cr.Fields[f].At
	s.anchor = &at
}

// drawScreen draws the page screen up over its page: its menu, the
// picker's frame.
func (p *Presenter) drawScreen(dst *gpu.Image, v engine.View) {
	s := &p.scr
	if n := screenName(v); n != s.up {
		s.up, s.sub, s.sel, s.top, s.anchor = n, scrMain, 0, 0, nil
		if v.Picker != nil {
			s.cur = v.Picker.Cur
		}
		if sl := v.Slots; sl != nil { // on the game last loaded or saved
			if i := slices.Index(sl.Names, sl.Current); i >= 0 && sl.Current != "" {
				s.sel = i
				if sl.CanSave {
					s.sel++ // under New Slot
				}
			}
		}
	}
	p.winRects = p.winRects[:0]
	if pk := v.Picker; pk != nil {
		s.cur = min(max(s.cur, 0), pk.N-1)
		if i := s.cur - pk.Page*pk.PerPage; i >= 0 && i < len(pk.Cells) {
			r := p.cellRect(pk.Cells[i])
			pal.SelectOutline(r, max(1, int(s.at.W/float64(s.pw))), func(e image.Rectangle, c color.RGBA) { dst.SubImage(e).(*gpu.Image).Fill(c) })
		}
		if s.sub != scrAsk {
			return
		}
	}
	title, items := p.screenItems(v)
	lines, keys, off := make([]string, len(items)), p.screenKeys(v, items), rowsOff(items)
	for i, it := range items {
		lines[i] = it.text
	}
	if s.sub == scrType { // the line being typed is no row to pick
		off[0] = false
	}
	s.sel = min(max(s.sel, 0), len(lines)-1)
	if screenName(v) == "slots" && s.sub == scrMain {
		p.slotList(dst, lines, keys, off)
		return
	}
	s.top = min(max(s.top, s.sel-scrRows+1), s.sel, max(0, len(lines)-scrRows))
	end := min(len(lines), s.top+scrRows)
	sel := s.sel - s.top
	if s.sub == scrType {
		sel = -1
	}
	if a := s.anchor; a != nil && s.sub == scrField { // the drop-down on its field
		k := s.at.W / float64(s.pw)
		w := 0
		for _, l := range lines {
			w = max(w, len(l))
		}
		r := p.cellRect(*a)
		cx := r.Min.X - int(k*8) + int(k*float64((w+4)*8))/2 // the text over the field's
		rects := slices.Clone(p.menuRects)
		p.drawMenu(dst, p.glyph, "", lines[s.top:end], keys[s.top:end], off[s.top:end], sel, k, cx, r.Max.Y) // under the field
		p.winRects, p.menuRects = slices.Clone(p.menuRects), rects
		p.menuBar(dst, p.winRects, len(lines), end-s.top, &s.top, &s.sel, k)
		return
	}
	k := p.keyWindow(dst, title, lines[s.top:end], keys[s.top:end], off[s.top:end], sel, screenName(v) == "creation" || screenName(v) == "icon")
	p.menuBar(dst, p.winRects, len(lines), end-s.top, &s.top, &s.sel, k)
}

// slotKey does the slot page's footer key h on the selected save: Enter
// picks the row, L loads, D deletes after a Yes, Esc leaves.
func (p *Presenter) slotKey(v engine.View, h byte) {
	s, sl := &p.scr, v.Slots
	switch h {
	case footPick:
		p.screenPick(v, s.sel-s.top)
		return
	case footBack:
		p.send(engine.ActClose, 0)
		return
	}
	i := s.sel
	if sl.CanSave {
		i-- // New Slot
	}
	if i < 0 || i >= len(sl.Names) {
		return
	}
	name := sl.Names[i]
	switch h {
	case 'L':
		p.sendA(engine.Action{Kind: engine.ActLoad, Text: name})
	case 'D':
		p.askThen("Delete "+name+"?", func() { s.open(scrMain); p.sendA(engine.Action{Kind: engine.ActDelete, Text: name}) })
	}
}

// slotFootAt is the footer word of the slot page under (x, y), or 0.
func (p *Presenter) slotFootAt(v engine.View, x, y int) byte {
	s := &p.scr
	if v.Slots == nil || s.sub != scrMain || s.pw == 0 {
		return 0
	}
	for _, w := range slotFooter(v.Slots, s.pw/8) {
		if image.Pt(x, y).In(p.cellRect(engine.CellRect{X1: w.x, Y1: s.ph/8 - 3, X2: w.x + len(w.label) - 1, Y2: s.ph/8 - 3})) {
			return w.key
		}
	}
	return 0
}

// screenKeys are the hotkeys of the page screen's rows: the saves on the
// slot list have none (no digits either), the rest as every menu.
func (p *Presenter) screenKeys(v engine.View, items []menuRow) []byte {
	if screenName(v) != "slots" || p.scr.sub != scrMain {
		return rowKeys(items)
	}
	k := make([]byte, len(items))
	for i, r := range items {
		k[i] = r.key
	}
	return k
}

// slotList draws the save slots' list on the page itself, inside the
// Saved Games box (screenPage): a row a line from row 3 down to above
// "No saving now", the name at column 3 and the time at 25, as the
// original-style F10 page; the rows scroll with the selection.
func (p *Presenter) slotList(dst *gpu.Image, lines []string, keys []byte, off []bool) {
	s := &p.scr
	cols, n := s.pw/8, max(1, s.ph/8-8)
	s.top = min(max(s.top, s.sel-n+1), s.sel, max(0, len(lines)-n))
	end := min(len(lines), s.top+n)
	w := cols - 4 // columns 2 to cols-3, inside the border
	if gpu.Sized(&p.menuImg, w*8, (end-s.top)*8) {
		p.menuBuf = make([]byte, w*8*(end-s.top)*8*4)
	}
	clear(p.menuBuf)
	set := func(x, y int, c color.RGBA) {
		o := (y*w*8 + x) * 4
		p.menuBuf[o], p.menuBuf[o+1], p.menuBuf[o+2], p.menuBuf[o+3] = c.R, c.G, c.B, 255
	}
	for r := s.top; r < end; r++ {
		y0 := (r - s.top) * 8
		if r == s.sel {
			for y := y0; y < y0+8; y++ {
				for x := range w * 8 {
					set(x, y, pal.EGA(1))
				}
			}
		}
		t, hk := hotText(lines[r], keys[r])
		for i := 0; i < len(t) && i+1 < w; i++ {
			c := pal.EGA(10)
			switch {
			case off[r]:
				c = pal.EGA(8)
			case i == hk:
				c = pal.EGA(15)
			}
			g := p.glyph(t[i])
			for y := range 8 {
				for b := range 8 {
					if g[y]&(0x80>>b) != 0 {
						set((i+1)*8+b, y0+y, c)
					}
				}
			}
		}
	}
	p.menuImg.WritePixels(p.menuBuf)
	at := p.cellRect(engine.CellRect{X1: 2, Y1: 3, X2: cols - 3, Y2: 3 + end - s.top - 1})
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(float64(at.Dx())/float64(w*8), float64(at.Dy())/float64((end-s.top)*8))
	op.GeoM.Translate(float64(at.Min.X), float64(at.Min.Y))
	dst.DrawImage(p.menuImg, op)
	for r := s.top; r < end; r++ {
		p.winRects = append(p.winRects, p.cellRect(engine.CellRect{X1: 2, Y1: 3 + r - s.top, X2: cols - 3, Y2: 3 + r - s.top}))
	}
	p.menuBar(dst, p.winRects, len(lines), end-s.top, &s.top, &s.sel, float64(at.Dy())/float64((end-s.top)*8))
}

// cellRect is a cell rectangle of the page on the window.
func (p *Presenter) cellRect(c engine.CellRect) image.Rectangle {
	s := &p.scr
	kx, ky := s.at.W/float64(s.pw), s.at.H/float64(s.ph)
	return image.Rect(int(s.at.X+float64(c.X1*8)*kx), int(s.at.Y+float64(c.Y1*8)*ky), int(s.at.X+float64((c.X2+1)*8)*kx), int(s.at.Y+float64((c.Y2+1)*8)*ky))
}

// screenPick picks shown line i of the page screen's menu.
func (p *Presenter) screenPick(v engine.View, i int) {
	_, items := p.screenItems(v)
	if i += p.scr.top; i >= 0 && i < len(items) && items[i].do != nil {
		p.scr.sel = i
		items[i].do()
	}
}

// screenKey takes key k on a page screen.
func (p *Presenter) screenKey(k uint16, v engine.View) bool {
	s := &p.scr
	if s.sub == scrType {
		switch done, cancel, _ := editKey(k, &s.text, &s.caret, s.max, false); {
		case done:
			s.open(s.back)
			p.sendA(engine.Action{Kind: s.act, N: s.n, Text: s.text})
		case cancel:
			s.open(s.back)
		}
		return true
	}
	if pk := v.Picker; pk != nil && s.sub != scrAsk {
		return p.pickerKey(k, pk)
	}
	if sl := v.Slots; sl != nil && s.sub == scrMain {
		h := byte(k)
		if h >= 'a' && h <= 'z' {
			h -= 'a' - 'A'
		}
		switch actOf(k) { // the pad's X and Y: L and D
		case input.Overview:
			h, k = 'L', 'L'
		case input.Minimap:
			h, k = 'D', 'D'
		}
		if (h == 'L' && !sl.LoadOnly || h == 'D') && k < 128 {
			p.slotKey(v, h)
			return true
		}
	}
	_, items := p.screenItems(v)
	n := len(items)
	keys := p.screenKeys(v, items)
	i, back, ok := menuNav(k, &s.sel, keys)
	switch {
	case back && s.sub == scrAsk:
		s.sub, s.sel = s.back, s.backSel
	case back && s.sub != scrMain:
		s.open(scrMain)
		s.anchor = nil
	case back:
		p.screenPick(v, n-1-s.top) // Exit
	case i >= 0:
		p.screenPick(v, i-s.top)
	}
	return ok
}

// pickerKey moves the picker's cursor (turning the page under it), Enter
// takes the picture, Esc leaves.
func (p *Presenter) pickerKey(k uint16, pk *engine.Picker) bool {
	s := &p.scr
	c, n, cols, per := s.cur, pk.N, pk.Cols, pk.PerPage
	switch actOf(k) {
	case input.Up:
		c = max(c-cols, c%cols)
	case input.Down:
		if c+cols < n {
			c += cols
		}
	case input.Left:
		c = max(c-1, 0)
	case input.Right:
		c = min(c+1, n-1)
	case input.Home:
		c = 0
	case input.End:
		c = n - 1
	case input.PgUp:
		c = max(c-per, 0)
	case input.PgDn:
		c = min(c+per, n-1)
	case input.Enter, input.Space, input.PadA:
		p.pickerTake(pk, c)
		return true
	case input.Esc:
		p.send(engine.ActClose, 0)
		return true
	default:
		return false
	}
	p.pickerTo(pk, c)
	return true
}

// pickerTo moves the cursor to c, on its page.
func (p *Presenter) pickerTo(pk *engine.Picker, c int) {
	p.scr.cur = c
	if pg := c / pk.PerPage; pg != pk.Page {
		p.send(engine.ActPage, pg)
	}
}

// pickerTake takes picture i, asking first where the picker asks.
func (p *Presenter) pickerTake(pk *engine.Picker, i int) {
	if i < 0 || i >= pk.N {
		return
	}
	take := func() { p.send(engine.ActPicked, i) }
	if pk.Ask != "" && i < len(pk.Labels) {
		p.askThen(fmt.Sprintf(pk.Ask, pk.Labels[i]), take)
		return
	}
	take()
}

// screenMouse takes the pointer on a page screen's page at cell (cx, cy):
// the picker's pictures and pager, the creation sheet's parts, the icon
// editor's.
func (p *Presenter) screenMouse(cx, cy int, left bool, v engine.View) bool {
	s := &p.scr
	in := func(r engine.CellRect) bool { return image.Pt(cx, cy).In(image.Rect(r.X1, r.Y1, r.X2+1, r.Y2+1)) }
	moved := image.Pt(cx, cy) != s.ptr
	s.ptr = image.Pt(cx, cy)
	if pk := v.Picker; pk != nil && s.sub == scrMain {
		for i, r := range pk.Cells {
			if in(r) {
				if moved || left {
					s.cur = pk.Page*pk.PerPage + i
				}
				if left {
					p.pickerTake(pk, s.cur)
				}
				return true
			}
		}
		switch {
		case !left:
		case pk.Prev != (engine.CellRect{}) && in(pk.Prev) && pk.Page > 0:
			p.pickerTo(pk, (pk.Page-1)*pk.PerPage)
		case pk.Next != (engine.CellRect{}) && in(pk.Next) && pk.Page < pk.Pages-1:
			p.pickerTo(pk, (pk.Page+1)*pk.PerPage)
		default:
			return false
		}
		return true
	}
	if ed := v.IconEditor; ed != nil && left && s.sub == scrMain {
		a, ok := iconClick(ed, cx, cy)
		if ok {
			p.sendA(a)
		}
		return ok
	}
	cr := v.Creation
	if cr == nil || !left || v.Picker != nil || s.sub != scrMain {
		return false
	}
	switch {
	case in(cr.Name):
		p.typeLine("Name", "", 15, engine.ActName, 0)
	case in(cr.Head):
		p.send(engine.ActPicker, engine.PickHead)
	case in(cr.Body):
		p.send(engine.ActPicker, engine.PickBody)
	case in(cr.Icon):
		p.send(engine.ActIconEdit, -1)
	default:
		for f, fl := range cr.Fields {
			if in(fl.At) {
				p.openField(cr, f)
				return true
			}
		}
		for i, r := range cr.Stats {
			if in(r) {
				p.typeLine([6]string{"STR", "INT", "WIS", "DEX", "CON", "CHA"}[i], "", 5, engine.ActStat, i)
				return true
			}
		}
		return false
	}
	return true
}

// open goes to submenu sub: back on the main menu, the row that opened the
// submenu comes back; leaving the main menu keeps that row; a submenu
// starts on its first row.
func (s *screenUI) open(sub int) {
	switch {
	case sub == scrMain:
		s.sel = s.mainSel
	case s.sub == scrMain:
		s.mainSel, s.sel = s.sel, 0
	default:
		s.sel = 0
	}
	s.sub, s.top = sub, 0
}
