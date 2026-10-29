package engine

// ovr23 (spells), first part (0x0000..0x100F) and the tail (0x2540..0x37B5).
// The spell effects between are in spells2.go. The spell record is
// DS:31FA + 16*spell (see spellRec): [0] class (casterLevel), [1] list
// group, [2]/[3] duration base/per level, [4]/[5] amount base/per level,
// [6] minimum 1 flag, [7] targeting kind, [8] save mode, [9] save type,
// [0xA] affect type.

import (
	"bytes"
	"fmt"
)

// tgt / setTgt = DS:6C91 + 4*i: the spell target list (1-based). Slot 1 is
// DS:6C95, kept in e.spellTarget.
func (e *Engine) tgt(i int) *Character {
	if i == 1 {
		return e.spellTarget
	}
	return e.spellTargets[i]
}

func (e *Engine) setTgt(i int, c *Character) {
	if i == 1 {
		e.spellTarget = c
	} else {
		e.spellTargets[i] = c
	}
}

// spellName = DS:28E9 + 0x29*spell.
func (e *Engine) spellName(spell int) string { return spellNames[spell] }

// spellMenu = ovr23:008A (mode, var index): shows the list built by
// buildSpellList (mode 1 cast, 2 memorize, 3 scribe, 4 learn) and returns
// the chosen spell, 0 for none. For mode 3 DS:6B80 is the scroll chosen.
func (e *Engine) spellMenu(mode int, index *int) byte {
	const do Choice = iota // the mode's word
	var menu Bar
	switch mode {
	case 1:
		menu = Bar{{"Cast", do}}
	case 3:
		menu = Bar{{"Scribe", do}}
	case 4:
		menu = Bar{{"Learn", do}}
	}
	title := ""
	if mode >= 1 && mode <= 4 { // set {1,2,3,4} at cs:001B
		title = "Choose Spell: "
	}
	y2 := 0x16
	showExit := mode != 4
	redraw := false
	if *index < 0 {
		e.DrawBoxTitled(1, 3, winRight, y2, 0, colBlack, "")
		redraw = true
		*index = 0
	}
	if mode == 4 || mode == 1 {
		redraw = true
	}
	var sel *ListNode
	var key Choice
	for {
		key = e.ListMenu(title, menu, colLightMagenta, colLightGreen, colWhite, 1, 5, 38, y2, e.spellList, showExit, &redraw, index, &sel).ID
		// set {0, 13, 'C', 'E', 'L', 'M', 'S'} at cs:004A: none, Enter, the words
		if key != PickRelist {
			break
		}
	}
	var res byte
	if key != NoPick { // set {0, 'E'} at cs:006A
		res = e.listedSpell(sel)
	}
	if idx := e.listedIndex(sel); mode == 3 && idx < len(e.spellItems) {
		e.usedScroll = e.spellItems[idx]
	}
	e.spellList = nil // ovr26:11F3 frees the list
	return res
}

// listedIndex: the number of entries (not headers) of the list before sel.
func (e *Engine) listedIndex(sel *ListNode) int {
	idx := 0
	for _, n := range e.spellList {
		if n == sel {
			break
		}
		if n.Flag == 0 {
			idx++
		}
	}
	return idx
}

// listedSpell: the spell of the list's line sel, 0 for none.
func (e *Engine) listedSpell(sel *ListNode) byte {
	if idx := e.listedIndex(sel); sel != nil && idx < len(e.spellIDs) {
		return e.spellIDs[idx]
	}
	return 0
}

// addSpellLine = ovr23:028D (spell): appends the spell to the menu list,
// behind a class header when the group changes. Bit 7 marks the spell as
// "to be memorised" (" *"). The asm's id array holds 0x15 bytes and runs
// into the item array when the book is listed; the slice has no limit.
func (e *Engine) addSpellLine(spell byte) {
	group := byte(0) // bp-6 is 0 for the first line
	if n := len(e.spellIDs); n > 0 {
		group = e.spellRec(e.spellIDs[n-1], 1)
	}
	if g := e.spellRec(spell&0x7F, 1); g != group {
		e.spellList = append(e.spellList, &ListNode{Text: cut(spellGroups[g], textCols), Flag: 1})
	}
	prefix := "  "
	if spell > 0x7F {
		prefix = " *"
	}
	e.spellList = append(e.spellList, &ListNode{Text: cut(prefix+e.spellName(int(spell&0x7F)), textCols)})
	e.spellIDs = append(e.spellIDs, spell&0x7F)
}

