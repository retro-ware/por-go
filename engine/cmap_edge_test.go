package engine

import "testing"

// TestTileOfNoWrap: squares just off the left and right edges are bare
// floor, not the far edge's wall pieces (the map never wraps).
func TestTileOfNoWrap(t *testing.T) {
	e := &Engine{cmap: &combatMap{}}
	bare := e.groundInfo(groundFloor)[3]
	var wall byte
	for g := range groundTable {
		if groundTable[g][3] != bare {
			wall = byte(g)
			break
		}
	}
	e.cmap.ground[3][0], e.cmap.ground[3][49] = wall, wall
	if got := e.tileOf(50, 2); got != bare {
		t.Errorf("right of row 2: tile %d, the next row's first square leaks in", got)
	}
	if got := e.tileOf(-1, 4); got != bare {
		t.Errorf("left of row 4: tile %d, the previous row's last square leaks in", got)
	}
	if got := e.tileOf(0, 3); got == bare {
		t.Errorf("on the map: bare ground, want the wall")
	}
}
