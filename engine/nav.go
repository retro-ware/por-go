package engine

import (
	"slices"
	"strings"
)

// Navigation that works the same way on every screen (not in the DOS
// game). The presenter sends typed actions; the ported routines read them
// (Engine.input) and decide on their meaning (DisplayInput for menu bars):
//
//   - in menus ActMove moves the highlight (Left/Up back, Right/Down on);
//     in menus that take the keypad, Up/Down step the selected member
//     (Home/End) and Left/Right move the highlight
//   - ActClose picks a menu's Exit item, so it always backs out
//   - the clickable parts of the screen are published as View.Spots: the
//     presenter sends ActHover and ActSpot for them

// Outcomes of a click (region.click) besides the presenter's actions.
const (
	actHotkey ActKind = -1 // pick the item of the waiting menu bar whose hotkey is N
	actRedraw ActKind = -2 // the click moved a cursor itself: the waiting menu hands back PickRedraw to be drawn again
	actChoice ActKind = -3 // a word of the slot screen's footer: the one whose Choice is N
)

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
	click          func(cx, cy int) Action // what the click stands for (ActNone: nothing)
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
	case ActHead, ActPortrait: // extra: the portrait changes in play (the original only at creation and training)
		if !e.ext.viewing { // only on the sheet: a stray Shift-click elsewhere must never change a face
			return false
		}
		c := e.selected()
		if a.N >= 0 && a.N < len(e.party) {
			c = e.party[a.N]
		}
		if a.Kind == ActHead {
			c.PortHead, c.Portrait = c.PortHead%0xE+1, nil // 1..14, as training's head pick
		} else if im := readPNG(a.Text); im != nil {
			c.Portrait = im
		} else {
			e.say("Not a PNG")
		}
		if e.ext.viewing {
			e.SheetPortrait()
		}
	case ActOrder: // extra: the Party Order bar takes a dragged card
		n := len(e.party)
		if !e.ext.ordering || a.N < 0 || a.N >= n || a.X < 0 || a.X >= n || a.N == a.X {
			return false
		}
		c := e.party[a.N]
		e.party = slices.Insert(slices.Delete(e.party, a.N, a.N+1), a.X, c)
		e.selectMember(a.X)
		e.DisplayPartyList()
	case ActIcon: // extra: the icon editor takes the PNG while it is up
		if f := e.snap.iconDrop; f != nil {
			f(readPNG(a.Text))
		}
	default:
		return false
	}
	return true
}

// input waits for the next action a ported routine reads. The extras
// (extraAct), the pointer and clicks are handled here: a click comes back
// as what it stands for.
func (e *Engine) input() Action {
	e.actsDone()
	for {
		if a := e.ext.held; a.Kind != ActNone {
			e.ext.held = Action{}
			return a
		}
		if a := e.take(e.nextAct()); a.Kind != ActNone {
			return a
		}
	}
}

// inputReady reports, without waiting, whether input has an action.
func (e *Engine) inputReady() bool {
	e.walkFeed()
	e.publish()
	for e.ext.held.Kind == ActNone && len(e.acts.ch) > 0 {
		e.ext.held = e.take(e.nextAct())
	}
	return e.ext.held.Kind != ActNone
}

// take does what a does on every screen; ActNone when it is used up.
func (e *Engine) take(a Action) Action {
	if a.Kind == ActContinue || a.Kind == ActClose { // Enter or Esc silences the voice-over
		e.stopVoice()
	}
	switch a.Kind {
	case ActSpot, ActHover:
		i := e.ext.floor + a.N
		if i < e.ext.floor || i >= len(e.ext.regions) {
			return Action{}
		}
		r := e.ext.regions[i]
		if a.X < r.x0 || a.X > r.x1 || a.Y < r.y0 || a.Y > r.y1 { // made on a screen since gone
			return Action{}
		}
		if a.Kind == ActSpot {
			return r.click(a.X, a.Y)
		}
		if r.hover != nil {
			r.hover(a.X, a.Y)
		}
		return Action{}
	}
	if e.extraAct(a) {
		return Action{}
	}
	return a
}

// pressed: a answers "press a key". On a modal screen (help, overview)
// the screen-wide actions close it too.
func (e *Engine) pressed(a Action) bool {
	switch a.Kind {
	case ActContinue, ActClose, ActPick, actHotkey, ActOverview, ActMap, ActMinimap:
		return true
	case ActMove:
		return a.N < DirWheelUp
	case ActInventory, ActHelp, ActSlots:
		return e.ext.modal > 0
	}
	return false
}

// anyKey waits for "press a key" and returns the action that answered.
func (e *Engine) anyKey() Action {
	for {
		if a := e.input(); e.pressed(a) {
			return a
		}
	}
}

// keyHit uses up the actions waiting and reports whether one was a key
// (the original's KeyPressed polls).
func (e *Engine) keyHit() bool {
	for e.inputReady() {
		if e.pressed(e.input()) {
			return true
		}
	}
	return false
}

// enterPicks: Enter confirms the highlighted item of a menu bar. Lists,
// member selection and screens reading the arrows themselves (aiming,
// resting) keep the game's own meaning of Enter.
func (e *Engine) enterPicks() bool {
	x := e.ext
	return x.menu > 0 && x.padHot != 0 && !x.list && !x.choosing && !x.typing && !x.raw
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

// rosterClick: a click on a member in the roster is that member's portrait.
func (e *Engine) rosterClick(cx, cy int) Action {
	if i := e.rosterRow(cx, cy); i >= 0 {
		return Action{Kind: ActMember, N: i}
	}
	return Action{}
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
		e.addRegion(region{x0: 0x11, y0: 1, x1: 0x26, y1: 0xE, click: func(int, int) Action { e.FullMap(); return Action{} }})
		return
	}
	if len(e.party) > 0 {
		e.rosterRegion()
	}
}
