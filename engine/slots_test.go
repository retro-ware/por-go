package engine

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSlotName(t *testing.T) {
	for in, want := range map[string]string{"Before Boss": "Before Boss", "../x/y": "xy", " a:B ": "aB", "": ""} {
		if got := slotName(in); got != want {
			t.Errorf("slotName(%q) = %q, want %q", in, got, want)
		}
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

// A save in the first command input after an area change names the area
// the party is in, not the one it left (the area block still does then).
func TestSaveToRunningArea(t *testing.T) {
	e := &Engine{ecl: make([]byte, 0x1E00), set: 1}
	e.Cfg.SaveDir = t.TempDir()
	e.area.AreaKind, e.gs.areaKind = 20, 0
	e.saveTo("X")
	b, _ := os.ReadFile(filepath.Join(e.slotRoot(), "X", "SAVGAMA.DAT"))
	s, err := OpenSave(b)
	if err != nil || s.f.Area.AreaKind != 0 {
		t.Errorf("saved area %d (%v), want 0, the running one", s.f.Area.AreaKind, err)
	}
}

// The named words sit at the offsets the listings use, as the scripts
// reach them (word i = byte offset 2i), and each record is 0x800 bytes.
func TestVarOffsets(t *testing.T) {
	var a areaVars
	var g gameVars
	for off, f := range map[int]*uint16{
		0x186: &a.MapX, 0x18C: &a.Clock[0], 0x1CA: &a.NoMagic, 0x1E0: &a.PrevX, 0x1F6: &a.NoCoords,
		0x2F8: &a.MarkFlags4, 0x318: &a.Mark3, 0x33E: &a.Mark4Off, 0x344: &a.RiverOpen, 0x366: &a.Boss,
		0x3A4: &a.Mark4, 0x3F4: &a.WallsA[0], 0x3FA: &a.WallsB[0], 0x400: &a.V4B00[0],
	} {
		if a.word(off/2) != f {
			t.Errorf("area word at %#x is not the named field", off)
		}
	}
	for off, f := range map[int]*uint16{
		0x550: &g.PartyExp, 0x580: &g.MaxDist, 0x58C: &g.Morale, 0x5A4: &g.RestEvery, 0x5AA: &g.LeftMap,
		0x5C2: &g.Portrait, 0x5CC: &g.Duel, 0x624: &g.SymbolSet, 0x666: &g.AttackedAlly,
		0x67C: &g.PartyCount, 0x6D8: &g.Shop, 0x6E0: &g.MonsterHit, 0x6E4: &g.PartyMove,
	} {
		if g.word(off/2) != f {
			t.Errorf("game word at %#x is not the named field", off)
		}
	}
	if n, m, k := len(recordBytes(&a)), len(recordBytes(&g)), len(recordBytes(&auxVars{})); n != 0x800 || m != 0x800 || k != 0x400 {
		t.Errorf("record sizes %#x %#x %#x, want 0x800 0x800 0x400", n, m, k)
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

// A slot saved with the explored squares by script area loads them by map:
// New Phlan's district (area 0) and Training Hall (11) pool on set 3's map 0.
func TestSeenByMap(t *testing.T) {
	e := &Engine{set: 1, data: "../data"}
	seen := e.seenByMap(map[int]*[256]uint8{0: {5: seenVisited, 6: seenSeen}, 11 << 16: {6: seenVisited, 7: seenSeen}, 20<<16 | 20: {1: seenVisited}})
	want := map[int]*[256]uint8{3 << 16: {5: seenVisited, 6: seenVisited, 7: seenSeen}, 2<<16 | 20: {1: seenVisited}}
	if len(seen) != len(want) {
		t.Fatalf("%d maps, want %d", len(seen), len(want))
	}
	for k, m := range want {
		if got := seen[k]; got == nil || *got != *m {
			t.Errorf("map %#x = %v, want %v", k, got, m)
		}
	}
}

// New Phlan (set 3, GEO 0) always has its built-in labels: MAPNOTES.TXT's
// old "area 0" (script area 0) loads as its map and goes over them, built-in
// copies are not written back, and removing the player's label shows the
// built-in one again.
func TestPhlanLabels(t *testing.T) {
	e := &Engine{set: 3, data: "../data"}
	e.Cfg.SaveDir = t.TempDir()
	if m := e.labels(); m[1<<4|15].text != "Boats" || len(m) != len(phlanLabels) {
		t.Fatalf("fresh New Phlan: %d labels, 15,1 = %q", len(m), m[1<<4|15].text)
	}
	os.WriteFile(filepath.Join(e.Cfg.SaveDir, notesFile), []byte("area 0\n15 1 4 My boat\n11 1 4 Harbor master\n"), 0o644)
	e.loadNotes()
	if m := e.labels(); m[1<<4|15].text != "My boat" || m[4<<4|0].text != "Gate to slums" {
		t.Fatalf("old notes' New Phlan 15,1 = %q, 0,4 = %q", m[1<<4|15].text, m[4<<4|0].text)
	}
	e.saveNotes()
	if b, _ := os.ReadFile(filepath.Join(e.Cfg.SaveDir, notesFile)); string(b) != "map 196608\n15 1 4 My boat\n" {
		t.Errorf("saved notes = %q", b)
	}
	delete(e.userLabels(), 1<<4|15)
	if l := e.labels()[1<<4|15]; l.text != "Boats" {
		t.Errorf("removed 15,1 = %q, want Boats", l.text)
	}
}
