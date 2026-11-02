// dispatch.go holds the presenter's key and pointer entry points (Key, Click, Hover) and their routing to the screens.

package remaster

import (
	"gpu"
	"image"
	"math"
	"por/engine"
	"por/present/input"
	"slices"
)

// Key turns window key k into actions (Actions hands them over). When the
// remastered view is up, Enter opens the vertical menu in place of the
// menu bar and Esc on the command bar the renderer's options.
func (p *Presenter) Key(k uint16, v engine.View) {
	if p.Playing() { // a key cuts the fight's moment short
		p.replay.skip = true
		return
	}
	if !p.key(k, v) {
		p.keyAct(k, v)
	}
}

// key takes k on the remastered screens; false leaves it to keyAct.
func (p *Presenter) key(k uint16, v engine.View) bool {
	if actOf(k) == input.Note { // the text box's "> Notes"
		if !p.noteBtn.Empty() {
			p.send(engine.ActNote, 0)
		}
		return true
	}
	if p.journal.open && v.Exploring {
		p.journalKey(k, v)
		return true
	}
	if p.identDone(v) { // the Id's pop-up: any key closes it
		return true
	}
	if confirm(v) != "" {
		return p.confirmKey(k)
	}
	if p.main && (p.mainSkip() || p.mainKey(k, v)) {
		return true
	}
	if p.pickKey(k) {
		return true
	}
	if askList(v) != nil && askMenu(v) != nil && !choosing(v) {
		return p.listKey(k, v)
	}
	if p.pageMenu && typing(v) == nil && p.pageMenuKey(k, v) { // a line being typed (a map label) takes its keys in keyAct
		return true
	}
	if v.Inventory != nil && p.inv.view != nil && !choosing(v) {
		return p.inventoryKey(k)
	}
	if p.fight {
		return p.fightKey(k)
	}
	if p.ctx > 0 { // the card's context menu: the arrows, Enter, a hotkey or Esc, as the game's menus
		if m, ok := memberKey(k, p.ctx-1, len(v.Party), true); p.campHome(v) && (ok || actOf(k) == input.PartyMenu || actOf(k) == input.Minimap) { // camp: another member's menu, or the party's (F9, Left/Right past the ends)
			p.campFocus(v, k, m)
			return true
		}
		if _, ok := memberKey(k, 0, len(v.Party), false); ok { // another member: selected (keyAct), their menu opens instead
			p.ctx = 0
			return false
		}
		if globalKey(k) { // help, the minimap, save/load ...: keyAct; other keys stay here (they would act on the view under it)
			return false
		}
		switch i, back, _ := menuNav(k, &p.ctxSel, p.ctxKeys); {
		case back:
			p.ctx = 0
		case i >= 0:
			p.ctxSend(i)
		}
		return true
	}
	if !p.active || p.view == nil {
		p.menu = false
		return false
	}
	if p.view.OptionsOpen() {
		return true
	}
	if screenUp(v) {
		return p.campKey(k, v)
	}
	if v.Exploring {
		if !p.menu {
			switch actOf(k) {
			case input.Enter, input.PadA:
				p.menu, p.sel = true, 0
			case input.Esc:
				p.view.OpenOptions()
			default:
				return p.exploreKey(k, v)
			}
			return true
		}
		cmds := commands(v)
		keys := make([]byte, len(cmds))
		for i, c := range cmds {
			keys[i] = c.key
		}
		switch i, back, ok := menuNav(k, &p.sel, keys); {
		case back:
			p.menu = false
		case i >= 0:
			p.menu = false
			p.runCommand(i)
		case !ok:
			p.menu = false
			return p.exploreKey(k, v)
		}
		return true
	}
	m := askMenu(v)
	if !p.menu {
		switch {
		case choosing(v): // a member pick has no menu: every key goes to the portraits (keyAct)
			return false
		case actOf(k) == input.Enter && noChoice(v): // "press enter to continue": nothing to pick
			return false
		case actOf(k) == input.Enter && m != nil:
			p.menu, p.menuFor, p.sel = true, menuID(m), 0
		case actOf(k) == input.Esc && v.Exploring:
			p.view.OpenOptions()
		default:
			return false
		}
		return true
	}
	if m == nil {
		p.menu = false
		return false
	}
	if n, ok := memberKey(k, selectedIndex(v), len(v.Party), true); ok { // the member the bar is for (camp, the temple)
		p.send(engine.ActLead, n)
		return true
	}
	keys := make([]byte, len(m.Items))
	for i, it := range m.Items {
		keys[i] = it.Key
	}
	switch i, back, ok := menuNav(k, &p.sel, keys); {
	case back && v.Exploring: // the command bar has no Exit item: the popup alone closes
		p.menu = false
	case back: // the game leaves the menu (its Exit item): hiding the popup alone left the game waiting in it
		p.send(engine.ActClose, 0)
	case i >= 0:
		p.menu = false
		p.send(engine.ActChoose, i)
	case !ok:
		return !globalKey(k) // help, the minimap, save/load ... go on (keyAct); other keys would act on the screen under the popup
	}
	return true
}