// addScrollSpells = ovr23:044E (all): lists the spells on the scroll in
// DS:6B80 (all: only those still to be scribed, > 0x80; else any), unless
// the scroll is hidden or too hard for the reader; remembers the scroll.
func (e *Engine) addScrollSpells(all byte) {
	sel := e.selected()
	// 00BA:2199 (sel, 0x10, out): the character has the affect
	if e.FindAffect(sel, affReadMagic) != nil || sel.Levels[clsCleric] > 0 && sel.Levels[clsCleric] < 0x80 && e.base(e.usedScroll.Type).Slot == 0xC {
		e.usedScroll.Hidden = 0
	}
	if e.usedScroll.Hidden != 0 {
		return
	}
	for _, v := range e.usedScroll.Spells {
		if all != 0 && v > 0x80 || all == 0 && v > 0 {
			e.addSpellLine(v)
			if e.spellItemCount < len(e.spellItems) {
				e.spellItems[e.spellItemCount] = e.usedScroll
			}
			e.spellItemCount++
		}
	}
}

// addAllScrolls = ovr23:051C (all): the spells on every scroll carried;
// DS:6B80 walks the item list and is nil at the end.
func (e *Engine) addAllScrolls(all byte) {
	e.spellItems = [0x31]*Item{}
	e.spellItemCount = 0
	for _, it := range e.carried(e.selected()) {
		e.usedScroll = it
		if e.isScroll(it) {
			e.addScrollSpells(all)
		}
	}
	e.usedScroll = nil
}

// buildSpellList = ovr23:059C (kind): fills the menu list; kinds as in
// SpellList. True when there is at least one line.
func (e *Engine) buildSpellList(kind int) bool {
	any := false
	e.spellList = nil
	e.spellIDs = nil
	c := e.selected()
	switch kind {
	case 0:
		for i := 0; i <= 0x14; i++ {
			if v := c.Memorized[i]; v > 0 && v < 0x80 {
				e.addSpellLine(v)
			}
		}
	case 5:
		for i := 0; i <= 0x14; i++ {
			if v := c.Memorized[i]; v > 0x7F {
				e.addSpellLine(v)
			}
		}
	case 1:
		for i := 1; i <= 0x37; i++ {
			if c.SpellBook[i-1] != 0 {
				e.addSpellLine(byte(i))
				// extra: how often it is memorized on a rest (memorizeMenu)
				if n := bytes.Count(c.Plan[:], []byte{byte(i)}); n > 0 {
					l := e.spellList[len(e.spellList)-1]
					l.Text = cut(fmt.Sprintf("%-30s x%d", l.Text, n), 0x25)
				}
			}
		}
		for i, l := range e.spellList { // extra: the level headers carry the slots planned
			if l.Flag != 0 && i+1 < len(e.spellList) {
				s := e.spellIDs[e.listedIndex(e.spellList[i+1])]
				lvl, cls := e.spellLevel(s), e.spellClass(s)
				l.Text = cut(fmt.Sprintf("%s  %d/%d", l.Text, e.planned(c, lvl, cls), c.spellSlot(int(cls), int(lvl))), textCols)
			}
		}
	case 2:
		e.addScrollSpells(0)
	case 3:
		e.addAllScrolls(0)
	case 6:
		e.addAllScrolls(1)
	case 4:
		for i := 1; i <= lastSpell; i++ {
			if c.spellSlot(int(e.spellRec(byte(i), 0)), int(e.spellRec(byte(i), 1))) > 0 && c.SpellBook[i-1] == 0 {
				e.addSpellLine(byte(i))
			}
		}
	}
	if len(e.spellList) > 0 {
		any = true
	}
	return any
}

// spellDuration = ovr23:0729 (spell): the duration of the spell in rounds (or
// turns); never 0 when the record says so.
func (e *Engine) spellDuration(spell int) int {
	s := byte(spell)
	var v byte
	if !e.usingItem {
		v = e.spellRec(s, 2) + byte(int(e.spellRec(s, 3))*e.casterLevel(spell))
	} else {
		v = e.spellRec(s, 2) + byte(int(e.spellRec(s, 3))*6)
	}
	if v == 0 && e.spellRec(s, 6) != 0 {
		v = 1
	}
	if v == 0xFF {
		v = 1
	}
	return int(v)
}

