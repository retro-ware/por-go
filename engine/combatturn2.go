package engine

import (
	"image"
	"slices"
	"strconv"
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
	e.ext.aoe, e.ext.bolt, e.ext.aoeSpell = 1, bolts[spell], spell // extra: area preview (aoe.go); one target: its own square
	if kind > 8 && kind <= 0xE {
		e.ext.aoe = int(kind&7) + 1
	}
	defer func() { e.ext.aoe, e.ext.bolt, e.ext.aoeSpell = 0, [2]int{}, 0 }()
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
				e.refuse("Already been targeted")
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
	*quit = false
	if spell > 0 && e.spellData(spell, 5) == 0 {
		e.refuse("Camp Only Spell")
		spell = 0
	}
	if a == 0 {
		e.ext.panel = sel
	}
	if spell > 0 {
		delay := int(e.spellData(spell, 6)) / 3
		if delay == 0 {
			e.CastSpell(byte(spell), byte(a), 1, quit)
			if e.ext.castCancelled { // extra: the aim was cancelled: back to the menu, the turn (and its moves) go on
				e.ext.castCancelled = false
			} else {
				*quit = e.clearCombatInfo(sel)
			}
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
	if !e.ask("Attack Ally?") {
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

// playerAim = ovr14:3558 (c, rng, showRange, allowGround, var found, var t):
// the player aims for c. extra: through Act: ActPoint moves the aim (the
// View shows why it cannot be taken, a spell's area), ActAim takes a
// square, ActClose gives up. Battle.Targets are the foes to cycle through
// (with a weapon: the ones it can hit, while there are any). rng 0xFF
// takes the range of the readied weapon.
func (e *Engine) playerAim(c *Character, rangeArg, showRange, allowGround int, found *bool, t *aimTarget) {
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
	e.buildNear(e.combatX(c), e.combatY(c), 0x7F, 0xFF, e.combatSize(c)) // ovr14:32E6
	list := slices.Clone(e.near)
	if len(list) == 0 { // extra: nobody to aim at
		return
	}
	hitOnly := false // with a weapon, the foes it can hit, while one can be
	foe := func(f *Character) bool { return f != nil && f.Side != c.Side && e.mayActAgainst(c, f) }
	for _, n := range list {
		if f := e.combatant(n.index); foe(f) && showRange != 0 && e.canTarget(c, f, int(rng), showRange) {
			hitOnly = true
		}
	}
	turn, targets, hits := e.snap.turn, e.snap.targets, e.snap.hits
	defer func() { e.snap.turn, e.snap.targets, e.snap.hits, e.snap.aimWhy = turn, targets, hits, "" }()
	e.snap.targets, e.snap.hits = nil, map[int]int{}
	for _, n := range list {
		f := e.combatant(n.index)
		if f != nil && !e.ext.enemiesOnly || foe(f) && (!hitOnly || e.canTarget(c, f, int(rng), showRange)) {
			e.snap.targets = append(e.snap.targets, int(n.index))
			if hitOnly && foe(f) {
				e.snap.hits[int(n.index)] = e.hitChance(c, f)
			}
		}
	}
	e.snap.turn = &Turn{Unit: e.combatIndex(c), Aim: true}
	undo := func() {}
	defer func() { undo(); e.cmap.cursor = 0; e.snap.cursorOn = false }()
	point := func(x, y int) (*Character, string) {
		undo()
		cand, why := e.aimCheck(c, list, x, y, int(rng), showRange, allowGround)
		e.cmap.cursor, e.cmap.cursorSize = 1, 1
		e.snap.cursor, e.snap.cursorOn, e.snap.aimWhy = image.Pt(x, y), true, why
		_, undo = e.aoeMarks(x, y, "")
		return cand, why
	}
	at := c
	if len(e.snap.targets) > 0 {
		at = e.combatant(byte(e.snap.targets[0]))
	}
	point(e.combatX(at), e.combatY(at))
	for {
		a := e.act()
		if a.Kind == ActClose {
			return
		}
		if a.Kind != ActPoint && a.Kind != ActAim || a.X < 0 || a.Y < 0 || a.X > 0x31 || a.Y > 0x18 {
			continue
		}
		cand, why := point(a.X, a.Y)
		if a.Kind == ActPoint {
			continue
		}
		if why != "" {
			e.refuse(why)
			continue
		}
		t.x, t.y, t.c = byte(a.X), byte(a.Y), cand
		if showRange == 1 || cand != nil && e.occ[a.Y][a.X] > 0 {
			e.aimConfirm(c, cand, showRange, found, t) // ovr14:2C3D: the target's own square; a weapon strikes
		} else {
			*found = true
		}
		if *found {
			return
		}
	}
}

// aimCheck (extra) is aimPrompt's and aimManual's test of the square
// (x, y) as the aim: who would be the target, and why it cannot be taken
// ("" when it can). A combatant in sight (list) is tested as aimPrompt
// does, any other square as aimManual's cursor.
func (e *Engine) aimCheck(c *Character, list []nearEntry, x, y, rng, showRange, allowGround int) (*Character, string) {
	who, ground := e.mapAt(x, y)
	if who > 0 && slices.ContainsFunc(list, func(n nearEntry) bool { return n.index == who }) {
		cand := e.combatant(who)
		if why := e.targetWhy(c, cand, rng, showRange); why != "" {
			return cand, why
		}
		if !e.mayActAgainst(c, cand) {
			return cand, "Cannot act against " + cand.Name()
		}
		return cand, ""
	}
	dist, tx, ty := 0xFF, x, y
	if !e.lineOfSight(&dist, &tx, &ty, e.combatX(c), e.combatY(c)) {
		return nil, "Out of sight"
	}
	dist >>= 1
	var cand *Character
	if who > 0 {
		cand = e.combatant(who)
	} else if ground == 0x1F {
		for b := 1; b <= e.nbodies; b++ {
			if int(e.bodies[b].x) == x && int(e.bodies[b].y) == y {
				cand = e.bodies[b].c
			}
		}
	}
	switch {
	case rng < dist:
		return cand, outOfRange(rng)
	case e.groundInfo(ground)[0] == 0xFF:
		return cand, "A wall is in the way"
	case cand == nil && allowGround == 0:
		return nil, "No one there to target"
	case cand == nil:
		return nil, ""
	case !e.mayActAgainst(c, cand):
		return cand, "Cannot act against " + cand.Name()
	case showRange != 1:
		return cand, ""
	case cand == c || who == 0 && ground == 0x1F:
		return cand, "Not a target"
	}
	if e.rangedWeapon(c) {
		var tmp *Item
		if !e.ammoFor(c, &tmp) {
			return cand, "No ammunition"
		} else if e.nearEnemies(c, 1) > 0 && !e.throwable(c) {
			return cand, "A foe is too close to shoot"
		}
	}
	return cand, ""
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
