package engine

import (
	"slices"
)

// List menu, ported from ovr26:0D9A (the node count of ovr26:0067 is len(list)).

// ListNode is the ovr26 list node: 0x2E bytes, text string[40] at +0, flag at
// +0x29 (non-zero = header row, drawn in promptColour and skipped by the
// cursor). The next pointer (+0x2A) is the slice order.
type ListNode struct {
	Text string
	Flag byte
	Dim  bool // (extra) greyed out and passed over by the cursor, as headers
	// Without (extra): menu words left off the bar while this line is selected
	Without []string
}

// dimColour draws a greyed-out line.
const dimColour = 8

// skipped: the cursor passes over the node.
func (p *ListNode) skipped() bool { return p.Flag != 0 || p.Dim }

// newList = ovr26:1140: n empty nodes (n<2 gives 1 node). freeList is a no-op.
func newList(n int) []*ListNode {
	if n < 1 {
		n = 1
	}
	l := make([]*ListNode, n)
	for i := range l {
		l[i] = &ListNode{}
	}
	return l
}

// listMenuCtx holds the "frame" the ovr26 helpers read from listMenu's locals.
type listMenuCtx struct {
	list                           []*ListNode
	index                          *int
	x1, y1, x2, y2                 int
	promptColour, normal, hi       int
	width, rows, total, leadingHdr int
}

// nth = ovr26:0000 (nil when i >= count or negative).
func (c *listMenuCtx) nth(i int) *ListNode {
	if i < 0 || i >= len(c.list) {
		return nil
	}
	return c.list[i]
}

// drawRows = ovr26:07A4: full redraw of the list area from node first.
// (lastHeaderBefore, ovr26:0711, is computed there but never used.)
func (e *Engine) lmDrawRows(c *listMenuCtx, first int) {
	e.ClearCells(c.x1, c.y1, c.x2, c.y2)
	y := c.y1
	for i := first; i >= 0 && i < len(c.list) && y <= c.y2; i++ {
		p := c.list[i]
		col := c.normal
		if p.Flag != 0 {
			col = c.promptColour
		}
		if p.Dim {
			col = dimColour
		}
		e.DisplayString(c.x1, y, col, p.Text)
		if len(p.Text) < c.width {
			e.displayCharN(c.x1+len(p.Text), y, colBlack, c.width-len(p.Text), ' ', true) // 0709:01DF
		}
		y++
	}
}

// firstNonSpace = ovr26:08D7, lastNonSpace = ovr26:0923 (1-based).
func firstNonSpace(s string) int {
	i := 1
	for i < len(s) && s[i-1] == ' ' {
		i++
	}
	return i
}
func lastNonSpace(s string) int {
	i := len(s)
	for i > 1 && s[i-1] == ' ' {
		i--
	}
	return i
}

// lmDrawRow = ovr26:0969 (highlighted) / ovr26:0A08 (normal): recolours only the
// trimmed text of row i.
func (e *Engine) lmDrawRow(c *listMenuCtx, i int, highlighted bool) {
	p := c.nth(i)
	if p == nil {
		return
	}
	a, b := firstNonSpace(p.Text), lastNonSpace(p.Text)
	col := c.hi
	if !highlighted {
		col = c.normal
		if p.Flag != 0 {
			col = c.promptColour
		}
		if p.Dim {
			col = dimColour
		}
	}
	if b < a {
		return
	}
	if !highlighted {
		e.DisplayString(c.x1+a-1, c.y1+i-e.listTop, col, p.Text[a-1:b]) // 09FD: Copy(text, a, b-a+1)
		return
	}
	bg := selBg(c.normal, c.hi) // extra: selection background, the line keeps its colour
	col = c.normal
	if p.Flag != 0 {
		col = c.promptColour
	}
	if p.Dim {
		col = dimColour
	}
	for j := a; j <= b; j++ {
		e.displayCharBg(c.x1+j-1, c.y1+i-e.listTop, col, bg, p.Text[j-1], true)
	}
}

