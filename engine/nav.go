package engine

import "strings"

// Navigation that works the same way on every screen (not in the DOS
// game). Keys are
// translated as the game reads them (Engine.ReadKey), so the ported
// routines only ever see keys the original understood:
//
//   - in menus the arrows move the highlight (Left/Up back, Right/Down on);
//     in menus that take the keypad, Up/Down step the selected member
//     (Home/End) and Left/Right move the highlight
//   - Esc picks a menu's Exit item, so Esc always backs out
//   - hovering a menu item highlights it, a click picks it; a click on a
//     list line moves the cursor there and a second click is Enter; clicks
//     pick party members and combat squares; the wheel moves the
//     highlight; the right button is Esc
//
// hintText names the keys that work right now (cmd/snap records it; the
// window no longer shows it, "?" lists the keys).

// Mouse events arrive as keys: KeyMouse | button<<10 | text cell (40x25).
const (
	KeyMouse       = 0xF000
	MouseLeft      = 0
	MouseRight     = 1
	MouseWheelUp   = 2
	MouseWheelDown = 3
)

// MouseKey encodes a mouse event at text cell (cx, cy).
func MouseKey(button, cx, cy int) uint16 {
	return KeyMouse | uint16(button)<<10 | uint16(cy*40+cx)
}

// keyNone is an extended key no routine acts on: a click that moved a
// cursor itself returns it, so the waiting menu loops and redraws.
const keyNone = 0xFE00

// keyRelist (extra) makes a list menu hand back to its caller to be built
// again (extras.go viewKey on the shop's Buy list).
const keyRelist = 0xFD00

// navCtx describes the menu that is waiting (displayInput keeps one per call).
type navCtx struct {
	menu    string     // the menu's items
	extraOK bool       // the menu takes keypad keys
	exit    bool       // the menu has an Exit item (and no other E hotkey)
	hot     func(byte) // highlight the item with this hotkey
}

func menuHasExit(menu string) bool {
	return strings.Contains(menu, "Exit") && strings.Count(menu, "E") == 1
}

// region is a clickable part of the screen in text cells (inclusive).
type region struct {
	x0, y0, x1, y1 int
	click          func(cx, cy int) uint16 // the key the click stands for (0: none)
	hover          func()                  // optional: the pointer came onto it
}

// modalOn opens a modal screen (help, overview, the F9 slots): any key or
// click closes it, but regions it adds itself (its own menus) take clicks.
func (e *Engine) modalOn() func() {
	floor, menuFloor := e.ext.floor, e.ext.menuFloor
	e.ext.modal++
	e.ext.floor, e.ext.menuFloor = e.regionMark(), e.ext.menu
	return func() { e.ext.modal--; e.ext.floor, e.ext.menuFloor = floor, menuFloor }
}

// addRegion makes r clickable until the list is cut back to a mark taken
// before (regionMark, regionCut). Later regions lie on top.
func (e *Engine) addRegion(r region) { e.ext.regions = append(e.ext.regions, r) }
func (e *Engine) regionMark() int    { return len(e.ext.regions) }
func (e *Engine) regionCut(n int)    { e.ext.regions = e.ext.regions[:n] }

// regionAt is the topmost region at (cx, cy) above the floor of the open
// modal screen, so the screen under it takes no clicks.
func (e *Engine) regionAt(cx, cy int) *region {
	for i := len(e.ext.regions) - 1; i >= e.ext.floor; i-- {
		r := &e.ext.regions[i]
		if cx >= r.x0 && cx <= r.x1 && cy >= r.y0 && cy <= r.y1 {
			return r
		}
	}
	return nil
}

// rawArrows marks a screen that reads the arrow keys itself, with its own
// hint line, until the returned func runs.
func (e *Engine) rawArrows(hint string) func() {
	old := e.ext.raw
	e.ext.raw = hint
	return func() { e.ext.raw = old }
}

