package engine

import "strconv"

// RandomItem = ovr22:1010 (type, var item): a new item of the given base
// type with a random bonus, or a magic item from the DS:0970 table.
func (e *Engine) RandomItem(typ byte) *Item {
	it := &Item{}
	k := 0 // DS:0970 record of a table item
	it.Readied = 0
	it.Hidden = 6
	it.Magic = 0
	it.Type = typ
	it.Weight = 0
	switch {
	case typ >= 1 && typ <= 0x3B, typ == 0x49, typ == 0x4D, typ == 0x5D:
		it.Plus = byte(e.randomItemClass())
		bonus := it.Plus + 0xA1
		switch {
		case typ == 0x15:
			if e.Dice(1, 5) == 5 {
				k = 0x90 // (sic: one word before the 0x91 record)
			}
			it.Words[2], it.Words[1] = 0x15, bonus
		case typ == 0x1C:
			it.Words[2], it.Words[1] = 0x1C, bonus
		case typ == 0x32, typ == 0x33:
			it.Words[2], it.Words[1], it.Words[0], it.Hidden = typ, 0x31, bonus, 4
		case typ == 0x34:
			it.Words[2], it.Words[1], it.Words[0], it.Hidden = typ, 0x32, bonus, 4
		case typ >= 0x35 && typ <= 0x3A:
			it.Words[2], it.Words[1], it.Words[0], it.Hidden = typ, 0x30, bonus, 4
		case typ == 0x49:
			it.Words[2], it.Words[1] = 0x3D, bonus
		case typ == 0x4D:
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
		case typ == 0x5D:
			it.Words[2], it.Words[1], it.Words[0] = 0x42, 0xE0, bonus
		default:
			it.Words[2], it.Words[1] = typ, bonus
		}
		it.Plus2 = 0
		it.Count = 0
		it.Weight = randomItemWeight(typ, it)
		mul := uint16(2000)
		switch typ {
		case 0x3B:
			mul = 2500
		case 0x49, 0x1C:
			mul = 150
		case 0x35, 0x36, 0x4D:
			mul = 3000
		case 0x37, 0x38:
			mul = 3500
		case 0x39:
			mul = 4000
		case 0x3A:
			mul = 5000
		}
		it.Price = uint16(int16(int8(it.Plus))) * mul
	case typ == 0x3D, typ == 0x3E:
		// scrolls: 1-3 spells, clerical (0x3D) or magic-user
		n := e.Dice(1, 3)
		if typ == 0x3D {
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
			if typ == 0x3D {
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
	case typ == 0x3F:
		k = 0x49
	case typ == 0x43:
		k = 0x91
	case typ == 0x45:
		switch d := e.Dice(1, 10); {
		case d == 1:
			k = 0x29
		case d <= 6:
			k = 0x79
		default:
			k = 0x81
		}
	case typ == 0x4E:
		switch e.Dice(1, 3) + 1 { // (sic: 1 can never come up, 4 gives nothing)
		case 2:
			k = 0x41
		case 3:
			k = 0x71
		}
	case typ == 0x4F:
		k = 0x39
	case typ == 0x54:
		k = 9
	case typ == 0x5C:
		k = 0x61
	case typ == 0x47:
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
	case typ == 0x46:
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
func randomItemWeight(typ byte, it *Item) uint16 {
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
		e.BottomMessage("No Gems or Jewelry")
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
				return cut(strconv.Itoa(n)+one, 0x28)
			}
			return cut(strconv.Itoa(n)+many, 0x28)
		}
		gems := count(int(p.gems), " Gem", " Gems")
		jewels := count(int(p.jewelry), " piece of Jewelry", " pieces of Jewelry")
		e.ClearBox(1, 1, 0x26, 0x16)
		e.displayName(c, 1, 1, false) // 00BA:1816
		e.DisplayString(1, 7, 0xF, "You have a fine collection of:")
		e.DisplayString(1, 9, 0xF, gems)
		e.DisplayString(1, 0xA, 0xF, jewels)
		menu := ""
		if p.gems != 0 {
			menu = "  Gems"
		}
		if p.jewelry != 0 {
			menu = cut(menu+"  Jewelry", 0x28)
		}
		menu = cut(menu+" Exit", 0x28)
		key, _ := e.DisplayInput("Appraise : ", menu, 0xD, 0xA, 0xF, true, false)
		switch key {
		case 'G':
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
			e.DisplayString(1, 0xC, 0xF, cut("The Gem is Valued at "+strconv.Itoa(int(v))+" gp.", 0x28))
			e.sellOrKeep(c, v, 0x65)
		case 'J':
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
			e.DisplayString(1, 0xC, 0xF, cut("The Jewel is Valued at "+strconv.Itoa(int(v))+" gp.", 0x28))
			e.sellOrKeep(c, v, 0xD6)
		case 'E', 0:
			done = true
		}
		e.ApplyItems(c)
	}
	return any
}

// sellOrKeep is the shared tail of the gem and jewel branches of 19B2:
// "Sell Keep" when the character can carry one more item, else only Sell.
func (e *Engine) sellOrKeep(c *Character, v uint16, word byte) {
	sellOnly := c.ItemCount >= 0x10
	menu := "Sell Keep"
	if sellOnly {
		menu = "Sell"
	}
	key, _ := e.DisplayInput("You can : ", menu, 0xD, 0xA, 0xF, true, false)
	if key == 'K' && !sellOnly {
		it := &Item{}
		it.Words[2] = word
		it.Type = 0x46
		it.Price = v
		it.Weight = 1
		it.Hidden = 0
		// extra: the original loses the gem when the item list is empty
		// (it appends only to its local copy); the port always keeps it.
		c.Items = append(c.Items, it)
		return
	}
	e.purse.gold += uint32(v)
}
