package engine

// The computer-controlled turn, ported from ovr10: monster AI, and party
// members under AI control or fleeing. Called from the combat loop (ovr09).

import "strconv"

// computerTurn = ovr10:000F (char): one computer turn. Picks the facing of
// the attack, then spells, then moves and attacks until the turn is used.
func (e *Engine) computerTurn(c *Character) {
	done := e.aiKeys(c)
	e.ClearRow24()
	e.ClearMessageArea()
	if c.Okay == 0 {
		done = e.clearCombatInfo(c)
	}
	ci := c.Combat
	d := ci.Pattern
	if d == 0 || d > 4 || e.Dice(1, 4) == 1 {
		d = byte(e.Dice(1, 8))
		if d == 8 {
			d = byte(e.Dice(1, 2) + 4)
		} else {
			d = byte(e.Dice(1, 4)) // the first roll is thrown away
		}
	}
	ci.Pattern = d
	if !done {
		done = e.aiFleeCheck(c)
	}
	if ci.Fleeing != 0 && ci.Turned == 0 {
		e.CharMessage(c, "flees in panic", true)
	}
	if done {
		return
	}
	if e.aiCastFromItem(c) {
		e.clearCombatInfo(c)
		return
	}
	if ci.Spell > 0 {
		e.CastSpell(ci.Spell, 1, 1, &done)
		done = e.clearCombatInfo(c)
		return
	}
	if e.aiTryCast(c) {
		e.clearCombatInfo(c)
		return
	}
	if e.aiCastKnown(c) {
		return
	}
	e.aiPickWeapon(c)
	done = e.aiKeys(c)
	for !done {
		if e.pickTarget(c, 0xFF, 1, 0) && int8(ci.Delay) > 0 {
			done = e.aiAttackLoop(c)
		} else {
			done = e.aiGiveUp(c)
		}
	}
}

// aiTryCast = ovr10:0203 (char): a spell caster with a spell picked
// (weakestUnturned) casts it.
func (e *Engine) aiTryCast(c *Character) bool {
	r := false // uninitialised in the original when the test fails: 0
	if c.Combat.TurnUsed == 0 && int8(c.Levels[clsCleric]) > 0 {
		var t *Character
		if e.weakestUnturned(c, &t) {
			r = true
			e.clericTurnUndead(c)
		}
	}
	return r
}

// canCast = ovr10:02EB (char, spell, level): may the computer cast the
// spell at this level: the level fits, and an area spell (DS:3209 set)
// must not catch anyone of the wrong side.
func (e *Engine) canCast(c *Character, spell, level int) bool {
	tbl := spellTable[spell][13:16]
	if int(tbl[0]) < level {
		return false
	}
	if spell != spCureLightWounds && tbl[1] == 0 {
		return true
	}
	if spell == spCureLightWounds {
		var t *Character
		if e.friendToHeal(c, &t) {
			return true
		}
	}
	n := e.nearEnemies(c, e.spellDuration(spell))
	if n == 0 {
		return false
	}
	if tbl[2] == 0 {
		return true
	}
	// ovr10:0255 (nested): is anyone near (x,y) not on the selected side
	wrongSide := func(x, y int) bool {
		r := false
		e.buildNear(x, y, int(spellTable[spell][15]), 0xFF, 1)
		sel := b2i(isParty(e.selected()))
		for i := 1; i <= len(e.near); i++ {
			if int(e.combatant(e.near[i-1].index).Side) != sel {
				r = true
			}
		}
		return r
	}
	for i := 1; i <= n; i++ {
		p := e.cpos[e.targets[i]]
		if wrongSide(int(p.x), int(p.y)) {
			return false
		}
	}
	return true
}

