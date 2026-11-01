package engine

import (
	"image"
	"image/draw"
)

// The area map (wilderness / city overview): ovr28 and ovr27:0000.

// DrawAreaMap = ovr28:0000 (x, y): the 5x5 tiles of the area map starting at
// map square (x,y), each drawn as a 3x3 cell tile. Landmark squares listed in
// the DS:0C30 table (6 bytes each: x, y, icon) replace the tile, some only
// while the area record flags say they are revealed.
func (e *Engine) DrawAreaMap(x, y int) {
	e.snap.win = WinAreaMap
	for row := 0; row < 5; row++ {
		for col := 0; col < 5; col++ {
			e.drawTile(3*col, 3*row, e.areaTile(col+x, row+y))
		}
	}
}

// areaTile is the area map's tile at map square (tx,ty): the terrain, or a
// landmark of the DS:0C30 table (x, y, icon) while it shows.
func (e *Engine) areaTile(tx, ty int) byte {
	icon := terrainMap[int8(byte(ty))][int8(byte(tx))] - 1
	for k := 1; k <= 9; k++ {
		ent := areaMapMarks[k]
		if int(int8(byte(tx))) != int(ent[0]) || int(int8(byte(ty))) != int(ent[1]) {
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
	return icon
}

// AreaMap (extra) is the whole area map for a screen of its own: every
// tile, Tile pixels square, and the party's square and marker.
type AreaMap struct {
	Img    image.Image
	Tile   int
	Party  image.Point // the party's map square
	Marker image.Image // the party's icon, facing its way
}

// areaMapView is the whole map as View.AreaMap; the picture is kept while
// no tile changes.
func (e *Engine) areaMapView() *AreaMap {
	t := e.sqrpaci[0]
	if t == nil {
		return nil
	}
	var tiles [36][44]byte
	for y := range tiles {
		for x := range tiles[y] {
			tiles[y][x] = e.areaTile(x, y)
		}
	}
	n := t.Width * 8
	s := &e.snap
	if s.areaImg == nil || s.areaTiles != tiles || s.areaPal != e.pal {
		m := image.NewRGBA(image.Rect(0, 0, 44*n, 36*n))
		for y := range tiles {
			for x, v := range tiles[y] {
				im, f := e.sqrpaci[0], int(v)
				if v > 0x7F {
					im, f = e.sqrpaci[1], int(v&0x7F)
				}
				if im == nil || f >= im.Count {
					continue
				}
				draw.Draw(m, image.Rect(x*n, y*n, x*n+n, y*n+n), e.picRGBA(im, f, false, -1), image.Point{}, draw.Src)
			}
		}
		s.areaImg, s.areaTiles, s.areaPal = m, tiles, e.pal
	}
	v := &AreaMap{Img: s.areaImg, Tile: n, Party: image.Pt(int(viewModeShift[e.viewMode])+int(e.area.MapX), int(e.area.MapY))}
	if im := e.icons[9][0]; im != nil {
		v.Marker = e.picRGBA(im, 0, e.facing > 3, -1)
	}
	return v
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
}

// areaMapInit = ovr28:0207: the three frames of the area map screen.
func (e *Engine) areaMapInit() {
	e.frame(1, 1, 15, 15, colBlack, "")
	e.frame(17, 1, winRight, 15, colBlack, "")
	e.frame(1, msgTop, winRight, winBottom, colBlack, "")
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
