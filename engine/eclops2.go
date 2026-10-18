package engine

import "slices"

// ECL opcodes 0x25..0x3D, ported from ovr04 (continued from eclops.go).

func init() {
	for op, f := range map[byte]eclOp{
		0x25: (*Engine).opOnGoto, 0x26: (*Engine).opOnGoto, 0x27: (*Engine).opTreasure,
		0x28: (*Engine).opRob, 0x29: (*Engine).opEncounterMenu, 0x2C: (*Engine).opParlay,
		0x2D: (*Engine).opCall, 0x2E: (*Engine).opDamage, 0x31: (*Engine).opRedrawView,
		0x32: (*Engine).opFindItem, 0x33: (*Engine).opNewLine, 0x34: (*Engine).opClock,
		0x36: (*Engine).opAddNPC, 0x38: (*Engine).opProgram, 0x39: (*Engine).opWho,
		0x3A: (*Engine).opDelay, 0x3B: (*Engine).opSpell, 0x3C: (*Engine).opProtection,
		0x3D: (*Engine).opClearBox,
	} {
		eclOps[op] = f
	}
}

// opOnGoto = ovr04:18F8 (ON GOTO 0x25, ON GOSUB 0x26): (v, n, addr*n).
func (e *Engine) opOnGoto() {
	e.FetchArgs(2)
	v := int(byte(e.ArgValue(1)))
	n := int(byte(e.ArgValue(2)))
	e.eclPC--
	e.FetchArgs(n)
	for i := 0; i < n; i++ {
		if v != i {
			continue
		}
		if e.es.op == 0x25 {
			e.eclPC = e.ArgAddr(i + 1)
		} else {
			e.Gosub(i + 1)
		}
		return
	}
}

// opTreasure = ovr04:19B0 (7 money amounts, items): items < 0x80 is an
// ITEM<n>.DAX block of 0x3F-byte records, 0xFF none, otherwise
// items-0x80 random items. Each lands at the head of the DS:6810 list.
func (e *Engine) opTreasure() {
	e.FetchArgs(8)
	var coins [7]uint16
	for i := range coins {
		coins[i] = e.ArgValue(i + 1)
	}
	e.loot = coinsToPurse(coins) // DS:67F4
	v := int(byte(e.ArgValue(8)))
	switch {
	case v < 0x80:
		b := e.L.Block(e.set.file("ITEM"), v)
		if len(b) == 0 {
			e.MessageRow24b(0xF, "Unable to find item file")
			e.Exit()
		}
		add := readRecords[Item](b, itemRecSize)
		slices.Reverse(add)
		e.run.treasure = append(add, e.run.treasure...)
		return
	case v == 0xFF:
		return
	}
	for i := 1; i <= v-0x80; i++ {
		e.run.itemType = e.randomItemType(e.run.itemType)
		it := e.RandomItem(e.run.itemType) // ovr22:1010
		e.run.treasure = append([]*Item{it}, e.run.treasure...)
	}
	for _, it := range e.run.treasure {
		e.ItemName(e.selected(), it, 0, 0, false, false) // 00BA:0442
	}
}

// randomItemType = the d100 tables of ovr04:1B63. A second roll of 48 or
// 49 leaves the previous type in place, as in the original.
func (e *Engine) randomItemType(prev byte) byte {
	t := prev
	r := e.Dice(1, 100)
	switch {
	case r <= 0x3C:
		r2 := e.Dice(1, 100)
		switch {
		case r2 >= 1 && r2 <= 0x2F, r2 >= 0x32 && r2 <= 0x3B:
			t = byte(r2)
			if r2 == 0x2D {
				t = 0x3B
			}
		case r2 >= 0x3C && r2 <= 0x5A:
			switch d := e.Dice(1, 10); {
			case d <= 4:
				t = 0x24
			case d <= 7:
				t = 0x23
			case d == 8:
				t = 0x22
			case d == 9:
				t = 0x25
			case d == 10:
				t = 0x26
			}
		case r2 >= 0x5B && r2 <= 0x5E:
			t = 0x49
		case r2 >= 0x5F && r2 <= 0x61:
			t = 0x5D
		case r2 >= 0x62:
			t = 0x4D
		}
	case r <= 0x55:
		t = 0x3D
	case r <= 0x5C:
		t = 0x3E
	case r <= 0x62:
		switch d := e.Dice(1, 16); {
		case d <= 7:
			t = 0x47
		case d <= 9:
			t = 0x46
		case d == 10:
			t = 0x54
		case d <= 12:
			t = 0x4E
		case d <= 14:
			t = 0x4F
		case d == 15:
			t = 0x5C
		default:
			t = 0x43
		}
	default:
		t = 0x45
	}
	return t
}

