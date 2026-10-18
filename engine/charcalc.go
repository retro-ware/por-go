package engine

// Derived character values, ported from ovr24.

// RecomputeCharacter = ovr24:0000: hit dice, highest level, attacks,
// spell slots, known cleric spells, saving throws and thief skills from
// the class levels.
func (e *Engine) RecomputeCharacter(c *Character) {
	c.HitDice = 0
	for i := 0; i <= 7; i++ {
		lvl := int(int8(c.Levels[i]))
		if v := hitDiceTable[i][lvl]; v > c.HitDice {
			c.HitDice = v
		}
		if int(int8(c.MaxLevel)) < lvl {
			c.MaxLevel = byte(lvl)
		}
		switch i {
		case 2: // fighter
			if lvl > 6 {
				c.BaseAttack[0] = 3
			} else {
				c.BaseAttack[0] = 2
			}
		case 3, 4: // paladin 7, ranger 8: three attacks per two rounds (CotAB ovr026)
			if lvl > 6 && i == 3 || lvl > 7 {
				c.BaseAttack[0] = 3
			}
		case 0: // cleric
			if lvl > 1 {
				for j := 1; j <= 3; j++ {
					c.SpellSlots[j-1] = clericSlots[lvl][j-1]
				}
				e.wisdomSlots(c)
				for k := 1; k <= 0x38; k++ {
					rec := spellTable[k][:2]
					if rec[0] == 0 && c.spellSlot(0, int(int8(rec[1]))) > 0 {
						c.SpellBook[k-1] = 1
					}
				}
			}
		case 5: // magic-user
			if lvl > 1 {
				for j := 1; j <= 3; j++ {
					c.SpellSlots[3+j-1] = muSlots[lvl][j-1]
				}
			}
		}
	}
	e.savingThrows(c)
	if int8(c.Levels[clsThief]) > 0 {
		e.thiefSkills(c)
	}
}

// wisdomSlots = ovr24:01AF: bonus cleric spells for wisdom 13..17.
func (e *Engine) wisdomSlots(c *Character) {
	w := c.Stats[statWis]
	for _, b := range []struct {
		min  byte
		slot int
	}{{0xC, 1}, {0xD, 1}, {0xE, 2}, {0xF, 2}, {0x10, 3}} {
		if w > b.min && c.SpellSlots[b.slot-1] > 0 {
			c.SpellSlots[b.slot-1]++
		}
	}
}

// savingThrows = ovr24:0255: the best of each class's table (DS:426E).
func (e *Engine) savingThrows(c *Character) {
	for s := 0; s <= 4; s++ {
		c.Saves[s] = 0x14
		for cl := 0; cl <= 7; cl++ {
			lvl := int(int8(c.Levels[cl]))
			if lvl <= 0 {
				continue
			}
			v := saveTable[cl*9+lvl][s]
			if cl == clsPaladin {
				v = paladinSaves[lvl][s]
			}
			if c.Saves[s] > v {
				c.Saves[s] = v
			}
		}
	}
}

// paladinSaves: CotAB's paladin rows (SaveThrowValues), levels 0..8; PoR's
// table only repeats the fighter's.
var paladinSaves = [9][5]byte{
	{20, 20, 20, 20, 20}, {12, 13, 14, 15, 15}, {12, 13, 14, 15, 15}, {11, 12, 13, 14, 14}, {11, 12, 13, 14, 14},
	{9, 9, 11, 11, 12}, {9, 9, 11, 11, 12}, {8, 9, 10, 10, 11}, {9, 9, 10, 10, 11},
}

// thiefSkills = ovr24:0329: level table DS:3CF3 plus race DS:3D3B, plus
// dexterity DS:3D4E for the first five skills.
func (e *Engine) thiefSkills(c *Character) {
	race := int(int8(c.Race))
	lvl := int(int8(c.Levels[clsThief]))
	for k := 1; k <= 8; k++ {
		r := int(thiefRace[race][k-1])
		t := int(thiefBase[lvl][k-1])
		if r < 0 && -r > t {
			c.Thief[k-1] = 0
			continue
		}
		c.Thief[k-1] = byte(t + r)
		if k < 6 {
			c.Thief[k-1] += byte(thiefDex[c.Stats[statDex]][k-1])
		}
	}
}

