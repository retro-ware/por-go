package engine

import "strconv"

// The adventure screen: frames, the view window and the status line.
// Ported from 00BA:272A/27D9/290B and ovr27.

// skyFg = ovr27:00C7, the horizon colour for DS:6F69.
func skyFg(v byte) byte {
	switch v {
	case 8, 0x41:
		return 0
	}
	return 0xF
}

// skyBand = ovr27:00FB, the band colour for DS:6F69.
func skyBand(v byte) byte {
	if v == 8 {
		return 8
	}
	return 7
}

// skyGround = ovr27:012F, the ground colour for DS:6F68.
func skyGround(v byte) byte {
	switch v {
	case 0xB:
		return 8
	case 6:
		return 1
	}
	return 6
}

// Redraw = ovr27:016D: redraw the view window for the current view mode.
func (e *Engine) Redraw() {
	mode := e.viewMode
	if e.area.In3D != 0 {
		mode = 1
	}
	switch {
	case mode == 1:
		e.es.cell = e.MapCell(e.posX, e.posY)
		sky := e.area.Sky
		if e.es.cell > 0x7F {
			sky = e.area.SkyInside
		}
		s := e.gs.sky
		s[0] = skyColour[sky]
		if e.es.f84E5 != 0 {
			s[1] = skyFg(e.es.v6F69)
			s[2] = skyBand(e.es.v6F69)
			s[3] = skyGround(e.es.v6F68)
		}
		e.SetSkyColours(s[0], s[1], s[2], s[3])
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
	e.DrawBoxTitled(1, 1, 0x26, 0x16, 0, 0xF, "")
	e.DrawBoxTitled(1, 0x11, 0x26, 0x16, 0, 0xF, "")
	e.DrawBoxTitled(1, 1, 0xF, 0xF, 0, 0xF, "")
	if inner {
		e.DrawBoxTitled(3, 3, 0xD, 0xD, 0, 0xF, "")
	}
	e.frameDrawn()
}

// DrawMainScreen = 00BA:27D9: rebuild the whole screen for DS:49F3.
func (e *Engine) DrawMainScreen() {
	switch e.beginFlag {
	case 0:
		e.DrawBoxTitled(1, 1, 0x26, 0x16, 0, 0xF, "")
	case 1:
		e.DrawMainFrame(true)
		e.LoadPortrait(int(e.es.f49FD), int(e.es.f49FE))
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
	e.ClearCells(0x11, 0xF, 0x26, 0xF)
	e.DisplayString(0x11, 0xF, 0xA, s)
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
