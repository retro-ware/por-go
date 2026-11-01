package remaster

import (
	"cmp"
	"fmt"
	"image"
	"slices"

	"gpu"

	"por/pal"

	"por/engine"
)

// Camp (View.Camp): the presenter's own screens over camp's state, each
// choice a decision for the selected member.

// Camp screens.
const (
	campMain = iota
	campRest
	campMemorize
	campScribe
	campBestiary
	campBeast // one bestiary entry
	campDrop  // the drop's confirmation

	templeHeal // the temple's services (temple.go)
	templeNot  // "is not blind": cure anyway?
	templePay  // pay for the cure?
	appraise   // the gems and jewelry (temple and shop)
	shopID     // the items to identify (shop.go)
	shopDeal   // identify this one?
)

type campUI struct {
	screen   int
	sel      int
	top      int    // the first line shown of a list too long for the screen
	byDanger bool   // the bestiary's order
	beast    string // the entry shown
	pick     int    // the temple's cure or the shop's item asked about
	icon     *gpu.Image
	iconSrc  image.Image
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
	to := func(s int) func() { return func() { c.screen, c.sel = s, 0 } }
	switch c.screen {
	case campRest:
		until := "Until night"
		if !v.Day {
			until = "Until day"
		}
		var it []menuRow
		if cp.Healing {
			it = append(it, menuRow{"Heal", 'H', act(engine.ActRest, engine.RestHeal)})
		}
		return "Rest", append(it,
			menuRow{"Memorize", 'M', act(engine.ActRest, engine.RestMemorize)},
			menuRow{"Wait an hour", 'W', act(engine.ActRest, engine.RestWait)},
			menuRow{until, 'U', act(engine.ActRest, engine.RestUntil)},
			menuRow{"Back", 'B', to(campMain)})
	case campMemorize, campScribe:
		list, title := cp.Book, "Spells to memorize"
		if c.screen == campScribe {
			list, title = cp.Scrolls, "Spells to scribe"
		}
		var it []menuRow
		for _, s := range list {
			text := fmt.Sprintf("%-18s x%d %d/%d", s.Name, s.Count, s.Planned, s.Slots)
			if c.screen == campScribe {
				text = s.Name
				if s.Count > 0 {
					text += " *"
				}
			}
			it = append(it, menuRow{text, 0, func() { p.campSpell(s.ID, 1) }})
		}
		return title, append(it, menuRow{"Back", 0, to(campMain)})
	case campBestiary:
		sort := "Sort by danger"
		if c.byDanger {
			sort = "Sort by name"
		}
		it := []menuRow{{sort, 0, func() { c.byDanger = !c.byDanger }}}
		for _, b := range p.beasts(v) {
			it = append(it, menuRow{b.Name, 0, func() { c.beast, c.screen = b.Name, campBeast }})
		}
		return "Bestiary", append(it, menuRow{"Back", 0, to(campMain)})
	case campBeast:
		for _, b := range cp.Bestiary {
			if b.Name == c.beast {
				return fmt.Sprintf("%s, danger %s: %s", b.Name, roman[b.Danger], b.Text), []menuRow{{"Back", 0, to(campBestiary)}}
			}
		}
		c.screen = campBestiary
		return p.campItems(v)
	case campDrop:
		return "Drop " + selectedName(v) + " from the party?", []menuRow{
			{"Yes", 'Y', func() { p.send(engine.ActDropMember, selectedIndex(v)); c.screen = campMain }},
			{"No", 'N', to(campMain)}}
	}
	var it []menuRow
	if cp.CanCast {
		it = append(it, menuRow{"Cast", 'C', act(engine.ActCastPick, -1)})
	}
	it = append(it, menuRow{"View", 'V', act(engine.ActMember, -1)}, menuRow{"Items", 'I', act(engine.ActInventory, -1)})
	if len(cp.Book) > 0 {
		it = append(it, menuRow{"Memorize", 'M', to(campMemorize)})
	}
	if len(cp.Scrolls) > 0 {
		it = append(it, menuRow{"Scribe", 'S', to(campScribe)})
	}
	it = append(it, menuRow{"Rest", 'R', to(campRest)}, menuRow{"Icon", 'O', act(engine.ActIconEdit, -1)}, menuRow{"Drop", 'D', func() { c.screen, c.sel = campDrop, 1 }})
	if len(cp.Bestiary) > 0 {
		it = append(it, menuRow{"Bestiary", 'B', to(campBestiary)})
	}
	return "Camp", append(it, menuRow{"Leave", 'L', act(engine.ActClose, 0)})
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
	lines, keys, off := make([]string, len(items)), rowKeys(items), rowsOff(items)
	for i, it := range items {
		lines[i] = it.text
	}
	p.camp.sel = min(max(p.camp.sel, 0), len(items)-1)
	if p.camp.screen == campBeast {
		var icon image.Image
		for _, b := range v.Camp.Bestiary {
			if b.Name == p.camp.beast {
				icon = b.Icon
			}
		}
		if icon != nil {
			if icon != p.camp.iconSrc {
				if p.camp.icon != nil {
					p.camp.icon.Deallocate()
				}
				p.camp.icon, p.camp.iconSrc = gpu.NewImageFromImage(icon), icon
			}
			s := k * 3 // the icon's pixels at three times the text's: its aspect kept
			w, h := int(s*float64(icon.Bounds().Dx())), int(s*float64(icon.Bounds().Dy()))
			dst.SubImage(image.Rect(cx-w/2, top, cx-w/2+w, top+h)).(*gpu.Image).Fill(pal.EGA(8)) // on grey: its black parts show
			op := &gpu.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(cx-w/2), float64(top))
			dst.DrawImage(p.camp.icon, op)
			top += int(s*float64(icon.Bounds().Dy())) + int(k*4)
		}
	}
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
		extra += len(wrap(title, 24)) + 1
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
	p.drawMenuIcons(dst, glyph, title, lines, keys, off, icons, c.sel-c.top, k, cx, top)
	p.menuBar(dst, p.menuRects, n, len(lines), &c.top, &c.sel, k)
}

