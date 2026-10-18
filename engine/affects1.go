package engine

import "bytes"

// Affect handlers of ovr13, offsets 0x0000..0x1C00. Each handler is a
// DS:6828 table entry: handler(char, affect, flag); flag 1 = removing.
// The rule globals DS:6816.. are e.rv (fleeRoll and friends see rules.go).
// DS:6826 (a far pointer scratch) is a Go local; where the original left
// it holding the target of the combat block, we use c.Combat.Target.

// combatInfo is c.Combat; outside a fight the original reads garbage, we
// read a blank block.
func combatInfo(c *Character) *CombatInfo {
	if c.Combat == nil {
		return &CombatInfo{}
	}
	return c.Combat
}

// targetOf is the character at char+0x108 -> +0xA.
func targetOf(c *Character) *Character {
	if t := combatInfo(c).Target; t != nil {
		return t
	}
	return &Character{}
}

// pascalIs compares the character's name with a Pascal string.
func pascalIs(c *Character, s []byte) bool {
	n := min(int(s[0]), 15)
	return c.PName[0] == s[0] && bytes.Equal(c.PName[1:1+n], s[1:1+n])
}

// nameMatches: is the name one of the 16-byte Pascal strings in names? Returns the number of matches (the originals count each).
func (e *Engine) nameMatches(c *Character, names [][16]byte) int {
	m := 0
	for i := range names {
		if pascalIs(c, names[i][:]) {
			m++
		}
	}
	return m
}

// blockAffect = ovr13:0000 (type): the affect being added is refused when
// type is 0 or the one being added; the damage goes with it.
func (e *Engine) blockAffect(typ affectType) {
	if typ == 0 || e.rv.affVal == typ {
		e.rv.damage = 0
		e.rv.affVal = 0
	}
}

// addAffectNow = ovr13:001F (char, type, data, time): adds the affect with
// the remove-call flag, false while the temple is curing.
func (e *Engine) addAffectNow(c *Character, typ affectType, data byte, time int) bool {
	if e.rv.cure != 0 {
		return false
	}
	e.AddAffect(c, typ, time, data, 1)
	return true
}

// killWithMessage = ovr13:0058 (char, status, message): the message, then
// the character goes down with the status unless already dead.
func (e *Engine) killWithMessage(c *Character, status byte, msg string) {
	msg = cut(msg, 0x28)
	e.CharMessage(c, msg, 0xA, false)
	if s := c.Status; s == 7 || s == 6 || s == 8 {
		return
	}
	c.Status = status
	c.Okay = 0
	c.HP = 0
	e.removeCombatAffects(c)
	e.affectGroup(c, grpDown)
	if c.Okay == 0 {
		e.combatRemoveFromMap(c)
	}
	e.MsgDelay()
	e.ClearMessageArea()
	if e.beginFlag != 5 {
		e.DisplayPartyList()
	}
}

// readiedWeapon = ovr13:101D (char): the weapon found by 00BA:2E8B, else
// the first readied item.
func (e *Engine) readiedWeapon(c *Character) *Item {
	var it *Item
	if c.Slots[0] != nil {
		ok := e.ammoFor(c, &it)
		if !ok || it == nil {
			it = c.Slots[0]
		}
	}
	return it
}

// avoidDistantAttack = ovr13:0FAC (char, percent): the selected character
// holding a ranged weapon may avoid an attack from afar.
func (e *Engine) avoidDistantAttack(c *Character, pct byte) {
	sel := e.selected()
	if sel.Slots[0] != nil && e.rangeTo(sel, c) > 1 && e.Dice(1, 100) <= int(pct) {
		e.CharMessage(c, "Avoids it", 0xA, true)
		e.rv.damage = 0
		e.rv.hitRoll = -1
		e.slotHits[1]--
	}
}

