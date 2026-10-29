package engine

import (
	"strconv"
	"strings"
)

// The shop, ported from ovr07. ECL COMBAT with the shop flag (game +0x6D8)
// set runs Shop instead of a fight; the goods are the treasure list.

// The words of the shop's Buy list.
const (
	shopBuyIt Choice = iota
	shopLook
	shopInv
)

// shopList = ovr07:0030 (var index, var list, var item, var key): the goods
// with their prices right-aligned in a 30-character field, in a "Shop" box.
// extra: the field runs to the box's right edge, so longer names fit.
func (e *Engine) shopList(index *int, item **Item) Pick {
	list := make([]*ListNode, len(e.run.treasure))
	for i, it := range e.run.treasure {
		if it.Price == 0 {
			it.Price = 1
		}
		price := strconv.Itoa(int(it.Price))
		if len(price) > 0x10 {
			price = price[:0x10]
		}
		const w = 0x24 // columns 2..0x25, one in from the frame (the original: 1..0x1E)
		line := []byte(strings.Repeat(" ", w))
		copy(line[w-len(price):], price)
		copy(line, cut(it.Name(), w-1-len(price))) // extra: the name stops before the price
		list[i] = &ListNode{Text: string(line)}
		list[i].Dim = !e.canUseItem(e.selected(), it) // extra: what the buyer cannot use is grey and passed over
	}
	e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colWhite, "Shop")
	gold := strconv.Itoa(int(e.purse.gold)) + "g" // extra: on the frame, the g under the prices' last digit
	e.DisplayString(winRight-len(gold), messageRow, colYellow, gold)
	if c := e.selected(); len(e.party) > 0 { // extra: who is buying (F1-F8 change it)
		e.displayName(c, 2, 2, false) // over the items; the class over the prices
		cls := cut(className(c.Class), 0x23-len(c.Name()))
		e.DisplayString(winRight-len(cls), 2, colLightCyan, cls)
	}
	redraw := true
	var sel *ListNode
	if len(list) > 0 { // extra: the cursor starts on something the buyer can use
		for k := 0; k < len(list) && list[*index%len(list)].Dim; k++ {
			*index = (*index + 1) % len(list)
		}
		*index %= len(list)
	}
	e.ext.buying = true
	key := e.ListMenu("Items: ", Bar{{"Buy", shopBuyIt}, {"Look", shopLook}, {"Inv", shopInv}}, colLightMagenta, colLightGreen, colWhite, 2, 4, 38, 22, list, true, &redraw, index, &sel)
	e.ext.buying = false
	*item = nil
	for i, n := range list {
		if n == sel {
			*item = e.run.treasure[i]
		}
	}
	for _, it := range e.run.treasure {
		e.ItemName(nil, it, 0, 0, false, false)
	}
	return key
}

// buyItem = ovr07:0239 (item, var overloaded): a copy goes to the
// selected character unless it is too much to carry.
func (e *Engine) buyItem(it *Item) (overloaded bool) {
	c := e.selected()
	if e.overloaded(it, c) {
		e.BottomMessage("OverLoaded")
		return true
	}
	e.AddItemCopy(c, it)
	e.ApplyItems(c)
	return false
}

// shopBuy = ovr07:0361: pick goods until Exit, paid from the party's gold.
func (e *Engine) shopBuy() {
	// extra: the list opens at its top on the first item (the original,
	// ovr07:037C, starts on the second, with the top left where the last
	// list had it)
	index := 0
	e.listTop = 0
	for {
		var it *Item
		key := e.shopList(&index, &it)
		switch key.ID {
		case PickRelist:
			continue // F1-F8 picked another buyer
		case shopLook: // extra: what the item does
			e.lookItem(it)
			continue
		case shopInv: // extra: the buyer's own items
			if len(e.carried(e.selected())) == 0 {
				e.BottomMessage("No items")
			} else {
				top := e.listTop // extra: back on the same page of the list
				e.ItemsMenu()
				e.listTop = top
			}
			continue
		}
		if key.ID != shopBuyIt && key.ID != PickEnter || it == nil {
			return
		}
		price := uint32(it.Price)
		if price > e.purse.gold {
			e.BottomMessage("Not enough Money.")
		} else if !e.buyItem(it) {
			e.purse.gold -= price
		}
	}
}

// Shop = ovr07:053E.
func (e *Engine) Shop() {
	e.beginFlag = 1
	e.DrawMainScreen()
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
	for leave := false; !leave; {
		e.shopMoney()
		p := e.DisplayInput("", menu, colLightMagenta, colLightGreen, colWhite, true, false)
		redraw := false
		switch p.ID {
		case browse:
			e.shopBuy()
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
		}
		e.DisplayPartyList()
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
				e.BottomMessage("No unknown items")
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