// aiCastFromItem = ovr10:03E4 (char): try to use an item that casts a
// spell the computer can cast; true when one was used.
func (e *Engine) aiCastFromItem(c *Character) bool {
	r := false
	var found *Item
	lvl := 7
	rounds := e.Dice(1, 7)
	if c.Combat.CanUse == 0 || e.rv.side[b2i(isParty(c))] == 0 || e.area.NoMagic != 0 {
		return false
	}
	for k := 1; k <= rounds; k++ {
		for _, p := range c.Items {
			if found != nil {
				break
			}
			sp := p.Spells[1]
			if !e.isScroll(p) && p.Spells[2] < 0x80 && p.Readied != 0 && sp > 0 {
				if sp > 0x38 {
					sp -= 0x17
				}
				if e.canCast(c, int(sp), lvl) {
					found = p
				}
			}
		}
		lvl--
	}
	if found != nil {
		e.useItem(found)
		r = true
	}
	return r
}

// aiCastKnown = ovr10:0545 (char): pick a known spell (c+0x17..) the
// computer may cast, trying lower levels each round; true when the spell
// call (castSpell) returned a non-empty string.
func (e *Engine) aiCastKnown(c *Character) bool {
	var arr [21]byte // the original has 20 bytes: the 21st write lands on a spare local
	n := 0
	if c.Combat.CanCast != 0 {
		for i := 0; i <= 0x14; i++ {
			if c.Memorized[i] > 0 {
				arr[n] = c.Memorized[i]
				n++
			}
		}
	}
	found := 0
	lvl := 7
	rounds := e.Dice(1, 7)
	if n > 0 && (c.Control > 0x7F || e.magicOn != 0) && e.rv.side[b2i(isParty(c))] != 0 {
		for it := 1; it <= rounds && found == 0; it++ {
			for t := 1; t < 4 && found == 0; t++ {
				sp := int(arr[e.Dice(1, n)-1])
				if e.canCast(c, sp, lvl) {
					found = sp
				}
			}
			lvl--
		}
	}
	if found > 0 {
		var ok bool
		e.castSpell(found, 1, &ok)
		return ok
	}
	return false
}

// aiStep: step i (1..5) of a step pattern. i 6 means no step was found;
// the original reads past the table there (DS:02D2) and the value is unused.
func aiStep(pattern byte, i int) byte {
	if i < 1 || i > 5 {
		return 0
	}
	return aiStepPatterns[pattern][i-1]
}

// aiCanStep = ovr10:0675 (char, i, dir, var blocked): can the combatant
// step in direction DS:02AE[facing*5+i]+dir; blocked is set when the
// square is off the map.
func (e *Engine) aiCanStep(c *Character, i, dir int) (ok, blocked bool) {
	ci := c.Combat
	r := false
	t := int(aiStep(ci.Pattern, i))
	d := (t + dir) % 8
	who, ground := e.groundAt(c, d)
	if ground == 0 {
		return false, true
	}
	if who != 0 {
		return false, false
	}
	gc := groundTable[ground][0]
	if gc >= ci.Moves {
		return false, false
	}
	cost := 2
	if d&1 != 0 {
		cost = 3
	}
	cost = int(byte(cost * int(gc)))
	if ground == 0x1E {
		if e.FindAffect(c, affAnimated) == nil && e.FindAffect(c, affCoughing) == nil &&
			e.FindAffect(c, affBlockPoisonHold) == nil && e.FindAffect(c, affBlockMind) == nil {
			cost = int(byte(ci.Moves + 1))
		}
	}
	if int(ci.Moves) >= cost {
		r = true
	}
	return r, false
}

