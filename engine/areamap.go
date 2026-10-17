package engine

// The area map (wilderness / city overview): ovr28 and ovr27:0000.

// DrawAreaMap = ovr28:0000 (x, y): the 5x5 tiles of the area map starting at
// map square (x,y), each drawn as a 3x3 cell tile. Landmark squares listed in
// the DS:0C30 table (6 bytes each: x, y, icon) replace the tile, some only
// while the area record flags say they are revealed.
func (e *Engine) DrawAreaMap(x, y int) {
	e.snap.win = WinAreaMap
	var sy, sx byte // [bp-2], [bp-1]: screen row and column
	for row := byte(0); row < 5; row++ {
		sx = 0
		for col := byte(0); col < 5; col++ {
			tx := byte(col + byte(x))
			ty := byte(row + byte(y))
			icon := terrainMap[int8(ty)][int8(tx)] - 1
			for k := 1; k <= 9; k++ {
				ent := areaMapMarks[k]
				if int(int8(tx)) != int(ent[0]) || int(int8(ty)) != int(ent[1]) {
					continue
				}
				set := false
				switch k {
				case 3:
					set = e.area.Mark3 != 0xFF
				case 4:
					set = e.area.Mark4 != 0 || e.area.MarkFlags4&8 != 0 || e.area.Mark4Off == 0
				case 5:
					set = e.area.MarkFlags57&2 == 0
				case 6:
					set = true
				case 7:
					set = e.area.MarkFlags57&1 == 0
				case 8, 9:
					set = e.area.Boss > 0xFD
				}
				if set {
					icon = byte(ent[2] - 1)
				}
			}
			e.drawTile(int(sx), int(sy), icon)
			sx += 3
		}
		sy += 3
	}
}

// drawAreaMapParty = ovr27:0000: the party marker on the area map. The
// marker square is the party position relative to the shown window, pinned
// to the border squares; the icon animates through 6 poses (DS:6DDC).
func (e *Engine) drawAreaMapParty() {
	v := int8(int(viewModeShift[e.viewMode]) + int(e.area.MapX))
	switch {
	case v < 2:
	case v < 0x2A:
		v = 2
	case v < 0x2B:
		v = 3
	default:
		v = 4
	}
	var y byte
	switch w := e.area.MapY; {
	case w < 2:
		y = byte(w)
	case w < 0x22:
		y = 2
	case w < 0x23:
		y = 3
	default:
		y = 4
	}
	c := e.areaIconTick
	e.drawIcon(int(v), int(y), byte(e.facing), c%2, c/2+9)
	e.areaIconTick = (c + 1) % 6
	e.Flip()
}

// areaMapInit = ovr28:0207: the three frames of the area map screen.
func (e *Engine) areaMapInit() {
	e.DrawBoxTitled(1, 1, 0xF, 0xF, 0, 0, "")
	e.DrawBoxTitled(0x11, 1, 0x26, 0xF, 0, 0, "")
	e.DrawBoxTitled(1, 0x11, 0x26, 0x16, 0, 0, "")
}

// areaMapIcons = 00BA:2B5E: switch to the area map view and draw it.
func (e *Engine) areaMapIcons() {
	e.beginFlag = 3
	e.areaMapInit()
	e.Redraw()
	e.drawAreaMapParty()
	e.StatusLine()
	e.DisplayPartyList()
}
