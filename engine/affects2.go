package engine

import (
	"fmt"
	"strconv"
)

// Affect handlers of GAME.OVR overlay 13 from 0x1C00 to the end, and the
// ovr13:31A2 routine that fills the DS:6828 handler table.

// setupAffectHandlers = ovr13:31A2: points each affect number at its handler. The
// original stores overlay thunk addresses; here the slots get the Go methods.
// Unlisted numbers keep no handler.
func (e *Engine) setupAffectHandlers() {
	for t, h := range map[affectType]AffectHandler{
		affBless:                 (*Engine).hitBonusUp,
		affCurse:                 (*Engine).hitBonusDown,
		affBonusVsRace4:          (*Engine).bonusVsRace4,
		affStudyingManual:        (*Engine).startsToTrain,
		affBonusByTargetRace:     (*Engine).bonusByTargetRace,
		affTrainingManual:        (*Engine).isHardier,
		affProtEvil:              (*Engine).protFromEvil,
		affProtGood:              (*Engine).protFromGood,
		affResistCold:            (*Engine).resistColdHalf,
		affCharmed:               (*Engine).charmSide,
		affEnlarged:              (*Engine).bestStrength,
		affFriends:               (*Engine).setCharisma,
		affPoisonDamage:          (*Engine).poisonDamage,
		affShield:                (*Engine).armourAtLeast,
		affHitBonusVsNames:       (*Engine).hitBonusVsNames,
		affResistFire:            (*Engine).resistFireHalf,
		affSilenced:              (*Engine).isSilenced,
		affSlowPoison:            (*Engine).poisonEnds,
		affSpiritualHammer:       (*Engine).spiritualHammerItem,
		affInvisible:             (*Engine).blindedToHit,
		affHitBonusVsTargetNames: (*Engine).hitBonusVsTargetNames,
		affMirrorImage:           (*Engine).mirrorImage,
		affEnfeebled:             (*Engine).damageLessQuarter,
		affCoughing:              (*Engine).coughingEnds,
		affHelpless:              (*Engine).clearCombatBlock,
		affAnimated:              (*Engine).collapses,
		affBlind:                 (*Engine).minus4All,
		affCauseDisease:          (*Engine).weakAndSick,
		affPrayerBonus:           (*Engine).plusOneSaveHit,
		affBestowCurse:           (*Engine).minus4HitSave,
		affBlink:                 (*Engine).alwaysMissDelayed,
		affStrength:              (*Engine).bestStrength,
		affHaste:                 (*Engine).agesOnce,
		affStinkingCloud:         (*Engine).clearStinkingCloud,
		affProtMissiles:          (*Engine).avoidUnmagicWeapon,
		affSlow:                  (*Engine).halveAttacks,
		affWeakened:              (*Engine).isWeakened,
		affSick:                  (*Engine).sickDamage,
		affProtEvilRadius:        (*Engine).protFromEvil,
		affProtGoodRadius:        (*Engine).protFromGood,
		affMinus4VsNames:         (*Engine).minus4VsNames,
		affMinus4VsBugbearGnoll:  (*Engine).minus4VsBugbearGnoll,
		affPrayer:                (*Engine).prayerSide,
		affMummyDisease:          (*Engine).healOverTime,
		affSnakeCharm:            (*Engine).clearCombatBlock,
		affHeld:                  (*Engine).clearCombatBlock,
		affAsleep:                (*Engine).clearCombatBlock,
		affTurnInvisible:         (*Engine).turnInvisible,
		affRotting:               (*Engine).rots,
		affHeldFast:              (*Engine).heldNoMoves,
		affRegenerating:          (*Engine).addAffect62,
		affFireResistance:        (*Engine).resistFirePerDie,
		affRegenerate:            (*Engine).regenerate,
		affPoisonBite:            (*Engine).poisonBite,
		affPoisonBite4:           (*Engine).poisonBite4,
		affPoisonBite2:           (*Engine).poisonBite2,
		affParalyse2d8:           (*Engine).paralyse2d8,
		affParalyseNotElf:        (*Engine).paralyseNotElf,
		affParalyse1d9:           (*Engine).paralyse1d9,
		affPoisonBiteMinus2:      (*Engine).poisonBiteMinus2,
		affMonsterInvisible:      (*Engine).missMinus4,
		affCamouflaged:           (*Engine).mostlyInvisible,
		affRake:                  (*Engine).rake,
		affStrikeTied:            (*Engine).strikeTied,
		affBloodDrain:            (*Engine).bloodDrain,
		affHoldTarget:            (*Engine).holdTarget,
		affHoldWeakTarget:        (*Engine).holdWeakTarget,
		affGetUp:                 (*Engine).getUpOrWait,
		affMagicDamage2d10:       (*Engine).magicDamage2d10,
		affAcidDamage:            (*Engine).acidDamage1d4,
		affFear:                  (*Engine).fear,
		affFearLifted:            (*Engine).fearLifted,
		affStoneGaze:             (*Engine).stoneGaze,
		affCharmGaze:             (*Engine).charmGaze,
		affDrainOneLevel:         (*Engine).drainOneLevel,
		affDrainTwoLevels:        (*Engine).drainTwoLevels,
		affDisease:               (*Engine).disease,
		affDisplaced:             (*Engine).firstHitMark,
		affSaveBonusVsType0:      (*Engine).saveBonusVsType0,
		affResistFireElectric:    (*Engine).resistFireElectric,
		affHalveFire:             (*Engine).halveFire,
		affHalveVsWeaponFlags:    (*Engine).halveVsWeaponFlags,
		affFallsDead:             (*Engine).fallsDead,
		affOnlyMagicWeapons:      (*Engine).onlyMagicWeapons,
		affSaveBonusVsType42:     (*Engine).saveBonusVsType42,
		affHeal3:                 (*Engine).heal3,
		affStandUpBleeding:       (*Engine).standUpBleeding,
		affRegenUnlessFireAcid:   (*Engine).regenUnlessFireAcid,
		affStartRegenerating:     (*Engine).addAffect3B,
		affReviveFull:            (*Engine).reviveFull,
		affGaseousEscape:         (*Engine).gaseousEscape,
		affAvoidDistant:          (*Engine).avoidDistant3C,
		affResist50:              (*Engine).resist50,
		affResist15:              (*Engine).resist15,
		affBlock90:               (*Engine).block90,
		affBlockSleepCharm:       (*Engine).blockSleepCharm,
		affBlockHold:             (*Engine).blockHeld,
		affBlockCold:             (*Engine).blockCold,
		affBlockPoisonHold:       (*Engine).blockPoisonHold,
		affBlockFire:             (*Engine).blockFire,
		affResistFireLessPerDie:  (*Engine).resistFireLessPerDie,
		affHalveElectric:         (*Engine).halveElectric,
		affHalveUnlessPlain:      (*Engine).halveUnlessPlainWeapon,
		affHalveVsMagicWeapon:    (*Engine).halveVsMagicWeapon,
		affHolyWaterDamage:       (*Engine).holyWaterDamage,
		affHalveCold:             (*Engine).halveCold,
		affOnlyMagicOrStrong:     (*Engine).onlyMagicOrStrong,
		affDodgeBoulders:         (*Engine).dodgeBoulders,
		affSpitAcid:              (*Engine).spitAcid,
		affOilDamage:             (*Engine).oilDamage,
		affHalveOrCancelUnmagic:  (*Engine).halveOrCancelUnmagic,
		affBlock30:               (*Engine).block30,
		affBlockMind:             (*Engine).blockMind,
		affGazeReflected:         (*Engine).gazeReflected,
		affGiantStrengthGirdle:   (*Engine).giantStrengthGirdle,
		affCursedAlignment:       (*Engine).cursedAlignment,
		affNeedsGiantStrength:    (*Engine).needsGiantStrength,
		affEndsSpiritualHammer:   (*Engine).removeEnds17,
	} {
		affectHandlers[t] = h
	}
	// Affects that only mark the character (the original's empty handlers).
	for _, t := range []affectType{affDetectMagic, affReduced, affReadMagic, affFindTraps,
		affDetectInvisible, affFeatherFall, affRepulsed, affPoisoned, affResistNormalWeapons, affMinorGlobe, affUnknown5C, affReflectableGaze} {
		affectHandlers[t] = (*Engine).noEffect
	}
	// Items that grant the affect named in their record (+0x3D).
	for _, t := range []affectType{affItem0, affItem1, affItem2, affItem5, affItem6, affItem8, affItemA, affItemB} {
		affectHandlers[t] = (*Engine).itemGrantsAffect
	}
}