// lmStep moves the cursor one row with the wrap rules of ovr26:0CC5 (without
// the header skip).
func (e *Engine) lmStep(c *listMenuCtx, forward bool) {
	top := e.listTop
	if forward {
		*c.index++
		if *c.index > top+c.rows-1 {
			*c.index = top
		}
		if *c.index > c.total-1 {
			*c.index = top
		}
	} else {
		*c.index--
		if *c.index < top {
			*c.index = top + c.rows - 1
		}
		if *c.index > c.total-1 {
			*c.index = c.total - 1
		}
	}
}

// lmSkipHeaders = ovr26:0B03: up to rows times, step past header rows.
func (e *Engine) lmSkipHeaders(c *listMenuCtx, forward bool) {
	for k := 0; k < c.rows; k++ {
		if p := c.nth(*c.index); p != nil && !p.skipped() {
			break
		}
		e.lmStep(c, forward)
	}
}

// lmMoveCursor = ovr26:0CC5.
func (e *Engine) lmMoveCursor(c *listMenuCtx, forward bool) {
	from := *c.index
	e.lmStep(c, forward)
	e.lmSkipHeaders(c, forward)
	if p := c.nth(*c.index); p != nil && p.Dim { // (extra) only grey lines on the page: step through them
		*c.index = from
		e.lmStep(c, forward)
	}
}

// lmPageScroll = ovr26:0C33.
func (e *Engine) lmPageScroll(c *listMenuCtx, forward bool) {
	rel := *c.index - e.listTop
	if forward {
		e.listTop += c.rows
		if e.listTop > c.total-c.rows {
			e.listTop = c.total - c.rows
		}
	} else {
		e.listTop -= c.rows
	}
	if e.listTop < 0 { // ponytail: original word arithmetic; clamp keeps Go indexes sane
		e.listTop = 0
	}
	*c.index = e.listTop + rel
	e.lmSkipHeaders(c, forward)
	e.lmDrawRows(c, e.listTop)
}