// poisonAttack = ovr13:1545 (char, bonus): the target of the combat block
// is poisoned unless it saves; no save means death.
func (e *Engine) poisonAttack(c *Character, bonus int) {
	t := targetOf(c)
	if !e.SavingThrow(t, 0, bonus) {
		e.CharMessage(t, "is Poisoned", 0xA, false)
		e.MsgDelay()
		e.AddAffect(t, affPoisoned, 0, 0xFF, 0)
		e.killWithMessage(t, 6, "dies from poison") // extra: the original says "is killed"
	}
}

// paralyseTarget = ovr13:15EB (char, time, extra): the target is paralysed
// unless it saves. extra (bp+6) is never used by the original.
func (e *Engine) paralyseTarget(c *Character, time int, extra byte) {
	t := targetOf(c)
	if !e.SavingThrow(t, 0, 0) {
		e.hitMessage(t, true, "is Paralyzed")
		e.AddAffect(t, affHeld, time, 0xC, 0)
	}
}

// hitBonusUp = ovr13:010D: to-hit and DS:6825 up.
func (e *Engine) hitBonusUp(c *Character, a *Affect, flag byte) {
	e.fleeRoll += 5
	e.rv.hitRoll++
}

// hitBonusDown = ovr13:011D: the reverse.
func (e *Engine) hitBonusDown(c *Character, a *Affect, flag byte) {
	if e.fleeRoll < 5 {
		e.fleeRoll = 0
	} else {
		e.fleeRoll -= 5
	}
	e.rv.hitRoll--
}

// bonusVsRace4 = ovr13:013B: +2 to hit and damage against a target of race 4.
func (e *Engine) bonusVsRace4(c *Character, a *Affect, flag byte) {
	if targetOf(c).Kind == 4 {
		e.rv.hitRoll += 2
		e.rv.damage += 2
	}
}

// startsToTrain = ovr13:017F: "starts to train".
func (e *Engine) startsToTrain(c *Character, a *Affect, flag byte) {
	e.CharMessage(c, "starts to train", 0xA, true)
	e.AddAffect(c, affTrainingManual, 0xA8C0, 0xFF, 1)
}

// bonusByTargetRace = ovr13:01C3: bonus by the target's race, fire-ish magic damage.
func (e *Engine) bonusByTargetRace(c *Character, a *Affect, flag byte) {
	var k byte
	switch targetOf(c).Kind {
	case 0xA:
		k = 1
	case 9, 0xC:
		k = 2
	case 4:
		k = 3
	}
	e.rv.hitRoll += int8(k)
	e.rv.damage += k
	e.rv.dmgType = 9
}

// isHardier = ovr13:0233: "is hardier" (the trained Constitution grows).
func (e *Engine) isHardier(c *Character, a *Affect, flag byte) {
	e.CharMessage(c, "is hardier", 0xA, true)
	c.Stats[statCon]++
	if c.Stats[statCon] >= 0x14 {
		e.AddAffect(c, affRegenerate, 0x3C, 0xFF, 1)
		return
	}
	gain := c.MaxHP - c.HPRoll
	var div byte
	if c.Stats[statCon] < 0xF {
		return
	}
	for i := 0; i <= 7; i++ {
		v := int8(c.Levels[i])
		if v <= 0 {
			continue
		}
		switch {
		case i >= 2 && i <= 4: // fighter, paladin, ranger
			div += byte(int(v) * (int(c.Stats[statCon]) - 0xF))
		case c.Stats[statCon] > 0xF:
			div += byte(v * 2)
		default:
			div += byte(v)
		}
	}
	if div == 0 {
		div = 1
	}
	gain /= div
	if c.Stats[statCon] < 0x11 || fighterKind(c) {
		c.MaxHP += gain
		c.HP += gain
	}
}

// protFromEvil = ovr13:0377: against an evil attacker (the selected
// character's alignment 2/5/8, alignmentNames), save +2, its to-hit -2.
func (e *Engine) protFromEvil(c *Character, a *Affect, flag byte) {
	if v := e.selected().Alignment; v == 2 || v == 5 || v == 8 {
		e.rv.save += 2
		e.rv.hitRoll -= 2
	}
}

