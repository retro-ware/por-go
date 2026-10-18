package engine

// Area of effect preview (extra): while the player aims a spell that hits
// everyone round the target square (chooseSpellTargets's kinds 9..E), the combatants
// it would hit get the dotted frame Move and Aim use (markReach) and counted in the prompt.
// The test is the cast's own: buildNear round the square, radius kind&7.
// ponytail: Lightning Bolt (kind 8, a line) gets no preview; boltPath
// would need a dry run.

import "strconv"

// aoeMarks marks who the spell aimed at (x,y) would hit; the prompt says
// how many on each side. undo takes the marks off again.
func (e *Engine) aoeMarks(x, y int, prompt string) (string, func()) {
	if e.ext.aoe == 0 {
		return prompt, func() {}
	}
	saved := e.near
	e.near = nil
	e.buildNear(x, y, e.ext.aoe-1, 0xFF, 1)
	hit := e.near
	e.near = saved
	foes, friends := 0, 0
	marks := map[[2]int][]int{}
	for _, n := range hit {
		p := e.cpos[n.index]
		if c := e.combatant(n.index); c != nil && c.Side == e.selected().Side {
			friends++
		} else {
			foes++
		}
		if int(p.x) != x || int(p.y) != y || e.cmap.cursor == 0 { // the cursor marks its own square
			marks[[2]int{int(p.x), int(p.y)}] = nil
		}
	}
	e.markReach(marks, true)
	undo := func() { e.markReach(marks, false) }
	return "Hits " + strconv.Itoa(foes) + " foe " + strconv.Itoa(friends) + " ally ", undo
}
