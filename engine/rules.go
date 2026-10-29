package engine

// Rules: affects, to-hit rolls, saving throws, damage and healing, ported
// from ovr25 and the START helpers it leans on (00BA:16EC ... 2637).

import (
	"slices"
	"strconv"
)

// CombatInfo is the 0x16-byte block at char+0x108 (allocated by ovr11).
type CombatInfo struct {
	Spell      byte       // +0x00 spell being cast (lost when hit)
	CanCast    byte       // +0x01
	CanUse     byte       // +0x02 may use an item
	Delay      byte       // +0x03 signed initiative; 0x14 = delayed
	AttackSlot byte       // +0x04 attack slot in use (1, 2)
	Sweeps     byte       // +0x05 sweeps left against weak foes
	Moves      byte       // +0x06 half squares left
	Guarding   byte       // +0x07
	Attacked   byte       // +0x08 has attacked this turn
	Facing     byte       // +0x09 0..7
	Target     *Character // +0x0A
	Dying      byte       // +0x0E rounds of bleeding left
	Attackers  byte       // +0x0F times attacked this round
	Turned     byte       // +0x10 turned by a cleric
	TurnUsed   byte       // +0x11 has tried to turn undead
	Flank      byte       // +0x12 accumulated turn from attackers
	Extra      byte       // +0x13 not one of the saved party
	Fleeing    byte       // +0x14
	Pattern    byte       // +0x15 AI move pattern, row of DS:02AE
}

// Rule globals (DS:6816..682A).
type ruleVars struct {
	save     byte       // DS:6816 saving throw total
	affVal   affectType // DS:6817 affect being added (0 = blocked)
	damage   byte       // DS:6818
	dmgType  byte       // DS:6819 1 fire 2 cold 4 electricity 8 magic 0x10 acid
	diceN    byte       // DS:681C
	hitRoll  int8       // DS:6822
	saveType byte       // DS:682A
	cure     byte       // DS:681F set while the temple removes affects
	side     [2]int8    // DS:6814 standing members per side
}

// AffectHandler is one entry of the DS:6828 table (filled by ovr13:31A2):
// handler(char, affect, flag); flag 1 = the affect is being removed.
type AffectHandler func(e *Engine, c *Character, a *Affect, flag byte)

var affectHandlers [0x8C]AffectHandler

// callAffect = ovr25:0000 (type, char, affect, flag).
func (e *Engine) callAffect(typ affectType, c *Character, a *Affect, flag byte) {
	if int(typ) < len(affectHandlers) && affectHandlers[typ] != nil {
		affectHandlers[typ](e, c, a, flag)
	}
}

// FindAffect = 00BA:2199 (out, type, char): first affect of the type.
func (e *Engine) FindAffect(c *Character, typ affectType) *Affect {
	for _, a := range c.Affects {
		if a.Type == typ {
			return a
		}
	}
	return nil
}

// RemoveAffect = ovr25:0026 (char, type, affect): affect nil means the first
// one of the type. Its handler runs first when the node asks for it.
func (e *Engine) RemoveAffect(c *Character, typ affectType, a *Affect) {
	if a == nil {
		a = e.FindAffect(c, typ)
	}
	if a == nil {
		return
	}
	if a.Call != 0 {
		e.callAffect(typ, c, a, 1)
	}
	for i, x := range c.Affects {
		if x == a {
			c.Affects = append(c.Affects[:i], c.Affects[i+1:]...)
			return
		}
	}
}

// AddAffect = ovr25:0E52 (char, type, time, data, call): append a node.
func (e *Engine) AddAffect(c *Character, typ affectType, time int, data, call byte) {
	a := &Affect{affectRecord: affectRecord{Type: typ, Time: uint16(time), Data: data, Call: call}}
	c.Affects = append(c.Affects, a)
}

// auraAffects = ovr25 cs:0140: affects that also work on everyone near the
// one who has them (prayer, chant ...).
func auraAffect(t affectType) bool {
	return t == affSilenced || t == affProtEvilRadius || t == affProtGoodRadius || t == affPrayer
}