// protFromGood = ovr13:03AC: the same against a good one (0/3/6).
func (e *Engine) protFromGood(c *Character, a *Affect, flag byte) {
	if v := e.selected().Alignment; v == 0 || v == 3 || v == 6 {
		e.rv.save += 2
		e.rv.hitRoll -= 2
	}
}

// resistColdHalf = ovr13:03E1: cold damage halved, save +3.
func (e *Engine) resistColdHalf(c *Character, a *Affect, flag byte) {
	if e.rv.dmgType&2 != 0 {
		e.rv.damage >>= 1
		e.rv.save += 3
	}
}

// charmSide = ovr13:0400: charm: the side follows the affect.
func (e *Engine) charmSide(c *Character, a *Affect, flag byte) {
	if flag != 0 {
		c.Side = (a.Data & 0x40) >> 6
		if c.Control == 0xB3 {
			c.Control = 0
		}
		return
	}
	if a.Data&0x20 == 0 {
		a.Data = a.Data + 0x20 + c.Side<<6
		c.Side = a.Data >> 7
		c.Auto = 1
		if c.Control <= 0x7F {
			c.Control = 0xB3
		}
		combatInfo(c).Target = nil
		e.countSides()
	}
	e.fleeRoll = 0x64
}

// bestStrength = ovr13:04BA: strength affects (the best one counts).
func (e *Engine) bestStrength(c *Character, a *Affect, flag byte) {
	if a.Data > 0x7F {
		return
	}
	c.Stats[statStr], c.Str00 = strDecode(a)
	var best *Affect
	var bs, bs00 byte
	for _, n := range c.Affects {
		if (n.Type == affEnlarged || n.Type == affStrength) && n != a {
			s, s00 := strDecode(n)
			if s > bs || bs == 0x12 && s00 > bs00 {
				bs, bs00, best = s, s00, n
			}
		}
	}
	if best != nil {
		best.Data = strValue(c.Stats[statStr], c.Str00)
		c.Stats[statStr], c.Str00 = bs, bs00
	}
}

// setCharisma = ovr13:05D6.
func (e *Engine) setCharisma(c *Character, a *Affect, flag byte) { c.Stats[statCha] = a.Data }

// poisonDamage = ovr13:05EB: poison: damage each round.
func (e *Engine) poisonDamage(c *Character, a *Affect, flag byte) {
	if e.addAffectNow(c, affPoisonDamage, a.Data, 0xA) && c.HP > 1 {
		e.rv.dmgType = 0
		e.Damage(c, 1, 0, false)
		if e.beginFlag != 5 {
			e.DisplayPartyList()
		}
	}
}

// armourAtLeast = ovr13:064E: armour class at least 0x39, save +1.
func (e *Engine) armourAtLeast(c *Character, a *Affect, flag byte) {
	if c.AC[0] < 0x39 {
		c.AC[0] = 0x39
	}
	e.rv.save++
	if e.curSpell == 0xF {
		e.rv.damage = 0
	}
}

// hitBonusVsNames = ovr13:0679: +1 to hit per matching name in DS:0332 (race 1, class 1).
func (e *Engine) hitBonusVsNames(c *Character, a *Affect, flag byte) {
	t := targetOf(c)
	if t.Kind == 1 && t.Size == 1 {
		e.rv.hitRoll += int8(e.nameMatches(t, hitNames1[:]))
	}
}

// resistFireHalf = ovr13:06E4: fire damage halved, save +3.
func (e *Engine) resistFireHalf(c *Character, a *Affect, flag byte) {
	if e.rv.dmgType&1 != 0 {
		e.rv.damage >>= 1
		e.rv.save += 3
	}
}

// isSilenced = ovr13:070F: "is silenced" (the spell being cast is lost).
func (e *Engine) isSilenced(c *Character, a *Affect, flag byte) {
	ci := combatInfo(c)
	if ci.CanUse != 0 {
		e.CharMessage(c, "is silenced", 0xA, true)
	}
	ci.CanUse = 0
	ci.CanCast = 0
}

