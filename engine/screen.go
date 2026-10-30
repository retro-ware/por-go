package engine

import "strconv"

// The adventure screen: frames, the view window and the status line.
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

// DrawMainFrame = 00BA:272A (inner): the screen, text and view-window
// frames, plus the inner picture frame when inner is set.
func (e *Engine) DrawMainFrame(inner bool) {
	e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colWhite, "")
	e.DrawBoxTitled(1, msgTop, paneRight, paneBottom, 0, colWhite, "")
	e.DrawBoxTitled(1, 1, 15, 15, 0, colWhite, "")
	if inner {
		e.DrawBoxTitled(3, 3, 13, 13, 0, colWhite, "")
	}
	e.pg.frameUp = true
}

// DrawMainScreen = 00BA:27D9: rebuild the whole screen for DS:49F3.
func (e *Engine) DrawMainScreen() {
	switch e.beginFlag {
	case 0:
		e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colWhite, "")
	case 1:
		e.DrawMainFrame(true)
		e.LoadPortrait(e.set, int(e.es.picHead), int(e.es.picBody))
		e.DrawPortrait(3, 3, true)
	case 2:
		e.DrawMainFrame(true)
		e.LoadAnimPic("PIC", 0x1D, false)
		e.DisplayPartyList()
		e.StatusLine()
	case 4:
		e.DrawMainFrame(true)
		e.Redraw()
		e.DisplayPartyList()
		e.StatusLine()
	case 3:
		e.DrawMainFrame(false)
		e.Redraw()
		e.DisplayPartyList()
		e.StatusLine()
	case 6:
		e.DrawMainFrame(true)
		e.LoadAnimPic("PIC", 1, false)
		e.DisplayPartyList()
	}
}

// pad2 pads a number string with "0" to two characters (00BA:290B).
func pad2(n int) string {
	s := strconv.Itoa(n & 0xFFFF)
	if len(s) > 2 {
		s = s[:2]
	}
	if len(s) < 2 {
		s = "0" + s
	}
	return s
}

// StatusLine = 00BA:290B: "x,y dir hh:mm" (+ camping/search) under the
// party list.
func (e *Engine) StatusLine() {
	x, y := e.posX, e.posY
	if e.viewMode > 1 {
		x, y = int(byte(e.area.MapX)), int(byte(e.area.MapY))
		if e.viewMode == 3 {
			x += 0xD
		}
		if e.viewMode == 4 {
			x += 0x1A
		}
		x &= 0xFF
	}
	hh := pad2(int(e.area.Clock[3]))
	mm := pad2(10*int(e.area.Clock[2]) + int(e.area.Clock[1]))
	s := ""
	if e.area.NoCoords == 0 {
		s = strconv.Itoa(x) + "," + strconv.Itoa(y) + " "
	}
	s += facingNames[e.viewFacing()] + " "
	s += hh + ":" + mm
	if e.beginFlag == 2 {
		s += " camping"
	} else if e.game.Search&1 != 0 {
		s += " sweeping"
	}
	if len(s) > 0x28 {
		s = s[:0x28]
	}
	e.ClearCells(17, 15, paneRight, 15)
	e.DisplayString(17, 15, colLightGreen, s)
}

// AreaMapScreen = 00BA:2B5E: switch to the area map.
func (e *Engine) AreaMapScreen() {
	e.beginFlag = 3
	e.areaMapInit()
	e.Redraw()
	e.drawAreaMapParty()
	e.StatusLine()
	e.DisplayPartyList()
}