// opRob = ovr04:1DB1 (who, pct, -): take pct% of the money and, with the
// same pct, items; who 0 is the selected member (and their share of the
// purse), else everyone. (The third operand is fetched but the original
// reads operand 2 again.)
func (e *Engine) opRob() {
	e.FetchArgs(3)
	who := byte(e.ArgValue(1))
	pct := int(byte(e.ArgValue(2)))
	robbed := []*Character{e.selected()}
	if who != 0 {
		robbed = e.party
	}
	share := float64(pct) / 100 * float64(len(robbed)) / float64(max(len(e.party), 1))
	for _, n := range []*uint32{&e.purse.gold, &e.purse.copper, &e.purse.gems, &e.purse.jewelry} {
		*n -= uint32(float64(*n) * share)
	}
	for _, c := range robbed {
		e.ItemSave(c, pct)
	}
}

// opEncounterMenu = ovr04:1EF8 (sprite, max dist, picture, result, 5
// reactions, 3 texts, flee threshold, monster strength): the approach
// menu. result: 0 none, 1 combat, 2 party flees, 3 parlay.
func (e *Engine) opEncounterMenu() {
	e.es.f84E9 = 1
	e.run.f84E4 = 0
	e.slowText = true
	hi, lo := e.PartyHpRange()
	e.FetchArgs(14)
	e.run.sprite = int(byte(e.ArgValue(1)))
	e.game.MaxDist = e.ArgValue(2)
	e.run.closeup = int(byte(e.ArgValue(3)))
	dst := e.ArgAddr(4)
	var react [5]byte
	for i := range react {
		react[i] = byte(e.ArgValue(i + 5))
	}
	var texts [4]string
	for i := 1; i <= 3; i++ {
		texts[i] = e.es.strAt(i)
	}
	fleeAt := int(byte(e.ArgValue(13)))
	strength := int(byte(e.ArgValue(14)))
	e.game.MonsterDist = uint16(e.FreeSteps(e.posX, e.posY, e.facing))
	if e.game.MaxDist < e.game.MonsterDist {
		e.game.MonsterDist = e.game.MaxDist
	}
	show := func() {
		e.ShowEncounter(e.run.sprite, e.run.closeup, int(byte(e.game.MonsterDist)), &e.es.f84C8)
	}
	show()
	dist := func() uint16 { return e.game.MonsterDist }
	approach := func() bool {
		if dist() == 0 {
			return false
		}
		e.game.MonsterDist = dist() - 1
		show()
		return true
	}
	waitMsg := func() {
		e.DisplayTextBlock(1, 0x11, 0x26, 0x16, 0xA, true, "Both sides wait.")
	}
	flee := func() {
		e.WriteVar(dst, 0)
		e.msgCol, e.msgRow = 1, 0x11
		e.DisplayTextBlock(1, 0x11, 0x26, 0x16, 0xA, true, "The monsters flee.")
	}
	txt := ""
	for again := true; again; {
		anim := e.es.f84E0 != 0 && e.es.f84E1 != 0
		clear := true
		again = false
		e.msgCol, e.msgRow = 1, 0x11
		pick := func(k, stop int) {
			for {
				txt = texts[k]
				k++
				if k > 3 {
					k = 1
				}
				if txt != "" || k == stop {
					return
				}
			}
		}
		switch dist() {
		case 0:
			for k := 1; ; {
				txt = texts[k]
				k++
				if txt != "" || k > 3 {
					break
				}
			}
		case 1:
			pick(2, 2)
		case 2:
			pick(3, 3)
		}
		if txt == "" {
			clear = false
		}
		e.DisplayTextBlock(1, 0x11, 0x26, 0x16, 0xA, clear, txt)
		parlay := dist() == 0 || e.area.In3D == 0
		menu := "~COMBAT ~WAIT ~FLEE ~ADVANCE"
		if parlay {
			menu = "~COMBAT ~WAIT ~FLEE ~PARLAY"
		}
		e.animInput = anim
		choice := e.MenuInput(false, 0xF, 0xA, 0xD, menu, "")
		e.animInput = false
		if parlay && choice == 3 {
			choice = 4
		}
		if choice < 0 || choice > 4 {
			continue
		}
		// advance: PARLAY at distance 0, else one step closer.
		advance := func() {
			if dist() == 0 {
				e.WriteVar(dst, 3)
				return
			}
			approach()
			again = true
		}
		switch react[choice] {
		case 0:
			if choice == 2 && lo >= fleeAt {
				e.WriteVar(dst, 2)
			} else {
				e.WriteVar(dst, 1)
			}
		case 1:
			switch choice {
			case 0:
				e.WriteVar(dst, 1)
			case 1:
				again = true
				waitMsg()
			case 2:
				e.WriteVar(dst, 2)
			case 3:
				if !approach() {
					waitMsg()
				}
				again = true
			case 4:
				advance()
			}
		case 2:
			if choice == 0 {
				if strength > hi {
					flee()
				} else {
					e.WriteVar(dst, 1)
				}
			} else {
				flee()
			}
		case 3:
			switch choice {
			case 0:
				e.WriteVar(dst, 1)
			case 1, 3:
				if !approach() {
					waitMsg()
				}
				again = true
			case 2:
				e.WriteVar(dst, 2)
			case 4:
				advance()
			}
		case 4:
			switch choice {
			case 0:
				e.WriteVar(dst, 1)
			case 1, 3, 4:
				advance()
			case 2:
				e.WriteVar(dst, 2)
			}
		}
	}
	e.ClearBox(0, 0x18, 0x27, 0x18)
	e.slowText = false
	e.es.f84E9 = 0
}