// poisonEnds = ovr13:0775: poison ends; death by poison when the poison was still in.
func (e *Engine) poisonEnds(c *Character, a *Affect, flag byte) {
	if e.FindAffect(c, affPoisoned) != nil {
		e.killWithMessage(c, 6, "dies from poison")
	}
	e.rv.cure = 1
	e.RemoveAffect(c, affPoisonDamage, nil)
	e.rv.cure = 0
}

// spiritualHammerItem = ovr13:07E0: the Spiritual Hammer (a hammer with name word 0xF3) comes and goes.
func (e *Engine) spiritualHammerItem(c *Character, a *Affect, flag byte) {
	var it *Item
	for _, i := range c.Items {
		if i.Type == itemHammer && i.Words[2] == 0xF3 {
			it = i
			break
		}
	}
	if flag != 0 && it != nil {
		e.RemoveItem(c, it)
	}
	if flag == 0 && it == nil && c.ItemCount < 0x10 {
		n := &Item{}
		n.Type = itemHammer
		n.Words[1] = byte(itemHammer)
		n.Words[2] = 0xF3
		n.Plus = 1
		n.Spells[1] = 0x17
		n.Spells[2] = 0x89
		e.AddItemCopy(c, n)
		e.CharMessage(c, "Gains an item", 0xA, true)
	}
	e.ApplyItems(c)
}

// blindedToHit = ovr13:0918: to-hit -4; DS:681E unless the selected one has affect 0x18.
func (e *Engine) blindedToHit(c *Character, a *Affect, flag byte) {
	if e.FindAffect(e.selected(), affDetectInvisible) == nil {
		e.affectVeto = 1
	}
	e.rv.hitRoll -= 4
}

// hitBonusVsTargetNames = ovr13:0947: +1 to hit per matching name in DS:0372 (target: race 1, class 1).
func (e *Engine) hitBonusVsTargetNames(c *Character, a *Affect, flag byte) {
	t := targetOf(c)
	if t.Kind == 1 && t.Size == 1 {
		e.rv.hitRoll += int8(e.nameMatches(t, hitNames2[:]))
	}
}

// mirrorImage = ovr13:09C0: mirror image: an attack may hit an image instead.
func (e *Engine) mirrorImage(c *Character, a *Affect, flag byte) {
	if e.Dice(1, int(a.Data)+1) > 1 && e.curSpell > 0 && e.spellOwnTargets == 0 {
		e.blockAffect(affAny)
		e.CharMessage(c, "lost an image", 0xA, true)
		a.Data--
		if a.Data == 0 {
			e.RemoveAffect(c, affMirrorImage, nil)
		}
	}
}

// damageLessQuarter = ovr13:0A3D: damage down by a quarter.
func (e *Engine) damageLessQuarter(c *Character, a *Affect, flag byte) {
	e.rv.damage -= e.rv.damage >> 2
}

// coughingEnds = ovr13:0A65: "is coughing" ends; armour class drops.
func (e *Engine) coughingEnds(c *Character, a *Affect, flag byte) {
	ci := combatInfo(c)
	if ci.CanUse != 0 {
		e.CharMessage(c, "is coughing", 0xA, true)
	}
	ci.CanUse = 0
	ci.CanCast = 0
	e.ApplyItems(c)
	if c.AC[1] > 0x34 {
		c.AC[1] -= 2
	} else {
		c.AC[1] = 0x32
	}
	c.AC[0] = c.AC[1]
	if c == e.selected() {
		e.combatPanel(c)
	}
}

// collapses = ovr13:0B1B: "collapses" (status 6) and the character is reset.
func (e *Engine) collapses(c *Character, a *Affect, flag byte) {
	a.Call = 0
	if e.rv.cure == 0 {
		e.killWithMessage(c, 6, "collapses")
	}
	c.Side = a.Data >> 4
	c.Auto = 1
	c.Undead = 0
	c.FightLevel = c.Levels[clsFighter]
	c.BaseMove = 0xC
	if c.Control == 0xB3 {
		c.Control = 0
	}
	c.Kind = 0
}

