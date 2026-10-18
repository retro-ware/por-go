package engine

import (
	"strconv"
	"strings"
)

// ovr20: the View screen (character sheet, items, money, spell lists).

// ClearScreen = 0709:0CAB.
func (e *Engine) ClearScreen() {
	if e.Trace != nil {
		e.Trace("0709:0CAB", "ClearScreen")
	}
	e.fillCells(0, 0, 0x27, 0x18)
}

// viewChar = DS:4716, the character on the sheet.
func (e *Engine) viewChar() *Character { return e.view.char }

type viewState struct {
	char  *Character // DS:4716
	start *Character // DS:471A
}

// CharacterSheet = ovr20:004A.
func (e *Engine) CharacterSheet() {
	c := e.selected()
	e.view.char = c
	e.ClearScreen()
	e.DrawBorder(1, 1, winRight, winBottom, 0)
	e.displayName(c, 1, 1, false)
	if c.Control > 0x7F {
		e.DisplayString(len(c.Name())+3, 1, colLightGreen, "(NPC)")
	}
	x := 1
	s := genderNames[c.Sex]
	e.DisplayString(x, 3, colWhite, s)
	x += len(s) + 1
	s = raceNames[int8(c.Race)]
	e.DisplayString(x, 3, colWhite, s)
	x += len(s) + 1
	e.DisplayString(x, 3, colWhite, "Age "+strconv.Itoa(int(c.Age)))
	e.DisplayString(1, 4, colWhite, alignmentNames[int8(c.Alignment)])
	e.DisplayString(1, 5, colWhite, className(c.Class))
	for i, l := range []string{"STR ", "INT ", "WIS ", "DEX ", "CON ", "CHA "} {
		e.DisplayString(1, i+7, colLightGreen, l)
		e.showStat(i, false)
	}
	e.showMoney()
	e.DisplayString(1, 15, colWhite, "Level")
	s = ""
	for i := 0; i <= 7; i++ {
		if l := int8(c.Levels[i]); l > 0 {
			if s != "" {
				s += "/"
			}
			s += strconv.Itoa(int(l))
		}
	}
	e.DisplayString(7, 15, colWhite, s)
	e.DisplayString(17, 15, colWhite, "Exp "+strconv.Itoa(int(int32(c.Exp))))
	e.showCombatStats()
	y := 0x14
	if w := c.Slots[slotWeapon]; w != nil {
		e.DisplayString(1, y, colWhite, "Weapon")
		e.ItemName(c, w, 8, y, false, true)
	}
	y++
	if a := c.Slots[slotArmor]; a != nil {
		e.DisplayString(2, y, colWhite, "Armor")
		e.ItemName(c, a, 8, y, false, true)
	}
	y++
	e.DisplayString(1, y, colWhite, "Status")
	e.DisplayString(8, y, colLightGreen, e.statusName(c))
	e.sheetEffects(c)
	if e.viewBit0 != 0 { // the original: only while set 3 is loaded (its HEAD/BODY); extra: everywhere, from charArt
		e.SheetPortrait()
	}
}

// Extra: the sheet lists the spell effects on the character (the camp
// menu's Display, which is not offered) on the free row under HP. When they
// do not all fit, the View menu offers Magic with the whole list.
const effectsRow = 0x13

func (e *Engine) effectsLine(c *Character) (string, bool) {
	names := e.affectList(c)
	if len(names) == 0 {
		return "", true
	}
	s := strings.Join(names, ", ")
	if len(s) <= winRight-9 {
		return s, true
	}
	n := 0
	for len(strings.Join(names[:n+1], ", "))+len(", +9 more") <= winRight-9 {
		n++
	}
	if n == 0 {
		return strconv.Itoa(len(names)) + " spell effects", false
	}
	return strings.Join(names[:n], ", ") + ", +" + strconv.Itoa(len(names)-n) + " more", false
}

func (e *Engine) effectsFit(c *Character) bool { _, ok := e.effectsLine(c); return ok }

