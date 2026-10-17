package blobber

import "github.com/hajimehoshi/ebiten/v2"

// View is the renderer for a host that owns the game loop and the rules
// (the remastered Pool of Radiance): it draws a Scene from a Camera the host
// moves, into the host's image, and animates each step and turn. Nothing
// here reads the keyboard but the options page (UpdateOptions).
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
// animated like the player's own (moves that come while one runs follow it
// without stopping), anything further is a jump.
func (v *View) Follow(c Camera) {
	g := v.g
	last := Camera{g.x, g.y, g.dir}
	if n := len(g.next); n > 0 {
		last = g.next[n-1]
	}
	if c == last {
		return
	}
	g.asked = false // the move asked for came
	if g.moving && g.smooth && len(g.next) < 8 && oneMove(last, c) {
		g.next = append(g.next, c)
		return
	}
	g.next = g.next[:0]
	g.followStep(c)
}

// oneMove: c is one step or one turn from l.
func oneMove(l, c Camera) bool {
	dx, dy := c.X-l.X, c.Y-l.Y
	return dx*dx+dy*dy == 1 && c.Dir == l.Dir || dx == 0 && dy == 0
}

// followStep animates the move to c, or jumps there when it is not one.
func (g *Game) followStep(c Camera) {
	if !oneMove(Camera{g.x, g.y, g.dir}, c) {
		SetMap(level, c.X, c.Y)
		Face(c.Dir)
		return
	}
	dx, dy := c.X-g.x, c.Y-g.y
	sway := 0.0 // a sidestep rolls the view, as the player's own does
	if r := dirs[(g.dir+1)%4]; [2]int{dx, dy} == r {
		sway = 1
	} else if [2]int{dx, dy} == [2]int{-r[0], -r[1]} {
		sway = -1
	}
	g.startMove(c.X, c.Y, c.Dir, sway)
}

// Flow tells the view whether the host's move key is held and reports
// whether to send the next move now: with fluid movement, once per move,
// halfway through it, so the move arrives before this one ends and the
// view flows on without slowing (walking and turning alike).
// ready says the host can take the move now.
func (v *View) Flow(held, ready bool) bool {
	g := v.g
	g.hold = held
	if !held {
		g.asked = false
		return false
	}
	if !ready || !Settings.Fluid || !g.smooth || g.asked || len(g.next) > 0 || !g.moving || g.t < 0.5 {
		return false
	}
	g.asked = true
	return true
}

// Fluid reports whether held keys are paced by Flow.
func (v *View) Fluid() bool { return Settings.Fluid && v.g.smooth }

// OpenOptions shows Blobber's options page (smooth movement, speed, the
// look); while OptionsOpen, UpdateOptions reads the keyboard for it.
func (v *View) OpenOptions() { v.g.menu, v.g.options, v.g.sel = true, true, 0 }

// OptionsOpen reports whether the options page is up.
func (v *View) OptionsOpen() bool { return v.g.menu }

// UpdateOptions runs the options page for one frame: arrows and Enter
// change settings, Esc closes it.
func (v *View) UpdateOptions() {
	if KeyJustPressed(ebiten.KeyEscape) {
		v.g.menu = false
		return
	}
	v.g.updateMenu()
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
	if v.g.menu {
		v.g.drawMenu(dst)
	}
}
