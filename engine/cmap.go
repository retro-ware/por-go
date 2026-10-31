package engine

import (
	"image"

	"strings"

	"por/dax"
)

// The combat map (ovr31, ovr32, ovr33). The original drew 7x7 squares of
// the 50x25 map on its drawing page, 3x3 cells a square; that page is
// gone (the battle view draws the map from the snapshot, battle.go), but
// its scrolling window stays as what the view looks at (Battle.View).

// combatMap is the 0x4E9-byte record at [DS:6716].
type combatMap struct {
	viewX, viewY int          // +2/+3 top left square of the view
	cursor       byte         // +4 draw the cursor icon
	cursorSize   byte         // +5 cursor footprint
	seeAll       byte         // +6 line of sight ignores walls
	ground       [25][50]byte // +7 ground type per square
}

// combatPos is the 4-byte DS:5F27 record of combatant i.
type combatPos struct{ x, y, id, size byte }

// body is a 7-byte DS:66D6 record: who fell where, and the ground it hid.
type body struct {
	c      *Character
	x, y   byte
	ground byte
}

const pageCells = 21 // the drawing page, 21x21 cells (outside combat)

// The combat view window (extra): viewW x viewH squares, scrolled as the
// original scrolled its 7x7 (recentre); the battle view's camera follows it.
const viewW, viewH = 12, 7

// footprint = ovr32:0000 (size, part, var dx, var dy): offset of one square
// of a combatant that covers several (DS:2844).
func (e *Engine) footprint(size, part int) (int, int, bool) {
	if size == 0 {
		return 0, 0, false
	}
	b := footprintTable[size][part]
	return int(int8(b[0])), int(int8(b[1])), int8(b[0]) >= 0
}

// groundInfo = DS:2744: 4 bytes per ground type: move cost (0xFF wall),
// view height, blocking height, tile.
func (e *Engine) groundInfo(g byte) []byte { return groundTable[g][:] }

// groundFloor is the ground both combat maps lay as bare floor.
const groundFloor = 0x17

// tileOf is the tile of map square (x, y). The original indexes the 50-wide
// grid flat, so a square just off one edge showed the far edge's pieces; here
// off the map is bare floor (ground 0 would be a wall piece).
func (e *Engine) tileOf(x, y int) byte {
	if x < 0 || y < 0 || x >= 50 || y >= 25 { // off the map (the wide page shows past the edges): bare floor, never wrapped
		return e.groundInfo(groundFloor)[3]
	}
	return e.groundInfo(e.cmap.ground[y][x])[3]
}

// drawPage = 0464:0D71 draws a frame at page cell (x,y), clipped to the
// page (which starts a cell in). masked skips the image's mask colour.
// Outside combat only (pictures, animations, the area map, the icon
// editor): the combat page is gone.
func (e *Engine) drawPage(x, y int, im *dax.Image, frame int, masked bool) {
	if im == nil || frame >= im.Count {
		return
	}
	m := -1
	if masked {
		m = im.Mask
	}
	e.picture(8+x*8, 8+y*8, im.Width*8, im.Height, im.Frame(frame), m)
	e.pg.layers[len(e.pg.layers)-1].clip = image.Rect(8, 8, 8+pageCells*8, 8+pageCells*8)
}

// drawTile = ovr33:00F1 (x, y, tile, page): a ground tile from the tile
// sets (DS:5E52, tiles from 0x80 in DS:5E56); the area map's icons.
func (e *Engine) drawTile(x, y int, t byte) {
	im, f := e.sqrpaci[0], int(t)
	if t > 0x7F {
		im, f = e.sqrpaci[1], int(t&0x7F)
	}
	e.drawPage(x, y, im, f, false)
}

// drawIcon = ovr33:04FC (x, y, dir, pose, slot): a combat icon on page
// square (x,y), mirrored when facing directions 4..7 (the area map's party
// marker, the icon editor).
func (e *Engine) drawIcon(x, y int, dir, pose, slot byte) {
	im := e.icons[slot][pose&1]
	if im == nil {
		return
	}
	if dir > 3 {
		im = im.Clone()
		im.FlipLeftToRight()
	}
	e.drawPage(x*3, y*3, im, 0, true)
}

