package engine

import "testing"

// A party lost on the first square after an area change: the area block
// still names the old area while the position is the new area's, so the
// next party's Begin must not inherit either (it loaded New Phlan at the
// slums' entry square, in the water).
func TestResetAdventure(t *testing.T) {
	e := &Engine{}
	e.gs.areaKind, e.area.AreaKind = 1, 0
	e.posX, e.posY, e.facing = 3, 9, 2
	e.mapPairA[1], e.purse.gold, e.game.PartyCount = 5, 20, 6
	e.resetAdventure()
	if e.gs.areaKind != 0 || e.area.AreaKind != 0 || e.posX != 15 || e.posY != 1 || e.facing != 6 {
		t.Errorf("area %d/%d at %d,%d facing %d, want New Phlan at 15,1 facing west", e.gs.areaKind, e.area.AreaKind, e.posX, e.posY, e.facing)
	}
	if e.mapPairA != [3]int{0, -1, -1} || e.purse.gold != 0 || e.game.PartyCount != 0 {
		t.Errorf("pairs %v gold %d count %d", e.mapPairA, e.purse.gold, e.game.PartyCount)
	}
}
