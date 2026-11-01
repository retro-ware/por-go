// control.go holds the combat map's controls: keys, the square cursor, targets, the camera's zoom, pan and chase, and picking units under the pointer.

package battle

import (
	"gpu"
	"image"
	"math"
	"por/engine"
	"por/present/input"
	"slices"
)

// Zoom moves the camera in (wheel up) or out by one wheel step: 15 percent
// of its distance at speed 1 (the Zoom speed option).
func (p *Presenter) Zoom(in bool, speed float64) {
	p.initCamera()
	f := 1 + 0.15*speed
	if in {
		p.distTo = max(minDist, p.distTo/f)
	} else {
		p.distTo = min(maxDist, p.distTo*f)
	}
}

// Key is the XCOM 2 scheme: W A S D and the arrows pan (held: see pan), Q and E turn,
// Home brings the camera back, 1..9 and 0 pick the bar's
// buttons, the bar's letters (hotkeys) theirs, Tab and Shift-Tab cycle the
// targets while aiming (else they and PgUp/PgDn move the bar's focus,
// which Enter presses), Enter takes the aimed one, Esc gives up (No) and
// Backspace ends the turn. It reports whether k was taken.
//
// a is what the binding table (present/remaster/bindings.go) makes of the
// key, k the key itself: the digits and the bar's letters are raw input.
func (p *Presenter) Key(a input.Act, k uint16) bool {
	p.initCamera()
	if k < 128 && k >= 'A' && k <= 'Z' {
		k += 'a' - 'A'
	}
	b := p.b
	aiming := b != nil && b.Turn != nil && b.Turn.Aim
	cycle := a == input.Tab || a == input.PgUp || a == input.PgDn
	back := a == input.PgUp || a == input.Tab && gpu.IsKeyPressed(gpu.KeyShift)
	switch {
	case a == input.TurnLeft:
		p.yawTo -= math.Pi / 4
	case a == input.TurnRight:
		p.yawTo += math.Pi / 4
	case a >= input.CursorUp && a <= input.CursorRight: // the square cursor (the pad's d-pad and left stick send these too)
		p.moveCursor(map[input.Act][2]float64{input.CursorUp: {1, 0}, input.CursorDown: {-1, 0}, input.CursorRight: {0, 1}, input.CursorLeft: {0, -1}}[a])
	case a == input.Up || a == input.Down || a == input.Left || a == input.Right: // the arrows pan
	case a == input.Home:
		p.offset = vec{}
	case k >= '0' && k <= '9':
		if i := (int(k) - '0' + 9) % 10; i < len(p.bar.rects) { // 1 is the first button, 0 the tenth
			p.press(p.bar.items[i])
		}
	case p.bar.hotkey(a, k) != nil:
		p.press(*p.bar.hotkey(a, k))
	case cycle && aiming && len(b.Targets) > 0: // Tab, Shift-Tab, the pad's shoulders: the next, previous target
		p.cycleTarget(back)
	case a == input.Space:
		return p.Key(input.Enter, k) // Space picks, as Enter
	case a == input.Enter && aiming:
		p.act(engine.Action{Kind: engine.ActAim, X: b.Cursor.X, Y: b.Cursor.Y})
	case cycle && len(p.bar.rects) > 0: // Tab, Shift-Tab, the pad's shoulders (PgUp, PgDn): the focus
		d := 1
		if back {
			d = -1
		}
		p.bar.moveFocus(d)
	case a == input.Enter && p.bar.focus > 0 && p.bar.focus <= len(p.bar.rects):
		p.press(p.bar.items[p.bar.focus-1])
	case a == input.Enter && p.free != nil: // the square cursor: as a click there
		return p.clickSquare(*p.free)
	case a == input.Esc && aiming:
		p.act(engine.Action{Kind: engine.ActClose})
	default:
		return false
	}
	return true
}

// cycleTarget moves the aim to the next of the turn's targets (back: the
// previous), the camera along.
func (p *Presenter) cycleTarget(back bool) {
	b := p.b
	i := 0
	for j, id := range b.Targets {
		for _, u := range b.Units {
			if u.ID == id && u.X == b.Cursor.X && u.Y == b.Cursor.Y {
				i = j + 1
				if back {
					i = j - 1 + len(b.Targets)
				}
			}
		}
	}
	for _, u := range b.Units {
		if u.ID == b.Targets[i%len(b.Targets)] {
			c := p.cursor
			p.tabFrom = &c
			p.act(engine.Action{Kind: engine.ActPoint, X: u.X, Y: u.Y})
		}
	}
}

