package engine

import (
	"testing"
	"time"
)

func TestLineAndMissile(t *testing.T) {
	e := &Engine{cmap: &combatMap{}}
	l := newLine(0, 0, 6, -3)
	var dirs []byte
	for e.step(l) {
		dirs = append(dirs, l.dir)
	}
	if l.x != 6 || l.y != -3 || len(dirs) != 6 || l.dir != 8 || dirs[0] != 1 || dirs[1] != 2 {
		t.Fatalf("line ended at %d,%d dirs %v last %d", l.x, l.y, dirs, l.dir)
	}
	e.flyMissile(1, 1, 9, 4, 2, 0) // must finish with the map stubs
}

// TestPlaceOneGivesUp: with every slot of every team taken, placing one more
// fails instead of running its rows off the block for ever (a soft-lock of a
// big fight on a deskewed map), whichever way each side faces.
func TestPlaceOneGivesUp(t *testing.T) {
	for _, deskew := range []bool{true, false} {
		for d := range byte(4) {
			for s := 0; s <= 1; s++ {
				e := &Engine{cmap: &combatMap{}}
				e.Deskew.Store(deskew)
				e.viewMode = 2 // no walls between the team blocks
				e.place.side, e.place.dir[s], e.place.rows[s] = byte(s), d, 3
				done := make(chan bool)
				go func() { done <- e.placeOne(1) }()
				select {
				case ok := <-done:
					if ok {
						t.Fatalf("deskew %v dir %d side %d: placed with no free slot", deskew, d, s)
					}
				case <-time.After(time.Second):
					t.Fatalf("deskew %v dir %d side %d: placeOne hangs", deskew, d, s)
				}
			}
		}
	}
}
