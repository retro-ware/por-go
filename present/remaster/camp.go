package remaster

import (
	"cmp"
	"fmt"
	"image"
	"slices"
	"time"

	"gpu"

	"por/engine"
	"por/present/input"
)

// Camp (View.Camp): the presenter's own screens over camp's state, each
// choice a decision for the selected member.

// Camp screens.
const (
	campMain = iota
	campMemorize // the spell levels
	campLevel    // one level's spells to memorize
	campScribe
	campBestiary
	campDrop // the drop's confirmation

	templeHeal // the temple's services (temple.go)
	templeNot  // "is not blind": cure anyway?
	templePay  // pay for the cure?
	appraise   // the gems and jewelry (temple and shop)
	shopID     // the items to identify (shop.go)
	shopDeal   // identify this one?
)

type campUI struct {
	screen    int
	sel       int
	top       int             // the first line shown of a list too long for the screen
	byDanger  bool            // the bestiary's order
	beast     string          // the entry shown
	beastAt   string          // the entry beastY scrolls
	beastY    int             // the entry's first pixel row shown
	beastMax  int             // its last beastY
	beastPane image.Rectangle // the entry on the window (the wheel scrolls it there)
	pointer   image.Point     // the pointer on the window (Hover), for the wheel
	wheeled   time.Time       // the last line the wheel moved the bestiary's list
	pick      int             // the temple's cure or the shop's item asked about
	ids       []int           // the spell lists' lines: each one's spell (0: a level, Exit)
	unset     []bool          // the spell lists' lines with none memorized (grey)
	level     string          // the level campLevel lists
	rows      map[int][2]int  // each screen left for a submenu: its highlighted row and first line shown, for back
}

// to opens screen n (a submenu, or a question on row sel), keeping the
// row that opened it for back.
func (c *campUI) to(n, sel int) func() {
	return func() {
		if c.rows == nil {
			c.rows = map[int][2]int{}
		}
		c.rows[c.screen] = [2]int{c.sel, c.top}
		c.screen, c.sel, c.top = n, sel, 0
	}
}

// back returns to screen n on the row that opened the submenu left.
func (c *campUI) back(n int) func() {
	return func() { c.screen, c.sel, c.top = n, c.rows[n][0], c.rows[n][1] }
}

