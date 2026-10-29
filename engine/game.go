package engine

// The adventuring loop: ovr04:31B4/32F8 (game loop and area change),
// ovr15 (command input, movement, doors) and ovr21 (clock).

// GameLoop = ovr04:32F8: load the current area, run its start events, then
// take commands until the script quits.
func (e *Engine) GameLoop() {
	r := &e.run
	e.ext.inGame = true
	r.walldefSet, r.geoSet = 0, 0
	r.restoreSel = false
	r.scriptLoad = 0
	r.needFrame = 1
	e.beginFlag = 4
	r.areaChanged = false
	e.gs.areaKind = byte(e.area.AreaKind)
	fresh := !e.loadedGame // extra: a new adventure, not a loaded game
	if k := e.gs.areaKind; (k == 0x19 || k == 0x1A || k == 0x1B) && e.area.In3D == 0 {
		e.beginFlag = 3
	}
	// the area's text script, also for a loaded game (whose save holds
	// the byte code)
	e.LoadArea(int(e.gs.areaKind))
	e.InitAreaScript()
	e.RunECL(e.es.events[4])
	if e.beginFlag != 3 {
		if r.needFrame == 1 {
			e.DrawMainScreen()
			r.needFrame = 0
		}
		e.Redraw()
	}
	if fresh {
		e.Music(4) // extra: the party first enters Phlan (the Amiga never plays song 4)
	}
	e.area.AreaKind = uint16(e.gs.areaKind)
	encamp := e.CommandInput()
	r.savedSel = e.selected()
	for {
		e.area.AreaKind = uint16(e.gs.areaKind)
		// Look / encamp: repeat while the last command was one of them.
		for (e.game.Search > 1 || encamp) && !r.quit {
			if encamp {
				e.Encamp()
			} else {
				r.searchSave = byte(e.game.Search & 1)
				e.game.Search = 1
				e.Redraw()
				e.RunECL(e.es.events[1])
				if r.areaChanged {
					e.AreaChange()
				}
				e.game.Search = uint16(r.searchSave)
			}
			if !r.quit {
				encamp = e.CommandInput()
				r.savedSel = e.selected()
			}
		}
		if !r.quit {
			e.RunECL(e.es.events[0])
		}
		if r.areaChanged {
			e.unback() // a backward step the scripts sent elsewhere
			e.AreaChange()
		} else if !r.quit {
			e.area.PrevX = uint16(int16(int8(e.posX)))
			e.area.PrevY = uint16(int16(int8(e.posY)))
			e.Move()
			e.Redraw()
			if int(e.area.PrevX) != e.posX || int(e.area.PrevY) != e.posY {
				e.PlaySound(sndStep)
			}
			e.es.picShown, e.es.picAnimating = 0, 1
			e.RunECL(e.es.events[1])
			if r.areaChanged {
				e.AreaChange()
			}
		}
		if r.quit {
			break
		}
		encamp = e.CommandInput() // ovr04:34FC jumps back to 3395: the write follows the input
		r.savedSel = e.selected()
	}
	r.quit = false
}

// AreaChange = ovr04:31B4: after a script switched areas, run the new
// area's start events and settle the view mode.
func (e *Engine) AreaChange() {
	r := &e.run
	for {
		e.es.cell = e.MapCell(e.posX, e.posY)
		e.game.LeftMap = 0
		sel := e.selected() // DS:5D92 kept at bp-4 (ovr04:31D5)
		e.prevViewMode = e.viewMode
		e.viewMode = 1
		e.RunECL(e.es.events[4])
		e.beginFlag = 4
		if e.area.In3D == 0 {
			switch e.gs.areaKind {
			case 0x19:
				e.viewMode, e.beginFlag = 2, 3
			case 0x1A:
				e.viewMode, e.beginFlag = 3, 3
				if e.area.Boss == 0xFE && e.area.RiverOpen == 0 {
					e.EndgameHook(1)
				}
			case 0x1B:
				e.viewMode, e.beginFlag = 4, 3
			}
		}
		if e.prevViewMode > 1 && e.viewMode == 1 {
			e.DrawBoxTitled(3, 3, 13, 13, 0, colLightGreen, "")
		}
		if (e.prevViewMode > 1 || e.viewMode == 1) && r.scriptLoad != 0 ||
			e.prevViewMode == 1 && e.viewMode == 1 {
			e.Redraw()
		}
		r.areaChanged = false
		e.RunECL(e.es.events[0])
		if !r.areaChanged {
			e.RunECL(e.es.events[1])
			e.selectChar(sel)
			e.DisplayPartyList()
		}
		if !r.areaChanged {
			return
		}
	}
}