// aiMoveStep = ovr10:07F2 (char): show the movement left and take one
// step towards the target (or a wandering direction when fleeing).
func (e *Engine) aiMoveStep(c *Character) {
	ci := c.Combat
	e.DisplayString(0, promptRow, colLightGreen, "Move/Attack, Move Left = "+strconv.Itoa(int(ci.Moves>>1))+" ")
	if e.aiKeys(c) {
		return
	}
	if ci.Moves>>1 <= 0 || int8(ci.Delay) <= 0 || (ci.Fleeing == 0 && c.Class == clsMage) {
		e.aiGiveUp(c)
		return
	}
	var dir int
	if ci.Fleeing == 0 {
		dir = e.directionTo(c, ci.Target)
	} else {
		ci.Pattern = byte(e.Dice(1, 2))
		f := int(e.facing)
		dir = f - ((f+2)%4)>>1
		if c.Side == sideParty {
			dir += 4
		}
		dir %= 8
		dir = int(byte(dir))
	}
	var ok, blocked bool
	done := false
	i := 1
	for i < 6 && !done {
		if ok, blocked = e.aiCanStep(c, i, dir); ok {
			break
		}
		if ci.Fleeing != 0 && blocked {
			done = e.tryFlee(c)
		} else {
			i++
		}
	}
	var d int
	if done {
		ci.Moves = 0
		ci.Fleeing = 0
		done = e.clearCombatInfo(c)
	} else {
		d = (int(aiStep(ci.Pattern, i)) + dir) % 8
		if i == 6 || int(e.aiLastDir) == (d+4)%8 {
			e.aiBlocked++
			ci.Pattern = byte(int(ci.Pattern)%6 + 1)
			if e.aiBlocked > 1 {
				ci.Target = nil
				if e.aiBlocked > 2 {
					ci.Moves = 0
					done = true
				} else if !e.pickTarget(c, 0xFF, 1, 0) {
					done = e.aiGiveUp(c)
				}
			}
		}
		if i < 6 {
			e.aiLastDir = byte(d)
		} else {
			done = true
		}
	}
	if done {
		return
	}
	e.combatTurn(c, byte(d), 0, false)
	e.freeAttacksOnLeave(c, int(ci.Facing))
	if c.Okay == 0 {
		e.clearCombatInfo(c)
		return
	}
	e.stepSquare(c, int(ci.Facing))
	if c.Okay == 0 {
		e.clearCombatInfo(c)
	}
	e.stinkingCloud(c)
}

// aiAttackLoop = ovr10:0B3C (char): move, pick a target and attack until
// the turn is over; true when it is.
func (e *Engine) aiAttackLoop(c *Character) bool {
	ci := c.Combat
	e.aiLastDir, e.aiBlocked, e.aiState3 = 8, 0, 0
	iter := 0
	cont, stop := true, false
	e.affectGroup(c, 0xE)
	if ci.Delay == 0 {
		cont = false
	}
	for !stop && cont {
		for ci.Fleeing != 0 && ci.Moves > 0 && int8(ci.Delay) > 0 && int8(ci.Delay) < 0x14 {
			e.aiMoveStep(c)
		}
		if ci.Delay == 0 || ci.Delay == 0x14 {
			cont = false
		} else {
			stop = false
		}
		if stop || !cont {
			continue
		}
		iter++
		if iter > 0x14 {
			stop = true
			cont = !e.aiGiveUp(c)
		}
		engaged := false
		rng := 1
		if w := c.Slots[slotWeapon]; w != nil {
			rng = int(e.base(w.Type).Range - 1)
		}
		if rng == 0 || rng == 0xFF {
			rng = 1
		}
		tgt := ci.Target
		if tgt != nil && (tgt.Okay == 0 || tgt.Side == sideParty) {
			tgt = nil
		}
		if tgt != nil && e.mayActAgainst(c, tgt) {
			tx, ty := e.combatX(tgt), e.combatY(tgt)
			r := rng
			if e.lineOfSight(&r, &tx, &ty, e.combatX(c), e.combatY(c)) && r>>1 <= rng {
				engaged = true
			}
		}
		if !engaged {
			n := e.nearEnemies(c, rng)
			if n == 0 {
				if e.pickTarget(c, 0xFF, 0, 0) {
					e.aiMoveStep(c)
				} else {
					stop = e.aiGiveUp(c)
				}
			} else {
				r := e.Dice(1, n)
				tgt = e.combatant(e.targets[r])
				if e.rangedWeapon(c) && !e.throwable(c) && e.nearEnemies(c, 1) > 0 {
					e.aiPickWeapon(c)
					stop = true
					continue
				}
				if e.rangeTo(c, tgt) == 1 || e.mayActAgainst(c, tgt) {
					engaged = true
				}
			}
		}
		if engaged {
			e.combatCentre(e.combatX(c), e.combatY(c), 2, e.directionTo(c, tgt))
		}
		if !engaged {
			continue
		}
		if e.sweep(c, tgt) {
			stop = e.clearCombatInfo(c)
			continue
		}
		e.countAttackTurn(c, tgt)
		var weapon *Item
		if e.rangedWeapon(c) {
			e.ammoFor(c, &weapon)
			if e.throwable(c) && e.rangeTo(c, tgt) == 1 {
				weapon = nil
			}
		}
		stop = e.fullAttack(c, tgt, 0, weapon)
		if stop {
			cont = false
		} else if tgt.Okay == 0 {
			stop = true
		}
	}
	return !cont
}

