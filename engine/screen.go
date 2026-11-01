package engine

// The adventure screen: the view window and what the screen is rebuilt with.
// Ported from 00BA:272A/27D9/290B and ovr27.

// Redraw = ovr27:016D: redraw the view window for the current view mode.
func (e *Engine) Redraw() {
	mode := e.viewMode
	if e.area.In3D != 0 {
		mode = 1
	}
	switch {
	case mode == 1:
		e.es.cell = e.MapCell(e.posX, e.posY)
		e.Draw3DView(e.posX, e.posY, e.viewFacing())
		e.viewDrawn()
	case mode >= 2 && mode <= 4:
		// ovr27:0250/0272: byte locals, signed clamps
		x := min(max(int(int8(int(viewModeShift[mode])+int(e.area.MapX)-2)), 0), 0x27)
		y := min(max(int(int8(int(e.area.MapY)-2)), 0), 0x1F)
		e.DrawAreaMap(x, y)
		e.drawAreaMapParty()
	}
}

// DrawMainScreen = 00BA:27D9: rebuild the whole screen for DS:49F3.
func (e *Engine) DrawMainScreen() {
	switch e.beginFlag {
	case 0:
		e.newPage()
		e.blank(1, 1, winRight, winBottom)
	case 1:
		e.newPage()
		e.LoadPortrait(e.set, int(e.es.picHead), int(e.es.picBody))
		e.DrawPortrait(3, 3, true)
	case 2:
		e.newPage()
		e.LoadAnimPic("PIC", 0x1D, false)
	case 4:
		e.newPage()
		e.Redraw()
	case 3:
		e.newPage()
		e.Redraw()
	case 6:
		e.newPage()
		e.LoadAnimPic("PIC", 1, false)
	}
}