// checkAffect = ovr25:0160 (char, type): run the handler (flag 0) when the
// character has the affect, or for an aura affect when anyone in range
// (anyone at all outside combat) has it.
func (e *Engine) checkAffect(c *Character, typ affectType) {
	a := e.FindAffect(c, typ)
	found := a != nil
	if !found && auraAffect(typ) {
		saved := e.nearList // DS:671B, 0xD8 bytes
		for _, p := range e.party {
			if found {
				break
			}
			pa := e.FindAffect(p, typ)
			if pa == nil {
				continue
			}
			a = pa
			if e.beginFlag != 5 {
				found = true
				continue
			}
			r := 1
			if typ == affPrayer {
				r = 6
			}
			e.nearCombatants(p, e.combatX(p), e.combatY(p), r)
			me := e.combatIndex(c)
			for _, n := range e.near {
				if int(n.index) == me {
					found = true
				}
			}
		}
		e.nearList = saved
	}
	if found {
		e.callAffect(typ, c, a, 0)
	}
}

// affectGroups = the lists of ovr25:02FA: which affects take part in each
// kind of check (the group number is the caller's).
var affectGroups = [][]affectType{
	0x00: {affGazeReflected},
	0x01: {affMinorGlobe, affBlink, affInvisible, affMonsterInvisible},
	0x02: {affDrainOneLevel, affDrainTwoLevels, affDisease, affParalyseNotElf, affMagicDamage2d10, affAcidDamage, affHoldTarget, affRake, affHoldWeakTarget},
	0x03: {affPoisonBite, affPoisonBite4, affPoisonBite2, affParalyse2d8, affParalyseNotElf, affParalyse1d9, affPoisonBiteMinus2, affMagicDamage2d10, affDrainOneLevel, affDrainTwoLevels, affDisease},
	0x04: {affEnfeebled, affBonusVsRace4, affBonusByTargetRace},
	0x05: {affMirrorImage, affProtMissiles, affAvoidDistant, affDodgeBoulders, affStartRegenerating, affHalveUnlessPlain, affHalveVsMagicWeapon, affOnlyMagicOrStrong, affHalveOrCancelUnmagic, affOnlyMagicWeapons, affHalveVsWeaponFlags, affResistNormalWeapons, affOilDamage, affHolyWaterDamage},
	0x06: {affResistFireLessPerDie, affFireResistance, affOilDamage, affResistNormalWeapons, affResistFireElectric, affResistCold, affResistFire, affResist50, affResist15, affBlockFire, affHalveElectric, affHalveCold, affShield, affHalveFire, affStartRegenerating, affMirrorImage},
	0x07: {affSnakeCharm, affHeld, affAsleep, affHelpless},
	0x08: {affStandUpBleeding, affFear, affFearLifted, affDisplaced, affCamouflaged, affTurnInvisible},
	0x09: {affResist50, affResist15, affBlock90, affBlockSleepCharm, affBlockHold, affBlockCold, affBlockPoisonHold, affBlockFire, affBlock30, affBlockMind},
	0x0A: {affBless, affCurse, affBlind, affBestowCurse, affPrayer, affBonusVsRace4, affBonusByTargetRace, affHitBonusVsNames, affHitBonusVsTargetNames},
	0x0B: {affBlind, affShield, affProtEvil, affProtGood, affProtEvilRadius, affProtGoodRadius, affCoughing},
	0x0C: {affProtEvil, affProtGood, affResistCold, affShield, affResistFire, affBlind, affBestowCurse, affProtEvilRadius, affProtGoodRadius, affPrayer, affFireResistance, affBlockPoisonHold, affBlockMind, affSaveBonusVsType0, affSaveBonusVsType42},
	0x0D: {affStandUpBleeding, affRegenUnlessFireAcid, affGaseousEscape, affBloodDrain, affStrikeTied},
	0x0E: {affStoneGaze, affCharmGaze, 0x58, affSpitAcid},
	0x0F: {affSilenced, affCoughing, affStrikeTied, affBloodDrain, affCharmed},
	0x10: {affInvisible, affMonsterInvisible, affBlink, affMinus4VsNames, affMinus4VsBugbearGnoll, affDisplaced},
	0x11: {affBless, affCurse, affCharmed},
	0x12: {affHaste, affSlow, affHeldFast},
	0x13: {affHeal3, affSpiritualHammer, affCamouflaged, affTurnInvisible, affCharmed},
}

// Affect check groups by use.
const (
	grpDamage    = 0x06
	grpAddAffect = 0x09
	grpAttacker  = 0x0A
	grpSave      = 0x0C
	grpDown      = 0x0D
	grpDefender  = 0x10
)

