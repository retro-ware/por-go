package engine

import "testing"

// The scripts' "hour < 14" sky check follows day 6:00-20:00 instead.
func TestSkyHour(t *testing.T) {
	e := &Engine{ecl: make([]byte, 0x10000)}
	// opCompare [49C9] 14
	for i, b := range []byte{0x03, 0x01, 0xC9, 0x49, 0x00, 14} {
		e.setEclByte(0x9900+i, b)
	}
	for _, c := range []struct {
		hour uint16
		day  bool
	}{{1, false}, {6, true}, {13, true}, {15, true}, {19, true}, {20, false}} {
		e.area.Clock[3] = c.hour
		e.eclPC = 0x9900
		e.opCompare()
		if lt := e.es.flags[2]; lt != c.day {
			t.Errorf("hour %d: day sky %v, want %v", c.hour, lt, c.day)
		}
	}
}

func TestRestoreSpells(t *testing.T) {
	was := [21]byte{3, 3, 1, 0x29, 0x85} // two cures, bless, a 2nd level spell, one pending
	now := [21]byte{0x29, 3, 0x83}       // one cure left, bless replaced
	want := [21]byte{3, 0x83, 0x81, 0x29, 0x85}
	if got := restoreSpells(was, now); got != want {
		t.Fatalf("got %x, want %x", got[:5], want[:5])
	}
}

// A cast spell is remembered and comes back as one to memorize again.
func TestRelearnUsedSpells(t *testing.T) {
	e := &Engine{}
	c := &Character{}
	c.Memorized[0], c.Memorized[1] = spCureLightWounds, spCureLightWounds
	e.party = []*Character{c}
	e.forgetSpell(spCureLightWounds)
	if c.Memorized[0] != 0 || c.Memorized[1] != spCureLightWounds {
		t.Fatalf("cast: %v", c.Memorized[:2])
	}
	r := restoreSpells(e.ext.learned[c], c.Memorized)
	if r[0]|r[1] != 3|0x80 || r[0]&r[1] != 3 { // one kept, one to memorize
		t.Errorf("relearn: %#x %#x, want 0x3 and 0x83", r[0], r[1])
	}
}