func (e *Engine) sheetEffects(c *Character) {
	s, _ := e.effectsLine(c)
	if s == "" {
		return
	}
	e.DisplayString(1, effectsRow, colWhite, "Effects")
	e.DisplayString(9, effectsRow, colLightGreen, s)
}

// showCombatStats = ovr20:0608: AC, HP, THAC0, damage, weight, movement.
func (e *Engine) showCombatStats() {
	c := e.selected()
	e.ApplyItems(c)
	y := 0x11
	e.DisplayString(1, y, colWhite, "AC    ")
	e.displayAC(c, 4, y)
	e.DisplayString(1, y+1, colWhite, "HP    ")
	e.displayHP(c, 4, y+1, false)
	x := 8
	e.DisplayString(x+1, y, colWhite, "THAC0   ")
	e.DisplayString(x+7, y, colLightGreen, strconv.Itoa(int(byte(0x3C-c.THAC0))))
	d := cut(damageText(c), textCols)
	e.DisplayString(x, y+1, colWhite, "Damage        ")
	e.DisplayString(x+7, y+1, colLightGreen, d)
	x = 0x16
	e.DisplayString(x, y, colWhite, "Encumbrance      ")
	e.DisplayString(x+0xC, y, colLightGreen, strconv.Itoa(int(c.Weight)))
	m := c.Move
	// extra: the original (ovr20:086A) doubles under Slow and halves under Haste
	if e.FindAffect(c, affHaste) != nil {
		m <<= 1
	}
	if e.FindAffect(c, affSlow) != nil {
		m >>= 1
	}
	e.DisplayString(x+3, y+1, colWhite, "Movement   ")
	e.DisplayString(x+0xC, y+1, colLightGreen, strconv.Itoa(int(m)))
}

// showStat = ovr20:0901 (stat, highlight): one ability score, with the
// exceptional strength in brackets.
func (e *Engine) showStat(stat int, hi bool) {
	col := colLightGreen
	if hi {
		col = colLightMagenta
	}
	c := e.selected()
	x := 5
	e.ClearCells(5, stat+7, 11, stat+7)
	v := c.Stats[stat]
	if v < 10 {
		x++
	}
	e.DisplayString(x, stat+7, col, strconv.Itoa(int(v)))
	if stat == 0 && v == 0x12 && c.Str00 > 0 {
		s := strconv.Itoa(int(c.Str00))
		if c.Str00 < 10 {
			s = "0" + s
		}
		if c.Str00 == 100 {
			s = "00"
		}
		e.DisplayString(7, 7, col, "("+s+")")
	}
}

// ViewCharacter = ovr20:0AB3 (var quit): the View command. Returns quit,
// which ItemsMenu sets when an item was used in combat.
func (e *Engine) ViewCharacter() bool {
	if e.beginFlag == 5 {
		e.combatViewEnter() // ovr31 (seg 03E0) +2F
	}
	key := byte(' ')
	quit := false
	e.view.start = e.selected()
	direct := e.ext.toItems // F1-F8: start on the inventory (extras.go)
	e.ext.toItems = false
	if direct && len(e.selected().Items) > 0 {
		var leave bool
		if quit, leave = e.inventory(direct); leave {
			key = 'E'
		}
	}
	if key != 'E' && !quit {
		e.CharacterSheet()
	}
	for key != 0 && key != 'E' && !quit {
		c := e.viewChar()
		spells := false
		for i := 0; i <= 0x14; i++ {
			if c.Memorized[i] > 0 {
				spells = true
			}
		}
		menu := ""
		if len(c.Items) > 0 {
			menu += "Items "
		}
		if spells {
			menu += "Spells "
		}
		if !e.effectsFit(c) {
			menu += "Magic " // extra: all of the spell effects
		}
		if e.paladinCan(c, affLaidOnHands) {
			menu += "Heal "
		}
		if e.paladinCan(c, affCuredDisease) {
			menu += "Cure "
		}
		menu = cut(menu+"Exit", textCols)
		e.ext.viewing = true // F1..F8 switch members (extras.go)
		k, extra := e.DisplayInput("View:", menu, colLightMagenta, colLightGreen, colWhite, false, false)
		e.ext.viewing = false
		key = k
		_ = extra
		switch key {
		case 'I':
			var leave bool
			if quit, leave = e.inventory(direct); leave {
				key = 'E'
			}
		case 'S':
			idx := -1
			e.SpellList(0, 0, &idx, &spells)
		case 'M':
			e.displayAffects("Magic", []*Character{c})
		case 'H':
			e.paladinHeal(c)
		case 'C':
			e.paladinCure(c)
		}
		if !quit && (key == 'I' || key == 'S' || key == 'M' || key == 'H' || key == 'C') {
			e.CharacterSheet()
		}
	}
	if e.beginFlag == 5 {
		e.combatViewLeave() // ovr31 (seg 03E0) +2A
	}
	e.DrawMainScreen()
	return quit
}