// affectGroup = ovr25:02FA (char, group).
func (e *Engine) affectGroup(c *Character, group int) {
	if group < len(affectGroups) {
		for _, t := range affectGroups[group] {
			e.checkAffect(c, t)
		}
	}
}

// stinkingCloud = ovr25:0AE7 (char): a character standing in a stinking
// cloud coughs (save) or is overcome by nausea.
func (e *Engine) stinkingCloud(c *Character) {
	if c.Okay == 0 {
		return
	}
	_, ground := e.groundAt(c, 8)
	if ground != 0x1E {
		return
	}
	for _, t := range []affectType{affCoughing, affHelpless, affAnimated, affBlockPoisonHold, affBlockMind} {
		if e.FindAffect(c, t) != nil {
			return
		}
	}
	saved := e.SavingThrow(c, saveVsParalysis, 0)
	var t affectType
	if saved {
		t = affCoughing
		e.AddAffectMsg(c, t, 1, 0xFF, 0, 0, saved, "starts to cough")
	} else {
		t = affHelpless
		e.AddAffectMsg(c, t, e.Dice(1, 4)+1, 0xFF, 0, 0, saved, "chokes and gags from nausea")
	}
	// ovr25 passes the out pointer of its last FindAffect (0x7D), which is nil.
	e.callAffect(t, c, nil, 0)
}

// RollHit = ovr25:0C4F (char, bonus): d20 (a natural 20 counts 100) plus
// bonus must beat the defender's armour value at +0x111.
func (e *Engine) RollHit(c *Character, bonus int) bool {
	e.rv.hitRoll = int8(e.Dice(1, 20))
	if e.rv.hitRoll <= 1 {
		return false
	}
	if e.rv.hitRoll == 20 {
		e.rv.hitRoll = 100
	}
	e.affectGroup(c, grpDefender)
	return e.rv.hitRoll >= 0 && int(e.rv.hitRoll)+int(byte(bonus)) > int(c.AC[0])
}

// AttackHits = ovr25:0CB7 (attacker, defender, need): attacking ends
// invisibility; d20 + THAC0 + the side's modifier must reach need.
func (e *Engine) AttackHits(att, def *Character, need int) bool {
	e.removeInvisibility(att)
	e.rv.hitRoll = int8(e.Dice(1, 20))
	if e.rv.hitRoll <= 1 {
		return false
	}
	if e.rv.hitRoll == 20 {
		e.rv.hitRoll = 100
	}
	e.affectGroup(att, grpAttacker)
	e.affectGroup(def, grpDefender)
	adj := int(int8(byte(e.game.PartyHit)))
	if att.Side != sideParty {
		adj = int(int8(byte(e.game.MonsterHit)))
	}
	return e.rv.hitRoll >= 0 && int(e.rv.hitRoll)+int(att.THAC0)+adj >= int(byte(need))
}

// SavingThrow = ovr25:0D63 (char, type, bonus): 1 always fails, 20 always
// saves, otherwise d20 + bonus + the character's save bonus must reach the
// target for the type (0 paralysis ... 4 spells).
func (e *Engine) SavingThrow(c *Character, typ, bonus int) bool {
	e.rv.save = byte(e.Dice(1, 20))
	switch e.rv.save {
	case 1:
		return false
	case 20:
		return true
	}
	e.rv.save = byte(int(e.rv.save) + int(int8(c.SaveBonus)) + int(int8(bonus)))
	e.rv.saveType = byte(typ)
	e.affectGroup(c, grpSave)
	return c.Saves[int(int8(typ))] <= e.rv.save
}

// RollDice = ovr25:0E2E (n, sides): Dice that also remembers n (DS:681C).
func (e *Engine) RollDice(n, sides int) int {
	e.rv.diceN = byte(n)
	return e.Dice(n, sides)
}

// KillCharacter = ovr25:0F06 (char, status, message).
func (e *Engine) KillCharacter(c *Character, status byte, msg string) {
	msg = cut(msg, textCols)
	if c.Okay == 0 {
		return
	}
	idx := e.combatIndex(c)
	e.combatRedrawChar(c, 3, 0)
	e.CharMessage(c, msg, true)
	c.Okay = 0
	c.Status = status
	if c.Status != statusRunning {
		c.HP = 0
	}
	e.combatClearIcon(0, 0, idx)
	e.Flip()
	e.cpos[idx].size = 0 // DS:5F2A + 4*idx
	e.combatRedrawMap()
	if e.clearCombatInfo(c) {
		e.removeCombatAffects(c)
	}
}

