package engine

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

// PartyMenu = ovr17:0161. Returns when the player begins adventuring or
// loads a game that starts play (beginFlag != 0).
func (e *Engine) PartyMenu() {
	e.Music(3) // Amiga 0x2D35C
	e.set = 3  // DS:5376 = 3
	var names []string
	var enabled []bool
	for i, m := range partyMenu {
		names, enabled = append(names, m.name), append(enabled, m.enabled)
		if i == 5 { // extra: deletes a saved character, listed after Add
			names, enabled = append(names, "Forget Saved Character"), append(enabled, true)
		}
	}
	const (
		mCreate = iota
		mDrop
		mModify
		mTrain
		mView
		mAdd
		mForget
		mRemove
		mLoad
		mSave
		mBegin
		mExit
	)
	redraw := true
	e.debug = false
	e.beginFlag = 0
	for {
		if redraw {
			e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colBlack, "")
			if len(e.party) > 0 {
				e.DisplayPartyList()
				enabled[mDrop], enabled[mModify] = true, true
				if e.partyExp() > 0 || e.debug {
					enabled[mTrain] = true // never cleared here, only without a party
				}
				enabled[mView], enabled[mRemove] = true, true
				enabled[mLoad], enabled[mForget] = false, false      // (extra) the list fills the box
				enabled[mSave], enabled[mBegin] = e.lettered(), true // (extra) F9 saves (slots.go)
			} else {
				for _, m := range []int{mDrop, mModify, mTrain, mView, mRemove, mSave, mBegin} {
					enabled[m] = false
				}
				enabled[mLoad], enabled[mForget] = true, true
			}
			redraw = false
		}
		// The menu bar is hidden here (colours 0); the selection indicator
		// goes on the list instead, which only offers the enabled entries.
		quick := len(e.party) == 0 // extra: the quickstart entry, listed under Create
		const mQuick = mExit + 1
		var shown Bar
		for i := range names {
			if enabled[i] {
				shown = append(shown, Entry{names[i][:1], Choice(i)})
				if i == mCreate && quick {
					shown = append(shown, Entry{"Q", mQuick}) // in list order, so the arrows step down the list
				}
			}
		}
		shown = append(shown, Entry{}) // the original's trailing space
		drawList := func(hot byte) {
			row := 0
			for i := range names {
				if !enabled[i] {
					continue
				}
				bg := colBlack
				if names[i][0] == hot {
					bg = selBg(0xA, 0xF)
				}
				e.displayCharBg(2, 12+row, colWhite, bg, names[i][0], true)
				for j := 1; j < len(names[i]); j++ {
					e.displayCharBg(2+j, 12+row, colLightGreen, bg, names[i][j], true)
				}
				row++
				if i == mCreate && quick {
					bg = colBlack
					if hot == 'Q' {
						bg = selBg(0xA, 0xF)
					}
					e.displayCharBg(2, 12+row, colWhite, bg, 'Q', true)
					for j, ch := range []byte("uickstart") {
						e.displayCharBg(3+j, 12+row, colLightGreen, bg, ch, true)
					}
					row++
				}
			}
		}
		e.ext.onSelect = drawList
		mark := e.regionMark()
		{ // extra (nav.go): the function list and the roster take clicks
			row := 0
			item := func(hot byte, w int) {
				e.addRegion(region{x0: 2, y0: 12 + row, x1: 1 + w, y1: 12 + row,
					click: func(int, int) Action { return Action{Kind: actHotkey, N: int(hot)} },
					hover: func(int, int) { e.ext.nav.hot(hot) }})
				row++
			}
			for i := range names {
				if enabled[i] {
					item(names[i][0], len(names[i]))
					if i == mCreate && quick {
						item('Q', len("Quickstart"))
					}
				}
			}
			if len(e.party) > 0 {
				e.rosterRegion()
			}
		}
		e.snap.main = nil // the remaster's main menu: the functions in list order
		for i := range names {
			if enabled[i] {
				e.snap.main = append(e.snap.main, names[i])
				if i == mCreate && quick {
					e.snap.main = append(e.snap.main, "Quickstart")
				}
			}
		}
		p := e.DisplayInput("", shown, colLightMagenta, colBlack, colBlack, true, false) // extra: no "Choose a function"
		e.snap.main = nil
		e.regionCut(mark)
		e.ext.onSelect = nil
		e.ClearRow24()
		if p.extra() {
			if memberPick(p) {
				e.partyKeypad(p)
				e.DisplayPartyList()
			}
			continue
		}
		if p.ID != mExit && p.ID != mSave {
			e.saved = false // DS:557D
		}
		switch p.ID {
		case mCreate:
			if enabled[mCreate] {
				e.CreateCharacter()
			}
		case mDrop:
			if enabled[mDrop] {
				e.DropCharacter()
			}
		case mModify:
			if enabled[mModify] {
				e.ModifyCharacter()
			}
		case mTrain:
			if enabled[mTrain] {
				e.TrainCharacter()
			}
		case mView:
			if enabled[mView] {
				e.ViewCharacter()
			}
		case mAdd:
			if enabled[mAdd] {
				e.AddCharacter()
			}
		case mForget:
			if enabled[mForget] {
				e.ForgetCharacter()
			}
		case mRemove:
			if enabled[mRemove] && len(e.party) > 0 {
				e.SaveCharacter(e.selected(), "")
				e.RemoveSelected(false, true)
			}
		case mLoad:
			if enabled[mLoad] {
				if e.lettered() {
					e.LoadGame()
				} else {
					e.SlotMenu(true) // (extra) the F9 screen, loading only
				}
				if e.beginFlag != 0 {
					e.loadedGame = true // DS:49F8
					return
				}
			}
		case mSave:
			if enabled[mSave] && len(e.party) > 0 {
				e.SaveGame()
			}
		case mBegin:
			if enabled[mBegin] && len(e.party) > 0 {
				e.beginFlag = 4
				e.ClearRow24()
				return
			}
		case mExit:
			if enabled[mExit] {
				if e.YesNo(colYellow, colLightGreen, colWhite, "Quit to DOS ") {
					for e.lettered() && !e.saved && len(e.party) > 0 {
						e.SaveGame()
					}
					e.Exit()
				}
			}
		case mQuick:
			if len(e.party) == 0 {
				e.QuickStart()
			}
		}
		redraw = true
	}
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
			case DirEnd, DirDown: // next, wrapping to the first (Down is an extra)
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
			case DirHome, DirUp: // previous, wrapping to the last (Up is an extra)
				if i > 0 {
					who = e.party[i-1]
				} else {
					who = e.party[len(e.party)-1]
				}
			}
			if e.whoVar && who != nil { // opWho's var is DS:5D92 itself
				e.selectChar(who)
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
