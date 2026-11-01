package engine

import (
	"slices"
)

// Navigation that works the same way on every screen (not in the DOS
// game). The presenter sends typed actions; Apply hands them to the screen
// up (take does the screen-wide ones), which decides on their meaning
// (displayThen's question for menu bars):
//
//   - the highlight is the presenter's; in menus that take the keypad
//     ActLead selects another member (F1-F8, Left/Right)
//   - ActClose picks a menu's Exit item, so it always backs out
//   - the presenter lays the screens out and takes their clicks: it
//     sends the decisions they stand for

// Outcomes of a click (region.click) besides the presenter's actions.
const (
	actHotkey ActKind = -1 // pick the item of the waiting menu bar whose hotkey is N
	actRedraw ActKind = -2 // a screen pushed over this one is gone (feed): it draws itself again
)

// modal: a screen over the whole page is up (help, the journal, the full
// map, the overview, the F10 slots): the screens under it take no click,
// and any action closes it.
func (e *Engine) modal() bool {
	return slices.ContainsFunc(e.screens, func(s screen) bool {
		switch s.mode {
		case modeSlots, modeHelp, modeJournal, modeMap, modeOverview:
			return true
		}
		return false
	})
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
		if e.typing() || e.modal() {
			return false
		}
		e.Help()
	case ActSlots:
		if e.mode() == modeTitle || e.modal() {
			return false
		}
		e.pushSlots(false) // the next decisions go to it, wherever the engine waits (until feeds the top screen)
	case ActHead, ActPortrait: // extra: the portrait changes in play (the original only at creation and training), and at the creation
		if e.mode() != modeSheet && e.mode() != modeItems && !e.ext.creating { // only on the sheet and the inventory: a stray Shift-click elsewhere must never change a face
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
		if e.mode() == modeSheet {
			e.SheetPortrait()
		} else if e.creation != nil {
			e.creationSheet(e.creation.c) // the creation sheet draws itself anew
		}
	case ActOrder: // extra: the Party Order bar takes a dragged card
		n := len(e.party)
		if !e.up(modeCamp) || a.N < 0 || a.N >= n || a.X < 0 || a.X >= n || a.N == a.X {
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

// take does what a does on every screen; ActNone when it is used up.
func (e *Engine) take(a Action) Action {
	if a.Kind == ActOK || a.Kind == ActClose { // Enter or Esc silences the voice-over
		e.stopVoice()
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
	case ActOK, ActClose, ActChoose, actHotkey, ActOverview, ActMap, ActMinimap, ActPage, ActLead:
		return true
	case ActInventory, ActHelp, ActSlots:
		return e.modal()
	}
	return false
}
