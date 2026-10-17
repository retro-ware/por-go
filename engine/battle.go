package engine

import (
	"image"
	"image/color"
	"slices"

	"por/dax"
	"por/ega"
)

// Battle is the combat map as the remaster's 3D view needs it.
type Battle struct {
	Ground   [25][50]Square
	Tiles    []image.Image // the tile set by tile number, in combat's colours
	Floor    color.RGBA    // the ground's colour
	Dungeon  bool          // tiles 0..DungeonTiles-1 are stone wall pieces (else outdoor things: trees, water ...)
	Units    []Unit
	Cursor   image.Point   // the square the cursor was last drawn on
	CursorOn bool          // the cursor icon is up (aiming, looking)
	CursorAt []image.Point // the squares the cursor covers (a large combatant's footprint)
	Marks    []image.Point // squares framed for a choice (reach of a move, an area of effect)
	Effects  []Effect      // sprites over the map: a missile, a hit, a fall
	View     image.Point   // top left square the original shows
}

// Effect is a sprite the original draws over the map for a moment.
type Effect struct {
	X, Y float64     // its top left corner, in squares (missiles move in thirds)
	Pic  image.Image // in combat's colours
}

// DungeonTiles is how many tiles DungCom puts at the start of the set.
const DungeonTiles = 0x18 + 1

// Square is one map square.
type Square struct {
	Tile   int
	Wall   bool // move cost 0xFF
	Height int  // blocking height
}

// Unit is one combatant on the map.
type Unit struct {
	ID         int // its place on the combat list: the same unit from snapshot to snapshot
	X, Y, Size int
	Party      bool
	Facing     int         // 0..7
	Icon       image.Image // the combat icon, facing and pose (striking) as the original draws it
	Active     bool        // whose turn it is
	Name       string
}

// battle builds the snapshot's Battle (nil outside combat).
func (e *Engine) battle() *Battle {
	s := &e.snap
	if e.beginFlag != 5 || e.sqrpaci[0] == nil || !e.cmapUp() {
		return nil
	}
	b := &Battle{
		Cursor: s.cursor, CursorOn: e.cmap.cursor != 0,
		Marks: s.marks, Dungeon: s.dungeon, View: image.Pt(e.cmap.viewX, e.cmap.viewY),
	}
	for part := 0; part <= 3; part++ {
		if dx, dy, ok := e.footprint(int(e.cmap.cursorSize), part); ok {
			b.CursorAt = append(b.CursorAt, s.cursor.Add(image.Pt(dx, dy)))
		}
	}
	for _, f := range s.effects {
		b.Effects = append(b.Effects, Effect{float64(f.x) / 3, float64(f.y) / 3, e.picRGBA(f.im, f.frame, false, -1)})
	}
	for y := range b.Ground {
		for x := range b.Ground[y] {
			g := e.groundInfo(e.cmap.ground[y][x])
			b.Ground[y][x] = Square{Tile: int(g[3]), Wall: g[0] == 0xFF, Height: int(g[2])}
		}
	}
	ground := groundColour(e.sqrpaci[0])
	b.Floor = e.S.RGBA(ground)
	b.Tiles = make([]image.Image, 0x100)
	for t, im := range e.sqrpaci {
		for f := 0; im != nil && f < min(im.Count, 0x80); f++ {
			b.Tiles[t*0x80+f] = e.picRGBA(im, f, false, int(ground))
		}
	}
	var active *Character
	if e.partySel < len(e.party) {
		active = e.party[e.partySel]
	}
	for i := 1; i <= e.ncomb; i++ {
		p, c := e.cpos[i], e.combatant(byte(i))
		if p.size == 0 || c == nil || c.Combat == nil || s.hidden[i] {
			continue
		}
		u := Unit{ID: i, X: int(p.x), Y: int(p.y), Size: int(p.size), Party: isParty(c),
			Facing: int(c.Combat.Facing), Active: c == active, Name: c.Name()}
		if im := e.icons[c.Order][s.poses[i]&1]; im != nil {
			u.Icon = e.picRGBA(im, 0, u.Facing > 3, -1)
		}
		b.Units = append(b.Units, u)
	}
	return b
}