// campItems are the lines of the camp screen up, and its title.
// The temple, the shop and training share the camp's screens.
func (p *Presenter) campItems(v engine.View) (string, []menuRow) {
	switch {
	case v.Temple != nil:
		return p.templeItems(v)
	case v.Shop != nil:
		return p.shopItems(v)
	case v.Training != nil:
		return p.trainItems(v)
	}
	c := &p.camp
	cp := v.Camp
	act := func(k engine.ActKind, n int) func() { return func() { p.send(k, n) } }
	switch c.screen {
	case campMemorize: // the levels, each its own submenu
		var it []menuRow
		c.ids, c.unset = c.ids[:0], c.unset[:0]
		for i, s := range cp.Book {
			if i == 0 || cp.Book[i-1].Level != s.Level {
				it, c.ids = append(it, menuRow{fmt.Sprintf("%-26s %d/%d", s.Level, s.Planned, s.Slots), 0, func() { c.level = s.Level; c.to(campLevel, 0)() }}), append(c.ids, 0)
			}
		}
		return "Spells to memorize", append(it, menuRow{"Exit", 0, c.back(campMain)})
	case campLevel, campScribe:
		list, title := cp.Scrolls, "Spells to scribe"
		if c.screen == campLevel {
			list, title = nil, c.level
			for _, s := range cp.Book {
				if s.Level == c.level {
					list, title = append(list, s), fmt.Sprintf("%c%s%c %d/%d", inkHead, s.Level, inkHead, s.Planned, s.Slots)
				}
			}
		}
		var it []menuRow
		c.ids, c.unset = c.ids[:0], c.unset[:0]
		for _, s := range list {
			text := fmt.Sprintf("%-26s x%d", s.Name, s.Count)
			if c.screen == campScribe {
				text = s.Name
				if s.Count > 0 {
					text += " *"
				}
			}
			it, c.ids, c.unset = append(it, menuRow{text, 0, func() { p.campSpell(s.ID, 1) }}), append(c.ids, s.ID), append(c.unset, c.screen == campLevel && s.Count == 0)
		}
		back := c.back(campMain)
		if c.screen == campLevel {
			back = c.back(campMemorize)
		}
		return title, append(it, menuRow{"Exit", 0, back}) // no hotkeys: the lines are spells
	case campBestiary:
		sort := "Sort by danger"
		if c.byDanger {
			sort = "Sort by name"
		}
		it := []menuRow{{sort, 0, func() { c.byDanger = !c.byDanger }}}
		for _, b := range p.beasts(v) {
			it = append(it, menuRow{b.Name, 0, func() { c.beast = b.Name }}) // shown beside the list (drawBestiary)
		}
		return "Bestiary", append(it, menuRow{"Exit", 'E', c.back(campMain)})
	case campDrop:
		return "Drop " + selectedName(v) + " from the party?", []menuRow{
			{"Yes", 'Y', func() { p.send(engine.ActDropMember, selectedIndex(v)); c.back(campMain)() }},
			{"No", 'N', c.back(campMain)}}
	}
	var it []menuRow // the party's, the rests first: each member's own are on their card's menu (drawCampMenus)
	if cp.Healing || cp.Studying {
		it = append(it, menuRow{"Until fixed", 'F', act(engine.ActRest, engine.RestFix)})
	}
	until := menuRow{"Until night", 'N', act(engine.ActRest, engine.RestUntil)}
	if !v.Day {
		until = menuRow{"Until day", 'D', act(engine.ActRest, engine.RestUntil)}
	}
	it = append(it, menuRow{"Wait an hour", 'W', act(engine.ActRest, engine.RestWait)}, until)
	if len(cp.Bestiary) > 0 {
		it = append(it, menuRow{"Bestiary", 'B', c.to(campBestiary, 0)})
	}
	return "Camp", append(it, menuRow{"Exit", 'E', act(engine.ActClose, 0)})
}

// campHome: camp's own screen is up, the members' menus beside their
// cards (drawCampMenus).
func (p *Presenter) campHome(v engine.View) bool {
	return v.Camp != nil && p.camp.screen == campMain && screenUp(v)
}

// drawCampMenus draws every member's context menu at once beside their
// card; the one with the keys (ctx) shows its highlight.
func (p *Presenter) drawCampMenus(dst *gpu.Image, v engine.View) {
	p.campCtx = p.campCtx[:0]
	for i := range min(len(v.Party), len(p.cards)) {
		lines, acts, keys := powerLines(v.Party[i].Powers)
		sel := -1
		if p.ctx == 1+i {
			sel = p.ctxSel
		}
		p.menuGrey = p.ctx != 1+i // the focus's border red, the others' grey
		for len(p.campImg) <= i {
			p.campImg, p.campBuf = append(p.campImg, nil), append(p.campBuf, nil)
		}
		p.tipImg, p.tipBuf, p.campImg[i], p.campBuf[i] = p.campImg[i], p.campBuf[i], p.tipImg, p.tipBuf // each menu its own image: one image redrawn in a frame shows its last pixels everywhere
		shown := keys                                                                                   // the white hotkeys only on the focus's menu
		if p.ctx != 1+i {
			shown = nil
		}
		rs, box := p.cardMenu(dst, i, lines, shown, sel)
		p.tipImg, p.tipBuf, p.campImg[i], p.campBuf[i] = p.campImg[i], p.campBuf[i], p.tipImg, p.tipBuf
		p.menuGrey = false
		p.campCtx = append(p.campCtx, rs)
		if p.ctx == 1+i {
			p.ctxActs, p.ctxKeys, p.ctxRects, p.ctxBox = acts, keys, append(p.ctxRects[:0], rs...), box
		}
	}
}

