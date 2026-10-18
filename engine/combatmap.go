package engine

import "slices"

// Combat map services (ovr31/ovr32 and the START sprite blitter) that the
// rules call: the near list and the helpers over the combat map.

// nearEntry is one 3-byte record of the DS:6718 list built by ovr31:0909.
type nearEntry struct{ index, dist, dir byte }

// MsgDelay = 0709:0DBE: wait msgDelay*225 ms.
func (e *Engine) MsgDelay() { e.Delay(int(e.msgDelay) * 225) }

// combatant = DS:65B9[i]: the combatant behind a near list index.
func (e *Engine) combatant(i byte) *Character {
	if int(i) < 1 || int(i) > len(e.combatants) {
		return nil
	}
	return e.combatants[i-1]
}

// sign = ovr31:0000.
func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// line is the ovr31 line walker record (0x1A bytes): ovr31:01A0 starts it,
// ovr31:02CC takes one step.
type line struct {
	x1, y1, err, dx, dy, incA, incB, sx, sy, x, y, steps int
	dir                                                  byte
}

func newLine(x0, y0, x1, y1 int) *line {
	l := &line{x1: x1, y1: y1, dx: abs(x1 - x0), dy: abs(y1 - y0), sx: sign(x1 - x0), sy: sign(y1 - y0), x: x0, y: y0}
	if l.dx > l.dy {
		l.err, l.incA, l.incB = 2*l.dy-l.dx, 2*(l.dy-l.dx), 2*l.dy
	} else {
		l.err, l.incA, l.incB = 2*l.dx-l.dy, 2*(l.dx-l.dy), 2*l.dx
	}
	return l
}

// step moves one square; dir is the direction taken (8 = none, DS:25C0).
func (e *Engine) step(l *line) bool {
	mx, my, moved := 0, 0, false
	if l.dx > l.dy {
		if l.x != l.x1 {
			mx = l.sx
			if l.err >= 0 {
				my = l.sy
				l.y += l.sy
				l.err += l.incA
				l.steps += 3
			} else {
				l.err += l.incB
				l.steps += 2
			}
			l.x += l.sx
			moved = true
		}
	} else if l.y != l.y1 {
		my = l.sy
		if l.err >= 0 {
			mx = l.sx
			l.x += l.sx
			l.err += l.incA
			l.steps += 3
		} else {
			l.err += l.incB
			l.steps += 2
		}
		l.y += l.sy
		moved = true
	}
	l.dir = mouseDir[my+1][mx+1]
	return moved
}