// Paladin powers from CotAB (ovr020). Two port-only affect types time them
// (affLaidOnHands, affCuredDisease): one heal a day, one cure disease a week
// (one is all a paladin of PoR's level 8 cap gets).

// paladinCan = CotAB CanCastHeal/CanCastCureDiseases: out of combat, okay,
// and the power not used yet.
func (e *Engine) paladinCan(c *Character, used affectType) bool {
	return int8(c.Levels[clsPaladin]) > 0 && e.beginFlag != 5 && c.Status == statusOkay && e.FindAffect(c, used) == nil
}

// paladinHeal = CotAB PaladinHeal: lay on hands for 2 hp per level.
func (e *Engine) paladinHeal(c *Character) {
	t := e.SelectCharacter("Heal whom? ", true, c)
	if t == nil {
		return
	}
	if e.Heal(t, 2*int(c.Levels[clsPaladin]), true) {
		e.DisplayMessageWait(colLightGreen, t.Name()+" feels better")
	} else {
		e.DisplayMessageWait(colLightGreen, t.Name()+" is unaffected")
	}
	e.AddAffect(c, affLaidOnHands, 1440, 0, 0)
}

// paladinCure = CotAB PaladinCureDisease: the temple's diseases go.
func (e *Engine) paladinCure(c *Character) {
	t := e.SelectCharacter("Cure whom? ", true, c)
	if t == nil {
		return
	}
	ill := false
	for _, d := range diseases {
		ill = ill || e.FindAffect(t, d) != nil
	}
	if !ill && e.YesNo(colLightMagenta, colLightGreen, colWhite, t.Name()+" is not diseased, cure anyway: ") != 'Y' {
		return
	}
	e.rv.cure = 1
	for _, d := range diseases {
		e.RemoveAffect(t, d, nil)
	}
	e.rv.cure = 0
	e.AddAffect(c, affCuredDisease, 0x2760, 0, 0)
	e.DisplayMessageWait(colLightGreen, t.Name()+" is cured")
}

// inventory runs ItemsMenu for ViewCharacter (extra:
// leave reports that the View page should close too: F1-F8 switched to
// another member, or Esc in an inventory F1-F8 opened, unless P came back
// to the View page).
func (e *Engine) inventory(direct bool) (quit, leave bool) {
	e.ext.toSheet, e.ext.switchTo = false, false
	quit = e.ItemsMenu()
	leave = !quit && (e.ext.switchTo || direct && !e.ext.toSheet || e.ext.throwing != nil)
	e.ext.toSheet, e.ext.switchTo = false, false
	return quit, leave
}