// minus4All = ovr13:0BAC: -4 to hit, armour and save.
func (e *Engine) minus4All(c *Character, a *Affect, flag byte) {
	e.rv.hitRoll -= 4
	c.AC[0] -= 4
	c.AC[1] -= 4
	e.rv.save -= 4
}

// weakAndSick = ovr13:0BCF: affects 0x2B and 0x2C together.
func (e *Engine) weakAndSick(c *Character, a *Affect, flag byte) {
	e.callAffect(affWeakened, c, a, flag)
	e.callAffect(affSick, c, a, flag)
}

// plusOneSaveHit = ovr13:0C06: +1 save and to-hit.
func (e *Engine) plusOneSaveHit(c *Character, a *Affect, flag byte) {
	e.rv.save++
	e.rv.hitRoll++
}

// minus4HitSave = ovr13:0C15: -4 to hit and save.
func (e *Engine) minus4HitSave(c *Character, a *Affect, flag byte) {
	e.rv.hitRoll -= 4
	e.rv.save -= 4
}

// alwaysMissDelayed = ovr13:0C26: DS:681E and the attack always misses.
func (e *Engine) alwaysMissDelayed(c *Character, a *Affect, flag byte) {
	if int8(combatInfo(c).Delay) > 0 {
		e.affectVeto = 1
		e.rv.hitRoll = -1
	}
}

// agesOnce = ovr13:0C4B: "ages" once (age +1), DS:681A doubled.
func (e *Engine) agesOnce(c *Character, a *Affect, flag byte) {
	if a.Data&0x10 == 0 {
		a.Data += 0x10
		e.CharMessage(c, "ages", 0xA, true)
		c.Age++
	}
	e.turnValue <<= 1
}

// clearStinkingCloud = ovr13:0CC3: the stinking cloud of this caster and side is cleared: its
// squares get back their ground, the cloud is freed, the squares of the
// other clouds are marked again.
func (e *Engine) clearStinkingCloud(c *Character, a *Affect, flag byte) {
	side := a.Data >> 4
	ci := -1
	for i, cl := range e.clouds {
		if cl.Ptr00 == c && cl.Num == side {
			ci = i
			break
		}
	}
	if ci < 0 {
		return
	}
	cl := e.clouds[ci]
	e.BottomMessage("The air clears a little...")
	square := func(cl *cloud, k int) (x, y byte) {
		idx := cloudDirs[k]
		return byte(dirDX[idx] + int(cl.X)), byte(dirDY[idx] + int(cl.Y))
	}
	set := func(x, y, v byte) { // the original does not check the bounds
		if int(y) < len(e.cmap.ground) && int(x) < len(e.cmap.ground[0]) {
			e.cmap.ground[y][x] = v
		}
	}
	for k := 1; k <= 4; k++ {
		if cl.Covered[k-1] == 0 {
			continue
		}
		x, y := square(cl, k)
		body := false
		for i := 1; i <= e.nbodies; i++ {
			if b := &e.bodies[i]; b.c != nil && b.x == x && b.y == y {
				body = true
			}
		}
		if body {
			set(x, y, 0x1F)
		} else {
			set(x, y, cl.Ground[k-1])
		}
	}
	e.clouds = append(e.clouds[:ci:ci], e.clouds[ci+1:]...)
	for _, p := range e.clouds {
		for k := 1; k <= 4; k++ {
			if p.Covered[k-1] != 0 {
				x, y := square(p, k)
				set(x, y, 0x1E)
			}
		}
	}
}

// avoidUnmagicWeapon = ovr13:1086: the selected character with a weapon of strength 0 may avoid
// the attack.
func (e *Engine) avoidUnmagicWeapon(c *Character, a *Affect, flag byte) {
	if w := e.readiedWeapon(e.selected()); w != nil && w.Plus == 0 {
		e.avoidDistantAttack(c, 0x64)
	}
}

// halveAttacks = ovr13:10C1: DS:681A halved.
func (e *Engine) halveAttacks(c *Character, a *Affect, flag byte) { e.turnValue >>= 1 }