// moveCursor moves the square cursor a square forward (d[0]) or right
// (d[1]) as the camera looks: aiming, the aim (ActPoint); else the free
// cursor, from the active unit's square, kept on the map.
func (p *Presenter) moveCursor(d [2]float64) {
	b := p.b
	if b == nil || b.Turn == nil {
		return
	}
	sin, cos := math.Sin(p.yaw), math.Cos(p.yaw)
	step := image.Pt(int(math.Round(-sin*d[0]+cos*d[1])), int(math.Round(-cos*d[0]-sin*d[1])))
	at := b.Cursor
	if !b.Turn.Aim {
		if p.free == nil {
			p.free = &image.Point{}
			for _, u := range b.Units {
				if u.Active {
					*p.free = image.Pt(u.X, u.Y)
				}
			}
		}
		at = *p.free
	}
	at = at.Add(step)
	at = image.Pt(min(max(at.X, 0), len(p.ground[0])-1), min(max(at.Y, 0), len(p.ground)-1))
	if b.Turn.Aim {
		p.act(engine.Action{Kind: engine.ActPoint, X: at.X, Y: at.Y})
		return
	}
	*p.free = at
}

// targetAt: a foe a click attacks stands on sq.
func targetAt(b *engine.Battle, sq image.Point) bool {
	for _, u := range b.Units {
		if slices.Contains(b.Targets, u.ID) && slices.Contains(u.Squares, sq) {
			return true
		}
	}
	return false
}

// pan moves the camera's centre by the held arrows or the right stick, relative to
// its heading, and reports whether any is held. A pan starts from where
// the camera is (not from the focus it was easing toward), so the first
// frames never swing the other way.
func (p *Presenter) pan(dt float64, focus vec) bool {
	f, r := -p.Stick[1], p.Stick[0] // the pad's right stick
	for _, h := range [4]struct {
		k    gpu.Key
		f, r float64
	}{{gpu.KeyUp, 1, 0}, {gpu.KeyDown, -1, 0}, {gpu.KeyRight, 0, 1}, {gpu.KeyLeft, 0, -1}} {
		if gpu.IsKeyPressed(h.k) && len(p.bar.rects) > 0 { // the arrows step in Move (no bar)
			f, r = f+h.f, r+h.r
		}
	}
	if f == 0 && r == 0 {
		return false
	}
	if p.offset == (vec{}) {
		p.offset = p.at.sub(focus)
	}
	v := dt * p.dist * 1.2 // squares a second, faster zoomed out
	sin, cos := math.Sin(p.yaw), math.Cos(p.yaw)
	p.offset = p.offset.add(vec{-sin*f + cos*r, 0, -cos*f - sin*r}.mul(v))
	return true
}

// chase is how much of the easing toward focus this frame takes (0..1):
// all of it when the focus is out of sight, none while
// it sits inside the view's comfort box (the eye does not follow what is
// in plain sight: less motion the player did not ask for), and rising
// smoothly as it leaves the box for the edge.
func (p *Presenter) chase(focus vec) float64 {
	const side, top, bottom = 0.6, 0.65, -0.3 // the box in -1..1 (the bar covers the bottom)
	nx, ny, ok := p.cam.ndc(focus)
	if !ok {
		return 1
	}
	out := max(0, math.Abs(nx)-side, ny-top, bottom-ny)
	return min(1, out/0.3)
}

func (p *Presenter) initCamera() {
	if p.distTo == 0 {
		p.dist, p.distTo = 12, 12
	}
}

// Pick is the map square under screen pixel (x, y), as last drawn.
func (p *Presenter) Pick(x, y int) (int, int, bool) {
	r := p.screen
	if r.Empty() || !image.Pt(x, y).In(r) {
		return 0, 0, false
	}
	return p.cam.pick(2*float64(x-r.Min.X)/float64(r.Dx())-1, 1-2*float64(y-r.Min.Y)/float64(r.Dy()))
}

// unitCentre is the middle of the squares u stands on (its footprint).
func unitCentre(u engine.Unit) [2]float64 {
	if len(u.Squares) == 0 {
		return [2]float64{float64(u.X) + .5, float64(u.Y) + .5}
	}
	var c [2]float64
	for _, s := range u.Squares {
		c[0] += float64(s.X) + .5
		c[1] += float64(s.Y) + .5
	}
	return [2]float64{c[0] / float64(len(u.Squares)), c[1] / float64(len(u.Squares))}
}

// unitAt is where u is shown (sliding to its square) and the squares its icon spans.
func (p *Presenter) unitAt(u engine.Unit, dt float64) (x, z, sq float32) {
	c := unitCentre(u)
	if p.units == nil {
		p.units = map[int]*[2]float64{}
	}
	at, ok := p.units[u.ID]
	if !ok {
		at = &[2]float64{c[0], c[1]}
		p.units[u.ID] = at
	}
	if math.Abs(at[0]-c[0])+math.Abs(at[1]-c[1]) > 3 { // a jump (placed, scrolled in): no slide
		*at = c
	}
	k := 1 - math.Exp(-dt*7) // per call: Draw asks twice a frame (shadow, figurine), so half a step each
	at[0] += (c[0] - at[0]) * k
	at[1] += (c[1] - at[1]) * k
	ib := iconSize(u)
	return float32(at[0]), float32(at[1]), float32(max(ib.X, ib.Y)) / tilePx
}

// iconSize is the size the figure's footprint follows: the built icon's
// (IconSize), else the picture's own.
func iconSize(u engine.Unit) image.Point {
	if u.IconSize != (image.Point{}) {
		return u.IconSize
	}
	return u.Icon.Bounds().Size()
}