// Click takes a click at screen pixel (x, y) on the remastered screens: a
// portrait, an action bar button, or an item of the vertical menu. It
// reports whether it was taken.
func (p *Presenter) Click(x, y int) bool {
	if p.Playing() { // the fight plays on: the click only cuts its moment short
		p.replay.skip = true
		return true
	}
	if image.Pt(x, y).In(p.noteBtn) { // the text box's "> Notes"
		p.send(engine.ActNote, 0)
		return true
	}
	if sheetUp(p.shownView) && gpu.IsKeyPressed(gpu.KeyShift) && image.Pt(x, y).In(p.sheet.faceAt) { // Shift-click on the sheet's portrait: the next head
		p.send(engine.ActHead, -1)
		return true
	}
	if p.journal.open { // the journal's own pointer (Mouse)
		p.journalMouse(image.Pt(x, y), true, false, 0, p.shownView)
		return true
	}
	if p.identDone(p.shownView) { // the Id's pop-up: any click closes it
		return true
	}
	if p.barClick(x, y) {
		return true
	}
	if confirm(p.shownView) != "" { // the yes/no question: its rows (or the battle bar's Yes, No); a click outside is No
		if p.fight && p.battle.Click(x, y) {
			return true
		}
		p.confirmPick(slices.IndexFunc(p.confirmAt, image.Pt(x, y).In))
		return true
	}
	if v := p.shownView; askList(v) != nil && askMenu(v) != nil && !choosing(v) && cardUnder(p.cards, x, y) < 0 { // a card is the member the list is for (below)
		p.listClick(x, y, v)
		return true
	}
	if v := p.shownView; p.pageMenu && pageScreen(v) {
		if h := p.slotFootAt(v, x, y); h != 0 {
			p.slotKey(v, h)
			return true
		}
		for i, r := range p.winRects {
			if image.Pt(x, y).In(r) {
				p.screenPick(v, i)
				return true
			}
		}
		if s := p.scr.sub; screenName(v) == "slots" || s == scrAsk || s == scrField { // outside a popup: back (the creation sheet, the pickers and the icon editor take their page's clicks: Mouse)
			p.screenKey(27, v)
			return true
		}
	}
	if p.pageMenu {
		for i, r := range p.winRects {
			if image.Pt(x, y).In(r) && sheetUp(p.shownView) {
				p.sheetItems(p.shownView)[i].do()
				return true
			}
			if image.Pt(x, y).In(r) {
				p.mapMenu = false
				p.send(engine.ActChoose, i)
				return true
			}
		}
		if v := p.shownView; !pageScreen(v) && !sheetUp(v) && v.FullMap == nil && v.Pages == nil { // outside a page's question: back (the sheet's portrait, the full map and the pagers take their page's clicks: Mouse)
			p.send(engine.ActClose, 0)
			return true
		}
	}
	if p.main {
		if p.mainSkip() {
			return true
		}
		return p.mainClick(x, y)
	}
	if p.pickClick(x, y) {
		return true
	}
	if v := p.shownView; p.campHome(v) && p.active { // camp: a line of any member's menu
		for i, rs := range p.campCtx {
			if j := slices.IndexFunc(rs, image.Pt(x, y).In); j >= 0 && i < len(v.Party) && j < len(v.Party[i].Powers) {
				p.ctx = 0
				p.memberAct(v.Party[i].Powers[j].Kind, i)
				return true
			}
		}
		p.ctx = 0 // elsewhere: the keys go back to the party's menu
	}
	if !p.fight {
		if p.ctx > 0 { // the context menu: a line acts for its member; a click elsewhere closes it (on another card: that one's)
			for j, r := range p.ctxRects {
				if image.Pt(x, y).In(r) && p.active {
					p.ctxSend(j)
					return true
				}
			}
			p.ctx = 0 // a click outside: back; on a card, that one's menu (below)
			if !slices.ContainsFunc(p.cards, image.Pt(x, y).In) {
				return true
			}
		}
		for i, r := range p.cards { // a portrait: its context menu while exploring, else its inventory (a right click: the sheet, Mouse)
			if image.Pt(x, y).In(r) && p.active {
				p.cardClick(i)
				if p.ordering { // while the party is ordered the card is picked up instead (Hover drops it)
					p.dragFrom, p.dragAt, p.dragStart, p.dragged = 1+i, image.Pt(x, y), image.Pt(x, y), false
				}
				return true
			}
		}
		if v := p.shownView; screenUp(v) && p.active && !p.view.OptionsOpen() {
			for i, r := range p.menuRects {
				if image.Pt(x, y).In(r) {
					p.campPick(v, p.camp.top+i)
					return true
				}
			}
			p.campBack(v) // a click outside: back
			return true
		}
		if v := p.shownView; v.Exploring && !p.menu && p.active && p.view != nil && !p.view.OptionsOpen() { // the view: walk, turn, the command menu
			p.exploreClick(x, y, v)
			return true
		}
		if !p.menu || !p.active || p.view == nil || p.view.OptionsOpen() {
			return false
		}
		for i, r := range p.menuRects {
			if image.Pt(x, y).In(r) {
				p.menu = false
				if p.command {
					p.runCommand(i)
					return true
				}
				p.send(engine.ActChoose, i)
				return true
			}
		}
		p.key(keyOf(input.Esc), p.shownView) // a click outside: back, as Esc
		return true
	}
	return p.fightClick(x, y)
}

