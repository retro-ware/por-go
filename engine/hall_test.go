package engine

import (
	"image"
	"testing"
)

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
	e.party[0].Exp, e.purse.gold = 1<<20, 1<<20 // Train is offered only to a member who would rise
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

// The title's pictures are whole-screen pictures (View.Full), 320x200.
func TestTitleFull(t *testing.T) {
	e := testEngine(t)
	e.Init()
	e.Title()
	v := e.Apply()
	if !v.Full || len(v.Pictures) == 0 {
		t.Fatalf("title: full %v, %d pictures", v.Full, len(v.Pictures))
	}
	var box image.Rectangle
	for _, p := range v.Pictures {
		box = box.Union(p.At)
	}
	if box.Dx() != 320 || box.Dy() != 200 {
		t.Errorf("title picture %v, want 320x200", box)
	}
}

// The credits come up on their own: the title picture before them is gone.
func TestCreditsClearTitle(t *testing.T) {
	e := testEngine(t)
	e.Init()
	e.fullPicture(e.LoadPic("TITLE", 2))
	e.drawCredits()
	e.publish()
	if v := e.View(); len(v.Pictures) != 0 || v.Full || !v.Credits {
		t.Errorf("credits: %d pictures, full %v, credits %v", len(v.Pictures), v.Full, v.Credits)
	}
}

// The sheet opened while exploring is a page of its own: the world's 3D
// view (View.Adventure) does not show over it.
func TestSheetOverWorld(t *testing.T) {
	e := testEngine(t)
	seed := uint32(7)
	e.FixedSeed = &seed
	e.Explore = true
	e.Init()
	e.Start(e.Play)
	if v := e.Apply(); !v.Adventure {
		t.Fatal("not out in the world")
	}
	if v := e.Apply(Action{Kind: ActMember, N: -1}); v.Sheet == nil || v.Adventure {
		t.Fatalf("View: sheet %v, adventure %v", v.Sheet != nil, v.Adventure)
	}
	if v := e.Apply(Action{Kind: ActClose}); !v.Adventure {
		t.Error("back from the sheet: not out in the world")
	}
}
