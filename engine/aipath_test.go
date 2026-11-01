package engine

import "testing"

// TestAIPathAroundWall: a wall stands between two foes, open only at its
// top end; the first step heads up towards the gap, not straight at the
// wall.
func TestAIPathAroundWall(t *testing.T) {
	e := &Engine{cmap: &combatMap{}}
	for y := range e.cmap.ground {
		for x := range e.cmap.ground[y] {
			e.cmap.ground[y][x] = groundFloor
		}
	}
	for y := 3; y < 25; y++ {
		e.cmap.ground[y][10] = 1 // wall at x 10, gap above y 3
	}
	a, b := &Character{}, &Character{}
	e.combatants, e.ncomb = []*Character{a, b}, 2
	e.cpos[1] = combatPos{x: 8, y: 10, size: 1}
	e.cpos[2] = combatPos{x: 12, y: 10, size: 1}
	e.occ[10][8], e.occ[10][12] = 1, 2
	d := e.aiPathDir(a, b)
	if d < 0 || dirDY[d] >= 0 {
		t.Fatalf("first step %d (dx %d dy %d) does not head for the gap", d, dirDX[d], dirDY[d])
	}
}
