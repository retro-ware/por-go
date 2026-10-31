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

func (e *Engine) drawPartyList() {
	x := 1
	if e.beginFlag != 0 {
		x = 0x11
	}
	y := 2
	e.DisplayString(x, y, colWhite, "Name")
	e.DisplayString(33, y, colWhite, "AC  HP")
	y += 2
	hi := e.rosterSel()
	for i, c := range e.party {
		e.ClearCells(x, y, winRight, y)
		if i == hi && e.ext.choosing {
			// extra: the member being chosen gets the menu selection background
			for j := 0; j < len(c.Name()); j++ {
				e.displayCharBg(x+j, y, colWhite, selBg(0xB, 0xF), c.Name()[j], true)
			}
		} else if i == hi {
			e.DisplayString(x, y, colWhite, c.Name())
		} else if c.Okay != 0 && c.Side == sideParty && levelReady(c) {
			e.DisplayString(x, y, colLightMagenta, c.Name()) // extra: ready for a level
		} else {
			e.displayName(c, x, y, false)
		}
		ac := c.AC[0]
		dx := 0
		switch {
		case ac <= 0x32, ac >= 0x3D && ac <= 0x45:
			dx = 1
		case ac >= 0x33 && ac <= 0x3C:
			dx = 2
		}
		e.displayAC(c, 0x20+dx, y)
		hp := c.HP
		dx = 0
		switch {
		case hp <= 9:
			dx = 2
		case hp <= 0x63:
			dx = 1
		}
		e.displayHP(c, 0x24+dx, y, false)
		y++
	}
	e.ClearCells(x, y, winRight, y)
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

// partyKeypad = ovr20:27DF (key): Home selects the previous character
// (wrapping to the last), End the next (wrapping to the first: the original
// leaves its walker on the head when there is no next), any other outcome
// of the menu the first.
// PickRelist (extra: a portrait, roster click or F1-F8 already chose the
// member through selectMember) keeps the selection.
func (e *Engine) partyKeypad(p Pick) {
	n := len(e.party)
	switch {
	case n == 0 || p.ID == PickRelist:
	case p.ID == PickMove && p.Dir == DirHome:
		e.selectMember((e.partySel + n - 1) % n)
	case p.ID == PickMove && p.Dir == DirEnd:
		e.selectMember((e.partySel + 1) % n)
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

// memberPick: the pick changes the selected member (Home, End, or a
// member chosen directly: PickRelist).
func memberPick(p Pick) bool {
	return p.ID == PickRelist || p.ID == PickMove && (p.Dir == DirHome || p.Dir == DirEnd)
}

// PartyMenu = ovr17:0161: the party menu takes decisions (View.MainMenu)
// until the player begins adventuring or loads a game that starts play
// (beginFlag != 0). extra: the front end lists the functions in its own
// order and asks its questions (Quit, Drop); there is no Save (F9 saves).
func (e *Engine) PartyMenu() {
	e.Music(3)     // Amiga 0x2D35C
	e.set = 3      // DS:5376 = 3
	if e.Explore { // -explore: straight into the 3D view (debugging the renderer); GameLoop clears it
		if len(e.party) == 0 {
			e.QuickStart()
		}
		if len(e.party) > 0 {
			e.beginFlag = 4
			return
		}
	}
	e.debug = false
	e.beginFlag = 0
	redraw := func() {
		e.snap.saves = len(e.slots()) > 0
		e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colBlack, "")
		if len(e.party) > 0 {
			e.DisplayPartyList()
		}
	}
	redraw()
	e.await(modeParty, func(a Action) bool {
		done := e.applyParty(a)
		if !done && e.mode == modeParty {
			redraw()
		}
		return done
	})
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
		e.ClearRow24()
		return true
	case ActQuit:
		e.Exit()
	}
	return false
}

// SelectCharacter = 00BA:2C48 (prompt, exit, var who): pick a party member
// with Next/Prev (keypad) and Select; Exit (offered when exit is set)
// gives nil. The roster highlights the current choice.
func (e *Engine) SelectCharacter(prompt string, exit bool, who *Character) *Character {
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
	e.ext.choosing = true
	defer func() {
		e.ext.choosing = false
		e.DisplayPartyList() // (the original leaves the roster as drawn)
	}()
	for {
		// who stays highlighted while the menu waits (the view animation
		// redraws the roster).
		e.rosterHi, e.rosterNone = who, who == nil
		e.DisplayPartyList() // 00BA:0767 (who)
		mark := e.regionMark()
		e.rosterRegion()                                                                                                            // extra (nav.go): a click on a member picks it, like a portrait or F1-F8 (menuBar.member)
		p := e.DisplayInput(prompt+" ", menu, colLightMagenta, colLightGreen, colWhite, true, e.beginFlag == 2 || e.beginFlag == 6) // 00BA:2C48 at 3844
		e.regionCut(mark)
		who = e.rosterHi // a member picked directly (menuBar.member)
		e.rosterHi, e.rosterNone = nil, false
		// The exit set (DS 00BA:2C1F) is checked before the extra flag.
		switch p.ID {
		case PickEnter, selectIt:
			return who
		case exitIt:
			return nil
		}
		if p.extra() {
			i := e.partyIndex(who)
			switch p.Dir {
			case DirEnd, DirDown, DirRight: // next, wrapping to the first (Down and Right are extras)
				if who == nil {
					// nil^.next reads 0000:0104 (the INT 41h vector, garbage in
					// the original); the harness pins it to nil, and nil wraps to
					// the first member (00BA:2D40)
					who = e.party[0]
				} else if i+1 < len(e.party) {
					who = e.party[i+1]
				} else {
					who = e.party[0]
				}
			case DirHome, DirUp, DirLeft: // previous, wrapping to the last (Up and Left are extras)
				if i > 0 {
					who = e.party[i-1]
				} else {
					who = e.party[len(e.party)-1]
				}
			}
			continue
		}
		if p.ID == NoPick { // (Esc) the original stores nil and keeps asking
			who = nil
		}
	}
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
