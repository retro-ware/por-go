package remaster

import (
	"image"
	"math"

	"gpu"

	"por/engine"
)

// Lists (View.List): the presenter scrolls and highlights the lines; the
// question's options follow them. A line picked is ActChoose -1, an option
// ActChoose N, each with the line highlighted as X.

const listRows = 12 // the lines shown at a time

type listUI struct {
	sel  int   // the row highlighted: a line, then the options
	top  int   // the first line shown
	opts []int // the options shown (Menu.Items indices), as last laid out
}

// listOpts are the options offered with line i: those it does not leave out.
func listOpts(v engine.View, i int) []int {
	var out []int
outer:
	for n, it := range v.Menu.Items {
		if i >= 0 && i < len(v.List) {
			for _, w := range v.List[i].Without {
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
	return min(p.list.sel, len(v.List)-1)
}

func (p *Presenter) drawList(dst *gpu.Image, glyph func(byte) [8]byte, v engine.View, k float64, cx, top int) {
	l := &p.list
	n := len(v.List)
	l.opts = listOpts(v, p.listLine(v))
	l.sel = min(max(l.sel, 0), n+len(l.opts)-1)
	if l.sel < n {
		l.top = min(max(l.top, l.sel-listRows+1), l.sel)
	}
	l.top = max(0, min(l.top, n-listRows))
	var lines []string
	var keys []byte
	for _, ln := range v.List[l.top:min(n, l.top+listRows)] {
		lines, keys = append(lines, ln.Text), append(keys, 0)
	}
	lines, keys = append(lines, ""), append(keys, 0)
	for _, o := range l.opts {
		lines, keys = append(lines, v.Menu.Items[o].Text), append(keys, v.Menu.Items[o].Key)
	}
	p.drawMenu(dst, glyph, v.Menu.Prompt, lines, keys, p.listRow(n), k, cx, top)
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
	n := len(v.List)
	switch {
	case sel < n && !v.List[sel].Off:
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActChoose, N: -1, X: sel})
	case sel >= n && sel-n < len(p.list.opts):
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActChoose, N: p.list.opts[sel-n], X: p.listLine(v)})
	default:
		return
	}
	p.asked = ""
}

// listKey takes key k on a list.
func (p *Presenter) listKey(k uint16, v engine.View) bool {
	l := &p.list
	n := len(v.List)
	rows := n + len(l.opts)
	step := func(d int) {
		for range rows {
			l.sel = (l.sel + d + rows) % rows
			if l.sel >= n || !v.List[l.sel].Off {
				return
			}
		}
	}
	switch {
	case k == 0x4800:
		step(-1)
	case k == 0x5000:
		step(1)
	case k == 0x4900 && l.sel < n: // PgUp, PgDn
		l.sel = max(0, l.sel-listRows)
		step(1)
		step(-1)
	case k == 0x5100 && l.sel < n:
		l.sel = min(n-1, l.sel+listRows)
		step(-1)
		step(1)
	case k == 13 || k == ' ' || k == KeyPadSelect:
		p.listPick(v, l.sel)
	case k == 27:
		p.send(engine.ActClose, 0)
	case k >= KeyF1 && k <= KeyF1+7<<8 && k&0xFF == 0:
		return false
	case k < 128:
		c := byte(k)
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		for i, o := range l.opts {
			if v.Menu.Items[o].Key == c {
				p.listPick(v, n+i)
				return true
			}
		}
		return false
	default:
		return false
	}
	return true
}

// listClick takes a click on a drawn row.
func (p *Presenter) listClick(x, y int, v engine.View) bool {
	n := len(v.List)
	shown := min(n, listRows)
	for r, rc := range p.menuRects {
		if !image.Pt(x, y).In(rc) {
			continue
		}
		switch {
		case r < shown:
			p.listPick(v, p.list.top+r)
		case r > shown:
			p.listPick(v, n+r-shown-1)
		}
		return true
	}
	return false
}

// drawPageMenu pops a page's question (its options or its list) up over
// the bottom of the page.
func (p *Presenter) drawPageMenu(dst *gpu.Image, note Note, v engine.View) {
	if sheetUp(v) {
		p.pageMenu = true
		items := p.sheetItems(v)
		lines, keys := make([]string, len(items)), make([]byte, len(items))
		for i, it := range items {
			lines[i], keys[i] = it.text, it.key
		}
		p.sel = min(max(p.sel, 0), len(lines)-1)
		p.keyWindow(dst, "View:", lines, keys, p.sel, true)
		return
	}
	m := v.Menu
	if m == nil || v.Typing != nil || v.Choosing {
		return
	}
	if v.List != nil {
		b := dst.Bounds()
		k := math.Max(1, math.Floor(scale(dst)))
		p.drawList(dst, note.Glyph, v, k, b.Dx()/2, b.Dy()/2-int(k*float64(min(len(v.List), listRows)+len(m.Items)+6)*4))
		return
	}
	if len(m.Items) < 2 { // "press Enter": nothing to pick
		return
	}
	if id := menuID(m); id != p.asked {
		p.asked, p.sel = id, 0
	}
	p.pageMenu = true
	lines, keys := make([]string, len(m.Items)), make([]byte, len(m.Items))
	for i, it := range m.Items {
		lines[i], keys[i] = it.Text, it.Key
	}
	p.sel = min(max(p.sel, 0), len(lines)-1)
	p.keyWindow(dst, m.Prompt, lines, keys, p.sel, true)
}

// pageMenuKey moves the page question's highlight (unless the page takes
// the arrows) and picks it.
func (p *Presenter) pageMenuKey(k uint16, v engine.View) bool {
	if sheetUp(v) {
		return p.sheetKey(k, v)
	}
	n := len(v.Menu.Items)
	switch {
	case k == 0x4800 && !v.Menu.Arrows:
		p.sel = (p.sel + n - 1) % n
	case k == 0x5000 && !v.Menu.Arrows:
		p.sel = (p.sel + 1) % n
	case k == 13 || k == KeyPadSelect:
		p.send(engine.ActChoose, p.sel)
	default:
		return false
	}
	return true
}