// itemBaseFlags is DS:5589, the base record's Material, for the item's type.
func (e *Engine) itemBaseFlags(it *Item) byte { return e.base(it.Type).Material }

// affectItem is the item standing in for the affect record (the original
// reads the item's bytes through the affect pointer).
func affectItem(a *Affect) *Item {
	if a.Item != nil {
		return a.Item
	}
	return &Item{}
}

// halveDamage is the shared "damage = damage div 2" of the resistance handlers.
func (e *Engine) halveDamage() { e.rv.damage >>= 1 }

// stoneGaze = ovr13:1CAD (c, a, flag): a gaze that turns the target to stone; an item with
// 0x76 on a readied piece reflects it back. DS:6826/6828 (the target pointer
// the original keeps in globals) is a local here.
func (e *Engine) stoneGaze(c *Character, a *Affect, flag byte) {
	t := c.Combat.Target
	e.CharMessage(c, "gazes...", false)
	e.hitColours(0x12)
	e.flyMissile(e.combatX(c), e.combatY(c), e.combatX(t), e.combatY(t), 4, 0x2D)
	if e.FindAffect(c, affReflectableGaze) != nil {
		reflected := false
		for _, p := range e.carried(t) {
			if reflected {
				break
			}
			if (p.Readied != 0 || dollSlot(e, t, p) < 0) && (p.Words[0] == 0x76 || p.Words[1] == 0x76 || p.Words[2] == 0x76) {
				e.CharMessage(t, "reflects it!", false)
				e.flyMissile(e.combatX(t), e.combatY(t), e.combatX(c), e.combatY(c), 4, 0x2D)
				reflected = true
				t = c
			}
		}
	}
	if !e.SavingThrow(t, saveVsPetrify, 0) {
		e.killWithMessage(t, statusStoned, "is Stoned")
	}
}

