package engine

// Spell effect routines of ovr23 (0x100F..0x2540). CastSpell (ovr23:0C20)
// calls them through the table DS:6B84 filled by ovr23:3413, indexed by the
// spell number DS:681B; each is called without arguments and works on the
// spell globals: spellID (DS:681B), spellN/spellList (DS:6C94/6C91, list
// entry 1 is DS:6C95, the first target).

// spellTableByte reads a byte of the spell record (16 bytes at DS:31FA+16*spell).
func (e *Engine) spellTableByte(spell, ofs int) byte {
	return spellTable[spell][ofs]
}

// spellBless = ovr23:100F: Bless, "is Blessed" on the selected character's side.
func (e *Engine) spellBless() {
	e.applySpellToSide(int(e.selected().Side), "is Blessed")
}

// spellBane = ovr23:1040: Bane, "is Cursed".
func (e *Engine) spellBane() {
	side := 0
	if isParty(e.selected()) {
		side = 1
	}
	e.applySpellToSide(side, "is Cursed")
}

// spellCureLightWounds = ovr23:106B: Cure Light Wounds on the first target, 1d8.
func (e *Engine) spellCureLightWounds() {
	if e.nSpellTargets != 0 {
		if e.Heal(e.tgt(1), e.Dice(1, 8), false) {
			e.healedMessage(e.tgt(1))
		}
	}
}

// spellDamage1d8 = ovr23:10A7: 1d8 magic damage.
func (e *Engine) spellDamage1d8() {
	e.applySpell(int(e.curSpell), 0, 0, e.RollDice(1, 8), 8, "")
}

// spellAffected = ovr23:10E9: "is affected" (shared by several spells).
func (e *Engine) spellAffected() { e.applySpell(int(e.curSpell), 0, 0, 0, 0, "is affected") }

// spellProtected = ovr23:1123: "is protected".
func (e *Engine) spellProtected() { e.applySpell(int(e.curSpell), 0, 0, 0, 0, "is protected") }

// spellColdResist = ovr23:1162: "is cold-resistant".
func (e *Engine) spellColdResist() { e.applySpell(int(e.curSpell), 0, 0, 0, 0, "is cold-resistant") }

// spellDamageLevel = ovr23:1190: damage equal to the caster level, type 9.
func (e *Engine) spellDamageLevel() {
	e.applySpell(int(e.curSpell), 0, 0, e.casterLevel(int(e.curSpell)), 9, "")
}

// spellCharmPerson = ovr23:11DD: Charm Person; large or strong targets are unaffected.
func (e *Engine) spellCharmPerson() {
	t := e.tgt(1)
	if t.Kind > 1 || t.Size > 1 {
		e.CharMessage(t, "is unaffected", 0xA, true)
		return
	}
	lvl := e.casterLevel(int(e.curSpell))
	e.applySpell(int(e.curSpell), (int(e.selected().Side)<<7+lvl)&0xFF, 1, 0, 0, "is charmed")
	if a := e.FindAffect(t, 0xB); a != nil {
		e.callAffect(0xB, e.tgt(1), a, 0)
	}
}

// spellStrengthPercent = ovr23:12A5: Strength; the percentile comes from the caster
// level (levels above 6 leave DS:4845 as it was).
func (e *Engine) spellStrengthPercent() {
	e.v4844 = 0x12
	switch e.casterLevel(int(e.curSpell)) {
	case 1:
		e.v4845 = 0
	case 2:
		e.v4845 = 1
	case 3:
		e.v4845 = 0x33
	case 4:
		e.v4845 = 0x4C
	case 5:
		e.v4845 = 0x5B
	case 6:
		e.v4845 = 0x64
	}
	t := e.tgt(1)
	out, ok := e.SetStrength(t, e.v4844, e.v4845)
	if ok {
		e.CharMessage(t, "is stronger", 0xA, true)
	}
	e.AddAffect(t, 0xC, e.spellAmount(int(e.curSpell)), out, 1)
}

