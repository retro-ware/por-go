package engine

import "strconv"

// RandomItem = ovr22:1010 (type, var item): a new item of the given base
// type with a random bonus, or a magic item from the DS:0970 table.
func (e *Engine) RandomItem(typ itemType) *Item {
	it := &Item{}
	k := 0 // DS:0970 record of a table item
	it.Readied = 0
	it.Hidden = 6
	it.Magic = 0
	it.Type = typ
	it.Weight = 0
	switch {
	case typ >= 1 && typ <= itemShield, typ == itemArrow, typ == itemBracers, typ == itemProtectionRing:
		it.Plus = byte(e.randomItemClass())
		bonus := it.Plus + 0xA1
		switch {
		case typ == itemJavelin:
			e.Dice(1, 5) // the original's one-in-five roll for a magic javelin read DS:0970 record 0x90, one word before the Cloak of Elvenkind's: a javelin named "of" (sic); extra: a plain javelin, the roll kept for the dice stream
			it.Words[2], it.Words[1] = byte(itemJavelin), bonus
		case typ == itemQuarrel:
			it.Words[2], it.Words[1] = byte(itemQuarrel), bonus
		case typ == itemLeatherArmor, typ == itemPaddedArmor:
			it.Words[2], it.Words[1], it.Words[0], it.Hidden = byte(typ), 0x31, bonus, 4
		case typ == itemStuddedLeather:
			it.Words[2], it.Words[1], it.Words[0], it.Hidden = byte(typ), 0x32, bonus, 4
		case typ >= itemRingMail && typ <= itemPlateMail:
			it.Words[2], it.Words[1], it.Words[0], it.Hidden = byte(typ), 0x30, bonus, 4
		case typ == itemArrow:
			it.Words[2], it.Words[1] = 0x3D, bonus
		case typ == itemBracers:
			it.Words[2], it.Words[1] = 0x4F, 0xA7
			it.Plus = it.Plus*2 + 2
			switch it.Plus {
			case 4:
				it.Words[0] = 0xDD
			case 6:
				it.Words[0] = 0xDE
			case 8:
				it.Words[0] = 0xDC
			}
		case typ == itemProtectionRing:
			it.Words[2], it.Words[1], it.Words[0] = 0x42, 0xE0, bonus
		default:
			it.Words[2], it.Words[1] = byte(typ), bonus
		}
		it.Plus2 = 0
		it.Count = 0
		it.Weight = randomItemWeight(typ, it)
		mul := uint16(2000)
		switch typ {
		case itemShield:
			mul = 2500
		case itemArrow, itemQuarrel:
			mul = 150
		case itemRingMail, itemScaleMail, itemBracers:
			mul = 3000
		case itemChainMail, itemSplintMail:
			mul = 3500
		case itemBandedMail:
			mul = 4000
		case itemPlateMail:
			mul = 5000
		}
		it.Price = uint16(int16(int8(it.Plus))) * mul
	case typ == itemMageScroll, typ == itemClericScroll:
		// scrolls: 1-3 spells, magic-user (0x3D) or clerical
		n := e.Dice(1, 3)
		if typ == itemMageScroll {
			it.Words[2] = 0xD1
		} else {
			it.Words[2] = 0xD0
		}
		it.Words[1] = byte(n + 0xD1)
		it.Words[0] = 0
		it.Plus = 1
		it.Weight = 0x19
		it.Count = 0
		it.Price = 0
		for i := 1; i <= n; i++ {
			lvl := e.Dice(1, 3)
			var s int
			if typ == itemMageScroll {
				switch lvl {
				case 1:
					s = e.Dice(1, 0xD) + 8
				case 2:
					s = e.Dice(1, 7) + 0x1C
				case 3:
					s = e.Dice(1, 0xB) + 0x2C
				}
			} else {
				switch lvl {
				case 1:
					s = e.Dice(1, 8)
				case 2:
					s = e.Dice(1, 7) + 0x15
				case 3:
					s = e.Dice(1, 9) + 0x23
				}
			}
			it.Spells[i-1] = byte(s)
			it.Price = it.Price + uint16(lvl)*300
		}
	case typ == itemGauntlets:
		k = 0x49
	case typ == itemCloak:
		k = 0x91
	case typ == itemRing:
		switch d := e.Dice(1, 10); {
		case d == 1:
			k = 0x29
		case d <= 6:
			k = 0x79
		default:
			k = 0x81
		}
	case typ == itemWand:
		switch e.Dice(1, 3) + 1 { // (sic: 1 can never come up, 4 gives nothing)
		case 2:
			k = 0x41
		case 3:
			k = 0x71
		}
	case typ == itemWandOfMissiles:
		k = 0x39
	case typ == itemStrengthPotion:
		k = 9
	case typ == itemCloakDisplacement:
		k = 0x61
	case typ == itemPotion:
		switch d := e.Dice(1, 0xB); {
		case d <= 5:
			k = 0x11
		case d <= 8:
			k = 1
		case d == 9:
			k = 0x19
		case d == 10:
			k = 0x51
		}
	case typ == itemValuable:
		if e.Dice(1, 10) <= 7 {
			k = 0x59
		} else {
			k = 0x89
		}
	}
	if k != 0 {
		w := func(i int) uint16 { return itemTable[i] }
		for j := 0; j <= 2; j++ {
			it.Words[j] = byte(w(k + j))
		}
		it.Plus, it.Plus2 = 1, 1
		if k == 0x61 {
			it.Plus, it.Plus2 = 2, 2
		}
		it.Weight = w(k + 3)
		it.Count = 0
		it.Price = w(k + 4)
		for j := 1; j <= 3; j++ {
			it.Spells[j-1] = byte(w(k + 4 + j))
		}
	}
	return it
}