// opParlay = ovr04:25CB (5 results, dst): the talk-style menu.
func (e *Engine) opParlay() {
	e.FetchArgs(6)
	var r [5]byte // the original keeps the results as bytes
	for i := range r {
		r[i] = byte(e.ArgValue(i + 1))
	}
	dst := e.ArgAddr(6)
	choice := e.MenuInput(false, 0xF, 0xA, 0xD, "~HAUGHTY ~SLY ~NICE ~MEEK ~ABUSIVE", " ")
	if choice >= 0 && choice < 5 {
		e.WriteVar(dst, uint16(r[choice]))
	}
}

// opCall = ovr04:2CCF (addr): calls into engine routines by their old
// addresses.
func (e *Engine) opCall() {
	e.FetchArgs(1)
	switch uint16(e.ArgAddr(1)) - 0x7FFF {
	case 0xAC91:
		e.es.cell = e.MapCell(e.posX, e.posY)
		s := &e.es
		if e.run.scriptLoad != 0 && (s.f84E0 != 0 || s.f84E3 != 0 || s.f84E5 != 0 || s.f84E6 != 0 || s.f84E8 != 0) {
			e.Redraw()
			e.StatusLine()
			s.f84E8, s.f84E5, s.f84E6, s.f84E0, s.f84E3 = 0, 0, 0, 0, 0
		}
	case 1:
		e.DuelSetup(1) // ovr08:1A6F
	case 2:
		e.DuelSetup(0)
	case 0x3A04:
		switch e.es.f84CC {
		case 8:
			e.PlaySound(sndStep)
		case 0xA:
			e.PlaySound(0xB) // DS:262A
		}
	case 0x401F:
		e.StepForward()
	case 0x4019:
		if e.viewMode == 1 {
			e.es.wallAhead = e.WallAt(e.posX, e.posY, e.facing)
		}
	}
}