// LoadBackpacIcons = ovr33:0022 (name, first, n): block 1 of the DAX file
// copied into the tile set from tile first on, frames 0..n (the Amiga's
// RANDCOM has two more, which would land on the tiles after it).
func (e *Engine) LoadBackpacIcons(name string, first, n int) {
	if first > 0x7F {
		return // the original stops with a fatal error
	}
	im := e.LoadPic(name, 1)
	t := e.sqrpaci[0]
	if im == nil || t == nil {
		return
	}
	for f := 0; f <= n && f < im.Count && first+f < t.Count && im.BPP == t.BPP; f++ {
		copy(t.Frame(first+f), im.Frame(f))
	}
}

// recolour = 0464:1E5C with DS:0CD2 → to: map the colours of frame 0 (the
// original takes a frame number, always passed 0, so only that frame changes).
func (e *Engine) recolour(im *dax.Image, tt *[16]byte) {
	if im == nil || im.Count == 0 {
		return
	}
	from := iconPalette
	d := im.Frame(0)
	for i, p := range d {
		for k := 0; k < 16; k++ {
			if from[k] == p {
				d[i] = tt[k]
				break
			}
		}
	}
}

// LoadIconPic = ovr33:01C8 (name, block, slot): the two poses of a combat
// icon, block and block+0x80, loaded masked on colour 0.
func (e *Engine) LoadIconPic(name string, block, slot int) {
	up := strings.ToUpper(name)
	load := func(n string, b int) *dax.Image { return e.LoadPicMasked(n, b, 0) }
	ic := &e.icons[slot]
	switch {
	case strings.HasPrefix(up, "CHEAD") || strings.HasPrefix(up, "CBODY"):
		if up[len(up)-1] == 'T' {
			block += 0x40
		}
		n := name[:len(name)-1]
		ic[0], ic[1] = load(n, block), load(n, block+0x80)
	case up == "COMSPR" || up == "ICON":
		ic[0], ic[1] = load(name, block), load(name, block+0x80)
		if up == "ICON" {
			e.recolour(ic[0], &iconPaletteCF2)
			e.recolour(ic[1], &iconPaletteCF2)
		}
	default:
		n := e.set.file(name)
		ic[0], ic[1] = load(n, block), load(n, block+0x80)
		e.recolour(ic[0], &iconPaletteCE2)
		e.recolour(ic[1], &iconPaletteCE2)
	}
}

// freeIcon = ovr33:014E (slot).
func (e *Engine) freeIcon(n byte) { e.icons[n] = [2]*dax.Image{} }

// iconToSprite = 00BA:1892 (slot, pose, frame, flip): copy an icon into a
// frame of the DS:6E36 sprite.
func (e *Engine) iconToSprite(slot, pose, frame int, flip bool) {
	im := e.icons[slot][pose]
	if im == nil {
		return
	}
	e.sprCursor = slot == 0x19 // the aim cursor: Next/Prev fly it from target to target
	if e.spr == nil || e.spr.BPP != im.BPP {
		e.spr = dax.NewImage(4, im.Width, im.Height)
	}
	if flip {
		im = im.Clone()
		im.FlipLeftToRight()
	}
	e.spr.Mask = im.Mask
	copy(e.spr.Frame(frame), im.Frame(0))
}

// hitColours = 00BA:19DE (slot): the four frames of the sprite from an icon.
func (e *Engine) hitColours(slot int) {
	e.iconToSprite(slot, 0, 0, false)
	e.iconToSprite(slot, 0, 1, true)
	e.iconToSprite(slot, 1, 2, true)
	e.iconToSprite(slot, 1, 3, false)
}

// drawHitFrame / restoreHitFrame = 0464:0D71 with DS:6E36 (mode 5) and
// DS:4A24 (mode 0): the sprite over the map at page thirds (x, y), now only
// for the snapshot (Battle.Effects).
func (e *Engine) drawHitFrame(f, x, y int) { e.showEffect(x, y, f, e.spr) }
func (e *Engine) restoreHitFrame(x, y int) { e.hideEffect(x, y) }

// combatViewEnter = ovr32:0105 / combatViewLeave = ovr32:00E3: the EGA
// palette entries 0 and 8 swap for the combat screen (EGA only, DS:4A1E == 1).
func (e *Engine) combatViewEnter() {
	if e.Cfg.Graphics == 'E' {
		e.SetPaletteColor(0, 0)
		e.SetPaletteColor(8, 8)
	}
}