// isWeakened = ovr13:10DE: "is weakened": strength down while it lasts (affect 0x2B).
func (e *Engine) isWeakened(c *Character, a *Affect, flag byte) {
	if !e.addAffectNow(c, affWeakened, a.Data, 0x3C) {
		return
	}
	if c.Stats[statStr] > 3 {
		e.CharMessage(c, "is weakened", 0xA, true)
		c.Stats[statStr]--
	} else if e.FindAffect(c, affHelpless) == nil {
		e.AddAffect(c, affHelpless, 0, 0xFF, 0)
	}
}

// sickDamage = ovr13:1168: affect 0x2C: damage while it lasts, or nausea at 1 hit point.
func (e *Engine) sickDamage(c *Character, a *Affect, flag byte) {
	if !e.addAffectNow(c, affSick, a.Data, 0xA) {
		return
	}
	if c.HP > 1 {
		e.rv.dmgType = 0
		e.Damage(c, 1, 0, false)
		if e.beginFlag != 5 {
			e.DisplayPartyList()
		}
	} else if e.FindAffect(c, affHelpless) == nil {
		e.AddAffect(c, affHelpless, 0, 0xFF, 0)
	}
}

// minus4VsNames = ovr13:11F9: -4 to hit against the names in DS:03F2 (selected: race 1, race bits 2).
func (e *Engine) minus4VsNames(c *Character, a *Affect, flag byte) {
	sel := e.selected()
	if sel.Kind == 1 && sel.Size&0x7F == 2 {
		e.rv.hitRoll -= int8(4 * e.nameMatches(targetOf(c), hitNames3[:]))
	}
}

// minus4VsBugbearGnoll = ovr13:1276: -4 to hit when the selected is a BUGBEAR or GNOLL of race 1.
func (e *Engine) minus4VsBugbearGnoll(c *Character, a *Affect, flag byte) {
	sel := e.selected()
	if sel.Kind == 1 &&
		(pascalIs(sel, []byte("\x07BUGBEAR")) || pascalIs(sel, []byte("\x05GNOLL"))) {
		e.rv.hitRoll -= 4
	}
}

// prayerSide = ovr13:12B2: same side as the affect: plusOneSaveHit, else -1 to hit and save.
func (e *Engine) prayerSide(c *Character, a *Affect, flag byte) {
	if c.Side == (a.Data&0x10)>>4 {
		e.plusOneSaveHit(c, a, flag)
	} else {
		e.rv.hitRoll--
		e.rv.save--
	}
}

// healOverTime = ovr13:12FB: affect 0x32 and a point of healing.
func (e *Engine) healOverTime(c *Character, a *Affect, flag byte) {
	if e.addAffectNow(c, affMummyDisease, a.Data, 0x3840) && e.Heal(c, 1, true) {
		e.healedMessage(c)
	}
}

// clearCombatBlock = ovr13:133F: the combat block is cleared.
func (e *Engine) clearCombatBlock(c *Character, a *Affect, flag byte) { e.clearCombatInfo(c) }

// turnInvisible = ovr13:1363.
func (e *Engine) turnInvisible(c *Character, a *Affect, flag byte) {
	e.AddAffect(c, affInvisible, 1, 0xC, 0)
}

// rots = ovr13:1399: "Rots...": charisma drops, then the affect or death.
func (e *Engine) rots(c *Character, a *Affect, flag byte) {
	if e.rv.cure != 0 {
		return
	}
	e.CharMessage(c, "Rots...", 0xA, true)
	if c.Stats[statCha] > 2 {
		c.Stats[statCha] -= 2
	} else {
		c.Stats[statCha] = 0
	}
	if c.Stats[statCha] < 3 {
		c.Stats[statCha] = 3
	}
	if a.Data>>4 > 1 {
		a.Data -= 0x10
		e.addAffectNow(c, affRotting, a.Data, 0xA8C0)
	} else {
		e.killWithMessage(c, 8, "Dies rots away")
	}
}

