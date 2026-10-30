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

// The creation sheet: Roll, Minimal and Keep on the bar, the race a
// drop-down whose grey entry (a dwarf for a magic-user) forces the class,
// a stat a drop-down of the values allowed, Keep saving the character
// under its default name.
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
	if v.Menu == nil || v.Menu.Items[0].Text != "Roll" || len(v.Menu.Items) != 3 || v.Menu.Items[2].Text != "Keep" {
		t.Fatalf("menu %+v", v.Menu)
	}
	c := e.selected()
	if c.Name() != "No-One" || c.Race != raceHuman || c.Class != clsFighter || c.Stats[0] == 0 || c.MaxHP == 0 {
		t.Fatalf("sample: %s %d %d stats %v hp %d", c.Name(), c.Race, c.Class, c.Stats, c.MaxHP)
	}
	// Class: the drop-down from the class row, the fighter first and the
	// other classes below in their order, the sheet gone.
	v = act(spot(v, 1, 5))
	if v.Menu == nil || v.Menu.Items[0].Text != "Exit" || v.Text[1][1].Set || v.TallRow != 19 {
		t.Fatalf("class list: menu %+v name cell %+v tall row %d", v.Menu, v.Text[1][1], v.TallRow)
	}
	classes := pickClasses()
	row := 5 + 1 + slices.Index(slices.DeleteFunc(classes, func(cl byte) bool { return cl == clsFighter }), clsMage)
	v = act(spot(v, 1, row))
	if c = e.selected(); c.Class != clsMage {
		t.Fatalf("class %d, want a magic-user", c.Class)
	}
	// Race: the drop-down at the race's column, Dwarf (grey for a magic-user)
	// the first below Human; it forces the class to fighter.
	v = act(spot(v, 6, 3))
	v = act(spot(v, 6, 4))
	if c = e.selected(); c.Race != raceDwarf || c.Class != clsFighter || c.IconSize != 1 {
		t.Fatalf("race %d class %d size %d, want a dwarf fighter", c.Race, c.Class, c.IconSize)
	}
	if v.TallRow != 16 || len(v.Overlays) != 0 {
		t.Errorf("tall row %d overlays %d", v.TallRow, len(v.Overlays))
	}
	// Charisma: the drop-down from its row, the current value first, 3..18
	// below it (the pointer on the row glows); the last fitting row picks.
	h := spot(v, 3, 12)
	h.Kind = ActHover
	if v = act(h); v.Text[12][1].Bg == v.Text[7][1].Bg {
		t.Errorf("the charisma row does not glow under the pointer: %v", v.Text[12][1].Bg)
	}
	v = act(spot(v, 3, 12))
	vals, _ := statChoices(c, statCha)
	if v.Menu == nil || v.Menu.Items[len(v.Menu.Items)-1].Text != "Exit" || v.TallRow != 22 { // Next pages the rest
		t.Fatalf("charisma list: menu %+v tall row %d", v.Menu, v.TallRow)
	}
	want := slices.DeleteFunc(vals, func(b byte) bool { return b == c.Stats[statCha] })[21-12-1]
	v = act(spot(v, 5, 21))
	if c.Stats[statCha] != want {
		t.Errorf("charisma %d, want %d", c.Stats[statCha], want)
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
	e.Act(Action{Kind: ActPick, N: 2}) // Keep
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