// translate turns a key from the window into the key the game reads, or 0
// when it was used up here.
func (e *Engine) translate(k uint16) uint16 {
	if k >= KeyViewMember && k < KeyViewMember+8 { // a portrait clicked: that member's View page, or the member a list or bar is for
		n := int(k - KeyViewMember)
		switch {
		case e.ext.modal > 0 || n >= len(e.party):
		case e.ext.inCommand:
			e.partySel = n
			return 'V'
		case e.ext.campBar || e.ext.buying:
			return e.viewKey(n)
		}
		return 0
	}
	if e.ext.modal > 0 { // help and overview: any key or click closes
		switch {
		case k >= KeyMouse:
			if k>>10&3 == MouseRight {
				return 27
			}
			cell := int(k & 0x3FF)
			if r := e.regionAt(cell%40, cell/40); r != nil && k>>10&3 == MouseLeft {
				return r.click(cell%40, cell/40)
			}
			if k>>10&3 == MouseLeft {
				return 13
			}
			return 0
		case k>>8 >= 0xE0 && k&0xFF == 0:
			return 13
		case e.ext.menu > e.ext.menuFloor && (k == 0x4800 || k == 0x4B00): // a menu bar of the screen's own (the slot screen's Overwrite? Yes No)
			return ','
		case e.ext.menu > e.ext.menuFloor && (k == 0x5000 || k == 0x4D00):
			return '.'
		case e.ext.menu > e.ext.menuFloor && k == 13 && e.enterPicks():
			return uint16(e.ext.padHot)
		}
		return k
	}
	switch {
	case k == 0x06 || k == KeyFontPrev: // Ctrl-F, Ctrl-Shift-F
		if k == 0x06 {
			e.cycleFont(1)
		} else {
			e.cycleFont(-1)
		}
		return 0
	case k == '?' && !e.ext.typing:
		e.Help()
		return 0
	case k == KeyF9 && e.ext.guarded && e.tr == nil:
		e.SlotMenu(false)
		return 0
	case k >= KeyF1 && k <= KeyF1+7<<8 && k&0xFF == 0:
		return e.viewKey(int(k-KeyF1) >> 8)
	case (k == 'p' || k == 'P') && e.ext.viewing:
		return e.sheetKey()
	case k >= KeyMouse:
		cell := int(k & 0x3FF)
		return e.mouseKey(int(k>>10&3), cell%40, cell/40)
	case k>>8 >= 0xE0 && k&0xFF == 0:
		return e.padKey(k)
	case k&0xFF == 0:
		return e.navArrow(k)
	case k == 27:
		return e.escKey()
	case (k == 'm' || k == 'M') && e.ext.fullMap && !e.ext.typing: // M closes the map it opened
		return 'E'
	case k == 13 && e.enterPicks():
		return uint16(e.ext.padHot)
	case e.extraKey(byte(k)):
		return 0
	}
	return k
}

// enterPicks: Enter confirms the highlighted item of a menu bar. Lists,
// member selection and screens reading the arrows themselves (aiming,
// resting) keep the game's own meaning of Enter.
func (e *Engine) enterPicks() bool {
	x := e.ext
	return x.menu > 0 && x.padHot != 0 && !x.list && !x.choosing && !x.typing && x.raw == ""
}

// navArrow gives the arrow keys their menu meaning where the screen does
// not read them itself.
func (e *Engine) navArrow(k uint16) uint16 {
	if e.ext.menu == 0 || e.ext.inCommand || e.ext.raw != "" {
		return k
	}
	back := k == 0x4800 || k == 0x4B00
	switch k {
	case 0x4800, 0x5000:
		if e.ext.list || e.ext.choosing {
			return k // lines and members
		}
	case 0x4B00, 0x4D00:
	default:
		return k
	}
	if back {
		return ','
	}
	return '.'
}

// escKey: Esc picks the Exit item of a menu that has one.
func (e *Engine) escKey() uint16 {
	if e.ext.menu > 0 && !e.ext.inCommand && e.ext.nav.exit {
		return 'E'
	}
	return 27
}

// mouseKey turns a mouse event into a key.
func (e *Engine) mouseKey(button, cx, cy int) uint16 {
	switch button {
	case MouseRight:
		return e.escKey()
	case MouseWheelUp, MouseWheelDown:
		up := button == MouseWheelUp
		switch {
		case e.ext.paging && up: // the pagers: back, on, or scroll a picture
			return 0x4800
		case e.ext.paging:
			return 0x5000
		case e.ext.menu == 0 || e.ext.raw != "":
			return 0
		case e.ext.list || e.ext.choosing:
			if up {
				return 0x4800
			}
			return 0x5000
		case up:
			return ','
		}
		return '.'
	}
	if r := e.regionAt(cx, cy); r != nil {
		return r.click(cx, cy)
	}
	if e.ext.inCommand {
		if e.ext.minimap && e.minimapAllowed() && e.beginFlag != 0 && cx >= 0x11 && cx <= 0x26 && cy >= 1 && cy <= 0xE {
			e.FullMap() // a click on the minimap
			return 0
		}
		return e.rosterClick(cx, cy)
	}
	if e.ext.menu == 0 {
		return 13 // "press a key"; ends a typed line too
	}
	return 0
}