// canLoseItem = ovr20:0D70 (item): readied items must be unreadied first;
// a scroll someone is about to scribe from asks before it goes.
func (e *Engine) canLoseItem(it *Item) bool {
	ok := false
	switch {
	case it.Readied != 0:
		e.BottomMessage("Must be unreadied")
	case !e.isScroll(it):
		ok = true
	case it.Spells[0] > 0x7F || it.Spells[1] > 0x7F || it.Spells[2] > 0x7F:
		c := e.selected()
		e.displayName(c, 1, 21, false)
		e.msgCol, e.msgRow = len(c.Name())+2, 0x15
		e.DisplayTextBlock(1, 21, winRight, winBottom, colYellow, false, " was going to scribe from that scroll")
		ok = e.YesNo(colLightMagenta, colLightGreen, colWhite, "is it Okay to lose it? ") == 'Y'
	default:
		ok = true
	}
	e.ClearCells(1, 21, winRight, winBottom)
	return ok
}

// SheetPortrait = ovr29:04F9: the character's head and body in a box at
// the top right of the sheet.
func (e *Engine) SheetPortrait() {
	c := e.selected()
	if c.PortHead == 0 || c.PortBody == 0 {
		return
	}
	e.DrawBoxTitled(28, 1, winRight, 11, 0, colBlack, "")
	e.LoadPortrait(charArt, int(portraitHead[c.PortHead]), int(portraitBody[c.PortBody]))
	e.DrawPortrait(0x1C, 1, true)
}

// ItemsMenu = ovr20:0EF8: the character's items with Ready, Use, Trade,
// Drop, Halve, Join, Sell and Id. Returns true when the View should end
// (an item was used in combat).
func (e *Engine) ItemsMenu() bool {
	c := e.selected()
	e.ApplyItems(c) // port: ItemCount may be stale (shop Inv for a buyer never viewed)
	key := byte(' ')
	var it *Item
	index := 0
	redraw, full := true, true
	quit := false
	for key != 0 && key != 'E' && !quit && c.ItemCount > 0 {
		count := c.ItemCount
		if len(c.Items) == 0 { // extra: a stale count with no items (the original spins here)
			break
		}
		hired := c.Control >= 0x80 && c.Okay != 0 && c.Status != statusAnimated
		menu := "Ready"
		if e.beginFlag == 5 && c.Okay != 0 {
			menu += " Throw" // extra: an unreadied throwable item, aimed at once
		}
		if c.Okay != 0 && e.area.NoMagic == 0 &&
			(e.beginFlag == 2 || e.beginFlag == 3 || e.beginFlag == 4 ||
				e.beginFlag == 5 && c.Combat != nil && c.Combat.CanUse != 0) {
			menu += " Use"
		}
		if !hired && e.beginFlag != 5 {
			menu += " Give" // extra: Trade renamed
		}
		menu += " Drop" // extra: no Halve or Join, Give takes an amount and stacks join on arrival
		if !hired && e.beginFlag == 1 {
			menu += " Sell"
		}
		menu += " Look" // extra: what the item does, as the shop's Look
		menu = cut(menu, textCols)
		list := make([]*ListNode, len(c.Items))
		for i, x := range c.Items {
			e.ItemName(c, x, 0, 0, true, false)
			list[i] = &ListNode{Text: x.Name()}
			if !e.isScroll(x) && !(x.Spells[1] > 0 && x.Spells[2] < 0x80) {
				list[i].Without = []string{"Use"} // extra: Use only for an item that has a use
			}
			if x.Readied != 0 || !e.throwableItem(x) {
				list[i].Without = append(list[i].Without, "Throw")
			}
		}
		if full || e.itemsRedraw {
			e.ClearScreen()
			e.DrawBorder(1, 1, winRight, winBottom, 0)
			e.displayName(c, 1, 1, true)
			e.DisplayString(len(c.Name())+4, 1, colLightGreen, "Items")
			e.DrawBoxTitled(1, 3, winRight, winBottom, 0, colBlack, "")
			e.DisplayString(1, 3, colWhite, "Ready Item")
			redraw, full, e.itemsRedraw = true, false, false
		}
		var sel *ListNode
		e.ext.items = true // P and F1-F8 (extras.go)
		key = e.ListMenu("", menu, colLightMagenta, colLightGreen, colWhite, 1, 5, 38, 22, list, true, &redraw, &index, &sel)
		e.ext.items = false
		it = nil
		for i, n := range list {
			if n == sel {
				it = c.Items[i]
			}
		}
		if it != nil {
			switch key {
			case 'R':
				e.readyItem(it)
			case 'U':
				if it.Readied == 0 {
					e.BottomMessage("Must be Readied")
					key = ' '
				} else if e.isScroll(it) || it.Spells[1] > 0 && it.Spells[2] < 0x80 {
					quit = e.useItem(it)
					if e.beginFlag != 5 {
						quit = false
					}
					if !quit {
						full = true
					}
				}
			case 'T':
				switch {
				case it.Readied != 0 || !e.throwableItem(it):
				case e.beginFlag == 5 && len(e.throwTargets(c, it)) == 0: // extra: nobody to throw it at
					e.BottomMessage("No target in range")
					key = ' '
				default:
					e.ext.throwing = it
					key = 'E'
				}
			case 'L':
				e.lookItem(it)
				full = true
			case 'G':
				e.tradeItem(it)
				full = true
			case 'D':
				if e.canLoseItem(it) {
					e.ItemName(c, it, 0, 0, false, false)
					e.DisplayTextBlock(1, 21, winRight, winBottom, colYellow, true, "Your "+it.Name()+"will be gone forever")
					if e.YesNo(colLightMagenta, colLightGreen, colWhite, "Drop It? ") == 'Y' {
						e.RemoveItem(e.selected(), it)
						redraw = true
					}
					e.ClearCells(1, 21, winRight, winBottom)
				} else {
					key = ' '
				}
			case 'S':
				if e.canLoseItem(it) {
					e.sellItem(it)
				} else {
					key = ' '
				}
			}
		}
		e.ApplyItems(c) // 146F: also reached with nothing selected
		if c.ItemCount != count {
			redraw = true
		}
	}
	return quit
}

