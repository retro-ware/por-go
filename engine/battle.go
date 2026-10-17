package engine

import (
	"image"
	"image/color"

	"por/dax"
)

// Battle is the combat map as the remaster's 3D view needs it.
type Battle struct {
	Ground     [25][50]Square
	Tiles      []image.Image // the tile set by tile number, in combat's colours
	Units      []Unit
	Cursor     image.Point // the square the cursor was last drawn on
	CursorOn   bool        // the cursor icon is up (aiming, looking)
	CursorSize int
	View       image.Point // top left square the original shows
}

// Square is one map square.
type Square struct {
	Tile   int
	Wall   bool // move cost 0xFF
	Height int  // blocking height
}

// Unit is one combatant on the map.
type Unit struct {
	X, Y, Size int
	Party      bool
	Facing     int         // 0..7
	Icon       image.Image // the combat icon, facing as the original draws it
	Active     bool        // whose turn it is
	Name       string
}

// battle builds the snapshot's Battle (nil outside combat).
func (e *Engine) battle() *Battle {
	if e.beginFlag != 5 || e.sqrpaci[0] == nil {
		return nil
	}
	s := &e.snap
	b := &Battle{
		Cursor: s.cursor, CursorOn: e.cmap.cursor != 0, CursorSize: int(e.cmap.cursorSize),
		View: image.Pt(e.cmap.viewX, e.cmap.viewY),
	}
	for y := range b.Ground {
		for x := range b.Ground[y] {
			g := e.groundInfo(e.cmap.ground[y][x])
			b.Ground[y][x] = Square{Tile: int(g[3]), Wall: g[0] == 0xFF, Height: int(g[2])}
		}
	}
	b.Tiles = make([]image.Image, 0x100)
	for t, im := range e.sqrpaci {
		for f := 0; im != nil && f < min(im.Count, 0x80); f++ {
			b.Tiles[t*0x80+f] = e.picRGBA(im, f, false)
		}
	}
	var active *Character
	if e.partySel < len(e.party) {
		active = e.party[e.partySel]
	}
	for i := 1; i <= e.ncomb; i++ {
		p, c := e.cpos[i], e.combatant(byte(i))
		if p.size == 0 || c == nil || c.Combat == nil {
			continue
		}
		u := Unit{X: int(p.x), Y: int(p.y), Size: int(p.size), Party: isParty(c),
			Facing: int(c.Combat.Facing), Active: c == active, Name: c.Name()}
		if im := e.icons[c.Order][0]; im != nil {
			u.Icon = e.picRGBA(im, 0, u.Facing > 3)
		}
		b.Units = append(b.Units, u)
	}
	return b
}

type picKey struct {
	im    *dax.Image
	frame int
	flip  bool
}

// picRGBA converts a frame in the screen's current palette (combat swaps
// colours 0 and 8), Clear pixels transparent. Cached per frame and palette.
func (e *Engine) picRGBA(im *dax.Image, f int, flip bool) image.Image {
	s := &e.snap
	if s.pics == nil || s.picsPal != e.S.Pal {
		s.pics, s.picsPal = map[picKey]image.Image{}, e.S.Pal
	}
	k := picKey{im, f, flip}
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
		if p == dax.Clear {
			out.SetRGBA(x, i/w, color.RGBA{})
			continue
		}
		out.SetRGBA(x, i/w, e.S.RGBA(p))
	}
	s.pics[k] = out
	return out
}
