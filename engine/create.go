package engine

import (
	"strconv"
	"strings"
)

// Character creation and dropping, ported from ovr17.

// freeCharacter = ovr17:0000 (var c): disposes the combat block, the item
// and affect lists and the record, then nils the pointer. Go's collector
// does the freeing.
func (e *Engine) freeCharacter(c **Character) {
	if *c != nil {
		(*c).Combat, (*c).Items, (*c).Affects = nil, nil, nil
	}
	*c = nil
}

// selectTemp makes c the selected character (DS:5D92 = c) although it is not
// in the party; the returned func restores the old selection.
func (e *Engine) selectTemp(c *Character) func() {
	sel, n := e.partySel, len(e.party)
	e.party = append(e.party, c)
	e.partySel = n
	e.tempSel = c
	return func() {
		e.party = e.party[:n]
		e.partySel = sel
		e.tempSel = nil
	}
}

// CreateCharacter = ovr17:056C: pick race, gender, class and alignment, roll
// the abilities (best of six 3d6 each, then the race/class/age limits), hit
// points and starting spells; keep it, name it, pick the pictures, save it.
func (e *Engine) CreateCharacter() {
	c := &Character{}
	for i := 1; i <= 6; i++ {
		t := defaultIconColours[i]
		c.IconColour[i-1] = byte((t+8)<<4 + t)
	}
	c.BaseAC = 0x32
	c.HitDice = 0x28
	c.Status = 0
	c.Okay = 1
	c.PortHead = 1
	c.PortBody = 1
	c.Size = 1
	c.RandomID = byte(e.Random(0x100))

	// pick runs one of the four selection menus; false = left with Esc/Exit,
	// which abandons the whole creation.
	pick := func(list []*ListNode) (int, bool) {
		if q := e.ext.quick; q != nil {
			return q.choose(list)
		}
		index, redraw := 1, true
		var sel *ListNode
		for {
			k := e.ListMenu("", "", 0xD, 0xA, 0xF, 1, 2, 0x26, 0x16, list, true, &redraw, &index, &sel)
			if k == 0 {
				return 0, false
			}
			if k == 0xD {
				return index, true
			}
		}
	}

	// Race.
	list := newList(7)
	list[0].Flag, list[0].Text = 1, "Pick Race"
	raceLoop := 0 // the loop variable, left at 7 and used below (quirk)
	for raceLoop = 1; raceLoop <= 7; raceLoop++ {
		if raceLoop >= 1 && raceLoop <= 5 {
			list[raceLoop].Text = "  " + raceNames[raceLoop]
		} else if raceLoop == 7 {
			list[raceLoop-1].Text = "  " + raceNames[raceLoop]
		}
	}
	raceLoop = 7 // for-loop exit value
	idx, ok := pick(list)
	if !ok {
		return
	}
	if idx == 6 {
		idx++
	}
	c.Race = byte(idx)
	switch c.Race {
	case raceHalfling:
		c.IconSize = 1
		e.AddAffect(c, affSaveBonusVsType0, 0, 0xFF, 0)
		e.AddAffect(c, affSaveBonusVsType42, 0, 0xFF, 0)
	case raceDwarf:
		c.IconSize = 1
		for _, t := range []affectType{affSaveBonusVsType0, affSaveBonusVsType42, affHitBonusVsTargetNames, affMinus4VsNames} {
			e.AddAffect(c, t, 0, 0xFF, 0)
		}
	case raceGnome:
		c.IconSize = 1
		for _, t := range []affectType{affSaveBonusVsType42, affHitBonusVsNames, affMinus4VsNames, affMinus4VsBugbearGnoll} {
			e.AddAffect(c, t, 0, 0xFF, 0)
		}
	case raceElf:
		c.IconSize = 2
		e.AddAffect(c, affBlock90, 0, 0xFF, 0)
	case raceHalfElf:
		c.IconSize = 2
		e.AddAffect(c, affBlock30, 0, 0xFF, 0)
	default:
		c.IconSize = 2
	}

	// Gender.
	list = newList(3)
	list[0].Flag, list[0].Text = 1, "Pick Gender"
	for i := 0; i <= 1; i++ {
		list[i+1].Text = "  " + genderNames[i]
	}
	if idx, ok = pick(list); !ok {
		return
	}
	c.Sex = byte(idx - 1)

	// Class.
	race := int(int8(c.Race))
	cnt := int(raceClasses[race][0])
	list = newList(cnt + 1)
	list[0].Flag, list[0].Text = 1, "Pick Class"
	for k := 1; k <= cnt; k++ {
		list[k].Text = "  " + className(raceClasses[race][k])
	}
	if idx, ok = pick(list); !ok {
		return
	}
	c.Class = raceClasses[race][idx]
	c.MaxLevel = 1
	class := int(int8(c.Class))
	switch {
	case class >= 0 && class <= 7:
		c.Levels[class] = 1
	case class == 8:
		c.Levels[clsCleric], c.Levels[clsFighter] = 1, 1
	case class == 9:
		c.Levels[clsCleric], c.Levels[clsFighter], c.Levels[clsMage] = 1, 1, 1
	case class == 10:
		c.Levels[clsCleric], c.Levels[clsRanger] = 1, 1
	case class == 11:
		c.Levels[clsCleric], c.Levels[clsMage] = 1, 1
	case class == 12:
		c.Levels[clsCleric], c.Levels[clsThief] = 1, 1
	case class == 13:
		c.Levels[clsFighter], c.Levels[clsMage] = 1, 1
	case class == 14:
		c.Levels[clsFighter], c.Levels[clsThief] = 1, 1
	case class == 15:
		c.Levels[clsFighter], c.Levels[clsMage], c.Levels[clsThief] = 1, 1, 1
	case class == 16:
		c.Levels[clsMage], c.Levels[clsThief] = 1, 1
	}
	if class == 3 {
		e.AddAffect(c, affProtEvil, 0, 0xFF, 0) // CotAB: a paladin's lasting protection from evil
	}
	if int8(c.Levels[clsThief]) > 0 {
		e.thiefSkills(c) // ovr24:0329
	}
	c.ClassMask = 0
	c.HitDice = 0
	for k := 0; k <= 7; k++ {
		if int8(c.Levels[k]) > 0 {
			if t := hitDiceTable[k][int8(c.Levels[k])]; t > c.HitDice {
				c.HitDice = t
			}
			c.ClassMask += classMask[k]
		}
	}
	e.savingThrows(c) // ovr24:0255

	// Alignment.
	cnt = int(classAlignments[class][0])
	list = newList(cnt + 1)
	list[0].Flag, list[0].Text = 1, "Pick Alignment"
	for k := 1; k <= cnt; k++ {
		list[k].Text = "  " + alignmentNames[int8(classAlignments[class][k])]
	}
	if idx, ok = pick(list); !ok {
		return
	}
	c.Alignment = classAlignments[class][idx]

	// Starting age: single classes roll the dice, multi-classes use the
	// maximum.
	age := 0
	ages := &startAge[race]
	if class <= 7 {
		a := ages[class]
		age = int(a.Base) + int(byte(e.Dice(int(a.Count), int(a.Sides))))
	} else {
		o := -1
		switch class {
		case 8, 9, 0xA, 0xB, 0xC: // 0xA (cleric/ranger) as in CotAB
			o = clsCleric
		case 0xD, 0xF, 0x10:
			o = clsMage
		case 0xE:
			o = clsFighter
		}
		if o >= 0 {
			age = int(ages[o].Base) + int(ages[o].Count)*int(ages[o].Sides)
		}
	}
	c.Age = uint16(age)

	// adjust: age, race limits and class minimums on a rolled stat.
	adjust := func(i int, v byte) byte {
		// Age steps DS:3F31 + race*10 (4 words) against the age word.
		older := func(j int) bool { return int(ageSteps[race][j]) < int(c.Age) }
		switch i {
		case 0:
			if v != 0 && older(0) {
				v++
			}
			if v != 0 && older(1) {
				v--
			}
			if v != 0 && older(2) {
				v -= 2
			}
			if v != 0 && older(3) {
				v--
			}
		case 1:
			if older(1) {
				v++
			}
			if older(3) {
				v++
			}
		case 2:
			if !older(0) {
				v--
			}
			if older(1) {
				v++
			}
			if older(2) {
				v++
			}
			if older(3) {
				v++
			}
		case 3:
			if older(2) {
				v -= 2
			}
			if older(3) {
				v--
			}
		case 4:
			if older(1) {
				v--
			}
			if older(2) {
				v--
			}
			if older(3) {
				v--
			}
		}
		// Race limits.
		lo, hi := raceLimits[race].limits(i)
		if v < lo {
			v = lo
		}
		if v > hi {
			v = hi
		}
		// Class minimum (DS:3F81 + class*6).
		if m := int(classMinStats[class][i]); int(v) < m {
			v = byte(m)
		}
		return v
	}
	// best (extra: Minimal stats): the most each stat can reach, from a
	// roll of 18, in the units of minStats (STR as strScore).
	var best [6]int
	for i := range best {
		v := adjust(i, 18)
		if i == 2 && v < 0xD && class >= 8 && class <= 0xC {
			v = 0xD
		}
		best[i] = int(v)
		if i == 0 {
			best[0] = strScore(v, 0)
			if v == 0x12 && fighterKind(c) {
				best[0] = strScore(v, min(100, raceLimits[race].Str00Max))
			}
		}
	}
	undo := e.selectTemp(c)
	e.CharacterSheet() // ovr20:004A
	tries := 0         // Minimal stats: rolls so far, 0 when not rolling
	for {
		c.Str00 = 0
		for i := 0; i <= 5; i++ {
			st := &c.Stats[i]
			*st = 0
			for n := 1; n <= 6; n++ {
				roll := byte(e.Dice(3, 6))
				// raceLoop is 7 here, so none of these ever applies.
				switch raceLoop {
				case 1:
					if i == 4 {
						roll++
					} else if i == 5 {
						roll--
					}
				case 2:
					if i == 3 {
						roll++
					} else if i == 4 {
						roll--
					}
				case 5:
					if i == 0 {
						roll--
					} else if i == 3 {
						roll++
					}
				case 6:
					if i == 0 || i == 4 {
						roll++
					} else if i == 5 {
						roll -= 2
					}
				}
				if *st < roll {
					*st = roll
				}
			}
			*st = adjust(i, *st)
			if i == 0 {
				if *st == 0x12 && fighterKind(c) {
					c.Str00 = byte(e.Random(100) + 1)
					if m := raceLimits[race].Str00Max; m < c.Str00 {
						c.Str00 = m
					}
				}
			}
			if i == 2 && *st < 0xD && class >= 8 && class <= 0xC { // set [8..12] at cs:0525
				*st = 0xD
			}
			if tries == 0 || tries%100 == 0 {
				e.showStat(i, false) // ovr20:0901
			}
		}

		// Hit points: one die per class, averaged, with the constitution
		// adjustment.
		c.HP = c.MaxHP
		c.BaseAttack[0], c.BaseDice[0], c.BaseSides[0], c.StrBonus, c.BaseMove = 2, 1, 2, 1, 0xC
		var sum, count byte
		for k := 0; k <= 7; k++ {
			if int8(c.Levels[k]) <= 0 {
				continue
			}
			if k == 0 {
				c.SpellSlots[0] = 1
			} else if k == 5 {
				c.SpellSlots[3] = 1
			}
			sum += byte(e.Dice(int(startHPDice[k][0]), int(startHPDice[k][1])))
			count++
			if k == 0 {
				e.wisdomSlots(c) // ovr24:01AF
				for s := 1; s <= 0x38; s++ {
					if rec := spellTable[s][:2]; rec[0] == 0 && rec[1] == 1 {
						c.SpellBook[s-1] = 1
					}
				}
			} else if k == 5 {
				c.SpellBook[10], c.SpellBook[17], c.SpellBook[18], c.SpellBook[20] = 1, 1, 1, 1
			}
		}
		c.Coins[3] = uint16(int16(int(sum) * 10 / int(count)))
		c.HPRoll = byte(e.rollHitDice(c, 0xFF))
		c.MaxHP = c.HPRoll
		adj := int(int8(e.conHPBonus(c)))
		if adj < 0 {
			if int(c.MaxHP) > -adj+int(count) {
				c.MaxHP = byte((int(c.MaxHP) + adj) / int(count))
			} else {
				c.MaxHP = 1
			}
		} else {
			c.MaxHP = byte((int(c.MaxHP) + adj) / int(count))
		}
		c.HP = c.MaxHP
		c.HPRoll = byte(int(c.HPRoll) / int(count))
		if tries > 0 { // Minimal stats: roll on until met or a key stops it
			stop := e.meetsMinStats(c)
			if !stop && tries%100 == 0 {
				e.ClearRow24()
				e.DisplayString(0, 24, 0xE, "Rolling "+strconv.Itoa(tries)+"... any key stops")
				e.Delay(16) // let the window draw and see keys
				if e.keyWaiting() {
					e.ReadKeyByte()
					stop = true
				}
			}
			if !stop {
				tries++
				continue
			}
			tries = 0
			e.ClearRow24()
			for i := 0; i <= 5; i++ {
				e.showStat(i, false)
			}
		}
		e.showCombatStats() // ovr20:0608
		if q := e.ext.quick; q != nil {
			if q.good(c) {
				break
			}
			continue
		}
		e.menuSel = 2
		e.ext.selSet = true
		k, _ := e.DisplayInput("Keep this character? ", "Yes No Minimal", 0xD, 0xA, 0xF, false, false)
		if k == 'Y' {
			break
		}
		if k == 'M' { // extra: reroll until the minimums are met or a key stops it
			e.askMinStats()
			tries = 1
			for i, name := range []string{"STR", "INT", "WIS", "DEX", "CON", "CHA"} {
				if e.ext.minStats[i] > best[i] {
					max := strconv.Itoa(best[i])
					if i == 0 {
						max = fmtStr(best[0])
					}
					msg := name + " " + fmtMin(i, e.ext.minStats[i]) + " impossible here, max " + max
					if i == 0 && best[0] == 1800 {
						msg = "STR 18/xx is for fighters only, max 18"
					}
					e.PromptRow24(0xE, cut(msg, 0x28))
					tries = 0
				}
			}
			if tries == 0 {
				e.ClearRow24()
			}
		}
	}
	e.CharacterSheet()
	undo()

	for q := e.ext.quick; q != nil; q = nil {
		c.SetName(q.name)
		c.PortHead, c.PortBody = q.head, q.body
	}
	for c.PName[0] == 0 {
		c.SetName(e.inputLine("Character name: ", 0xD, 0xF)) // 0709:09E3 (extra: case kept)
		if c.PName[0] != 0 {
			break
		}
	}
	undo = e.selectTemp(c)
	if e.ext.quick == nil {
		e.choosePortrait()
		e.iconEditor()
	}
	e.SaveCharacter(c, "") // ovr18:069E
	undo()
	e.freeCharacter(&c)
}