// spellReduce = ovr23:136E: Reduce (undoes the strength affect 0x0C) unless the target saves.
func (e *Engine) spellReduce() {
	if e.nSpellTargets > 0 {
		t := e.tgt(1)
		if !e.SavingThrow(t, 4, 0) && e.CureAffect(t, 0xC) {
			e.CharMessage(t, "has been reduced", 0xA, true)
		}
	}
}

// spellFriends = ovr23:13D8: Friends, charisma +2d4 (at most 25).
func (e *Engine) spellFriends() {
	t := e.tgt(1)
	old := t.Stats[statCha]
	t.Stats[statCha] += byte(e.Dice(2, 4))
	if t.Stats[statCha] > 0x19 {
		t.Stats[statCha] = 0x19
	}
	e.applySpell(int(e.curSpell), int(old), 1, 0, 0, "is friendly")
}

// spellMagicMissile = ovr23:1439: magic damage of (level/2)d4 + level/2.
func (e *Engine) spellMagicMissile() {
	lvl := e.casterLevel(int(e.curSpell))
	e.applySpell(int(e.curSpell), 0, 0, (e.RollDice(lvl>>1, 4)+lvl>>1)&0xFF, 8, "")
}

// spellShield = ovr23:1499: "is shielded".
func (e *Engine) spellShield() { e.applySpell(int(e.curSpell), 0, 0, 0, 0, "is shielded") }

// spellBurningHands = ovr23:14C7: caster level + 1d8 damage, type 0xC.
func (e *Engine) spellBurningHands() {
	lvl := e.casterLevel(int(e.curSpell))
	e.applySpell(int(e.curSpell), 0, 0, (lvl+e.RollDice(1, 8))&0xFF, 0xC, "")
}

// spellSleep = ovr23:151B: Sleep. 4d4 hit dice of targets fall asleep, weakest
// first in list order; the others (and the already asleep) leave the list.
func (e *Engine) spellSleep() {
	e.spellFlag6820 = 1
	e.v4844 = byte(e.Dice(4, 4))
	for i := 1; i <= e.nSpellTargets; i++ {
		t := e.tgt(i)
		switch lvl := int8(t.MaxLevel); {
		case lvl >= 0 && lvl <= 1:
			e.v4845 = 1
		case lvl == 2:
			e.v4845 = 2
		case lvl == 3:
			e.v4845 = 4
		case lvl == 4:
			e.v4845 = 6
		case lvl == 5:
			if t.Race == 0 {
				e.v4845 = 0xA
			} else {
				e.v4845 = 0x14
			}
		default:
			e.v4845 = 0x14
		}
		if t.Status != 1 && e.FindAffect(t, 0x35) == nil && e.v4844 >= e.v4845 {
			e.v4844 -= e.v4845
		} else {
			e.setTgt(i, nil)
		}
	}
	e.applySpell(int(e.curSpell), 0, 0, 0, 0, "falls asleep")
}

// spellHold = ovr23:1658: Hold Person/Monster. The save bonus depends on the
// number of targets; each target gets a missile and the affect from the spell record.
func (e *Engine) spellHold() {
	var bonus int8 // other counts leave the local unset (0)
	switch e.nSpellTargets {
	case 1:
		if e.curSpell == 0x17 {
			bonus = -2
		} else {
			bonus = -3
		}
	case 2:
		bonus = -1
	}
	spell := int(e.curSpell)
	for i := e.nSpellTargets; i >= 1; i-- {
		t := e.tgt(i)
		if t == nil {
			continue
		}
		if i < e.nSpellTargets {
			e.soundWait(0)
			e.PlaySound(sndSpell)
			e.hitColours(0x12)
			sel := e.selected()
			e.flyMissile(e.combatX(sel), e.combatY(sel), e.combatX(t), e.combatY(t), 4, 0x1E)
			e.soundWait(0xA)
		}
		saved := e.SavingThrow(t, int(e.spellTableByte(spell, 9)), int(bonus))
		if t.Kind > 1 || t.Size > 1 {
			saved = true
		}
		e.AddAffectMsg(t, e.spellTableByte(spell, 10), e.spellAmount(spell), byte(e.casterLevel(spell)), 0,
			e.spellTableByte(spell, 8), saved, "is held")
	}
}