// opDamage = ovr04:2775 (flags, dice, sides, bonus, save): roll damage
// against one member, everyone (flags 0x40) or flags random members.
// With flags 0x80 a saving throw (type save&7, bonus flags&0x1F) avoids
// it; 0x20 means no save when hitting everyone.
func (e *Engine) opDamage() {
	sel := e.partySel
	e.FetchArgs(5)
	flags := byte(e.ArgValue(1))
	n := int(byte(e.ArgValue(2)))
	sides := int(byte(e.ArgValue(3)))
	bonus := int(byte(e.ArgValue(4)))
	save := byte(e.ArgValue(5))
	roll := func() int { return int(byte(e.Dice(n, sides))) + bonus }
	dmg := roll()
	all := flags&0x40 != 0
	pick := 0
	if !all {
		pick = e.Dice(1, int(byte(e.game.PartyCount)))
	}
	member := func(k int) *Character { // k is 1-based; the walk stops at the end
		if k < 1 {
			k = 1
		}
		if k > len(e.party) {
			k = len(e.party)
		}
		return e.party[k-1]
	}
	if flags&0x80 != 0 {
		sb := int(flags & 0x1F)
		typ := int(save & 7)
		switch {
		case all:
			for _, c := range e.party {
				if flags&0x20 != 0 || !e.SavingThrow(c, typ, sb) {
					e.HitMessage(c, dmg)
				}
			}
		case save&0x80 != 0:
			c := e.selected()
			if typ == 0 || !e.SavingThrow(c, typ-1, sb) {
				e.HitMessage(c, dmg)
			}
		case len(e.party) > 0:
			c := member(pick)
			if !e.SavingThrow(c, typ, sb) {
				e.HitMessage(c, dmg)
			}
		}
	} else if len(e.party) > 0 {
		for i := 1; i <= int(flags); i++ {
			c := member(e.Dice(1, int(byte(e.game.PartyCount))))
			if e.RollHit(c, int(save)) { // ovr25:0C4F
				e.HitMessage(c, dmg)
			}
			dmg = roll()
		}
	}
	e.run.quit = true
	for _, c := range e.party {
		if c.Okay != 0 {
			e.run.quit = false
		}
	}
	if e.run.quit {
		e.Music(5) // Amiga 0x416C2
		e.DrawBoxTitled(1, 1, 0x26, 0x16, 0, 0xF, "The END!")
		e.msgCol, e.msgRow = 2, 2
		e.DisplayTextBlock(1, 1, 0x26, 0x16, 0xA, true, "The entire party is killed!")
		e.Delay(3000)
	}
	e.partySel = sel
	e.MessageRow24b(0xF, "press <enter>/<return> to continue")
}

// opRedrawView = ovr04:2AA2.
func (e *Engine) opRedrawView() {
	e.eclPC++
	if e.es.f84E3 != 0 {
		e.Redraw()
		e.es.f84E3, e.es.f84E0 = 0, 0
	}
}

// opFindItem = ovr04:266A (item): eq when someone carries it.
func (e *Engine) opFindItem() {
	e.FetchArgs(1)
	v := byte(e.ArgValue(1))
	e.es.flags = [6]bool{}
	e.es.flags[1] = true
	for _, c := range e.party {
		for _, it := range c.Items {
			if it.Type == v {
				e.es.flags[0], e.es.flags[1] = true, false
				return
			}
		}
	}
}

// opNewLine = ovr04:2AEB: text cursor to the start of the next row.
func (e *Engine) opNewLine() {
	e.eclPC++
	e.msgCol = 1
	e.msgRow++
	e.run.f84DF = 0
}

// opClock = ovr04:2AC1 (n): advance the clock n minutes. (The original
// passes an uninitialised local as the operand number to ArgValue; operand 1
// is what the scripts mean.)
func (e *Engine) opClock() {
	e.FetchArgs(1)
	e.AdvanceClock(int(byte(e.ArgValue(1))), 1)
}

