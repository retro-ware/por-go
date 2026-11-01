package engine

import "testing"

// Camp shows the camp picture (PIC 0x1D) in the view window and its fire
// burns: frames go by on the ticks while camp is up, and stop once it is gone.
func TestCampfireAnimates(t *testing.T) {
	e := quickStarted(t)
	e.CampMenu(nil)
	if !e.animating() || len(e.pic.frames) < 2 {
		t.Fatalf("camp: animating %v, %d frames", e.animating(), len(e.pic.frames))
	}
	first := e.pic.cur
	for i := 0; i < 200 && e.pic.cur == first; i++ {
		e.pic.last -= 1000 // the frame's delay gone by
		e.Apply(Action{Kind: ActTick})
	}
	if e.pic.cur == first {
		t.Fatal("the campfire does not change frames")
	}
	if v := e.View(); v.Window != WinPicture || !v.Ticks {
		t.Fatalf("camp view: window %v, ticks %v", v.Window, v.Ticks)
	}
	e.Apply(Action{Kind: ActClose})
	if e.animInput {
		t.Fatal("the picture still animates after camp")
	}
}
