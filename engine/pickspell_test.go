package engine

import "testing"

// TestPickSpell: pickSpell lists the member's memorized spells once each,
// a wrong ActCast is ignored, the right one hands on its ID, ActClose 0.
func TestPickSpell(t *testing.T) {
	e := quickStarted(t)
	c := e.selected()
	c.Memorized[0], c.Memorized[1] = spCureLightWounds, spCureLightWounds
	got := byte(0xFF)
	e.pickSpell(func(id byte) { got = id })
	if n := len(e.snap.spellPick); n != 1 {
		t.Errorf("%d spells offered, want 1", n)
	}
	e.feed(Action{Kind: ActCast, N: 99}, 0)
	if got != 0xFF {
		t.Errorf("a wrong spell picked %d", got)
	}
	e.feed(Action{Kind: ActCast, N: spCureLightWounds}, 0)
	if got != spCureLightWounds {
		t.Errorf("picked %d, want %d", got, spCureLightWounds)
	}
	e.pickSpell(func(id byte) { got = id })
	e.feed(Action{Kind: ActClose}, 0)
	if got != 0 {
		t.Errorf("close picked %d", got)
	}
}
