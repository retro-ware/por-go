package remaster

import "por/engine"

// The character sheet (View.Sheet): the page is the engine's; its View
// menu is the presenter's.
func (p *Presenter) sheetItems(v engine.View) []menuRow {
	var it []menuRow
	if v.Sheet.Items {
		it = append(it, menuRow{"Items", 'I', func() { p.send(engine.ActInventory, -1) }})
	}
	for _, pw := range v.Sheet.Powers {
		it = append(it, menuRow{pw.Name, pw.Name[0], func() { p.send(pw.Kind, -1) }})
	}
	return append(it, menuRow{"Exit", 'E', func() { p.send(engine.ActClose, 0) }})
}

// sheetUp: the sheet's menu is the page's question.
func sheetUp(v engine.View) bool { return v.Sheet != nil && askMenu(v) == nil && v.Inventory == nil }

// sheetKey takes key k on the sheet: the menu's keys, the member keys another member.
func (p *Presenter) sheetKey(k uint16, v engine.View) bool {
	items := p.sheetItems(v)
	if m, ok := memberKey(k, selectedIndex(v), len(v.Party), true); ok {
		p.send(engine.ActMember, m)
		return true
	}
	i, back, ok := menuNav(k, &p.sel, rowKeys(items))
	switch {
	case back:
		return false // ActClose (keyAct)
	case i >= 0:
		items[i].do()
	}
	return ok
}
