// Package battle draws the combat map as a 3D tabletop: wall blocks in the
// original's tiles, figurines extruded from the combat icons on pedestals
// (blue for the party, red for its enemies), under a camera that turns in
// eighths like XCOM's. It only reads engine.Battle.
package battle

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"time"

	"gpu"

	"por/engine"
)

// Presenter keeps the GPU side of one combat map.
type Presenter struct {
	white  *gpu.Image
	atlas  *gpu.Image
	tiles  [atlasCols * atlasCols]image.Image // the tiles the atlas holds
	ground [25][50]engine.Square              // the squares the map mesh shows
	mesh   *gpu.Mesh
	box    *gpu.Mesh
	figs   map[image.Image]*figure // ponytail: never evicted; a few dozen icons a game

	yaw, yawTo float64    // camera heading: 0 looks north, as the original's screen
	at         [2]float64 // the square the camera looks at (eased)
	last       time.Time
}

type figure struct {
	mesh *gpu.Mesh
	tex  *gpu.Image
}

// Camera framing.
const (
	pitch    = 0.75 // radians above the ground
	distance = 12.0 // squares from what it looks at
	pedestal = 0.08 // a pedestal's height
)

var (
	partyColour = [4]float32{0.25, 0.45, 1, 1}
	enemyColour = [4]float32{0.9, 0.2, 0.15, 1}
	markColour  = [4]float32{1, 0.8, 0.2, 1}
	sun         = gpu.MeshOptions{Light: [3]float32{0.4, 1, 0.7}, Ambient: 0.55}
)

// Key turns the camera with [ and ]; it reports whether k was taken.
func (p *Presenter) Key(k uint16) bool {
	switch k {
	case '[':
		p.yawTo -= math.Pi / 4
	case ']':
		p.yawTo += math.Pi / 4
	default:
		return false
	}
	return true
}

// Draw shows b on dst.
func (p *Presenter) Draw(dst *gpu.Image, b *engine.Battle) {
	p.sync(b)
	focus := [2]float64{float64(b.View.X) + 6, float64(b.View.Y) + 3.5}
	for _, u := range b.Units {
		if u.Active {
			focus = unitCentre(u)
		}
	}
	p.ease(focus)

	r := dst.Bounds()
	at := [3]float32{float32(p.at[0]), 0, float32(p.at[1])}
	h, s, c := distance*math.Cos(pitch), math.Sin(p.yaw), math.Cos(p.yaw)
	eye := [3]float32{at[0] + float32(h*s), float32(distance * math.Sin(pitch)), at[2] + float32(h*c)}
	vp := mul(perspective(0.8, float32(r.Dx())/float32(r.Dy()), 0.1, 200), lookAt(eye, at))

	dst.Fill(color.RGBA{0x10, 0x10, 0x18, 0xFF})
	dst.ClearDepth()
	dst.DrawMesh(p.mesh, p.atlas, vp, &sun)
	plate := func(x, z, side float32, tint [4]float32) {
		op := sun
		op.Model, op.Tint = place(x, 0, z, 0, side, 0.02, side), tint
		dst.DrawMesh(p.box, p.white, vp, &op)
	}
	if b.CursorOn {
		plate(float32(b.Cursor.X)+.5, float32(b.Cursor.Y)+.5, 1, markColour)
	}
	for _, u := range b.Units {
		if u.Icon == nil {
			continue
		}
		ib := u.Icon.Bounds()
		sq := float32(max(ib.Dx(), ib.Dy())) / tilePx // squares the icon spans
		cen := unitCentre(u)
		x, z := float32(cen[0]), float32(cen[1])
		if u.Active {
			plate(x, z, sq*.95, markColour)
		}
		op := sun
		op.Model, op.Tint = place(x, 0, z, 0, sq*.7, pedestal, sq*.7), enemyColour
		if u.Party {
			op.Tint = partyColour
		}
		dst.DrawMesh(p.box, p.white, vp, &op)
		f := p.figure(u.Icon)
		size := sq * .9
		half := size * float32(ib.Dy()) / float32(max(ib.Dx(), ib.Dy())) / 2
		op = sun
		// ponytail: figurines turn to face the camera (a slab seen edge on reads as nothing)
		op.Model = place(x, pedestal+half, z, float32(p.yaw), size, size, size)
		dst.DrawMesh(f.mesh, f.tex, vp, &op)
	}
}

// unitCentre is the middle of the squares u's icon covers.
func unitCentre(u engine.Unit) [2]float64 {
	ib := u.Icon.Bounds()
	return [2]float64{float64(u.X) + float64(ib.Dx())/tilePx/2, float64(u.Y) + float64(ib.Dy())/tilePx/2}
}

// ease moves the camera toward its heading and focus, framerate independent.
func (p *Presenter) ease(focus [2]float64) {
	now := time.Now()
	k := 1.0
	if !p.last.IsZero() {
		k = 1 - math.Exp(-now.Sub(p.last).Seconds()*10)
	}
	p.last = now
	p.yaw += (p.yawTo - p.yaw) * k
	p.at[0] += (focus[0] - p.at[0]) * k
	p.at[1] += (focus[1] - p.at[1]) * k
}

// sync brings the atlas and the map mesh up to b.
func (p *Presenter) sync(b *engine.Battle) {
	if p.white == nil {
		p.white = gpu.NewImage(1, 1)
		p.white.Fill(color.White)
		p.box = gpu.NewMesh(box())
		p.atlas = gpu.NewImage(atlasCols*tilePx, atlasCols*tilePx)
	}
	changed := false
	for i := range p.tiles {
		var t image.Image
		if i < len(b.Tiles) {
			t = b.Tiles[i]
		}
		if t != p.tiles[i] {
			p.tiles[i], changed = t, true
		}
	}
	if changed {
		a := image.NewRGBA(p.atlas.Bounds())
		for i, t := range p.tiles {
			if t != nil {
				o := image.Pt(i%atlasCols*tilePx, i/atlasCols*tilePx)
				draw.Draw(a, image.Rectangle{o, o.Add(t.Bounds().Size())}, t, t.Bounds().Min, draw.Src)
			}
		}
		p.atlas.WritePixels(a.Pix)
	}
	if p.mesh == nil || b.Ground != p.ground {
		p.ground = b.Ground
		if p.mesh != nil {
			p.mesh.Dispose()
		}
		p.mesh = gpu.NewMesh(mapMesh(&p.ground))
	}
}

// figure is the extruded figurine for an icon (cached).
func (p *Presenter) figure(icon image.Image) *figure {
	if f, ok := p.figs[icon]; ok {
		return f
	}
	if p.figs == nil {
		p.figs = map[image.Image]*figure{}
	}
	f := &figure{gpu.NewMesh(extrude(icon)), gpu.NewImageFromImage(icon)}
	p.figs[icon] = f
	return f
}
