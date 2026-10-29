package engine

import "strings"

// Menu input, ported from ovr26.

func isHotkey(c byte) bool { return c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' } // set [0-9A-Z]

// menuItems = ovr26:00CB: every [0-9A-Z] character starts an item; an item
// runs to two cells before the next hotkey. Returns (start,end) pairs for
// items 1..n; slot 0 is unused (DS locals -0x8f/-0x8e hold the count there).
func menuItems(menu string) (start, end []int) {
	start, end = []int{0, 0}, []int{0, 0}
	for i := 1; i <= len(menu); i++ {
		if !isHotkey(menu[i-1]) {
			continue
		}
		if start[len(start)-1] == 0 {
			start[len(start)-1] = i
		} else {
			end[len(end)-1] = i - 2
			start = append(start, i)
			end = append(end, 0)
		}
	}
	end[len(end)-1] = len(menu)
	return
}

// itemSpan (extra): the cells highlighted for item j, the word holding its
// hotkey ("color-1" for 1, where the original lights "1 color-"); the
// original span when the word holds more than one hotkey.
func itemSpan(menu string, start, end []int, j int) (a, b int) {
	if j < 1 || j >= len(start) || start[j] < 1 {
		return 0, 0
	}
	a, b = start[j], start[j]
	for a > 1 && menu[a-2] != ' ' {
		a--
	}
	for b < len(menu) && menu[b] != ' ' {
		b++
	}
	for i := a; i <= b; i++ {
		if i != start[j] && isHotkey(menu[i-1]) {
			return start[j], max(end[j], start[j])
		}
	}
	for { // extra: lowercase words after it belong to the item ("sWeep ahead")
		w := b + 1
		for w < len(menu) && menu[w] != ' ' {
			w++
		}
		if b >= len(menu) || w == b+1 || strings.IndexFunc(menu[b+1:w], func(r rune) bool { return isHotkey(byte(r)) }) >= 0 {
			return a, b
		}
		b = w
	}
}

// menuGlyph (extra): how menu character i (1-based) is shown. A hotkey
// inside a word reads lower case and the word's first letter upper case,
// so "sWeep" shows as "Sweep" with the w lit, "aiM" as "Aim".
func menuGlyph(menu string, i int) byte {
	c := menu[i-1]
	lower := func(c byte) bool { return c >= 'a' && c <= 'z' }
	word := menu[i-1:]
	if n := strings.IndexByte(word, ' '); n >= 0 {
		word = word[:n]
	}
	switch {
	case isHotkey(c) && c >= 'A' && i > 1 && lower(menu[i-2]):
		return c + 32
	case lower(c) && (i == 1 || menu[i-2] == ' ') && strings.IndexFunc(word, func(r rune) bool { return r >= 'A' && r <= 'Z' }) >= 0:
		return c - 32
	}
	return c
}

// drawMenu = ovr26:01DA.
func (e *Engine) drawMenu(prompt, menu string, start, end []int, sel, normal, highlight int) {
	if len(menu) == 0 {
		return
	}
	x0 := len(prompt)
	s0, e0 := itemSpan(menu, start, end, sel)
	e.ext.padHot = 0
	for i := s0; i >= 1 && i <= e0 && i <= len(menu); i++ {
		if isHotkey(menu[i-1]) {
			e.ext.padHot = menu[i-1] // gamepad "select" picks this item
			break
		}
	}
	if e.ext.onSelect != nil {
		e.ext.onSelect(e.ext.padHot)
	}
	bg := selBg(normal, highlight)
	for i := 1; i <= len(menu); i++ {
		c := menu[i-1]
		g := menuGlyph(menu, i)
		switch {
		case s0 <= i && i <= e0 && highlight != 0: // extra: the item keeps its colours on the selection background
			col := normal
			if isHotkey(c) {
				col = highlight
			}
			e.displayCharBg(x0+i-1, promptRow, col, bg, g, true)
		case isHotkey(c):
			e.DisplayChar(x0+i-1, promptRow, highlight, g, true)
		default:
			e.DisplayChar(x0+i-1, promptRow, normal, g, true)
		}
	}
	if x0+len(menu) < 0x27 {
		e.displayCharN(x0+len(menu), promptRow, colBlack, 0x27-len(menu)-x0+1, ' ', true)
	}
}

// keypadMap = DS:288C: digits 1..9 -> scan-code letters (End Down PgDn
// Left - Right Home Up PgUp).
var keypadMap = []byte("OPQK MGHI")

// DisplayInput = ovr26:03BD (prompt, menu, promptColour, normalColour,
// highlightColour, allowExtra, anim). Returns the key and whether it was a
// keypad/extended key. e.menuSel (DS:6B2B) persists between calls.
func (e *Engine) DisplayInput(prompt, menu string, promptColour, normal, highlight int, allowExtra, anim bool) (byte, bool) {
	last := e.pic.last // [bp-57] is per call: an outer DisplayInput keeps its own
	// extra: each menu starts where it was left (the original shares one highlight)
	if e.ext.selSet {
		e.ext.selSet = false
	} else {
		e.menuSel = max(e.ext.sels[menu], 1)
	}
	k, x := e.displayInput(prompt, menu, promptColour, normal, highlight, allowExtra, anim)
	if e.ext.sels == nil {
		e.ext.sels = map[string]int{}
	}
	e.ext.sels[menu] = e.menuSel
	e.pic.last = last
	e.viewAnim = 0 // 06D8 (not deferred: a trace run must stop with the original's state)
	return k, x
}

func (e *Engine) displayInput(prompt, menu string, promptColour, normal, highlight int, allowExtra, anim bool) (byte, bool) {
	saved := e.animInput // [bp+0xA], read by the wait loop (animateWhileWaiting)
	e.animInput = anim
	defer func() { e.animInput = saved }()
	if len(prompt) > 40 {
		prompt = prompt[:40]
	}
	if len(menu) > 40 {
		menu = menu[:40]
	}
	e.menuExtra = false
	e.ext.menu++
	nav, mark := e.ext.nav, e.regionMark()
	defer func() {
		e.ext.padHot = 0
		e.ext.menu--
		e.ext.nav = nav
		e.regionCut(mark)
	}()
	allowEnter := normal != 0 || highlight != 0
	start, end := menuItems(menu)
	count := len(start) - 1
	outer := e.menuShown(prompt, menu, start, end)
	defer func() { e.snap.menu = outer }()
	if e.menuSel > count || e.menuSel < 1 { // extra: never start with nothing highlighted
		e.menuSel = 1
	}
	e.pic.last = e.Ticks() // 0432: [bp-57], the animation timer starts over in every call
	e.Ticks()              // 043D: the unused timeout base
	if len(prompt) > 0 {
		e.DisplayString(0, promptRow, promptColour, prompt)
	}
	e.drawMenu(prompt, menu, start, end, e.menuSel, normal, highlight) // 048B; redrawn only on a move
	e.menuRegions(prompt, menu, start, end, normal, highlight, allowExtra)
	redraw := func() { e.drawMenu(prompt, menu, start, end, e.menuSel, normal, highlight) }
	m := &menuBar{e: e, menu: menu, start: start, count: count, allowExtra: allowExtra, allowEnter: allowEnter, redraw: redraw}
	for {
		// The input timeout DS:6B2E..6B32 is never set by the game.
		e.animateWhileWaiting()
		if !e.inputReady() { // 0566
			e.idle()
			continue
		}
		if k, x, ok := m.decide(e.input()); ok {
			e.menuExtra = x
			return k, x
		}
	}
}

// menuBar is the menu bar DisplayInput waits on.
type menuBar struct {
	e                      *Engine
	menu                   string
	start                  []int
	count                  int
	allowExtra, allowEnter bool
	redraw                 func()
}

// decide = the key handling of ovr26:03BD, on actions: what the menu hands
// back for a, if it ends the wait (ok).
func (m *menuBar) decide(a Action) (k byte, extra, ok bool) {
	e := m.e
	switch a.Kind {
	case ActPick:
		if mn := e.snap.menu; mn != nil && a.N >= 0 && a.N < len(mn.Items) {
			return m.hotkey(mn.Items[a.N].Key)
		}
	case actHotkey:
		return m.hotkey(byte(a.N))
	case ActSelect:
		if mn := e.snap.menu; mn != nil && a.N >= 0 && a.N < len(mn.Items) && e.ext.nav.hot != nil {
			e.ext.nav.hot(mn.Items[a.N].Key)
		}
	case actRedraw:
		return m.scan(menuRedraw)
	case ActContinue:
		if e.enterPicks() {
			return m.hotkey(e.ext.padHot)
		}
		return m.enter()
	case ActClose: // picks the Exit item of a menu that has one, so it always backs out
		if e.ext.modal == 0 && !e.ext.inCommand && e.ext.nav.exit {
			return m.hotkey('E')
		}
		return 0, false, true
	case ActMove:
		return m.move(a.N)
	case ActMember: // a portrait: that member's View page, or the member a list or bar is for
		switch n := a.N; {
		case e.ext.modal > 0 || n < 0 || n >= len(e.party):
		case e.ext.inCommand:
			e.partySel = n
			return m.hotkey('V')
		case e.ext.campBar || e.ext.buying:
			return m.member(n)
		}
	case ActText:
	default:
		if e.ext.modal > 0 { // the slot screen's bar: any other action is Enter
			if a.Kind == ActSlots {
				return m.scan(menuSlots)
			}
			return m.enter()
		}
		switch a.Kind {
		case ActInventory:
			return m.member(a.N)
		case ActOverview: // P: the overview, on the View page the items
			switch {
			case e.ext.viewing:
				if len(e.carried(e.selected())) > 0 {
					return m.hotkey('I')
				}
			case e.ext.inCommand:
				e.PartyOverview()
			default:
				return m.hotkey('P')
			}
		case ActMap:
			switch {
			case e.ext.fullMap && !e.ext.typing: // M closes the map it opened
				return m.hotkey('E')
			case e.ext.inCommand:
				e.FullMap()
			default:
				return m.hotkey('M')
			}
		case ActMinimap:
			if e.ext.inCommand {
				e.toggleMinimap()
			}
		}
	}
	return 0, false, false
}

// hotkey = ovr26:062A..06C2: the item with hotkey k; with the keypad
// allowed a digit is the keypad's key.
func (m *menuBar) hotkey(k byte) (byte, bool, bool) {
	k = upper(k)
	done, extra := k == ' ', false
	if isHotkey(k) {
		for i := 1; i <= len(m.menu); i++ {
			if m.menu[i-1] != k {
				continue
			}
			done = true
			for j := 1; j < len(m.start); j++ {
				if m.start[j] == i {
					m.e.menuSel = j
					m.redraw() // ovr26:0685
					break
				}
			}
		}
	}
	if m.allowExtra && (k >= '1' && k <= '9' || k == '\\') {
		if k == '\\' {
			k = '7' // 0899:06AD stores 0x37 itself, not the keypad table entry (original quirk)
		} else {
			k = keypadMap[k-'1']
		}
		extra, done = true, true
	}
	return k, extra, done
}

// scan hands back an extended key's scan letter where the menu takes them.
func (m *menuBar) scan(c byte) (byte, bool, bool) { return c, true, m.allowExtra }

// enter is Enter, taken by a menu with colours.
func (m *menuBar) enter() (byte, bool, bool) { return 13, false, m.allowEnter }

// step moves the highlight back or on, round the ends.
func (m *menuBar) step(back bool) (byte, bool, bool) {
	e := m.e
	switch {
	case back && e.menuSel == 1:
		e.menuSel = m.count
	case back:
		e.menuSel--
	case e.menuSel == m.count:
		e.menuSel = 1
	default:
		e.menuSel++
	}
	m.redraw()
	return 0, false, false
}

// move gives ActMove its meaning: the highlight (Left/Up back, Right/Down
// on), or in lists and member selection the lines and members, or the
// keypad's keys for menus that take them.
func (m *menuBar) move(d int) (byte, bool, bool) {
	x := m.e.ext
	key := func(d int) (byte, bool, bool) {
		switch d {
		case DirBack:
			return m.step(true)
		case DirOn:
			return m.step(false)
		}
		return m.scan(dirScan[d])
	}
	if d < 0 || d > DirWheelDown {
		return 0, false, false
	}
	up := d == DirUp || d == DirWheelUp
	switch {
	case x.modal > 0: // a bar of the modal screen's own
		switch d {
		case DirWheelUp, DirWheelDown:
			return 0, false, false
		case DirUp, DirLeft:
			return m.step(true)
		case DirDown, DirRight:
			return m.step(false)
		}
		return key(d)
	case d >= DirWheelUp: // the pagers: back, on
		line := DirDown
		if up {
			line = DirUp
		}
		switch {
		case x.paging:
			return key(line)
		case x.raw:
			return 0, false, false
		case x.list || x.choosing:
			return key(line)
		}
		return m.step(up)
	case x.inCommand || x.raw:
		return key(d)
	case d == DirUp || d == DirDown:
		if x.list || x.choosing {
			return key(d) // lines and members
		}
		return m.step(up)
	case d == DirLeft || d == DirRight:
		return m.step(d == DirLeft)
	}
	return key(d)
}

// member: F1..F8 (n = 0..7) on the command bar or in a menu with View
// select member n and open their inventory (View, starting on Items). On
// the View page or in the inventory they switch to member n (Exit, then
// View again), staying on the page they were on. Not in combat, where View is the fighter's.
func (m *menuBar) member(n int) (byte, bool, bool) {
	e := m.e
	if n < 0 || n >= len(e.party) || e.beginFlag == 5 {
		return 0, false, false
	}
	switch {
	case e.ext.viewing:
		if n == e.partySel {
			return 0, false, false
		}
		e.partySel = n // the View page for member n
		e.ext.held = Action{Kind: actHotkey, N: 'V'}
		return m.hotkey('E')
	case e.ext.campBar || e.ext.buying: // camp: F1-F8 pick the member the menu is for; the shop's Buy list: the buyer
		if n == e.partySel {
			return 0, false, false
		}
		e.partySel = n
		return m.scan(menuRelist)
	case e.ext.inCommand || strings.Contains(m.menu, "View"):
		e.partySel, e.ext.toItems = n, true
		return m.hotkey('V')
	}
	return 0, false, false
}

// menuRegions (extra, nav.go): the items of the menu bar highlight under
// the pointer and are picked by a click.
func (e *Engine) menuRegions(prompt, menu string, start, end []int, normal, highlight int, allowExtra bool) {
	sel := func(j int) {
		if e.menuSel != j {
			e.menuSel = j
			e.drawMenu(prompt, menu, start, end, j, normal, highlight)
		}
	}
	e.ext.nav = navCtx{menu: menu, extraOK: allowExtra, exit: menuHasExit(menu), hot: func(h byte) {
		for j := 1; j < len(start); j++ {
			if start[j] >= 1 && menu[start[j]-1] == h {
				sel(j)
				return
			}
		}
	}}
	if e.ext.inCommand {
		e.commandRegions()
	}
	for j := 1; j < len(start); j++ {
		if start[j] < 1 {
			continue
		}
		j, hot := j, menu[start[j]-1]
		a, b := itemSpan(menu, start, end, j)
		e.addRegion(region{x0: len(prompt) + a - 1, y0: 24, x1: len(prompt) + b - 1, y1: 24,
			click: func(int, int) Action { return Action{Kind: actHotkey, N: int(hot)} },
			hover: func(int, int) { sel(j) }})
	}
}

// YesNo = ovr26:10D1 (c1, c2, c3, prompt): "Yes No" on row 24, 'N' preselected.
func (e *Engine) YesNo(c1, c2, c3 int, prompt string) byte {
	e.menuSel = 2
	e.ext.selSet = true // (extra) keep the preselected No
	for {
		k, _ := e.DisplayInput(prompt, "Yes No", c1, c2, c3, false, false)
		if k == 'N' || k == 'Y' {
			return k
		}
	}
}
