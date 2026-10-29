package engine

// Area map (GEO), wall sets (WALLDEF) and the 3D view, ported from ovr30.
//
// A GEO block is 2 header bytes and four 16x16 planes indexed y*16+x:
//
//	0x000: north wall (high nibble), east wall (low nibble)
//	0x100: south wall (high nibble), west wall (low nibble)
//	0x200: cell byte (ECL reads it as DS:6AB1)
//	0x300: door bits, 2 per direction: N 0..1, E 2..3, S 4..5, W 6..7
//
// A wall value 1..15 indexes the 15 loaded wall definitions (3 WALLDEF
// slots x 5 walls); each definition is 0x9C symbol ids laid out per view
// position (see viewPart).

const (
	walldefSize  = 0x9C     // one wall's symbol ids
	walldefSet   = 5 * 0x9C // 0x30C: one WALLDEF block chunk
	geoSize      = 0x402    // GEO block: 2-byte header + 4 planes
	symWallBase  = 0x2D     // DS:272C: ids >= this are relocated per slot
	viewX, viewY = 3, 3     // 3D view origin in screen cells: page cell 2 (the page starts one cell in)
)

type geoState struct {
	geo      [0x400]byte          // DS:6A5C
	walldef  [3 * walldefSet]byte // DS:6A58
	sky      [4]byte              // DS:6A60..6A63: sky, horizon, band, ground
	areaKind byte                 // DS:84DC: 0 or 0xA = map edges are solid
}

// dirDX/dirDY = DS:2736 / DS:273F, indexed by direction 0..8 (0 N, 2 E, 4 S, 6 W).
var (
	dirDX = [9]int{0, 1, 1, 1, 0, -1, -1, -1, 0}
	dirDY = [9]int{-1, -1, 0, 1, 1, 1, 0, -1, 0}
)

// View part tables DS:0C8C..0CD1, indexed by part 0..9 (see Draw3DView):
// first symbol in the wall definition, width and height in cells, and the
// view cell of the part's top-left corner.
var (
	partBase = [10]int{0, 2, 6, 10, 22, 38, 54, 110, 132, 154}
	partW    = [10]int{1, 1, 1, 3, 2, 2, 7, 2, 2, 1}
	partH    = [10]int{2, 4, 4, 4, 8, 8, 8, 11, 11, 2}
	partX    = [10]int{5, 4, 6, 4, 2, 7, 2, 0, 9, 5}
	partY    = [10]int{4, 3, 3, 3, 1, 1, 1, 0, 0, 4}
)

// onMap = ovr30:031F.
func onMap(x, y int) bool { return x >= 0 && x <= 15 && y >= 0 && y <= 15 }

// wrap brings an off-map coordinate back in like ovr30 does (>15 → 0, <0 → 15).
func wrap(v int) int {
	if v > 15 {
		return 0
	}
	if v < 0 {
		return 15
	}
	return v
}

// offMapSolid is the guard shared by the ovr30 accessors: off the map the
// answer is 0 when DS:84DC is 0 or 0xA, otherwise the coordinates wrap.
func (e *Engine) offMapSolid(x, y int) bool {
	return !onMap(x, y) && (e.gs.areaKind == 0 || e.gs.areaKind == 0xA)
}

// WallAt = ovr30:047B (x, y, dir): wall value 0..15 on that side of the cell.
func (e *Engine) WallAt(x, y, dir int) byte {
	if e.offMapSolid(x, y) {
		return 0
	}
	i := wrap(y)<<4 + wrap(x)
	switch dir {
	case 0:
		return e.gs.geo[i] >> 4
	case 2:
		return e.gs.geo[i] & 0xF
	case 4:
		return e.gs.geo[0x100+i] >> 4
	case 6:
		return e.gs.geo[0x100+i] & 0xF
	}
	return 0
}

// DoorAt = ovr30:0350 (x, y, dir): the 2-bit door state on that side.
// Off the map with solid edges it is 0; on the map it defaults to 1 for an
// open side (no wall).
func (e *Engine) DoorAt(x, y, dir int) byte {
	if e.offMapSolid(x, y) {
		return 0
	}
	x, y = wrap(x), wrap(y)
	if e.WallAt(x, y, dir) == 0 {
		return 1
	}
	b := e.gs.geo[0x300+y<<4+x]
	switch dir {
	case 0:
		return b & 3
	case 2:
		return b >> 2 & 3
	case 4:
		return b >> 4 & 3
	case 6:
		return b >> 6 & 3
	}
	return 1
}

