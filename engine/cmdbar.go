package engine

// The context-sensitive command bar: Cast is
// offered only when the selected member could cast a spell.

// commandBar drops the commands of bar that cannot be used now.
func (e *Engine) commandBar(bar Bar) Bar {
	var keep Bar
	for _, it := range bar {
		if it.ID == cmdCast && !e.canCastNow() {
			continue
		}
		keep = append(keep, it)
	}
	return keep
}

// canCastNow: Cast would offer a spell (the checks of canStudy and
// CastMenu, without their messages).
func (e *Engine) canCastNow() bool {
	c := e.selected()
	if c == nil || e.area.NoMagic > 0 || c.Status != statusOkay || c.Okay == 0 {
		return false
	}
	list, ids := e.spellList, e.spellIDs
	any := e.buildSpellList(0)
	e.spellList, e.spellIDs = list, ids
	return any
}