// spellAmount = ovr23:07D6 (spell): the amount (damage or rounds) of the
// spell; a near procedure of the overlay.
func (e *Engine) spellAmount(spell int) int {
	var v int
	switch spell {
	case 0x28:
		v = e.Dice(1, 6) * 10
	case 0x39, 0x3D:
		v = e.Dice(5, 4)
	case 0x3B:
		v = e.Dice(1, 4)*10 + 0x28
	case 0x3F:
		if e.beginFlag == 5 {
			v = e.Dice(2, 10) * 10
		} else {
			v = (e.Dice(1, 10) + 10) * 10
		}
	case 0x43:
		v = 0x5A0
	default:
		v = int(e.spellRec(byte(spell), 4)) + int(e.spellRec(byte(spell), 5))*e.casterLevel(spell)
	}
	return int(int16(v))
}

// applySpell = ovr23:08C3 (spell, level, call, dmg, dmgType, msg): applies
// the spell record to every target: saving throw, damage and affect.
// level 0 means the caster level; dmg is also the flag that sets the damage
// type (DS:6819); dmg is cleared for the rest when an attack misses.
func (e *Engine) applySpell(spell, level, call, dmg, dmgType int, msg string) {
	msg = cut(msg, textCols)
	if dmg == 0 {
		e.rv.dmgType = 0
	} else {
		e.rv.dmgType = byte(dmgType)
	}
	if e.nSpellTargets == 0 {
		return
	}
	lvl := byte(level)
	if level == 0 {
		lvl = byte(e.casterLevel(spell))
	}
	s := byte(spell)
	for i := 1; i <= e.nSpellTargets; i++ {
		t := e.tgt(i)
		if t == nil {
			continue
		}
		saved := false
		if e.spellRec(s, 8) != 0 {
			saved = e.SavingThrow(t, int(e.spellRec(s, 9)), 0)
		}
		if e.spellRec(e.curSpell, 2) == 0xFF {
			e.ApplyItems(t)
			e.affectGroup(t, 0xB)
			if !e.AttackHits(e.selected(), t, int(t.AC[0])) {
				dmg = 0
				saved = true
			}
		}
		if dmg > 0 {
			e.Damage(t, dmg, e.spellRec(s, 8), saved)
		}
		if e.spellRec(s, 0xA) > 0 {
			e.AddAffectMsg(t, affectType(e.spellRec(s, 0xA)), e.spellAmount(spell), lvl, byte(call), e.spellRec(s, 8), saved, msg)
		}
	}
	e.rv.dmgType = 0
}

// spellTargetHook = lcall [DS:6B84].
func (e *Engine) spellTargetHook(spell, mode int) bool {
	if e.combatTargets {
		return e.chooseSpellTargets(spell, mode)
	}
	return e.spellTargetSetup(spell, mode)
}

// spellTargetSetup = ovr23:0A91 (spell, mode, var ok): fills the target
// list by the targeting kind of the spell (1 the caster, 2 a chosen member,
// 4 the whole party); ok is false for any other kind or when the choice is
// cancelled. This is slot 0 of the DS:6B84 table; mode is not used.
func (e *Engine) spellTargetSetup(spell, mode int) (ok bool) {
	if e.spellCaster == nil {
		e.spellCaster = e.selected()
	}
	e.spellTarget = e.selected()
	e.nSpellTargets = 1
	ok = true
	switch e.spellRec(byte(spell), 7) {
	case 1:
	case 2:
		e.DrawMainScreen()
		e.spellCaster = e.SelectCharacter("Cast Spell on whom", true, e.spellCaster)
		if e.spellCaster == nil {
			e.spellTarget = nil
			e.nSpellTargets = 0
			ok = false
		} else {
			e.spellTarget = e.spellCaster
		}
	case 4:
		for i, c := range e.party {
			e.setTgt(i+1, c)
		}
		if len(e.party) > 0 {
			e.nSpellTargets = len(e.party)
		}
	default:
		ok = false
	}
	return
}

// applySpellToSide = ovr23:0F4F (side, msg): drops the targets of the other side
// (and, for Bless in combat, those with an enemy next to them), then
// applies the spell record to the rest.
func (e *Engine) applySpellToSide(side int, msg string) {
	msg = cut(msg, textCols)
	e.spellOwnTargets = 1
	for i := 1; i <= e.nSpellTargets; i++ {
		t := e.tgt(i)
		if t == nil || int(t.Side) != side ||
			e.curSpell == spBless && e.beginFlag == 5 && e.nearEnemies(t, 1) > 0 {
			e.setTgt(i, nil)
		}
	}
	e.applySpell(int(e.curSpell), 0, 0, 0, 0, msg)
}

