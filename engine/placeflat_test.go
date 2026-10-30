package engine_test

import (
	"testing"
	"time"

	"por/engine"
)

// TestDeskewPlacement: on a deskewed map each side stands on its own
// cell's floor, the monsters in the cell ahead of the party, so nothing but
// the open cell edge lies between them (the start of Phlan: a corridor
// facing west).
func TestDeskewPlacement(t *testing.T) {
	e := explore(t)
	e.Deskew.Store(true)
	e.DevAction = e.DevScene("combat")
	seed := uint32(1)
	e.FixedSeed = &seed
	e.Init()
	go e.Play()
	var v engine.View
	for range 7 {
		for !e.Idle() {
			time.Sleep(10 * time.Millisecond)
		}
		if v = e.View(); v.Battle != nil {
			break
		}
		e.Act(cont)
		time.Sleep(50 * time.Millisecond)
	}
	if v.Battle == nil {
		t.Fatal("no fight")
	}
	for _, u := range v.Battle.Units {
		x0 := 22 // the party's cell: floor columns 22..26, rows 11..14
		if !u.Party {
			x0 -= 6 // the cell west of it
		}
		if u.X < x0 || u.X > x0+4 || u.Y < 11 || u.Y > 14 {
			t.Errorf("%s (party %v) at %d,%d, want x %d..%d, y 11..14", u.Name, u.Party, u.X, u.Y, x0, x0+4)
		}
	}
}
