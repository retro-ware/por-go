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
	quit  bool       // an item or spell used in combat took the turn: the sheet and inventory close
	leave bool       // the sheet was opened on the inventory (F1-F8): leaving that leaves it
}

// CharacterSheet = ovr20:004A: the selected member's sheet (View.SheetPage,
// laid out by the presenter).
func (e *Engine) CharacterSheet() {
	c := e.selected()
	e.view.char = c
	e.ClearScreen()
	col := colLightRed // displayName
	if c.Okay != 0 {
		col = colLightCyan
		if c.Side == sideMonsters {
			col = colYellow
		}
	}
	sp := &SheetPage{Name: c.Name(), NameColour: col, NPC: c.Control > 0x7F,
		Sex: genderNames[c.Sex], Race: raceNames[int8(c.Race)], Age: int(c.Age),
		Alignment: alignmentNames[int8(c.Alignment)], Class: className(c.Class), Str00: str00Text(c), Creating: e.ext.creating}
	for i, v := range c.Stats {
		sp.Stats[i] = int(v)
	}
	for _, l := range []Coin{{"Gold", int(e.purse.gold)}, {"Copper", int(e.purse.copper)}, {"Gems", int(e.purse.gems)}, {"Jewelry", int(e.purse.jewelry)}} { // showMoney
		if l.N != 0 {
			sp.Money = append(sp.Money, l)
		}
	}
	e.snap.sheet = sp
	e.combatStats(c, sp)
	if !e.ext.creating { // extra: the creation's sheet: no level, experience or status yet (create.go)
		for i := 0; i <= 7; i++ {
			if l := int8(c.Levels[i]); l > 0 {
				if sp.Levels != "" {
					sp.Levels += "/"
				}
				sp.Levels += strconv.Itoa(int(l))
			}
		}
		sp.Exp = int(int32(c.Exp))
		if _, need := levelSpan(c); need > 0 { // extra: the experience the next level takes, green once it is there
			sp.Next = int(need)
		}
		if w := c.Slots[slotWeapon]; w != nil {
			e.ItemName(c, w, 0, 0, false, false)
			sp.Weapon = w.Name()
		}
		if a := c.Slots[slotArmor]; a != nil {
			e.ItemName(c, a, 0, 0, false, false)
			sp.Armor = a.Name()
		}
		sp.Status = e.statusName(c)
		sp.Effects, _ = e.effectsLine(c)
	}
	if e.viewBit0 != 0 { // the original: only while set 3 is loaded (its HEAD/BODY); extra: everywhere, from charArt
		e.SheetPortrait()
	}
}

// Extra: the sheet lists the spell effects on the character (the camp
// menu's Display, which is not offered) on the free row under HP. When they
// do not all fit, the View menu offers Magic with the whole list.
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

// combatStats = showCombatStats (ovr20:0608): AC, HP, THAC0, damage,
// party items, movement.
func (e *Engine) combatStats(c *Character, sp *SheetPage) {
	e.ApplyItems(c)
	sp.AC, sp.HP, sp.MaxHP = 60-int(c.AC[0]), int(c.HP), int(c.MaxHP)
	sp.THAC0, sp.Damage = int(byte(0x3C-c.THAC0)), cut(damageText(c), textCols)
	if !e.ext.creating {
		sp.PartyItems = e.poolText() // extra: the party inventory's slots for Encumbrance
	}
	m := c.Move
	// extra: the original (ovr20:086A) doubles under Slow and halves under Haste
	if e.FindAffect(c, affHaste) != nil {
		m <<= 1
	}
	if e.FindAffect(c, affSlow) != nil {
		m >>= 1
	}
	sp.Move = int(m)
}

// str00Text is the exceptional strength shown after an 18: "(57)", "(00)".
func str00Text(c *Character) string {
	if c.Stats[0] != 0x12 || c.Str00 == 0 {
		return ""
	}
	s := strconv.Itoa(int(c.Str00))
	if c.Str00 < 10 {
		s = "0" + s
	}
	if c.Str00 == 100 {
		s = "00"
	}
	return "(" + s + ")"
}

// pushSheet = ovr20:0AB3 (var quit), the View command: it puts the sheet
// of the selected member up (View.Sheet).
func (e *Engine) pushSheet() { e.pushSheetOn(false) }

