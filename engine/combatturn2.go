package engine

import (
	"os"
	"strconv"
	"strings"
)

// GAME.OVR overlay 14, second half (0x1BF8..0x3C72): choosing targets for
// spells and missiles, the attack animation and a few combat helpers.

// aimTarget is the 7-byte record the targeting routines fill in: who was
// aimed at (nil for an empty square) and the square.
type aimTarget struct {
	c    *Character
	x, y byte
}

// spellData returns byte k of the 16-byte spell record at DS:3200.
func (e *Engine) spellData(spell, k int) byte { return spellTable[spell&0xFF][6+k] }

// friendToHeal = ovr14:1BF8 (c, var who): the friend to heal around c: the most
// hurt of the same side on the nine squares round c (c itself only when
// under half its hit points), or the first body lying there when no one
// with fewer than 8 hit points is.
func (e *Engine) friendToHeal(c *Character, who **Character) bool {
	var best *Character
	bestHP, body := byte(0xFF), 0
	for dir := 0; dir <= 8; dir++ {
		x := byte(e.combatX(c)) + byte(dirDX[dir])
		y := byte(e.combatY(c)) + byte(dirDY[dir])
		idx, ground := e.mapAt(int(x), int(y))
		if idx > 0 {
			*who = e.combatant(idx)
			t := *who
			if t != nil && t.Side == c.Side && t.HP < t.MaxHP {
				if t.HP < bestHP || (t == c && t.HP < t.MaxHP>>1) {
					best, bestHP = t, t.HP
				}
			}
		} else if ground == 0x1F {
			for b := 1; b <= e.nbodies; b++ {
				bd := e.bodies[b]
				if bd.c != nil && (bd.c.Status < statusDead || bd.c.Status > statusGone) && bd.x == x && bd.y == y {
					body = b
				}
			}
		}
	}
	if bestHP >= 8 && body != 0 {
		*who = e.bodies[body].c
	} else {
		*who = best
	}
	return *who != nil
}

// chooseSpellTarget = ovr14:1E16 (spell, silent, allowGround, var t): one
// target for the selected character's spell. With silent 0 the player
// aims; otherwise it is picked for him. The square goes to DS:6DB9/6DBA.
func (e *Engine) chooseSpellTarget(spell, silent, allowGround int, t *aimTarget) bool {
	sel := e.selected()
	found := false
	if silent == 0 {
		e.ext.enemiesOnly = spellTable[spell&0xFF][7] == 0 // extra: a hostile spell cycles through foes only
		e.playerAim(sel, e.spellDuration(spell), 0, allowGround, &found, t)
		e.ext.enemiesOnly = false
		sel.Combat.Target = t.c
	} else if e.spellData(spell, 8) == 0 {
		t.c = sel
		if spell != spCureLightWounds || e.friendToHeal(sel, &t.c) {
			t.x, t.y = byte(e.combatX(t.c)), byte(e.combatY(t.c))
			found = true
		}
	} else {
		for tries := 0x14; tries > 0 && !found; tries-- {
			ok := true
			if !e.pickTarget(sel, e.spellDuration(spell), 0, 1) {
				continue
			}
			tp := sel.Combat.Target
			if area := int(e.spellData(spell, 9)); area > 0 {
				e.buildNear(e.combatX(tp), e.combatY(tp), area, 0xFF, 1)
				for _, n := range e.near {
					if int(e.combatant(n.index).Side) != b2i(isParty(sel)) {
						ok = false
					}
				}
			}
			if e.isHelpless(tp) {
				for i := 1; i <= 4; i++ {
					if affectType(e.spellData(spell, 4)) == helplessAffects[i] {
						ok = false
					}
				}
			}
			if ok {
				t.c = sel.Combat.Target
				t.x, t.y = byte(e.combatX(t.c)), byte(e.combatY(t.c))
				found = true
			}
		}
	}
	if found {
		e.tgtX, e.tgtY = int(t.x), int(t.y)
	} else {
		*t = aimTarget{}
	}
	return found
}