// CastSpell = ovr23:0C20 (spell, mode, b, var quit): casts the spell of
// the selected character. b non-zero announces it ("casts"); quit is set
// when a combat-only item was used up. A combat-only spell or item outside
// a fight asks to lose it / use it.
func (e *Engine) CastSpell(spell, mode, b byte, quit *bool) {
	caster := e.selected()
	ok := true
	if e.beginFlag != 5 && e.spellRec(spell, 7) == 0 {
		if !e.usingItem {
			e.DisplayString(1, 19, colLightGreen, e.spellName(int(spell)))
			e.DisplayString(1, 20, colLightGreen, "is a combat-only spell...")
			if e.YesNo(colLightMagenta, colWhite, colLightGreen, "Lose it? ") {
				e.forgetSpell(spell)
			}
		} else {
			e.DisplayString(1, 19, colLightGreen, "That Item")
			e.DisplayString(1, 20, colLightGreen, "is a combat-only item...")
			if e.YesNo(colLightMagenta, colWhite, colLightGreen, "Use it? ") {
				*quit = true
			}
		}
		b = 0
		ok = false
	}
	if b != 0 && !e.usingItem {
		e.announceCast(caster, "casts", int(spell))
	}
	for ok {
		*quit = e.spellTargetHook(int(spell), int(mode)) // the asm passes the caller's var
		if *quit {
			ok = false
			if e.beginFlag == 5 {
				e.hitColours(0x12)
				x, y := e.combatX(caster), e.combatY(caster)
				dir := 0
				for !e.inArc(x, y, e.tgtX, e.tgtY, dir) {
					dir++
				}
				e.combatTurn(caster, byte(dir), 1, false)
				e.soundWait(0)
				switch spell {
				case 0x2F:
					e.PlaySound(sndSpell47)
				case 0x33:
					e.PlaySound(sndSpell51)
				default:
					e.PlaySound(sndSpell)
				}
				e.flyMissile(x, y, e.tgtX, e.tgtY, 4, 0x1E)
				e.soundWait(0xA)
				if e.combatOnScreen(caster, false) {
					e.combatTurn(caster, caster.Combat.Facing, 1, true)
					e.combatTurn(caster, caster.Combat.Facing, 0, false)
				}
			}
			e.removeInvisibility(caster)
			if !e.usingItem {
				e.forgetSpell(spell)
			}
			e.curSpell = spell
			e.spellFx[spell]()
			e.curSpell = 0
			e.spellOwnTargets = 0
		} else if e.beginFlag != 5 {
			ok = false
		} else {
			if mode == 0 { // extra: the player cancelled the aim (the original asked "Abort Spell?" and lost
				e.ext.castCancelled = true // the spell and the turn): the spell is kept and the turn goes on
				break
			}
			e.BottomMessage("Spell Aborted")
			if !e.usingItem {
				e.forgetSpell(spell)
			}
			ok = false
		}
	}
	e.ClearMessageArea()
}

// spellRemoveCurse = ovr23:2540: the target (DS:6C95) loses the curse
// affect, or else the curse of its first cursed item.
func (e *Engine) spellRemoveCurse() {
	t := e.spellTarget
	if e.CureAffect(t, affBestowCurse) {
		e.hitMessage(t, true, "is un-cursed")
		return
	}
	found := false
	for _, it := range t.Items {
		if found {
			break
		}
		if it.Magic != 0 {
			found = true
			it.Magic = 0
		}
	}
	if found {
		e.hitMessage(t, true, "'s item is un-cursed")
	}
}

// affects is a spell whose whole effect is applySpell with its message:
// ovr23:10E9 "is affected" and its siblings (the ovr23 address is on
// each spellFx line).
func (e *Engine) affects(level, call int, msg string) func() {
	return func() { e.applySpell(int(e.curSpell), level, call, 0, 0, msg) }
}

// fxFireball = ovr23:266B: a burst of radius 2 round the target square,
// level d6 of fire (spFireballPower rolls its own level); the near list only
// becomes the target list when the area holds no word at +0x1CC.
func (e *Engine) fxFireball() {
	var lvl int
	if e.curSpell == spFireballPower {
		lvl = int(byte(e.Dice(1, 3)*2 + 1))
	} else {
		lvl = e.casterLevel(int(e.curSpell))
	}
	if e.area.In3D == 0 {
		e.buildNear(e.tgtX, e.tgtY, 2, 0xFF, 1)
		for i, n := range e.near {
			e.setTgt(i+1, e.combatant(n.index))
		}
		e.nSpellTargets = len(e.near)
	}
	e.combatCentre(e.tgtX, e.tgtY, 0, 8)
	e.applySpell(int(e.curSpell), 0, 0, e.RollDice(lvl, 6), 9, "")
}

