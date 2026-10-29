package engine

// Area of effect preview (extra): while the player aims a spell, the
// combatants it would hit get the dotted frame Move and Aim use (markReach)
// and are counted in the prompt, its squares go to the snapshot (area).
// The test is the cast's own: buildNear round the square, radius kind&7
// for chooseSpellTargets's kinds 9..E, 0 (the square itself) for single
// targets; a bolt (bolts) walks boltWalk dry: the squares it crosses are
// the area, the ones it strikes the marks.

import (
	"image"
	"strconv"
)

// bolts is the reach and bounce boltPath gets from each spell whose effect
// is a bolt (fxLightning, fxBolt).
var bolts = map[int][2]int{spLightningBolt: {8, 1}, 60: {3, 0}}

// aoeMarks marks who the spell aimed at (x,y) would hit; the prompt says
// how many on each side. undo takes the marks off again.
func (e *Engine) aoeMarks(x, y int, prompt string) (string, func()) {
	if e.ext.aoe == 0 {
		return prompt, func() {}
	}
	var hit []byte // combatant numbers
	e.snap.area, e.snap.skipped = nil, nil
	if b := e.ext.bolt; b[0] > 0 {
		hit, e.snap.area = e.boltPreview(x, y, b[0], b[1])
	} else {
		saved := e.near
		e.near = nil
		e.buildNear(x, y, e.ext.aoe-1, 0xFF, 1)
		for _, n := range e.near {
			hit = append(hit, n.index)
		}
		e.near = saved
		// the squares buildNear's line test reaches round (x,y), for views that draw the area
		for sy := max(0, y-e.ext.aoe+1); sy <= min(24, y+e.ext.aoe-1); sy++ {
			for sx := max(0, x-e.ext.aoe+1); sx <= min(49, x+e.ext.aoe-1); sx++ {
				r, ax, ay := e.ext.aoe-1, sx, sy
				if e.lineOfSight(&r, &ax, &ay, x, y) {
					e.snap.area = append(e.snap.area, image.Pt(sx, sy))
				}
			}
		}
	}
	foes, friends := 0, 0
	marks := map[[2]int][]int{}
	for _, i := range hit {
		p := e.cpos[i]
		if c := e.combatant(i); c != nil && c.Side == e.selected().Side {
			if e.passedOver(c) { // left out by the cast: shown apart, not counted
				e.snap.skipped = append(e.snap.skipped, image.Pt(int(p.x), int(p.y)))
				continue
			}
			friends++
		} else {
			foes++
		}
		if int(p.x) != x || int(p.y) != y || e.cmap.cursor == 0 { // the cursor marks its own square
			marks[[2]int{int(p.x), int(p.y)}] = nil
		}
	}
	e.snap.marks = nil
	for k := range marks {
		e.snap.marks = append(e.snap.marks, image.Pt(k[0], k[1]))
	}
	undo := func() { e.snap.marks, e.snap.area, e.snap.skipped = nil, nil, nil }
	return "Hits " + strconv.Itoa(foes) + " foe " + strconv.Itoa(friends) + " ally ", undo
}

// passedOver: applySpellToSide's test for an ally in the area the spell
// aimed leaves out: Bless skips one with an enemy next to it. The near
// and target lists are kept.
func (e *Engine) passedOver(c *Character) bool {
	if e.ext.aoeSpell != spBless {
		return false
	}
	near, targets := e.near, e.targets
	e.near = nil
	defer func() { e.near, e.targets = near, targets }()
	return e.nearEnemies(c, 1) > 0
}

// boltPreview walks a bolt from (x,y) as its cast would (the target
// square, then boltPath(reach, ..., bounce)) without striking: who it hits
// (each once) and the squares it crosses. ponytail: the occupants stay
// put, so a foe the cast would kill still stops the line.
func (e *Engine) boltPreview(x, y, reach, bounce int) (hit []byte, line []image.Point) {
	tx, ty := e.tgtX, e.tgtY
	defer func() { e.tgtX, e.tgtY = tx, ty }()
	e.tgtX, e.tgtY = x, y
	seen := map[byte]bool{}
	strike := func(x, y int) {
		if who, _ := e.mapAt(x, y); who > 0 && !seen[who] {
			seen[who] = true
			hit = append(hit, who)
		}
	}
	strike(x, y)
	line = append(line, image.Pt(x, y))
	e.boltWalk(reach, bounce, func(x, y int) { line = append(line, image.Pt(x, y)) }, func(_, _, x, y int) bool {
		strike(x, y)
		_, ground := e.mapAt(x, y)
		return ground > 0 && groundTable[ground][0] == 0xFF
	})
	return hit, line
}