// charmGaze = ovr13:1E77 (c, a, flag): charm gaze; needs ovr14:1085 and a line of sight.
func (e *Engine) charmGaze(c *Character, a *Affect, flag byte) {
	t := c.Combat.Target
	tx, ty := e.combatX(t), e.combatY(t)
	if !e.mayActAgainst(c, t) {
		return
	}
	rng := 0 // [bp-2] is never set in the original
	if !e.lineOfSight(&rng, &tx, &ty, e.combatX(c), e.combatY(c)) {
		return
	}
	e.CharMessage(c, "Gazes...", false)
	e.hitColours(0x12)
	e.flyMissile(e.combatX(c), e.combatY(c), e.combatX(t), e.combatY(t), 4, 0x2D)
	e.curSpell = spCharmPerson
	data := byte(int(int8(c.Side))<<7 + 0xC)
	saved := e.SavingThrow(t, saveVsSpell, 0xFE)
	e.AddAffectMsg(t, affCharmed, 0, data, 1, 1, saved, "is charmed")
	if x := e.FindAffect(t, affCharmed); x != nil {
		e.callAffect(affCharmed, t, x, 0)
	}
}

// drainLevels = ovr13:2122 (char, n): n levels are drained; hit points,
// class levels and experience follow. Its nested helper at 2002 is bestClass.
func (e *Engine) drainLevels(c *Character, n int) {
	var best uint32 // parent local [bp-0x31], also read after bestClass runs
	// bestClass = ovr13:2002: the class whose level has the highest experience
	// entry (DS:409B, 14 longs per class); sets best to that experience.
	bestClass := func() int {
		best = 0
		idx, maxLvl := 0, int8(0)
		for i := 0; i < 8; i++ {
			lvl := int8(c.Levels[i])
			var cur uint32
			if lvl > 1 {
				cur = uint32(xpTable[i][lvl])
			}
			if lvl >= maxLvl && (int32(cur) >= int32(best) || cur == 0xFFFFFFFF) {
				best, idx, maxLvl = cur, i, lvl
			}
		}
		return idx
	}
	msg := "a level"
	if n > 1 {
		msg = cut(strconv.Itoa(n)+" levels", textCols)
	}
	e.CharMessage(c, "is drained "+msg, true)
	if c.Status == statusAnimated {
		e.killWithMessage(c, statusGone, "is dead forever")
		c.MaxHP = 0
		return
	}
	for lv := 1; lv <= n; lv++ {
		if c.Status == statusGone {
			continue
		}
		c.Drained++
		var sum byte
		for i := 0; i < 8; i++ {
			sum += c.Levels[i]
		}
		per := 0
		if sum != 0 { // the original divides by zero (runtime error) here
			per = int(c.MaxHP) / int(sum)
		}
		e.debugValues("max,cur,lost", int(c.MaxHP), int(c.HP), int(c.DrainedHP))
		for k := 1; k <= per; k++ {
			if c.MaxHP > 0 {
				c.MaxHP--
			}
			if c.HPRoll > 0 {
				c.HPRoll--
			}
			if c.HP > 0 {
				c.HP--
			}
			c.DrainedHP++
		}
		if int8(c.MaxLevel) < 2 {
			e.killWithMessage(c, statusGone, "is dead forever")
			c.MaxHP = 0
			if i := bestClass(); int8(c.Levels[i]) > 0 && n == 2 {
				c.Levels[bestClass()] = 0
			}
			// jumps over RecomputeCharacter
		} else {
			c.Levels[bestClass()]--
			if c.HP == 0 && c.Status != statusDead {
				e.killWithMessage(c, statusDead, "is killed")
			}
			e.RecomputeCharacter(c)
		}
		i := bestClass()
		c.MaxLevel = c.Levels[i]
		c.Exp = uint32(best)
		e.ApplyItems(c)
	}
}

