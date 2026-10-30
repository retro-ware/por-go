package blobber

import (
	"gpu"
	"os"
)

// View is the renderer for a host that owns the game loop and the rules
// (the remastered Pool of Radiance): it draws a Scene from a Camera the host
// moves, into the host's image, and animates each step and turn. Nothing
// here reads the keyboard but the Esc menu (UpdateOptions).
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

// OpenOptions opens the Esc (pause) menu at its root; while OptionsOpen,
// UpdateOptions reads the keyboard and the gamepads for it.
func (v *View) OpenOptions() {
	v.g.menu, v.g.page, v.g.confirm, v.g.sel, v.g.lastPoint = true, "", nil, 0, -1
	pollPads(0) // a pad button held from opening it counts as pressed only once it goes down again
}

// OptionsOpen reports whether the Esc menu is up.
func (v *View) OptionsOpen() bool { return v.g != nil && v.g.menu }

// UpdateOptions runs the Esc menu for one frame: arrows and Enter pick and
// change, Esc goes back to the root or closes it. Quit exits the program.
func (v *View) UpdateOptions() {
	pollPads(0)
	if KeyJustPressed(gpu.KeyEscape) {
		if !v.g.back() {
			v.g.menu = false
		}
		return
	}
	if v.g.updateMenu() {
		os.Exit(0)
	}
}

// PointOptions is the mouse on the Esc menu: line (-1: none) is under the
// pointer and highlights; click picks it (Enter), back goes back (Esc),
// wheel steps a setting (Left / Right).
func (v *View) PointOptions(line int, click, back bool, wheel int) {
	g := v.g
	if back {
		if !g.back() {
			g.menu = false
		}
		return
	}
	items := g.menuOptions()
	if line < 0 || line >= len(items) {
		return
	}
	if line != g.lastPoint { // only a move highlights, so the keys keep theirs
		g.sel, g.lastPoint = line, line
	}
	d := 0
	switch {
	case click:
	case wheel != 0 && g.page != "" && g.confirm == nil:
		d = 1 // wheel up: more
		if wheel < 0 {
			d = -1
		}
	default:
		return
	}
	g.sel, g.lastPoint = line, -1 // a new page highlights under the pointer once it moves
	items[line].change(g, d)
	if g.page != "" {
		Settings.Smooth, Settings.Dither, Settings.Fullscreen, Settings.Torch = g.smooth, g.dither, gpu.IsFullscreen(), g.torch
		if OnSettings != nil {
			OnSettings(Settings)
		}
	}
	if g.quit {
		os.Exit(0)
	}
}

// Update advances the animation by one tick (1/60 s).
func (v *View) Update() {
	v.g.time += 1.0 / 60
	updateEnts(v.g.time)
	v.g.tween()
}

// Draw draws the 3D view filling dst.
func (v *View) Draw(dst *gpu.Image) {
	b := dst.Bounds()
	v.g.resize(b.Dx(), b.Dy())
	n := gpu.Stats.Draws
	v.g.draw3D(dst, v.g.camera())
	gpu.Stats.Draws3D += gpu.Stats.Draws - n
}

// DrawMenu draws the Esc menu, when it is open, over what the host drew on the view.
func (v *View) DrawMenu(dst *gpu.Image) {
	if v.g.menu {
		v.g.drawMenu(dst)
	}
}