// Encamp = ovr04:2DD3: the camp event, the camp menu, then the area's
// after-camp event.
func (e *Engine) Encamp() {
	e.RunECL(e.es.events[2])
	if e.CampMenu() {
		e.DrawMainScreen()
		e.RunECL(e.es.events[3])
	}
	e.Redraw()
}

// CommandInput = ovr15:0989: the adventuring command bar, until a command
// ends input (Encamp, Search here, a step). True for Encamp.
func (e *Engine) CommandInput() (encamp bool) {
	e.game.HoldMove = 0
	e.areaMusic()
	e.unback() // a backward step the scripts ended
	switch {
	case e.viewMode == 1:
		for done := false; !done; {
			if f := e.DevAction; f != nil { // headless checks (POR_DEV)
				e.DevAction = nil
				f()
			}
			e.ext.inCommand = true
			p := e.DisplayInput("", e.commandBar(commandBar3D), colLightMagenta, colLightGreen, colWhite, true, false)
			e.ext.inCommand = false
			encamp = p.ID == cmdEncamp
			if !p.extra() {
				done = e.command(p.ID, true)
			} else {
				switch p.Dir {
				case DirUp:
					e.checkEdge()
					done = true
				case DirDown:
					// (extra) Down steps backwards, facing kept (the original turns round): Move and the step scripts go the way back, the screen never shows it (viewFacing)
					e.facing = (e.facing + 4) % 8
					e.ext.back = true
					e.es.wallAhead = e.WallAt(e.posX, e.posY, e.facing) // the step scripts read it (City Hall: 0xC04E 11 is the locked door)
					e.checkEdge()
					done = true
				case DirLeft:
					e.turn(6, true)
				case DirRight:
					e.turn(2, true)
				default:
					e.partyKeypad(p)
					e.DisplayPartyList()
				}
			}
			e.StatusLine()
		}
	case e.viewMode >= 2 && e.viewMode <= 4:
		if e.prevViewMode < 2 {
			e.LoadIconPic("ICON", 0, 9)
			e.LoadIconPic("ICON", 1, 0xA)
			e.LoadIconPic("ICON", 2, 0xB)
			e.areaMapIcons() // 00BA:2B5E
		}
		for done := false; !done; {
			e.ext.inCommand = true
			p := e.DisplayInput("", e.commandBar(commandBar3D), colLightMagenta, colLightGreen, colWhite, true, false)
			e.ext.inCommand = false
			encamp = p.ID == cmdEncamp
			if !p.extra() {
				done = e.command(p.ID, false)
			} else {
				done = true
				switch p.Dir {
				case DirUp:
					e.facing = 0
				case DirRight:
					e.facing = 2
				case DirDown:
					e.facing = 4
				case DirLeft:
					e.facing = 6
				default:
					e.partyKeypad(p)
					e.DisplayPartyList()
					done = false
				}
			}
			e.StatusLine()
		}
	}
	e.prevViewMode = e.viewMode
	if e.run.textCleared == 0 {
		e.ClearCells(1, msgTop, winRight, winBottom)
		e.run.textCleared = 1
	}
	return encamp
}

// commandBar3D = the 3D view's command bar (DS:04B6) without Area: the
// minimap (Tab, extras.go) shows all Area showed.
var commandBar3D = Bar{{"Cast", cmdCast}, {"Inv", cmdInv}, {"View", cmdView}, {"Encamp", cmdEncamp}, {"sWeep", cmdSweep}, {"Search", cmdSearch}}

