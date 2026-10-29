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
	defer func() { e.snap.shop = false }()
	e.ItemsMenu()
}

// shopWares adds the goods and the party's money to the inventory view.
func (e *Engine) shopWares(v *Inventory, c *Character) {
	v.Gold, v.Copper = int(e.purse.gold), int(e.purse.copper)
	v.Wares = []InvItem{}
	for i, it := range e.run.treasure {
		e.ItemName(nil, it, 0, 0, false, false)
		x := InvItem{Index: i, Name: it.Name(), Type: byte(it.Type), Count: int(it.Count), Slot: dollSlot(e, c, it) >= 0,
			Wrong: !e.canUseItem(c, it), Look: e.itemLines(it), Price: strconv.Itoa(int(it.Price)) + " gold",
			Kind: itemKind(e, c, it)}
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

// Shop = ovr07:053E.
func (e *Engine) Shop() {
	greeting := e.ext.cells // extra: the merchant's greeting comes back after each redraw (the original cleared it)
	e.beginFlag = 1
	e.DrawMainScreen()
	e.keepPane(greeting)
	e.DisplayPartyList()
	for _, it := range e.run.treasure {
		e.ItemName(nil, it, 0, 0, false, false)
	}
	const (
		browse Choice = iota
		appraise
		identify
		exit
	)
	menu := Bar{{"Browse", browse}, {"Appraise", appraise}, {"Identify", identify}, {"Exit", exit}} // extra: Buy renamed Browse, no View, Id spelled out
	if e.ext.sels == nil {
		e.ext.sels = map[string]int{}
	}
	e.ext.sels[menu.String()] = 1 // extra: a new visit starts on Browse, not where the last one left
	for leave := false; !leave; { // extra: no "Party money" line: it wrote over the merchant's greeting (the inventory shows the purse)
		p := e.DisplayInput("", menu, colLightMagenta, colLightGreen, colWhite, true, false)
		redraw := false
		switch p.ID {
		case browse:
			e.shopBrowse()
			redraw = true
		case appraise:
			redraw = e.Appraise()
		case identify:
			redraw = e.identifyMenu()
		case exit:
			leave = true
		case PickMove, PickRelist:
			if memberPick(p) {
				e.partyKeypad(p)
			}
		}
		if redraw {
			e.DrawMainScreen()
			e.keepPane(greeting)
		}
		e.DisplayPartyList()
	}
}

// keepPane (extra) puts the text pane's cells back as they were in cells.
func (e *Engine) keepPane(cells [25][40]textCell) {
	for y := msgTop; y <= winBottom; y++ {
		for x := 1; x <= winRight; x++ {
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

// identifyMenu (extra): the shop's Id, moved out of the item menu. The
// party's unknown items under their owners' names; only those Id tells
// something about. Returns true when the screen needs redrawing.
func (e *Engine) identifyMenu() bool {
	index := 0
	e.listTop = 0
	for {
		var list []*ListNode
		var owner []*Character
		var item []*Item
		for i, c := range e.party {
			head := false
			own := c.Items
			if i == 0 { // extra: the party inventory, listed with the first member
				own = e.carried(c)
			}
			for _, it := range own {
				if !unknown(it) {
					continue
				}
				if !head {
					list, owner, item = append(list, &ListNode{Text: c.Name(), Flag: 1}), append(owner, nil), append(item, nil)
					head = true
				}
				e.ItemName(c, it, 0, 0, false, false)
				list, owner, item = append(list, &ListNode{Text: cut(" "+it.Name(), 0x24)}), append(owner, c), append(item, it)
			}
		}
		if len(list) == 0 {
			if index == 0 {
				e.BottomMessage("Nothing to identify")
				return false
			}
			return true
		}
		if index == 0 || index >= len(list) || list[index].Flag != 0 {
			index = 1
		}
		e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colWhite, "Identify for 200g")
		gold := strconv.Itoa(int(e.purse.gold)) + "g" // as in the buy screen
		e.DisplayString(winRight-len(gold), messageRow, colYellow, gold)
		redraw := true
		var sel *ListNode
		const id Choice = 0
		key := e.ListMenu("", Bar{{"Id", id}}, colLightCyan, colLightGreen, colWhite, 2, 3, 38, 22, list, true, &redraw, &index, &sel).ID
		if key != id && key != PickEnter || sel == nil {
			return true
		}
		for i, n := range list {
			if n == sel && item[i] != nil {
				e.identifyItem(owner[i], item[i])
			}
		}
	}
}