// Hover moves the vertical menu's highlight to the item under screen pixel (x, y).
// down: the left button is held; a card pressed while the party is being
// ordered (Click took it) follows the pointer and goes where it is let go.
func (p *Presenter) Hover(x, y int, down bool) {
	p.camp.pointer = image.Pt(x, y)
	if p.journal.open {
		p.journalMouse(image.Pt(x, y), false, false, 0, p.shownView)
		return
	}
	if p.barHover(y, down) {
		return
	}
	if confirm(p.shownView) != "" { // the yes/no question takes a moved pointer (in combat the bar's hover goes on below)
		if pt := image.Pt(x, y); pt != p.pointer {
			p.pointer = pt
			if i := slices.IndexFunc(p.confirmAt, pt.In); i >= 0 {
				p.confirmSel = i
			}
		}
		if !p.fight {
			return
		}
	}
	if v := p.shownView; askList(v) != nil && askMenu(v) != nil && !choosing(v) {
		p.listHover(x, y, v)
	}
	if p.main {
		p.mainHover(x, y)
		return
	}
	if p.pickList() != nil {
		if image.Pt(x, y) != p.pointer { // only a moved pointer takes the highlight
			p.pointer = image.Pt(x, y)
			if i := slices.IndexFunc(p.menuRects, p.pointer.In); i >= 0 {
				p.pickSel = i
			}
		}
		return
	}
	if p.pageMenu && image.Pt(x, y) != p.pointer { // the page's menus (keyWindow): the sheet's, a question's, the page screens'
		p.pointer = image.Pt(x, y)
		if i := slices.IndexFunc(p.winRects, p.pointer.In); i >= 0 {
			switch {
			case !pageScreen(p.shownView):
				p.sel = i
			case p.scr.sub != scrType:
				p.scr.sel = p.scr.top + i
			}
		}
	}
	if p.fight {
		p.pointer = image.Pt(x, y)
		if !p.fightLog {
			p.battle.Point(x, y)
		}
		return
	}
	p.cardHover = 1 + cardUnder(p.cards, x, y)
	if p.dragFrom > 0 {
		p.dragAt = image.Pt(x, y)
		if d := p.dragAt.Sub(p.dragStart); float64(d.X*d.X+d.Y*d.Y) >= math.Pow(8*gpu.DeviceScaleFactor(), 2) { // about 8 points before it is a drag: a click only selects
			p.dragged = true
		}
		if !down {
			if to := p.cardHover; p.dragged && to > 0 && to != p.dragFrom {
				p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActOrder, N: p.dragFrom - 1, X: to - 1})
			}
			p.dragFrom = 0
		}
	}
	if p.campHome(p.shownView) && image.Pt(x, y) != p.pointer { // camp: the window under the pointer takes the focus
		p.pointer = image.Pt(x, y)
		p.campHover(p.shownView, x, y)
		return
	}
	if screenUp(p.shownView) && p.ctx == 0 && image.Pt(x, y) != p.pointer { // camp-style screens highlight under the pointer too
		p.pointer = image.Pt(x, y)
		if i := slices.IndexFunc(p.menuRects, p.pointer.In); i >= 0 {
			p.camp.sel = p.camp.top + i
		}
		return
	}
	if !(p.menu || p.ctx > 0) || image.Pt(x, y) == p.pointer {
		return
	}
	p.pointer = image.Pt(x, y)
	if p.ctx > 0 { // a moved pointer takes the context menu's highlight (keys keep it while it rests)
		if i := slices.IndexFunc(p.ctxRects, p.pointer.In); i >= 0 { // off the lines the highlight stays
			p.ctxSel = i
		}
		return
	}
	for i, r := range p.menuRects {
		if p.pointer.In(r) {
			p.sel = i
		}
	}
}