// chooseSpellTargets = ovr14:20C3 (spell, silent, var ok): the targets of the spell
// into DS:6C91+4i (count DS:6C94). ok is false when none was chosen.
func (e *Engine) chooseSpellTargets(spell, silent int) (ok bool) {
	var tmp aimTarget
	ok = true
	e.nSpellTargets = 0
	e.spellOwnTargets = 0
	e.tgtX, e.tgtY = e.combatX(e.selected()), e.combatY(e.selected())
	sel := e.selected()
	fromNear := func(rng int) {
		e.buildNear(int(e.tgtX), int(e.tgtY), rng, 0xFF, 1)
		for i, n := range e.near {
			e.setTgt(i+1, e.combatant(n.index))
		}
		e.nSpellTargets = len(e.near)
		e.spellOwnTargets = 1
	}
	kind := e.spellData(spell, 0) & 0xF
	e.ext.aoe, e.ext.bolt = 1, bolts[spell] // extra: area preview (aoe.go); one target: its own square
	if kind > 8 && kind <= 0xE {
		e.ext.aoe = int(kind&7) + 1
	}
	defer func() { e.ext.aoe, e.ext.bolt = 0, [2]int{} }()
	switch {
	case kind == 0:
		e.setTgt(1, sel)
		e.nSpellTargets = 1
	case kind == 0xF:
		if !e.chooseSpellTarget(spell, silent, 0, &tmp) {
			ok = false
		} else if sel.Combat.Target != nil {
			e.setTgt(1, sel.Combat.Target)
			e.nSpellTargets = 1
		} else {
			fromNear(int(e.spellData(spell, 0)&0xF) >> 4) // always 0, as in the original
		}
	case kind >= 8 && kind <= 0xE:
		picked := e.chooseSpellTarget(spell, silent, 1, &tmp)
		if !picked {
			ok = false
		} else {
			fromNear(int(e.spellData(spell, 0) & 7))
		}
	default:
		left := int(kind&3) + 1
		n := 0
		for left > 0 {
			if !e.chooseSpellTarget(spell, silent, 0, &tmp) {
				left--
				continue
			}
			dup := false
			for k := 1; k <= n; k++ {
				if e.tgt(k) == tmp.c {
					dup = true
				}
			}
			if !dup {
				n++
				e.setTgt(n, tmp.c)
				left--
			} else if silent == 0 {
				e.BottomMessage("Already been targeted")
			} else {
				left--
			}
			e.eraseCursor(e.combatX(e.tgt(n)), e.combatY(e.tgt(n)))
		}
		e.nSpellTargets = n
		if n == 0 {
			ok = false
		}
	}
	return
}

// castSpell = ovr14:2414 (spell, a, var quit): the selected character picks
// (spell 0: from their memorised spells) and casts a spell. Spells with a
// casting time of 3 or more only begin and are cast in a later round.
func (e *Engine) castSpell(spell, a int, quit *bool) {
	sel := e.selected()
	have, idx := true, -1 // bp-6 = 1, bp-5 = 0xFFFF
	*quit = false
	if spell == 0 {
		spell = int(e.SpellList(0, 1, &idx, &have))
	}
	if spell > 0 && e.spellData(spell, 5) == 0 {
		e.BottomMessage("Camp Only Spell")
		spell = 0
	}
	if a == 0 {
		e.combatUseHook()
		e.combatRedrawChar(sel, 3, 1)
		e.combatPanel(sel)
	}
	if spell > 0 {
		delay := int(e.spellData(spell, 6)) / 3
		if delay == 0 {
			e.CastSpell(byte(spell), byte(a), 1, quit)
			*quit = e.clearCombatInfo(sel)
		} else {
			*quit = true
			e.CharMessage(sel, "Begins Casting", true)
			sel.Combat.Spell = byte(spell)
			if int(int8(sel.Combat.Delay)) > delay {
				sel.Combat.Delay -= byte(delay)
			} else {
				sel.Combat.Delay = 1
			}
		}
	}
}

// rangedFacing = ovr14:2574 (c, target): c may backstab the target: a thief
// with no armour or leather, no weapon or a club, dagger or broad, long or
// short sword, standing behind it (the direction from c to it is the one it faces).
func (e *Engine) rangedFacing(c, target *Character) bool {
	flag := false
	w, armor := c.Slots[slotWeapon], c.Slots[slotArmor] // +0xCC, +0xD4
	var wt itemType                                     // uninitialised in the original when there is no weapon
	if w != nil {
		wt = w.Type
	}
	if int8(c.Levels[clsThief]) > 0 && (armor == nil || armor.Type == itemLeatherArmor) {
		if w == nil || wt == itemClub || wt == itemDagger || (wt >= itemBroadSword && wt <= itemShortSword) {
			flag = true
		}
	}
	return flag && target.Combat.Attackers > 1 && byte(e.directionTo(c, target)) == target.Combat.Facing
}

