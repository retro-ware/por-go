package engine

import "testing"

// Leaving a map west lands on the east edge (the next area keeps it);
// inside the map an edge step stays put.
func TestStepForwardEdge(t *testing.T) {
	e := &Engine{}
	e.posX, e.posY, e.facing = 0, 4, 6
	e.StepForward()
	if e.posX != 0 {
		t.Fatalf("edge step without LeftMap moved to x=%d", e.posX)
	}
	e.game.LeftMap = 1
	e.StepForward()
	if e.posX != 15 || e.posY != 4 {
		t.Fatalf("leaving west: %d,%d, want 15,4", e.posX, e.posY)
	}
}