func (e *Engine) combatViewLeave() {
	if e.Cfg.Graphics == 'E' {
		e.SetPaletteColor(8, 0)
		e.SetPaletteColor(0, 8)
	}
}

// updateScreenPos = ovr32:0068: screen squares of every combatant.
func (e *Engine) updateScreenPos() {
	for i := 1; i <= e.ncomb; i++ {
		e.combatScreenX[i] = int8(int(e.cpos[i].x) - e.cmap.viewX)
		e.combatScreenY[i] = int8(int(e.cpos[i].y) - e.cmap.viewY)
	}
}

// drawCursor = ovr32:0127 (x, y): the cursor on (x, y), with its icon when
// cmap.cursor is set (the snapshot's Cursor; the view draws it).
func (e *Engine) drawCursor(x, y int) {
	e.snap.cursor, e.snap.cursorOn = image.Pt(x, y), e.cmap.cursor != 0
}

// eraseCursor = ovr32:02D5 (x, y).
func (e *Engine) eraseCursor(x, y int) { e.snap.cursorOn = false }

// combatRedrawMap = ovr32:03A0: rebuild the occupancy grid (DS:60DB).
func (e *Engine) combatRedrawMap() {
	e.occ = [25][50]byte{}
	clear(e.snap.hidden)
	for i := 1; i <= e.ncomb; i++ {
		p := e.cpos[i]
		if p.size == 0 {
			continue
		}
		for part := 0; part <= 3; part++ {
			if dx, dy, ok := e.footprint(int(p.size), part); ok {
				x, y := int(p.x)+dx, int(p.y)+dy
				if x >= 0 && x < 50 && y >= 0 && y < 25 {
					e.occ[y][x] = byte(i)
				}
			}
		}
		e.combatScreenX[i] = int8(int(p.x) - e.cmap.viewX)
		e.combatScreenY[i] = int8(int(p.y) - e.cmap.viewY)
	}
}

// mapAt = ovr32:04BE (x, y, var who, var ground).
func (e *Engine) mapAt(x, y int) (byte, byte) {
	if x < 0 || x > 0x31 || y < 0 || y > promptRow {
		return 0, 0
	}
	return e.occ[y][x], e.cmap.ground[y][x]
}

// combatInView = ovr32:06E9 (x, y): a screen square inside the view.
func combatInView(x, y int) bool { return x >= 0 && x < viewW && y >= 0 && y < viewH }

// combatOnScreen = ovr32:071A (char, all): any (or with all, every) square
// of the combatant in view.
func (e *Engine) combatOnScreen(c *Character, all bool) bool {
	idx := e.combatIndex(c)
	if e.cpos[idx].size == 0 {
		return false
	}
	r := true
	for part := 0; part <= 3; part++ {
		dx, dy, ok := e.footprint(int(e.cpos[idx].size), part)
		if !ok {
			continue
		}
		if !combatInView(int(e.combatScreenX[idx])+dx, int(e.combatScreenY[idx])+dy) {
			r = false
			if all {
				return r
			}
		} else {
			r = true
			if !all {
				return r
			}
		}
	}
	return r
}

// recentre = ovr32:07D4 (x, y, margin): scroll the view so (x,y) is at
// least margin squares inside it (0xFF: always centre).
func (e *Engine) recentre(x, y, r int) bool {
	const half = viewW / 2
	cx, cy := e.cmap.viewX+half, e.cmap.viewY+3
	rr := r
	if r == 0xFF {
		rr = 0
	}
	if r != 0xFF && x >= cx-rr && x <= cx+rr && y >= cy-rr && y <= cy+rr {
		return false
	}
	if x < cx-rr {
		for x < cx && cx > half {
			cx--
		}
	} else if x > cx+rr {
		for x > cx && cx < 0x31-(viewW-1-half) {
			cx++
		}
	}
	if y < cy-rr {
		for y < cy && cy > 3 {
			cy--
		}
	} else if y > cy+rr {
		for y > cy && cy < 0x15 {
			cy++
		}
	}
	e.cmap.viewX, e.cmap.viewY = cx-half, cy-3
	e.cmapShown()
	e.updateScreenPos()
	return true
}

