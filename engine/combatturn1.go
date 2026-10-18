package engine

// Combat turn, first half (ovr14:0000..1BF8): setting up a combatant's turn,
// movement, attacks of opportunity, fleeing, sweeps, turning undead and the
// attack itself. The second half (from 1BF8) is ported separately.

// ds2 reads the word at DS:ofs of the exe image.

import "strconv"

// turnSetup = ovr14:0000 (char): fill the combat block for a new round.
func (e *Engine) turnSetup(c *Character) {
	cb := c.Combat
	cb.Spell, cb.CanCast, cb.CanUse, cb.Attacked, cb.AttackSlot = 0, 1, 1, 0, 2
	e.setSpeed(c)
	e.turnValue = c.BaseAttack[1]
	e.modMoves = 0
	e.affectGroup(c, 0x12)
	c.Attacks[1] = byte(e.halfRoundUp(e.turnValue))
	cb.Sweeps = c.FightLevel
	if c.Okay != 0 {
		cb.Delay = byte(int(dexMissile(c.Stats[statDex])) + e.Dice(1, 6))
		if int8(cb.Delay) < 1 {
			cb.Delay = 1
		}
		if byte((int(int8(c.Side))+1)&int(e.game.Surprise)) != 0 { // only the low byte is tested
			cb.Delay -= 6
		}
		if int8(cb.Delay) < 0 || int8(cb.Delay) > 0x14 {
			cb.Delay = 0
		}
	} else {
		cb.Delay = 0
	}
	cb.Moves = byte(e.moveRate(c))
}

// moveRate = ovr14:0123 (char): movement rate of the turn (half of it is kept in +0x113
// by setSpeed); the affects of group 0x12 may change it.
func (e *Engine) moveRate(c *Character) int {
	v := c.Move
	if c.Side == sideParty {
		v += byte(e.game.PartyMove)
	}
	e.modMoves = 1
	if v < 1 || v > 0x60 {
		v = 1
	}
	e.turnValue = v << 1
	e.affectGroup(c, 0x12)
	e.modMoves = 0
	return int(e.turnValue)
}

// damageRoll = ovr14:0191 (att, vic, slot): damage roll of the attack slot (1 or 2),
// tripled-or-so for a backstab, then the
// affects of groups 4 (attacker) and 5 (victim) may change DS:6818.
func (e *Engine) damageRoll(a, b *Character, slot int) {
	d := int(int8(a.Bonus[slot-1])) + e.RollDice(int(a.Dice[slot-1]), int(a.Sides[slot-1]))
	e.rv.damage = byte(d)
	// (the original compares the byte with 0 "above or equal": never clamps)
	if e.rangedFacing(a, b) {
		e.rv.damage = byte(int(e.rv.damage) * (int(uint16(int16(int8(a.Levels[clsThief]))))>>2 + 2))
	}
	e.rv.dmgType = 0
	if r := int8(a.Levels[clsRanger]); r > 0 && b.Kind == 2 {
		e.rv.damage += byte(r) // CotAB ranger_vs_giant: + ranger level against giants (kind 2)
	}
	e.affectGroup(a, 4)
	e.affectGroup(b, 5)
}

// attackText = ovr14:02F9 (att, vic, kind, damage, shown, hit): the attack text. kind 1 is
// a plain hit, 2 a backstab, 3 a kill of a helpless target; damage is
// applied, shown printed; hit 0 is a miss.
func (e *Engine) attackText(a, b *Character, kind, damage, shown, hit int) {
	var s string
	switch kind {
	case 2:
		s = "-Backstabs-"
	case 3:
		s = "slays helpless"
	default:
		s = "Attacks"
	}
	e.CharMessage(a, s, false)
	e.struck(a, b, hit != 0, damage&0xFF)
	s = ""
	if kind == 1 {
		s = "(from behind) "
	}
	if hit == 0 {
		s += "and Misses"
	} else {
		if kind == 3 {
			s = "with one cruel blow"
		} else {
			s += "Hitting for " + strconv.Itoa(shown&0xFF)
			if shown&0xFF == 1 {
				s += " point "
			} else {
				s += " points "
			}
			s += "of damage"
		}
		e.ApplyDamage(b, damage&0xFF)
	}
	e.combatSay(b, s, false)
	if damage&0xFF > 0 {
		b.Combat.CanCast = 0
		if b.Combat.Spell > 0 {
			e.MsgDelay()
			e.CharMessage(b, "lost a spell", true)
			e.forgetSpell(b.Combat.Spell)
			b.Combat.Spell = 0
		} else {
			e.MsgDelay()
		}
	} else {
		e.MsgDelay()
	}
	if b.Okay == 0 {
		e.CharMessage(b, "goes down", false)
		if b.Status == statusDying {
			e.combatSay(nil, "and is Dying", false)
		}
		if st := b.Status; st >= statusDead && st <= statusGone { // set at CS:02CF
			e.CharMessage(b, "is killed", false)
		}
		e.removeCombatAffects(b)
		e.affectGroup(b, 0xD)
		if b.Okay == 0 {
			e.combatRemoveFromMap(b)
		} else {
			e.MsgDelay()
		}
	}
	e.ClearMessageArea()
}