// The commands of the command bar.
const (
	cmdCast Choice = iota
	cmdInv
	cmdView
	cmdEncamp
	cmdSweep
	cmdSearch
	cmdSpells // extra: a card's context menu: the member's powers (ActSpells..ActCure, in that order)
	cmdMagic
	cmdHeal
	cmdCure
)

// command carries out a command of the command bar (the original's Area is
// replaced by the minimap, Tab). Returns true when input is over.
func (e *Engine) command(k Choice, view3D bool) bool {
	switch k {
	case cmdCast:
		e.CastMenu() // ovr16:04A1; canStudy says why a member in no condition cannot (the original ignores the key on the 3D view)
	case cmdInv: // extra: straight to the inventory
		if len(e.carried(e.selected())) == 0 {
			e.BottomMessage("No items")
		} else {
			e.ext.toItems = true
			e.ViewCharacter()
		}
	case cmdView:
		e.ViewCharacter() // ovr20:0AB3
	case cmdSpells, cmdMagic, cmdHeal, cmdCure: // extra: a card's context menu
		e.power(e.selected(), ActSpells+ActKind(k-cmdSpells))
		e.DrawMainScreen()
	case cmdEncamp:
		return true
	case cmdSweep: // Sweep (the original's Search): the slow, searching pace on or off
		e.game.Search = e.game.Search ^ 1
	case cmdSearch: // Search (the original's Look): search this square once
		e.game.Search = e.game.Search | 2
		e.AdvanceClock(3, 1) // extra: 3 minutes (the original took 10)
		e.eclPC = e.es.events[1]
		return true
	}
	return false
}

// turn = the turn keys of ovr15:0989: face dir+d, look again.
func (e *Engine) turn(d int, sound bool) {
	e.facing = (e.facing + d) % 8
	if sound {
		e.PlaySound(sndStep)
	}
	e.es.wallAhead = e.WallAt(e.posX, e.posY, e.facing)
	e.Draw3DView(e.posX, e.posY, e.facing)
	e.viewDrawn()
}

// checkEdge = ovr15:068D: game+0x5AA is set when stepping through an open
// side would leave the map.
func (e *Engine) checkEdge() {
	x, y := e.posX, e.posY
	e.game.LeftMap = 0
	if e.DoorAt(x, y, e.facing) == 0 {
		return
	}
	x += dirDX[e.facing]
	y += dirDY[e.facing]
	if x > 15 {
		e.posX = 15
		e.game.LeftMap = 1
	}
	if x < 0 {
		e.posX = 0
		e.game.LeftMap = 1
	}
	if y > 15 {
		e.posY = 15
		e.game.LeftMap = 1
	}
	if y < 0 {
		e.posY = 0
		e.game.LeftMap = 1
	}
}

// viewFacing (extra) is the facing shown: a step backwards (Down) turns the
// party round only for the move and the step scripts, never on screen.
func (e *Engine) viewFacing() int {
	if e.ext.back {
		return (e.facing + 4) % 8
	}
	return e.facing
}

// unback (extra): after a step backwards (Down) the party faces the way it
// did before.
func (e *Engine) unback() {
	if e.ext.back {
		e.ext.back = false
		e.facing = (e.facing + 4) % 8
		e.es.wallAhead = e.WallAt(e.posX, e.posY, e.facing)
	}
}