// heldNoMoves = ovr13:1452: the combat block loses +6; DS:681A cleared when DS:681D is set.
func (e *Engine) heldNoMoves(c *Character, a *Affect, flag byte) {
	combatInfo(c).Moves = 0
	if e.modMoves != 0 {
		e.turnValue = 0
	}
}

// addAffect62 = ovr13:1472.
func (e *Engine) addAffect62(c *Character, a *Affect, flag byte) {
	e.AddAffect(c, affHeal3, 0, 0xFF, 0)
}

// resistFirePerDie = ovr13:1490: fire damage is cut by 2 per die (not below one per die), save +4.
func (e *Engine) resistFirePerDie(c *Character, a *Affect, flag byte) {
	if e.rv.dmgType&1 == 0 {
		return
	}
	for i := byte(1); i <= e.rv.diceN; i++ {
		e.rv.damage -= 2
		if e.rv.damage < e.rv.diceN {
			e.rv.damage = e.rv.diceN
		}
	}
	e.rv.save += 4
	if e.rv.dmgType&8 == 0 {
		e.blockAffect(affAny)
	}
}

// regenerate = ovr13:14EB: affect 0x3E and a point of healing.
func (e *Engine) regenerate(c *Character, a *Affect, flag byte) {
	if e.addAffectNow(c, affRegenerate, a.Data, 0x3C) && e.Heal(c, 1, true) {
		e.healedMessage(c)
	}
}

// poisonBite = ovr13:165D, 1671, 1685: poison bites with save bonus 0, 4, 2.
func (e *Engine) poisonBite(c *Character, a *Affect, flag byte)  { e.poisonAttack(c, 0) }
func (e *Engine) poisonBite4(c *Character, a *Affect, flag byte) { e.poisonAttack(c, 4) }
func (e *Engine) poisonBite2(c *Character, a *Affect, flag byte) { e.poisonAttack(c, 2) }

// paralyse2d8 = ovr13:1699: paralysis for 2d8.
func (e *Engine) paralyse2d8(c *Character, a *Affect, flag byte) {
	e.paralyseTarget(c, e.Dice(2, 8), 0)
}

// paralyseNotElf = ovr13:16BB: paralysis for 0x3F unless the target is an elf.
func (e *Engine) paralyseNotElf(c *Character, a *Affect, flag byte) {
	if targetOf(c).Race != raceElf {
		e.paralyseTarget(c, 0x3F, 0)
	}
}

// paralyse1d9 = ovr13:16E6: paralysis for 1d9+10.
func (e *Engine) paralyse1d9(c *Character, a *Affect, flag byte) {
	e.paralyseTarget(c, e.Dice(1, 9)+10, 0xFE)
}

// poisonBiteMinus2 = ovr13:170B: poison with save bonus 0xFE (-2).
func (e *Engine) poisonBiteMinus2(c *Character, a *Affect, flag byte) { e.poisonAttack(c, 0xFE) }

// missMinus4 = ovr13:171F: DS:681E, to-hit -4.
func (e *Engine) missMinus4(c *Character, a *Affect, flag byte) {
	e.affectVeto = 1
	e.rv.hitRoll -= 4
}

// mostlyInvisible = ovr13:1730: invisibility 95% of the time.
func (e *Engine) mostlyInvisible(c *Character, a *Affect, flag byte) {
	if e.Dice(1, 100) <= 0x5F {
		e.AddAffect(c, affInvisible, 1, 0xC, 0)
	}
}

// bloodDrain = ovr13:176F: blood drain (affect 0x4B on the drinker, 0x3A on the victim,
// the victim is combatant a.Raw[3]).
func (e *Engine) bloodDrain(c *Character, a *Affect, flag byte) {
	t := e.combatant(a.Data)
	if t == nil {
		t = &Character{}
	}
	if flag != 0 || c.Okay == 0 || t.Okay == 0 {
		e.RemoveAffect(t, affHeldFast, nil)
		if flag == 0 {
			a.Call = 0
			e.RemoveAffect(c, affBloodDrain, a)
		}
		if c.Okay != 0 && t.Okay != 0 {
			combatInfo(c).Turned = 1
		}
		return
	}
	e.CharMessage(c, "Sucks some Blood", 0xA, true)
	dmg := e.RollDice(1, 4)
	e.rv.dmgType = 0
	e.Damage(t, dmg, 0, false)
	e.clearCombatInfo(c)
	time := int(a.Time)
	if t.Okay != 0 && time>>4 > dmg {
		time -= dmg << 4
		a.Time = uint16(time)
	} else {
		e.RemoveAffect(c, affBloodDrain, a)
	}
}

