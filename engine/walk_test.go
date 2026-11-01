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
	e.Start(e.Play)
	keys := func(as ...engine.Action) engine.View {
		for _, a := range as {
			keyWait(e, a)
		}
		return e.View()
	}
	v := settle(e)
	x, y := v.X, v.Y // the start of Phlan: a corridor running west
	keys(engine.Action{Kind: engine.ActMap}, engine.Action{Kind: engine.ActPoint, X: x - 3, Y: y})
	e.Apply(resolve(e, hot('W')))
	for range 3 {
		time.Sleep(30 * time.Millisecond)
		e.Apply(engine.Action{Kind: engine.ActHover, N: -1, X: 5, Y: 5})
	}
	if v = settle(e, 3*time.Second); v.X != x-3 || v.Y != y {
		t.Fatalf("walk from %d,%d to %d,%d ended at %d,%d", x, y, x-3, y, v.X, v.Y)
	}
	keys(engine.Action{Kind: engine.ActMap}, engine.Action{Kind: engine.ActPoint, X: x, Y: y})
	e.Apply(resolve(e, hot('W')), engine.Action{Kind: engine.ActOK}) // a key stops the walk back at once
	if v = settle(e); v.X != x-3 || v.Y != y {
		t.Fatalf("a key did not stop the walk back: at %d,%d, want %d,%d", v.X, v.Y, x-3, y)
	}
}