// readyItem = ovr20:14D0 (item): ready or unready, with the reasons it
// cannot be readied and the item's own effect (+0x3E > 0x7F).
func (e *Engine) readyItem(it *Item) {
	effect := it.Spells[2] > 0x7F
	c := e.selected()
	if it.Readied != 0 {
		if it.Magic != 0 {
			e.BottomMessage("It's Cursed")
			return
		}
		it.Readied = 0
		if effect {
			e.itemAffect(it, c, 1)
		}
		return
	}
	why := 0
	typ := it.Type
	if int(e.base(typ).Hands)+int(c.Hands) > 2 {
		why = 3
	}
	slot := int(e.base(typ).Slot)
	switch {
	case slot <= 8:
		if c.Slots[slot] != nil {
			why = 2
		}
	case slot == 9:
		if c.Slots[slotRing2] != nil {
			why = 2
		}
	}
	if typ == itemArrow && c.Slots[slotArrows] != nil {
		why, slot = 2, slotArrows
	}
	if typ == itemQuarrel && c.Slots[slotBolts] != nil {
		why, slot = 2, slotBolts
	}
	if e.base(typ).Classes&c.ClassMask == 0 {
		why = 1
	}
	switch why {
	case 0:
		it.Readied = 1
		if effect {
			e.itemAffect(it, c, 0)
		}
	case 1:
		e.BottomMessage("Wrong Class")
	case 2:
		// slot 9 (ring 2 taken) names Slots[9], which may be empty: the
		// original then reads through a nil pointer.
		name := ""
		if o := c.Slots[slot]; o != nil {
			e.ItemName(c, o, 0, 0, false, false)
			name = o.Name()
		}
		e.BottomMessage("already using " + name)
	case 3:
		if !(e.beginFlag == 5 && c.Auto != 0) {
			e.BottomMessage("Your hands are full!")
		}
	}
}