// campKey takes key k on a camp screen.
func (p *Presenter) campKey(k uint16, v engine.View) bool {
	c := &p.camp
	_, items := p.campItems(v)
	n := len(items)
	spells := v.Camp != nil && (c.screen == campMemorize || c.screen == campScribe)
	switch {
	case spells && (k == 0x4B00 || k == 8) && c.sel < n-1: // Left, Backspace: one less
		p.campSpellAt(v, c.sel, -1)
	case spells && k == 0x4D00 && c.sel < n-1: // Right: one more
		p.campSpellAt(v, c.sel, 1)
	case k >= KeyF1 && k <= KeyF1+7<<8 && k&0xFF == 0: // F1-F8: the member camp acts for
		p.send(engine.ActLead, int(k-KeyF1)>>8)
	default:
		switch i, back, _ := menuNav(k, &c.sel, rowKeys(items)); {
		case back:
			p.campBack(v)
		case i >= 0:
			p.campPick(v, i)
		}
	}
	return true
}

// campBack goes back one level: the screen's Back, Leave, Exit or No row,
// else the screen closes. It never picks a list's line.
func (p *Presenter) campBack(v engine.View) {
	_, items := p.campItems(v)
	if n := len(items); n > 0 && slices.Contains([]string{"Back", "Leave", "Exit", "No"}, items[n-1].text) {
		p.campPick(v, n-1)
		return
	}
	p.send(engine.ActClose, 0)
}

// campSpellAt changes the plan of the spell on line i.
func (p *Presenter) campSpellAt(v engine.View, i, n int) {
	list := v.Camp.Book
	if p.camp.screen == campScribe {
		list = v.Camp.Scrolls
	}
	if i < len(list) {
		p.campSpell(list[i].ID, n)
	}
}

// campPick picks line i of the camp screen up.
func (p *Presenter) campPick(v engine.View, i int) {
	if _, items := p.campItems(v); i >= 0 && i < len(items) {
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
		return fmt.Sprintf("The %s is Valued at %d gp.", what, a.Valued), append(it, menuRow{"Sell", 'S', func() { p.send(engine.ActKeep, 0) }})
	}
	var it []menuRow
	if a.Gems > 0 {
		it = append(it, menuRow{fmt.Sprintf("Gems (%d)", a.Gems), 'G', func() { p.send(engine.ActAppraise, 0) }})
	}
	if a.Jewelry > 0 {
		it = append(it, menuRow{fmt.Sprintf("Jewelry (%d)", a.Jewelry), 'J', func() { p.send(engine.ActAppraise, 1) }})
	}
	title := "You have a fine collection of:"
	if len(it) == 0 {
		title = "Nothing to appraise"
	}
	return title, append(it, menuRow{"Back", 'B', back})
}
