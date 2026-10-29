package engine

import "testing"

// A card dragged onto another's place moves the member there, while the
// Party Order bar is up and only then.
func TestOrderDrag(t *testing.T) {
	e := &Engine{}
	a, b, c := &Character{}, &Character{}, &Character{}
	e.party = []*Character{a, b, c}
	if e.extraAct(Action{Kind: ActOrder, N: 0, X: 2}) || e.party[0] != a {
		t.Fatal("moved without the bar")
	}
	e.ext.ordering = true
	if !e.extraAct(Action{Kind: ActOrder, N: 0, X: 2}) || e.party[0] != b || e.party[1] != c || e.party[2] != a || e.partySel != 2 {
		t.Fatalf("first to last: %v", e.party)
	}
	if e.extraAct(Action{Kind: ActOrder, N: 1, X: 3}) {
		t.Fatal("moved out of the party")
	}
}