// combatCentre = ovr32:0996 (x, y, margin, dir): bring the square next to
// (x,y) in direction dir (8 = itself) into view and put the cursor there.
func (e *Engine) combatCentre(x, y, r, dir int) {
	nx, ny := x+dirDX[dir], y+dirDY[dir]
	e.recentre(nx, ny, r)
	e.eraseCursor(x, y)
	if !combatInView(nx-e.cmap.viewX, ny-e.cmap.viewY) {
		nx = min(max(nx, 0), 0x31)
		ny = min(max(ny, 0), 0x18)
	}
	e.drawCursor(nx, ny)
}

// combatTurn = ovr32:0AF8 (char, dir, pose, hide): face dir and show the
// icon in the given pose (not with hide).
func (e *Engine) combatTurn(c *Character, dir, pose byte, hide bool) {
	idx := e.combatIndex(c)
	if !e.combatOnScreen(c, true) {
		e.combatCentre(e.combatX(c), e.combatY(c), 3, 8)
	}
	c.Combat.Facing = dir
	if hide {
		e.unitHidden(idx)
	}
	if !hide && e.combatOnScreen(c, true) {
		e.unitDrawn(idx, pose)
	}
}

func (e *Engine) combatX(c *Character) int    { return int(e.cpos[e.combatIndex(c)].x) }    // ovr32:0BE7
func (e *Engine) combatY(c *Character) int    { return int(e.cpos[e.combatIndex(c)].y) }    // ovr32:0C0F
func (e *Engine) combatSize(c *Character) int { return int(e.cpos[e.combatIndex(c)].size) } // ovr32:0C37

// combatIndex = ovr32:0C5F (char): its index in DS:65B9, 0 if none.
func (e *Engine) combatIndex(c *Character) int {
	for i := 1; i <= e.ncomb; i++ {
		if e.combatant(byte(i)) == c {
			return i
		}
	}
	return 0
}

// groundAt = ovr32:0CB9 (char, dir, var who, var ground): who stands on
// and the dearest ground of the squares the combatant would cover one step
// in direction dir (8: where it stands). 0 = off the map, 0x1E = cloud.
func (e *Engine) groundAt(c *Character, dir int) (byte, byte) {
	var who byte
	gr := byte(0x17)
	best := byte(1)
	idx := e.combatIndex(c)
	for part := 0; part <= 3; part++ {
		dx, dy, ok := e.footprint(int(e.cpos[idx].size), part)
		if !ok {
			continue
		}
		o, g := e.mapAt(int(e.cpos[idx].x)+dx+dirDX[dir], int(e.cpos[idx].y)+dy+dirDY[dir])
		if int(o) == idx {
			o = 0
		}
		if o > 0 {
			who = o
		}
		switch {
		case g == 0 || gr == 0:
			gr = 0
		case g == 0x1E || gr == 0x1E:
			gr = 0x1E
		case e.groundInfo(g)[0] >= best:
			best = e.groundInfo(g)[0]
			gr = g
		}
	}
	return who, gr
}

// combatRemoveFromMap = ovr32:0E11 (char): a fallen combatant leaves the
// map; unless summoned, a body (ground 0x1F) stays where it fell.
func (e *Engine) combatRemoveFromMap(c *Character) {
	e.soundWait(0)
	if e.beginFlag != 5 {
		e.playSound(sndFall, e.fallEvent(c))
		e.MsgDelay()
		e.soundWait(0xA)
		return
	}
	if int(e.bodyIndex) < len(e.bodies) && e.bodies[e.bodyIndex].c == c {
		return // the original's search (DS:70C7, not its loop index)
	}
	idx := e.combatIndex(c)
	x, y := e.combatX(c), e.combatY(c)
	if !e.combatOnScreen(c, true) {
		e.combatCentre(x, y, 3, 8)
	}
	if c.Combat.Extra == 0 && e.nbodies+1 < len(e.bodies) {
		e.nbodies++
		b := &e.bodies[e.nbodies]
		b.ground = e.cmap.ground[y][x]
		if e.cmap.ground[y][x] != 0x1E {
			e.cmap.ground[y][x] = 0x1F
		}
		b.c, b.x, b.y = c, byte(x), byte(y)
	}
	e.playSound(sndFall, e.fallEvent(c))
	e.MsgDelay()
	e.cpos[idx].size = 0
	e.combatRedrawMap()
	e.combatCentre(e.cmap.viewX+viewW/2, e.cmap.viewY+3, 3, 8)
	ci := c.Combat
	ci.Delay, ci.Moves, ci.Spell, ci.Guarding = 0, 0, 0, 0
	e.soundWait(0xA)
}