// Readied-item slots at char+0xCC (13 far pointers). Slot numbers come
// from the base item record; 2 is body armour, 9 and 10 the two rings, 11
// arrows, 12 quarrels.
const (
	slotWeapon = 0
	slotArmor  = 2
	slotRing1  = 9
	slotRing2  = 10
	slotArrows = 11
	slotBolts  = 12
)

// itemBase is the 16-byte base record of an item type (DS:5582 + 16*type,
// from the ITEMS file).
type itemBase struct {
	Slot       byte // +0 readied slot (0 weapon ... 9 ring, 0xB..0xD scrolls)
	Hands      byte // +1
	LargeDice  byte // +2 damage against large targets
	LargeSides byte // +3
	LargeBonus byte // +4 signed
	Speed      byte // +5 rate of fire / weapon speed
	Protection byte // +6 0x80 + AC given
	Material   byte // +7 flags the resistance handlers test (0x81, bit 0)
	_          byte // +8
	Dice       byte // +9
	Sides      byte // +A
	Bonus      byte // +B signed
	Range      byte // +C squares + 1 (1 = melee)
	Classes    byte // +D class bits that may use it (ClassMask)
	Usage      byte // +E flags: 1 arrows, 2 missile, 4 strength bonus, 8 fires ammunition, 0x10 thrown, 0x80 bolts
	_          byte // +F
}

// itemTypes is the size of the base item table (0x810 bytes).
const itemTypes = 0x81

// base is the base record of an item type (a blank one past the table).
func (e *Engine) base(typ itemType) *itemBase {
	if int(typ) < len(e.items) {
		return &e.items[typ]
	}
	return &itemBase{}
}

// readiedWeight = DS:4846.
var readiedWeight uint16

// ApplyItems = 00BA:0BB8: everything that follows from the items carried
// and readied: slots, weight, hands, attack and damage, armour class,
// movement and saving throw bonus.
func (e *Engine) ApplyItems(c *Character) {
	c.Slots = [13]*Item{}
	bag := false
	c.ItemCount = 0
	c.Hands = 0
	c.Weight = 0
	readiedWeight = 0
	for _, it := range c.Items {
		c.ItemCount++
		w := it.Weight
		if it.Count > 0 {
			w *= uint16(it.Count)
		}
		c.Weight = c.Weight + w
		if it.Readied == 0 {
			continue
		}
		readiedWeight += w
		typ := it.Type
		switch s := e.base(typ).Slot; {
		case s <= 8:
			c.Slots[s] = it
		case s == 9:
			if c.Slots[slotRing1] == nil {
				c.Slots[slotRing1] = it
			} else if c.Slots[slotRing2] == nil {
				c.Slots[slotRing2] = it
			}
		}
		if typ == itemArrow {
			c.Slots[slotArrows] = it
		}
		if typ == itemQuarrel {
			c.Slots[slotBolts] = it
		}
		if it.Words[0] == 0xBA {
			bag = true
		}
		c.Hands += e.base(typ).Hands
	}
	c.Dice, c.Sides, c.Bonus = c.BaseDice, c.BaseSides, c.BaseBonus
	var ac [5]int8
	ringOnly := false
	c.SaveBonus = 0
	c.AC[0] = c.BaseAC
	c.Move = c.BaseMove
	c.THAC0 = c.HitDice
	ac[0] = dexAC(c.Stats[statDex])
	if c.Slots[slotWeapon] == nil {
		c.THAC0 += byte(strHit(c))
		c.Bonus[0] += byte(strDamage(c))
	}
	e.weaponStats(c)
	for _, it := range c.Items {
		if it.Readied == 0 {
			continue
		}
		e.armourMove(it, c)
		if e.itemAC(c, it, &ac) {
			ringOnly = true
		}
	}
	if ringOnly {
		ac[3] = 0
	}
	if bag {
		if c.Weight < 5000 {
			c.Weight = 0
		} else {
			c.Weight = c.Weight - 5000
		}
		if c.Weight < readiedWeight {
			c.Weight = readiedWeight
		}
	}
	e.encumbrance(c)
	if int(ac[4]) < int(c.AC[0]) {
		ac[4] = int8(c.AC[0])
	}
	c.AC[0] = 0
	for i := 0; i <= 4; i++ {
		c.AC[0] += byte(ac[i])
	}
	c.AC[1] = byte(int(ac[4]) + int(ac[2]) + int(ac[3]) - 2)
	if fighterKind(c) && int8(c.Race) > 0 {
		c.FightLevel = max(c.Levels[clsFighter], c.Levels[clsPaladin], c.Levels[clsRanger])
	} else {
		c.FightLevel = 1
	}
}

