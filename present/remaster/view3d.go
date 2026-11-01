// view3d.go holds the 3D view's overlays: the map pane, the scenes and pictures, the door-open animation and boxes drawn over the view.

package remaster

import (
	"fmt"
	"gpu"
	"gpu/vector"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"por/engine"
	"por/phlan-poc/blobber"
	"por/phlan-poc/blobber/maps"
	"strconv"
	"strings"
)

// mapRect is where the map goes: a square in the top right corner, a card
// gap from the edges, mapPx on a side.
func mapRect(dst *gpu.Image, _ int) image.Rectangle {
	b := dst.Bounds()
	gap, side := cardGap*cardBase(dst), mapPx(dst)
	return image.Rect(b.Dx()-gap-side, gap, b.Dx()-gap, gap+side)
}

// mapPx is the map's side: one map pixel to a card pixel at size 1 (Esc,
// Graphics, Minimap size), at most 60 percent of the height and a third of
// the width; the cards make way for it.
func mapPx(dst *gpu.Image) int {
	b := dst.Bounds()
	return int(mapSide * fitScale(float64(cardBase(dst)), blobber.Settings.Map, func(k float64) bool {
		return k*mapSide <= float64(b.Dy())*0.6 && k*mapSide <= float64(b.Dx())/3
	}))
}

// fitScale is unit (K) times size s (one of blobber.UISteps, whole) when
// ok takes it, else the largest smaller whole step of unit ok takes, else
// (a box too big for the window even at K) the largest whole scale under
// unit ok takes, its pixels square, else halves of 1 down to an eighth.
func fitScale(unit, s float64, ok func(k float64) bool) float64 {
	s = max(1, math.Round(s)) // a size saved before as a fraction: 1
	for n := s; n >= 1; n-- {
		if ok(unit * n) {
			return unit * n
		}
	}
	for k := math.Ceil(unit) - 1; k >= 1; k-- {
		if ok(k) {
			return k
		}
	}
	k := 1.0
	for k > 0.125 && !ok(k) { // a window too small for it even at 1: halves, as far as it fits
		k /= 2
	}
	return k
}

// mapSide is View.Map's side in pixels.
const mapSide = 16 * engine.MapCell

// drawMap shows the map as the full map draws it (View.Map) in mapRect.
func (p *Presenter) drawMap(dst *gpu.Image, v engine.View, font func(byte, bool) [8]byte) {
	p.mapAt = image.Rectangle{}
	if v.Map == nil || blobber.Settings.MapOff {
		return
	}
	if p.mapImg == nil {
		p.mapImg = gpu.NewImage(mapSide, mapSide)
	}
	if v.Map != p.mapSrc { // a new View: upload it once
		p.mapImg.WritePixels(mapImage(v.Map, engine.MapCell, v.Ink, font).Pix)
		p.mapSrc = v.Map
	}
	r := mapRect(dst, len(v.Party))
	p.mapAt = r
	a := float32(blobber.Settings.MapAlpha) // the opacity slider (Esc, Options)
	vector.StrokeRect(dst, float32(r.Min.X)+.5, float32(r.Min.Y)+.5, float32(r.Dx())-1, float32(r.Dy())-1, 1, color.NRGBA{0x80, 0x80, 0x80, uint8(255 * a)})
	vector.DrawFilledRect(dst, float32(r.Min.X+1), float32(r.Min.Y+1), float32(r.Dx()-2), float32(r.Dy()-2), color.NRGBA{A: uint8(255 * a)})
	k := float64(r.Dx()-2) / mapSide // any scale: the square keeps the aspect
	op := &gpu.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(a)
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(r.Min.X)+(float64(r.Dx())-mapSide*k)/2, float64(r.Min.Y)+(float64(r.Dy())-mapSide*k)/2)
	dst.DrawImage(p.mapImg, op)
}

// scene is the remastered map of the area v shows, or nil for an area
// without one (logged once: there is no other view of it).
func (p *Presenter) scene(v engine.View) *blobber.Scene {
	name, ok := areaMaps[v.Area]
	if !ok {
		name = fmt.Sprintf("area%d", v.Area)
	}
	return p.sceneNamed(name)
}

// openAt (POR_OPEN=map:NAME:X:Y:D, D 0..3 for N E S W) shows the remastered
// map NAME from square X, Y alone, whatever the engine shows: the
// comparison sheets (phlan-poc/compare/compare.py) shoot it with POR_SHOT.
var openAt = os.Getenv("POR_OPEN")

// drawOpen draws the view openAt names.
func (p *Presenter) drawOpen(dst *gpu.Image) {
	f := strings.Split(openAt, ":")
	if len(f) != 5 || f[0] != "map" {
		log.Fatalf("POR_OPEN=%q: want map:NAME:X:Y:D", openAt)
	}
	x, _ := strconv.Atoi(f[2])
	y, _ := strconv.Atoi(f[3])
	d, _ := strconv.Atoi(f[4])
	sc := p.sceneNamed(f[1])
	if sc == nil {
		log.Fatalf("POR_OPEN=%q: no map %s", openAt, f[1])
	}
	if p.shown != f[1] {
		p.view.Show(sc, blobber.Camera{X: x, Y: y, Dir: d})
		p.shown = f[1]
		blobber.SetFlats(sc.Floor, sc.Ceiling)
	}
	p.animate()
	p.view.Draw(dst)
}

// sceneNamed is the remastered map NAME, or nil (logged once).
func (p *Presenter) sceneNamed(name string) *blobber.Scene {
	if p.view == nil {
		var err error
		if p.view, err = blobber.NewView(); err != nil {
			log.Fatalln("remaster:", err)
		}
		blobber.Window = func(d *gpu.Image, t string, l []string, k []byte, sel int, b bool) { // its menu and options in the game's box
			p.keyWindow(d, t, l, k, nil, sel, b)
		}
	}
	if sc, ok := p.scenes[name]; ok {
		return sc
	}
	var sc *blobber.Scene
	if f, err := maps.Levels.Open(name + ".map"); err != nil {
		log.Println("remaster:", err)
	} else {
		defer f.Close()
		if sc, _, err = blobber.ParseScene(f); err != nil {
			log.Printf("remaster: %s.map: %v", name, err)
		}
	}
	if p.scenes == nil {
		p.scenes = map[string]*blobber.Scene{}
	}
	p.scenes[name] = sc // nil too: a missing or broken map is logged once
	return sc
}

// Scripted stops animate's clock: a capture script ticks the renderer itself (Tick), so
// every frame of a scripted run is the same run to run.
var Scripted bool

// drawBox copies box r of the classic frame onto dst at scale k, centred
// on cx, cy: its inside, in a plain window of tiles' cells.
func drawBox(dst, inside *gpu.Image, tiles [3]*gpu.Image, r engine.CellRect, k float64, cx, cy int) {
	px := r.Pixels()
	var g gpu.GeoM
	g.Scale(k, k)
	g.Translate(float64(cx)-k*float64(px.Dx())/2, float64(cy)-k*float64(px.Dy())/2)
	in := window(dst, tiles, g, r.X2-r.X1+1, r.Y2-r.Y1+1, nil)
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(in.Min.X), float64(in.Min.Y))
	dst.DrawImage(inside, op)
}
