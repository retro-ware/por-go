package remaster

import (
	"image"
	"math"
	"slices"

	"gpu"

	"por/engine"
	"por/present/input"
)

// Lists (View.Ask (AskList)): the presenter scrolls and highlights the lines; the
// question's options follow them. A line picked is ActChoose -1, an option
// ActChoose N, each with the line highlighted as X.

const listRows = 12 // the lines shown at a time

type listUI struct {
	sel  int         // the row highlighted: a line, then the options
	top  int         // the first line shown
	opts []int       // the options shown (Menu.Items indices), as last laid out
	at   image.Point // the pointer, as last seen: only a moved one takes the highlight
}

// listOpts are the options offered with line i: those it does not leave out.
func listOpts(v engine.View, i int) []int {
	var out []int
outer:
	for n, it := range askMenu(v).Items {
		if i >= 0 && i < len(askList(v)) {
			for _, w := range askList(v)[i].Without {
				if w == it.Text {
					continue outer
				}
			}
		}
		out = append(out, n)
	}
	return out
}

// listLine is the line the highlight is on or was last on.
func (p *Presenter) listLine(v engine.View) int {
	return min(p.list.sel, len(askList(v))-1)
}

func (p *Presenter) drawList(dst *gpu.Image, glyph func(byte) [8]byte, v engine.View, k float64, cx, top int) {
	l := &p.list
	n := len(askList(v))
	l.opts = listOpts(v, p.listLine(v))
	l.sel = min(max(l.sel, 0), n+len(l.opts)-1)
	if l.sel < n {
		l.top = min(max(l.top, l.sel-listRows+1), l.sel)
	}
	l.top = max(0, min(l.top, n-listRows))
	var lines []string
	var off []bool
	keys := slices.Clone(p.listKeys(v)[l.top:min(n, l.top+listRows)])
	for _, ln := range askList(v)[l.top:min(n, l.top+listRows)] {
		lines, off = append(lines, ln.Text), append(off, ln.Off)
	}
	lines, keys = append(lines, ""), append(keys, 0)
	for _, o := range l.opts {
		lines, keys = append(lines, askMenu(v).Items[o].Text), append(keys, askMenu(v).Items[o].Key)
	}
	p.drawMenu(dst, glyph, askMenu(v).Prompt, lines, keys, off, p.listRow(n), k, cx, top)
	var sel *int
	if l.sel < n {
		sel = &l.sel
	}
	p.menuBar(dst, p.menuRects, n, min(n, listRows), &l.top, sel, k)
}

// listKeys are the hotkeys of the list's lines then its options: the
// options' letters, digits on the lines that can be picked.
func (p *Presenter) listKeys(v engine.View) []byte {
	n := len(askList(v))
	keys := make([]byte, n, n+len(p.list.opts))
	for _, o := range p.list.opts {
		keys = append(keys, askMenu(v).Items[o].Key)
	}
	return digitKeys(keys, func(i int) bool { return i < n && !askList(v)[i].Off })
}

// listRow is the drawn row of the highlight.
func (p *Presenter) listRow(n int) int {
	if p.list.sel < n {
		return p.list.sel - p.list.top
	}
	return min(n, listRows) + 1 + p.list.sel - n
}

// listPick takes the row sel: a line or an option.
func (p *Presenter) listPick(v engine.View, sel int) {
	n := len(askList(v))
	switch {
	case sel < n && !askList(v)[sel].Off:
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActChoose, N: -1, X: sel})
	case sel >= n && sel-n < len(p.list.opts):
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActChoose, N: p.list.opts[sel-n], X: p.listLine(v)})
	default:
		return
	}
	p.asked = ""
}

// listKey takes key k on a list: the standard menu's keys (menuNav) over
// the lines and the options; a move passes over the lines left out (Off).
func (p *Presenter) listKey(k uint16, v engine.View) bool {
	l := &p.list
	n := len(askList(v))
	if _, ok := memberKey(k, 0, len(v.Party), false); ok { // the member the list is for (keyAct)
		return false
	}
	keys := p.listKeys(v)
	i, back, ok := menuNav(k, &l.sel, keys)
	d := 1
	if a := actOf(k); a == input.Up || a == input.PgUp || a == input.End {
		d = -1
	}
	for range len(keys) {
		if l.sel >= n || !askList(v)[l.sel].Off {
			break
		}
		l.sel = (l.sel + d + len(keys)) % len(keys)
	}
	switch {
	case back:
		p.send(engine.ActClose, 0)
	case i >= 0:
		p.listPick(v, i)
	}
	return ok
}

// listAt is the row (a line, then the options) drawn as menu row r, -1 the
// blank between them.
func (p *Presenter) listAt(v engine.View, r int) int {
	n := len(askList(v))
	shown := min(n, listRows)
	switch {
	case r < shown:
		return p.list.top + r
	case r > shown:
		return n + r - shown - 1
	}
	return -1
}

// listClick takes a click on a drawn row; outside the popup it is back.
func (p *Presenter) listClick(x, y int, v engine.View) {
	r := slices.IndexFunc(p.menuRects, image.Pt(x, y).In)
	switch {
	case r < 0:
		p.send(engine.ActClose, 0)
	case p.listAt(v, r) >= 0:
		p.listPick(v, p.listAt(v, r))
	}
}