// campFocus moves camp's one focus for key k: F1-F8 to member m's menu,
// F9 to the party's menu, Left/Right (the pad's d-pad) and Tab round them
// in the screen's order: the first half of the members (the left column,
// or the row's left half), the party's menu, the rest.
func (p *Presenter) campFocus(v engine.View, k uint16, m int) {
	order := []int{} // ctx values: 0 the party's menu, 1+i member i's
	for i := range v.Party {
		if i == (len(v.Party)+1)/2 {
			order = append(order, 0)
		}
		order = append(order, 1+i)
	}
	if len(v.Party) < 2 {
		order = append(order, 0)
	}
	n, at := len(order), slices.Index(order, p.ctx)
	switch a := actOf(k); a {
	case input.PartyMenu:
		p.ctx = 0
		return
	case input.Left, input.PrevMember:
		p.ctx = order[(at+n-1)%n]
	case input.Right, input.NextMember, input.Minimap: // Minimap: Tab
		p.ctx = order[(at+1)%n]
	default:
		p.ctx = 1 + m
	}
	p.ctxSel = 0
	if p.ctx > 0 {
		p.send(engine.ActLead, p.ctx-1)
	}
}

// campHover gives the focus to the camp window under the pointer (x, y)
// and its line the highlight: the party's menu, or a member's (who is
// then selected).
func (p *Presenter) campHover(v engine.View, x, y int) {
	if i := slices.IndexFunc(p.menuRects, image.Pt(x, y).In); i >= 0 {
		p.ctx, p.camp.sel = 0, p.camp.top+i
		return
	}
	for i, rs := range p.campCtx {
		if j := slices.IndexFunc(rs, image.Pt(x, y).In); j >= 0 {
			if p.ctx != 1+i {
				p.send(engine.ActLead, i)
			}
			p.ctx, p.ctxSel = 1+i, j
			return
		}
	}
}

// memberAct carries out power k of member n's context menu: camp's
// Memorize, Scribe and Drop open the presenter's screens for them.
func (p *Presenter) memberAct(k engine.ActKind, n int) {
	screen := map[engine.ActKind]int{engine.ActPlan: campMemorize, engine.ActScribe: campScribe, engine.ActDropMember: campDrop}
	if s, ok := screen[k]; ok {
		p.send(engine.ActLead, n)
		p.camp.to(s, map[bool]int{true: 1}[s == campDrop])() // the drop's question on No
		return
	}
	p.send(k, n)
}

// campSpell plans one more (n > 0) or one less of spell id, or toggles
// its scribing.
func (p *Presenter) campSpell(id, n int) {
	if p.camp.screen == campScribe {
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActScribe, N: -1, X: id})
		return
	}
	p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActPlan, N: -1, X: id, Y: n})
}

// beasts is the bestiary in the order chosen.
func (p *Presenter) beasts(v engine.View) []engine.Beast {
	b := slices.Clone(v.Camp.Bestiary)
	if p.camp.byDanger {
		slices.SortStableFunc(b, func(x, y engine.Beast) int { return cmp.Compare(x.Danger, y.Danger) })
	} else {
		slices.SortStableFunc(b, func(x, y engine.Beast) int { return cmp.Compare(x.Name, y.Name) })
	}
	return b
}

var roman = [...]string{"", "I", "II", "III", "IV", "V", "VI", "VII", "VIII"}

func selectedIndex(v engine.View) int {
	for i, m := range v.Party {
		if m.Selected {
			return i
		}
	}
	return -1
}

func selectedName(v engine.View) string {
	if i := selectedIndex(v); i >= 0 {
		return v.Party[i].Name
	}
	return ""
}