// combatSquareFree = ovr32:10A7 (char, x, y, uncover): put the combatant on
// (x,y) if every square it needs is free ground; with uncover, its body is
// taken off the map.
func (e *Engine) combatSquareFree(c *Character, x, y int, uncover bool) bool {
	if e.beginFlag != 5 {
		return true
	}
	idx := e.combatIndex(c)
	p := &e.cpos[idx]
	p.size = c.Size & 0x7F
	p.x, p.y = byte(x), byte(y)
	who, g := e.groundAt(c, 8)
	if who != 0 || g == 0 || e.groundInfo(g)[0] == 0xFF {
		p.size = 0
		return false
	}
	if uncover && c.Combat.Extra == 0 {
		for k := 1; k <= e.nbodies; k++ {
			if e.bodies[k].c == c {
				if e.bodies[k].ground != 0x1F {
					g = e.bodies[k].ground
				}
				e.bodies[k] = body{}
			}
		}
		covered := false
		for k := 1; k <= e.nbodies; k++ {
			if e.bodies[k].c != nil && int(e.bodies[k].x) == x && int(e.bodies[k].y) == y {
				covered = true
			}
		}
		if !covered {
			e.cmap.ground[y][x] = g
		}
	}
	e.combatRedrawMap()
	return true
}

// combatRedrawChar = ovr32:1298 (char, margin, cursor): centre on the
// combatant, with the cursor icon over all its squares when asked.
func (e *Engine) combatRedrawChar(c *Character, r int, cursor byte) {
	e.cmap.cursor = cursor
	e.cmap.cursorSize = byte(e.combatSize(c))
	e.combatCentre(e.combatX(c), e.combatY(c), r, 8)
	e.cmap.cursor, e.cmap.cursorSize = 0, 1
}

// nearSort = ovr31:002E: order the near list by distance.
func (e *Engine) nearSort() {
	l := e.near
	for i := 0; i < len(l)-1; i++ {
		for j := i + 1; j < len(l); j++ {
			a, b := l[i], l[j]
			if b.dist < a.dist || (b.dist == a.dist && b.dir < a.dir && b.dir%2 <= a.dir%2) {
				l[i], l[j] = b, a
			}
		}
	}
}

// lineOfSight = ovr31:0422 (map, var range, var tx, var ty, x, y): walk from
// (x,y) towards (tx,ty) until the target, a wall higher than the line or
// the range (in half squares) stops it.
func (e *Engine) lineOfSight(rng *int, tx, ty *int, x, y int) bool {
	limit := *rng*2 + 1
	a := newLine(x, y, *tx, *ty)
	h := int(e.groundInfo(e.cmap.ground[y][x])[1])
	b := newLine(0, h, max(a.dx, a.dy), h)
	for {
		if (e.cmap.seeAll == 0 && int(e.groundInfo(e.cmap.ground[a.y][a.x])[2]) > b.y) || a.steps&0xFF > limit {
			*tx, *ty = a.x, a.y
			*rng = a.steps & 0xFF
			return false
		}
		e.step(b)
		if !e.step(a) {
			*rng = a.steps & 0xFF
			return true
		}
	}
}

// inArc = ovr31:057B (x1, y1, x2, y2, dir): (x2,y2) lies in the quarter
// facing dir from (x1,y1) (dir 8: anywhere).
func (e *Engine) inArc(x1, y1, x2, y2, dir int) bool {
	if x1 < 0 || x1 > 0x31 || y1 < 0 || y1 > promptRow || x2 < 0 || x2 > 0x31 || y2 < 0 || y2 > promptRow {
		return false
	}
	if dir == 0xFF {
		dir = 8
	}
	nx, ny := x1+dirDX[dir], y1+dirDY[dir]
	if (x1 == x2 && y1 == y2) || (nx == x2 && ny == y2) {
		return true
	}
	switch dir {
	case 0:
		return (x2 >= nx && y2 <= ny+nx-x2) || (x2 <= nx && y2 <= ny+x2-nx)
	case 1:
		return (x2 >= nx && y2 <= ny+nx-x2) || (x2 >= nx+ny-y2 && y2 <= ny)
	case 2:
		return (x2 >= nx+ny-y2 && y2 <= ny) || (x2 >= nx+y2-ny && y2 >= ny)
	case 3:
		return (x2 >= nx+y2-ny && y2 >= ny) || (x2 >= nx && y2 >= ny+x2-nx)
	case 4:
		return (x2 >= nx && y2 >= ny+x2-nx) || (x2 <= nx && y2 >= ny+nx-x2)
	case 5:
		return (x2 <= nx && y2 >= ny+nx-x2) || (x2 <= nx+ny-y2 && y2 >= ny)
	case 6:
		return (x2 <= nx+ny-y2 && y2 >= ny) || (x2 <= nx+y2-ny && y2 <= ny)
	case 7:
		return (x2 <= nx+y2-ny && y2 <= ny) || (x2 <= nx && y2 <= ny+x2-nx)
	}
	return true
}