// spellFireResist = ovr23:17E9: "is fire resistant".
func (e *Engine) spellFireResist() { e.applySpell(int(e.curSpell), 0, 0, 0, 0, "is fire resistant") }

// spellSilence = ovr23:1822: "is silenced".
func (e *Engine) spellSilence() { e.applySpell(int(e.curSpell), 0, 0, 0, 0, "is silenced") }

// spellRaiseDead = ovr23:185B: Raise Dead style: a dead target is dropped from
// the list; one with affect 0x37 is brought to 1 hit point and stunned (0x0F).
func (e *Engine) spellRaiseDead() {
	t := e.tgt(1)
	if t.Status == 1 {
		e.setTgt(1, nil)
		return
	}
	if e.FindAffect(t, 0x37) == nil {
		return
	}
	if t.HP == 0 {
		t.HP = 1
	}
	e.applySpell(int(e.curSpell), 0xFF, 1, 0, 0, "is affected")
	e.callAffect(0x4E, t, nil, 1)
	e.AddAffect(t, 0xF, 10, 0xFF, 1)
}

// spellUndeadTargets = ovr23:190F: the party members of kind 0xE whose hit points
// fit in the caster's hit points become the targets.
func (e *Engine) spellUndeadTargets() {
	e.v4844 = e.selected().HP
	e.nSpellTargets = 0
	for _, p := range e.party {
		if p.Kind == 0xE && e.v4844 >= p.HP {
			e.v4844 -= p.HP
			e.nSpellTargets++
			e.setTgt(e.nSpellTargets, p)
		}
	}
	e.applySpell(int(e.curSpell), 0, 0, 0, 0, "is charmed")
}

// spellApplyAnd17 = ovr23:19C3: applies the spell, then runs the affect 0x17 handler on the first target.
func (e *Engine) spellApplyAnd17() {
	e.applySpell(int(e.curSpell), 0, 1, 0, 0, "")
	e.callAffect(0x17, e.tgt(1), nil, 0)
}

// spellInvisible = ovr23:1A16: "is invisible".
func (e *Engine) spellInvisible() { e.applySpell(int(e.curSpell), 0, 0, 0, 0, "is invisible") }

// spellKnock = ovr23:1A4F: Knock.
func (e *Engine) spellKnock() { e.applySpell(int(e.curSpell), 0, 0, 0, 0, "Knock-Knock") }

// spellMirrorImage = ovr23:1A8A: Mirror Image, 1d4 images.
func (e *Engine) spellMirrorImage() {
	e.applySpell(int(e.curSpell), e.Dice(1, 4), 0, 0, 0, "is duplicated")
}

// spellWeaken = ovr23:1ACC: "is weakened".
func (e *Engine) spellWeaken() { e.applySpell(int(e.curSpell), 0, 0, 0, 0, "is weakened") }