// holdTarget = ovr13:18B5: the target is held (affect 0x4B on c with the target's index).
func (e *Engine) holdTarget(c *Character, a *Affect, flag byte) {
	t := targetOf(c)
	e.AddAffect(c, affBloodDrain, 0xCF, byte(e.combatIndex(t)), 1)
	e.AddAffect(t, affHeldFast, 0, 0xFF, 0)
	e.callAffect(affHeldFast, t, nil, 0)
}

// holdWeakTarget = ovr13:1935: hold: a target under 0x13 strength is held fast.
func (e *Engine) holdWeakTarget(c *Character, a *Affect, flag byte) {
	t := targetOf(c)
	if t.Stats[statStr] < 0x13 {
		e.AddAffect(c, affStrikeTied, 0, byte(e.combatIndex(t)), 1)
		e.AddAffect(t, affHeldFast, 0, 0xFF, 0)
		e.callAffect(affHeldFast, t, nil, 0)
		e.CharMessage(t, "is held fast", 0xA, true)
	}
}

// getUpOrWait = ovr13:19D9: back on its feet or affect 0x4E (the local result is unused).
func (e *Engine) getUpOrWait(c *Character, a *Affect, flag byte) {
	if !e.Revive(c, c.HP) {
		e.addAffectNow(c, affGetUp, a.Data, 1)
	}
}

// magicDamage2d10 = ovr13:1A19: 2d10 magic damage to the target.
func (e *Engine) magicDamage2d10(c *Character, a *Affect, flag byte) {
	e.rv.dmgType = 9
	t := targetOf(c)
	e.Damage(t, e.RollDice(2, 10), 0, false)
}

// acidDamage1d4 = ovr13:1A4C: 1d4 acid damage to the target.
func (e *Engine) acidDamage1d4(c *Character, a *Affect, flag byte) {
	e.rv.dmgType = 0x10
	t := targetOf(c)
	e.Damage(t, e.RollDice(1, 4), 0, false)
}

// fear = ovr13:1AAA: fear: the enemies save or are paralysed / weakened (by +0x73).
func (e *Engine) fear(c *Character, a *Affect, flag byte) {
	for _, p := range e.party {
		if p.Okay == 0 || byte(b2i(isParty(c))) != p.Side {
			continue
		}
		saved := e.SavingThrow(p, 4, 0)
		switch v := int8(p.MaxLevel); {
		case v >= 0 && v <= 3:
			e.AddAffectMsg(p, affHeld, 0, 0xC, 0, 1, saved, "is paralyzed with fear")
		case v == 4 || v == 5:
			e.AddAffectMsg(p, affCurse, 0, 0xC, 0, 1, saved, "is weakened by fear")
		}
	}
}

// fearLifted = ovr13:1BA8: the fear (0x52) is lifted from everyone and the enemies are
// paralysed for 1d4 rounds.
func (e *Engine) fearLifted(c *Character, a *Affect, flag byte) {
	for _, p := range e.party {
		e.RemoveAffect(p, affFearLifted, nil)
		if p.Okay == 0 || byte(b2i(isParty(c))) != p.Side {
			continue
		}
		var bonus int
		if byte(e.rv.side[p.Side]) >= 6 {
			bonus++
		}
		if p.Race == raceHuman {
			bonus += 2
		}
		saved := e.SavingThrow(p, 0, bonus)
		e.AddAffectMsg(p, affHeld, e.Dice(1, 4), 0xC, 0, 1, saved, "is paralyzed with fear")
	}
}
