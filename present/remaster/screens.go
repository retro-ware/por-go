package remaster

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"time"

	"gpu"

	"por/engine"
	"por/pal"
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
	scrSlot   // what to do with a save
)

const scrRows = 14 // the lines a page screen's menu shows at a time

type screenUI struct {
	up     string // the screen up (screenName): another one starts on its main menu
	sub    int
	sel    int
	top    int
	field  int
	prompt string         // scrType: the question ...
	text   string         // ... the line typed ...
	max    int            // ... how long it may get ...
	act    engine.ActKind // ... and what it sends
	n      int
	ask    string // scrAsk: the question; yes on Yes, back to sub on No
	yes    func()
	back   int
	slot   string           // scrSlot: the save
	cur    int              // the picker's cursor
	at     Rect             // where the page went on the window ...
	pw, ph int              // ... and its size in pixels
	anchor *engine.CellRect // the drop-down's field: the menu opens on it
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
	to := func(sub int) func() { return func() { s.sub, s.sel, s.top = sub, 0, 0 } }
	switch s.sub {
	case scrAsk:
		return s.ask, []menuRow{{"Yes", 'Y', func() { s.sub, s.sel = scrMain, 0; s.yes() }}, {"No", 'N', to(s.back)}}
	case scrType:
		t := s.text
		if p.blink() {
			t += "_"
		}
		return s.prompt, []menuRow{{t + strings.Repeat(" ", max(0, s.max+1-len(t))), 0, nil}}
	}
	switch screenName(v) {
	case "creation":
		cr := v.Creation
		switch s.sub {
		case scrChange:
			it := []menuRow{{"Name", 'N', func() { p.typeLine("Name:", "", 15, engine.ActName, 0) }}}
			for f, fl := range cr.Fields {
				it = append(it, menuRow{fl.Name, 0, func() { p.openField(cr, f) }})
			}
			for i, name := range [6]string{"STR", "INT", "WIS", "DEX", "CON", "CHA"} {
				it = append(it, menuRow{name, 0, func() { p.typeLine(name+":", "", 5, engine.ActStat, i) }})
			}
			return "Change", append(it,
				menuRow{"Head", 'H', func() { s.sub = scrMain; p.send(engine.ActPicker, engine.PickHead) }},
				menuRow{"Body", 'B', func() { s.sub = scrMain; p.send(engine.ActPicker, engine.PickBody) }},
				menuRow{"Icon", 'I', func() { s.sub = scrMain; p.send(engine.ActIconEdit, -1) }},
				menuRow{"Back", 0, to(scrMain)})
		case scrField:
			fl := cr.Fields[s.field]
			var it []menuRow
			for i, o := range fl.Options {
				t := o.Text
				if !o.Fits { // picking it changes what depends on it
					t = "(" + t + ")"
				}
				it = append(it, menuRow{t, 0, func() { s.sub, s.anchor = scrMain, nil; p.sendX(engine.ActSetField, s.field, i) }})
			}
			return "", it
		}
		return "", []menuRow{
			{"Roll", 'R', act(engine.ActRoll, 0)},
			{"Minimal", 'M', func() {
				p.typeLine("Minimums (STR INT WIS DEX CON CHA):", cr.Minimums, 30, engine.ActRoll, 1)
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
			return func() { s.sub, s.sel = scrMain, 0; p.sendA(engine.Action{Kind: k, Text: name}) }
		}
		if s.sub == scrSlot {
			it := []menuRow{{"Load", 'L', named(engine.ActLoad, s.slot)}}
			if sl.CanSave {
				it = append(it, menuRow{"Overwrite", 'O', func() { p.askThen("Overwrite "+s.slot+"?", named(engine.ActSave, s.slot)) }})
			}
			return s.slot, append(it,
				menuRow{"Delete", 'D', func() { p.askThen("Delete "+s.slot+"?", named(engine.ActDelete, s.slot)) }},
				menuRow{"Back", 'B', to(scrMain)})
		}
		var it []menuRow
		if sl.CanSave {
			it = append(it, menuRow{"New Slot", 0, func() { p.typeLine("Save as:", "", 20, engine.ActSave, 0) }})
		}
		for i, name := range sl.Names {
			it = append(it, menuRow{fmt.Sprintf("%-21s %s", upTo(name, 21), sl.Times[i]), 0, func() {
				if sl.LoadOnly {
					named(engine.ActLoad, name)()
					return
				}
				s.sub, s.sel, s.slot = scrSlot, 0, name
			}})
		}
		if len(it) == 0 {
			it = append(it, menuRow{"No saved games yet", 0, nil})
		}
		return "Saved Games", append(it, menuRow{"Back", 0, act(engine.ActClose, 0)})
	}
	return "", nil
}

func (p *Presenter) blink() bool { return time.Now().UnixMilli()/400%2 == 0 }

// typeLine starts a typed line: prompt, starting as text, sending act N
// with the line on Enter.
func (p *Presenter) typeLine(prompt, text string, n int, act engine.ActKind, an int) {
	s := &p.scr
	s.back, s.sub, s.prompt, s.text, s.max, s.act, s.n = scrMain, scrType, prompt, text, n, act, an
}

// askThen asks q; yes runs on Yes.
func (p *Presenter) askThen(q string, yes func()) {
	s := &p.scr
	s.back, s.sub, s.ask, s.yes, s.sel = s.sub, scrAsk, q, yes, 1
}

// openField opens field f's drop-down on its text, the value it holds highlighted.
func (p *Presenter) openField(cr *engine.Creation, f int) {
	s := &p.scr
	s.sub, s.field, s.sel, s.top = scrField, f, cr.Fields[f].Cur, 0
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
	}
	p.winRects = p.winRects[:0]
	if pk := v.Picker; pk != nil {
		s.cur = min(max(s.cur, 0), pk.N-1)
		if i := s.cur - pk.Page*pk.PerPage; i >= 0 && i < len(pk.Cells) {
			r := p.cellRect(pk.Cells[i])
			t := max(1, int(s.at.W/float64(s.pw)))
			c := pal.EGA(10)
			for _, e := range []image.Rectangle{{r.Min, image.Pt(r.Max.X, r.Min.Y+t)}, {image.Pt(r.Min.X, r.Max.Y-t), r.Max}, {r.Min, image.Pt(r.Min.X+t, r.Max.Y)}, {image.Pt(r.Max.X-t, r.Min.Y), r.Max}} {
				dst.SubImage(e).(*gpu.Image).Fill(c)
			}
		}
		if s.sub != scrAsk {
			return
		}
	}
	title, items := p.screenItems(v)
	lines, keys := make([]string, len(items)), make([]byte, len(items))
	for i, it := range items {
		lines[i], keys[i] = it.text, it.key
	}
	s.sel = min(max(s.sel, 0), len(lines)-1)
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
		p.drawMenu(dst, p.glyph, "", lines[s.top:end], keys[s.top:end], sel, k, cx, r.Max.Y) // under the field
		p.winRects, p.menuRects = slices.Clone(p.menuRects), rects
		p.menuBar(dst, p.winRects, len(lines), end-s.top, &s.top, &s.sel, k)
		return
	}
	k := p.keyWindow(dst, title, lines[s.top:end], keys[s.top:end], sel, screenName(v) == "creation" || screenName(v) == "icon")
	p.menuBar(dst, p.winRects, len(lines), end-s.top, &s.top, &s.sel, k)
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
		switch done, cancel, _ := editKey(k, &s.text, s.max, false); {
		case done:
			s.sub = s.back
			p.sendA(engine.Action{Kind: s.act, N: s.n, Text: s.text})
		case cancel:
			s.sub = s.back
		}
		return true
	}
	if pk := v.Picker; pk != nil && s.sub != scrAsk {
		return p.pickerKey(k, pk)
	}
	_, items := p.screenItems(v)
	n := len(items)
	keys := rowKeys(items)
	for i, it := range items {
		if it.do == nil {
			keys[i] = 0
		}
	}
	i, back, ok := menuNav(k, &s.sel, keys)
	switch {
	case back && s.sub == scrAsk:
		s.sub = s.back
	case back && s.sub != scrMain:
		s.sub, s.sel, s.anchor = scrMain, 0, nil
	case back:
		p.screenPick(v, n-1-s.top) // Exit, Back
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
	switch k {
	case 0x4800:
		c = max(c-cols, c%cols)
	case 0x5000:
		if c+cols < n {
			c += cols
		}
	case 0x4B00:
		c = max(c-1, 0)
	case 0x4D00:
		c = min(c+1, n-1)
	case 0x4700:
		c = 0
	case 0x4F00:
		c = n - 1
	case 0x4900:
		c = max(c-per, 0)
	case 0x5100:
		c = min(c+per, n-1)
	case 13, ' ', KeyPadSelect:
		p.pickerTake(pk, c)
		return true
	case 27:
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
	if pk := v.Picker; pk != nil && s.sub == scrMain {
		for i, r := range pk.Cells {
			if in(r) {
				s.cur = pk.Page*pk.PerPage + i
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
		p.typeLine("Name:", "", 15, engine.ActName, 0)
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
				p.typeLine([6]string{"STR", "INT", "WIS", "DEX", "CON", "CHA"}[i]+":", "", 5, engine.ActStat, i)
				return true
			}
		}
		return false
	}
	return true
}