// drainOneLevel = ovr13:23A4 (c, a, flag): drains one level from the gaze target.
func (e *Engine) drainOneLevel(c *Character, a *Affect, flag byte) { e.drainLevels(c.Combat.Target, 1) }

// drainTwoLevels = ovr13:23C2 (c, a, flag): drains two levels from the gaze target.
func (e *Engine) drainTwoLevels(c *Character, a *Affect, flag byte) {
	e.drainLevels(c.Combat.Target, 2)
}

// disease = ovr13:23EC (c, a, flag): disease; the target stops healing (0x32) and gets
// the wasting affect 0x39.
func (e *Engine) disease(c *Character, a *Affect, flag byte) {
	t := c.Combat.Target
	if e.FindAffect(t, affMummyDisease) == nil {
		e.AddAffect(t, affMummyDisease, 0x3840, 0xFF, 1)
		e.AddAffect(t, affRotting, 0xA8C0, byte(e.Dice(1, 6)<<4+0xF), 1)
		e.CharMessage(t, "is Diseased", true)
	}
}

// firstHitMark = ovr13:2490 (c, a, flag): marks the affect (bit 4 of its data) the first time
// it is rolled with a hit roll set.
func (e *Engine) firstHitMark(c *Character, a *Affect, flag byte) {
	if e.combatRound == 0 && e.rv.hitRoll == 0 {
		a.Data &= 0x0F
	} else if a.Data&0x10 == 0 {
		e.rv.hitRoll = -1
		a.Data |= 0x10
	}
}

// levelBonus is the 1..5 step of the level ranges 4-6, 7-10, 11-13, 14-17,
// 18-20 used by the save handlers (0 otherwise; the original leaves its
// local uninitialised there).
func levelBonus(level byte) byte {
	switch {
	case level >= 4 && level <= 6:
		return 1
	case level >= 7 && level <= 10:
		return 2
	case level >= 11 && level <= 13:
		return 3
	case level >= 14 && level <= 17:
		return 4
	case level >= 18 && level <= 20:
		return 5
	}
	return 0
}

// saveBonusVsType0 = ovr13:24D9 (c, a, flag): saving throw bonus by level for save type 0.
func (e *Engine) saveBonusVsType0(c *Character, a *Affect, flag byte) {
	if e.rv.saveType == saveVsParalysis {
		e.rv.save += levelBonus(c.Stats[statCon])
	}
}

// resistFireElectric = ovr13:2556 (c, a, flag): fully resisted by Magic Missile, fire/electric (4)
// otherwise only drops the affect being added.
func (e *Engine) resistFireElectric(c *Character, a *Affect, flag byte) {
	if e.curSpell == spMagicMissile {
		e.blockAffect(affAny)
		e.CharMessage(c, "is unaffected", true)
	} else if e.rv.dmgType&dmgElectric != 0 {
		e.blockAffect(affAny)
	}
}

// halveFire = ovr13:25A2 (c, a, flag): fire damage (bit 0) is halved.
func (e *Engine) halveFire(c *Character, a *Affect, flag byte) {
	if e.rv.dmgType&dmgFire != 0 {
		e.halveDamage()
	}
}

// halveVsWeaponFlags = ovr13:25BC (c, a, flag): halves damage when the selected character's weapon
// has base flags 0x81.
func (e *Engine) halveVsWeaponFlags(c *Character, a *Affect, flag byte) {
	if w := e.readiedWeapon(e.selected()); w != nil && e.itemBaseFlags(w)&0x81 != 0 {
		e.halveDamage()
	}
}