// drawCamp draws the camp screen up at scale k, centred on cx, its top at
// top; a bestiary entry's icon goes over its box.
func (p *Presenter) drawCamp(dst *gpu.Image, glyph func(byte) [8]byte, v engine.View, k float64, cx, top, bottom int) {
	title, items := p.campItems(v)
	lines, keys, off := make([]string, len(items)), p.campKeys(items), rowsOff(items)
	for i, it := range items {
		lines[i] = it.text
		if i < len(p.camp.unset) && p.camp.unset[i] && p.camp.screen == campLevel { // none memorized: grey, still picked
			off[i] = true
		}
	}
	p.camp.sel = min(max(p.camp.sel, 0), len(items)-1)
	var icons []image.Image // the rows' icons: the items' (the shop's Identify), the creatures' (the bestiary)
	switch {
	case v.Shop != nil && p.camp.screen == shopID:
		for _, u := range v.Shop.Unknown {
			icons = append(icons, p.inv.iconImage(u.Icon))
		}
	case p.camp.screen == campBestiary && v.Camp != nil:
		icons = []image.Image{nil} // the sort row
		for _, b := range p.beasts(v) {
			icons = append(icons, b.Icon)
		}
	}
	if icons != nil {
		icons = append(icons, make([]image.Image, max(0, len(lines)-len(icons)))...)[:len(lines)]
	}
	c := &p.camp
	lh := 1 // a line's cells
	if len(icons) > 0 {
		lh = iconRow
	}
	extra := 2 // the border; the title and a blank line under it
	if title != "" {
		extra += len(titleLines(title)) + 1
	}
	fit := max(3, (int(float64(bottom)/(k*8))-extra)/lh) // the lines that fit: more scroll, the scale stays
	n := len(lines)
	c.top = min(max(c.top, c.sel-fit+1), c.sel, max(0, n-fit))
	if n > fit {
		end := c.top + fit
		lines, keys, off = lines[c.top:end], keys[c.top:end], off[c.top:end]
		if icons != nil {
			icons = icons[c.top:end]
		}
	}
	rows := extra + lh*len(lines)
	top = max(0, min(top, bottom-int(k*float64(rows*8)))) // moved up to fit above bottom
	if yesNo(keys) {                                      // a question: where every one goes (keyWindow)
		p.keyWindow(dst, title, lines, keys, off, c.sel, false)
		p.menuRects = slices.Clone(p.winRects)
		return
	}
	sel := c.sel - c.top
	if p.campHome(v) && p.ctx > 0 { // a member's menu has the focus: no highlight, no white hotkeys here
		sel, p.menuGrey, keys = -1, true, nil
	}
	p.drawMenuIcons(dst, glyph, title, lines, keys, off, icons, sel, k, cx, top)
	p.menuGrey = false
	p.menuBar(dst, p.menuRects, n, len(lines), &c.top, &c.sel, k)
	if (p.spellList(v) || v.Training != nil && len(v.Training.Learn) > 0) && c.sel < len(c.ids) && len(p.menuRects) > 0 {
		p.drawSpellTip(dst, c.ids[c.sel], k, cx)
	}
}

// drawSpellTip draws spell id's rules under the spell list (above it when
// they do not fit), on its own image: the card tip has the tip's.
func (p *Presenter) drawSpellTip(dst *gpu.Image, id int, k float64, cx int) {
	lines := engine.SpellRules(id, 36)
	if lines == nil {
		return
	}
	box := p.menuRects[0]
	for _, r := range p.menuRects {
		box = box.Union(r)
	}
	h := int(k) * 8 * (len(lines) + 2)
	top := box.Max.Y + int(k)*16
	if top+h > dst.Bounds().Dy() {
		top = max(0, box.Min.Y-int(k)*32-h)
	}
	p.tipImg, p.tipBuf, p.spellTipImg, p.spellTipBuf = p.spellTipImg, p.spellTipBuf, p.tipImg, p.tipBuf
	p.tipMenu(dst, "", lines, make([]byte, len(lines)), -1, k, cx, top)
	p.tipImg, p.tipBuf, p.spellTipImg, p.spellTipBuf = p.spellTipImg, p.spellTipBuf, p.tipImg, p.tipBuf
}

// spellList: a camp spell list is up (its rules drawn by it, the picture
// hidden under them).
func (p *Presenter) spellList(v engine.View) bool {
	s := p.camp.screen
	return v.Camp != nil && (s == campMemorize || s == campLevel || s == campScribe)
}

