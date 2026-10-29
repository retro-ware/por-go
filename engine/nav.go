package engine

import "strings"

// Navigation that works the same way on every screen (not in the DOS
// game). The presenter sends typed actions; they become the keys the
// ported routines read (Engine.readKey), so those only ever see keys the
// original understood:
//
//   - in menus ActMove moves the highlight (Left/Up back, Right/Down on);
//     in menus that take the keypad, Up/Down step the selected member
//     (Home/End) and Left/Right move the highlight
//   - ActClose picks a menu's Exit item, so it always backs out
//   - the clickable parts of the screen are published as View.Spots: the
//     presenter sends ActHover and ActSpot for them

// keyNone is an extended key no routine acts on: a click that moved a
// cursor itself returns it, so the waiting menu loops and redraws.
const keyNone = 0xFE00

// keyRelist (extra) makes a list menu hand back to its caller to be built
// again (extras.go viewKey on the shop's Buy list).
const keyRelist = 0xFD00

// keyF9 is the key the slot screen closes on (ActSlots).
const keyF9 = 0x4300

// dirKey is the BIOS key of each ActMove direction (0: the wheel's and the
// bar's own).
var dirKey = [...]uint16{DirUp: 0x4800, DirDown: 0x5000, DirLeft: 0x4B00, DirRight: 0x4D00,
	DirHome: 0x4700, DirEnd: 0x4F00, DirPgUp: 0x4900, DirPgDn: 0x5100, DirCentre: 0x4C00,
	DirBack: ',', DirOn: '.', DirWheelUp: 0, DirWheelDown: 0}

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

// region is a clickable part of the screen in text cells (inclusive),
// published as a View spot.
type region struct {
	x0, y0, x1, y1 int
	click          func(cx, cy int) uint16 // the key the click stands for (0: none)
	hover          func(cx, cy int)        // optional: the pointer came onto it
}

// modalOn opens a modal screen (help, overview, the F9 slots): any action
// closes it, but spots it adds itself (its own menus) take clicks.
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

// spots are the regions above the floor of the open modal screen, as View.Spots.
func (e *Engine) spots() []CellRect {
	var s []CellRect
	for _, r := range e.ext.regions[e.ext.floor:] {
		s = append(s, CellRect{r.x0, r.y0, r.x1, r.y1})
	}
	return s
}

// rawArrows marks a screen that reads the arrow keys itself until the
// returned func runs.
func (e *Engine) rawArrows() func() {
	old := e.ext.raw
	e.ext.raw = true
	return func() { e.ext.raw = old }
}

// extraAct runs the actions that work on every screen, and reports
// whether a was one.
func (e *Engine) extraAct(a Action) bool {
	switch a.Kind {
	case ActFont:
		e.cycleFont(a.N)
	case ActSound:
		e.soundToggle()
	case ActHelp:
		if e.ext.typing || e.ext.modal > 0 {
			return false
		}
		e.Help()
	case ActSlots:
		if !e.ext.guarded || e.ext.modal > 0 {
			return false
		}
		e.SlotMenu(false)
	default:
		return false
	}
	return true
}

// actKey turns an action into the key the game reads, or 0 when it was
// used up here.
func (e *Engine) actKey(a Action) uint16 {
	switch a.Kind {
	case ActSpot, ActHover:
		i := e.ext.floor + a.N
		if i < e.ext.floor || i >= len(e.ext.regions) {
			return 0
		}
		r := e.ext.regions[i]
		if a.X < r.x0 || a.X > r.x1 || a.Y < r.y0 || a.Y > r.y1 { // made on a screen since gone
			return 0
		}
		if a.Kind == ActSpot {
			return r.click(a.X, a.Y)
		}
		if r.hover != nil {
			r.hover(a.X, a.Y)
		}
		return 0
	case ActMember: // a portrait: that member's View page, or the member a list or bar is for
		switch n := a.N; {
		case e.ext.modal > 0 || n < 0 || n >= len(e.party):
		case e.ext.inCommand:
			e.partySel = n
			return 'V'
		case e.ext.campBar || e.ext.buying:
			return e.viewKey(n)
		}
		return 0
	case ActPick:
		if m := e.snap.menu; m != nil && a.N >= 0 && a.N < len(m.Items) {
			return uint16(m.Items[a.N].Key)
		}
		return 0
	case ActSelect:
		if m := e.snap.menu; m != nil && a.N >= 0 && a.N < len(m.Items) && e.ext.nav.hot != nil {
			e.ext.nav.hot(m.Items[a.N].Key)
		}
		return 0
	}
	if e.extraAct(a) {
		return 0
	}
	if e.ext.modal > 0 { // help and overview: any action closes
		bar := e.ext.menu > e.ext.menuFloor // a menu bar of the screen's own (the slot screen's Overwrite? Yes No)
		switch a.Kind {
		case ActClose:
			return 27
		case ActSlots:
			return keyF9
		case ActFont, ActSound, ActText:
			return 0
		case ActMove:
			switch {
			case a.N >= DirWheelUp:
				return 0
			case bar && (a.N == DirUp || a.N == DirLeft):
				return ','
			case bar && (a.N == DirDown || a.N == DirRight):
				return '.'
			}
			return dirKey[a.N]
		case ActContinue:
			if bar && e.enterPicks() {
				return uint16(e.ext.padHot)
			}
		}
		return 13
	}
	switch a.Kind {
	case ActInventory:
		return e.viewKey(a.N)
	case ActOverview:
		if e.ext.viewing {
			return e.sheetKey()
		}
		if e.extraKey('P') {
			return 0
		}
		return 'P'
	case ActMap:
		if e.ext.fullMap && !e.ext.typing { // M closes the map it opened
			return 'E'
		}
		if e.extraKey('M') {
			return 0
		}
		return 'M'
	case ActMinimap:
		if e.extraKey(9) {
			return 0
		}
		return 9
	case ActMove:
		if a.N >= 0 && a.N < len(dirKey) {
			return e.moveKey(a.N)
		}
	case ActClose:
		return e.escKey()
	case ActContinue:
		if e.enterPicks() {
			return uint16(e.ext.padHot)
		}
		return 13
	}
	return 0
}

// enterPicks: Enter confirms the highlighted item of a menu bar. Lists,
// member selection and screens reading the arrows themselves (aiming,
// resting) keep the game's own meaning of Enter.
func (e *Engine) enterPicks() bool {
	x := e.ext
	return x.menu > 0 && x.padHot != 0 && !x.list && !x.choosing && !x.typing && !x.raw
}

// moveKey gives a move its meaning on the screen: walking, a line, a
// member or the menu's highlight.
func (e *Engine) moveKey(d int) uint16 {
	if d == DirWheelUp || d == DirWheelDown { // the pagers: back, on, or scroll a picture
		up := d == DirWheelUp
		switch {
		case e.ext.paging && up:
			return 0x4800
		case e.ext.paging:
			return 0x5000
		case e.ext.menu == 0 || e.ext.raw:
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
	k := dirKey[d]
	if e.ext.menu == 0 || e.ext.inCommand || e.ext.raw {
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

// commandRegions: on the command bar the minimap opens the full map and
// the roster selects.
func (e *Engine) commandRegions() {
	if e.ext.minimap && e.minimapAllowed() && e.beginFlag != 0 {
		e.addRegion(region{x0: 0x11, y0: 1, x1: 0x26, y1: 0xE, click: func(int, int) uint16 { e.FullMap(); return 0 }})
		return
	}
	if len(e.party) > 0 {
		e.rosterRegion()
	}
}
