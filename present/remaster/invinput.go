// invinput.go holds the inventory screen's input: its keys and its mouse.

package remaster

import (
	"gpu"
	"image"
	"por/engine"
	"por/present/input"
	"slices"
	"strings"
	"time"
)

// inventoryKey handles a key on the inventory screen: Up/Down, Home/End the
// items, the member keys the member, Tab (pad Y) the tab, PgUp/PgDn (the
// shoulders) the menu row's focus, Enter (pad A) the focused word or the
// item; false leaves it to the game (F10, ?, Ctrl-F).
func (p *Presenter) inventoryKey(k uint16) bool {
	inv := &p.inv
	if inv.ask != nil {
		switch i, back, _ := menuNav(k, &inv.askSel, []byte{'Y', 'N'}); {
		case i == 0:
			inv.do(inv.ask.Kind)
		case i == 1 || back:
			inv.ask = nil
		}
		return true
	}
	type pick struct {
		ware bool
		i    int
	}
	var order []pick // the items as the screen shows them: the goods or the doll, then the list
	for _, it := range inv.wares.shown(inv.view.Wares) {
		order = append(order, pick{true, it.Index})
	}
	for _, it := range inv.view.Doll {
		if it != nil {
			order = append(order, pick{false, it.Index})
		}
	}
	for _, it := range inv.list.shown(inv.view.Pool) {
		order = append(order, pick{false, it.Index})
	}
	step := func(d int) {
		if len(order) == 0 {
			return
		}
		i := slices.Index(order, pick{inv.ware >= 0, max(inv.ware, inv.sel)})
		if i < 0 && d < 0 {
			i = 0
		}
		inv.choose(order[(i+d+len(order))%len(order)].ware, order[(i+d+len(order))%len(order)].i)
	}
	a := actOf(k)
	switch a {
	case input.PadA, input.Space: // pad A, Space: as Enter
		a = input.Enter
	}
	m, member := memberKey(k, inv.member, inv.count, true)
	switch {
	case a == input.Esc && inv.carry != nil: // the stack goes back where it was
		inv.carry = nil
	case a == input.Esc:
		inv.do(engine.ActClose)
	case a == input.Minimap && inv.view.Wares == nil: // Tab, pad Y: the sheet (they toggle)
		inv.do(engine.ActSheet)
	case a == input.Minimap: // the shop and the loot: the next tab of the highlighted item's list
		pn := &inv.list
		if inv.ware >= 0 {
			pn = &inv.wares
		}
		pn.tab, pn.top = (pn.tab+1)%len(tabNames), 0
	case a == input.Up:
		step(-1)
	case a == input.Down:
		step(1)
	case a == input.Home && len(order) > 0: // Home, End: the first and last item
		inv.choose(order[0].ware, order[0].i)
	case a == input.End && len(order) > 0:
		inv.choose(order[len(order)-1].ware, order[len(order)-1].i)
	case (a == input.PgUp || a == input.PgDn) && len(inv.words) > 0: // PgUp/PgDn, the pad's shoulders: the menu row's focus, as the battle bar's
		d, n := 1, len(inv.words)
		if a == input.PgUp {
			d = -1
		}
		if inv.focus == 0 && d < 0 {
			inv.focus = n
		} else {
			inv.focus = (inv.focus-1+d+n)%n + 1
		}
	case a == input.Enter && inv.focus > 0:
		if w := inv.words[inv.focus-1]; w.on {
			inv.do(w.act)
		}
	case a == input.Enter && inv.ware >= 0:
		inv.do(engine.ActBuy)
	case a == input.Enter:
		inv.do(engine.ActReady)
	case member:
		if inv.view.Switch {
			inv.acts = append(inv.acts, engine.Action{Kind: engine.ActMember, N: m})
		}
	case k > ' ' && k < 128:
		for _, w := range inv.menu(inv.view) {
			if w.on && strings.EqualFold(w.text[:1], string(rune(k))) {
				inv.do(w.act)
			}
		}
	default:
		return false
	}
	return true
}