// removeAffectFromSide = ovr23:2764 (type, side, msg): the first casterLevel
// targets of that side lose the affect; the others are dropped. A nil
// target (garbage in the asm) is dropped as well.
func (e *Engine) removeAffectFromSide(typ affectType, side int, msg string) {
	msg = cut(msg, textCols)
	e.spellOwnTargets = 1
	lvl := byte(e.casterLevel(int(e.curSpell)))
	for i := 1; i <= e.nSpellTargets; i++ {
		t := e.tgt(i)
		if t != nil && int(t.Side) == side && lvl > 0 {
			lvl--
			if e.CureAffect(t, typ) {
				e.setTgt(i, nil)
			}
		} else {
			e.setTgt(i, nil)
		}
	}
	e.applySpell(int(e.curSpell), 0, 0, 0, 0, msg)
	for i := 1; i <= e.nSpellTargets; i++ {
		if t := e.tgt(i); t != nil {
			e.affectGroup(t, 0x12)
		}
	}
}

// fxHaste = ovr23:2892: slow is removed from the caster's side.
func (e *Engine) fxHaste() { e.removeAffectFromSide(affSlow, int(e.selected().Side), "is Hasted") }

// strikeSquare = ovr23:28BC (x, y, dmg, saveType): whoever stands on the
// square takes dmg (half on a save, DS:6819 = 0xC); true when the ground
// there stops a bolt.
func (e *Engine) strikeSquare(x, y, dmg, saveType int) bool {
	blocked := false
	who, ground := e.mapAt(x, y)
	if ground > 0 && groundTable[ground][0] == 0xFF {
		blocked = true
	}
	if who > 0 {
		t := e.combatant(who)
		e.rv.dmgType = dmgElectric | dmgMagic
		e.Damage(t, int(byte(dmg)), 2, e.SavingThrow(t, saveType, 0))
		e.hitColours(0x13)
		e.rv.dmgType = 0
	}
	return blocked
}

// boltPath = ovr23:295B (reach, dmg, saveType, bounce): the bolt goes on
// past the target square, away from the caster, for reach*2 steps, hitting
// each square; where it is stopped it comes back toward the caster (once,
// when bounce is set, those squares count extra) with the direction
// reversed.
func (e *Engine) boltPath(reach, dmg, saveType, bounce int) {
	e.hitColours(0x13)
	if sel := e.selected(); e.combatX(sel) == e.tgtX && e.combatY(sel) == e.tgtY {
		return
	}
	e.spellOwnTargets = 1
	e.boltWalk(reach, bounce, nil, func(x0, y0, x, y int) bool {
		e.flyMissile(x0, y0, x, y, 4, 0x32)
		return e.strikeSquare(x, y, dmg, saveType)
	})
	e.spellOwnTargets = 0
}

// boltWalk is boltPath's line (split out, extra: the aim preview walks it
// dry): from the target square away from the selected caster; step (if
// any) sees each square crossed, strike each square a leg ends on (from
// the leg's start) and says whether the ground there stops the bolt.
// e.tgtX/tgtY move to where it turns, as in the original.
func (e *Engine) boltWalk(reach, bounce int, step func(x, y int), strike func(x0, y0, x, y int) bool) {
	prevWho, _ := e.mapAt(e.tgtX, e.tgtY)
	dirSign := 1
	sel := e.selected()
	cx, cy := e.combatX(sel), e.combatY(sel)
	flag := bounce
	if cx == e.tgtX && cy == e.tgtY {
		return
	}
	var who, ground byte // who is uninitialised in the asm until the first step
	_, ground = e.mapAt(e.tgtX, e.tgtY)
	r := byte(reach * 2)
	for r > 0 {
		x0, y0 := e.tgtX, e.tgtY
		x1 := int(int16(e.tgtX + (e.tgtX-cx)*dirSign*int(r)))
		y1 := int(int16(e.tgtY + (e.tgtY-cy)*dirSign*int(r)))
		l := newLine(x0, y0, x1, y1)
		for {
			lastX, lastY := byte(l.x), byte(l.y)
			if !(x0 == x1 && y0 == y1) {
				for {
					more := e.step(l)
					who, ground = e.mapAt(int(byte(l.x)), int(byte(l.y)))
					if more && step != nil {
						step(int(byte(l.x)), int(byte(l.y)))
					}
					if !more {
						break
					}
					if who > 0 && who != prevWho {
						break
					}
					if ground == 0 || groundTable[ground][0] > 1 {
						break
					}
					if byte(l.steps) >= r {
						break
					}
				}
			}
			prevWho = who
			if ground == 0 {
				r = 0
			}
			blocked := strike(int(lastX), int(lastY), int(byte(l.x)), int(byte(l.y)))
			if blocked {
				e.tgtX, e.tgtY = int(byte(l.x)), int(byte(l.y))
				back := newLine(e.tgtX, e.tgtY, cx, cy)
				for e.step(back) {
				}
				if flag != 0 && byte(back.steps) <= 8 {
					l.steps += 8
				}
				dirSign = -dirSign
				flag = 0
				prevWho = 0
			}
			if byte(l.steps) < r {
				r -= byte(l.steps)
			} else {
				r = 0
			}
			if blocked || r == 0 {
				break
			}
		}
	}
}