// Move = ovr15:0D6D: step forward in the 3D view (through a door if it can
// be opened) or across the area map.
func (e *Engine) Move() {
	defer e.unback()
	switch {
	case e.viewMode == 1:
		if e.game.HoldMove >= 0xFF {
			e.game.HoldMove = 0
			break
		}
		moved := false
		switch e.DoorAt(e.posX, e.posY, e.facing) {
		case 1:
			moved = true
		case 2:
			moved = e.lockedDoor(false)
		case 3:
			moved = e.lockedDoor(true)
		}
		if moved {
			e.stepAhead()
		} else {
			e.PlaySound(sndBump)
		}
		e.unback()
		e.StatusLine()
	case e.viewMode >= 2 && e.viewMode <= 4:
		if e.game.HoldMove < 0xFF {
			e.mapMove()
			e.StatusLine()
		} else {
			e.game.HoldMove = 0
		}
	}
	e.FreeAnimPic()
	e.port = portrait{headID: 0xFF, bodyID: 0xFF} // 0464:2595 x2
}

// lockedDoor = the door menu of ovr15:0D6D. A plain locked door (state 2)
// offers Bash, Pick and Knock; a wizard-locked one (3) only lets Bash work.
func (e *Engine) lockedDoor(wizard bool) bool {
	const (
		bash Choice = iota
		pick
		knockIt
		exit
	)
	menu := Bar{{}} // without Bash the bar starts with a space (as in the original)
	if e.run.doorTry[0] != 0 {
		menu[0] = Entry{"Bash", bash}
	}
	if e.run.doorTry[1] != 0 && e.partyHasSkill(6) {
		menu = append(menu, Entry{"Pick", pick})
	}
	knock := false
	if wizard {
		knock = e.partyHasSkill(5)
	} else {
		knock = e.findCaster(spKnock) != nil
	}
	if e.run.doorTry[2] != 0 && knock {
		menu = append(menu, Entry{"Knock", knockIt})
	}
	menu = append(menu, Entry{"Exit", exit})
	if menu.String() == " Exit" {
		e.setDoor(e.posX, e.posY, e.facing, 0)
		return false
	}
	switch e.DisplayInput("Locked. ", menu, colLightMagenta, colLightGreen, colWhite, false, false).ID {
	case bash:
		return e.bashDoor()
	case pick:
		if wizard {
			e.run.doorTry[1] = 0
			return false
		}
		return e.pickLock()
	case knockIt:
		return e.knock()
	}
	return false
}

// setDoor = ovr15:0000 (state 0) and ovr15:0109 (state 1): set the door
// bits of one side of a cell. 0109 ignores cells off the map.
func (e *Engine) setDoor(x, y, dir int, state byte) {
	if !onMap(x, y) {
		return
	}
	shift := uint(dir) // N 0, E 2, S 4, W 6
	i := 0x300 + y<<4 + x
	e.gs.geo[i] = e.gs.geo[i]&^(3<<shift) | state<<shift
}

// openDoor opens the side ahead from both cells (ovr15:0109 twice).
func (e *Engine) openDoor() {
	x, y, d := e.posX, e.posY, e.facing
	e.setDoor(x, y, d, 1)
	e.setDoor(x+dirDX[d], y+dirDY[d], (d+4)%8, 1)
}

// partyHasSkill = ovr15:0236 (n): someone has a positive thief skill
// +0x96+n.
func (e *Engine) partyHasSkill(n int) bool {
	for _, c := range e.party {
		if int8(c.Levels[n]) > 0 {
			return true
		}
	}
	return false
}

// bashDoor = ovr15:0294: each member in turn tries to force the door.
func (e *Engine) bashDoor() bool {
	ok := false
	d6 := func(max int) bool { return e.Dice(1, 6) <= max }
	for _, c := range e.party {
		if ok {
			break
		}
		str, exc := c.Stats[statStr], c.Str00
		if e.DoorAt(e.posX, e.posY, e.facing) == 3 {
			switch {
			case str == 0x12:
				switch {
				case exc >= 0x5B && exc <= 0x63:
					ok = e.Dice(1, 6) == 1
				case exc == 0x64:
					ok = d6(2)
				default:
					e.run.doorTry[0] = 0
				}
			case str == 0x13 || str == 0x14:
				ok = d6(3)
			case str == 0x15 || str == 0x16:
				ok = d6(4)
			case str == 0x17:
				ok = d6(5)
			case str == 0x18:
				ok = e.Dice(1, 8) <= 7
			case str == 0x19:
				ok = true
			default:
				e.run.doorTry[0] = 0
			}
			continue
		}
		switch {
		case str >= 3 && str <= 7:
			ok = e.Dice(1, 6) == 1
		case str >= 8 && str <= 0xF:
			ok = d6(2)
		case str >= 0x10 && str <= 0x11:
			ok = d6(3)
		case str == 0x12:
			switch {
			case exc <= 0x32:
				ok = d6(3)
			case exc <= 0x63:
				ok = d6(4)
			case exc == 0x64:
				ok = d6(5)
			}
		}
	}
	if ok {
		e.openDoor()
	}
	return ok
}