// waitingFoesStrike = ovr14:0627 (char): the enemies next to a mover that were waiting (+7) lose
// their reaction, turn to it and attack it.
func (e *Engine) waitingFoesStrike(c *Character) {
	n := e.nearEnemies(c, 1)
	for i := 1; i <= n; i++ {
		if c.Okay == 0 {
			continue
		}
		t := e.combatant(e.targets[i])
		if t.Combat.Guarding == 0 || e.isHelpless(t) {
			continue
		}
		e.combatCentre(e.combatX(c), e.combatY(c), 2, 8)
		t.Combat.Guarding = 0
		e.countAttackTurn(t, c)
		e.fullAttack(t, c, 0, nil)
	}
}

// stepSquare = ovr14:0712 (dir, char): step one square (2 movement points, 3 diagonally).
func (e *Engine) stepSquare(c *Character, dir int) {
	cb := c.Combat
	cost := byte(2)
	if dir&1 != 0 {
		cost = 3
	}
	if cost > cb.Moves {
		cb.Moves = 0
		return
	}
	cb.Moves -= cost
	idx := e.combatIndex(c)
	ox, oy := e.cpos[idx].x, e.cpos[idx].y
	nx := ox + byte(dirDX[dir])
	ny := oy + byte(dirDY[dir])
	margin := 1
	if c.Auto != 0 {
		margin = 3
		if !combatInView(int(int8(nx))-e.cmap.viewX, int(int8(ny))-e.cmap.viewY) {
			e.combatCentre(int(ox), int(oy), 2, 8)
		}
	}
	e.combatClearIcon(0, 0, idx)
	e.cpos[idx].x, e.cpos[idx].y = nx, ny
	e.combatRedrawMap()
	e.combatCentre(int(nx), int(ny), margin, 8)
	cb.Attackers, cb.Flank = 0, 0
	e.PlaySound(sndStep)
	e.waitingFoesStrike(c)
	if c.Okay == 0 || e.isHelpless(c) {
		cb.Moves = 0
	}
}

// freeAttacksOnLeave = ovr14:08B2 (dir, char): the mover steps (virtually) one square in dir; the
// enemies that were next to it and no longer are get a free attack.
func (e *Engine) freeAttacksOnLeave(c *Character, dir int) {
	var lst [0x0D]byte // bp-0xD+i, i = 1..: the DS:6DE4 list before the move
	idx := e.combatIndex(c)
	n := e.nearEnemies(c, 1)
	if n == 0 {
		return
	}
	for i := 1; i <= n; i++ {
		lst[i] = e.targets[i]
	}
	dx, dy := byte(dirDX[dir]), byte(dirDY[dir])
	e.cpos[idx].x += dx
	e.cpos[idx].y += dy
	m := e.nearEnemies(c, 1)
	e.cpos[idx].x -= dx
	e.cpos[idx].y -= dy
	for i := 1; i <= n; i++ {
		found := false
		for j := 1; j <= m; j++ {
			if e.targets[j] == lst[i] {
				found = true
			}
		}
		if found {
			lst[i] = 0
		}
	}
	for i := 1; i <= n; i++ {
		if lst[i] == 0 || c.Okay == 0 {
			continue
		}
		t := e.combatant(lst[i])
		if e.isHelpless(t) || !e.mayActAgainst(t, c) {
			continue
		}
		if e.FindAffect(t, affBloodDrain) != nil || e.FindAffect(t, affStrikeTied) != nil {
			continue
		}
		attacked := false
		tcb := t.Combat
		for k := int(tcb.Facing) + 6; k <= int(tcb.Facing)+10; k++ {
			if attacked {
				continue
			}
			if int8(tcb.Delay) <= 0 && tcb.Attackers != 0 {
				if !e.inArc(e.combatX(t), e.combatY(t), e.combatX(c), e.combatY(c), k%8) {
					continue
				}
			}
			w := 1
			if t.BaseAttack[0] == 0 {
				w = 2
			}
			for s := 1; s <= 2; s++ {
				if t.Attacks[s-1] > 0 {
					w = s
				}
			}
			if t.Attacks[w-1] == 0 {
				t.Attacks[w-1] = 1
			}
			tcb.AttackSlot = byte(w)
			saved := t.Combat.Target
			e.fullAttack(t, c, 1, nil)
			attacked = true
			t.Combat.Target = saved
			if c.Okay != 0 {
				e.combatPanel(c)
			}
		}
	}
}

