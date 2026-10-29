package engine

import "time"

// Auto-walk (extra): Walk on the full map sends the
// party to the cursor square along the shortest way through open sides,
// one step at a time, as if the arrows were pressed. It
// stops at the square, at any key, or when something happens: a script
// that does more than compute (text, a picture, a menu, monsters), a menu
// other than the command bar, or a step that did not land where expected.

type walkState struct {
	tx, ty     int // target square
	area       int // areaKey the walk belongs to
	ex, ey, ef int // where the last key should leave the party
	event      bool
	last       time.Time
}

const walkPace = 150 * time.Millisecond

// walkQuiet are the ECL ops a plain step may run without stopping the walk
// (PICTURE and PRINT only stop it when they show something: opPicture,
// opPrint).
var walkQuiet = func() (q [0x3E]bool) {
	for _, op := range []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0E,
		0x11, 0x12, 0x13, 0x14, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x25, 0x26, 0x2A,
		0x2D, 0x2F, 0x30, 0x31, 0x33, 0x34, 0x35, 0x3A, 0x3D} {
		q[op] = true
	}
	return
}()

// walkOp notes an ECL op run during a walk.
func (e *Engine) walkOp(op byte) {
	if w := e.ext.walk; w != nil && (int(op) >= len(walkQuiet) || !walkQuiet[op]) {
		w.event = true
	}
}

// startWalk sets off for (tx, ty); false when no known way leads there.
func (e *Engine) startWalk(tx, ty int) bool {
	if e.posX == tx && e.posY == ty {
		return true // there already
	}
	if e.viewMode != 1 || e.walkDir(tx, ty) < 0 {
		return false
	}
	e.ext.walk = &walkState{tx: tx, ty: ty, area: e.areaKey(), ex: e.posX, ey: e.posY, ef: e.facing}
	return true
}

// walkFeed hands the command bar the walk's next key when it is time.
func (e *Engine) walkFeed() {
	w := e.ext.walk
	if w == nil || e.ext.held.Kind != ActNone { // the last step is still being read
		return
	}
	stop := func() { e.ext.walk = nil }
	if len(e.acts.ch) > 0 { // any action stops the walk and is used up
		e.nextAct()
		stop()
		return
	}
	if !e.ext.inCommand || e.ext.fullMap || w.event || e.viewMode != 1 || e.areaKey() != w.area ||
		e.posX != w.ex || e.posY != w.ey || e.facing != w.ef ||
		e.posX == w.tx && e.posY == w.ty {
		stop()
		return
	}
	if time.Since(w.last) < walkPace {
		return
	}
	d := e.walkDir(w.tx, w.ty)
	if d < 0 {
		stop()
		return
	}
	w.last = time.Now()
	step := DirUp // a step ahead
	switch (d - e.facing + 8) % 8 {
	case 0:
		w.ex, w.ey = e.posX+dirDX[d], e.posY+dirDY[d]
	case 6:
		step, w.ef = DirLeft, (e.facing+6)%8
	default:
		step, w.ef = DirRight, (e.facing+2)%8 // twice to turn round
	}
	e.ext.held = Action{Kind: ActMove, N: step}
}

// walkDir is the direction (0 2 4 6) of the first step on the shortest
// way from the party to (tx, ty) through open sides, or -1. Doors that are
// locked count as walls; the map shows every wall, so does the way.
func (e *Engine) walkDir(tx, ty int) int {
	if !onMap(e.posX, e.posY) || !onMap(tx, ty) {
		return -1
	}
	from := [256]int{}
	for i := range from {
		from[i] = -1
	}
	start, goal := e.posY<<4|e.posX, ty<<4|tx
	from[goal] = goal
	queue := []int{goal} // from the target, so from[] points the way back
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		x, y := s&15, s>>4
		for _, d := range [4]int{0, 2, 4, 6} {
			nx, ny := x+dirDX[d], y+dirDY[d]
			n := ny<<4 | nx
			if !onMap(nx, ny) || from[n] >= 0 ||
				e.DoorAt(nx, ny, (d+4)%8) != 1 {
				continue
			}
			from[n] = s
			if n == start {
				nxt := s
				for d := range dirDX {
					if d%2 == 0 && e.posX+dirDX[d] == nxt&15 && e.posY+dirDY[d] == nxt>>4 {
						return d
					}
				}
				return -1
			}
			queue = append(queue, n)
		}
	}
	return -1
}