// fxLightning = ovr23:2BB7: level d6 down the line, bouncing.
func (e *Engine) fxLightning() {
	d := byte(e.Dice(e.casterLevel(int(e.curSpell)), 6))
	e.strikeSquare(e.tgtX, e.tgtY, int(d), 4)
	e.boltPath(8, int(d), 4, 1)
}

// fxSlow = ovr23:2C09.
func (e *Engine) fxSlow() {
	e.removeAffectFromSide(affHaste, b2i(isParty(e.selected())), "is Slowed")
}

// fxRestore = ovr23:2C43: gives back one drained level (the class that
// needs the fewest experience points) and the points lost with it.
func (e *Engine) fxRestore() {
	t := e.spellTarget
	if t.Drained == 0 {
		return
	}
	q := t.DrainedHP / t.Drained
	t.MaxHP += q
	t.HP += q
	t.HPRoll += q
	t.DrainedHP -= q
	t.Drained--
	best := int8(0xD)
	bestXP := int32(0x989680)
	var bestClass int // uninitialised in the asm when nothing qualifies
	for k := 0; k <= 7; k++ {
		lvl := int8(t.Levels[k])
		if lvl <= 0 || lvl > best {
			continue
		}
		xp := xpTable[k][byte(lvl+1)]
		if xp <= 0 || xp >= bestXP {
			continue
		}
		if atRaceLimit(t, k) {
			continue
		}
		best, bestClass, bestXP = lvl, k, xp
	}
	t.Levels[bestClass]++
	if int32(t.Exp) < bestXP {
		t.Exp = uint32(bestXP)
	}
	e.RecomputeCharacter(t)
	e.CharMessage(t, "is restored", true)
}

// fxSpeedy = ovr23:2E02.
func (e *Engine) fxSpeedy() {
	if !e.CureAffect(e.spellTarget, affSlow) {
		e.applySpell(int(e.curSpell), 0, 0, 0, 0, "is Speedy")
	}
}

// fxCureParalysis = ovr23:2E4D: affect 0x37 is cured (and the affect 0x16
// that goes with it removed); otherwise a small heal.
func (e *Engine) fxCureParalysis() {
	t := e.spellTarget
	if e.CureAffect(t, affPoisoned) {
		e.rv.cure = 1
		e.RemoveAffect(t, affSlowPoison, nil)
		e.rv.cure = 0
		return
	}
	if e.cureDiseaseAffects() {
		return
	}
	if e.Heal(t, e.Dice(1, 4)+8, false) {
		e.hitMessage(t, true, "is Healed")
	}
}

// fxStronger = ovr23:2EE5: strength 0x15 for the spell's duration.
func (e *Engine) fxStronger() {
	t := e.spellTarget
	out, ok := e.SetStrength(t, 0x15, 0)
	if ok {
		e.CharMessage(t, "is stronger", true)
	}
	e.AddAffect(t, affStrength, e.spellAmount(int(e.curSpell)), out, 1)
}

// fxBolt = ovr23:2F4D: Dice(1,6)+0x14 on the target square, then a fixed
// 0x14 along the line.
func (e *Engine) fxBolt() {
	e.strikeSquare(e.tgtX, e.tgtY, e.Dice(1, 6)+0x14, 4)
	e.boltPath(3, 0x14, 4, 0)
}

// fxHeal2d4 = ovr23:2FD0.
func (e *Engine) fxHeal2d4() {
	t := e.spellTarget
	if e.Heal(t, e.Dice(2, 4)+2, false) {
		e.hitMessage(t, true, "is Healed")
	}
}

