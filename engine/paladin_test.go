package engine

import "testing"

// Paladin and ranger rules ported from CotAB.
func TestPaladinRanger(t *testing.T) {
	e := &Engine{}
	pal := &Character{}
	pal.Levels[clsPaladin] = 7
	e.RecomputeCharacter(pal)
	if pal.BaseAttack[0] != 3 || pal.Saves[0] != 8 || !fighterKind(pal) {
		t.Errorf("paladin 7: attacks %d saves %v", pal.BaseAttack[0], pal.Saves)
	}
	rng := &Character{}
	rng.Levels[clsRanger] = 7
	e.RecomputeCharacter(rng)
	if rng.BaseAttack[0] != 2 {
		t.Errorf("ranger 7: attacks %d", rng.BaseAttack[0])
	}
	if xpTable[clsPaladin][9] != -1 || xpTable[clsRanger][8] != -1 || xpTable[clsRanger][7] != 90001 {
		t.Error("level caps")
	}
	if raceClasses[7][0] != 6 || raceClasses[4][0] != 13 || classAlignments[4][0] != 3 {
		t.Error("class lists")
	}
	if !levelBlocked(4, 0x11, clsRanger, 7) || levelBlocked(7, 0x11, clsRanger, 7) {
		t.Error("half-elf ranger limit")
	}
}

// A paladin saved by the port still reads as CotAB's ConvertPoolRadPlayer
// expects (ovr017): the 0x11D record at the same offsets, 9-byte affect
// records, and none of the new affects among the ones CotAB keeps.
func TestPaladinCotabTransfer(t *testing.T) {
	c := &Character{}
	c.Class, c.Alignment, c.Exp = 3, 0, 175001
	c.Levels[clsPaladin] = 8
	b := c.record()
	if len(b) != 0x11D || b[0x2F] != 3 || b[0x96+3] != 8 || b[0xA0] != 0 || b[0xAC] != 0x99 || b[0xAE] != 0x02 {
		t.Fatalf("record % x", b[0x2F:0x30])
	}
	cotabKeeps := map[affectType]bool{18: true, 26: true, 47: true, 48: true, 97: true, 107: true, 124: true} // ovr017 asc_49280
	for _, typ := range []affectType{affProtEvil, affLaidOnHands, affCuredDisease} {
		if cotabKeeps[typ] {
			t.Errorf("CotAB would import affect %#x", typ)
		}
	}
	if n := len(writeRecords([]*affectRecord{{Type: affLaidOnHands, Time: 1440}})); n != 9 {
		t.Errorf("affect record %d bytes", n)
	}
}