// pickLock = ovr15:04DF: d100 against each healthy member's +0x78 skill.
func (e *Engine) pickLock() bool {
	ok := false
	for _, c := range e.party {
		if ok {
			break
		}
		if e.Dice(1, 100) <= int(c.Thief[1]) && c.Status == statusOkay {
			ok = true
		}
	}
	e.run.doorTry[1] = 0
	if ok {
		e.openDoor()
	}
	return ok
}

// spellSlot = ovr15:05AB (c, spell): index of the memorized spell, or 0xFF.
func spellSlot(c *Character, spell byte) int {
	for i := 0; i <= 0x14; i++ {
		if c.Memorized[i] == spell {
			return i
		}
	}
	return 0xFF
}

// findCaster = ovr15:05EE (spell): first member with the spell memorized.
func (e *Engine) findCaster(spell byte) *Character {
	for _, c := range e.party {
		if spellSlot(c, spell) != 0xFF {
			return c
		}
	}
	return nil
}

// knock = ovr15:0648: use up a memorized Knock.
func (e *Engine) knock() bool {
	c := e.findCaster(spKnock)
	if c == nil {
		return false
	}
	e.noteSpells(c)
	c.Memorized[spellSlot(c, spKnock)] = 0
	return true
}

// stepAhead = ovr15:073B: walk one cell (wrapping), look, and let time pass.
func (e *Engine) stepAhead() {
	// the step sound plays once the loop sees the square change (ovr04:3395)
	e.posX = wrap(e.posX + dirDX[e.facing])
	e.posY = wrap(e.posY + dirDY[e.facing])
	e.es.wallAhead = e.WallAt(e.posX, e.posY, e.facing)
	e.run.doorTry = [3]byte{1, 1, 1}
	e.es.cell = e.MapCell(e.posX, e.posY)
	if e.game.Search&1 != 0 {
		e.AdvanceClock(3, 1) // extra: 3 minutes a sweeping step (the original took 10)
	} else {
		e.AdvanceClock(1, 1)
	}
}

// mapMove = ovr15:0818: one step on the area map, unless the target square
// is impassable (DS:3648 map, 0x2C bytes a row).
func (e *Engine) mapMove() {
	x := min(max(int(int8(byte(int(e.area.MapX)+dirDX[e.facing]))), 0), 0xF)
	y := min(max(int(int8(byte(int(e.area.MapY)+dirDY[e.facing]))), 0), 0x23)
	col := int(int8(byte(int(viewModeShift[e.viewMode]) + x)))
	t := terrainMap[y][col]
	switch {
	case t == 0x4E, t >= 0x7E && t <= 0x80, t >= 0x82 && t <= 0x84,
		t >= 0xA8 && t <= 0xAC, t >= 0xAE && t <= 0xB3, t == 0xBC,
		t >= 0xDD && t <= 0xDE, t >= 0xE1 && t <= 0xE9, t == 0xF5:
		return
	case t >= 0x66 && t <= 0x6C, t == 0x72:
		if e.area.RiverOpen == 0 {
			e.DisplayMessageWait(colYellow, "The river's stench drives you back.")
			return
		}
	}
	e.area.MapX = uint16(int16(x))
	e.area.MapY = uint16(int16(y))
	e.AdvanceClock(0xC, 3)
}