// fallsDead = ovr13:2610 (c, a, flag): the affect ends; a living character falls dead.
func (e *Engine) fallsDead(c *Character, a *Affect, flag byte) {
	a.Call = 0
	if c.Okay != 0 {
		e.killWithMessage(c, statusDead, "Falls dead")
	}
}

// onlyMagicWeapons = ovr13:264B (c, a, flag): damage is cancelled unless the weapon is magical
// (+0x32 non-zero, or 0xB1 in +0x31).
func (e *Engine) onlyMagicWeapons(c *Character, a *Affect, flag byte) {
	w := e.readiedWeapon(e.selected())
	if w == nil || w.Plus == 0 && w.Words[2] != 0xB1 {
		e.rv.damage = 0
	}
}

// saveBonusVsType42 = ovr13:268A (c, a, flag): like 24D9 for save types 4 and 2.
func (e *Engine) saveBonusVsType42(c *Character, a *Affect, flag byte) {
	if e.rv.saveType == saveVsSpell || e.rv.saveType == saveVsWand {
		e.rv.save += levelBonus(c.Stats[statCon])
	}
}

// heal3 = ovr13:2700 (c, a, flag): +3 hit points up to the maximum.
func (e *Engine) heal3(c *Character, a *Affect, flag byte) {
	c.HP += 3
	if c.HP > c.MaxHP {
		c.HP = c.MaxHP
	}
}

// standUpBleeding = ovr13:2730 (c, a, flag): a dying or unconscious character gets back up with
// the hit points it had left to bleed (6 minus the rounds), then 0x5F runs
// for 1d4+1 and this affect (0x63) is removed.
func (e *Engine) standUpBleeding(c *Character, a *Affect, flag byte) {
	var n byte
	if c.Status == statusDying && c.Combat.Dying < 6 {
		n = 6 - c.Combat.Dying
	}
	if c.Status == statusUnconscious {
		n = 6
	}
	if n > 0 && e.Revive(c, n) {
		e.AddAffect(c, affFallsDead, e.Dice(1, 4)+1, 0xFF, 1)
		a.Call = 0
		e.RemoveAffect(c, affStandUpBleeding, a)
	}
}

// regenUnlessFireAcid = ovr13:27D9 (c, a, flag): damage that is neither fire (1) nor acid (0x10)
// starts affect 0x66 for 3d6.
func (e *Engine) regenUnlessFireAcid(c *Character, a *Affect, flag byte) {
	if e.rv.dmgType&dmgFire == 0 && e.rv.dmgType&dmgAcid == 0 {
		e.AddAffect(c, affReviveFull, e.Dice(3, 6), 0xFF, 1)
	}
}

// addAffect3B = ovr13:2814 (c, a, flag): adds 0x3B for 3 rounds unless 0x62 or 0x3B is there.
func (e *Engine) addAffect3B(c *Character, a *Affect, flag byte) {
	if e.FindAffect(c, affHeal3) == nil && e.FindAffect(c, affRegenerating) == nil {
		e.AddAffect(c, affRegenerating, 3, 0xFF, 1)
	}
}

// reviveFull = ovr13:2866 (c, a, flag): back up at full hit points, else re-add 0x66.
func (e *Engine) reviveFull(c *Character, a *Affect, flag byte) {
	if !e.Revive(c, c.MaxHP) {
		e.addAffectNow(c, affReviveFull, a.Data, 1)
	}
}

// gaseousEscape = ovr13:28BF (c, a, flag): turns gaseous and leaves the fight.
func (e *Engine) gaseousEscape(c *Character, a *Affect, flag byte) {
	a.Call = 0
	e.combatRedrawChar(c, 3, 0)
	e.KillCharacter(c, statusRunning, "Turns gaseous and escapes")
}

// avoidDistant3C = ovr13:2901 (c, a, flag).
func (e *Engine) avoidDistant3C(c *Character, a *Affect, flag byte) { e.avoidDistantAttack(c, 0x3C) }

// resistRoll = ovr13:2915 (base): the chance base - 5*(11 - caster level)
// percent (as a byte) to drop the affect being added.
func (e *Engine) resistRoll(base byte) {
	lvl := e.casterLevel(int(e.curSpell)) & 0xFF
	thr := byte(int(base) - 5*(0xB-lvl))
	if e.rv.affVal != 0 || e.rv.dmgType&dmgMagic != 0 {
		if byte(e.Dice(1, 100)) <= thr {
			e.blockAffect(affAny)
		}
	}
}