// campKey takes key k on a camp screen.
func (p *Presenter) campKey(k uint16, v engine.View) bool {
	c := &p.camp
	_, items := p.campItems(v)
	n := len(items)
	spells := p.spellList(v)
	m, member := memberKey(k, selectedIndex(v), len(v.Party), true)
	switch a := actOf(k); {
	case c.screen == campBestiary && (a == input.PgUp || a == input.PgDn): // the entry, eight lines a step
		step := 80
		if a == input.PgUp {
			step = -step
		}
		c.beastY = min(max(c.beastY+step, 0), c.beastMax)
	case spells && (a == input.Less || a == input.Backspace) && c.sel < n-1: // -, Backspace: one less
		p.campSpellAt(v, c.sel, -1)
	case spells && a == input.More && c.sel < n-1: // +: one more
		p.campSpellAt(v, c.sel, 1)
	case actOf(k) == input.PartyMenu:
	case (member || actOf(k) == input.Minimap) && p.campHome(v): // another menu takes the keys
		p.campFocus(v, k, m)
	case member && !yesNo(p.campKeys(items)): // the member camp acts for (not under a question about one)
		p.send(engine.ActLead, m)
	default:
		switch i, back, ok := menuNav(k, &c.sel, p.campKeys(items)); {
		case back:
			p.campBack(v)
		case i >= 0:
			p.campPick(v, i)
		case !ok:
			return !globalKey(k) // help, the minimap, save/load ... go on (keyAct); other keys would act on the view under the screen
		}
	}
	return true
}

// campBack goes back one level: the screen's Exit or No row, else the
// screen closes. It never picks a list's line.
func (p *Presenter) campBack(v engine.View) {
	_, items := p.campItems(v)
	if n := len(items); n > 0 && slices.Contains([]string{"Exit", "No"}, items[n-1].text) {
		p.campPick(v, n-1)
		return
	}
	p.send(engine.ActClose, 0)
}

// campKeys are the camp screen's hotkeys: none on the spell lists (the
// digits would land in the counts), the rest as every menu.
func (p *Presenter) campKeys(items []menuRow) []byte {
	if s := p.camp.screen; s == campMemorize || s == campLevel || s == campScribe {
		return make([]byte, len(items))
	}
	return rowKeys(items)
}

// campSpellAt changes the plan of the spell on line i.
func (p *Presenter) campSpellAt(v engine.View, i, n int) {
	if c := &p.camp; i < len(c.ids) && c.ids[i] != 0 {
		p.campSpell(c.ids[i], n)
	}
}

// campPick picks line i of the camp screen up.
func (p *Presenter) campPick(v engine.View, i int) {
	if _, items := p.campItems(v); i >= 0 && i < len(items) && items[i].do != nil { // a heading picks nothing
		p.camp.sel = i
		items[i].do()
	}
}

// screenUp: one of the camp-style screens is up (camp, temple, shop, training).
func screenUp(v engine.View) bool {
	return (v.Camp != nil || v.Temple != nil || v.Shop != nil || v.Training != nil) && askMenu(v) == nil
}

// sendX sends a decision with an X.
func (p *Presenter) sendX(k engine.ActKind, n, x int) {
	p.inv.acts = append(p.inv.acts, engine.Action{Kind: k, N: n, X: x})
}

// appraiseItems is the appraise screen; back goes to the screen before.
func (p *Presenter) appraiseItems(a engine.Appraise, back func()) (string, []menuRow) {
	if a.Valued > 0 {
		what := "Gem"
		if a.Jewel {
			what = "Jewel"
		}
		var it []menuRow
		if a.CanKeep {
			it = append(it, menuRow{"Keep", 'K', func() { p.send(engine.ActKeep, 1) }})
		}
		sell := func() { p.send(engine.ActKeep, 0) }
		return fmt.Sprintf("The %s is valued at %s. Leaving sells it.", what, price(engine.Money{Gold: a.Valued})), append(it, menuRow{"Sell", 'S', sell},
			menuRow{"Exit", 'E', sell}) // back to the list: the original's leaving sells (Esc too, campBack), the shop stays
	}
	var it []menuRow
	if a.Gems > 0 {
		it = append(it, menuRow{fmt.Sprintf("Gems (%d)", a.Gems), 'G', func() { p.send(engine.ActAppraise, 0) }})
	}
	if a.Jewelry > 0 {
		it = append(it, menuRow{fmt.Sprintf("Jewelry (%d)", a.Jewelry), 'J', func() { p.send(engine.ActAppraise, 1) }})
	}
	title := "You have a fine collection"
	if len(it) == 0 {
		title = "Nothing to appraise"
	}
	return title, append(it, menuRow{"Exit", 'E', back})
}