// aiGiveUp = ovr10:0F4F (char): the turn ends; a combatant that cannot go
// on is cleared, otherwise 00BA:2686 finishes it.
func (e *Engine) aiGiveUp(c *Character) bool {
	e.ClearMessageArea()
	if e.isHelpless(c) || e.rangedWeapon(c) || c.Combat.Delay == 0 {
		return e.clearCombatInfo(c)
	}
	return e.guard(c)
}

// aiKeys = ovr10:0FC1 (char): keys during the computer turn: '2' toggles
// magic, space ends the turn of everyone not yet moved, '-' (stingCheat).
func (e *Engine) aiKeys(c *Character) bool {
	r := false
	if e.keyWaiting() {
		k := e.ReadKeyByte()
		if k == 0 {
			k = e.ReadKeyByte()
		}
		switch k {
		case 0x32:
			e.magicOn ^= 1
			if e.magicOn != 0 {
				e.BottomMessage("Magic On")
			} else {
				e.BottomMessage("Magic Off")
			}
		case 0x20:
			for _, p := range e.party {
				if p.Control < 0x80 && p.Status != statusAnimated {
					p.Auto = 0
				}
			}
			if c.Auto == 0 {
				c.Combat.Delay = 0x14
				r = true
			}
		case 0x2D:
			e.stingCheat()
		}
	}
	e.ClearKeyboard()
	return r
}

// aiFleeCheck = ovr10:10D7 (char): a forced flee, or a morale check that
// makes the combatant flee or surrender.
func (e *Engine) aiFleeCheck(c *Character) bool {
	ci := c.Combat
	r := false
	ci.Fleeing = 0
	e.removeTiedAffects(c)
	if ci.Turned != 0 {
		ci.Fleeing = 1
		e.CharMessage(c, "is forced to flee", true)
		return r
	}
	if c.Control <= 0x7F {
		return r
	}
	e.fleeRoll = byte(int(c.Control&0x7F) * 2)
	if e.fleeRoll > 0x66 {
		e.fleeRoll = 0
	}
	e.affectGroup(c, 0x11)
	t := 100 - int(c.HP)*100/int(c.MaxHP) // cdq clobbers dx: signed word divide
	if !(int(e.fleeRoll) < t || e.fleeRoll == 0) {
		return r
	}
	e.fleeRoll = e.foesHealth
	e.affectGroup(c, 0x11)
	t2 := uint16(100 - e.game.Morale)
	if !(uint16(e.fleeRoll) < t2 || e.fleeRoll == 0 || c.Side == sideParty) {
		return r
	}
	a := e.fastestFoeMove(c)
	if a <= e.moveRate(c)>>1 {
		ci.Fleeing = 1
		e.RemoveAffect(c, affStrikeTied, nil)
		e.RemoveAffect(c, affBloodDrain, nil)
		return r
	}
	if c.Stats[statInt] > 5 {
		e.KillCharacter(c, statusUnconscious, "Surrenders")
		r = e.clearCombatInfo(c)
	}
	return r
}

// weaponScore = ovr10:128D (char, item): how good the item is as the
// character's weapon (0 = unusable).
func (e *Engine) weaponScore(c *Character, it *Item) int {
	t := e.base(it.Type)
	s := byte(int(t.Dice) * int(t.Sides))
	if int8(it.Plus) > 0 {
		s += byte(int(int8(it.Plus)) << 3)
	}
	if int8(t.Bonus) > 0 {
		s += byte(int(int8(t.Bonus)) << 1)
	}
	if it.Type == itemHolyWater && c.Combat.Target != nil && int8(c.Combat.Target.Undead) > 0 {
		s = 8
	}
	if t.Usage&8 != 0 {
		s += byte((int(t.Speed) - 1) << 1)
	}
	if t.Hands <= 1 {
		s += 3
	}
	if int(c.Hands)+int(t.Hands) > 3 {
		s = 0
	}
	if it.Spells[2] == byte(affCursedAlignment) && it.Spells[1]&0xF != c.Alignment {
		s = 0
	}
	if it.Spells[1] == 0x53 {
		s = 0
	}
	if it.Magic != 0 {
		s = 0
	}
	return int(s)
}