// resist50 = ovr13:2976 (c, a, flag).
func (e *Engine) resist50(c *Character, a *Affect, flag byte) { e.resistRoll(0x32) }

// resist15 = ovr13:2984 (c, a, flag).
func (e *Engine) resist15(c *Character, a *Affect, flag byte) { e.resistRoll(0xF) }

// block90 = ovr13:2992 (c, a, flag): 90%: blocks 0x35 and 0xB.
func (e *Engine) block90(c *Character, a *Affect, flag byte) {
	if e.Dice(1, 100) <= 0x5A {
		e.blockAffect(affAsleep)
		e.blockAffect(affCharmed)
	}
}

// blockSleepCharm = ovr13:29B6 (c, a, flag): blocks 0xB and 0x35.
func (e *Engine) blockSleepCharm(c *Character, a *Affect, flag byte) {
	e.blockAffect(affCharmed)
	e.blockAffect(affAsleep)
}

// blockHeld = ovr13:29CB (c, a, flag): blocks Hold Person.
func (e *Engine) blockHeld(c *Character, a *Affect, flag byte) { e.blockAffect(affHeld) }

// blockCold = ovr13:29D9 (c, a, flag): cold damage (bit 1) is blocked.
func (e *Engine) blockCold(c *Character, a *Affect, flag byte) {
	if e.rv.dmgType&dmgCold != 0 {
		e.blockAffect(affAny)
	}
}

// blockPoisonHold = ovr13:29F0 (c, a, flag): blocks poison and Hold Person; saves of type 0 always pass.
func (e *Engine) blockPoisonHold(c *Character, a *Affect, flag byte) {
	e.blockAffect(affPoisoned)
	e.blockAffect(affHeld)
	if e.rv.saveType == saveVsParalysis {
		e.rv.save = 0x64
	}
}

// blockFire = ovr13:2A11 (c, a, flag): fire damage (bit 0) is blocked.
func (e *Engine) blockFire(c *Character, a *Affect, flag byte) {
	if e.rv.dmgType&dmgFire != 0 {
		e.blockAffect(affAny)
	}
}

// resistFireLessPerDie = ovr13:2A28 (c, a, flag): fire damage loses 1 per die, never below one per die.
func (e *Engine) resistFireLessPerDie(c *Character, a *Affect, flag byte) {
	if e.rv.dmgType&dmgFire != 0 {
		for k := byte(1); k <= e.rv.diceN; k++ {
			e.rv.damage--
			if e.rv.damage < e.rv.diceN {
				e.rv.damage = e.rv.diceN
			}
		}
	}
}

// halveElectric = ovr13:2A6D (c, a, flag): electricity damage (bit 2) is halved.
func (e *Engine) halveElectric(c *Character, a *Affect, flag byte) {
	if e.rv.dmgType&dmgElectric != 0 {
		e.halveDamage()
	}
}

// halveUnlessPlainWeapon = ovr13:2A87 (c, a, flag): halves damage unless the weapon is a plain
// (flags non-zero, bit 0 clear) base item.
func (e *Engine) halveUnlessPlainWeapon(c *Character, a *Affect, flag byte) {
	if w := e.readiedWeapon(e.selected()); w != nil {
		if f := e.itemBaseFlags(w); f == 0 || f&1 != 0 {
			e.halveDamage()
		}
	}
}

// halveVsMagicWeapon = ovr13:2AE6 (c, a, flag): halves damage when the weapon's +0x32 is positive.
func (e *Engine) halveVsMagicWeapon(c *Character, a *Affect, flag byte) {
	if w := e.readiedWeapon(e.selected()); w != nil && int8(w.Plus) > 0 {
		e.halveDamage()
	}
}

// holyWaterDamage = ovr13:2B20 (c, a, flag): holy water readied by the
// selected character makes the damage 1d6+1.
func (e *Engine) holyWaterDamage(c *Character, a *Affect, flag byte) {
	if w := e.selected().Slots[slotWeapon]; w != nil && w.Type == itemHolyWater {
		e.rv.damage = byte(e.RollDice(1, 6) + 1)
	}
}

