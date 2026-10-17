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