// directionTo = ovr14:263D (from, to): the direction 0..7 (8 when none fits)
// from one combatant to the other.
func (e *Engine) directionTo(from, to *Character) int {
	x1, y1 := e.combatX(from), e.combatY(from)
	x2, y2 := e.combatX(to), e.combatY(to)
	dir := 0
	for !e.inArc(x1, y1, x2, y2, dir) {
		dir++
	}
	return dir
}

// attackAnimation = ovr14:26B0 (from, to, weapon): the weapon's sprite
// flies from one combatant to the other.
func (e *Engine) attackAnimation(from, to *Character, weapon *Item) {
	dir := e.directionTo(from, to)
	frames, delay, base := 1, 0xA, 0xD
	switch typ := weapon.Type; {
	case typ == itemDart || typ == itemJavelin || typ == itemQuarrel || typ == itemSpear || typ == itemArrow:
		switch {
		case dir&1 == 1 && (dir == 3 || dir == 5):
			e.iconToSprite(base+1, 1, 0, dir == 5)
		case dir&1 == 1:
			e.iconToSprite(base+1, 0, 0, dir == 7)
		default:
			e.iconToSprite(base+dir%4, dir>>2, 0, false)
		}
	case typ == itemHandAxe || typ == itemClub || typ == itemHammer:
		e.hitColours(base + 3)
		frames, delay = 4, 0x32
	case typ == itemHolyWater || typ == itemOil:
		e.hitColours(base + 4)
		frames, delay = 4, 0x32
	default:
		if typ == itemSling {
			base++
		}
		e.iconToSprite(base+7, 0, 0, false)
		e.iconToSprite(base+7, 1, 1, false)
		frames, delay = 2, 0x14
	}
	e.soundWait(0)
	e.PlaySound(sndMissile)
	e.flyMissile(e.combatX(from), e.combatY(from), e.combatX(to), e.combatY(to), frames, delay)
	e.PlaySound(sndMissileEnd)
	e.soundWait(0xA)
}

// foesHealthPercent = ovr14:2879: DS:6E2F = the monsters' remaining hit points in
// five percent steps (20 * hp / maximum, times 5; dead ones count in the
// maximum).
func (e *Engine) foesHealthPercent() {
	var hp, max uint16
	for _, c := range e.party {
		if c.Side == sideMonsters {
			if c.Okay != 0 {
				hp += uint16(c.HP)
			}
			max += uint16(c.MaxHP)
		}
	}
	if max != 0 {
		e.foesHealth = byte(20 * hp / max * 5) // the 16-bit product, dx cleared before the divide
	}
}

// fastestFoeMove = ovr14:2908 (c): the highest moveRate / 2 over the living
// combatants on the other side.
func (e *Engine) fastestFoeMove(c *Character) int {
	best := byte(0)
	for _, t := range e.party {
		if b2i(isParty(c)) == int(t.Side) && t.Okay != 0 {
			if v := byte(e.moveRate(t)) >> 1; v > best {
				best = v
			}
		}
	}
	return int(best)
}

// mayAttack = ovr14:2998 (c, target): may c attack the target? Attacking
// a friend asks first; yes turns every sane character of the party hostile.
func (e *Engine) mayAttack(c, target *Character) bool {
	if b2i(isParty(target)) == int(c.Side) || c.Auto != 0 {
		return true
	}
	if e.YesNo(colLightMagenta, colLightGreen, colWhite, "Attack Ally: ") != 'Y' {
		return false
	}
	e.attackedAlly = 1
	e.game.AttackedAlly = 1
	for _, m := range e.party {
		if m.Status == statusOkay && m.Control > 0x7F {
			m.Side = sideMonsters
			if m.Combat != nil {
				m.Combat.Target = nil
			}
		}
	}
	e.countSides()
	return true
}