// cardUnder is the card at screen pixel (x, y), -1 for none.
func cardUnder(cards []image.Rectangle, x, y int) int {
	for i, r := range cards {
		if image.Pt(x, y).In(r) {
			return i
		}
	}
	return -1
}

// menuID tells menus apart: prompt and items.
func menuID(m *engine.Menu) string {
	id := m.Prompt
	for _, it := range m.Items {
		id += "|" + it.Text
	}
	return id
}

// ctxSend acts line j of the context menu for its member and closes it; a
// spell pick opens where the menu was.
func (p *Presenter) ctxSend(j int) {
	if p.ctxActs[j] == engine.ActSpells {
		p.pickAt = p.ctxBox
	}
	n := p.ctx - 1
	p.ctx = 0
	p.memberAct(p.ctxActs[j], n)
}

// cardClick is a left click on member i's card: while exploring (the
// command bar) it selects the member and opens its context menu, elsewhere
// it is the member's inventory; while the party is ordered it only selects.
func (p *Presenter) cardClick(i int) {
	switch {
	case noChoice(p.shownView): // a script's unseen "press Enter" wait (PressToContinue before its text): the card continues, as a click elsewhere
		p.send(engine.ActOK, 0)
	case choosing(p.shownView): // a member pick (a spell's target): the card is the pick, exploring or not
		p.send(engine.ActMember, i)
	case p.ordering:
		p.send(engine.ActLead, i)
	case p.command, p.campHome(p.shownView): // camp: the keys go to its menu
		p.send(engine.ActLead, i)
		p.ctx, p.ctxSel = 1+i, 0 // the first line highlighted: Enter acts at once
	default:
		p.send(engine.ActInventory, i)
	}
}