// opAddNPC = ovr04:2B7E (monster, morale).
func (e *Engine) opAddNPC() {
	e.FetchArgs(2)
	v := int(byte(e.ArgValue(1)))
	e.AddMonsterToParty(v) // ovr18:1262
	c := e.selected()
	if c == nil {
		return
	}
	c.Control = byte(e.ArgValue(2))>>1 | 0x80
	if v == 0x18 {
		c.Side = 1
	} else {
		c.Side = 0
	}
	e.ApplyItems(c) // 00BA:0BB8
	e.DisplayPartyList()
}

// opProgram = ovr04:2E10 (n): 0 party menu, 8 ovr19:02A4, 9 encamp.
func (e *Engine) opProgram() {
	e.FetchArgs(1)
	v := byte(e.ArgValue(1))
	if e.run.restoreSel {
		e.selectChar(e.run.savedSel)
		e.run.restoreSel = false
	}
	switch v {
	case 0:
		e.PartyMenu()
		e.DrawMainScreen()
	case 8:
		e.ProgramEight() // ovr19:02A4
	case 9:
		pc := e.eclPC
		e.Encamp()
		e.eclPC = pc
		e.opExit()
	}
}

// opWho = ovr04:2B33 (prompt): choose a party member.
func (e *Engine) opWho() {
	e.FetchArgs(1)
	e.ClearBox(1, 0x11, 0x26, 0x16)
	start := e.selected()
	// extra: one party purse, so the first non-NPC pays (inn, ECL3 A1B4) or
	// gambles (tavern, A501), an okay one if there is one
	if q := e.es.strAt(1); q == "WHO WILL PAY?" || q == "WHO WILL GAMBLE?" {
		pick := -1
		for i, c := range e.party {
			if c.Control < 0x80 && (pick < 0 || c.Okay != 0 && e.party[pick].Okay == 0) {
				pick = i
			}
		}
		if pick >= 0 {
			e.partySel = pick
			return
		}
	}
	e.whoVar = true
	who := e.SelectCharacter(e.es.strAt(1), false, start) // 00BA:2C48 (var DS:5D92)
	e.whoVar = false
	if who == nil { // Esc then Enter stores nil; the harness pins it to the op's start
		who = start
	}
	for i, c := range e.party {
		if c == who {
			e.partySel = i
		}
	}
}

// opDelay = ovr04:271F.
func (e *Engine) opDelay() {
	e.eclPC++
	e.Delay(int(e.msgDelay) * 0xE1)
}

// opSpell = ovr04:2C03 (spell, dst slot, dst count): look for the spell
// among the first member's memorized ones (the scan never gets past it).
func (e *Engine) opSpell() {
	e.FetchArgs(3)
	v := byte(e.ArgValue(1))
	slot, count := 1, 0
	if len(e.party) > 0 {
		c := e.party[0]
		for {
			if c.Memorized[slot] == v {
				break
			}
			if slot > 0x51 {
				slot = 0xFF
				break
			}
			slot++
		}
		count = 1
	}
	// Original bug kept: the two destination addresses are stored in byte
	// variables, so only their low bytes are used.
	e.WriteVar(e.ArgAddr(2)&0xFF, uint16(slot))
	e.WriteVar(e.ArgAddr(3)&0xFF, uint16(count))
}

// opProtection = ovr04:2E7D (base): draw the glyph string stored in the
// variables from base on, each value+1 a symbol id, until one is <= 1.
func (e *Engine) opProtection() {
	e.FetchArgs(1)
	e.es.f84C8 = [2]byte{}
	e.es.f84E0 = 0
	if e.run.f84DE != 0 && e.msgRow > 0xD {
		e.msgCol, e.msgRow = 2, 5
		e.es.f84E2 = 1
	}
	e.run.f84DF = 1
	base := e.ArgAddr(1)
	for i := 0; ; i++ {
		v := byte(e.ReadVar(base+i) + 1)
		if v <= 1 {
			break
		}
		e.DisplayChar(e.msgCol, e.msgRow, 0xF, v, false)
		e.msgCol++
	}
	e.msgRow++
}

// opClearBox = ovr04:2B14: blank the view window for text.
func (e *Engine) opClearBox() {
	e.eclPC++
	e.ClearBox(1, 1, 0xF, 0xF)
	e.run.f84DE = 1
}