// tryFlee = ovr14:0C6A (char) bool: try to run away; "Got Away" removes the character.
func (e *Engine) tryFlee(c *Character) bool {
	away := false
	if e.nearEnemies(c, 0xFF) == 0 {
		away = true
	} else {
		a := byte(e.moveRate(c)) >> 1
		b := byte(e.fastestFoeMove(c))
		if b < a {
			away = true
		} else if b == a && e.Dice(1, 2) == 1 {
			away = true
		}
	}
	if away {
		e.KillCharacter(c, statusRunning, "Got Away")
	} else {
		e.ext.refusal = true
		e.BottomMessage("Escape is blocked")
	}
	return e.clearCombatInfo(c)
}

// setSpeed = ovr14:0D23 (char): the character's speed in +0x113 from its movement base
// (+0xA1) or the readied weapon's speed (DS:5582 record +5, at least 2).
func (e *Engine) setSpeed(c *Character) {
	flag := false
	old := c.Attacks[0]
	c.Attacks[0] = c.BaseAttack[0]
	var it *Item
	if e.rangedWeapon(c) && e.ammoFor(c, &it) {
		flag = true
		v := byte(0) // (the original reads the item at +0xCC unchecked)
		if c.Slots[slotWeapon] != nil {
			v = e.base(c.Slots[slotWeapon].Type).Speed
		}
		if v < 2 {
			v = 2
		}
		e.turnValue = v
	} else {
		e.turnValue = c.Attacks[0]
	}
	e.modMoves = 0
	e.affectGroup(c, 0x12)
	r := byte(e.halfRoundUp(e.turnValue))
	if flag && it != nil {
		w := byte(1)
		if it.Count > w {
			w = it.Count
		}
		if w < r {
			r = w
		}
	}
	cb := c.Combat
	if cb.Attacked != 0 && r >= old && (int(r) >= int(old)*2 || flag) {
		return
	}
	c.Attacks[0] = r
}

// halfRoundUp = ovr14:0E52 (v): half of v, rounded up when DS:6DE3 is odd.
func (e *Engine) halfRoundUp(v byte) int {
	if e.combatRound&1 != 0 {
		v++
	}
	return int(v >> 1)
}

// sweep = ovr14:0E82 (att, vic) bool: a fighter with more attacks than... sweeps: the
// attacker (fewer attacks than +5 allows, victim not a "big" target) hits
// several adjacent weak enemies, one attack each.
func (e *Engine) sweep(a, b *Character) bool {
	res := false
	acb := a.Combat
	if a.Attacks[0] < acb.Sweeps && b.MaxLevel == 0 && e.rangeTo(a, b) == 1 {
		n := e.nearEnemies(a, 1)
		cnt := 0
		bi := 0
		for i := 1; i <= n; i++ {
			t := e.combatant(e.targets[i])
			if t == b {
				bi = i
			}
			if t.MaxLevel == 0 {
				cnt++
			}
		}
		if byte(cnt) > a.Attacks[0] {
			if byte(cnt) > acb.Sweeps {
				cnt = int(acb.Sweeps)
			}
			e.CharMessage(a, "sweeps", true)
			if e.combatant(e.targets[1]) != b {
				e.targets[bi] = e.targets[1] // (bi 0 when b was not found: DS:6DE4)
				e.targets[1] = byte(e.combatIndex(b))
			}
			for i := 1; i <= n; i++ {
				if cnt > 0 {
					t := e.combatant(e.targets[i])
					if t.MaxLevel == 0 {
						e.countAttackTurn(a, t)
						a.Attacks[0] = 1
						e.fullAttack(a, t, 0, nil)
						cnt--
					}
				}
			}
			res = true
		}
	}
	return res
}

