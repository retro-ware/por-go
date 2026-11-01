package remaster

import "por/engine"

// The character sheet (View.Sheet): the page is the engine's; its View
// menu is the presenter's.
func (p *Presenter) sheetItems(v engine.View) []campItem {
	var it []campItem
	if v.Sheet.Items {
		it = append(it, campItem{"Items", 'I', func() { p.send(engine.ActInventory, -1) }})
	}
	for _, pw := range v.Sheet.Powers {
		it = append(it, campItem{pw.Name, pw.Name[0], func() { p.send(pw.Kind, -1) }})
	}
	return append(it, campItem{"Exit", 'E', func() { p.send(engine.ActClose, 0) }})
}

// sheetUp: the sheet's menu is the page's question.
func sheetUp(v engine.View) bool { return v.Sheet != nil && askMenu(v) == nil && v.Inventory == nil }

// sheetKey takes key k on the sheet: the menu's keys, F1-F8 another member.
func (p *Presenter) sheetKey(k uint16, v engine.View) bool {
	items := p.sheetItems(v)
	n := len(items)
	switch {
	case k == 0x4800:
		p.sel = (p.sel + n - 1) % n
	case k == 0x5000:
		p.sel = (p.sel + 1) % n
	case k == 13 || k == KeyPadSelect:
		items[min(max(p.sel, 0), n-1)].do()
	case k >= KeyF1 && k <= KeyF1+7<<8 && k&0xFF == 0:
		p.send(engine.ActMember, int(k-KeyF1)>>8)
	case k < 128 && k > ' ':
		h := byte(k) &^ 0x20
		for _, it := range items {
			if it.key == h {
				it.do()
				return true
			}
		}
		return false
	default:
		return false
	}
	return true
}