// itemAffect calls ovr25:0000 with the item standing in for the affect
// record, as the original does.
func (e *Engine) itemAffect(it *Item, c *Character, flag byte) {
	a := &Affect{Item: it}
	// the affect fields are the item's first bytes, as in the original
	n := it.PName
	a.Type, a.Time, a.Data, a.Call = affectType(n[0]), uint16(n[1])|uint16(n[2])<<8, n[3], n[4]
	e.callAffect(affectType(it.Spells[2]), c, a, flag)
}

// tradeItem = ovr20:1734 (item). extra: a stack asks how many (all by
// default); giving all of a readied item unreadies it first (curses hold).
func (e *Engine) tradeItem(it *Item) {
	give := *it
	if it.Count > 1 {
		s := strings.TrimSpace(e.InputString("How many? ("+strconv.Itoa(int(it.Count))+") ", colLightMagenta, 3))
		n, err := strconv.Atoi(s)
		switch {
		case s == "":
		case err != nil || n < 1:
			return
		case n < int(it.Count):
			give.Count = byte(n)
			give.Readied = 0
		}
	}
	all := give.Count == it.Count
	if all && it.Readied != 0 {
		e.readyItem(it)
		if it.Readied != 0 {
			return
		}
		give.Readied = 0
	}
	if all && !e.canLoseItem(it) {
		return
	}
	to := e.view.start
	e.DrawMainScreen()
	to = e.SelectCharacter("Give to Whom?", true, to)
	if to == nil || to == e.selected() { // extra: to oneself would join and then drop the stack
		return
	}
	e.view.start = to
	if e.overloaded(&give, to) {
		e.BottomMessage("Overloaded")
		return
	}
	e.AddItemCopy(to, &give)
	if all {
		e.RemoveItem(e.selected(), it)
	} else {
		it.Count -= give.Count
	}
	e.ApplyItems(to)
}

// joinItems = ovr20:189D (item): every matching stack joins it, up to 255
// a stack. (The original compares +0x3C..+0x3E of each item with itself.)
func (e *Engine) joinItems(c *Character, it *Item) {
	total := int(it.Count)
	for _, p := range append([]*Item(nil), c.Items...) {
		if p == it || !sameStack(p, it) {
			continue
		}
		if int(p.Count)+total <= 0xFF {
			total += int(p.Count)
			e.RemoveItem(c, p)
		} else {
			it.Count = 0xFF
			p.Count -= byte(0xFF - total)
			total = int(p.Count)
			it = p
		}
	}
	it.Count = byte(total)
}

// useItem = ovr20:1A8A (item): cast the scroll's spell or the item's own
// (+0x3D, charges +0x3C). Returns the spell's "turn used" result.
func (e *Engine) useItem(it *Item) bool {
	idx := -1
	e.usingItem = false
	var spell byte
	quit := false
	if e.isScroll(it) {
		e.usedScroll = it
		var flag bool
		spell = e.SpellList(2, 1, &idx, &flag)
	} else if it.Spells[1] > 0 && it.Spells[2] < 0x80 {
		e.usingItem = true
		spell = it.Spells[1]
	}
	if spell == 0 {
		return false
	}
	c := e.selected()
	if e.beginFlag == 5 && c.Auto == 0 {
		e.combatUseHook() // 00BA:2B88
	}
	if e.usingItem {
		if spell > 0x38 {
			spell -= 0x17
		}
		e.CharMessage(c, "uses an item", false)
		if e.beginFlag == 5 {
			e.DisplayString(0, messageRow, colLightGreen, "Item:")
			e.ItemName(c, it, 5, 0x17, false, true)
		} else {
			e.ItemName(c, it, 1, 0x16, false, true)
		}
		e.MsgDelay()
		e.ClearMessageArea()
	}
	e.usingItem = true
	e.CastSpell(spell, c.Auto, 0, &quit)
	e.usingItem = false
	if e.beginFlag == 5 && e.spellRec(spell, 0xB) != 0 {
		quit = e.clearCombatInfo(c)
	}
	if quit {
		if e.isScroll(it) {
			e.scrollUsed(c, it, spell)
		} else if it.Spells[0] > 0 {
			if it.Count > 1 {
				it.Count--
			} else {
				it.Spells[0]--
				if it.Spells[0] == 0 {
					e.RemoveItem(c, it)
				}
			}
		}
	}
	return quit
}

