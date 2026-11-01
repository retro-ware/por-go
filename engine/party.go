package engine

import "slices"

// Party menu, ported from ovr17:0161 and the list display 00BA:0767.

// DisplayPartyList = 00BA:0767 (selected): the roster with AC and HP.
// While adventuring with the minimap on (extras.go) the panel shows the
// map instead.
func (e *Engine) DisplayPartyList() {
	if e.beginFlag != 0 && e.ext.minimap && e.minimapAllowed() {
		e.DrawMinimap()
		return
	}
	e.drawPartyList()
}

// drawPartyList = 00BA:0767's page: the rows it clears; the heading, the
// names, AC and HP are the presenter's (View.Roster, View.Party).
func (e *Engine) drawPartyList() {
	if e.beginFlag == 0 { // the party menu's roster is the presenter's (View.Party)
		return
	}
	e.blank(rosterPanel.X1, 4, winRight, 4+len(e.party))
}

// rosterSel is the member the roster highlights: SelectCharacter's
// candidate while it waits (-1: none), else the selected one.
func (e *Engine) rosterSel() int {
	switch {
	case e.rosterNone:
		return -1
	case e.rosterHi != nil:
		return e.partyIndex(e.rosterHi)
	}
	return e.partySel
}

// partyKeypad = ovr20:27DF (key): any outcome of the menu but another
// member selects the first. PickRelist (extra: Home/End, a portrait,
// roster click or F1-F8 already chose the member through selectMember)
// keeps the selection.
func (e *Engine) partyKeypad(p Pick) {
	switch {
	case len(e.party) == 0 || p.ID == PickRelist:
	default:
		e.selectMember(0)
	}
}

// selectMember makes member n the selected (active) one. Every switch the
// player makes goes through it: portraits, roster clicks, F1-F8, the keypad.
func (e *Engine) selectMember(n int) {
	if n >= 0 && n < len(e.party) {
		e.partySel = n
	}
}

// PartyMenu = ovr17:0161: the party menu takes decisions (View.MainMenu)
// until the player begins adventuring or loads a game that starts play
// (beginFlag != 0), then then (nil: nothing) runs. extra: the front end
// lists the functions in its own order and asks its questions (Quit,
// Drop); there is no Save (F9 saves).
func (e *Engine) PartyMenu(then func()) {
	e.Music(3)     // Amiga 0x2D35C
	e.set = 3      // DS:5376 = 3
	if e.Explore { // -explore: straight into the 3D view (debugging the renderer); GameLoop clears it
		if len(e.party) == 0 {
			e.QuickStart()
		}
		if len(e.party) > 0 {
			e.beginFlag = 4
			if then != nil {
				then()
			}
			return
		}
	}
	e.debug = false
	e.beginFlag = 0
	redraw := func() {
		e.snap.saves = len(e.slots()) > 0
		e.ClearScreen() // the menu and the roster are the presenter's (View.MainMenu, View.Party)
	}
	redraw()
	e.push(modeParty, func(a Action) bool {
		done := e.applyParty(a)
		if !done && e.mode() == modeParty {
			redraw()
		}
		return done
	}, then)
}

// partyFunctions are the party menu's decisions open now (ovr17:0161's
// enabled entries). extra: Quick Start and Forget (a saved character)
// without a party, Load only then and with saved games.
func (e *Engine) partyFunctions() []ActKind {
	if len(e.party) == 0 {
		fs := []ActKind{ActQuickStart, ActAdd, ActCreate, ActForget, ActQuit}
		if e.snap.saves {
			fs = append([]ActKind{ActLoad}, fs...)
		}
		return fs
	}
	fs := []ActKind{ActBegin, ActAdd, ActCreate, ActDropMember, ActModify}
	if e.partyExp() > 0 || e.debug {
		fs = append(fs, ActTraining)
	}
	return append(fs, ActMember, ActRemove, ActQuit)
}

