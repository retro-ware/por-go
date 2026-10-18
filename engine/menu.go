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
		if e.ext.menu == 0 && e.tr == nil {
			e.S.SetHint("")
			e.ext.hint = ""
		}
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
	extra := false
	e.drawMenu(prompt, menu, start, end, e.menuSel, normal, highlight) // 048B; redrawn only on a move
	e.menuRegions(prompt, menu, start, end, normal, highlight, allowExtra)
	for {
		// The input timeout DS:6B2E..6B32 is never set by the game.
		e.animateWhileWaiting()
		e.pollMouse()
		e.showHint()
		if !e.keyWaiting() { // 0566
			e.idle()
			continue
		}
		k := e.ReadKeyByte()
		if k == 0 {
			k = e.ReadKeyByte()
			if e.ext.acted { // extra: a click did its action itself (nav.go keyNone)
				e.ext.acted = false
				return byte(keyNone >> 8), false
			}
			if allowExtra {
				e.menuExtra = true
				return k, true
			}
			continue
		}
		switch k {
		case 27:
			return 0, false
		case 13:
			if allowEnter {
				return 13, false
			}
			continue
		case ',':
			if e.menuSel == 1 {
				e.menuSel = count
			} else {
				e.menuSel--
			}
			e.drawMenu(prompt, menu, start, end, e.menuSel, normal, highlight)
			continue
		case '.':
			if e.menuSel == count {
				e.menuSel = 1
			} else {
				e.menuSel++
			}
			e.drawMenu(prompt, menu, start, end, e.menuSel, normal, highlight)
			continue
		}
		k = upper(k)
		done := false
		if k == ' ' || k != 0 && strings.IndexByte(e.ext.hiddenKeys, k) >= 0 {
			done = true
		} else if isHotkey(k) {
			for i := 1; i <= len(menu); i++ {
				if menu[i-1] != k {
					continue
				}
				done = true
				for j := 1; j < len(start); j++ {
					if start[j] == i {
						e.menuSel = j
						e.drawMenu(prompt, menu, start, end, e.menuSel, normal, highlight) // ovr26:0685
						break
					}
				}
			}
		}
		if allowExtra && (k >= '1' && k <= '9' || k == '\\') {
			if k == '\\' {
				k = '7' // 0899:06AD stores 0x37 itself, not the keypad table entry (original quirk)
			} else {
				k = keypadMap[k-'1']
			}
			extra, done = true, true
		}
		if done {
			e.menuExtra = extra
			return k, extra
		}
	}
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
	for j := 1; j < len(start); j++ {
		if start[j] < 1 {
			continue
		}
		j, hot := j, menu[start[j]-1]
		a, b := itemSpan(menu, start, end, j)
		e.addRegion(region{x0: len(prompt) + a - 1, y0: 24, x1: len(prompt) + b - 1, y1: 24,
			click: func(int, int) uint16 { return uint16(hot) },
			hover: func() { sel(j) }})
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