// buildNear = ovr31:0909 (x, y, range, dir, size, map): every combatant on
// the map seen from the squares of a combatant of that size at (x,y),
// within range and the quarter dir (0xFF: all round), nearest first.
func (e *Engine) buildNear(x, y, rng, dir, size int) {
	var sx, sy [4]int
	var sok [4]bool
	for k := 0; k <= 3; k++ {
		dx, dy, ok := e.footprint(size, k)
		sx[k], sy[k], sok[k] = x+dx, y+dy, ok
	}
	e.near = e.near[:0]
	for i := 1; i <= e.ncomb; i++ {
		p := e.cpos[i]
		if p.size == 0 {
			continue
		}
		var tx, ty [4]int
		var tok [4]bool
		for m := 0; m <= 3; m++ {
			dx, dy, ok := e.footprint(int(p.size), m)
			tx[m], ty[m], tok[m] = int(p.x)+dx, int(p.y)+dy, ok
		}
		found, best, bk, bm := false, 0xFF, 0, 0
		for m := 0; m <= 3; m++ {
			if !tok[m] {
				continue
			}
			for k := 0; k <= 3; k++ {
				if !sok[k] || !e.inArc(sx[k], sy[k], tx[m], ty[m], dir) {
					continue
				}
				ax, ay, r := tx[m], ty[m], rng
				if e.lineOfSight(&r, &ax, &ay, sx[k], sy[k]) {
					found = true
					if r < best {
						best, bk, bm = r, k, m
					}
				}
			}
		}
		if !found {
			continue
		}
		d := 0
		if dir < 8 {
			d = dir
		} else {
			for !e.inArc(sx[bk], sy[bk], tx[bm], ty[bm], d) {
				d++
			}
		}
		e.near = append(e.near, nearEntry{byte(i), byte(best), byte(d)})
	}
	e.nearSort()
}

// nearCombatants = ovr32:0C37 + ovr31:0909: everyone the combatant at (x,y)
// sees within range r, all round.
func (e *Engine) nearCombatants(c *Character, x, y, r int) {
	e.buildNear(x, y, r, 0xFF, e.combatSize(c))
}

// BuildCharIcon = ovr18:0482 (recolour): the character's combat icon: the
// CBODY poses with the CHEAD poses laid over them (ovr18:032F), in icon
// slot +0xBF; with recolour the six icon colour pairs (+0xC1..) replace
// the DS:3CF5 colours. The original works on the selected character.
func (e *Engine) BuildCharIcon(c *Character, recolour bool) {
	size := string("?ST"[c.IconSize])
	slot := int(c.Order)
	e.LoadIconPic("CHEAD"+size, int(c.IconHead), 0xB)
	e.LoadIconPic("CBODY"+size, int(c.IconBody), slot)
	for p := 0; p < 2; p++ {
		head, body := e.icons[0xB][p], e.icons[slot][p]
		if head == nil || body == nil {
			continue
		}
		// data planes are ORed, mask planes ANDed: a transparent pixel is
		// all-zero data, opaque where either picture is
		for i, v := range head.Data {
			if i >= len(body.Data) || int(v) == head.Mask {
				continue
			}
			if int(body.Data[i]) == body.Mask {
				body.Data[i] = v
			} else {
				body.Data[i] |= v
			}
		}
	}
	if recolour {
		var to [16]byte
		for i := range to {
			to[i] = byte(i)
		}
		pos := defaultIconColours
		for i := 1; i <= 6; i++ {
			v := c.IconColour[i-1]
			to[pos[i]] = v & 0xF
			to[int(pos[i])+8] = v >> 4
		}
		for p := 0; p < 2; p++ {
			recolourIcon(e.icons[slot][p], to)
		}
	}
	e.freeIcon(0xB)
}