// removeInvisibility = ovr25:0FD2.
func (e *Engine) removeInvisibility(c *Character) {
	for a := e.FindAffect(c, affInvisible); a != nil; a = e.FindAffect(c, affInvisible) {
		e.RemoveAffect(c, affInvisible, a)
	}
}

// combatAffects = DS:0C14: affects that end with the fight (or a death).
var combatAffects = [16]affectType{affTrainingManual, affCharmed, affCoughing, affHelpless, affAnimated, affSnakeCharm, affHeld, affAsleep, affRepulsed, affHeldFast, affRegenerating, affFallsDead, affHeal3, affEndsSpiritualHammer, affStrikeTied, affBloodDrain}

// removeCombatAffects = ovr25:100A.
func (e *Engine) removeCombatAffects(c *Character) {
	for _, t := range combatAffects {
		e.RemoveAffect(c, t, nil)
	}
}

// removeTiedAffects = ovr25:1041 (DS:0C24): the tied-strike and blood-drain links end.
func (e *Engine) removeTiedAffects(c *Character) {
	for _, t := range []affectType{affStrikeTied, affBloodDrain} {
		e.RemoveAffect(c, t, nil)
	}
}

// CureAffect = ovr25:1081 (char, type): "is Cured" and the affect goes.
func (e *Engine) CureAffect(c *Character, typ affectType) bool {
	a := e.FindAffect(c, typ)
	if a == nil {
		return false
	}
	e.CharMessage(c, "is Cured", true)
	e.RemoveAffect(c, typ, a)
	return true
}

// strValue = ovr25:10E4 (str, str00): one byte for strength, 18/xx as
// xx+1 (18/00 = 101), others +100.
func strValue(str, str00 byte) byte {
	if str == 18 {
		return str00 + 1
	}
	return str + 0x64
}

// strDecode = ovr25:110D: the inverse, from an affect's saved strength.
func strDecode(a *Affect) (str, str00 byte) {
	str = a.Data & 0x7F
	if str <= 0x65 {
		return 18, str - 1
	}
	return str - 0x64, 0
}

// SetStrength = ovr25:115C (char, str, str00, out): magical strength
// (affects 0x0C / 0x26 remember the natural value). Returns false, with
// the request encoded, when it would lower the current strength.
func (e *Engine) SetStrength(c *Character, str, str00 byte) (byte, bool) {
	cur, cur00 := c.Stats[statStr], c.Str00
	if str < cur || str == 18 && str00 < cur00 {
		return strValue(str, str00) + 0x80, false
	}
	i := slices.IndexFunc(c.Affects, func(a *Affect) bool {
		return (a.Type == affEnlarged || a.Type == affStrength) && a.Data < 0x80
	})
	if i >= 0 {
		keep := c.Affects[i]
		v := strValue(cur, cur00)
		cur, cur00 = strDecode(keep)
		keep.Data = v + 0x80
	}
	c.Stats[statStr] = str
	c.Str00 = str00
	e.ApplyItems(c)
	return strValue(cur, cur00), true
}

// Saving throw types (the Saves order).
const (
	saveVsParalysis = iota // paralysis, poison, death magic
	saveVsPetrify          // petrification, polymorph
	saveVsWand             // rod, staff, wand
	saveVsBreath
	saveVsSpell
)

// Damage type bits (rv.dmgType).
const (
	dmgFire     = 0x01
	dmgCold     = 0x02
	dmgElectric = 0x04
	dmgMagic    = 0x08
	dmgAcid     = 0x10
)

// DamageMode values for Damage (what a successful save does).
const (
	saveNegate = 1
	saveHalf   = 2
)