// mayActAgainst = ovr14:1085 (t, c) bool: may t act against c? Affect group 1 of c can veto
// (DS:681E); then group 0 of t is run with c as its target.
func (e *Engine) mayActAgainst(t, c *Character) bool {
	if c == nil {
		return false
	}
	if t == c {
		return true
	}
	e.affectVeto = 0
	e.affectGroup(c, 1)
	if e.affectVeto == 0 {
		saved := t.Combat.Target
		t.Combat.Target = c
		e.affectGroup(t, 0)
		t.Combat.Target = saved
	}
	return e.affectVeto == 0
}

// clericTurnUndead = ovr14:116A (char): a cleric turns undead (ovr14:116A).
func (e *Engine) clericTurnUndead(c *Character) {
	e.CharMessage(c, "turns undead...", false)
	e.ClearRow24()
	e.MsgDelay()
	any, stop := false, false
	c.Combat.TurnUsed = 1
	limit := 6
	hd := e.Dice(1, 0xC)
	roll := e.Dice(1, 0x14)
	var cls int
	switch lv := int8(c.Levels[clsCleric]); {
	case lv >= 1 && lv <= 8:
		cls = int(lv)
	case lv >= 9 && lv <= 13:
		cls = 9
	default:
		cls = 10
	}
	var t *Character
	for e.weakestUnturned(c, &t) && hd > 0 && !stop {
		x := int(turnUndead[t.Undead][cls])
		ax := x
		if ax < 0 {
			ax = -ax
		}
		if roll < ax {
			stop = true
			continue
		}
		any = true
		e.combatRedrawChar(t, 3, 0)
		e.combatPanel(t)
		if x > 0 {
			t.Combat.Turned = 1
			e.hitMessage(t, true, "is turned")
		} else {
			e.CharMessage(t, "Is destroyed", false)
			e.combatRemoveFromMap(t)
			t.Status = statusGone
			t.Okay = 0
		}
		if limit > 0 {
			limit--
		}
		hd--
		if hd == 0 && limit > 0 && x < 0 {
			hd++
		}
		e.ClearMessageArea()
	}
	if !any {
		e.BottomMessage("Nothing Happens...")
	}
	e.countSides()
	e.clearCombatInfo(c)
	e.ClearMessageArea()
}

// weakestUnturned = ovr14:1351 (var out, char) bool: the weakest (lowest +0x76 > 0) enemy not
// yet turned, within reach.
func (e *Engine) weakestUnturned(c *Character, out **Character) bool {
	best := int8(0xD)
	*out = nil
	n := e.nearEnemies(c, 0xFF)
	found := false
	for i := 1; i <= n; i++ {
		t := e.combatant(e.targets[i])
		if t.Combat.Turned == 0 && int8(t.Undead) > 0 && int8(t.Undead) < best {
			best = int8(t.Undead)
			*out = t
			found = true
		}
	}
	return found
}

// attackSlots = ovr14:1406 (att, vic, w, var done): the attacker's attack slots against the
// victim; done is 1 when the attacker has used all its attacks.
func (e *Engine) attackSlots(a, b *Character, w int) (done bool) {
	acb := a.Combat
	done = false
	e.slotHits[1], e.slotHits[2] = 0, 0
	e.slotShots[1], e.slotShots[2] = 0, 0
	any, stop := false, false
	e.rv.damage = 0
	acb.Attacked = 1
	if e.isHelpless(b) {
		e.PlaySound(sndBump)
		for acb.AttackSlot > 0 && a.Attacks[acb.AttackSlot-1] == 0 {
			acb.AttackSlot--
		}
		e.slotShots[acb.AttackSlot]++
		e.attackText(a, b, 3, (int(b.HP)+5)&0xFF, 1, 1)
		e.removeInvisibility(a)
		for s := 1; s <= 2; s++ {
			a.Attacks[s-1] = 0
		}
		any = true
		done = true
	} else {
		if a.Slots[slotWeapon] != nil && (b.Size > 0x80 || b.Size&7 > 1) {
			typ := a.Slots[slotWeapon].Type
			a.Dice[0] = e.base(typ).LargeDice
			a.Sides[0] = e.base(typ).LargeSides
			a.Bonus[0] = byte(int(int8(a.Bonus[0])) - int(int8(e.base(typ).Bonus)) + int(int8(e.base(typ).LargeBonus)))
		}
		e.ApplyItems(b)
		e.affectGroup(b, 0xB)
		hand := 0
		var need byte
		if e.rangedFacing(a, b) {
			need = b.AC[1] - 2
		} else {
			if b.Combat.Attackers > 1 && byte(e.directionTo(a, b)) == b.Combat.Facing && b.Combat.Flank > 4 {
				w = 1
			}
			if w != 0 {
				hand = 1
			}
			need = b.AC[hand]
		}
		bonus := int(need)
		e.rangePenalty(a, b, &bonus)
		need = byte(bonus)
		mode := 0
		if w != 0 {
			mode = 1
		}
		if e.rangedFacing(a, b) {
			mode = 2
		}
		for s := int(acb.AttackSlot); s >= 1; s-- {
			for a.Attacks[s-1] != 0 && !stop {
				a.Attacks[s-1]--
				acb.AttackSlot = byte(s)
				e.slotShots[s]++
				if e.AttackHits(a, b, int(need)) || e.isHelpless(b) {
					e.slotHits[s]++
					e.PlaySound(sndHit) // extra: the weapon-hit sound (the original's bump is barely audible)
					any = true
					e.damageRoll(a, b, s)
					e.attackText(a, b, mode, int(e.rv.damage), int(e.rv.damage), 1)
					if b.Okay != 0 {
						e.affectGroup(a, s+1)
					}
					if b.Okay == 0 {
						stop = true
					}
				}
			}
		}
		if !any {
			e.PlaySound(sndMiss)
			e.attackText(a, b, mode, 0, 0, 0)
		}
		done = true
		for s := 1; s <= 2; s++ {
			if a.Attacks[s-1] > 0 {
				done = false
			}
		}
		acb.Sweeps = 0
	}
	if done {
		done = e.clearCombatInfo(a)
	}
	return
}

