// Package battle draws the combat map as a tabletop: the map is a board
// on a wooden table, dungeon walls are stone blocks, outdoor things stand
// or lie on it, and the combatants are figurines extruded from their combat
// icons on round painted bases (blue for the party, red for its enemies).
// The camera turns in eighths like XCOM's. It only reads engine.Battle:
// what it shows is what the original shows, in 3D.
package battle

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"gpu"

	"por/engine"
)

// Presenter keeps the GPU side of one combat map.
type Presenter struct {
	white, atlas, shadow, wood *gpu.Image
	tiles                      [256]image.Image // the tiles the atlas holds
	empty                      [256]bool        // tiles with nothing on the ground
	floor                      color.RGBA
	ground                     [25][50]engine.Square // the squares the meshes show
	dungeon                    bool
	board, shadows, table      *gpu.Mesh
	box, base, flatQuad        *gpu.Mesh
	figs                       map[image.Image]*figure // ponytail: never evicted; a few dozen pictures a game
	bar                        bar

	b             *engine.Battle  // the last frame's
	out           []engine.Action // queued for Actions
	Picking       bool            // the spell list is up (Cast): the bar is off
	pointed       image.Point     // the square the aim was last pointed at
	cursor        image.Point     // the aim cursor's square in the last frame
	tabFrom       *image.Point    // Tab was pressed with the aim here: the camera jumps to where it goes
	tabTo         *vec            // the target Tab went to: watched while aiming
	focus, offset vec             // what the camera watches; the W A S D pan from it
	yaw, yawTo    float64         // camera heading: 0 looks north, as the original's screen
	dist, distTo  float64         // camera distance in squares
	at            vec             // what the camera looks at (eased)
	cam           camera          // the last frame's camera, for picking
	screen        image.Rectangle
	units         map[int]*[2]float64 // each unit's shown position (eased between squares)
	sparks        []spark
	strike        engine.Strike // the last attack shown
	strikeT       float64       // seconds since it began
	burst         bool          // its sparks are out
	last          time.Time
	clock         float64 // seconds shown, for pulses
}

type figure struct {
	mesh *gpu.Mesh
	tex  *gpu.Image
	glow [3]float32 // its average colour, for an effect's light and sparks
}

// spark is a particle rising from a spell or a hit.
type spark struct {
	p, v      vec
	life, max float64
	col       [3]float32
}

// Framing and look.
const (
	pitch     = 0.82 // camera radians above the board
	minDist   = 5.0
	maxDist   = 24.0
	pedestal  = 0.09 // a base's height
	lunge     = 0.35 // seconds an attack's lunge lasts
	boardEdge = 0.25 // how far the board stands above the table
)

var (
	partyColour = [4]float32{0.22, 0.42, 0.95, 1}
	enemyColour = [4]float32{0.88, 0.2, 0.14, 1}
	cursorGlow  = [4]float32{1, 0.78, 0.2, 1}
	markGlow    = [4]float32{0.25, 0.8, 0.85, 1} // the original frames choices in light cyan
	hoverGlow   = [4]float32{0.45, 0.45, 0.45, 1}
	areaGlow    = [4]float32{1, 0.5, 0.1, 1}
	areaFill    = [4]float32{0.22, 0.09, 0.01, 1}
	targetGlow  = [4]float32{0.95, 0.12, 0.08, 1}
	blockedGlow = [4]float32{1, 0.08, 0.05, 1} // an aim that cannot be taken: area, frames and cursor in red
	blockedFill = [4]float32{0.26, 0.015, 0.01, 1}
	reachGlow   = [4]float32{0.015, 0.05, 0.13, 1}
	pathGlow    = [4]float32{0.1, 0.35, 0.8, 1}
	skipGlow    = [4]float32{0.55, 0.55, 0.75, 1} // an ally in a spell's area the cast passes over (Bless: an enemy next to it)
	freeGlow    = [4]float32{0.9, 0.1, 0.05, 1}
	sunDir      = [3]float32{0.45, 1, 0.6}
)

// Zoom moves the camera in (wheel up) or out.
func (p *Presenter) Zoom(in bool) {
	p.initCamera()
	if in {
		p.distTo = max(minDist, p.distTo/1.15)
	} else {
		p.distTo = min(maxDist, p.distTo*1.15)
	}
}