// listHover highlights the row under (x, y) when the pointer moved.
func (p *Presenter) listHover(x, y int, v engine.View) {
	if image.Pt(x, y) == p.list.at {
		return
	}
	p.list.at = image.Pt(x, y)
	if r := slices.IndexFunc(p.menuRects, image.Pt(x, y).In); r >= 0 {
		if i := p.listAt(v, r); i >= 0 && (i >= len(askList(v)) || !askList(v)[i].Off) {
			p.list.sel = i
		}
	}
}

// drawPageMenu pops a page's question (its options or its list) up over
// the bottom of the page.
func (p *Presenter) drawPageMenu(dst *gpu.Image, note Note, v engine.View) {
	if pageScreen(v) {
		p.pageMenu = true
		p.drawScreen(dst, v)
		return
	}
	m := askMenu(v)
	if v.FullMap != nil { // no bar: the cursor cell's context menu, once opened
		p.pageMenu, p.winRects = true, p.winRects[:0]
		if p.mapMenu = p.mapMenu && m != nil && typing(v) == nil; p.mapMenu {
			p.drawMapMenu(dst, v, m)
		}
		return
	}
	if m == nil || typing(v) != nil || choosing(v) {
		return
	}
	if askList(v) != nil {
		b := dst.Bounds()
		k := math.Max(1, math.Floor(scale(dst)))
		p.drawList(dst, note.Glyph, v, k, b.Dx()/2, b.Dy()/2-int(k*float64(min(len(askList(v)), listRows)+len(m.Items)+6)*4))
		return
	}
	if len(m.Items) < 2 { // "press Enter": nothing to pick
		return
	}
	if id := menuID(m); id != p.asked {
		p.asked, p.sel = id, 0
	}
	p.pageMenu = true
	rows := itemRows(m)
	lines, keys := make([]string, len(rows)), rowKeys(rows)
	for i, r := range rows {
		lines[i] = r.text
	}
	p.sel = min(max(p.sel, 0), len(lines)-1)
	p.keyWindow(dst, m.Prompt, lines, keys, nil, p.sel, true)
}

// drawMapMenu draws the full map's question m as the cursor cell's
// context menu, beside the cell (winRects its lines).
func (p *Presenter) drawMapMenu(dst *gpu.Image, v engine.View, m *engine.Menu) {
	if id := menuID(m); id != p.asked {
		p.asked, p.sel = id, 0
	}
	rows := itemRows(m)
	lines, keys := make([]string, len(rows)), rowKeys(rows)
	w := 0
	for i, r := range rows {
		lines[i] = r.text
		w = max(w, len(r.text)+4)
	}
	p.sel = min(max(p.sel, 0), len(lines)-1)
	fm, s, b := v.FullMap, &p.scr, dst.Bounds()
	kx, ky := s.at.W/float64(s.pw), s.at.H/float64(s.ph)
	c := fm.At.Add(fm.Cursor.Add(image.Pt(1, 0)).Mul(fm.Cell)) // the cell's top right, on the page
	k := math.Max(1, math.Floor(scale(dst)))
	bw, bh := int(k*float64(w*8)), int(k*float64((len(lines)+2)*8))
	x, top := int(s.at.X+float64(c.X)*kx), int(s.at.Y+float64(c.Y)*ky)
	x, top = min(x, b.Dx()-bw), min(top, b.Dy()-bh)
	rects := slices.Clone(p.menuRects) // not the game's menu: its clicks stay
	p.drawMenu(dst, p.glyph, "", lines, keys, nil, p.sel, k, x+bw/2, top)
	p.winRects, p.menuRects = slices.Clone(p.menuRects), rects
}

// pageMenuKey takes key k on a page's question: the standard menu's keys
// (menuNav), unless the page takes the arrows (the full map).
func (p *Presenter) pageMenuKey(k uint16, v engine.View) bool {
	if pageScreen(v) {
		return p.screenKey(k, v)
	}
	if sheetUp(v) {
		return p.sheetKey(k, v)
	}
	if v.FullMap != nil && !p.mapMenu { // the full map: the arrows move its cursor, Enter opens its cell's menu, Esc leaves
		switch a := actOf(k); {
		case input.Cursor(a), a == input.Esc:
			return false
		case a == input.Enter || a == input.Space || a == input.PadA:
			p.mapMenu, p.sel = true, 0
			return true
		}
	}
	if v.FullMap != nil && input.Cursor(actOf(k)) && actOf(k) != input.Up && actOf(k) != input.Down { // the menu open: Left/Right do nothing
		return true
	}
	switch i, back, ok := menuNav(k, &p.sel, rowKeys(itemRows(askMenu(v)))); {
	case back && p.mapMenu: // only the menu closes
		p.mapMenu = false
		return true
	case back:
		return false // ActClose (keyAct)
	case i >= 0:
		p.mapMenu = false
		p.send(engine.ActChoose, i)
	default:
		return ok
	}
	return true
}

// itemRows are an engine menu's items as menu rows (picking is ActChoose).
func itemRows(m *engine.Menu) []menuRow {
	if m == nil { // the question went since the last frame (a key between them)
		return nil
	}
	rows := make([]menuRow, len(m.Items))
	for i, it := range m.Items {
		rows[i] = menuRow{it.Text, it.Key, func() {}}
	}
	return rows
}