// spellStinkingCloud = ovr23:1B11: Stinking Cloud. A 0x13-byte cloud record (caster,
// the four squares' old ground and flags, target square, number of the
// caster's clouds) is added; its squares become ground 0x1E.
func (e *Engine) spellStinkingCloud() {
	e.spellFlag6820 = 1
	sel := e.selected()
	lvl := e.casterLevel(int(e.curSpell))
	n := 0
	// the last cloud of the list is not counted
	for i := 0; i < len(e.clouds)-1; i++ {
		if e.clouds[i].Ptr00 == sel {
			n++
		}
	}
	cl := &cloud{}
	e.clouds = append(e.clouds, cl)
	e.AddAffect(sel, 0x28, lvl, byte(lvl+n<<4), 1)
	cl.Ptr00 = sel
	cl.Num = byte(n)
	cl.X = byte(e.tgtX)
	cl.Y = byte(e.tgtY)
	var who [5]byte
	for k := 1; k <= 4; k++ {
		dir := int(cloudDirs[k])
		x := int(int8(e.tgtX)) + dirDX[dir]
		y := int(int8(e.tgtY)) + dirDY[dir]
		w, g := e.mapAt(x, y)
		who[k] = w
		if g > 0 && e.groundInfo(g)[0] < 0xFF {
			cl.Covered[k-1] = 1
		} else {
			cl.Covered[k-1] = 0
		}
		if g == 0x1E {
			// an older cloud already covers the square: keep its ground
			for _, o := range e.clouds {
				if o == cl {
					continue
				}
				for j := 1; j <= 4; j++ {
					if o.Covered[j-1] == 0 {
						continue
					}
					oj := int(cloudDirs[j])
					if dirDX[dir]+int(int8(e.tgtX)) == dirDX[oj]+int(int8(o.X)) &&
						dirDY[dir]+int(int8(e.tgtY)) == dirDY[oj]+int(int8(o.Y)) &&
						o.Ground[j-1] != 0x1E {
						g = o.Ground[j-1]
					}
				}
			}
		} else if g == 0x1F {
			// a body lies there: keep the ground under it
			for b := 1; b <= e.nbodies; b++ {
				if int(int8(e.bodies[b].x)) == x && int(int8(e.bodies[b].y)) == y {
					g = e.bodies[b].ground
				}
			}
		}
		cl.Ground[k-1] = g
		if cl.Covered[k-1] != 0 {
			e.cmap.ground[y][x] = 0x1E
		}
	}
	e.CharMessage(sel, "Creates a noxious cloud", 0xA, false)
	e.combatCentre(int(e.tgtX), int(e.tgtY), 0xFF, 8)
	e.MsgDelay()
	e.ClearMessageArea()
	for k := 1; k <= 4; k++ {
		if who[k] > 0 {
			e.stinkingCloud(e.combatant(who[k]))
		}
	}
}

// spellEnlarge = ovr23:1F41: Enlarge/Strength of the first target: 1d4/1d6/1d8
// by its classes, added to its strength, exceptional strength above 18.
func (e *Engine) spellEnlarge() {
	var ext, add byte // add is an unset local when no class qualifies
	t := e.tgt(1)
	if int8(t.Levels[clsMage]) > 0 {
		add = byte(e.Dice(1, 4))
	}
	if int8(t.Levels[clsCleric]) > 0 || int8(t.Levels[clsThief]) > 0 {
		add = byte(e.Dice(1, 6))
	}
	if fighterKind(t) {
		add = byte(e.Dice(1, 8))
	}
	str := t.Stats[statStr] + add
	if str > 0x12 {
		if fighterKind(t) {
			ext = byte(int(str-0x12)*10 + int(t.Str00))
			if ext > 0x64 {
				ext = 0x64
			}
			str = 0x12
		} else {
			str = 0x12
		}
	}
	out, ok := e.SetStrength(t, str, ext)
	if ok {
		e.CharMessage(t, "is stronger", 0xA, true)
	}
	e.AddAffect(t, 0x26, e.spellAmount(int(e.curSpell)), out, 1)
}