// MapCell = ovr30:057A (x, y): the cell byte (plane 2).
func (e *Engine) MapCell(x, y int) byte {
	if e.offMapSolid(x, y) {
		return 0
	}
	return e.gs.geo[0x200+wrap(y)<<4+wrap(x)]
}

// SetSkyColours = ovr30:0155 (sky, horizon, band, ground).
func (e *Engine) SetSkyColours(a, b, c, d byte) { e.gs.sky = [4]byte{a, b, c, d} }

// drawSky = ovr30:0174: the four colour bands behind the walls (0464:2632
// fills x=2 cells, 11 cells wide, between the given pixel rows).
func (e *Engine) drawSky() {
	for i, band := range [4][2]int{{0x10, 0x2B}, {0x3B, 1}, {0x3C, 2}, {0x3E, 0x2A}} {
		e.FillRect(viewX-1, band[0], 0xB, band[1], e.gs.sky[i])
	}
}

// FillRect = 0464:2632 (x cells, y rows, w cells, h rows, colour), in page coordinates.
func (e *Engine) FillRect(x, y, w, h int, colour byte) {
	if e.Trace != nil {
		e.Trace("0464:2632", "FillRect", x, y, w, h, int(colour), "", "")
	}
	// The original fills the drawing page DS:4A20, one cell up and left
	// of the screen.
	e.S.Lock()
	e.fill(x*8+8, y+8, (x+w)*8+8, y+h+8, colour)
	e.S.Unlock()
	for py := y; py < y+h; py++ {
		for px := x * 8; px < (x+w)*8; px++ {
			e.pageSet(px, py, colour)
		}
	}
}

// drawWallPart = ovr30:0211 (x, y, wall, part): blit one part of a wall
// definition at view cell (x,y), clipped to the 11x11 view, masked.
func (e *Engine) drawWallPart(x, y int, wall byte, part int) {
	s := partBase[part]
	for yy := y; yy <= y+partH[part]-1; yy++ {
		for xx := x; xx <= x+partW[part]-1; xx++ {
			if yy >= 0 && yy <= 10 && xx >= 0 && xx <= 10 {
				if id := e.gs.walldef[(int(wall)-1)*walldefSize+s]; id > 0 {
					e.Put8x8SymbolMasked(xx+viewX, yy+viewY, int(id))
				}
			}
			s++
		}
	}
}

// Draw3DView = ovr30:05FD (x, y, dir): the first-person view from cell
// (x,y). Walls are drawn far to near: the row two cells ahead, the row one
// ahead, then the party's own cell, each scanned outwards from the centre
// to the left and to the right.
func (e *Engine) Draw3DView(x, y, dir int) {
	e.snap.win = WinThreeD
	e.S.Hold()
	e.pg.held++
	e.pageDraw = true
	defer e.Flip()
	defer func() { e.pageDraw = false }()
	defer func() { e.pg.held-- }()
	defer e.S.Release() // 0464:1B52 via the near thunk ovr30:05F3: show the finished frame
	e.drawSky()
	left, right, back := (dir+6)%8, (dir+2)%8, (dir+4)%8
	cx, cy := x+2*dirDX[dir], y+2*dirDY[dir]
	for dist := 2; dist >= 0; dist-- {
		switch dist {
		case 2:
			// Fronts of the 4 cells to each side (part 0), plus the short
			// side wall (part 9) where a front ends next to an open cell.
			for _, side := range [2]struct{ dir, step, edge int }{{left, -2, 1}, {right, 2, -1}} {
				px, py, off := cx, cy, 0
				var prev byte
				for i := 0; i < 4; i++ {
					front := e.WallAt(px, py, dir)
					if !onMap(px, py) && e.WallAt(px, py, (side.dir+4)%8) == 0 {
						prev = 0
					}
					if front != 0 {
						if prev > 0 {
							e.drawWallPart(partX[9]+off+side.edge, partY[9], prev, 9)
						}
						prev = front
						e.drawWallPart(partX[0]+off, partY[0], front, 0)
					} else {
						if prev > 0 && e.WallAt(px-dirDX[side.dir], py-dirDY[side.dir], side.dir) != 0 {
							e.drawWallPart(partX[9]+off+side.edge, partY[9], prev, 9)
						}
						prev = 0
					}
					off += side.step
					px += dirDX[side.dir]
					py += dirDY[side.dir]
				}
			}
			// Side walls two ahead (parts 1 left, 2 right), 3 cells outwards.
			for _, side := range [2]struct{ dir, step, part int }{{left, -2, 1}, {right, 2, 2}} {
				px, py, off := cx, cy, 0
				for i := 0; i < 3; i++ {
					if w := e.WallAt(px, py, side.dir); w != 0 {
						d := 0
						if i != 0 {
							d = side.step / 2
						}
						e.drawWallPart(partX[side.part]+off+d, partY[side.part], w, side.part)
					}
					off += side.step
					px += dirDX[side.dir]
					py += dirDY[side.dir]
				}
			}
		case 1:
			// From two cells out towards the centre: fronts (part 3) and
			// the near side walls (part 4 left, 5 right).
			for _, side := range [2]struct{ from, toward, off, step, part int }{
				{left, right, -6, 3, 4}, {right, left, 6, -3, 5}} {
				px, py, off := cx+2*dirDX[side.from], cy+2*dirDY[side.from], side.off
				for i := 0; i < 3; i++ {
					if w := e.WallAt(px, py, dir); w != 0 {
						e.drawWallPart(partX[3]+off, partY[3], w, 3)
					}
					if w := e.WallAt(px, py, side.from); w != 0 {
						e.drawWallPart(partX[side.part]+off, partY[side.part], w, side.part)
					}
					off += side.step
					px += dirDX[side.toward]
					py += dirDY[side.toward]
				}
			}
		case 0:
			// The party's own row: fronts (part 6) and the walls right
			// beside the party (part 7 left, 8 right).
			for _, side := range [2]struct{ from, toward, off, step, part int }{
				{left, right, -7, 7, 7}, {right, left, 7, -7, 8}} {
				px, py, off := cx+dirDX[side.from], cy+dirDY[side.from], side.off
				for i := 0; i < 2; i++ {
					if w := e.WallAt(px, py, dir); w != 0 {
						e.drawWallPart(partX[6]+off, partY[6], w, 6)
					}
					if w := e.WallAt(px, py, side.from); w != 0 {
						e.drawWallPart(partX[side.part]+off, partY[side.part], w, side.part)
					}
					off += side.step
					px += dirDX[side.toward]
					py += dirDY[side.toward]
				}
			}
		}
		cx += dirDX[back]
		cy += dirDY[back]
	}
}