// rosterRow is the party member listed at cell (cx, cy), or -1.
func (e *Engine) rosterRow(cx, cy int) int {
	x := 1
	if e.beginFlag != 0 {
		x = 0x11
		if e.ext.minimap && e.minimapAllowed() {
			return -1
		}
	}
	i := cy - 4
	if cx < x || cx > 0x26 || i < 0 || i >= len(e.party) {
		return -1
	}
	return i
}

// rosterClick selects the member clicked in the roster. It sets the one
// before and sends End (next member), so the menu's own keypad handling
// runs (partyKeypad, the redraw).
func (e *Engine) rosterClick(cx, cy int) uint16 {
	i := e.rosterRow(cx, cy)
	if i < 0 {
		return 0
	}
	e.partySel = (i + len(e.party) - 1) % len(e.party)
	return 0x4F00
}

// rosterRegion makes the roster clickable for a menu that takes the keypad.
func (e *Engine) rosterRegion() {
	x := 1
	if e.beginFlag != 0 {
		x = 0x11
	}
	e.addRegion(region{x0: x, y0: 4, x1: 0x26, y1: 4 + len(e.party) - 1, click: e.rosterClick})
}

// pollMouse runs the hover action of the region the pointer moved onto.
func (e *Engine) pollMouse() {
	if e.tr != nil { // no window
		return
	}
	cx, cy, ok := e.S.Mouse()
	at := 0
	if ok {
		at = cy*40 + cx + 1
	}
	if at == e.ext.mouseAt {
		return
	}
	e.ext.mouseAt = at
	if !ok {
		return
	}
	if r := e.regionAt(cx, cy); r != nil && r.hover != nil {
		r.hover()
	}
}

// showHint updates the window's hint line.
func (e *Engine) showHint() {
	if e.tr != nil { // no window
		return
	}
	if h := e.hintText(); h != e.ext.hint {
		e.ext.hint = h
		e.S.SetHint(h)
	}
}

// hintText lists the keys of the current screen as key=action|key=action.
func (e *Engine) hintText() string {
	x := e.ext
	switch {
	case x.paging:
		return "any key, click=more|Esc, right click=close"
	case x.modal > 0:
		return "any key, click=close"
	case x.typing:
		return "type=text|Backspace=erase|Enter, click=done"
	case x.raw != "":
		return x.raw
	case x.menu == 0:
		return "any key, click=continue|?=help"
	case x.inCommand:
		h := "arrows=walk, turn, Down=back|, . wheel=highlight|Enter, click=choose|Home End, click roster=member"
		if e.viewMode != 1 {
			h = "arrows=move|, . wheel=highlight|Enter, click=choose|Home End, click roster=member"
		}
		if e.minimapAllowed() {
			h += "|Tab=minimap|M, click minimap=map"
		}
		return h + "|F1-F8=member items|P=party|?=help"
	case x.list:
		h := "↑↓ wheel, click=line|Enter, click again=choose|PgUp PgDn=page|← →=highlight"
		if x.picker {
			h = "↑↓ wheel, hover=line|Enter, click=choose|PgUp PgDn=page"
		}
		if x.buying {
			h += "|F1-F8=who gets it"
		}
		if x.nav.exit {
			h += "|Esc=back"
		}
		return h
	case x.choosing:
		h := "↑↓ wheel, click=member|Enter=select"
		if x.nav.exit {
			h += "|Esc=back"
		}
		return h
	}
	h := "arrows, wheel, hover=highlight|Enter, click=choose|letter=shortcut"
	if x.nav.extraOK {
		h += "|Home End=member"
	}
	if x.nav.exit {
		h += "|Esc=back"
	}
	if x.viewing {
		h += "|P=items|F1-F8=member"
	}
	if x.campBar {
		h += "|F1-F8=member"
	}
	return h + "|?=help"
}
