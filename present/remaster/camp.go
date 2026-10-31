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
)

type campUI struct {
	screen   int
	sel      int
	byDanger bool   // the bestiary's order
	beast    string // the entry shown
	icon     *gpu.Image
	iconSrc  image.Image
}

type campItem struct {
	text string
	key  byte
	do   func()
}

// campItems are the lines of the camp screen up, and its title.
func (p *Presenter) campItems(v engine.View) (string, []campItem) {
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
		var it []campItem
		if cp.Healing {
			it = append(it, campItem{"Heal", 'H', act(engine.ActRest, engine.RestHeal)})
		}
		return "Rest", append(it,
			campItem{"Memorize", 'M', act(engine.ActRest, engine.RestMemorize)},
			campItem{"Wait an hour", 'W', act(engine.ActRest, engine.RestWait)},
			campItem{until, 'U', act(engine.ActRest, engine.RestUntil)},
			campItem{"Back", 'B', to(campMain)})
	case campMemorize, campScribe:
		list, title := cp.Book, "Spells to memorize"
		if c.screen == campScribe {
			list, title = cp.Scrolls, "Spells to scribe"
		}
		var it []campItem
		for _, s := range list {
			text := fmt.Sprintf("%-18s x%d %d/%d", s.Name, s.Count, s.Planned, s.Slots)
			if c.screen == campScribe {
				text = s.Name
				if s.Count > 0 {
					text += " *"
				}
			}
			it = append(it, campItem{text, 0, func() { p.campSpell(s.ID, 1) }})
		}
		return title, append(it, campItem{"Back", 0, to(campMain)})
	case campBestiary:
		sort := "Sort by danger"
		if c.byDanger {
			sort = "Sort by name"
		}
		it := []campItem{{sort, 0, func() { c.byDanger = !c.byDanger }}}
		for _, b := range p.beasts(v) {
			it = append(it, campItem{b.Name, 0, func() { c.beast, c.screen = b.Name, campBeast }})
		}
		return "Bestiary", append(it, campItem{"Back", 0, to(campMain)})
	case campBeast:
		for _, b := range cp.Bestiary {
			if b.Name == c.beast {
				return fmt.Sprintf("%s, danger %s: %s", b.Name, roman[b.Danger], b.Text), []campItem{{"Back", 0, to(campBestiary)}}
			}
		}
		c.screen = campBestiary
		return p.campItems(v)
	case campDrop:
		return "Drop " + selectedName(v) + " from the party?", []campItem{
			{"Yes", 'Y', func() { p.send(engine.ActDropMember, selectedIndex(v)); c.screen = campMain }},
			{"No", 'N', to(campMain)}}
	}
	var it []campItem
	if cp.CanCast {
		it = append(it, campItem{"Cast", 'C', act(engine.ActCastPick, -1)})
	}
	it = append(it, campItem{"View", 'V', act(engine.ActMember, -1)}, campItem{"Items", 'I', act(engine.ActInventory, -1)})
	if len(cp.Book) > 0 {
		it = append(it, campItem{"Memorize", 'M', to(campMemorize)})
	}
	if len(cp.Scrolls) > 0 {
		it = append(it, campItem{"Scribe", 'S', to(campScribe)})
	}
	it = append(it, campItem{"Rest", 'R', to(campRest)}, campItem{"Icon", 'O', act(engine.ActIconEdit, -1)}, campItem{"Drop", 'D', to(campDrop)})
	if len(cp.Bestiary) > 0 {
		it = append(it, campItem{"Bestiary", 'B', to(campBestiary)})
	}
	return "Camp", append(it, campItem{"Leave", 'L', act(engine.ActClose, 0)})
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
func (p *Presenter) drawCamp(dst *gpu.Image, glyph func(byte) [8]byte, v engine.View, k float64, cx, top int) {
	title, items := p.campItems(v)
	lines, keys := make([]string, len(items)), make([]byte, len(items))
	for i, it := range items {
		lines[i], keys[i] = it.text, it.key
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
	p.drawMenu(dst, glyph, title, lines, keys, p.camp.sel, k, cx, top)
}

// campKey takes key k on a camp screen.
func (p *Presenter) campKey(k uint16, v engine.View) bool {
	c := &p.camp
	_, items := p.campItems(v)
	n := len(items)
	spells := c.screen == campMemorize || c.screen == campScribe
	switch {
	case k == 0x4800:
		c.sel = (c.sel + n - 1) % n
	case k == 0x5000:
		c.sel = (c.sel + 1) % n
	case k == 13 || k == ' ':
		p.campPick(v, c.sel)
	case k == 27 && c.screen == campMain:
		p.send(engine.ActClose, 0)
	case k == 27 && c.screen == campBeast:
		c.screen = campBestiary
	case k == 27:
		c.screen, c.sel = campMain, 0
	case spells && (k == 0x4B00 || k == 8) && c.sel < n-1: // Left, Backspace: one less
		p.campSpellAt(v, c.sel, -1)
	case spells && k == 0x4D00 && c.sel < n-1: // Right: one more
		p.campSpellAt(v, c.sel, 1)
	case k >= KeyF1 && k <= KeyF1+7<<8 && k&0xFF == 0: // F1-F8: the member camp acts for
		p.send(engine.ActLead, int(k-KeyF1)>>8)
	case k < 128:
		h := byte(k)
		if h >= 'a' && h <= 'z' {
			h -= 'a' - 'A'
		}
		for i, it := range items {
			if it.key != 0 && it.key == h {
				p.campPick(v, i)
			}
		}
	}
	return true
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
