package engine_test

import (
	"testing"
	"time"

	"por/engine"
)

// TestAutoWalk: Walk on the full map takes the party to the cursor square, and
// the pointer moving on meanwhile (the map closing under it is a hover too)
// does not stop it; a key does.
func TestAutoWalk(t *testing.T) {
	e := explore(t)
	e.Init()
	go e.Play()
	keys := func(as ...engine.Action) engine.View {
		for _, a := range as {
			for !e.Idle() {
				time.Sleep(10 * time.Millisecond)
			}
			e.Act(resolve(e, a))
			time.Sleep(50 * time.Millisecond)
		}
		for !e.Idle() {
			time.Sleep(10 * time.Millisecond)
		}
		return e.View()
	}
	at := func(x, y int) bool {
		for t0 := time.Now(); time.Since(t0) < 3*time.Second; time.Sleep(20 * time.Millisecond) {
			if v := e.View(); v.X == x && v.Y == y {
				return true
			}
		}
		return false
	}
	v := keys()
	x, y := v.X, v.Y // the start of Phlan: a corridor running west
	left := engine.Action{Kind: engine.ActMove, N: engine.DirLeft}
	keys(engine.Action{Kind: engine.ActMap}, left, left, left, hot('W'))
	for range 3 {
		e.Act(engine.Action{Kind: engine.ActHover, N: -1, X: 5, Y: 5})
		time.Sleep(30 * time.Millisecond)
	}
	if !at(x-3, y) {
		v = e.View()
		t.Fatalf("walk from %d,%d to %d,%d ended at %d,%d", x, y, x-3, y, v.X, v.Y)
	}
	right := engine.Action{Kind: engine.ActMove, N: engine.DirRight}
	keys(engine.Action{Kind: engine.ActMap}, right, right, right)
	e.Act(resolve(e, hot('W')))
	e.Act(engine.Action{Kind: engine.ActContinue}) // a key stops the walk back at once
	time.Sleep(time.Second)
	if v = e.View(); v.X != x-3 || v.Y != y {
		t.Fatalf("a key did not stop the walk back: at %d,%d, want %d,%d", v.X, v.Y, x-3, y)
	}
}