// randomItemWeight = ovr22:1295..14A2: weight by base type (arrows and
// darts also get a count).
func randomItemWeight(typ itemType, it *Item) uint16 {
	switch typ {
	case 1, 0xD, 0xE, 0x23:
		return 0x4B
	case 2, 0x14, 0x1D, 0x1F, 0x20, 0x21, 0x27, 0x2A, 0x2C, 0x2E, 0x3B:
		return 0x32
	case 3, 0x18, 0x28:
		return 0x7D
	case 4, 0xF, 0x17, 0x22, 0x2B, 0x2D, 0x33:
		return 0x64
	case 5, 0xC, 0x11, 0x13, 0x32:
		return 0x96
	case 6:
		return 0xF
	case 7:
		return 0x1E
	case 8, 0x4D:
		return 0xA
	case 9:
		it.Count = 5
		return 0x19
	case 0xA, 0x1A, 0x24:
		return 0x3C
	case 0xB, 0x10, 0x19, 0x1B, 0x29, 0x2F:
		return 0x50
	case 0x12:
		return 0xAF
	case 0x15:
		return 0x14
	case 0x16, 0x1E:
		return 0x28
	case 0x25:
		return 0x23
	case 0x26, 0x35:
		return 0xFA
	case 0x34:
		return 0xC8
	case 0x36, 0x38:
		return 0x190
	case 0x37:
		return 0x12C
	case 0x39:
		return 0x15E
	case 0x3A:
		return 0x1C2
	case 0x5D:
		return 1
	}
	it.Count = 0xA
	return 0x28
}