// wallSlots = ovr30:0DE8 set {1,2,3}.
func validWallSlot(s int) bool { return s >= 1 && s <= 3 }

// LoadWalldef = ovr30:0E08 (block, slot): WALLDEF<n>.DAX block into wall
// slot 1..3. A block may hold several 5-wall chunks; they fill consecutive
// slots, and each slot gets its own symbol set (8X8D block block*10+k for
// multi-chunk blocks, else block itself). Symbol ids >= 0x2D are moved up
// to that slot's symbol range.
func (e *Engine) LoadWalldef(block, slot int) {
	if slot < 1 || slot > 3 {
		return
	}
	b := e.L.Block(e.set.file("WALLDEF"), block)
	if len(b) == 0 || slot+len(b)/walldefSet > 4 {
		e.PromptRow24(colWhite, "Unable to load wallset in LoadWallSet.")
		return
	}
	reloc := byte(symFix[slot] - symFix[1])
	n := 0
	for o := 0; o+walldefSet <= len(b); o += walldefSet {
		s := slot + n
		copy(e.gs.walldef[(s-1)*walldefSet:s*walldefSet], b[o:])
		n++
	}
	for k := 1; k <= n; k++ {
		s := slot + k - 1
		if !validWallSlot(s) {
			continue
		}
		e.mapPairA[s-1], e.mapPairB[s-1] = -1, -1
		for i := (s - 1) * walldefSet; i < s*walldefSet; i++ {
			if e.gs.walldef[i] >= symWallBase {
				e.gs.walldef[i] += reloc
			}
		}
		if n > 1 {
			bl := block
			if bl == 0 {
				bl = 0xA
			}
			e.LoadSymbolSet(s, bl*10+k)
		} else {
			e.LoadSymbolSet(s, block)
		}
	}
	if block == 0xA { // ovr30:1061: block 10 is recorded as 0 (quirk)
		block = 0
	}
	e.mapPairA[slot-1], e.mapPairB[slot-1] = block, slot
}

// LoadGeo = ovr30:10C7 (n): GEO<n>.DAX block n into the map planes.
func (e *Engine) LoadGeo(n int) {
	b := e.L.Block(e.set.file("GEO"), n)
	if len(b) != 0 && len(b) != geoSize {
		e.PromptRow24(colWhite, "Unable to load geo in Load3DMap.")
	}
	if len(b) >= geoSize {
		copy(e.gs.geo[:], b[2:geoSize])
	}
	e.area.Geo = uint16(n)
}