// Damage = ovr25:1344 (char, damage, mode, saved): reports and applies
// damage, the lost spell, and going down.
func (e *Engine) Damage(c *Character, dmg int, mode byte, saved bool) {
	e.rv.damage = byte(dmg)
	e.affectGroup(c, grpDamage)
	if saved {
		switch mode {
		case saveNegate:
			e.rv.damage = 0
		case saveHalf:
			e.rv.damage /= 2
		}
	}
	if e.rv.damage == 0 {
		return
	}
	var msg string
	if e.rv.damage != 1 {
		msg = "takes " + strconv.Itoa(int(e.rv.damage)) + " points of damage "
	} else {
		msg = "takes 1 point of damage "
	}
	switch e.rv.dmgType &^ dmgMagic {
	case 1:
		msg += "from Fire"
	case 2:
		msg += "from Cold"
	case 4:
		msg += "from Electricity"
	case 0x10:
		msg += "from Acid"
	}
	if e.rv.dmgType&dmgMagic == e.rv.dmgType {
		msg += "from Magic"
	}
	e.hitMessage(c, false, msg)
	e.ApplyDamage(c, int(e.rv.damage))
	if e.beginFlag == 5 && c.Combat != nil {
		c.Combat.CanCast = 0
		if c.Combat.Spell > 0 {
			e.CharMessage(c, "lost a spell", true)
			e.forgetSpell(c.Combat.Spell)
			c.Combat.Spell = 0
		}
	}
	if c.Okay == 0 {
		msg = "Goes Down"
		if c.Status == statusDying {
			msg += ", and is Dying"
		}
		if s := c.Status; s >= statusDead && s <= statusGone {
			msg = "is killed"
		}
		e.CharMessage(c, msg, false)
		if e.beginFlag != 5 {
			e.MsgDelay()
		} else {
			e.removeCombatAffects(c)
			e.affectGroup(c, grpDown)
			if c.Okay == 0 {
				e.combatRemoveFromMap(c)
			} else {
				e.MsgDelay()
			}
		}
	}
	e.ClearMessageArea()
}

// AddAffectMsg = ovr25:1656 (char, type, time, data, call, saveMode, saved,
// message): adds an affect unless something blocks it (or a save negates
// it); a shorter or permanent copy is replaced.
func (e *Engine) AddAffectMsg(c *Character, typ affectType, time int, data, call, saveMode byte, saved bool, msg string) {
	msg = cut(msg, textCols)
	e.rv.affVal = typ
	e.affectGroup(c, grpAddAffect)
	if e.rv.affVal == 0 || saved && saveMode == 1 {
		e.CharMessage(c, "is Unaffected", true)
		return
	}
	if a := e.FindAffect(c, typ); a != nil {
		if a.Time > 0 && time > int(a.Time) || time == 0 {
			e.RemoveAffect(c, typ, a)
		}
	}
	e.AddAffect(c, typ, time, data, call)
	if msg != "" {
		e.hitMessage(c, true, msg)
		e.ClearMessageArea()
	}
}

// Heal = ovr25:175D (char, amount, magical): only the living (okay,
// animated, unconscious, dying) heal; affect 0x32 blocks natural healing.
func (e *Engine) Heal(c *Character, amount int, magical bool) bool {
	switch c.Status {
	case 0, 1, 4, 5:
	default:
		return false
	}
	if !(c.HP < c.MaxHP && magical) {
		if e.FindAffect(c, affMummyDisease) != nil || magical {
			return false
		}
	}
	c.HP += byte(amount)
	if c.HP > c.MaxHP {
		c.HP = c.MaxHP
	}
	if c.Okay == 0 {
		if c.Status == statusDying {
			c.Status = statusUnconscious
		}
		if c.Status == statusUnconscious && e.beginFlag != 5 {
			e.callAffect(affGetUp, c, nil, 1)
		}
	}
	return true
}

// Revive = ovr25:1869 (char, hp): back on its feet if its square is free.
func (e *Engine) Revive(c *Character, hp byte) bool {
	if !e.combatSquareFree(c, e.combatX(c), e.combatY(c), true) {
		return false
	}
	c.Status = statusOkay
	c.Okay = 1
	c.HP = hp
	if e.beginFlag == 5 {
		e.combatRedrawChar(c, 3, 0)
	}
	msg := "gets back up"
	if c.Side == sideMonsters {
		msg = "stands up and grins"
	}
	e.CharMessage(c, msg, true)
	e.countSides()
	return true
}

// CharMessage = 00BA:16EC (char, message, row, wait): the name and a
// message in the combat side panel, or in the box under the view.
func (e *Engine) CharMessage(c *Character, msg string, wait bool) {
	if e.beginFlag == 5 {
		e.combatSay(c, msg, true) // extra: under the view, not in the side pane
	} else {
		r := 0x11
		if e.resting != 0 {
			r = 0x12
		} else {
			e.DrawBorder(1, r, winRight, winBottom, 0)
		}
		e.ClearCells(1, r, winRight, winBottom)
		e.displayName(c, 1, r+1, false)
		e.DisplayTextBlock(1, r+2, winRight, winBottom, colLightGreen, true, msg)
	}
	if wait {
		e.MsgDelay()
		e.ClearMessageArea()
	}
}