// aimPrompt = ovr14:2AA0 (c, cand, rng, showRange): shows how far the
// candidate is and asks Next, Prev, Manual, Target, Center or Exit.
func (e *Engine) aimPrompt(c, cand *Character, rng, showRange int) byte {
	menu := ""
	if showRange != 0 {
		e.DisplayString(0, messageRow, colLightGreen, "Range = "+strconv.Itoa(e.rangeTo(c, cand))+"  ")
	}
	why := e.targetWhy(c, cand, rng, showRange)
	can := why == ""
	e.snap.aimWhy = why // extra: views draw the preview red
	defer func() { e.snap.aimWhy = "" }()
	if can {
		menu = "Attack " // extra: Target renamed
	}
	menu = cut("Next Prev Manual "+menu+"Center Exit", textCols)
	e.combatRedrawChar(cand, 3, 1)
	e.combatPanel(cand)
	_, undo := e.aoeMarks(e.combatX(cand), e.combatY(cand), "") // the menu leaves no room for the count
	mark := e.regionMark()
	// extra (nav.go): click a foe to aim at it, the aimed one again to attack; pointing at
	// one aims there too. An area spell takes any square: the free cursor goes there.
	pick := func(sx, sy int, click bool) uint16 {
		who := e.occ[sy][sx]
		switch {
		case who > 0 && e.combatant(who) == cand:
			if click && !can { // extra: say why instead of doing nothing
				e.ext.aimRefused = why
				return 'C'
			}
			if click {
				return 'A'
			}
			return 0
		case e.ext.aimGround:
			e.ext.jump, e.ext.aimTake = &[2]int8{int8(sx), int8(sy)}, click
			return 'M'
		case who == 0 || !click && len(e.ext.aimMarks) > 0 && !e.aimStop(sx, sy):
			return 0 // pointing at one Next would pass over: stay
		}
		e.ext.jump, e.ext.aimTake = &[2]int8{int8(sx), int8(sy)}, click // a click casts / attacks once the aim is there
		return 'N'
	}
	e.addRegion(region{x0: pageCol, y0: 1, x1: pageCol + viewW*3 - 1, y1: 21, click: func(cx, cy int) uint16 {
		if sx, sy, in := e.combatSquare(cx, cy); in {
			return pick(sx, sy, true)
		}
		return 0
	}, square: func(x, y int) uint16 { return pick(x, y, false) }})
	var key byte
	switch t := e.ext.autoAim; { // extra: the main menu's clicked foe
	case t == nil && e.ext.aimTake: // extra: clicked here: take it at once
		e.ext.aimTake = false
		if key = 'A'; !can {
			key, _ = e.combatInput("Aim:", menu, false)
		}
	case t == nil:
		key, _ = e.combatInput("Aim:", menu, false)
	case t == cand:
		e.ext.autoAim = nil
		if can {
			key = 'A'
		} else {
			key, _ = e.combatInput("Aim:", menu, false)
		}
	case !e.ext.autoJumped:
		e.ext.autoJumped = true
		e.ext.jump = &[2]int8{int8(e.combatX(t)), int8(e.combatY(t))}
		key = 'N'
	default: // not among the candidates: aim as usual
		e.ext.autoAim = nil
		key, _ = e.combatInput("Aim:", menu, false)
	}
	e.regionCut(mark)
	undo()
	if r := e.ext.aimRefused; r != "" && key == 'C' {
		e.ext.aimRefused = ""
		e.aimRefused(r)
	}
	if key == 'A' {
		key = 'T'
	}
	return key
}

// aimStop (extra): is a foe Next stops on standing at (x, y)?
func (e *Engine) aimStop(x, y int) bool {
	_, ok := e.ext.aimMarks[[2]int{x, y}]
	return ok
}

// combatInput = ovr26:03BD with the picture not animated (prompt, menu,
// allowExtra): the key, and the DS:6E31 flag is dropped.
func (e *Engine) combatInput(prompt, menu string, allowExtra bool) (byte, bool) {
	return e.DisplayInput(prompt, menu, colLightMagenta, colLightGreen, colWhite, allowExtra, false)
}

// canTarget (aimPrompt's test, ovr14:2AA0): is Target offered for cand?
// In range, and with a missile weapon: ammunition, and no foe adjacent
// unless the weapon is thrown.
func (e *Engine) canTarget(c, cand *Character, rng, showRange int) bool {
	return e.targetWhy(c, cand, rng, showRange) == ""
}

// targetWhy (extra) is canTarget's test with the reason it fails ("" when
// it does not): in range, and with a missile weapon: ammunition, and no foe
// adjacent unless the weapon is thrown.
func (e *Engine) targetWhy(c, cand *Character, rng, showRange int) string {
	if e.rangeTo(c, cand) > rng {
		return outOfRange(rng)
	}
	var tmp *Item
	switch {
	case showRange == 0:
		return ""
	case cand == c:
		return "Not a target"
	case !e.rangedWeapon(c):
		return ""
	case !e.ammoFor(c, &tmp):
		return "No ammunition"
	case e.nearEnemies(c, 1) > 0 && !e.throwable(c):
		return "A foe is too close to shoot"
	}
	return ""
}