// Key is the XCOM 2 scheme: W A S D and the arrows pan (held: see pan), Q and E turn,
// Home brings the camera back, 1..9 and 0 pick the bar's
// buttons, the bar's letters (hotkeys) theirs, Tab and Shift-Tab cycle the
// targets while aiming, Enter takes the aimed one, Esc gives up and
// Backspace ends the turn. It reports whether k was taken.
func (p *Presenter) Key(k uint16) bool {
	p.initCamera()
	if k < 128 && k >= 'A' && k <= 'Z' {
		k += 'a' - 'A'
	}
	b := p.b
	aiming := b != nil && b.Turn != nil && b.Turn.Aim
	switch {
	case k == 'q':
		p.yawTo -= math.Pi / 4
	case k == 'e':
		p.yawTo += math.Pi / 4
	case k == 'w' || k == 'a' || k == 's' || k == 'd':
	case k == 0x4800 || k == 0x5000 || k == 0x4B00 || k == 0x4D00: // the arrows pan
	case k == 0x4700:
		p.offset = vec{}
	case k >= '0' && k <= '9':
		if i := (int(k) - '0' + 9) % 10; i < len(p.bar.rects) { // 1 is the first button, 0 the tenth
			p.press(p.bar.items[i])
		}
	case p.bar.hotkey(k) != nil:
		p.press(*p.bar.hotkey(k))
	case k == 9 && aiming && len(b.Targets) > 0:
		i := 0
		for j, id := range b.Targets {
			for _, u := range b.Units {
				if u.ID == id && u.X == b.Cursor.X && u.Y == b.Cursor.Y {
					i = j + 1
					if gpu.IsKeyPressed(gpu.KeyShift) {
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
	case k == 13 && aiming:
		p.act(engine.Action{Kind: engine.ActAim, X: b.Cursor.X, Y: b.Cursor.Y})
	case k == 27 && (aiming || b != nil && b.Ask != ""):
		p.act(engine.Action{Kind: engine.ActClose})
	default:
		return false
	}
	return true
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

// pan moves the camera's centre by the held W A S D or arrows, relative to its heading.
func (p *Presenter) pan(dt float64) {
	var f, r float64
	for _, h := range [8]struct {
		k    gpu.Key
		f, r float64
	}{{gpu.KeyW, 1, 0}, {gpu.KeyS, -1, 0}, {gpu.KeyD, 0, 1}, {gpu.KeyA, 0, -1},
		{gpu.KeyUp, 1, 0}, {gpu.KeyDown, -1, 0}, {gpu.KeyRight, 0, 1}, {gpu.KeyLeft, 0, -1}} {
		if gpu.IsKeyPressed(h.k) && (h.k == gpu.KeyW || h.k == gpu.KeyS || h.k == gpu.KeyD || h.k == gpu.KeyA || len(p.bar.rects) > 0) { // the arrows step in Move (no bar)
			f, r = f+h.f, r+h.r
		}
	}
	v := dt * p.dist * 1.2 // squares a second, faster zoomed out
	sin, cos := math.Sin(p.yaw), math.Cos(p.yaw)
	p.offset = p.offset.add(vec{-sin*f + cos*r, 0, -cos*f - sin*r}.mul(v))
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

// Draw shows b on dst.
func (p *Presenter) Draw(dst *gpu.Image, b *engine.Battle) {
	p.initCamera()
	p.b = b
	if b.Turn == nil || b.Turn.Aim || !slices.Contains(b.Turn.Can, engine.ActCast) {
		p.Picking = false
	}
	p.sync(b)
	dt := p.tick()
	if b.Strike.Seq != p.strike.Seq {
		p.strike, p.strikeT, p.burst = b.Strike, 0, false
	}
	p.strikeT += dt

	// where the eye goes: an effect, else the target Tab went to, else whose
	// turn it is (the aim cursor otherwise is the player's to follow: the camera pans by hand)
	if !b.CursorOn {
		p.tabFrom, p.tabTo = nil, nil
	}
	if f := p.tabFrom; f != nil && b.Cursor != *f { // Tab moved the aim: look there at once
		t := vec{float64(b.Cursor.X) + .5, 0, float64(b.Cursor.Y) + .5}
		p.tabFrom, p.tabTo, p.at = nil, &t, t
	}
	p.cursor = b.Cursor
	focus := vec{float64(b.View.X) + 6, 0, float64(b.View.Y) + 3.5}
	for _, u := range b.Units {
		if u.Active {
			c := unitCentre(u)
			focus = vec{c[0], 0, c[1]}
		}
	}
	if p.tabTo != nil {
		focus = *p.tabTo
	}
	if n := len(b.Effects); n > 0 {
		f := b.Effects[n-1]
		focus = vec{f.X + .5, 0, f.Y + .5}
	}
	if focus != p.focus { // a new thing to watch: let go of the pan
		p.focus, p.offset = focus, vec{}
	}
	p.pan(dt)
	focus = focus.add(p.offset)
	k := 1 - math.Exp(-dt*8)
	p.yaw += (p.yawTo - p.yaw) * k
	p.dist += (p.distTo - p.dist) * k
	p.at = p.at.add(focus.sub(p.at).mul(k))

	r := dst.Bounds()
	h := p.dist * math.Cos(pitch)
	p.cam = camera{
		eye:  vec{p.at[0] + h*math.Sin(p.yaw), p.dist * math.Sin(pitch), p.at[2] + h*math.Cos(p.yaw)},
		at:   p.at,
		fovy: 0.8, aspect: float64(r.Dx()) / float64(r.Dy()),
	}
	p.screen = r
	vp := p.cam.viewProj()
	eye := [3]float32{float32(p.cam.eye[0]), float32(p.cam.eye[1]), float32(p.cam.eye[2])}
	light := gpu.MeshOptions{Light: sunDir, Ambient: 0.5, Eye: eye}
	// an effect lights its surroundings in its own colour
	var glows []*figure
	for _, f := range b.Effects {
		g := p.figure(f.Pic)
		glows = append(glows, g)
		light.Point = [4]float32{float32(f.X) + .5, 1.1, float32(f.Y) + .5, 3.5}
		light.PointColor = [4]float32{g.glow[0], g.glow[1], g.glow[2], 1.6}
	}
	opts := func(model gpu.Mat4, tint [4]float32, spec float32) *gpu.MeshOptions {
		o := light
		o.Model, o.Tint, o.Spec = model, tint, spec
		return &o
	}

	dst.Fill(color.RGBA{0x0C, 0x0B, 0x0A, 0xFF})
	dst.ClearDepth()
	dst.DrawMesh(p.table, p.wood, vp, opts(gpu.Ident, [4]float32{1, 1, 1, 1}, 0.1))
	dst.DrawMesh(p.board, p.atlas, vp, opts(gpu.Ident, [4]float32{1, 1, 1, 1}, 0.04))

	// shadows under what stands: the map's things, then the figurines
	shade := gpu.MeshOptions{Blend: gpu.MeshAlpha, Tint: [4]float32{0, 0, 0, 0.7}}
	dst.DrawMesh(p.shadows, p.shadow, vp, &shade)
	shown := map[int]bool{}
	for _, u := range b.Units {
		if u.Icon == nil {
			continue
		}
		shown[u.ID] = true
		x, z, sq := p.unitAt(u, dt)
		o := shade
		o.Model = place(x+.12, 0.004, z+.1, 0, sq*1.05, 1, sq*1.05)
		dst.DrawMesh(p.flatQuad, p.shadow, vp, &o)
	}
	for id := range p.units {
		if !shown[id] {
			delete(p.units, id)
		}
	}

	// frames on the board: choices, the cursor, the square under the mouse
	frame := func(x, z int, tint [4]float32, lift float32) {
		o := gpu.MeshOptions{Blend: gpu.MeshAdd, Tint: tint}
		fx, fz := float32(x), float32(z)
		for _, bar := range [4][4]float32{{fx + .5, fz + .06, .9, .06}, {fx + .5, fz + .94, .9, .06}, {fx + .06, fz + .5, .06, .9}, {fx + .94, fz + .5, .06, .9}} {
			o.Model = place(bar[0], 0.006+lift, bar[1], 0, bar[2], 0.01, bar[3])
			dst.DrawMesh(p.box, p.white, vp, &o)
		}
	}
	marks, fill, cursor := markGlow, areaFill, cursorGlow
	if b.Area { // a spell's area: XCOM's orange
		marks = areaGlow
	}
	if b.AimWhy != "" { // the aimed square cannot be taken: say so in red
		marks, fill, cursor = blockedGlow, blockedFill, blockedGlow
	}
	for _, m := range b.Marks {
		if !targetAt(b, m) { // a target's red frame stands alone
			frame(m.X, m.Y, marks, 0)
		}
	}
	for _, a := range b.AreaAt { // the ground a spell's area covers: an orange wash
		if slices.Contains(b.Skipped, a) { // an ally the cast passes over: framed apart below
			continue
		}
		o := gpu.MeshOptions{Blend: gpu.MeshAdd, Tint: fill, Model: place(float32(a.X)+.5, 0.0035, float32(a.Y)+.5, 0, .96, .005, .96)}
		dst.DrawMesh(p.box, p.white, vp, &o)
	}
	for _, s := range b.Skipped {
		frame(s.X, s.Y, skipGlow, 0.001)
	}
	for _, u := range b.Units { // the foes a click attacks: red
		if slices.Contains(b.Targets, u.ID) {
			for _, s := range u.Squares {
				frame(s.X, s.Y, targetGlow, 0.001)
			}
		}
	}
	for _, r := range b.Reach { // where a click walks: XCOM's blue
		o := gpu.MeshOptions{Blend: gpu.MeshAdd, Tint: reachGlow, Model: place(float32(r.X)+.5, 0.003, float32(r.Y)+.5, 0, .96, .005, .96)}
		dst.DrawMesh(p.box, p.white, vp, &o)
	}
	if mx, my, ok := p.Pick(gpu.CursorPosition()); ok && !p.overBar(gpu.CursorPosition()) {
		frame(mx, my, hoverGlow, 0)
		for i, s := range b.Paths[image.Pt(mx, my)] { // the walk a click takes; red where leaving lets a foe strike
			if i == 0 { // the mover's own square
				continue
			}
			tint := pathGlow
			if s.Free {
				tint = freeGlow
			}
			o := gpu.MeshOptions{Blend: gpu.MeshAdd, Tint: tint, Model: place(float32(s.X)+.5, 0.004, float32(s.Y)+.5, 0, .5, .005, .5)}
			dst.DrawMesh(p.box, p.white, vp, &o)
		}
	}
	pulse := float32(0.75 + 0.25*math.Sin(p.clock*5))
	if b.CursorOn {
		for _, c := range b.CursorAt {
			frame(c.X, c.Y, [4]float32{cursor[0] * pulse, cursor[1] * pulse, cursor[2] * pulse, 1}, 0.002)
		}
	}

	// the figurines
	var heads []head // where each one's health bar goes (Shift)
	for _, u := range b.Units {
		if u.Icon == nil {
			continue
		}
		x, z, sq := p.unitAt(u, dt)
		tint := enemyColour
		if u.Party {
			tint = partyColour
		}
		dst.DrawMesh(p.base, p.white, vp, opts(place(x, 0, z, 0, sq*.78, pedestal, sq*.78), tint, 0.25))
		if u.Active { // whose turn: a gold ring round the base
			dst.DrawMesh(p.base, p.white, vp, opts(place(x, 0, z, 0, sq*.86, pedestal*.5, sq*.86),
				[4]float32{0.95 * pulse, 0.72 * pulse, 0.2 * pulse, 1}, 0.9))
		}
		f := p.figure(u.Icon)
		ib := u.Icon.Bounds()
		size := sq * .92
		half := size * float32(ib.Dy()) / float32(max(ib.Dx(), ib.Dy())) / 2
		// the attacker lunges at its target; the target flashes red and sheds sparks when hit, grey ones when missed
		tint = [4]float32{1, 1, 1, 1}
		if t := p.strikeT; t < lunge {
			if to, ok := p.units[p.strike.To]; ok && u.ID == p.strike.From {
				dx, dz := float32(to[0])-x, float32(to[1])-z
				if n := float32(math.Hypot(float64(dx), float64(dz))); n > 0 {
					d := float32(math.Sin(math.Pi*t/lunge)) * .35 / n
					x, z = x+dx*d, z+dz*d
				}
			}
			if u.ID == p.strike.To && t >= lunge/2 {
				col, n := [3]float32{.6, .6, .6}, 6
				if p.strike.Hit {
					col, n = [3]float32{1, .15, .05}, 16
					w := float32(4 * (1 - t/lunge)) // the flash, fading
					tint = [4]float32{1 + w, 1 - w/4, 1 - w/4, 1}
				}
				for i := 0; !p.burst && i < n; i++ {
					p.sparks = append(p.sparks, spark{
						p:   vec{float64(x), pedestal + float64(half), float64(z)},
						v:   vec{rand.Float64()*2 - 1, rand.Float64() * 1.5, rand.Float64()*2 - 1},
						max: 0.3 + rand.Float64()*.3, col: col,
					})
				}
				p.burst = true
			}
		}
		o := opts(place(x, pedestal+half, z, 0, size, size, size), tint, 0.12)
		o.Ambient = 0.85 // fixed facing, so its back is lit enough to read from any heading
		dst.DrawMesh(f.mesh, f.tex, vp, o)
		heads = append(heads, head{u, vec{float64(x), float64(pedestal + 2*half + .1), float64(z)}})
	}

	// effects: the sprite hangs over its square, glowing, shedding sparks
	for i, f := range b.Effects {
		g := glows[i]
		ib := f.Pic.Bounds()
		sq := float32(max(ib.Dx(), ib.Dy())) / tilePx
		x, z := float32(f.X)+float32(ib.Dx())/tilePx/2, float32(f.Y)+float32(ib.Dy())/tilePx/2
		full := gpu.MeshOptions{Tint: [4]float32{1.3, 1.3, 1.3, 1}, Light: sunDir, Ambient: 1}
		full.Model = place(x, 1.05, z, float32(p.yaw), sq*.9, sq*.9, sq*.9) // over the heads, facing the eye: light, not figurines
		dst.DrawMesh(g.mesh, g.tex, vp, &full)
		halo := gpu.MeshOptions{Blend: gpu.MeshAdd, Tint: [4]float32{g.glow[0], g.glow[1], g.glow[2], 0.5}}
		halo.Model = place(x, 0.01, z, 0, 2.2, 1, 2.2)
		dst.DrawMesh(p.flatQuad, p.shadow, vp, &halo) // a pool of light on the board
		for range 2 {
			p.sparks = append(p.sparks, spark{
				p:   vec{float64(x) + rand.Float64()*.6 - .3, 0.8 + rand.Float64()*.4, float64(z) + rand.Float64()*.6 - .3},
				v:   vec{rand.Float64()*.4 - .2, 0.5 + rand.Float64()*.6, rand.Float64()*.4 - .2},
				max: 0.5 + rand.Float64()*.5, col: g.glow,
			})
		}
	}
	p.drawSparks(dst, vp, dt)
	if gpu.IsKeyPressed(gpu.KeyShift) {
		p.drawHealth(dst, vp, heads)
	}
}

// drawSparks moves, fades and draws the particles.
func (p *Presenter) drawSparks(dst *gpu.Image, vp gpu.Mat4, dt float64) {
	live := p.sparks[:0]
	for _, s := range p.sparks {
		s.life += dt
		if s.life >= s.max {
			continue
		}
		s.p = s.p.add(s.v.mul(dt))
		a := float32(1 - s.life/s.max)
		o := gpu.MeshOptions{Blend: gpu.MeshAdd, Tint: [4]float32{s.col[0], s.col[1], s.col[2], a}}
		o.Model = place(float32(s.p[0]), float32(s.p[1]), float32(s.p[2]), float32(p.yaw), .05, .05, .05)
		dst.DrawMesh(p.box, p.white, vp, &o)
		live = append(live, s)
	}
	p.sparks = live
}

// tick is the time since the last frame, at most a tenth of a second.
func (p *Presenter) tick() float64 {
	now := time.Now()
	dt := 0.1
	if !p.last.IsZero() {
		dt = min(now.Sub(p.last).Seconds(), 0.1)
	}
	p.last = now
	p.clock += dt
	return dt
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
	ib := u.Icon.Bounds()
	return float32(at[0]), float32(at[1]), float32(max(ib.Dx(), ib.Dy())) / tilePx
}

// sync brings the atlas and the board up to b.
func (p *Presenter) sync(b *engine.Battle) {
	if p.white == nil {
		p.white = gpu.NewImage(1, 1)
		p.white.Fill(color.White)
		p.box = gpu.NewMesh(box())
		p.base = gpu.NewMesh(disc(24))
		p.flatQuad = gpu.NewMesh(quad(nil, [4][3]float32{{-.5, 0, .5}, {.5, 0, .5}, {.5, 0, -.5}, {-.5, 0, -.5}},
			[4][2]float32{{0, 1}, {1, 1}, {1, 0}, {0, 0}}, [3]float32{0, 1, 0}))
		p.atlas = gpu.NewImage(atlasCols*tilePx, atlasRows*tilePx)
		p.shadow = gpu.NewImageFromImage(blob(32))
		p.wood = gpu.NewImageFromImage(woodTexture(64))
		p.table = gpu.NewMesh(tableMesh())
	}
	changed := b.Floor != p.floor
	for i := range p.tiles {
		var t image.Image
		if i < len(b.Tiles) {
			t = b.Tiles[i]
		}
		if t != p.tiles[i] {
			p.tiles[i], changed = t, true
			p.empty[i] = isEmpty(t)
		}
	}
	if changed {
		p.floor = b.Floor
		a := image.NewRGBA(p.atlas.Bounds())
		put := func(slot int, t image.Image) {
			o := image.Pt(slot%atlasCols*tilePx, slot/atlasCols*tilePx)
			draw.Draw(a, image.Rectangle{o, o.Add(t.Bounds().Size())}, t, t.Bounds().Min, draw.Src)
		}
		for i, t := range p.tiles {
			if t != nil {
				put(i, t)
			}
		}
		put(wallSlot, stoneTexture(p.tiles[:engine.DungeonTiles], b.Floor))
		put(floorSlot, floorTile(b.Floor))
		p.atlas.WritePixels(a.Pix)
	}
	if p.board == nil || changed || b.Ground != p.ground || b.Dungeon != p.dungeon {
		p.ground, p.dungeon = b.Ground, b.Dungeon
		for _, m := range []*gpu.Mesh{p.board, p.shadows} {
			if m != nil {
				m.Dispose()
			}
		}
		empty := func(t int) bool { return t < 0 || t >= len(p.empty) || p.empty[t] }
		p.board = gpu.NewMesh(append(mapMesh(&p.ground, p.dungeon, empty, p.tiles[:]), boardSides()...))
		p.shadows = gpu.NewMesh(standShadows(&p.ground, p.dungeon, empty))
	}
}

// figure is the extruded mesh for a picture (cached).
func (p *Presenter) figure(pic image.Image) *figure {
	if f, ok := p.figs[pic]; ok {
		return f
	}
	if p.figs == nil {
		p.figs = map[image.Image]*figure{}
	}
	f := &figure{gpu.NewMesh(extrude(pic)), gpu.NewImageFromImage(pic), averageColour(pic)}
	p.figs[pic] = f
	return f
}

// isEmpty: no tile, or nothing on the ground.
func isEmpty(t image.Image) bool {
	if t == nil {
		return true
	}
	b := t.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := t.At(x, y).RGBA(); a != 0 {
				return false
			}
		}
	}
	return true
}

// averageColour is a picture's mean opaque colour, brightened to a light's.
func averageColour(t image.Image) [3]float32 {
	var r, g, bl, n float64
	b := t.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if cr, cg, cb, a := t.At(x, y).RGBA(); a > 0x8000 {
				r, g, bl, n = r+float64(cr), g+float64(cg), bl+float64(cb), n+1
			}
		}
	}
	if n == 0 {
		return [3]float32{1, 1, 1}
	}
	m := max(r, g, bl) / n / 0xFFFF
	if m == 0 {
		return [3]float32{1, 1, 1}
	}
	return [3]float32{float32(r / n / 0xFFFF / m), float32(g / n / 0xFFFF / m), float32(bl / n / 0xFFFF / m)}
}
