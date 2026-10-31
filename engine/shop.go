package engine

import "strconv"

// The shop, ported from ovr07. ECL COMBAT with the shop flag (game +0x6D8)
// set runs Shop instead of a fight; the goods are the treasure list.

// shopBrowse (extra; the original's Buy list, ovr07:0361): the inventory
// screen with the goods beside the party inventory; ActBuy buys one into
// the party inventory, paid from the party's gold, Sell sells.
func (e *Engine) shopBrowse() {
	for _, it := range e.run.treasure {
		if it.Price == 0 {
			it.Price = 1
		}
	}
	e.snap.shop = true
	e.pushItems()
	items := &e.screens[len(e.screens)-1]
	leave := items.leave
	items.leave = func() { leave(); e.snap.shop = false }
}

// shopWares adds the goods and the party's money to the inventory view.
func (e *Engine) shopWares(v *Inventory, c *Character) {
	v.Gold, v.Copper = int(e.purse.gold), int(e.purse.copper)
	v.Wares = []InvItem{}
	for i, it := range e.run.treasure {
		e.ItemName(nil, it, 0, 0, false, false)
		x := InvItem{Index: i, Name: it.Name(), Type: byte(it.Type), Count: int(it.Count), Slot: dollSlot(e, c, it) >= 0,
			Wrong: !e.canUseItem(c, it), Look: e.itemLines(it, e.readiedLike(c, it)), Price: strconv.Itoa(int(it.Price)) + " gold",
			Kind: itemKind(e, c, it)}
		if e.snap.loot {
			x.Price = ""
		}
		for _, w := range it.Words {
			if w != 0 {
				x.Words = append(x.Words, itemWords[w])
			}
		}
		v.Wares = append(v.Wares, x)
	}
	e.sortItems(v.Wares, e.run.treasure)
}

// buyItem = ovr07:0239 (item, var overloaded): a copy goes to the
// selected character unless it is too much to carry.
func (e *Engine) buyItem(it *Item) (overloaded bool) {
	c := e.selected()
	if e.overloaded(it, c) {
		e.refuse("No room in the party items") // extra: over the inventory screen (the original: OverLoaded)
		return true
	}
	e.AddItemCopy(c, it)
	e.ApplyItems(c)
	return false
}

// Shop = ovr07:053E: the shop takes decisions until the party leaves
// (it puts the shop up and returns).
func (e *Engine) Shop() {
	greeting := e.cellsCopy() // extra: the merchant's greeting comes back after each redraw (the original cleared it)
	e.beginFlag = 1
	e.DrawMainScreen()
	e.keepPane(greeting)
	e.DisplayPartyList()
	for _, it := range e.run.treasure {
		e.ItemName(nil, it, 0, 0, false, false)
	}
	e.dasp("store_enter")
	e.push(modeShop, func(a Action) bool {
		done, redraw := e.applyShop(a)
		if redraw {
			e.DrawMainScreen()
			e.keepPane(greeting)
		}
		e.DisplayPartyList()
		return done
	}, func() { e.dasp("store_leave") })
}

// applyShop applies a decision in the shop (extra: Buy is Browse, the
// goods beside the inventory; Id is the shop's, not the item menu's);
// done when the party leaves.
func (e *Engine) applyShop(a Action) (done, redraw bool) {
	switch a.Kind {
	case ActClose:
		e.sellValued()
		return true, false
	case ActLead:
		e.actor(a.N)
	case ActInventory:
		e.actor(a.N)
		e.shopBrowse()
	case actRedraw: // back from the goods
		return false, true
	case ActAppraise, ActKeep:
		e.applyAppraise(a)
	case ActIdentify:
		if a.N < 0 || a.N >= len(e.party) {
			break
		}
		c := e.party[a.N]
		if own := e.identifiable(a.N); a.X >= 0 && a.X < len(own) && unknown(own[a.X]) {
			e.identifyItem(c, own[a.X])
		}
	}
	return false, false
}

// identifiable is member i's items Id looks at (extra: the party
// inventory with the first member).
func (e *Engine) identifiable(i int) []*Item {
	if i == 0 {
		return e.carried(e.party[0])
	}
	return e.party[i].Items
}

// shopView is the shop's state for the View: the party's unknown items
// under their owners' names, only those Id tells something about.
func (e *Engine) shopView() *Shop {
	v := &Shop{IDPrice: 200, Gold: int(e.purse.gold), Appraise: e.appraiseView()}
	for i, c := range e.party {
		for k, it := range e.identifiable(i) {
			if unknown(it) {
				e.ItemName(c, it, 0, 0, false, false)
				v.Unknown = append(v.Unknown, Unknown{Member: i, Item: k, Owner: c.Name(), Name: it.Name()})
			}
		}
	}
	return v
}

// keepPane (extra) puts the text pane's cells back as they were in cells.
func (e *Engine) keepPane(cells [][]textCell) {
	for y := msgTop; y <= winBottom; y++ {
		for x := 1; x <= winRight; x++ {
			if y >= len(cells) || x >= len(cells[y]) {
				continue
			}
			if c := cells[y][x]; c.set {
				e.drawChar(x, y, int(c.colour), int(c.bg), c.c, c.upcase)
			}
		}
	}
}

// canUseItem (extra): the item's class mask lets c ready it (the Wrong
// Class test of readyItem).
func (e *Engine) canUseItem(c *Character, it *Item) bool {
	return e.base(it.Type).Classes&c.ClassMask != 0
}

// unknown (extra): Id would reveal a word of the item's name.
func unknown(it *Item) bool {
	for i := 1; i <= 3; i++ {
		if it.Words[i-1] != 0 && (it.Hidden>>(3-i))&1 != 0 {
			return true
		}
	}
	return false
}