// outOfRange (extra): the refusal for a target farther than rng squares.
func outOfRange(rng int) string {
	if rng == 1 {
		return "Out of range: 1 square at most"
	}
	return "Out of range: " + strconv.Itoa(rng) + " squares at most"
}

// aimRefused (extra): why the aimed square cannot be taken, in red on the
// status row for a moment.
func (e *Engine) aimRefused(why string) {
	e.ClearCells(0, messageRow, lastCol, messageRow)
	e.DisplayString(0, messageRow, colLightRed, cut(why, textCols))
	e.MsgDelay()
	e.MsgDelay()
	e.ClearCells(0, messageRow, lastCol, messageRow)
}

// aimConfirm = ovr14:2C3D (c, cand, mode, var found, var t): take cand as
// the target. With mode 1 (a missile attack) the attack is made at once.
func (e *Engine) aimConfirm(c, cand *Character, mode int, found *bool, t *aimTarget) {
	var res *Item
	*found = true
	if mode == 1 && !e.mayAttack(c, cand) {
		*found = false
	}
	if !*found {
		*t = aimTarget{}
		return
	}
	t.c = cand
	t.x, t.y = byte(e.combatX(cand)), byte(e.combatY(cand))
	e.cmap.cursor = 0
	e.combatCentre(e.cmap.viewX+viewW/2, e.cmap.viewY+3, 3, 8)
	if mode == 1 {
		if e.sweep(c, cand) {
			*found = e.clearCombatInfo(c)
		} else {
			e.countAttackTurn(c, cand)
			if e.rangedWeapon(c) && e.ammoFor(c, &res) && e.throwable(c) && e.rangeTo(c, cand) == 1 {
				res = nil
			}
			*found = e.fullAttack(c, cand, 0, res)
		}
	}
}