// clockLimits = DS:363A: rounds per minute, minutes per ten, tens per
// hour, hours, days, months, years.
var clockLimits = [7]int{10, 10, 6, 24, 30, 12, 0x100}

// normalizeClock = ovr21:02F2: carry each unit into the next; a year
// passing ages the party.
func (e *Engine) normalizeClock(t *[7]int) {
	for i := 0; i <= 6; i++ {
		if t[i] < clockLimits[i] {
			continue
		}
		if i == 6 {
			for _, c := range e.party {
				c.Age = c.Age + 1
			}
			continue
		}
		t[i+1]++
		t[i] -= clockLimits[i]
	}
}

// AdvanceClock = ovr21:03D5 (count, unit): add count units to the area
// clock (area+0x18C, 7 words), then run down timed effects.
func (e *Engine) AdvanceClock(count, unit int) {
	var t [7]int
	for i := range t {
		t[i] = int(e.area.Clock[i])
	}
	for i := 1; i <= count; i++ {
		t[unit] = int(uint16(t[unit] + 1))
		e.normalizeClock(&t)
	}
	for i := range t {
		e.area.Clock[i] = uint16(t[i])
	}
	e.tickEffects(count, unit)
}

// tickEffects = ovr21:0000 (count, unit): count down the timed entries in
// each member's effect list (+0x7F) in steps of up to 10 rounds; expired
// ones are removed. While camping only members flagged in DS:471D are
// looked at.
func (e *Engine) tickEffects(count, unit int) {
	r := &e.run
	if e.beginFlag != 2 {
		for i := 1; i < len(r.effTick); i++ { // FillChar(DS:471E, 0x48, 1)
			r.effTick[i] = 1
		}
	} else {
		any := false
		for i := 1; ; i++ { // repeat .. until: index 1 is always looked at
			if i < len(r.effTick) && r.effTick[i] != 0 {
				any = true
			}
			if any || i+1 > int(e.game.PartyCount) {
				break
			}
		}
		if !any {
			return
		}
	}
	w := count
	for u := unit; u > 1; u-- {
		w = int(uint16(w * clockLimits[u-1]))
	}
	next := func(c *Character, a *Affect) *Affect {
		for i, x := range c.Affects {
			if x == a && i+1 < len(c.Affects) {
				return c.Affects[i+1]
			}
		}
		return nil
	}
	head := func(c *Character) *Affect {
		if len(c.Affects) == 0 {
			return nil
		}
		return c.Affects[0]
	}
	for w > 0 {
		step := w
		if step > 10 {
			step = 10
		}
		for i, c := range e.party {
			if i+1 >= len(r.effTick) || r.effTick[i+1] == 0 {
				continue
			}
			r.effTick[i+1] = 0
			cur, pred := head(c), head(c) // bp-1B, bp-13
			var last *Affect              // bp-0F: the tail when the walk starts
			for a := cur; a != nil; a = next(c, a) {
				last = a
			}
			for done := false; cur != nil && !done; {
				done = cur == last
				d := int(cur.Time)
				switch {
				case d == 0:
					cur = next(c, cur)
				case step < d:
					cur.Time = uint16(d - step)
					r.effTick[i+1] = 1
					cur = next(c, cur)
				default:
					after := next(c, cur)
					e.RemoveAffect(c, cur.Type, cur) // ovr25:0026
					// ovr21:0203 quirk: when the node before the removed one
					// is the head, the walk restarts at the head.
					if pred == head(c) {
						cur = head(c)
					} else {
						cur = after
					}
				}
				// ovr21:0239: pred = the node before cur (or cur itself).
				pred = head(c)
				for pred != nil && pred != cur && next(c, pred) != cur {
					pred = next(c, pred)
				}
			}
			for ; cur != nil; cur = next(c, cur) { // ovr21:028E
				if cur.Time > 0 {
					r.effTick[i+1] = 1
				}
			}
		}
		if w > 10 {
			w -= 10
		} else {
			w = 0
		}
	}
}