// groundColour is the pixel value the combat tiles paint the bare ground
// in: their most common one (0 on DOS, a slot's grey on the Amiga).
func groundColour(im *dax.Image) uint8 {
	var n [256]int
	for _, p := range im.Data {
		n[p]++
	}
	best := 0
	for c := range n {
		if c != dax.Clear && n[c] > n[best] {
			best = c
		}
	}
	return uint8(best)
}

type picKey struct {
	pix    string // the frame's pixels: frames change in place (the hit sprite, backpack icons)
	w      int
	flip   bool
	ground int  // the ground's pixel value, transparent (-1: none)
}

// picRGBA converts a frame in the screen's current palette (combat swaps
// colours 0 and 8), Clear pixels (and with ground, colour 0) transparent. Cached per picture and palette.
func (e *Engine) picRGBA(im *dax.Image, f int, flip bool, ground int) image.Image {
	s := &e.snap
	if s.pics == nil || s.picsPal != e.S.Pal {
		s.pics, s.picsPal = map[picKey]image.Image{}, e.S.Pal
	}
	k := picKey{string(im.Frame(f)), im.Width, flip, ground}
	if out, ok := s.pics[k]; ok {
		return out
	}
	w, h := im.Width*8, im.Height
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, p := range im.Frame(f) {
		x := i % w
		if flip {
			x = w - 1 - x
		}
		if p == dax.Clear || int(p) == ground {
			out.SetRGBA(x, i/w, color.RGBA{})
			continue
		}
		out.SetRGBA(x, i/w, e.S.RGBA(p))
	}
	s.pics[k] = out
	return out
}

// cmapRow is the pixel row under the combat page: the frame's bottom ribbon.
const cmapRow = 8 + viewH*24 + 2

// cmapShown remembers the frame row under the combat page as the map is
// drawn, so publish can tell the map is still up (sheets draw over it).
func (e *Engine) cmapShown() {
	if e.S == nil { // unit tests without a screen
		return
	}
	e.S.Lock()
	e.snap.cmapRow = append(e.snap.cmapRow[:0], e.S.Pix[cmapRow*ega.W:(cmapRow+1)*ega.W]...)
	e.S.Unlock()
}

func (e *Engine) cmapUp() bool {
	e.S.Lock()
	defer e.S.Unlock()
	return len(e.snap.cmapRow) > 0 && string(e.S.Pix[cmapRow*ega.W:(cmapRow+1)*ega.W]) == string(e.snap.cmapRow)
}

// effect is a sprite on the combat page: x, y in thirds of a square on the map.
type effect struct {
	x, y, frame int
	im          *dax.Image
}

// showEffect / hideEffect record the hit sprite at page thirds (px, py).
func (e *Engine) showEffect(px, py, frame int, im *dax.Image) {
	e.snap.effects = append(e.snap.effects, effect{e.cmap.viewX*3 + px, e.cmap.viewY*3 + py, frame, im})
}

func (e *Engine) hideEffect(px, py int) {
	x, y := e.cmap.viewX*3+px, e.cmap.viewY*3+py
	e.snap.effects = slices.DeleteFunc(e.snap.effects, func(f effect) bool { return f.x == x && f.y == y })
}

// ClickSquare returns a left click for the combat page that lands on map
// square (x, y), wherever the page has scrolled: the 3D view sees the
// whole map.
func (e *Engine) ClickSquare(x, y int) uint16 {
	e.snap.click.Store(int32(y*50 + x + 1))
	return MouseKey(MouseLeft, pageCol, 1)
}

// unitDrawn / unitHidden track what the page shows of combatant i.
func (e *Engine) unitDrawn(i int, pose byte) {
	if e.snap.poses == nil {
		e.snap.poses, e.snap.hidden = map[int]byte{}, map[int]bool{}
	}
	e.snap.poses[i] = pose
	delete(e.snap.hidden, i)
}

func (e *Engine) unitHidden(i int) {
	e.unitDrawn(i, 0)
	e.snap.hidden[i] = true
}