// aimManual = ovr14:2DFA (c, cand, rng, showRange, allowGround, var found,
// var t): move a cursor over the map with the arrow keys; Enter or T takes
// the square (and who stands on it), Esc or E gives up.
func (e *Engine) aimManual(c, cand *Character, rng, showRange, allowGround int, found *bool, t *aimTarget) {
	*t = aimTarget{}
	x, y := int8(e.combatX(cand)), int8(e.combatY(cand))
	if j := e.ext.jump; j != nil { // extra: the square aimPrompt was pointed at
		e.ext.jump = nil
		x, y = j[0], j[1]
	}
	key, dir := byte(0x20), 8
	*found = false
	e.cmap.cursor, e.cmap.cursorSize = 1, 1
	defer e.rawArrows("arrows, point=move the cursor|Enter, click=target|C=center|Esc=back")()
	defer func() { e.snap.aimWhy = "" }()
	for !(key == 0 || key == 13 || key == 'E' || key == 'T') {
		e.combatCentre(int(x), int(y), 3, dir)
		x += int8(byte(dirDX[dir]))
		y += int8(byte(dirDY[dir]))
		x, y = min(max(x, 0), 0x31), min(max(y, 0), 0x18)
		who, ground := e.mapAt(int(x), int(y))
		e.ClearKeyboard()
		ok, why := false, "Out of sight" // extra: why, for the refusal
		fail := func(s string) {         // the first reason found is the one told
			if ok {
				why = s
			}
			ok = false
		}
		dist, tx, ty := 0xFF, int(x), int(y)
		if e.lineOfSight(&dist, &tx, &ty, e.combatX(c), e.combatY(c)) {
			ok = true
			if showRange != 0 {
				e.DisplayString(0, messageRow, colLightGreen, "Range = "+strconv.Itoa(dist>>1)+"  ")
			}
		} else if showRange != 0 {
			e.ClearCells(0, messageRow, lastCol, messageRow)
		}
		dist >>= 1
		cand = nil
		if ok {
			if who > 0 {
				cand = e.combatant(who)
			} else if ground == 0x1F {
				for b := 1; b <= e.nbodies; b++ {
					if int8(e.bodies[b].x) == x && int8(e.bodies[b].y) == y {
						cand = e.bodies[b].c
					}
				}
			}
		}
		if cand != nil {
			e.combatPanel(cand)
		} else {
			e.ClearCells(0, messageRow, lastCol, messageRow)
		}
		if rng < dist {
			fail(outOfRange(rng))
		}
		if e.groundInfo(ground)[0] == 0xFF {
			fail("A wall is in the way")
		}
		if cand != nil {
			if !e.mayActAgainst(c, cand) {
				fail("Cannot act against " + cand.Name())
			}
			if showRange == 1 {
				if cand == c || (who == 0 && ground == 0x1F) {
					fail("Not a target")
				} else if e.rangedWeapon(c) {
					var tmp *Item
					if !e.ammoFor(c, &tmp) {
						fail("No ammunition")
					} else if e.nearEnemies(c, 1) > 0 && !e.throwable(c) {
						fail("A foe is too close to shoot")
					}
				}
			}
		} else if allowGround == 0 {
			fail("No one there to target")
		}
		menu := "Center Exit"
		if ok {
			menu = cut("Attack "+menu, textCols) // extra: Target renamed
		}
		mark := e.regionMark()
		{ // extra (nav.go): point at a square to put the cursor there, click to target it
			pick := func(sx, sy int, click bool) uint16 {
				if sx == int(x) && sy == int(y) {
					if click && ok {
						return 'T'
					}
					if click { // extra: say why instead of doing nothing
						e.ext.aimRefused = why
						return 'C'
					}
					return 0
				}
				e.ext.jump, e.ext.aimTake = &[2]int8{int8(sx), int8(sy)}, click
				return keyNone
			}
			e.addRegion(region{x0: pageCol, y0: 1, x1: pageCol + viewW*3 - 1, y1: 21, click: func(cx, cy int) uint16 {
				if sx, sy, in := e.combatSquare(cx, cy); in {
					return pick(sx, sy, true)
				}
				return 0
			}, square: func(x, y int) uint16 { return pick(x, y, false) }})
		}
		e.markReach(e.ext.aimMarks, true) // the cursor's last square drew over its frame
		e.snap.aimWhy = ""                // extra: views draw the preview red where it cannot be cast
		if !ok {
			e.snap.aimWhy = why
		}
		prompt, undo := e.aoeMarks(int(x), int(y), "(Use Cursor keys) ")
		if take := e.ext.aimTake; take && ok { // a click away from the cursor: moved there, now the target
			e.ext.aimTake = false
			key = 'T'
		} else {
			if take {
				e.aimRefused(why) // extra: clicked where it cannot go
			}
			e.ext.aimTake = false
			key, _ = e.combatInput(prompt, menu, true)
		}
		undo()
		if key == 'A' {
			key = 'T'
		}
		e.regionCut(mark)
		if key == 13 || key == 'T' || key == 'C' && e.ext.aimRefused != "" {
			if e.ext.aimRefused = ""; !ok { // extra: tell why and stay, instead of leaving the cursor
				e.aimRefused(why)
				key = ' '
			}
		}
		if j := e.ext.jump; j != nil { // the clicked square; the step below is dir 8 (none)
			e.ext.jump = nil
			e.eraseCursor(int(x), int(y))
			x, y = j[0], j[1]
		}
		switch key {
		case 13, 'T':
			e.cmap.cursor = 0
			if ok {
				t.x, t.y = byte(x), byte(y)
				t.c = cand
				if showRange == 1 {
					e.aimConfirm(c, t.c, showRange, found, t)
				} else {
					*found = true
				}
			}
			if !ok || !*found {
				e.eraseCursor(int(x), int(y))
				*found = false
				*t = aimTarget{}
			}
		case 0x48:
			dir = 0
		case 0x49:
			dir = 1
		case 0x4D:
			dir = 2
		case 0x51:
			dir = 3
		case 0x50:
			dir = 4
		case 0x4F:
			dir = 5
		case 0x4B:
			dir = 6
		case 0x47:
			dir = 7
		case 0, 0x45:
			e.eraseCursor(int(x), int(y))
			*t = aimTarget{}
			*found = false
		case 'C':
			e.combatCentre(int(x), int(y), 0, 8)
			dir = 8
		default:
			dir = 8
		}
	}
}