// pushSheetOn is pushSheet; direct (F1-F8) opens it on the inventory, and
// leaving that leaves it.
func (e *Engine) pushSheetOn(direct bool) {
	if e.beginFlag == 5 {
		e.combatViewEnter() // ovr31 (seg 03E0) +2F
		e.snap.sheetOn = true
	}
	e.view.start, e.view.quit, e.view.leave = e.selected(), false, false
	off := e.redrawWith(e.CharacterSheet)
	e.push(modeSheet, e.applySheet, func() {
		off()
		e.snap.sheetOn = false
		if e.beginFlag == 5 {
			e.combatViewLeave() // ovr31 (seg 03E0) +2A
		}
		e.snap.portrait = CellRect{} // extra: Shift-click and dropped PNGs go with the sheet (DrawMainScreen keeps the page)
		e.DrawMainScreen()
	})
	if direct && len(e.carried(e.selected())) > 0 {
		e.view.leave = true
		e.pushItems()
		return
	}
	e.CharacterSheet() // Shift-click and dropped PNGs change the portrait (viewing)
}

// applySheet applies a decision on the sheet; done when it is left.
func (e *Engine) applySheet(a Action) (done bool) {
	c := e.viewChar()
	if c == nil {
		c = e.selected()
	}
	switch a.Kind {
	case ActClose:
		return true
	case actRedraw: // back from the inventory or a power
		if e.view.quit || e.view.leave {
			return true
		}
	case ActMember:
		if e.beginFlag == 5 || a.N < 0 || a.N >= len(e.party) { // in combat the sheet is the fighter's
			return false
		}
		e.selectMember(a.N)
	case ActInventory, ActOverview:
		if a.Kind == ActInventory && a.N >= 0 && a.N < len(e.party) && e.beginFlag != 5 {
			e.selectMember(a.N)
			c = e.selected()
		}
		if len(e.carried(c)) > 0 {
			e.pushItems()
			return false
		}
	case ActSpells, ActMagic, ActHeal, ActCure:
		if !slices.ContainsFunc(e.powers(c), func(p Power) bool { return p.Kind == a.Kind }) {
			return false
		}
		e.power(c, a.Kind, func(quit bool) {
			if quit {
				e.view.quit = true // the sheet goes on its actRedraw (or at once)
			}
		})
		if e.view.quit {
			return true
		}
		if e.mode() != modeSheet { // the power's screens are up: the sheet draws itself again on their actRedraw
			return false
		}
	default:
		return false
	}
	e.CharacterSheet()
	return false
}

// sheetView is the sheet's decisions for the View.
func (e *Engine) sheetView() *Sheet {
	c := e.selected()
	if c == nil {
		return nil
	}
	return &Sheet{Items: len(e.carried(c)) > 0, Powers: e.powers(c)}
}

// Paladin powers from CotAB (ovr020). Two port-only affect types time them
// (affLaidOnHands, affCuredDisease): one heal a day, one cure disease a week
// (one is all a paladin of PoR's level 8 cap gets).

// paladinCan = CotAB CanCastHeal/CanCastCureDiseases: out of combat, okay,
// and the power not used yet.
func (e *Engine) paladinCan(c *Character, used affectType) bool {
	return int8(c.Levels[clsPaladin]) > 0 && e.beginFlag != 5 && c.Status == statusOkay && e.FindAffect(c, used) == nil
}

// paladinHeal = CotAB PaladinHeal: lay on hands for 2 hp per level; then.
func (e *Engine) paladinHeal(c *Character, then func()) {
	e.SelectCharacter("Heal whom? ", true, c, func(t *Character) {
		if t == nil {
			then()
			return
		}
		if e.Heal(t, 2*int(c.Levels[clsPaladin]), true) {
			e.DisplayMessageWait(colLightGreen, t.Name()+" feels better")
		} else {
			e.DisplayMessageWait(colLightGreen, t.Name()+" is unaffected")
		}
		e.AddAffect(c, affLaidOnHands, 1440, 0, 0)
		then()
	})
}

// paladinCure = CotAB PaladinCureDisease: the temple's diseases go; then.
func (e *Engine) paladinCure(c *Character, then func()) {
	e.SelectCharacter("Cure whom? ", true, c, func(t *Character) {
		if t == nil {
			then()
			return
		}
		ill := false
		for _, d := range diseases {
			ill = ill || e.FindAffect(t, d) != nil
		}
		cure := func(yes bool) {
			if !yes {
				then()
				return
			}
			e.rv.cure = 1
			for _, d := range diseases {
				e.RemoveAffect(t, d, nil)
			}
			e.rv.cure = 0
			e.AddAffect(c, affCuredDisease, 0x2760, 0, 0)
			e.DisplayMessageWait(colLightGreen, t.Name()+" is cured")
			then()
		}
		if ill {
			cure(true)
			return
		}
		e.YesNo(colLightMagenta, colLightGreen, colWhite, t.Name()+" is not diseased, cure anyway: ", cure)
	})
}

