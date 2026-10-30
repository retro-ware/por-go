package engine

import (
	"os"
	"slices"
	"testing"
	"time"
)

// The creation's rules: a grey pick forces the dependants to their first
// valid value.
func TestKindEnforce(t *testing.T) {
	k := kind{race: raceElf, sex: 0, class: clsFighterThief, align: 7}
	if k = (kind{raceElf, 0, clsPaladin, 7}).enforce(fClass); k.race != raceHuman || k.align != 0 {
		t.Errorf("elf paladin: %+v, want a lawful good human", k)
	}
	if k = (kind{raceHuman, 0, clsPaladin, 8}).enforce(fAlign); k.class != clsCleric || k.align != 8 {
		t.Errorf("chaotic evil paladin: %+v, want a cleric", k)
	}
	if k = (kind{raceDwarf, 0, clsMage, 3}).enforce(fRace); k.class != clsFighter || k.align != 3 {
		t.Errorf("dwarf magic-user: %+v, want a neutral good fighter", k)
	}
	if k = (kind{raceDwarf, 1, clsFighter, 3}).enforce(fSex); k != (kind{raceDwarf, 1, clsFighter, 3}) {
		t.Errorf("gender: %+v changed", k)
	}
	if cl := pickClasses(); slices.Contains(cl, clsDruid) || slices.Contains(cl, clsMonk) || !slices.Contains(cl, clsClericRanger) {
		t.Errorf("classes offered: %v", cl)
	}
}

// The creation sheet: Roll, Minimal, Set and Keep on the bar, the race a
// drop-down whose grey entry (a dwarf for a magic-user) forces the class,
// Keep saving the character under its default name.
func TestCreateSheet(t *testing.T) {
	if _, err := os.Stat("../data/dos/TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	e, err := New("../data")
	if err != nil {
		t.Fatal(err)
	}
	e.Cfg.SaveDir = t.TempDir()
	done := make(chan bool, 1)
	go func() { e.CreateCharacter(); done <- true }()
	wait := func() View {
		for !e.Idle() {
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(20 * time.Millisecond)
		return e.View()
	}
	act := func(a Action) View {
		e.Act(a)
		time.Sleep(20 * time.Millisecond)
		return wait()
	}
	spot := func(v View, cx, cy int) Action { // the topmost spot at a cell
		for i := len(v.Spots) - 1; i >= 0; i-- {
			if r := v.Spots[i]; cx >= r.X1 && cx <= r.X2 && cy >= r.Y1 && cy <= r.Y2 {
				return Action{Kind: ActSpot, N: i, X: cx, Y: cy}
			}
		}
		t.Fatalf("no spot at %d,%d in %v", cx, cy, v.Spots)
		return Action{}
	}
	v := wait()
	if v.Menu == nil || v.Menu.Items[0].Text != "Roll" || len(v.Menu.Items) != 4 || v.Menu.Items[3].Text != "Keep" {
		t.Fatalf("menu %+v", v.Menu)
	}
	c := e.selected()
	if c.Name() != "No-One" || c.Race != raceHuman || c.Class != clsFighter || c.Stats[0] == 0 || c.MaxHP == 0 {
		t.Fatalf("sample: %s %d %d stats %v hp %d", c.Name(), c.Race, c.Class, c.Stats, c.MaxHP)
	}
	// Class: the drop-down (14 entries, opened from row 2 to fit above the
	// repeated row 16); Magic-User is the fifth entry (row 6).
	v = act(spot(v, 1, 5))
	if v.Menu == nil || v.Menu.Items[0].Text != "Exit" {
		t.Fatalf("class list: menu %+v", v.Menu)
	}
	v = act(spot(v, 1, 6))
	if c = e.selected(); c.Class != clsMage {
		t.Fatalf("class %d, want a magic-user", c.Class)
	}
	// Race: Dwarf (row 3, grey for a magic-user) forces the class to fighter.
	v = act(spot(v, 6, 3))
	v = act(spot(v, 1, 3))
	if c = e.selected(); c.Race != raceDwarf || c.Class != clsFighter || c.IconSize != 1 {
		t.Fatalf("race %d class %d size %d, want a dwarf fighter", c.Race, c.Class, c.IconSize)
	}
	if v.TallRow != 16 || len(v.Overlays) != 0 {
		t.Errorf("tall row %d overlays %d", v.TallRow, len(v.Overlays))
	}
	// The head picker: a grid; the second head is at column 13.
	v = act(spot(v, 30, 2))
	if v.Menu == nil || v.Menu.Prompt != "Head:" {
		t.Fatalf("head picker: menu %+v", v.Menu)
	}
	v = act(spot(v, 13, 2))
	if c.PortHead != 2 {
		t.Errorf("head %d, want 2", c.PortHead)
	}
	e.Act(Action{Kind: ActPick, N: 3}) // Keep
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Keep did not end the creation")
	}
	var d Character
	if !e.LoadCharacter("No-One", &d, false) || d.Race != raceDwarf || d.PortHead != 2 {
		t.Errorf("saved: %v race %d head %d", d.Name(), d.Race, d.PortHead)
	}
}