// playerAim = ovr14:3558 (c, rng, showRange, allowGround, var found, var t):
// the player aims for c. Candidates are walked through nearest first
// (N next, P previous, M manual cursor, T take, C centre, Esc/E give up).
// rng 0xFF takes the range of the readied weapon.
func (e *Engine) playerAim(c *Character, rangeArg, showRange, allowGround int, found *bool, t *aimTarget) {
	e.iconToSprite(0x19, 0, 0, false)
	*t = aimTarget{}
	*found = false
	rng := byte(rangeArg)
	if rng == 0xFF {
		if w := c.Slots[slotWeapon]; w != nil {
			rng = e.base(w.Type).Range - 1
		} else {
			rng = 1
		}
	}
	if rng == 0 || rng == 0xFF {
		rng = 1
	}
	e.ext.aimGround = allowGround != 0
	defer func() { e.ext.aimGround = false }()
	// nested ovr14:32E6 and ovr14:3389 work on the caller's locals
	var list []nearEntry
	var cur, count, prevX, prevY byte
	skipAlly := false   // extra: N/P pass over allies (hostile spells), while a foe can be aimed at
	hitOnly := false    // extra: with a weapon, N/P pass over foes it cannot hit, while one can be
	rebuild := func() { // ovr14:32E6
		e.markReach(e.ext.aimMarks, false)
		e.buildNear(e.combatX(c), e.combatY(c), 0x7F, 0xFF, e.combatSize(c))
		list = append([]nearEntry(nil), e.near...)
		count = byte(len(list))
		skipAlly, hitOnly = false, false
		for _, n := range list {
			if f := e.combatant(n.index); e.ext.enemiesOnly && f != nil && f.Side != c.Side && e.mayActAgainst(c, f) {
				skipAlly = true
				hitOnly = hitOnly || showRange != 0 && e.canTarget(c, f, int(rng), showRange)
			}
		}
		e.ext.aimMarks = map[[2]int][]int{}
		if showRange != 0 { // extra: a weapon's: the foes it can hit, with the chance, for views that draw targets
			e.snap.targets, e.snap.hits = nil, map[int]int{}
		}
		for _, n := range list {
			if f := e.combatant(n.index); skipAlly && f != nil && f.Side != c.Side && e.mayActAgainst(c, f) &&
				(!hitOnly || e.canTarget(c, f, int(rng), showRange)) {
				e.ext.aimMarks[[2]int{e.combatX(f), e.combatY(f)}] = nil
				if hitOnly {
					e.snap.targets = append(e.snap.targets, int(n.index))
					e.snap.hits[int(n.index)] = e.hitChance(c, f)
				}
			}
		}
	}
	if showRange != 0 {
		targets, hits := e.snap.targets, e.snap.hits // the main menu's, back after the aim
		defer func() { e.snap.targets, e.snap.hits = targets, hits }()
	}
	defer func() { e.markReach(e.ext.aimMarks, false); e.ext.aimMarks = nil }()
	step := func(by byte, show bool) *Character { // ovr14:3389
		ent := func(i byte) nearEntry {
			if int(i) < 1 || int(i) > len(list) {
				return nearEntry{}
			}
			return list[i-1]
		}
		if show {
			prevX, prevY = e.cpos[ent(cur).index].x, e.cpos[ent(cur).index].y
		} else {
			e.eraseCursor(int(prevX), int(prevY))
		}
		cur += by
		if cur == 0 {
			cur = count
		}
		if cur > count {
			cur = 1
		}
		idx := ent(cur).index
		res := e.combatant(idx)
		nx, ny := e.cpos[idx].x, e.cpos[idx].y
		if show {
			e.flyMissile(int(prevX), int(prevY), int(nx), int(ny), 1, 0)
			prevX, prevY = nx, ny
		}
		return res
	}
	rebuild()
	if count == 0 { // extra: nobody to aim at (N/P would spin)
		return
	}
	cur = 1
	var by, showBy byte // showBy is uninitialised in the original until a key sets it
	cand := step(by, true)
	by = 1
	key := byte(' ')
	for !*found && !(key == 0 || key == 'E') {
		if !e.mayActAgainst(c, cand) || skipAlly && cand.Side == c.Side ||
			hitOnly && !e.canTarget(c, cand, int(rng), showRange) {
			cand = step(by, false)
			continue
		}
		e.markReach(e.ext.aimMarks, true)
		key = e.aimPrompt(c, cand, int(rng), showRange)
		switch key {
		case 'N':
			by, showBy = 1, 1
			if j := e.ext.jump; j != nil { // extra: the clicked foe
				e.ext.jump = nil
				showBy = 0
				for i, n := range list {
					if n.index == e.occ[j[1]][j[0]] {
						e.eraseCursor(e.combatX(cand), e.combatY(cand))
						cur = byte(i + 1) // step(0) below lands on it
					}
				}
			}
		case 'P':
			by, showBy = 0xFF, 0xFF
		case 'M':
			e.aimManual(c, cand, int(rng), showRange, allowGround, found, t)
			e.iconToSprite(0x19, 0, 0, false)
			rebuild()
			showBy = 0
		case 'T':
			e.aimConfirm(c, cand, showRange, found, t)
			e.iconToSprite(0x19, 0, 0, false)
			rebuild()
			showBy = 0
		case 'C':
			e.combatCentre(e.combatX(cand), e.combatY(cand), 0, 8)
			showBy = 0
		}
		e.eraseCursor(e.combatX(cand), e.combatY(cand))
		cand = step(showBy, !*found && !(key == 0 || key == 'E'))
	}
	if showRange != 0 {
		e.ClearCells(0, messageRow, lastCol, messageRow)
	}
}