// ClearMessageArea = 00BA:17E3.
func (e *Engine) ClearMessageArea() {
	if e.beginFlag != 5 {
		e.ClearCells(1, 18, winRight, winBottom)
	}
}

// hitMessage = 00BA:1FEB (char, magical, message): in combat the target
// flashes (once per message delay step for magic) with a sound.
func (e *Engine) hitMessage(c *Character, magical bool, msg string) {
	msg = cut(msg, textCols)
	if e.beginFlag != 5 {
		e.CharMessage(c, msg, true)
		return
	}
	e.soundWait(0)
	sel := 0x17
	if magical {
		sel = 0x16
	}
	e.hitColours(sel)
	if !e.combatOnScreen(c, true) {
		e.combatCentre(e.combatX(c), e.combatY(c), 3, 8)
	}
	if magical {
		e.PlaySound(sndMagicHit)
	} else {
		e.PlaySound(sndHit)
	}
	e.CharMessage(c, msg, false)
	n := 0
	if magical {
		n = int(e.msgDelay)
	}
	idx := e.combatIndex(c)
	px, py := int(3*e.combatScreenX[idx]), int(3*e.combatScreenY[idx]) // byte products
	for i := 0; i <= n; i++ {
		for f := 0; f <= 3; f++ {
			e.drawHitFrame(f, px, py)
			e.Flip()
			e.Delay(0x46)
			e.restoreHitFrame(px, py)
		}
	}
	e.soundWait(0xA)
	e.Flip()
	if n == 0 {
		e.MsgDelay()
	}
}

// ApplyDamage = 00BA:2228 (char, damage): hit points down to 0, then
// unconscious / dying (bleeding rounds) / dead at 10 below.
func (e *Engine) ApplyDamage(c *Character, dmg int) {
	hp := int(c.HP)
	over, left := 0, 0
	if hp >= dmg {
		left = hp - dmg
	} else {
		over = dmg - hp
	}
	over &= 0xFF
	switch {
	case over > 9 || left == 0 && c.Status == statusAnimated:
		c.Status = statusDead
	case over > 0:
		c.Status = statusDying
		if e.beginFlag == 5 && c.Combat != nil {
			c.Combat.Dying = byte(over)
		}
	case left == 0:
		c.Status = statusUnconscious
	}
	if s := c.Status; s == statusOkay || s == statusAnimated {
		c.HP = byte(left)
		return
	}
	c.Okay = 0
	c.HP = 0
	if e.beginFlag == 5 {
		e.rv.side[int8(c.Side)&1]--
		if c.Combat != nil {
			c.Combat.Delay = 0
		}
	}
}

// forgetSpell = 00BA:148B (spell): the selected character's first
// memorised copy of the spell is gone.
func (e *Engine) forgetSpell(sp byte) {
	c := e.selected()
	if c == nil {
		return
	}
	e.noteSpells(c)
	for i := 0; i <= 0x14; i++ {
		if c.Memorized[i] == sp {
			c.Memorized[i] = 0
			break
		}
	}
}

// healedMessage = 00BA:2348 (char).
func (e *Engine) healedMessage(c *Character) {
	msg := "is partially healed"
	if c.HP == c.MaxHP {
		msg = "is fully healed"
	}
	e.CharMessage(c, msg, true)
	if e.beginFlag != 5 {
		e.DisplayPartyList()
	}
}

// countSides = 00BA:23DB: standing members of each side (DS:6814/6815).
func (e *Engine) countSides() {
	e.rv.side = [2]int8{}
	for _, c := range e.party {
		if c.Okay != 0 {
			e.rv.side[int8(c.Side)&1]++
		}
	}
}

// clearCombatInfo = 00BA:2637 (char): reset the combat block; always true.
func (e *Engine) clearCombatInfo(c *Character) bool {
	if c.Combat != nil {
		ci := c.Combat
		ci.Delay, ci.Spell, ci.Guarding, ci.Moves = 0, 0, 0, 0
	}
	return true
}