// weaponStats = 00BA:0000: to-hit and damage of the readied weapon.
func (e *Engine) weaponStats(c *Character) {
	w := c.Slots[slotWeapon]
	if w == nil {
		return
	}
	typ := w.Type
	flags := e.base(typ).Usage
	c.THAC0 = c.HitDice
	if flags&2 != 0 { // missile
		c.THAC0 += byte(dexMissile(c.Stats[statDex]))
	}
	c.Bonus[0] = e.base(typ).Bonus
	if flags&4 != 0 { // strength applies
		c.THAC0 += byte(strHit(c))
		c.Bonus[0] += byte(strDamage(c))
	}
	plus := int8(w.Plus)
	if flags > 0x7F && c.Slots[slotBolts] != nil {
		plus += int8(c.Slots[slotBolts].Plus)
	}
	if flags&1 != 0 && c.Slots[slotArrows] != nil {
		plus += int8(c.Slots[slotArrows].Plus)
	}
	c.Bonus[0] += byte(plus)
	if c.Race == raceElf && (w.Type >= itemCompositeLongBow && w.Type <= itemShortBow || w.Type == itemShortSword || w.Type == itemLongSword) {
		plus++ // elves with bows and swords
	}
	c.THAC0 += byte(plus)
	c.Dice[0] = e.base(typ).Dice
	c.Sides[0] = e.base(typ).Sides
}

// armourMove = 00BA:01FB (char, item): body armour sets the movement by
// its weight; magical armour moves 3 faster up to 9.
func (e *Engine) armourMove(it *Item, c *Character) {
	if e.base(it.Type).Slot != slotArmor {
		return
	}
	switch w := it.Weight; {
	case w <= 0x96:
		c.Move = c.BaseMove
	case w <= 0x18F:
		c.Move = 9
	default:
		c.Move = 6
	}
	if it.Plus != 0 && c.Move <= 9 {
		c.Move += 3
	}
}

// itemAC = 00BA:0282 (char, item, ac, ringOnly): ac[1] shield, ac[2]
// protective items, ac[3] best ring, ac[4] best armour; magical body
// armour cancels rings.
func (e *Engine) itemAC(c *Character, it *Item, ac *[5]int8) (ringOnly bool) {
	typ := it.Type
	v := e.base(typ).Protection
	if v <= 0x7F {
		return
	}
	v &= 0x7F
	slot := e.base(typ).Slot
	plus := int8(it.Plus)
	switch {
	case slot == 1:
		ac[1] = int8(v) + plus
	case v == 0:
		if slot == 9 {
			if plus > ac[3] {
				ac[3] = plus
			}
		} else {
			ac[2] += plus
		}
		c.SaveBonus += it.Plus2
	case int(v)+int(plus) > int(ac[4]):
		ac[4] = int8(v) + plus
		if plus > 0 && slot == slotArmor {
			ringOnly = true
		}
	}
	return
}

// encumbrance = 00BA:03A0: movement by the weight over the allowance.
func (e *Engine) encumbrance(c *Character) {
	over := int(int16(c.Weight - uint16(strCarry(c))))
	if over < 0 {
		over = 0
	}
	m := byte(3)
	switch {
	case over <= 0x200:
		m = c.Move
	case over <= 0x300:
		m = 9
	case over <= 0x400:
		m = 6
	}
	if m < c.Move {
		c.Move = m
	}
}

// dexAC = 00BA:10E6.
func dexAC(d byte) int8 {
	switch {
	case d >= 1 && d <= 3:
		return -4
	case d >= 4 && d <= 6:
		return int8(d) - 7
	case d >= 15 && d <= 18:
		return int8(d) - 14
	case d >= 19 && d <= 20:
		return 4
	case d >= 21 && d <= 23:
		return 5
	case d >= 24 && d <= 25:
		return 6
	}
	return 0
}

// dexMissile = 00BA:1168.
func dexMissile(d byte) int8 {
	switch {
	case d <= 2:
		return -4
	case d <= 5:
		return int8(d) - 6
	case d >= 16 && d <= 18:
		return int8(d) - 15
	case d >= 19 && d <= 20:
		return 3
	case d >= 21 && d <= 23:
		return 4
	case d >= 24 && d <= 25:
		return 5
	}
	return 0
}

