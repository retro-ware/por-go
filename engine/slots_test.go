package engine

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"
)

func TestSlotName(t *testing.T) {
	for in, want := range map[string]string{"Before Boss": "Before Boss", "../x/y": "xy", " a:B ": "aB", "": ""} {
		if got := slotName(in); got != want {
			t.Errorf("slotName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAdoptLettered(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"savgamB.dat", "CHRDATB1.SAV", "CHRDATB1.ITM", "CHRDATA1.SAV", "ROSTER.SAV"} {
		os.WriteFile(filepath.Join(dir, f), nil, 0o644)
	}
	e := &Engine{}
	e.Cfg.SaveDir = dir
	if got := e.slots(); len(got) != 1 || got[0].name != "GAME B" {
		t.Fatalf("slots = %v, want GAME B", got)
	}
	for _, f := range []string{"saves/GAME B/SAVGAMA.DAT", "saves/GAME B/CHRDATB1.SAV", "saves/GAME B/CHRDATB1.ITM", "savgamB.dat", "CHRDATB1.SAV", "CHRDATA1.SAV"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s missing", f)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "saves/GAME B/CHRDATA1.SAV")); err == nil {
		t.Error("another letter's party was copied")
	}
	os.RemoveAll(filepath.Join(dir, "saves/GAME B"))
	if got := e.slots(); len(got) != 0 {
		t.Errorf("a deleted slot came back: %v", got)
	}
}

// An F9 save leaves the area block as it was: saveSlot packs the wall slots
// into area+0x3F4.., and area+0x3FE is the view animation flag.
func TestSaveToKeepsArea(t *testing.T) {
	e := &Engine{ecl: make([]byte, 0x1E00), set: 1}
	e.Cfg.SaveDir = t.TempDir()
	e.mapPairB = [3]int{5, 6, 7}
	e.saveTo("X")
	if w := e.area.WallsB[2]; w != 0 {
		t.Errorf("area+0x3FE = %d after a save, want 0", w)
	}
	if _, err := os.Stat(filepath.Join(e.slotRoot(), "X", "SAVGAMA.DAT")); err != nil {
		t.Error("no save written")
	}
	if l := e.slots(); len(l) != 1 || time.Since(l[0].at) > time.Minute {
		t.Errorf("slots = %v, want X at the save's time", l)
	}
}

// The named words sit at the offsets the listings use.
func TestVarOffsets(t *testing.T) {
	var a areaVars
	var g gameVars
	for _, c := range []struct{ got, want uintptr }{
		{unsafe.Offsetof(a.MapX), 0x186}, {unsafe.Offsetof(a.Clock), 0x18C}, {unsafe.Offsetof(a.NoMagic), 0x1CA},
		{unsafe.Offsetof(a.PrevX), 0x1E0}, {unsafe.Offsetof(a.NoCoords), 0x1F6}, {unsafe.Offsetof(a.MarkFlags4), 0x2F8},
		{unsafe.Offsetof(a.Mark3), 0x318}, {unsafe.Offsetof(a.Mark4Off), 0x33E}, {unsafe.Offsetof(a.RiverOpen), 0x344},
		{unsafe.Offsetof(a.Boss), 0x366}, {unsafe.Offsetof(a.Mark4), 0x3A4}, {unsafe.Offsetof(a.WallsA), 0x3F4},
		{unsafe.Offsetof(a.WallsB), 0x3FA}, {unsafe.Offsetof(a.V4B00), 0x400},
		{unsafe.Offsetof(g.PartyExp), 0x550}, {unsafe.Offsetof(g.MaxDist), 0x580}, {unsafe.Offsetof(g.Morale), 0x58C},
		{unsafe.Offsetof(g.RestEvery), 0x5A4}, {unsafe.Offsetof(g.LeftMap), 0x5AA}, {unsafe.Offsetof(g.Portrait), 0x5C2},
		{unsafe.Offsetof(g.Duel), 0x5CC}, {unsafe.Offsetof(g.SymbolSet), 0x624}, {unsafe.Offsetof(g.AttackedAlly), 0x666},
		{unsafe.Offsetof(g.PartyCount), 0x67C}, {unsafe.Offsetof(g.Shop), 0x6D8}, {unsafe.Offsetof(g.MonsterHit), 0x6E0},
		{unsafe.Offsetof(g.PartyMove), 0x6E4},
	} {
		if c.got != c.want {
			t.Errorf("field at %#x, want %#x", c.got, c.want)
		}
	}
}

// The explored squares go with the slot: a load brings back the save's, not
// what was explored since.
func TestSeenFollowsSlot(t *testing.T) {
	e := &Engine{set: 1}
	e.Cfg.SaveDir = t.TempDir()
	e.ext.seen = map[int]*[256]uint8{3: {5: seenVisited}}
	e.saveTo("X")
	b, err := os.ReadFile(filepath.Join(e.slotRoot(), "X", seenFile))
	if err != nil || len(b) == 0 {
		t.Fatal("no explored squares saved")
	}
	e.ext.seen = map[int]*[256]uint8{3: {5: seenVisited, 6: seenSeen}, 9: {}}
	f, _ := os.Open(filepath.Join(e.slotRoot(), "X", seenFile))
	e.ext.seen = nil
	gob.NewDecoder(f).Decode(&e.ext.seen)
	f.Close()
	if len(e.ext.seen) != 1 || e.ext.seen[3][5] != seenVisited || e.ext.seen[3][6] != 0 {
		t.Errorf("seen after reload = %v", e.ext.seen)
	}
}