// flyMissile = 00BA:1A29 (x1, y1, x2, y2, frames, delay): the missile
// sprite (DS:6E36, loaded by hitColours) flies from square to square in
// thirds of a square, cycling its frames and scrolling the map along.
func (e *Engine) flyMissile(x1, y1, x2, y2, frames, delay int) {
	var path []byte
	l := newLine(x1*3, y1*3, x2*3, y2*3)
	for {
		moved := e.step(l)
		path = append(path, l.dir)
		if !moved {
			break
		}
	}
	n := len(path)
	if n < 2 {
		return
	}
	path = append(path, 8) // the rest of the 0x94 byte buffer
	dX, dY := x2-x1, y2-y1
	var done bool
	var cx, cy int
	if combatInView(x1-e.cmap.viewX, y1-e.cmap.viewY) && combatInView(x2-e.cmap.viewX, y2-e.cmap.viewY) {
		done, cx, cy = true, e.cmap.viewX+viewW/2, e.cmap.viewY+3
	} else if abs(dX) <= viewW-1 && abs(dY) <= 6 {
		done, cx, cy = true, x1+dX/2, y1+dY/2
	} else {
		cx, cy = e.cmap.viewX+viewW/2, e.cmap.viewY+3
	}
	e.combatCentre(cx, cy, 0xFF, 8)
	i, f, ox, oy := 0, 0, 0, 0
	for {
		px := (x1-e.cmap.viewX)*3 + ox
		py := (y1-e.cmap.viewY)*3 + oy
		sx, sy := x1, y1
		out := false
		for {
			if delay > 0 || px%3 == 0 || py%3 == 0 {
				e.drawHitFrame(f, px, py)
				e.Flip()
				e.Delay(delay)
				e.restoreHitFrame(px, py)
				if f++; f >= frames {
					f = 0
				}
			}
			i++
			d := path[i]
			mx, my := dirDX[d], dirDY[d]
			px += mx
			py += my
			out = px < 0 || px > viewW*3-3 || py < 0 || py > 0x12
			if !out && i < n {
				ox += mx
				oy += my
				if abs(ox) == 3 {
					x1 += sign(ox)
					cx += sign(ox)
					ox = 0
				}
				if abs(oy) == 3 {
					y1 += sign(oy)
					cy += sign(oy)
					oy = 0
				}
			}
			if i >= n || out {
				break
			}
		}
		if i < n {
			if sign(x1-sx) != 0 {
				if x1 >= e.cmap.viewX+viewW-1 {
					cx = x1 + 3
				} else if x1 <= e.cmap.viewX {
					cx = x1 - 3
				}
			}
			if sign(y1-sy) != 0 {
				if y1 >= e.cmap.viewY+6 {
					cy = y1 + 3
				} else if y1 <= e.cmap.viewY {
					cy = y1 - 3
				}
			}
			e.combatCentre(cx, cy, 0xFF, 8)
		} else {
			done = true
			if !combatInView(x2-e.cmap.viewX, y2-e.cmap.viewY) {
				e.combatCentre(x2, y2, 3, 8)
			}
			px, py := (x2-e.cmap.viewX)*3, (y2-e.cmap.viewY)*3
			e.drawHitFrame(f, px, py)
			if delay > 0 {
				e.Flip()
				e.Delay(delay)
				e.restoreHitFrame(px, py)
			}
		}
		if done {
			break
		}
	}
	e.Flip()
}

// isParty = 00BA:23B7 (char): true for the party side.
func isParty(c *Character) bool { return c.Side == sideParty }

// nearEnemies = 00BA:2433 (char, range): combatants of the other side
// within range; the near list keeps only them, copied to DS:6DE4.
func (e *Engine) nearEnemies(c *Character, r int) int {
	e.nearCombatants(c, e.combatX(c), e.combatY(c), r)
	var want byte // the original keeps the targets whose side byte equals isParty(c)
	if isParty(c) {
		want = 1
	}
	kept := slices.DeleteFunc(e.near, func(n nearEntry) bool {
		t := e.combatant(n.index)
		return t == nil || t.Side != want
	})
	e.near = kept
	for i, n := range kept {
		e.targets[i+1] = n.index
	}
	return len(kept)
}

// rangeTo = 00BA:255B (char, target): distance in squares (list distance
// halved) from char to target, the near list left as it was.
func (e *Engine) rangeTo(c, t *Character) int {
	saved := e.near
	e.near = append([]nearEntry(nil), saved...)
	e.cmap.seeAll = 1 // [DS:6716]+6: line of sight ignores walls
	e.nearCombatants(c, e.combatX(c), e.combatY(c), 0xFF)
	e.cmap.seeAll = 0
	var d byte
	for i, n := range e.near {
		if e.combatant(n.index) == t || i == len(e.near)-1 {
			d = n.dist
			break
		}
	}
	// only the 0xD8 entry bytes come back, the count (DS:671A) stays
	list := make([]nearEntry, len(e.near))
	copy(list, saved)
	e.near = list
	return int(d >> 1)
}

// casterLevel = 00BA:26C2 (spell): level the spell is cast at by the
// selected character; items (DS:6DBF) cast cleric and mage spells at 6.
func (e *Engine) casterLevel(spell int) int {
	lvl := 0
	switch spellTable[spell][0] {
	case 0:
		lvl = int(e.selected().Levels[clsCleric])
	case 1:
		lvl = int(e.selected().Levels[clsMage])
	case 2:
		lvl = 0xC
	}
	if e.usingItem && spellTable[spell][0] != 2 {
		lvl = 6
	}
	return lvl
}
