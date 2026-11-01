package engine

import "testing"

// The training hall (opProgram 0) puts the party menu up; Train shows the
// hall's offer, and Begin goes back out where the party stood.
func TestHallTrain(t *testing.T) {
	e := testEngine(t)
	seed := uint32(7)
	e.FixedSeed = &seed
	e.Explore = true
	e.Init()
	e.Start(e.Play)
	x, y, n := e.posX, e.posY, len(e.screens)
	e.game.PartyExp = 0xFF
	e.after(func() { e.PartyMenu(nil) }, e.DrawMainScreen)
	if v := e.Apply(); v.MainMenu == nil || !v.Hall || v.Logo != nil || v.Backdrop != nil {
		t.Fatalf("hall's menu: menu %v hall %v, title art %v", v.MainMenu != nil, v.Hall, v.Logo != nil || v.Backdrop != nil)
	}
	if v := e.Apply(Action{Kind: ActTraining, N: 0}); v.Training == nil || v.Adventure {
		t.Fatalf("Train: training %v, adventure %v", v.Training != nil, v.Adventure)
	}
	e.Apply(Action{Kind: ActClose})
	v := e.Apply(Action{Kind: ActBegin})
	if v.Hall || e.ext.hall {
		t.Error("still the hall's menu after Begin")
	}
	if len(e.screens) != n || e.posX != x || e.posY != y || !v.Adventure || !v.Exploring {
		t.Errorf("after Begin: %d screens (was %d), at %d,%d (was %d,%d), adventure %v", len(e.screens), n, e.posX, e.posY, x, y, v.Adventure)
	}
}

// The engine draws nothing of the original's screens: what it publishes
// as pictures is art, never a fill or a frame.
func TestPicturesAreArt(t *testing.T) {
	e := testEngine(t)
	seed := uint32(7)
	e.FixedSeed = &seed
	e.Explore = true
	e.Init()
	e.Start(e.Play)
	for _, a := range []Action{{}, {Kind: ActCamp}, {Kind: ActClose}, {Kind: ActMember, N: 0}, {Kind: ActClose}} {
		v := e.Apply(a)
		for _, p := range v.Pictures {
			if p.Fill || p.Img == nil {
				t.Fatalf("after %v: a fill or empty picture at %v", a.Kind, p.At)
			}
		}
	}
}
