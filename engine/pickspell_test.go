package engine

import "testing"

// TestPickSpell: pickSpell lists the member's memorized spells once each,
// a wrong ActCast is ignored, the right one returns its ID, ActClose 0.
func TestPickSpell(t *testing.T) {
	e, err := New("../data")
	if err != nil {
		t.Skip(err)
	}
	e.items = e.readItems()
	if !e.LoadSlotNamed("in-Slums") {
		t.Skip("no in-Slums save")
	}
	c := e.selected()
	c.Memorized[0], c.Memorized[1] = spCureLightWounds, spCureLightWounds
	for _, a := range []Action{{Kind: ActCast, N: 99}, {Kind: ActCast, N: spCureLightWounds}, {Kind: ActClose}} {
		e.Act(a)
	}
	if got := e.pickSpell(); got != spCureLightWounds {
		t.Errorf("picked %d, want %d", got, spCureLightWounds)
	}
	if got := e.pickSpell(); got != 0 {
		t.Errorf("close picked %d", got)
	}
}
