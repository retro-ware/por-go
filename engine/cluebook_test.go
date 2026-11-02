package engine_test

import (
	"testing"

	"por/engine"
)

// TestClueBookResumes: B opens the clue book again on the page and the
// picture scroll it was closed on; Esc there gives the chapter list.
func TestClueBookResumes(t *testing.T) {
	e := explore(t)
	e.Init()
	e.Start(e.Play)
	settle(e)
	key := func(a engine.Action) engine.View { keyWait(e, a); return settle(e) }
	key(engine.Action{Kind: engine.ActClueBook})
	v := key(engine.Action{Kind: engine.ActChoose, N: -1, X: 5})
	for len(v.Pages.Scans) == 0 || v.Pages.Hint.Text[0] != 'u' { // on to a page whose picture scrolls
		v = key(engine.Action{Kind: engine.ActPage, N: 1})
	}
	v = key(engine.Action{Kind: engine.ActPage, N: 1})
	page, src := v.Pages.Page, v.Pages.Scans[0].Src
	key(engine.Action{Kind: engine.ActClose})
	if v = key(engine.Action{Kind: engine.ActClose}); v.Pages != nil || v.Ask != nil {
		t.Fatal("the clue book did not close")
	}
	v = key(engine.Action{Kind: engine.ActClueBook})
	if v.Pages == nil || v.Pages.Page != page || v.Pages.Scans[0].Src != src {
		t.Fatalf("reopened: %+v, want page %d at %v", v.Pages, page, src)
	}
	if v = key(engine.Action{Kind: engine.ActClose}); v.Pages != nil || v.Ask == nil {
		t.Fatal("Esc: no chapter list")
	}
}
