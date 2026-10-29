package engine

import "testing"

// The scripts' "hour < 14" sky check follows day 6:00-20:00 instead.
func TestSkyHour(t *testing.T) {
	e := &Engine{ecl: make([]byte, 0x10000)}
	// Compare [49C9] 14
	e.script = &Script{Code: []Instr{{Op: 0x03, Args: []Arg{{Type: 1, N: 0x49C9}, {Type: 0, N: 14}}}}}
	for _, c := range []struct {
		hour uint16
		day  bool
	}{{1, false}, {6, true}, {13, true}, {15, true}, {19, true}, {20, false}} {
		e.area.Clock[3] = c.hour
		e.eclPC, e.cur, e.argPos = 0, 0, 0
		e.opCompare()
		if lt := e.es.flags[2]; lt != c.day {
			t.Errorf("hour %d: day sky %v, want %v", c.hour, lt, c.day)
		}
	}
}

// The planned spells c does not hold are set to be memorized on the next
// rest, as far as the free slots go; what c holds stays.
func TestPlanSpells(t *testing.T) {
	e := &Engine{}
	c := &Character{}
	c.SpellSlots[0] = 3 // three 1st-level cleric spells
	c.Plan = [21]byte{spCureLightWounds, spCureLightWounds, spCureLightWounds}
	c.Memorized = [21]byte{spBless, spCureLightWounds | 0x80}
	e.planSpells(c)
	want := [21]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, spBless, spCureLightWounds | 0x80, spCureLightWounds | 0x80}
	if c.Memorized != want {
		t.Fatalf("got %x, want %x", c.Memorized, want)
	}
}