// ListMenu = ovr26:0D9A. index is 0-based in/out (persists across calls),
// redraw is set to force a full redraw and is cleared here. Returns the
// pick (an item, PickEnter, PickRelist; NoPick on exit) and sets *selected
// to the node under the cursor (nil on exit). An item with ID PickExit
// ends the list as its own Exit does.
func (e *Engine) ListMenu(title string, menu Bar, promptColour, normal, hi, x1, y1, x2, y2 int,
	list []*ListNode, showExit bool, redraw *bool, index *int, selected **ListNode) Pick {
	if len(title) > 40 {
		title = title[:40]
	}
	if len(list) == 0 {
		*index = 0
		*selected = nil
		return Pick{ID: NoPick}
	}
	key := menu.String()
	e.menuSel = 1 // DS:6B2B
	c := &listMenuCtx{list: list, index: index, x1: x1, y1: y1, x2: x2, y2: y2,
		promptColour: promptColour, normal: normal, hi: hi,
		width: x2 - x1 + 1, rows: y2 - y1 + 1, total: len(list)}
	for c.leadingHdr < len(list) && list[c.leadingHdr].Flag != 0 {
		c.leadingHdr++
	}
	if e.listTop > *index {
		e.listTop = *index
		*redraw = true
	}
	if e.listTop > c.total {
		e.listTop = 0
		*redraw = true
	}
	*index++
	e.lmMoveCursor(c, false)
	clicked := false // the 13 came from a click on the line
	{                // extra (nav.go): the line under the pointer is highlighted, so a
		// click on it is Enter (the picked line, the highlighted menu word)
		mark := e.regionMark()
		defer e.regionCut(mark)
		at := func(cy int) int {
			i := e.listTop + cy - y1
			if p := c.nth(i); p == nil || p.skipped() {
				return -1
			}
			return i
		}
		move := func(i int) {
			e.lmDrawRow(c, *index, false)
			*index = i
			e.lmDrawRow(c, *index, true)
		}
		r := region{x0: x1, y0: y1, x1: x2, y1: y2, click: func(_, cy int) Action {
			i := at(cy)
			if i < 0 {
				return Action{}
			}
			if i == *index || len(menu) == 0 {
				*index = i
				clicked = true
				return Action{Kind: ActContinue}
			}
			move(i)
			return Action{Kind: actRedraw}
		}}
		r.hover = func(_, cy int) {
			{
				if i := at(cy); i >= 0 && i != *index {
					move(i)
				}
			}
		}
		e.addRegion(r)
	}
	if *redraw {
		e.lmDrawRows(c, e.listTop)
	}
	for {
		*redraw = false
		e.lmDrawRow(c, *index, true)
		var m Bar
		var without []string
		if p := c.nth(*index); p != nil {
			without = slices.Clone(p.Without)
		}
		for _, it := range menu {
			if i := slices.Index(without, it.Word); i >= 0 {
				without = slices.Delete(without, i, i+1)
				continue
			}
			m = append(m, it)
		}
		hasNext, hasPrev := false, false
		if c.total-c.rows > e.listTop {
			m = append(m, Entry{"Next", pickNext})
			hasNext = true
		}
		if e.listTop > c.leadingHdr {
			m = append(m, Entry{"Prev", pickPrev})
			hasPrev = true
		}
		if showExit {
			m = append(m, Entry{"Exit", PickExit})
		}
		ids := m.ids()
		if w, ok := e.ext.listWord[key]; ok { // extra: the same word stays highlighted on every page
			if j := slices.Index(ids, w); j >= 0 {
				e.ext.selSet, e.menuSel = true, j+1
			}
		}
		e.ext.list, e.ext.picker = true, len(menu) == 0
		p := e.DisplayInput(title, m, promptColour, normal, hi, true, false)
		e.ext.list, e.ext.picker = false, false
		e.lmDrawRow(c, *index, false)
		if e.menuSel >= 1 && e.menuSel <= len(ids) {
			if e.ext.listWord == nil {
				e.ext.listWord = map[string]Choice{}
			}
			e.ext.listWord[key] = ids[e.menuSel-1]
		}
		// extra: Enter picks the highlighted menu word (a click on the line
		// still takes the first)
		if p.ID == PickEnter && !clicked && len(menu) > 0 && e.menuSel >= 1 && e.menuSel <= len(ids) {
			p.ID = ids[e.menuSel-1]
		}
		clicked = false
		switch p.ID {
		case PickMove:
			switch p.Dir {
			case DirHome, DirUp:
				e.lmMoveCursor(c, false)
			case DirEnd, DirDown:
				e.lmMoveCursor(c, true)
			case DirPgUp:
				if hasPrev {
					e.lmPageScroll(c, false)
				}
			case DirPgDn:
				if hasNext {
					e.lmPageScroll(c, true)
				}
			}
		case PickRelist: // (extra) the caller rebuilds the list (another member: F1-F8)
			*selected = nil
			return p
		case PickRedraw, PickSlots:
		case pickPrev:
			e.lmPageScroll(c, false)
		case pickNext:
			e.lmPageScroll(c, true)
		case PickExit, NoPick:
			*selected = nil
			return Pick{ID: NoPick}
		default:
			*selected = c.nth(*index)
			return p
		}
	}
}

// listKey (transitional): ListMenu for callers still switching on letters.
func (e *Engine) listKey(title, menu string, promptColour, normal, hi, x1, y1, x2, y2 int,
	list []*ListNode, showExit bool, redraw *bool, index *int, selected **ListNode) byte {
	b := textBar(menu)
	if menu == "" {
		b = nil
	}
	for i := range b {
		if b[i].Word == "Exit" {
			b[i].ID = PickExit
		}
	}
	p := e.ListMenu(title, b, promptColour, normal, hi, x1, y1, x2, y2, list, showExit, redraw, index, selected)
	k, _ := legacyKey(menu, p)
	return k
}
