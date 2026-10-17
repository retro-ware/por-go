package blobber

import "github.com/hajimehoshi/ebiten/v2"

// View is the renderer for a host that owns the game loop and the rules
// (the remastered Pool of Radiance): it draws a Scene from a Camera the host
// moves, into the host's image, and animates each step and turn. Nothing
// here reads the keyboard.
//
// ponytail: one View per process; the scene and camera live in package
// state shared with Run's Game. Make them per-View when two views must
// draw at once.
type View struct{ g *Game }

// Camera is a cell and a facing (0 N, 1 E, 2 S, 3 W).
type Camera struct{ X, Y, Dir int }

// NewView makes the textures and shaders. Call it once ebiten runs.
func NewView() (*View, error) {
	g, err := newGame(0)
	if err != nil {
		return nil, err
	}
	g.follow = true
	return &View{g}, nil
}

// Show makes s the scene and puts the camera at c at once.
func (v *View) Show(s *Scene, c Camera) {
	s.Show()
	SetMap(s.Rows, c.X, c.Y)
	Face(c.Dir)
}

// Follow moves the camera to c: a step to a neighbouring cell or a turn is
// animated like the player's own, anything further is a jump.
func (v *View) Follow(c Camera) {
	g := v.g
	if c.X == g.x && c.Y == g.y && c.Dir == g.dir {
		return
	}
	dx, dy := c.X-g.x, c.Y-g.y
	if dx*dx+dy*dy > 1 || dx*dx+dy*dy == 1 && c.Dir != g.dir {
		SetMap(level, c.X, c.Y)
		Face(c.Dir)
		return
	}
	sway := 0.0 // a sidestep rolls the view, as the player's own does
	if r := dirs[(g.dir+1)%4]; [2]int{dx, dy} == r {
		sway = 1
	} else if [2]int{dx, dy} == [2]int{-r[0], -r[1]} {
		sway = -1
	}
	g.startMove(c.X, c.Y, c.Dir, sway)
}

// Update advances the animation by one tick (1/60 s).
func (v *View) Update() {
	v.g.time += 1.0 / 60
	updateEnts(v.g.time)
	v.g.tween()
}

// Draw draws the 3D view filling dst.
func (v *View) Draw(dst *ebiten.Image) {
	b := dst.Bounds()
	v.g.resize(b.Dx(), b.Dy())
	v.g.draw3D(dst, v.g.camera())
}