// DropCharacter = ovr17:1F37: asks twice, then the selected member leaves
// the party (a hired NPC bids farewell, anyone else is dumped).
func (e *Engine) DropCharacter() {
	if len(e.party) == 0 {
		return
	}
	c := e.selected()
	if e.YesNo(0xE, 0xA, 0xF, "Drop "+c.Name()+" forever? ") == 'Y' &&
		e.YesNo(0xE, 0xA, 0xF, "Are you sure? ") == 'Y' {
		if c.Okay == 0 {
			e.BottomMessage("You dump " + c.Name() + " out back.")
		} else {
			e.BottomMessage(c.Name() + " bids you farewell.")
		}
		e.RemoveSelected(false, true) // ovr17:3291
		return
	}
	e.BottomMessage(c.Name() + " breathes a sigh of relief.")
}

// strScore folds 18/xx into one number: 18/01 = 1801, 18/00 = 1900.
func strScore(st, x byte) int {
	if st == 18 && x > 0 {
		return 1800 + int(x)
	}
	return int(st) * 100
}

func (e *Engine) meetsMinStats(c *Character) bool {
	for i, m := range e.ext.minStats {
		v := int(c.Stats[i])
		if i == 0 {
			v = strScore(c.Stats[0], c.Str00)
		}
		if v < m {
			return false
		}
	}
	return true
}