// fxDamage2d4 = ovr23:3058: 2d4+2 of damage type 8.
func (e *Engine) fxDamage2d4() {
	e.applySpell(int(e.curSpell), 0, 0, e.RollDice(2, 4)+2, 8, "")
}

// fxNone = ovr23:3092: does nothing.
func (e *Engine) fxNone() {}

// breathWeapon = ovr23:30D9 (char, affect, flag): the breath weapon affect
// (type 0x58). Half the time (when DS:6DE3 is set) nothing happens; the
// monster breathes at the target square with its hit points as damage.
func (e *Engine) breathWeapon(c *Character, a *Affect, flag int) {
	if e.combatRound != 0 && e.Dice(1, 100) <= 0x32 {
		return
	}
	cx, cy := e.combatX(c), e.combatY(c)
	e.CharMessage(c, "Breathes!", true)
	e.spellTargetHook(spLightningBolt, 1) // aimed like a bolt; the result (DS:70C6) is not used
	e.tgtX = cx + sign(e.tgtX-cx)
	e.tgtY = cy + sign(e.tgtY-cy)
	if cx+1 == e.tgtX {
		e.tgtX++
	}
	if cy+1 == e.tgtY {
		e.tgtY++
	}
	e.removeInvisibility(c)
	e.hitColours(0x13)
	e.flyMissile(cx, cy, e.tgtX, e.tgtY, 4, 0x32)
	e.strikeSquare(e.tgtX, e.tgtY, int(c.MaxHP), 3)
	e.boltPath(0xA, int(c.MaxHP), 3, 0)
	if a.Data > 0xFD {
		a.Data--
	} else {
		e.RemoveAffect(c, affBreathWeapon, a)
	}
	e.clearCombatInfo(c)
}

// isScroll = ovr23:323D (item): the base item type is a scroll (11..13).
func (e *Engine) isScroll(it *Item) bool {
	if it == nil {
		return false
	}
	t := e.base(it.Type).Slot
	return t > 0xA && t < 0xE
}

// scrollUsed = ovr23:328A (char, item, spell): the spell is wiped from the
// scroll; the scroll goes when the item byte +0x30 falls under 0xD2.
func (e *Engine) scrollUsed(c *Character, it *Item, spell byte) {
	s := spell & 0x7F
	idx := 0
	for i := 1; i <= 3; i++ {
		if it.Spells[i-1]&0x7F == s {
			idx = i
		}
	}
	if idx == 0 {
		return
	}
	it.Spells[idx-1] = 0
	it.Words[1]--
	if it.Words[1] < 0xD2 {
		e.RemoveItem(c, it)
	}
}

// announceCast = ovr23:331A (char, msg, spell): announces the cast, in the
// side panel in combat, else in the message box.
func (e *Engine) announceCast(c *Character, msg string, spell int) {
	msg = cut(msg, textCols)
	if e.beginFlag == 5 {
		e.CharMessage(c, "Casts a Spell", true)
		e.combatSay(nil, "Spell: "+e.spellName(spell), true)
		return
	}
	e.say(c.Name() + " " + msg + " " + e.spellName(spell)) // extra: the original's flash in the message area is a toast, as CharMessage's
	e.MsgDelay()
}