// spellAnimateDead = ovr23:206F: Animate Dead. Corpses of the party list (status 6,
// kind 0) are put back on their feet on the caster's side, one per caster level.
func (e *Engine) spellAnimateDead() {
	e.spellFlag6820 = 1
	lvl := e.casterLevel(int(e.curSpell))
	e.nSpellTargets = 0
	sel := e.selected()
	for _, p := range e.party {
		if lvl <= 0 {
			break
		}
		if p.Status != 6 || p.Kind != 0 || !e.combatSquareFree(p, e.combatX(p), e.combatY(p), true) {
			continue
		}
		data := byte(int(p.Side)<<4 + e.casterLevel(int(e.curSpell)))
		p.Side = sel.Side
		p.Auto = 1
		p.Undead = 2
		p.FightLevel = 0
		p.BaseMove = 6
		for i := 0; i <= 0x14; i++ {
			p.Memorized[i] = 0
		}
		if p.Control > 0x7F {
			p.Control = 0xB2
		} else {
			p.Control = 0xB3
		}
		p.Kind = 4
		if e.beginFlag == 5 {
			p.Combat.Target = nil
		}
		lvl--
		if e.Revive(p, p.MaxHP) {
			e.AddAffectMsg(p, 0x20, 0, data, 1, 0, false, "is animated")
			p.Status = 1
		}
	}
}

// spellCureBlind = ovr23:2217: cures blindness (affect 0x21): "can see".
func (e *Engine) spellCureBlind() {
	if e.CureAffect(e.tgt(1), 0x21) {
		e.hitMessage(e.tgt(1), true, "can see")
	}
}

// spellBlind = ovr23:225D: "is blind".
func (e *Engine) spellBlind() { e.applySpell(int(e.curSpell), 0, 0, 0, 0, "is blind") }

// cureDiseaseAffects = ovr23:228A: with the cure flag (DS:681F) set, cures
// affects 0x22, 0x2B (and 0x2C, 0x1F) and 0x32 (and 0x39) of the first target.
func (e *Engine) cureDiseaseAffects() bool {
	cured := false
	e.rv.cure = 1
	t := e.tgt(1)
	if e.CureAffect(t, 0x22) {
		cured = true
	}
	if e.CureAffect(e.tgt(1), 0x2B) {
		cured = true
		e.RemoveAffect(e.tgt(1), 0x2C, nil)
		e.RemoveAffect(e.tgt(1), 0x1F, nil)
	}
	if e.CureAffect(e.tgt(1), 0x32) {
		cured = true
		e.RemoveAffect(e.tgt(1), 0x39, nil)
	}
	e.rv.cure = 0
	return cured
}

// spellCureDisease = ovr23:232F: Cure Blindness spell body (the result is unused).
func (e *Engine) spellCureDisease() { e.cureDiseaseAffects() }

// spellDisease = ovr23:234C: "is diseased".
func (e *Engine) spellDisease() { e.applySpell(int(e.curSpell), 0, 1, 0, 0, "is diseased") }

// spellDispel = ovr23:2385: Dispel Magic. Each pass walks the affects of the
// first target (the loop variable only counts passes); every temporary
// affect goes with a chance of 50% +5% per level above, -10% per level
// below, the level of the affect being its data low nibble.
func (e *Engine) spellDispel() {
	e.spellFlag6820 = 1
	lvl := e.casterLevel(int(e.curSpell))
	for i := 1; i <= e.nSpellTargets; i++ {
		hit := false
		t := e.tgt(1)
		for _, a := range append([]*Affect(nil), t.Affects...) {
			if a.Data >= 0xFF {
				continue
			}
			e.v4845 = a.Data & 0xF
			switch {
			case byte(lvl) > e.v4845:
				e.v4844 = byte((lvl-int(e.v4845))*5 + 0x32)
			case byte(lvl) < e.v4845:
				e.v4844 = byte(0x32 - (int(e.v4845)-lvl)*2)
			default:
				e.v4844 = 0x32
			}
			if byte(e.Dice(1, 100)) <= e.v4844 {
				e.RemoveAffect(t, a.Type, a)
				hit = true
			}
		}
		if hit {
			e.hitMessage(t, true, "is affected")
		}
	}
}

// spellPrayer = ovr23:24D6: "is praying", cast at caster level with the side in the high nibble.
func (e *Engine) spellPrayer() {
	e.applySpell(int(e.curSpell), int(e.selected().Side)<<4+e.casterLevel(int(e.curSpell)), 0, 0, 0, "is praying")
}