// countAttackTurn = ovr14:17F6 (att, vic): the victim counts one more attack and turns towards
// the attacker (the turned amount accumulates in +0x12).
func (e *Engine) countAttackTurn(att, vic *Character) {
	cb := vic.Combat
	cb.Attackers++
	d := (e.directionTo(vic, att) - int(cb.Facing) + 8) % 8
	if d > 4 {
		d = 8 - d
	}
	cb.Flank = byte((int(cb.Flank) + d) % 8)
}

// fullAttack = ovr14:1884 (att, vic, flag, item, var done): a full attack of att on vic
// (turning both, then the slots of attackSlots). item is a thrown item that is
// spent; done is set when the attacker's turn ends.
func (e *Engine) fullAttack(a, b *Character, flag int, it *Item) (done bool) {
	if b.Combat.Attackers < 3 && flag == 0 {
		dir := byte(e.directionTo(b, a))
		if e.combatOnScreen(b, false) {
			e.combatTurn(b, dir, 0, false)
		} else {
			b.Combat.Facing = dir
		}
	}
	dir := byte(e.directionTo(a, b))
	e.combatPanel(a)
	e.combatTurn(a, dir, 1, false)
	a.Combat.Target = b
	e.Delay(0x64)
	if it != nil {
		e.attackAnimation(a, b, it)
	}
	if a.Slots[slotWeapon] != nil && a.Slots[slotWeapon].Type == itemSling {
		e.attackAnimation(a, b, a.Slots[slotWeapon])
	}
	done = true
	if a.Attacks[0] > 0 || a.Attacks[1] > 0 {
		saved := e.partySel
		for i, c := range e.party {
			if c == a {
				e.partySel = i
			}
		}
		done = e.attackSlots(a, b, flag)
		if it != nil {
			if it.Count > 0 {
				it.Count -= e.slotShots[1]
			}
			if it.Count == 0 {
				if e.throwable(a) && it.Spells[2] != byte(affEndsSpiritualHammer) {
					cp := *it
					n := &cp
					e.run.treasure = append([]*Item{n}, e.run.treasure...)
					n.Readied = 0
					e.RemoveItem(a, it)
					e.thrownCopy = n
				} else {
					e.RemoveItem(a, it)
				}
			}
		}
		e.ApplyItems(a)
		e.partySel = saved
	}
	if done {
		done = e.clearCombatInfo(a)
	}
	if e.combatOnScreen(a, false) {
		e.combatTurn(a, a.Combat.Facing, 1, true)
		e.combatTurn(a, a.Combat.Facing, 0, false)
	}
	return
}

// rangePenalty = ovr14:1B1E (att, vic, var need): the attacker's range band: every full
// weapon range band beyond the first adds 2, then 3, to the number to hit.
func (e *Engine) rangePenalty(a, b *Character, need *int) {
	d := e.rangeTo(a, b)
	f := d
	if e.rangedWeapon(a) {
		f = (int(e.base(a.Slots[slotWeapon].Type).Range) - 1) / 3
	}
	if d-f > 0 {
		d -= f
		*need += 2
	}
	if d-f > 0 {
		d -= f
		*need += 3
	}
}