// Appraise = ovr22:19B2 (var any): the shop and temple "Appraise" command.
// Each of the party's gems and pieces of jewelry is valued and sold or kept
// as an item. any is false when there is nothing to show.
func (e *Engine) Appraise() (any bool) {
	any = true
	c := e.selected()
	p := &e.purse
	if p.gems == 0 && p.jewelry == 0 {
		e.BottomMessage("Nothing to appraise")
		return false
	}
	for done := false; !done; {
		if p.gems == 0 && p.jewelry == 0 {
			done = true
			continue
		}
		count := func(n int, one, many string) string {
			switch n {
			case 0:
				return ""
			case 1:
				return cut(strconv.Itoa(n)+one, textCols)
			}
			return cut(strconv.Itoa(n)+many, textCols)
		}
		gems := count(int(p.gems), " Gem", " Gems")
		jewels := count(int(p.jewelry), " piece of Jewelry", " pieces of Jewelry")
		e.ClearBox(1, 1, winRight, winBottom)
		e.displayName(c, 1, 1, false) // 00BA:1816
		e.DisplayString(1, 7, colWhite, "You have a fine collection of:")
		e.DisplayString(1, 9, colWhite, gems)
		e.DisplayString(1, 10, colWhite, jewels)
		const (
			gem Choice = iota
			jewel
			exit
		)
		menu := Bar{{}} // the original's spacing: "  Gems  Jewelry Exit"
		if p.gems != 0 {
			menu = append(menu, Entry{" Gems", gem})
		}
		if p.jewelry != 0 {
			menu = append(menu, Entry{" Jewelry", jewel})
		}
		menu = append(menu, Entry{"Exit", exit})
		switch e.DisplayInput("Appraise : ", menu, colLightMagenta, colLightGreen, colWhite, true, false).ID {
		case gem:
			if p.gems == 0 {
				break
			}
			p.gems--
			var v uint16
			switch d := e.Dice(1, 100); {
			case d <= 25:
				v = 10
			case d <= 50:
				v = 50
			case d <= 70:
				v = 100
			case d <= 90:
				v = 500
			case d <= 99:
				v = 1000
			default:
				v = 5000
			}
			e.DisplayString(1, 12, colWhite, cut("The Gem is Valued at "+strconv.Itoa(int(v))+" gp.", textCols))
			e.sellOrKeep(c, v, 0x65)
		case jewel:
			if p.jewelry == 0 {
				break
			}
			p.jewelry--
			var v uint16
			switch d := e.Dice(1, 100); {
			case d <= 10:
				v = uint16(e.Random(900) + 100)
			case d <= 20:
				v = uint16(e.Random(1000) + 200)
			case d <= 40:
				v = uint16(e.Random(1500) + 300)
			case d <= 50:
				v = uint16(e.Random(2500) + 500)
			case d <= 70:
				v = uint16(e.Random(5000) + 1000)
			case d <= 90:
				v = uint16(e.Random(6000) + 2000)
			default:
				v = uint16(e.Random(10000) + 2000)
			}
			e.DisplayString(1, 12, colWhite, cut("The Jewel is Valued at "+strconv.Itoa(int(v))+" gp.", textCols))
			e.sellOrKeep(c, v, 0xD6)
		case exit, NoPick:
			done = true
		}
		e.ApplyItems(c)
	}
	return any
}

// sellOrKeep is the shared tail of the gem and jewel branches of 19B2:
// "Sell Keep" when the character can carry one more item, else only Sell.
func (e *Engine) sellOrKeep(c *Character, v uint16, word byte) {
	it := &Item{}
	it.Words[2] = word
	it.Type = itemValuable
	it.Price = v
	it.Weight = 1
	it.Hidden = 0
	sellOnly := e.overloaded(it, c) // extra: room in the party inventory
	const (
		sell Choice = iota
		keep
	)
	menu := Bar{{"Sell", sell}, {"Keep", keep}}
	if sellOnly {
		menu = menu[:1]
	}
	if e.DisplayInput("You can : ", menu, colLightMagenta, colLightGreen, colWhite, true, false).ID == keep {
		// extra: the original loses the gem when the item list is empty
		// (it appends only to its local copy); the port always keeps it.
		e.AddItemCopy(c, it)
		return
	}
	e.dasp("store5")
	e.purse.gold += uint32(v)
}