// halveCold = ovr13:2B63 (c, a, flag): cold damage (bit 1) is halved.
func (e *Engine) halveCold(c *Character, a *Affect, flag byte) {
	if e.rv.dmgType&dmgCold != 0 {
		e.halveDamage()
	}
}

// onlyMagicOrStrong = ovr13:2B7D (c, a, flag): damage is cancelled unless the weapon has +0x32
// set, or the attacker is a monster (race 0) of level (+0x73) 4 or more.
func (e *Engine) onlyMagicOrStrong(c *Character, a *Affect, flag byte) {
	if w := e.readiedWeapon(e.selected()); w != nil && w.Plus != 0 {
		return
	}
	s := e.selected()
	if int8(s.Race) > raceMonster || int8(s.MaxLevel) < 4 {
		e.rv.damage = 0
	}
}

// dodgeBoulders = ovr13:2BC8 (c, a, flag): a boulder or giant boulder thrown
// by the selected character may be avoided (50%) from afar.
func (e *Engine) dodgeBoulders(c *Character, a *Affect, flag byte) {
	if w := e.selected().Slots[slotWeapon]; w != nil && (w.Type == itemBoulder || w.Type == itemGiantBoulder) {
		e.avoidDistantAttack(c, 0x32)
	}
}

// spitAcid = ovr13:2C1C (c, a, flag): spits acid (25%) at a target under 4 squares away:
// 8d4 damage, half on a save; the affects 0x79 and 0x50 end.
func (e *Engine) spitAcid(c *Character, a *Affect, flag byte) {
	t := c.Combat.Target
	if e.Dice(1, 100) > 0x19 || e.rangeTo(c, t) >= 4 {
		return
	}
	e.clearCombatInfo(c)
	e.CharMessage(c, "Spits Acid", true)
	e.hitColours(0x17)
	e.flyMissile(e.combatX(c), e.combatY(c), e.combatX(t), e.combatY(t), 1, 0x1E)
	dmg := e.RollDice(8, 4)
	saved := e.SavingThrow(t, saveVsBreath, 0)
	e.Damage(t, dmg, saveHalf, saved)
	e.RemoveAffect(c, affSpitAcid, a)
	e.RemoveAffect(c, affAcidDamage, nil)
}

// oilDamage = ovr13:2D38 (c, a, flag): oil readied by the selected character
// makes the damage 3d8; fire or magic types (mask 9) add the dice count.
func (e *Engine) oilDamage(c *Character, a *Affect, flag byte) {
	if w := e.selected().Slots[slotWeapon]; w != nil && w.Type == itemOil {
		e.rv.damage = byte(e.RollDice(3, 8))
	}
	if e.rv.dmgType&(dmgFire|dmgMagic) != 0 {
		e.rv.damage += e.rv.diceN
	}
}

// halveOrCancelUnmagic = ovr13:2D88 (c, a, flag): halves damage with a +0x32 = 0, +0x31 = 0xB1
// weapon, cancels it for another +0x32 = 0 weapon or no weapon.
func (e *Engine) halveOrCancelUnmagic(c *Character, a *Affect, flag byte) {
	w := e.readiedWeapon(e.selected())
	switch {
	case w == nil:
		e.rv.damage = 0
	case w.Plus == 0 && w.Words[2] == 0xB1:
		e.halveDamage()
	case w.Plus == 0:
		e.rv.damage = 0
	}
}

// block30 = ovr13:2DE4 (c, a, flag): 30%: blocks 0xB and 0x35.
func (e *Engine) block30(c *Character, a *Affect, flag byte) {
	if e.Dice(1, 100) <= 0x1E {
		e.blockAffect(affCharmed)
		e.blockAffect(affAsleep)
	}
}

// blockMind = ovr13:2E08 (c, a, flag): blocks 0xB, 0x35, 0x34 and 0x37; saves of type 0 pass.
func (e *Engine) blockMind(c *Character, a *Affect, flag byte) {
	e.blockAffect(affCharmed)
	e.blockAffect(affAsleep)
	e.blockAffect(affHeld)
	e.blockAffect(affPoisoned)
	if e.rv.saveType == saveVsParalysis {
		e.rv.save = 0x64
	}
}