// strIndex = 00BA:11EA: strength as one scale (18/xx = 18..23, 19+ = 24+).
func strIndex(c *Character) byte {
	s, x := c.Stats[statStr], c.Str00
	switch {
	case s <= 0x11:
		return s
	case s == 0x12:
		switch {
		case x == 0:
			return 0x12
		case x <= 0x32:
			return 0x13
		case x <= 0x4B:
			return 0x14
		case x <= 0x5A:
			return 0x15
		case x <= 0x63:
			return 0x16
		case x == 0x64:
			return 0x17
		}
	case s <= 0x19:
		return s + 5
	}
	return 0 // the original returns an unset local here
}

// strHit = 00BA:1282.
func strHit(c *Character) int8 {
	if c.StrBonus == 0 {
		return 0
	}
	switch i := strIndex(c); {
	case i >= 1 && i <= 3:
		return -3
	case i >= 4 && i <= 5:
		return -2
	case i >= 6 && i <= 7:
		return -1
	case i >= 0x11 && i <= 0x13:
		return 1
	case i >= 0x14 && i <= 0x16:
		return 2
	case i >= 0x17 && i <= 0x19:
		return 3
	case i >= 0x1A && i <= 0x1B:
		return 4
	case i >= 0x1C && i <= 0x1E:
		return int8(i) - 0x17
	}
	return 0
}

// strDamage = 00BA:1325.
func strDamage(c *Character) int8 {
	if c.StrBonus == 0 {
		return 0
	}
	switch i := strIndex(c); {
	case i >= 1 && i <= 2:
		return -2
	case i >= 3 && i <= 5:
		return -1
	case i == 0x10:
		return 1
	case i >= 0x11 && i <= 0x13:
		return int8(i) - 0x10
	case i >= 0x14 && i <= 0x1D:
		return int8(i) - 0x11
	case i == 0x1E:
		return 0xE
	}
	return 0
}

// strCarry = 00BA:13AB: weight allowance change.
func strCarry(c *Character) int16 {
	switch i := int16(strIndex(c)); {
	case i >= 1 && i <= 3:
		return -350
	case i >= 4 && i <= 5:
		return -250
	case i >= 6 && i <= 7:
		return -150
	case i >= 0xC && i <= 0xD:
		return 100
	case i >= 0xE && i <= 0xF:
		return 200
	case i == 0x10:
		return 350
	case i >= 0x11 && i <= 0x15:
		return (i-0x11)*250 + 500
	case i >= 0x16 && i <= 0x1A:
		return (i-0x16)*1000 + 2000
	case i == 0x1B:
		return 7500
	case i >= 0x1C && i <= 0x1E:
		return (i-0x1C)*3000 + 9000
	}
	return 0
}

// isHelpless = 00BA:0B73: any of the four affects in DS:286C.
func (e *Engine) isHelpless(c *Character) bool {
	found := false
	for i := 1; i <= 4; i++ {
		if e.FindAffect(c, helplessAffects[i]) != nil {
			found = true
		}
	}
	return found
}

// atRaceLimit = 00BA:2F57 (char, class): the demihuman level limit is
// reached (fighter limits depend on strength 17/18; race 4 clerics stop at 5).
func atRaceLimit(c *Character, class int) bool {
	lvl, str := int8(c.Levels[class]), c.Stats[statStr]
	if lvl <= 0 {
		return false
	}
	fighter := class == clsFighter
	switch c.Race {
	case raceDwarf:
		return fighter && (lvl == 8 && str == 0x11 || lvl == 7 && str < 0x11)
	case raceElf:
		return fighter && (lvl == 7 || lvl == 6 && str == 0x11 || lvl == 5 && str < 0x11)
	case raceGnome:
		return fighter && (lvl == 6 || lvl == 5 && str < 0x12)
	case raceHalfElf:
		if class == clsCleric {
			return lvl == 5
		}
		return fighter && (lvl == 8 || lvl == 7 && str == 0x11 || lvl == 6 && str < 0x11)
	case raceHalfling:
		return fighter && (lvl == 6 || lvl == 5 && str == 0x11 || lvl == 4 && str < 0x11)
	}
	return false
}
