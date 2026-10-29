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
	hi := e.partySel
	if e.rosterNone {
		hi = -1
	} else if e.rosterHi != nil {
		hi = e.partyIndex(e.rosterHi)
	}
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

// partyKeypad = ovr20:27DF (key): Home selects the previous character
// (wrapping to the last), End the next (wrapping to the first: the original
// leaves its walker on the head when there is no next), anything else the first.
func (e *Engine) partyKeypad(k byte) {
	if len(e.party) == 0 {
		return
	}
	switch k {
	case 'G':
		if e.partySel == 0 {
			e.partySel = len(e.party) - 1
		} else {
			e.partySel--
		}
	case 'O':
		if e.partySel+1 < len(e.party) {
			e.partySel++
		} else {
			e.partySel = 0
		}
	default:
		e.partySel = 0
	}
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
		shown := ""
		for i := range names {
			if enabled[i] {
				shown += names[i][:1] + " "
				if i == mCreate && quick {
					shown += "Q " // in list order, so the arrows step down the list
				}
			}
		}
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
		k, extra := e.DisplayInput("Choose a function ", shown, colLightMagenta, colBlack, colBlack, true, false)
		e.regionCut(mark)
		e.ext.onSelect = nil
		e.ClearRow24()
		if extra {
			if k == 'G' || k == 'O' {
				e.partyKeypad(k)
				e.DisplayPartyList()
			}
			continue
		}
		if k != 'E' && k != 'S' {
			e.saved = false // DS:557D
		}
		switch k {
		case 'C':
			if enabled[mCreate] {
				e.CreateCharacter()
			}
		case 'D':
			if enabled[mDrop] {
				e.DropCharacter()
			}
		case 'M':
			if enabled[mModify] {
				e.ModifyCharacter()
			}
		case 'T':
			if enabled[mTrain] {
				e.TrainCharacter()
			}
		case 'V':
			if enabled[mView] {
				e.ViewCharacter()
			}
		case 'A':
			if enabled[mAdd] {
				e.AddCharacter()
			}
		case 'F':
			if enabled[mForget] {
				e.ForgetCharacter()
			}
		case 'R':
			if enabled[mRemove] && len(e.party) > 0 {
				e.SaveCharacter(e.selected(), "")
				e.RemoveSelected(false, true)
			}
		case 'L':
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
		case 'S':
			if enabled[mSave] && len(e.party) > 0 {
				e.SaveGame()
			}
		case 'B':
			if enabled[mBegin] && len(e.party) > 0 {
				e.beginFlag = 4
				e.ClearRow24()
				return
			}
		case 'E':
			if enabled[mExit] {
				if e.YesNo(colYellow, colLightGreen, colWhite, "Quit to DOS ") == 'Y' {
					for e.lettered() && !e.saved && len(e.party) > 0 {
						e.SaveGame()
					}
					e.Exit()
				}
			}
		case 'Q':
			if len(e.party) == 0 {
				e.QuickStart()
			}
		case 'J':
			if e.Cfg.Param == "STING" {
				e.debug = true
				e.BottomMessage("I Understand, master...")
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
	menu := "Select"
	if exit {
		menu += " Exit"
	}
	index := func(c *Character) int {
		for i, p := range e.party {
			if p == c {
				return i
			}
		}
		return -1
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
		{ // extra (nav.go): click a member to select it
			e.addRegion(region{x0: 0, y0: 0, x1: 0x27, y1: 0x17, click: func(cx, cy int) Action {
				if i := e.rosterRow(cx, cy); i >= 0 {
					who = e.party[i]
					return Action{Kind: actHotkey, N: 'S'}
				}
				return Action{}
			}})
		}
		key, extra := e.DisplayInput(prompt+" ", menu, colLightMagenta, colLightGreen, colWhite, true, e.beginFlag == 2 || e.beginFlag == 6) // 00BA:2C48 at 3844
		e.regionCut(mark)
		e.rosterHi, e.rosterNone = nil, false
		// The exit set (DS 00BA:2C1F) is checked before the extra flag.
		switch key {
		case 13, 'S', 0x1B:
			return who
		case 'E':
			if !extra {
				return nil
			}
			return who
		}
		if extra {
			i := index(who)
			switch key {
			case 'O', 'P': // next, wrapping to the first (P = cursor down, an extra)
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
			case 'G', 'H': // previous, wrapping to the last (H = cursor up, an extra)
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
		if key == 0 { // (Esc) the original stores nil and keeps asking
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
