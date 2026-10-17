package blobber

import "testing"

// Follow animates a step or a turn and jumps anything further; held keys never move a View.
func TestViewFollow(t *testing.T) {
	g := &Game{follow: true, t: 1}
	game = g
	v := &View{g}
	v.Show(&Scene{Rows: []string{"#####", "#...#", "#...#", "#####"}}, Camera{1, 1, 1})
	v.Follow(Camera{2, 1, 1})
	if !g.moving || g.x != 2 || g.sway != 0 {
		t.Fatalf("step east: moving %v x %d sway %v", g.moving, g.x, g.sway)
	}
	for range 120 {
		v.Update()
	}
	if g.moving || g.tx != 2.5 {
		t.Fatalf("step should end on the cell: moving %v tx %v", g.moving, g.tx)
	}
	v.Follow(Camera{2, 2, 1}) // to the party's right: a sidestep
	if g.sway != 1 {
		t.Fatalf("sidestep sway %v", g.sway)
	}
	v.Follow(Camera{3, 2, 2}) // a step and a turn at once: a jump
	if g.moving || g.x != 3 || g.dir != 2 {
		t.Fatalf("jump: moving %v at %d facing %d", g.moving, g.x, g.dir)
	}
}

// Moves that come while one is animating queue up and flow on without braking.
func TestViewFollowQueue(t *testing.T) {
	g := &Game{follow: true, smooth: true, t: 1}
	game = g
	v := &View{g}
	v.Show(&Scene{Rows: []string{"######", "#....#", "######"}}, Camera{1, 1, 1})
	v.Follow(Camera{2, 1, 1})
	v.Follow(Camera{3, 1, 1}) // the engine took the next step already
	if g.x != 2 || len(g.next) != 1 {
		t.Fatalf("second step should queue: x %d queued %d", g.x, len(g.next))
	}
	for i := 0; i < 600 && (g.moving || len(g.next) > 0); i++ {
		v.Update()
		if g.x == 3 && g.t < 0.1 && g.v < 1 {
			t.Fatalf("second step started from rest: v %v", g.v)
		}
	}
	if g.moving || g.tx != 3.5 {
		t.Fatalf("should end on 3,1: moving %v tx %v", g.moving, g.tx)
	}
}

// A held key (fluid movement) asks for the next move halfway through one;
// a move that arrives while the view waits at the cell keeps its speed,
// for turns as for steps.
func TestViewFlow(t *testing.T) {
	Settings.Fluid = true
	g := &Game{follow: true, smooth: true, t: 1}
	game = g
	v := &View{g}
	v.Show(&Scene{Rows: []string{"######", "#....#", "######"}}, Camera{1, 1, 1})
	for _, c := range []Camera{{2, 1, 1}, {2, 1, 2}} { // a step, then a turn
		v.Follow(c)
		asked := false
		for i := 0; i < 600 && !asked; i++ {
			v.Update()
			asked = v.Flow(true, true)
		}
		if !asked {
			t.Fatalf("%v: never asked for the next move", c)
		}
		for i := 0; i < 600 && g.t < 1; i++ { // the host is slow: the view reaches the cell
			v.Update()
		}
		if !g.moving || g.v == 0 {
			t.Fatalf("%v: stopped at the cell while held: moving %v v %v", c, g.moving, g.v)
		}
	}
	v.Flow(false, true)
	for i := 0; i < 600 && g.moving; i++ {
		v.Update()
	}
	if g.moving {
		t.Fatal("still moving after the key was let go")
	}
}