// spellTableInit = ovr23:3413: the DS:6B84 table of spell effects (indexed
// by spell, slot 0 is spellTargetSetup), the breath affect handler (type
// 0x58, DS:6988) and the spell globals.
func (e *Engine) spellTableInit() {
	e.rv.cure = 0 // DS:681F
	e.usingItem = false
	e.spellCaster = nil
	e.itemsRedraw = true // DS:6821
	affectHandlers[0x58] = func(e *Engine, c *Character, a *Affect, flag byte) { e.breathWeapon(c, a, int(flag)) }
	e.spellFx[spBless] = e.spellBless
	e.spellFx[spCurse] = e.spellBane
	e.spellFx[spCureLightWounds] = e.spellCureLightWounds
	e.spellFx[spCauseLightWounds] = e.spellDamage1d8
	e.spellFx[spDetectMagicCl] = e.affects(0, 0, "is affected")   // ovr23:10E9
	e.spellFx[spProtFromEvilCl] = e.affects(0, 0, "is protected") // ovr23:1123
	e.spellFx[spProtFromGoodCl] = e.affects(0, 0, "is protected")
	e.spellFx[spResistCold] = e.affects(0, 0, "is cold-resistant") // ovr23:1162
	e.spellFx[spBurningHands] = e.spellDamageLevel
	e.spellFx[spCharmPerson] = e.spellCharmPerson
	e.spellFx[spDetectMagicMU] = e.affects(0, 0, "is affected")
	e.spellFx[spEnlarge] = e.spellStrengthPercent
	e.spellFx[spReduce] = e.spellReduce
	e.spellFx[spFriends] = e.spellFriends
	e.spellFx[spMagicMissile] = e.spellMagicMissile
	e.spellFx[spProtFromEvilMU] = e.affects(0, 0, "is protected")
	e.spellFx[spProtFromGoodMU] = e.affects(0, 0, "is protected")
	e.spellFx[spReadMagic] = e.affects(0, 0, "is affected")
	e.spellFx[spShield] = e.affects(0, 0, "is shielded") // ovr23:1499
	e.spellFx[spShockingGrasp] = e.spellBurningHands
	e.spellFx[spSleep] = e.spellSleep
	e.spellFx[spFindTraps] = e.affects(0, 0, "is affected")
	e.spellFx[spHoldPersonCl] = e.spellHold
	e.spellFx[spResistFire] = e.affects(0, 0, "is fire resistant") // ovr23:17E9
	e.spellFx[spSilence] = e.affects(0, 0, "is silenced")          // ovr23:1822
	e.spellFx[spSlowPoison] = e.spellRaiseDead
	e.spellFx[spSnakeCharm] = e.spellUndeadTargets
	e.spellFx[spSpiritualHammer] = e.spellApplyAnd17
	e.spellFx[spDetectInvisibility] = e.affects(0, 0, "is affected")
	e.spellFx[spInvisibility] = e.affects(0, 0, "is invisible") // ovr23:1A16
	e.spellFx[spKnock] = e.affects(0, 0, "Knock-Knock")         // ovr23:1A4F
	e.spellFx[spMirrorImage] = e.spellMirrorImage
	e.spellFx[spRayOfEnfeeblement] = e.affects(0, 0, "is weakened") // ovr23:1ACC
	e.spellFx[spStinkingCloud] = e.spellStinkingCloud
	e.spellFx[spStrength] = e.spellEnlarge
	e.spellFx[spAnimateDead] = e.spellAnimateDead
	e.spellFx[spCureBlindness] = e.spellCureBlind
	e.spellFx[spCauseBlindness] = e.affects(0, 0, "is blind") // ovr23:225D
	e.spellFx[spCureDisease] = e.spellCureDisease
	e.spellFx[spCauseDisease] = e.affects(0, 1, "is diseased") // ovr23:234C
	e.spellFx[spDispelMagicCl] = e.spellDispel
	e.spellFx[spPrayer] = e.spellPrayer
	e.spellFx[spRemoveCurse] = e.spellRemoveCurse
	e.spellFx[spBestowCurse] = e.affects(0, 0, "has been cursed!") // ovr23:2604
	e.spellFx[spBlink] = e.affects(0, 0, "is blinking")            // ovr23:263D
	e.spellFx[spDispelMagicMU] = e.spellDispel
	e.spellFx[spFireball] = e.fxFireball
	e.spellFx[spHaste] = e.fxHaste
	e.spellFx[spHoldPersonMU] = e.spellHold
	e.spellFx[spInvisibility10] = e.affects(0, 0, "is invisible")
	e.spellFx[spLightningBolt] = e.fxLightning
	e.spellFx[spProtFromEvil10] = e.affects(0, 0, "is protected")
	e.spellFx[spProtFromGood10] = e.affects(0, 0, "is protected")
	e.spellFx[spProtFromNormalMissiles] = e.affects(0, 0, "is protected")
	e.spellFx[spSlow] = e.fxSlow
	e.spellFx[spRestoration] = e.fxRestore
	e.spellFx[57] = e.fxSpeedy
	e.spellFx[58] = e.fxCureParalysis
	e.spellFx[59] = e.fxStronger
	e.spellFx[60] = e.fxBolt
	e.spellFx[61] = e.affects(0, 0, "is paralyzed") // ovr23:2F99
	e.spellFx[62] = e.fxHeal2d4
	e.spellFx[63] = e.affects(0, 0, "is invisible") // ovr23:302A
	e.spellFx[spFireballPower] = e.fxFireball
	e.spellFx[65] = e.fxDamage2d4
	e.spellFx[66] = e.fxNone
	e.spellFx[67] = e.affects(0xFF, 1, "is Reading") // ovr23:30A2
}