// spellRec = DS:31FA + 16*spell.
func (e *Engine) spellRec(spell byte, k int) byte {
	return spellTable[spell][k]
}

// sellItem = ovr20:1CEE (item): half the value (a twentieth per piece
// of a stack), paid into the party's purse. extra: the halves are kept as
// copper where the original dropped them.
func (e *Engine) sellItem(it *Item) {
	v := uint32(it.Price) * 100 // copper
	if it.Count > 1 {
		// (the arrows/quarrels branch of the original can never be taken)
		v = uint32(it.Count) * v / 0x14
	}
	e.ItemName(e.viewChar(), it, 0, 0, false, false)
	e.DisplayTextBlock(1, 21, winRight, winBottom, colYellow, true,
		"I'll give you "+moneyText(v/200, v%200)+" for your "+it.Name())

	if e.YesNo(colLightMagenta, colLightGreen, colWhite, "Is It a Deal? ") == 'Y' {
		e.BottomMessage("Sold!")
		e.RemoveItem(e.selected(), it)
		e.purse.addCopper(v)
	}
	e.ClearCells(1, 21, winRight, winBottom)
}

// identifyItem = ovr20:1F52 (item, redraw): 200 gp shows the hidden words.
func (e *Engine) identifyItem(c *Character, it *Item) (redraw bool) {
	e.ItemName(c, it, 0, 0, false, false)
	if e.YesNo(colLightMagenta, colLightGreen, colWhite, "Is It a Deal? ") == 'Y' && e.pay(200) {
		if it.Hidden == 0 {
			e.DisplayTextBlock(1, 21, winRight, winBottom, colYellow, true, "I can't tell anything new about your "+it.Name())
		} else {
			it.Hidden = 0
			e.ItemName(c, it, 0, 0, false, false)
			e.DisplayTextBlock(1, 21, winRight, winBottom, colYellow, true, "It looks like some sort of "+it.Name())
			redraw = true
		}
		e.MsgDelay()
	}
	e.ClearCells(1, 21, winRight, winBottom)
	return
}

// overloaded = ovr20:275B (item, char): no room for one more item.
func (e *Engine) overloaded(it *Item, c *Character) bool {
	e.ApplyItems(c)
	over := c.ItemCount > 0xF
	w := it.Weight
	if it.Count > 0 {
		w *= uint16(it.Count)
	}
	if int32(c.Weight+w) > int32(strCarry(c))+1500 {
		over = true
	}
	return over
}

// SpellList = ovr20:299C (kind, mode, index, any): the list of spells of
// one kind (0 memory, 1 book, 2 scroll, 3 scrolls, 4 choose, 5 to be
// memorised, 6 to be scribed); returns the chosen spell or 0.
func (e *Engine) SpellList(kind, mode int, index *int, any *bool) byte {
	titles := []string{"in Memory", "in Spell Book", "on Scroll", "on Scrolls", "to choose from", "to be memorized", "to be scribed"}
	title := ""
	if kind >= 0 && kind < len(titles) {
		title = titles[kind]
	}
	*any = e.buildSpellList(kind)
	if !*any {
		return 0
	}
	if *index < 0 {
		e.DrawBoxTitled(1, 1, winRight, 1, 0, colBlack, "")
		c := e.selected()
		e.displayName(c, 1, 1, true)
		e.DisplayString(len(c.Name())+4, 1, colLightGreen, "Spells "+title)
	}
	return e.spellMenu(mode, index)
}