// InventoryMouse handles the pointer at screen pixel (x, y) with the left
// button down or not, the right one just pressed and the wheel's movement.
// A click on a stack picks the whole of it up on the cursor, the next
// click drops it: onto the doll it is readied, into the party inventory it
// is unreadied, back where it came from it is put back; a right click
// picks up or drops one of a stack, with Shift half of it; Shift-click
// readies or unreadies the whole stack at once, as does dragging it onto
// the doll or off it; a right click elsewhere closes the screen. The arrows
// are the previous and next member, the menu row acts on the highlighted
// item; the wheel and the scroll bar scroll.
func (p *Presenter) InventoryMouse(x, y int, down, right bool, wheel float64) {
	inv := &p.inv
	if inv.k == 0 || inv.view == nil {
		return
	}
	moved := image.Pt(x, y) != inv.mouseAt // only a moved pointer takes the question's highlight
	inv.mouseAt = image.Pt(x, y)
	inv.hover = image.Pt(int(float64(x)/inv.kx), int(float64(y)/inv.ky))
	pressed, released := down && !inv.held, !down && inv.held
	inv.held = down

	// the wheel: a line a step, at most one a pause; the rest of a fling is dropped
	pn := &inv.list // the list under the pointer
	if inv.view.Wares != nil && inv.hover.X < inv.list.at.Min.X {
		pn = &inv.wares
	}
	if inv.wheel += wheel; (inv.wheel >= 1 || inv.wheel <= -1) && time.Since(inv.wheeled) >= wheelPause {
		if inv.wheel > 0 { // wheel up: up, as every list
			pn.top--
		} else {
			pn.top++
		}
		pn.clamp()
		inv.wheel, inv.wheeled = 0, time.Now()
	} else if time.Since(inv.wheeled) < wheelPause {
		inv.wheel = 0
	}

	if inv.ask != nil { // the question: its Yes and No (hover highlights), else any click is No
		inv.press, inv.dragging, inv.scrolling = nil, false, nil
		i := slices.IndexFunc(inv.askAt, image.Pt(x, y).In)
		if i >= 0 && moved {
			inv.askSel = i
		}
		switch {
		case pressed && i == 0:
			inv.do(inv.ask.Kind)
		case pressed || right:
			inv.ask = nil
		}
		return
	}
	if sc := inv.scrolling; sc != nil && down && sc.scroll.Dy() > 0 {
		sc.top = scrollBar{sc.scroll, sc.n, sc.rows, sc.top}.to(inv.hover.Y)
	}
	if inv.press != nil && down && !inv.dragging {
		d := inv.hover.Sub(inv.pressAt)
		inv.dragging = d.X*d.X+d.Y*d.Y > 9
	}
	shift := gpu.IsKeyPressed(gpu.KeyShift)
	if pressed && shift && inv.view.Wares == nil && inv.hover.In(inv.face) { // Shift-click on the portrait: the next head
		inv.acts = append(inv.acts, engine.Action{Kind: engine.ActHead, N: -1})
		return
	}
	if right && inv.view.Wares == nil { // a stack on the cursor: one of it (Shift: half) is dropped where the pointer is; else one of the stack under the pointer (Shift: half) is picked up; elsewhere the screen closes
		switch c := inv.carry; {
		case c != nil && shift:
			inv.put(max(1, c.n/2))
		case c != nil:
			inv.put(1)
		default:
			for _, l := range inv.lines {
				if inv.hover.In(l.r) {
					if shift {
						inv.pickUp(&l, max(1, max(l.item.Count, 1)/2))
					} else {
						inv.pickUp(&l, 1)
					}
					return
				}
			}
			inv.do(engine.ActClose)
		}
		return
	}
	if right { // in the shop a right click buys a ware or sells an item
		if inv.view.Wares != nil {
			for _, l := range inv.lines {
				if inv.hover.In(l.r) && !l.doll {
					inv.choose(l.ware, l.item.Index)
					if l.ware && inv.view.Loot {
						inv.do(engine.ActBuy)
					} else if l.ware {
						inv.ask, inv.askSel = &engine.Action{Kind: engine.ActBuy, N: l.item.Index}, 1
					} else if inv.view.Sell {
						inv.do(engine.ActSell)
					}
					return
				}
			}
		}
		inv.do(engine.ActClose)
		return
	}
	if pressed {
		for _, w := range inv.words {
			if inv.hover.In(w.r) && w.on {
				inv.do(w.act)
				return
			}
		}
		for _, sc := range []*pane{&inv.list, &inv.wares} {
			if inv.hover.In(sc.scroll) {
				inv.scrolling = sc
			}
		}
		for _, pn := range []*pane{&inv.list, &inv.wares} {
			for i, r := range pn.tabs {
				if inv.hover.In(r) {
					pn.tab, pn.top = i, 0
					return
				}
			}
		}
		if inv.carry != nil && inv.view.Wares == nil { // the stack on the cursor is dropped where the pointer is
			inv.put(inv.carry.n)
			return
		}
		for _, l := range inv.lines {
			if !inv.hover.In(l.r) {
				continue
			}
			if inv.view.Wares != nil && !l.doll && shift { // Shift-click: a quick trade, no question
				inv.choose(l.ware, l.item.Index)
				if l.ware {
					inv.do(engine.ActBuy)
				} else if inv.view.Sell {
					inv.remove(engine.ActSell, l.item.Index)
				}
				return
			}
			if inv.view.Wares == nil && shift { // Shift-click: the whole stack is readied or unreadied
				inv.choose(false, l.item.Index)
				inv.do(engine.ActReady)
				return
			}
			inv.press, inv.pressAt = &l, inv.hover
		}
		return
	}
	if !released {
		return
	}
	h, drag := inv.press, inv.dragging
	inv.press, inv.dragging, inv.scrolling = nil, false, nil
	if h == nil {
		return
	}
	partySide := inv.hover.X >= inv.list.at.Min.X // dropped on the party's side of the divider
	if h.ware {                                   // a click highlights a ware, a second one or a drag over to the party's items buys it
		was := inv.ware
		inv.choose(true, h.item.Index)
		switch {
		case drag && partySide && !inv.view.Loot:
			inv.ask, inv.askSel = &engine.Action{Kind: engine.ActBuy, N: h.item.Index}, 1 // a drag asks first (loot is free: no question)
		case drag && partySide, !drag && was == h.item.Index:
			inv.do(engine.ActBuy)
		}
		return
	}
	inv.choose(false, h.item.Index)
	if inv.view.Wares != nil { // the shop: a drag over to the goods sells
		if drag && !partySide && inv.view.Sell {
			inv.do(engine.ActSell)
		}
		return
	}
	switch {
	case !drag: // a click picks the whole stack up on the cursor (the next click drops it)
		inv.pickUp(h, 0)
	case h.doll && !inv.hover.In(inv.dollAt), // dragged off the doll
		!h.doll && inv.hover.In(inv.dollAt): // or onto it: the whole stack
		inv.do(engine.ActReady)
	}
}