// askMinStats (extra): one prompt per stat; blank keeps the last value.
func (e *Engine) askMinStats() {
	for i, name := range []string{"STR", "INT", "WIS", "DEX", "CON", "CHA"} {
		for {
			cur := strconv.Itoa(e.ext.minStats[i])
			if i == 0 {
				cur = fmtStr(e.ext.minStats[0])
			}
			s := strings.TrimSpace(e.InputString("Minimum "+name+" ("+cur+"): ", 0xD, 6))
			if s == "" {
				break
			}
			if v, ok := parseMinStat(s, i == 0); ok {
				e.ext.minStats[i] = v
				break
			}
		}
	}
}

func fmtMin(i, v int) string {
	if i == 0 {
		return fmtStr(v)
	}
	return strconv.Itoa(v)
}

func fmtStr(v int) string {
	if v <= 1800 || v > 1900 {
		return strconv.Itoa(v / 100)
	}
	x := v - 1800
	if x == 100 {
		return "18/00"
	}
	if x < 10 {
		return "18/0" + strconv.Itoa(x)
	}
	return "18/" + strconv.Itoa(x)
}

func parseMinStat(s string, str bool) (int, bool) {
	a, b, slash := strings.Cut(s, "/")
	n, err := strconv.Atoi(a)
	if err != nil || n < 0 || n > 25 {
		return 0, false
	}
	if !slash {
		if str {
			return n * 100, true
		}
		return n, true
	}
	x, err := strconv.Atoi(b)
	if !str || n != 18 || err != nil || x < 0 || x > 99 {
		return 0, false
	}
	if x == 0 {
		x = 100
	}
	return strScore(18, byte(x)), true
}