// pickTarget = ovr14:37E4 (c, rng, a, b): a target for c's attack from the
// enemies within rng, kept in c's combat block. Up to two passes of 20
// random picks; the second sees through walls (not with b set).
func (e *Engine) pickTarget(c *Character, rng, a, b int) bool {
	first, found, second := false, false, false
	tgt := c.Combat.Target
	if b != 0 || tgt == nil || tgt.Side == c.Side || tgt.Okay == 0 || !e.mayActAgainst(c, tgt) {
		c.Combat.Target = nil
	}
	if c.Combat.Target != nil {
		found = true
	}
	for !found && !second {
		second = first
		if first && b == 0 {
			e.cmap.seeAll = 1
		}
		tries := 0x14
		n := e.nearEnemies(c, rng)
		for tries > 0 && !found && n > 0 {
			tries--
			r := e.Dice(1, n)
			if e.targets[r] > 0 {
				cand := e.combatant(e.targets[r])
				if (a != 0 && e.cmap.seeAll != 0) || e.mayActAgainst(c, cand) {
					found = true
					c.Combat.Target = cand
				} else {
					e.targets[r] = 0
					for i := 1; i <= n; i++ {
						if e.targets[i] > 0 {
							found = true // sic, the original sets the result flag
						}
					}
					if found {
						found = false
					} else {
						n = 0
					}
				}
			}
		}
		if !first {
			first = true
		}
	}
	e.cmap.seeAll = 0
	return found
}

// rake = ovr14:39D2 (c, affect, flag): the affect handler of an
// attacker's rake: while DS:6823 is 2 and its victim is up, c rakes it.
func (e *Engine) rake(c *Character, a *Affect, flag byte) {
	if e.slotHits[1] == 2 && c.Combat.Target != nil && c.Combat.Target.Okay != 0 {
		c.Combat.AttackSlot = 2
		c.Attacks[1], c.Dice[1], c.Sides[1], c.Attacks[0] = 2, 2, 4, 0
		e.CharMessage(c, "Rakes...", true)
		e.fullAttack(c, c.Combat.Target, 0, nil)
		e.clearCombatInfo(c)
	}
}

// strikeTied = ovr14:3A86 (c, affect, flag): the affect handler tying c to
// the combatant a.Data (DS:6826): c strikes at it until one of them is
// down or the affect is removed.
func (e *Engine) strikeTied(c *Character, a *Affect, flag byte) {
	e.tiedTarget = e.combatant(a.Data)
	if flag != 0 || c.Okay == 0 || e.tiedTarget.Okay == 0 {
		e.RemoveAffect(e.tiedTarget, affHeldFast, nil)
		if flag == 0 {
			a.Call = 0
			e.RemoveAffect(c, affStrikeTied, a)
		}
		return
	}
	c.Attacks[0], c.Attacks[1], c.Dice[0], c.Sides[0] = 1, 0, 3, 4
	e.fullAttack(c, e.tiedTarget, 1, nil)
	e.clearCombatInfo(c)
	if e.tiedTarget.Okay == 0 {
		e.RemoveAffect(c, affStrikeTied, a)
	}
}

// stingCheat = ovr14:3B93: the command line word STING kills every monster
// ("The Gods intervene!"). True when it was given.
func (e *Engine) stingCheat() bool {
	// DOS upper-cased the command line, hence EqualFold
	if len(os.Args) < 2 || !strings.EqualFold(os.Args[1], "STING") {
		return false
	}
	e.BottomMessage("The Gods intervene!")
	for _, c := range e.party {
		if c.Side == sideMonsters {
			c.Okay = 0
			c.Status = statusDead
			if i := e.combatIndex(c); i == 0 {
				e.ncomb = 0 // DS:5F2A is slot 0's size byte
			} else {
				e.cpos[i].size = 0
			}
		}
		e.clearCombatInfo(c)
	}
	e.combatCentre(e.cmap.viewX+viewW/2, e.cmap.viewY+3, 0xFF, 8)
	return true
}