// applyParty applies a decision on the party menu; done when play starts.
// N is the member a member's function is for (< 0: the selected one).
func (e *Engine) applyParty(a Action) (done bool) {
	if a.Kind == actRedraw && e.beginFlag != 0 { // back from Load with a game
		e.loadedGame = true // DS:49F8
		return true
	}
	if a.Kind == ActLead {
		e.actor(a.N)
		return false
	}
	if !slices.Contains(e.partyFunctions(), a.Kind) {
		return false
	}
	switch a.Kind {
	case ActCreate:
		e.CreateCharacter()
	case ActModify:
		e.actor(a.N)
		e.ModifyCharacter()
	case ActAdd:
		e.AddCharacter()
	case ActForget:
		e.ForgetCharacter()
	case ActLoad: // (extra) the F9 screen, loading only
		e.pushSlots(true)
	case ActQuickStart:
		e.QuickStart()
	case ActDropMember: // ovr17:1F37, the player asked twice
		c := e.actor(a.N)
		if c.Okay == 0 {
			e.say("You dump " + c.Name() + " out back.")
		} else {
			e.say(c.Name() + " bids you farewell.")
		}
		e.RemoveSelected(false, true) // ovr17:3291
	case ActTraining:
		e.actor(a.N)
		e.pushTraining()
	case ActMember:
		e.actor(a.N)
		e.pushSheet()
	case ActRemove:
		e.SaveCharacter(e.actor(a.N), "")
		e.RemoveSelected(false, true)
	case ActBegin:
		e.beginFlag = 4
		e.clearPrompt()
		return true
	case ActQuit:
		e.Exit()
	}
	return false
}

// SelectCharacter = 00BA:2C48 (prompt, exit, var who): pick a party member
// with Next/Prev (keypad: the presenter's ActLead) and Select; Exit (offered when exit is set)
// gives nil. The roster highlights the current choice.
func (e *Engine) SelectCharacter(prompt string, exit bool, who *Character, then func(*Character)) {
	prompt = cut(prompt, textCols)
	const (
		selectIt Choice = iota
		exitIt
	)
	menu := Bar{{"Select", selectIt}}
	if exit {
		menu = append(menu, Entry{"Exit", exitIt})
	}
	if who == nil && len(e.party) > 0 {
		who = e.party[0]
	}
	done := func(c *Character) {
		e.DisplayPartyList() // (the original leaves the roster as drawn)
		then(c)
	}
	var ask func()
	ask = func() {
		// who stays highlighted while the menu waits (the view animation
		// redraws the roster).
		e.rosterHi, e.rosterNone = who, who == nil                                                               // the roster shows who (00BA:0767) once the question is up
		e.question(AskMember, prompt+" ", menu, true, true, e.beginFlag == 2 || e.beginFlag == 6, func(p Pick) { // 00BA:2C48 at 3844
			who = e.rosterHi // a member picked directly (menuBar.member)
			e.rosterHi, e.rosterNone = nil, false
			// The exit set (DS 00BA:2C1F) is checked before the extra flag.
			switch p.ID {
			case PickEnter, selectIt:
				done(who)
				return
			case exitIt:
				done(nil)
				return
			}
			if p.extra() { // ActLead stepped the candidate (who), or a click redrew
				ask()
				return
			}
			if p.ID == NoPick { // (Esc) the original stores nil and keeps asking
				who = nil
			}
			ask()
		})
	}
	ask()
}

// RemoveSelected = ovr17:3291 (keepCount, freeIcon): the selected member
// leaves the party list; the one before it (or the new first) becomes
// selected. Unless keepCount, the party count (game +0x67C) drops.
func (e *Engine) RemoveSelected(keepCount, freeIcon bool) {
	if len(e.party) == 0 {
		return
	}
	i := e.partySel
	if i >= len(e.party) {
		return
	}
	if freeIcon {
		e.freeIcon(e.party[i].Order) // ovr33:014E
	}
	e.party = append(e.party[:i], e.party[i+1:]...)
	if i > 0 {
		e.partySel = i - 1
	} else {
		e.partySel = 0
	}
	if !keepCount {
		e.game.PartyCount--
	}
}
