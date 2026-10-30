package engine

// Area map (GEO) and the 3D view's state, ported from ovr30. The view
// itself is the remaster's (present/remaster, phlan-poc/blobber): the
// original's wall drawing is gone, the engine only says where the party
// stands and looks (View.X, Y, Facing, Area).
//
// A GEO block is 2 header bytes and four 16x16 planes indexed y*16+x:
//
//	0x000: north wall (high nibble), east wall (low nibble)
//	0x100: south wall (high nibble), west wall (low nibble)
//	0x200: cell byte (ECL reads it as DS:6AB1)
//	0x300: door bits, 2 per direction: N 0..1, E 2..3, S 4..5, W 6..7
//
// A wall value 1..15 indexes the 15 wall definitions of the 3 WALLDEF
// slots; only the slots' block numbers are kept (the save file holds them).

const geoSize = 0x402 // GEO block: 2-byte header + 4 planes

type geoState struct {
	geo      [0x400]byte // DS:6A5C
	areaKind byte        // DS:84DC: 0 or 0xA = map edges are solid
}

// dirDX/dirDY = DS:2736 / DS:273F, indexed by direction 0..8 (0 N, 2 E, 4 S, 6 W).
var (
	dirDX = [9]int{0, 1, 1, 1, 0, -1, -1, -1, 0}
	dirDY = [9]int{-1, -1, 0, 1, 1, 1, 0, -1, 0}
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

// Draw3DView = ovr30:05FD (x, y, dir): the first-person view from cell
// (x,y). The remaster draws it (the page's view window stays black under
// it); here the window is set to it.
func (e *Engine) Draw3DView(x, y, dir int) {
	e.snap.win = WinThreeD
	r := InnerWindow.Pixels()
	e.fill(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, colBlack)
}

// validWallSlot = ovr30:0DE8 set {1,2,3}.
func validWallSlot(s int) bool { return s >= 1 && s <= 3 }

// LoadWalldef = ovr30:0E08 (block, slot): WALLDEF<n>.DAX block into wall
// slot 1..3. Only the slot's block is recorded (the save file keeps it);
// the walls are the remaster's.
func (e *Engine) LoadWalldef(block, slot int) {
	if !validWallSlot(slot) {
		return
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