// gazeReflected = ovr13:2E37 (c, a, flag): sets DS:681E when the gaze target carries a ring
// (0x98 with 0xFC) or 0x76 item in slots 1..3 of a used item.
func (e *Engine) gazeReflected(c *Character, a *Affect, flag byte) {
	t := c.Combat.Target
	for _, p := range e.carried(t) { // extra: a mirror works from the party inventory (it has no slot)
		if e.affectVeto != 0 {
			break
		}
		if p.Readied != 0 || dollSlot(e, t, p) < 0 {
			for k := 1; k <= 3; k++ {
				if p.Words[k-1] == 0x98 && p.Words[2] == 0xFC || p.Words[k-1] == 0x76 {
					e.affectVeto = 1
				}
			}
		}
	}
}

// itemGrantsAffect = ovr13:2EDA (c, a, flag): an item stands in for a; its +0x3D is the affect it
// grants (added on use with call 0, then run once; removed with flag set).
func (e *Engine) itemGrantsAffect(c *Character, a *Affect, flag byte) {
	typ := affectType(affectItem(a).Spells[1])
	if flag != 0 {
		e.RemoveAffect(c, typ, nil)
		return
	}
	e.AddAffect(c, typ, 0, 0xC, 0)
	e.callAffect(typ, c, nil, 0)
}

// giantStrengthGirdle = ovr13:2F57 (c, a, flag): girdle of giant strength (affect 0x26). Removed
// (flag set): the affect goes unless the character already has 18/00 and
// the stored value cannot be told apart; added: strength 18/00.
func (e *Engine) giantStrengthGirdle(c *Character, a *Affect, flag byte) {
	if flag != 0 {
		found := false
		giant := c.Stats[statStr] == 0x12 && c.Str00 == 0x64
		for _, p := range append([]*Affect(nil), c.Affects...) {
			if found {
				break
			}
			if p.Type == affStrength {
				str, str00 := strDecode(p)
				// p.Raw[0] is the type byte (the original compares the first byte, not the data)
				if giant && p.Type < 0x80 || str == 0x12 && str00 == 0x64 && !giant {
					e.RemoveAffect(c, affStrength, p)
					found = true
				}
			}
		}
		return
	}
	out, ok := e.SetStrength(c, 0x12, 0x64)
	if ok {
		if e.beginFlag == 5 {
			e.beginFlag = 2
			e.CharMessage(c, "is stronger", true)
			e.beginFlag = 5
		} else {
			e.CharMessage(c, "is stronger", true)
		}
	}
	e.itemsRedraw = true // DS:6821
	e.AddAffect(c, affStrength, 0, out, 1)
}

// cursedAlignment = ovr13:30A5 (c, a, flag): cursed item (a stands in as the item): with the
// wrong alignment (+0x3D low nibble vs char +0xA0) it is unreadied and
// hurts by its high nibble.
func (e *Engine) cursedAlignment(c *Character, a *Affect, flag byte) {
	if flag != 0 {
		return
	}
	it := affectItem(a)
	if it.Spells[1]&0xF != c.Alignment {
		it.Readied = 0
		n := int(it.Spells[1] >> 4)
		e.rv.dmgType = dmgMagic
		if e.beginFlag == 5 {
			e.combatUseHook()
		}
		e.Damage(c, n, 0, false)
		e.itemsRedraw = true
	}
}

// needsGiantStrength = ovr13:3134 (c, a, flag): the item needs strength 19 or more.
func (e *Engine) needsGiantStrength(c *Character, a *Affect, flag byte) {
	if flag == 0 && c.Stats[statStr] < 0x13 {
		affectItem(a).Readied = 0
		e.refuse("Must have Giant Strength")
	}
}

// removeEnds17 = ovr13:317A (c, a, flag): removing it ends affect 0x17.
func (e *Engine) removeEnds17(c *Character, a *Affect, flag byte) {
	if flag != 0 {
		e.RemoveAffect(c, affSpiritualHammer, nil)
	}
}

// noEffect = ovr13:319B (c, a, flag): empty handler.
func (e *Engine) noEffect(c *Character, a *Affect, flag byte) {}

// debugValues = ovr34:0004 (msg, a, b, c): with DS:49F7 set, show the three
// numbers on the top row and msg on row 24.
func (e *Engine) debugValues(msg string, a, b, c int) {
	if len(msg) > 0x28 {
		msg = msg[:0x28]
	}
	if e.debugShow == 0 {
		return
	}
	e.ClearBox(0, 0, lastCol, 0)
	e.DisplayString(0, 0, colYellow, fmt.Sprintf("%d , %d , %d", a, b, c))
	e.PromptRow24(colYellow, msg)
	e.ClearBox(0, 0, lastCol, 0)
}
