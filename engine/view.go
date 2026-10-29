package engine

import (
	"image"
	"slices"
	"strconv"
	"strings"
)

// ovr20: the View screen (character sheet, items, money, spell lists).

// ClearScreen = 0709:0CAB.
func (e *Engine) ClearScreen() {
	e.fillCells(0, 0, 0x27, 0x18)
	e.clearPage()
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
	e.snap.tallRow = 13 // the remaster fills a 4:3 window: the blank row under the ability scores repeats
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
	e.DisplayString(x, y, colWhite, "Party items      ") // extra: the party inventory's slots for Encumbrance
	e.DisplayString(x+0xC, y, colLightGreen, e.poolText())
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
		e.snap.sheetOn = true
		defer func() { e.snap.sheetOn = false }()
	}
	const (
		items Choice = iota
		spellsIt
		magic
		heal
		cure
		exit
		sheet // (no item) the sheet is up
	)
	key := sheet
	quit := false
	e.view.start = e.selected()
	direct := e.ext.toItems // F1-F8: start on the inventory (extras.go)
	e.ext.toItems = false
	if direct && len(e.carried(e.selected())) > 0 {
		var leave bool
		if quit, leave = e.inventory(direct); leave {
			key = exit
		}
	}
	if key != exit && !quit {
		e.CharacterSheet()
	}
	for key != NoPick && key != exit && !quit {
		c := e.viewChar()
		var menu Bar
		if len(e.carried(c)) > 0 {
			menu = append(menu, Entry{"Items", items})
		}
		for _, p := range e.powers(c) {
			menu = append(menu, Entry{p.Name, map[ActKind]Choice{ActSpells: spellsIt, ActMagic: magic, ActHeal: heal, ActCure: cure}[p.Kind]})
		}
		menu = append(menu, Entry{"Exit", exit})
		e.ext.viewing = true // F1..F8 switch members (extras.go)
		key = e.DisplayInput("View:", menu, colLightMagenta, colLightGreen, colWhite, false, false).ID
		e.ext.viewing = false
		switch key {
		case items:
			var leave bool
			if quit, leave = e.inventory(direct); leave {
				key = exit
			}
		case spellsIt:
			quit = e.power(c, ActSpells)
		case magic:
			e.power(c, ActMagic)
		case heal:
			e.power(c, ActHeal)
		case cure:
			e.power(c, ActCure)
		}
		if !quit && key >= items && key <= cure {
			e.CharacterSheet()
		}
	}
	if e.beginFlag == 5 {
		e.combatViewLeave() // ovr31 (seg 03E0) +2A
	}
	e.snap.portrait = CellRect{} // extra: Shift-click and dropped PNGs go with the sheet (DrawMainScreen keeps the page)
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
	if !ill && !e.YesNo(colLightMagenta, colLightGreen, colWhite, t.Name()+" is not diseased, cure anyway: ") {
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

// inventory runs ItemsMenu for ViewCharacter (extra: leave reports that
// the View page should close too: the inventory was opened straight (F1-F8, I)).
func (e *Engine) inventory(direct bool) (quit, leave bool) {
	quit = e.ItemsMenu()
	return quit, !quit && direct
}

// canLoseItem = ovr20:0D70 (item): readied items must be unreadied first;
// a scroll someone is about to scribe from asks before it goes.
func (e *Engine) canLoseItem(it *Item) bool {
	ok := false
	switch {
	case it.Readied != 0:
		e.refuse("Must be unreadied")
	case !e.isScroll(it):
		ok = true
	case it.Spells[0] > 0x7F || it.Spells[1] > 0x7F || it.Spells[2] > 0x7F:
		c := e.selected()
		e.displayName(c, 1, 21, false)
		e.msgCol, e.msgRow = len(c.Name())+2, 0x15
		e.DisplayTextBlock(1, 21, winRight, winBottom, colYellow, false, " was going to scribe from that scroll")
		ok = e.YesNo(colLightMagenta, colLightGreen, colWhite, "is it Okay to lose it? ")
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
	if c.Portrait == nil && (c.PortHead == 0 || c.PortBody == 0) {
		return
	}
	box := CellRect{28, 1, winRight, 11}
	e.DrawBoxTitled(box.X1, box.Y1, box.X2, box.Y2, 0, colBlack, "")
	e.snap.portrait = box  // Shift-click and a dropped PNG land here
	if c.Portrait != nil { // extra: the dropped PNG, fitted into the box at the window's resolution
		e.pg.overlays = []Overlay{{Img: c.Portrait, At: fit(c.Portrait.Bounds(), box.Pixels())}} // the whole box: the remaster's rope runs outside its cells
		return
	}
	e.pg.overlays = nil
	e.LoadPortrait(charArt, int(portraitHead[c.PortHead]), int(portraitBody[c.PortBody]))
	e.DrawPortrait(0x1C, 1, true)
}

// fit is the largest rectangle of b's shape inside r, centred across and
// on r's bottom edge (a portrait stands on its frame's foot).
func fit(b, r image.Rectangle) image.Rectangle {
	w, h := r.Dx(), b.Dy()*r.Dx()/max(b.Dx(), 1)
	if h > r.Dy() {
		w, h = b.Dx()*r.Dy()/max(b.Dy(), 1), r.Dy()
	}
	return image.Rect(0, 0, w, h).Add(r.Min.Add(image.Pt((r.Dx()-w)/2, r.Dy()-h)))
}

// ItemsMenu = ovr20:0EF8: the character's items with Ready, Use, Trade,
// Drop, Halve, Join, Sell and Id. Returns true when the View should end
// (an item was used in combat). extra: the remastered inventory screen
// draws it (View.Inventory) and acts through Act: no Halve, Join, Trade
// or Id (unreadied items are the party's), Throw in combat, F1-F8 or the
// screen's arrows show another member outside combat; Drop and Sell come
// already confirmed.
func (e *Engine) ItemsMenu() bool {
	e.snap.invOn = true
	defer func() { e.snap.invOn = false }()
	for {
		c := e.selected()
		e.ApplyItems(c) // port: ItemCount may be stale (shop Inv for a buyer never viewed)
		items := e.carried(c)
		a := e.act()
		var it *Item
		if a.N >= 0 && a.N < len(items) {
			it = items[a.N]
		}
		switch inv := e.inventoryView(c, items); {
		case a.Kind == ActClose:
			return false
		case a.Kind == ActMember:
			if e.beginFlag != 5 { // in combat the inventory is the fighter's
				e.selectMember(a.N)
			}
		case a.Kind >= ActSpells && a.Kind <= ActCure:
			if slices.ContainsFunc(inv.Powers, func(p Power) bool { return p.Kind == a.Kind }) {
				e.snap.invOn = false // they show on the game's own screens
				quit := e.power(c, a.Kind)
				e.snap.invOn = true
				if quit && e.beginFlag == 5 {
					return true
				}
			}
		case a.Kind == ActBuy && e.snap.shop: // N is a ware, not one of items
			if a.N >= 0 && a.N < len(e.run.treasure) {
				w := e.run.treasure[a.N]
				switch {
				case uint32(w.Price) > e.purse.gold:
					e.refuse("Not enough Money.")
				case !e.buyItem(w):
					e.purse.gold -= uint32(w.Price)
					e.say("Bought!")
				}
			}
		case it == nil:
		case a.Kind == ActReady:
			e.swapReady(c, it)
		case a.Kind == ActUse && inv.Use && e.usable(it):
			if it.Readied == 0 && dollSlot(e, c, it) >= 0 { // extra: items with no slot are used from the party inventory
				e.refuse("Must be Readied")
				break
			}
			e.snap.invOn = false // its targets and spells show on the game's own screens
			quit := e.useItem(it)
			e.snap.invOn = true
			if quit && e.beginFlag == 5 {
				return true
			}
		case a.Kind == ActDrop:
			if e.canLoseItem(it) {
				e.RemoveItem(c, it)
			}
		case a.Kind == ActSell && inv.Sell:
			if e.canLoseItem(it) {
				e.RemoveItem(c, it)
				e.purse.addCopper(sellValue(it))
				e.say("Sold!")
			}
		}
		e.ApplyItems(c) // 146F: also reached with nothing selected
	}
}

// usable: the item has a Use (a scroll, or a spell of its own).
func (e *Engine) usable(it *Item) bool {
	return e.isScroll(it) || it.Spells[1] > 0 && it.Spells[2] < 0x80
}

// swapReady readies it in place of the item in its slot (extra: a swap,
// not readyItem's "already using"), else readies or unreadies it.
func (e *Engine) swapReady(c *Character, it *Item) {
	if o := e.inTheWay(c, it); o != nil && e.base(it.Type).Classes&c.ClassMask != 0 {
		e.readyItem(o)
		if o.Readied != 0 { // cursed, or no room for it
			return
		}
		e.ApplyItems(c) // the slot and hands free
	}
	e.readyItem(it)
}

// missileSlot (extra): readied now, the unreadied thrown weapon it would
// go into the Missiles slot (the arrows', Readied 2), as a weapon is in hand.
func (e *Engine) missileSlot(c *Character, it *Item) bool {
	return it.Readied == 0 && e.throwableItem(it) && c.Slots[slotWeapon] != nil && c.Slots[slotWeapon] != it
}

// missile (extra) is what c's Ranged action shoots: the thrown weapon in the
// Missiles slot, the ammunition the launcher in hand takes, or a launcher
// that needs none (a sling); nil when there is nothing to shoot.
func (e *Engine) missile(c *Character) *Item {
	if m := c.Slots[slotArrows]; m != nil && m.Readied == 2 {
		return m
	}
	w := c.Slots[slotWeapon]
	var ammo *Item
	if w == nil || e.base(w.Type).Usage&8 == 0 || !e.ammoFor(c, &ammo) { // not a launcher, or without its ammunition
		return nil
	}
	if ammo == nil {
		return w
	}
	return ammo
}

// inTheWay is the readied item of c whose slot it would take (readyItem's
// "already using"), nil for none.
func (e *Engine) inTheWay(c *Character, it *Item) *Item {
	if it.Readied != 0 {
		return nil
	}
	slot := int(e.base(it.Type).Slot)
	switch {
	case it.Type == itemArrow || e.missileSlot(c, it):
		slot = slotArrows
	case it.Type == itemQuarrel:
		slot = slotBolts
	case it.Type == itemBracers:
		for _, o := range c.Items {
			if o != it && o.Type == itemBracers && o.Readied != 0 {
				return o
			}
		}
		return nil
	case slot == 9:
		slot = slotRing2
	case slot > 8:
		return nil
	}
	return c.Slots[slot]
}

// readyItem = ovr20:14D0 (item): ready or unready, with the reasons it
// cannot be readied and the item's own effect (+0x3E > 0x7F).
func (e *Engine) readyItem(it *Item) {
	effect := it.Spells[2] > 0x7F
	c := e.selected()
	if it.Readied != 0 {
		if it.Magic != 0 {
			e.refuse("It's Cursed")
			return
		}
		if e.partyMember(c) && e.poolFull(it) && !(e.beginFlag == 5 && c.Auto != 0) { // extra: it goes back into the party inventory; the computer's turn never stalls on a full one (it overflows, addToPool)
			e.refuse("No room in the party items")
			return
		}
		it.Readied = 0
		if effect {
			e.itemAffect(it, c, 1)
		}
		return
	}
	if dollSlot(e, c, it) < 0 { // extra: used straight from the party inventory
		e.refuse("Nothing to ready")
		return
	}
	why := 0
	typ := it.Type
	missile := e.missileSlot(c, it) // extra: a thrown weapon beside the weapon in hand: the Missiles slot, no hands
	if int(e.base(typ).Hands)+int(c.Hands) > 2 && !missile {
		why = 3
	}
	slot := int(e.base(typ).Slot)
	if missile {
		slot = slotArrows
	}
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
	if typ == itemArrow || typ == itemQuarrel || missile { // the Missiles slot (extra) holds one of arrows, quarrels or a thrown weapon
		for _, s := range []int{slotArrows, slotBolts} {
			if c.Slots[s] != nil {
				why, slot = 2, s
			}
		}
	}
	if typ == itemBracers { // extra: the bracers have a doll slot of their own
		for _, o := range c.Items {
			if o != it && o.Type == itemBracers && o.Readied != 0 {
				e.ItemName(c, o, 0, 0, false, false)
				e.refuse("already using " + o.Name())
				return
			}
		}
	}
	if e.base(typ).Classes&c.ClassMask == 0 {
		why = 1
	}
	switch why {
	case 0:
		e.takeFromPool(c, it) // extra: out of the party inventory
		it.Readied = 1
		if missile {
			it.Readied = 2
		}
		if effect {
			e.itemAffect(it, c, 0)
		}
	case 1:
		e.refuse("Wrong Class")
	case 2:
		// slot 9 (ring 2 taken) names Slots[9], which may be empty: the
		// original then reads through a nil pointer.
		name := ""
		if o := c.Slots[slot]; o != nil {
			e.ItemName(c, o, 0, 0, false, false)
			name = o.Name()
		}
		e.refuse("already using " + name)
	case 3:
		if !(e.beginFlag == 5 && c.Auto != 0) {
			e.refuse("Your hands are full!")
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
			e.ItemName(c, it, 0, 0, false, false)
			e.combatSay(nil, "Item: "+it.Name(), false)
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

// sellValue = ovr20:1CEE (item): what selling it pays, in copper (200 a
// gold piece, so half the value; a twentieth per piece of a stack).
// extra: the halves are kept as copper where the original dropped them.
func sellValue(it *Item) uint32 {
	v := uint32(it.Price) * 100
	if it.Count > 1 {
		// (the arrows/quarrels branch of the original can never be taken)
		v = uint32(it.Count) * v / 0x14
	}
	return v
}

// identifyItem = ovr20:1F52 (item, redraw): 200 gp shows the hidden words.
func (e *Engine) identifyItem(c *Character, it *Item) (redraw bool) {
	e.ItemName(c, it, 0, 0, false, false)
	if e.YesNo(colLightMagenta, colLightGreen, colWhite, "Is It a Deal? ") && e.pay(200) {
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
// extra: a party member takes it into the party inventory, which has a
// slot count instead of the original's 16 items and weight allowance.
func (e *Engine) overloaded(it *Item, c *Character) bool {
	e.ApplyItems(c)
	if e.partyMember(c) {
		return e.poolFull(it)
	}
	return c.ItemCount > 0xF
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

// powers is what c can do besides items: the View menu's Spells, Magic,
// Heal and Cure (the inventory offers them too).
func (e *Engine) powers(c *Character) []Power {
	var ps []Power
	if slices.ContainsFunc(c.Memorized[:0x15], func(s byte) bool { return s > 0 }) {
		ps = append(ps, Power{"Spells", ActSpells})
	}
	if !e.effectsFit(c) {
		ps = append(ps, Power{"Magic", ActMagic}) // extra: all of the spell effects
	}
	if e.paladinCan(c, affLaidOnHands) {
		ps = append(ps, Power{"Heal", ActHeal})
	}
	if e.paladinCan(c, affCuredDisease) {
		ps = append(ps, Power{"Cure", ActCure})
	}
	return ps
}

// memberPowers (extra) is a card's context menu out of combat: View, Items
// when c carries any, then powers.
func (e *Engine) memberPowers(c *Character) []Power {
	if e.beginFlag == 5 {
		return nil
	}
	ps := []Power{{"View", ActMember}}
	if len(e.carried(c)) > 0 {
		ps = append(ps, Power{"Items", ActInventory})
	}
	return append(ps, e.powers(c)...)
}

// power carries out one of c's powers; quit is true when a cast in combat
// used the turn.
func (e *Engine) power(c *Character, k ActKind) (quit bool) {
	switch k {
	case ActSpells: // extra: the list casts (the original's View only shows it; its Cast commands cast for the selected member)
		if e.beginFlag != 5 {
			e.selectMember(slices.Index(e.party, c))
			e.CastMenu()
			return false
		}
		if c.Combat == nil || c.Combat.CanCast == 0 || e.area.NoMagic != 0 { // the bar offers no Cast either
			e.refuse("Cannot cast now")
			return false
		}
		if spell := e.pickSpell(); spell != 0 { // as the bar's Cast: aim, then the turn is over
			e.castSpell(int(spell), 0, &quit)
		}
	case ActMagic:
		e.displayAffects("Magic", []*Character{c})
	case ActHeal:
		e.paladinHeal(c)
	case ActCure:
		e.paladinCure(c)
	}
	return quit
}

// pickSpell (extra) offers the selected member's memorized spells as
// View.SpellPick (the combat's Cast list, on any screen) and waits for the
// pick: the spell, or 0 for ActClose. The spell page (SpellList) is left to
// memorizing and scribing.
func (e *Engine) pickSpell() byte {
	defer func() { e.snap.spellPick = nil }()
	if !e.buildSpellList(0) {
		return 0
	}
	for _, id := range e.spellIDs {
		if !slices.ContainsFunc(e.snap.spellPick, func(s Spell) bool { return s.ID == int(id) }) { // memorized twice: once in the list
			e.snap.spellPick = append(e.snap.spellPick, Spell{int(id), e.spellName(int(id))})
		}
	}
	for {
		switch a := e.act(); {
		case a.Kind == ActCast && slices.ContainsFunc(e.snap.spellPick, func(s Spell) bool { return s.ID == a.N }):
			return byte(a.N)
		case a.Kind == ActClose:
			return 0
		}
	}
}