// canLoseItem = ovr20:0D70 (item): readied items must be unreadied first.
// (Its question for a scroll someone was going to scribe from is the
// front end's: InvItem.Scribe.)
func (e *Engine) canLoseItem(it *Item) bool {
	if it.Readied != 0 {
		e.refuse("Must be unreadied")
		return false
	}
	return true
}

// scribing: someone was going to scribe from the scroll (ovr20:0D70).
func (e *Engine) scribing(it *Item) bool {
	return e.isScroll(it) && (it.Spells[0] > 0x7F || it.Spells[1] > 0x7F || it.Spells[2] > 0x7F)
}

// SheetPortrait = ovr29:04F9: the character's head and body in a box at
// the top right of the sheet (SheetPage.PortraitAt; the presenter frames it).
func (e *Engine) SheetPortrait() {
	c := e.selected()
	if c.Portrait == nil && (c.PortHead == 0 || c.PortBody == 0) {
		return
	}
	box := CellRect{winRight - 10, 1, winRight, 11} // against the right border, whatever the page's width
	e.ClearCells(box.X1, box.Y1, box.X2, box.Y2)
	e.snap.portrait = box // Shift-click and a dropped PNG land here
	if sp := e.snap.sheet; sp != nil {
		sp.PortraitAt, sp.Portrait = box, c.Portrait // extra: the dropped PNG, fitted into the box at the window's resolution
	}
	if c.Portrait != nil {
		return
	}
	e.LoadPortrait(charArt, int(portraitHead[c.PortHead]), int(portraitBody[c.PortBody]))
	e.DrawPortrait(box.X1, 1, true)
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

// ItemsMenu (ovr20:0EF8, pushItems): the character's items with Ready, Use, Trade,
// Drop, Halve, Join, Sell and Id. view.quit is set when the View should end
// (an item was used in combat). extra: the remastered inventory screen
// draws it (View.Inventory) and takes decisions (applyItems): no Halve,
// Join, Trade or Id (unreadied items are the party's), Throw in combat,
// F1-F8 or the screen's arrows show another member outside combat; Drop
// and Sell come already confirmed. pushItems puts the selected member's
// inventory up.
func (e *Engine) pushItems() {
	e.snap.invOn = true
	e.ApplyItems(e.selected()) // port: ItemCount may be stale (shop Inv for a buyer never viewed)
	e.push(modeItems, e.applyItems, func() { e.snap.invOn = false })
}

// itemsUsed: a power or an item used from the inventory is done (quit:
// in combat it took the turn, and the inventory goes: view.quit).
func (e *Engine) itemsUsed(quit bool) {
	e.snap.invOn = true
	if quit && e.beginFlag == 5 {
		e.view.quit = true
	}
}

// applyItems applies a decision in the inventory; done when it is left
// (or an item used in combat took the turn: view.quit).
func (e *Engine) applyItems(a Action) (done bool) {
	c := e.selected()
	items := e.carried(c)
	var it *Item
	if a.N >= 0 && a.N < len(items) {
		it = items[a.N]
	}
	defer e.ApplyItems(c) // 146F: also reached with nothing selected
	switch inv := e.inventoryView(c, items); {
	case a.Kind == ActClose:
		return true
	case a.Kind == actRedraw: // back from a power or a use: in combat one that took the turn ends it
		return e.view.quit
	case a.Kind == ActMember:
		if e.beginFlag != 5 { // in combat the inventory is the fighter's
			e.selectMember(a.N)
			e.ApplyItems(e.selected())
		}
	case a.Kind >= ActSpells && a.Kind <= ActCure:
		if slices.ContainsFunc(inv.Powers, func(p Power) bool { return p.Kind == a.Kind }) {
			e.snap.invOn = false // they show on the game's own screens
			e.power(c, a.Kind, e.itemsUsed)
			return e.view.quit
		}
	case a.Kind == ActTakeAll && e.snap.loot: // the Loot screen's All: each piece in turn, until one does not fit
		for len(e.run.treasure) > 0 && !e.buyItem(e.run.treasure[0]) {
			e.run.treasure = e.run.treasure[1:]
		}
		return len(e.run.treasure) == 0
	case a.Kind == ActBuy && e.snap.shop: // N is a ware, not one of items
		if a.N >= 0 && a.N < len(e.run.treasure) {
			w := e.run.treasure[a.N]
			switch {
			case e.snap.loot: // the Loot screen's Take (lootTake)
				if !e.buyItem(w) {
					e.run.treasure = slices.Delete(e.run.treasure, a.N, a.N+1)
					return len(e.run.treasure) == 0
				}
			case uint32(w.Price) > e.purse.gold:
				e.refuse("Not enough Money.")
			case !e.buyItem(w):
				e.purse.gold -= uint32(w.Price)
				e.dasp("store5")
				e.say("Bought!")
			}
		}
	case it == nil:
	case a.Kind == ActReady:
		e.readySome(c, it, a.X) // X: that many of the stack (0: all)
	case a.Kind == ActUse && inv.Use && e.usable(it):
		if it.Readied == 0 && dollSlot(e, c, it) >= 0 { // extra: items with no slot are used from the party inventory
			e.refuse("Must be Readied")
			break
		}
		e.snap.invOn = false // its targets and spells show on the game's own screens
		e.useThen(it, e.itemsUsed)
		return e.view.quit
	case a.Kind == ActDrop:
		if e.canLoseItem(it) {
			e.dasp("drop")
			e.RemoveItem(c, it)
		}
	case a.Kind == ActSell && inv.Sell:
		if e.canLoseItem(it) {
			e.RemoveItem(c, it)
			e.purse.addCopper(sellValue(it))
			e.dasp("store5")
			e.say("Sold!")
		}
	}
	return false
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

// readySome (extra): readies or unreadies n of a stack (n 0 or the whole
// stack: all of it, swapReady), the rest staying where it was; a part
// readied beside a like readied stack joins it (arrows), so does one
// unreadied beside a like stack in the party inventory (addToPool).
func (e *Engine) readySome(c *Character, it *Item, n int) {
	if n <= 0 || n >= int(it.Count) || it.Count == 0 || it.Spells[0] >= 2 {
		if o := e.inTheWay(c, it); it.Readied == 0 && o != nil && sameStack(o, it) && int(o.Count)+int(it.Count) <= 0xFF {
			o.Count += it.Count // a whole stack beside a like readied one: it joins
			e.RemoveItem(c, it)
			return
		}
		e.swapReady(c, it)
		return
	}
	part := *it
	part.Count = byte(n)
	it.Count -= byte(n)
	if it.Readied != 0 { // n of them go back to the party inventory
		part.Readied = 0
		if e.poolFull(&part) {
			it.Count += byte(n)
			e.refuse("No room in the party items")
			return
		}
		e.addToPool(&part)
		return
	}
	if o := e.inTheWay(c, &part); o != nil && sameStack(o, &part) && int(o.Count)+n <= 0xFF {
		o.Count += byte(n)
		return
	}
	e.pool = append(e.pool, &part) // n of them are readied: the part is an item of its own
	e.swapReady(c, &part)
	if part.Readied == 0 { // refused: back into the stack
		e.pool = slices.DeleteFunc(e.pool, func(p *Item) bool { return p == &part })
		it.Count += byte(n)
	}
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
	slot := e.readySlot(it.Type)
	switch {
	case it.Type == itemArrow || e.missileSlot(c, it):
		slot = slotArrows
	case it.Type == itemQuarrel:
		slot = slotBolts
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
		e.dasp("wield")
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
	slot := e.readySlot(typ)
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
	if e.base(typ).Classes&c.ClassMask == 0 {
		why = 1
	}
	switch why {
	case 0:
		e.takeFromPool(c, it) // extra: out of the party inventory
		e.dasp("wield")
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

// useThen = ovr20:1A8A (item): cast the scroll's spell or the item's own
// (+0x3D, charges +0x3C), the spell picked and the cast's questions asked
// first (castAsks); then gets the spell's "turn used" result.
func (e *Engine) useThen(it *Item, then func(quit bool)) {
	e.usingItem = false
	use := func(spell byte) {
		if spell == 0 {
			then(false)
			return
		}
		s := spell
		if e.usingItem && s > 0x38 {
			s -= 0x17
		}
		item := e.usingItem
		e.castAsks(s, e.selected().Auto, true, func() {
			e.usingItem = item
			then(e.useItem(it, spell))
		})
	}
	switch {
	case e.isScroll(it):
		e.usedScroll = it
		idx := -1
		var flag bool
		e.SpellList(2, 1, &idx, &flag, use)
	case it.Spells[1] > 0 && it.Spells[2] < 0x80:
		e.usingItem = true
		use(it.Spells[1])
	default:
		use(0)
	}
}

// useItem is ovr20:1A8A once spell is picked (usingItem: the item's own):
// it casts it; true when the turn is used.
func (e *Engine) useItem(it *Item, spell byte) bool {
	quit := false
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
	e.ext.castCancelled = false // a cancelled aim: nothing used, the inventory stays
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

// identifyItem = ovr20:1F52 (item): 200 gp shows the hidden words (the
// player took the deal).
func (e *Engine) identifyItem(c *Character, it *Item) {
	if !e.pay(200) {
		return
	}
	e.dasp("pseudo_id")
	e.ItemName(c, it, 0, 0, false, false)
	if it.Hidden == 0 {
		e.say("I can't tell anything new about your " + it.Name())
		return
	}
	it.Hidden = 0
	e.ItemName(c, it, 0, 0, false, false)
	e.say("It looks like some sort of " + it.Name())
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
// memorised, 6 to be scribed); then gets the chosen spell or 0.
func (e *Engine) SpellList(kind, mode int, index *int, any *bool, then func(byte)) {
	titles := []string{"in Memory", "in Spell Book", "on Scroll", "on Scrolls", "to choose from", "to be memorized", "to be scribed"}
	title := ""
	if kind >= 0 && kind < len(titles) {
		title = titles[kind]
	}
	*any = e.buildSpellList(kind)
	if !*any {
		then(0)
		return
	}
	if sp := e.snap.sheet; *index < 0 && sp != nil { // over the sheet's name row (the presenter's)
		sp.Spells, sp.Portrait = "Spells "+title, nil
		e.snap.portrait = CellRect{}
	}
	e.spellMenu(mode, index, then)
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

// power carries out one of c's powers; then gets true when a cast in
// combat used the turn.
func (e *Engine) power(c *Character, k ActKind, then func(quit bool)) {
	done := func() { then(false) }
	switch k {
	case ActSpells: // extra: the list casts (the original's View only shows it; its Cast commands cast for the selected member)
		if e.beginFlag != 5 {
			e.selectMember(slices.Index(e.party, c))
			e.after(e.CastMenu, done)
			return
		}
		if c.Combat == nil || c.Combat.CanCast == 0 || e.area.NoMagic != 0 { // the bar offers no Cast either
			e.refuse("Cannot cast now")
			done()
			return
		}
		e.pickSpell(func(spell byte) {
			if spell == 0 {
				done()
				return
			}
			cast := func() { // as the bar's Cast: aim, then the turn is over
				var quit bool
				e.castSpell(int(spell), 0, &quit)
				then(quit)
			}
			if e.spellData(int(spell), 6)/3 == 0 { // cast now: aimed first (castSpell only begins a longer one)
				e.aimSpell(c, int(spell), cast)
			} else {
				cast()
			}
		})
	case ActMagic:
		e.displayAffects("Magic", []*Character{c}, done)
	case ActHeal:
		e.paladinHeal(c, done)
	case ActCure:
		e.paladinCure(c, done)
	default:
		done()
	}
}

// pickSpell (extra) offers the selected member's memorized spells as
// View.Ask (AskSpell) (the combat's Cast list, on any screen); then gets the
// pick: the spell, or 0 for ActClose. The spell page (SpellList) is left to
// memorizing and scribing.
func (e *Engine) pickSpell(then func(byte)) {
	if !e.buildSpellList(0) {
		then(0)
		return
	}
	var spells []Spell
	for _, id := range e.spellIDs {
		if !slices.ContainsFunc(spells, func(s Spell) bool { return s.ID == int(id) }) { // memorized twice: once in the list
			spells = append(spells, Spell{int(id), e.spellName(int(id))})
		}
	}
	var id byte
	e.asking(func() { // an ask (AskSpell): the presenter shows the pick
		e.eclWait = &eclWait{ask: Ask{Kind: AskSpell, Spells: spells}, apply: func(a Action) bool {
			switch {
			case a.Kind == ActCast && slices.ContainsFunc(spells, func(s Spell) bool { return s.ID == a.N }):
				id = byte(a.N)
				return true
			case a.Kind == ActClose:
				return true
			}
			return false
		}}
	}, func() { then(id) })
}