// aiPickWeapon = ovr10:13CA (char): choose the best melee or missile
// weapon, and the best second item, and ready them.
func (e *Engine) aiPickWeapon(c *Character) {
	b := func(it *Item) *itemBase { return e.base(it.Type) }
	if w := c.Slots[slotWeapon]; w != nil {
		c.Hands -= b(w).Hands
	}
	if w := c.Slots[slotShield]; w != nil {
		c.Hands -= b(w).Hands
	}
	var best1, best2, best3 *Item
	bs1, bs3 := byte(1), byte(0)
	bs2 := byte(int(c.BaseDice[0]) * int(c.BaseSides[0]))
	if int8(c.BaseBonus[0]) > 0 {
		bs2 += byte(int(int8(c.BaseBonus[0])) << 1)
	}
	for _, p := range c.Items {
		if b(p).Slot == 0 && b(p).Classes&c.ClassMask != 0 {
			sc := byte(e.weaponScore(c, p))
			f := b(p).Usage
			if (f&8 != 0 || f&0x10 != 0) && sc > bs1 {
				best1, bs1 = p, sc
			}
			if f&8 == 0 && sc > bs2 {
				best2, bs2 = p, sc
			}
		}
		if b(p).Slot == 1 && b(p).Classes&c.ClassMask > 0 {
			sc := byte(0)
			if int8(p.Plus) >= 0 {
				sc = byte(int(int8(p.Plus)) + 1)
			}
			if sc > bs3 {
				best3, bs3 = p, sc
			}
		}
	}
	ranged := best1 != nil && b(best1).Range > 1 && b(best1).Usage&0x14 == 0x14
	var pend *Item
	var f byte
	if best1 != nil {
		f = b(best1).Usage
		if f&0x10 != 0 {
			pend = best1
		}
		if f&8 != 0 {
			if f&1 != 0 {
				pend = c.Slots[slotArrows]
			}
			if f&0x80 != 0 {
				pend = c.Slots[slotBolts]
			}
		}
	}
	ok := pend != nil || f == 0x0A
	chosen := best2
	if best1 != nil && int(bs1) > int(bs2)>>1 && ok && (ranged || e.nearEnemies(c, 1) == 0) {
		chosen = best1
	}
	done := false
	ok = true
	if w := c.Slots[slotWeapon]; w != nil && (w == chosen || w.Magic != 0) {
		ok = false
	}
	if ok {
		if c.Slots[slotWeapon] != nil {
			e.readyItem(c.Slots[slotWeapon])
		}
		e.ApplyItems(c)
		if w := c.Slots[slotShield]; w != nil && w.Magic == 0 {
			c.Hands -= b(w).Hands
		}
		if chosen != nil {
			e.readyItem(chosen)
		}
		done = true
	}
	e.ApplyItems(c)
	e.setSpeed(c)
	ok = true
	if w := c.Slots[slotShield]; w != nil && (w == best3 || w.Magic != 0) {
		ok = false
	}
	if c.Hands > 2 {
		// the original passes chosen even when nil
		if w := c.Slots[slotShield]; w == nil || w.Magic != 0 {
			if chosen != nil {
				e.readyItem(chosen)
			}
		} else {
			e.readyItem(w)
		}
		done = true
	} else if c.Hands < 2 && ok {
		if c.Slots[slotShield] != nil {
			e.readyItem(c.Slots[slotShield])
		}
		e.ApplyItems(c)
		if best3 != nil {
			e.readyItem(best3)
		}
		done = true
	}
	e.ApplyItems(c)
	if done {
		e.combatPanel(c)
	}
}
