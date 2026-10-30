package engine

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
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
// a stat typed where it stands, Keep saving the character under its
// default name.
func TestCreateSheet(t *testing.T) {
	if _, err := os.Stat("../data/dos/TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	e, err := New("../data")
	if err != nil {
		t.Fatal(err)
	}
	e.Cfg.SaveDir = t.TempDir()
	e.viewBit0 = 1 // the sheet's portrait, as init sets it
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
	// Class: the drop-down's box from the class row, the fighter first and
	// the other classes below in their order, the sheet still there.
	v = act(spot(v, 1, 5))
	if v.Menu == nil || v.Menu.Items[0].Text != "Exit" {
		t.Fatalf("class list: menu %+v", v.Menu)
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
	if len(v.Overlays) != 0 {
		t.Errorf("overlays %d", len(v.Overlays))
	}
	// Charisma: typed where it stands.
	v = act(spot(v, 3, 12))
	if v.Typing == nil || v.Typing.Max != 5 {
		t.Fatalf("charisma: typing %+v", v.Typing)
	}
	act(Action{Kind: ActText, Text: "9"})
	v = act(Action{Kind: ActContinue})
	if c.Stats[statCha] != 9 || v.Typing != nil {
		t.Errorf("charisma %d, want 9; typing %+v", c.Stats[statCha], v.Typing)
	}
	// The head picker: no bar, a grid centred in the whole page (three
	// columns of 11 with a blank between, from column 2); the second head
	// is at column 14. 14 heads, 12 a page on 25 rows: the pager line on
	// row 23 (the box's last row, over the border on the prompt row), its
	// ">" turns the page and a click on the second card there takes head
	// 14; "<" turns it back. A window of 30 rows holds them all: the cell
	// of the "<" is the second card of the fifth row there.
	v = act(spot(v, 30, 2))
	if v.Menu != nil {
		t.Fatalf("head picker: menu %+v", v.Menu)
	}
	v = act(spot(v, 25, 23))
	v = act(spot(v, 14, 2))
	if c.PortHead != 14 || v.Menu == nil {
		t.Fatalf("head %d, want 14 (page 2); menu %+v", c.PortHead, v.Menu)
	}
	v = act(spot(v, 30, 2))
	v = act(spot(v, 14, 23))
	v = act(Action{Kind: ActResize, X: 40, Y: 30})
	if len(v.Text) != 30 || len(v.Text[0]) != 40 || v.Menu != nil {
		t.Fatalf("30 rows: %dx%d, menu %+v", len(v.Text[0]), len(v.Text), v.Menu)
	}
	c.PortHead = 0
	v = act(spot(v, 14, 23))
	if c.PortHead != 14 || v.Menu == nil {
		t.Fatalf("30 rows: head %d, want 14 (no pager); menu %+v", c.PortHead, v.Menu)
	}
	v = act(spot(v, 30, 2))
	v = act(Action{Kind: ActResize, X: 40, Y: 25})
	if len(v.Text) != 25 || v.Menu != nil {
		t.Fatalf("25 rows again: %d, menu %+v", len(v.Text), v.Menu)
	}
	v = act(spot(v, 14, 23)) // page 2 (head 14 framed): back to page 1
	v = act(spot(v, 14, 2))
	if c.PortHead != 2 {
		t.Errorf("head %d, want 2", c.PortHead)
	}
	// Dropped PNGs: one on the icon's cells (IconBox) is the combat icon,
	// one on the portrait box the portrait, as in the editor and on the
	// sheet in play; the sheet shows both at once.
	dir := t.TempDir()
	write := func(name string, w, h int) string {
		p := filepath.Join(dir, name)
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, image.NewRGBA(image.Rect(0, 0, w, h)))
		f.Close()
		return p
	}
	if v.IconBox == (CellRect{}) || v.Portrait == (CellRect{}) || v.IconDrop {
		t.Fatalf("drop targets: icon %+v portrait %+v whole screen %v", v.IconBox, v.Portrait, v.IconDrop)
	}
	act(Action{Kind: ActIcon, Text: write("icon.png", 24, 24)})
	v = act(Action{Kind: ActPortrait, N: -1, Text: write("face.png", 64, 64)})
	if c.Icon == nil || c.Portrait == nil || len(v.Overlays) != 3 {
		t.Errorf("dropped: icon %v portrait %v overlays %d, want both and 3", c.Icon != nil, c.Portrait != nil, len(v.Overlays))
	}
	e.Act(Action{Kind: ActPick, N: 2}) // Keep
	ended := func(what string) {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal(what + " did not end")
		}
	}
	ended("Keep")
	var d Character
	if !e.LoadCharacter("No-One", &d, false) || d.Race != raceDwarf || d.PortHead != 2 || d.Icon == nil || d.Portrait == nil {
		t.Errorf("saved: %v race %d head %d icon %v portrait %v", d.Name(), d.Race, d.PortHead, d.Icon != nil, d.Portrait != nil)
	}
	// Add: the saved characters as cards (the portrait PNG an overlay), no
	// bar: Enter takes the one framed; Esc leaves.
	go func() { e.AddCharacter(); done <- true }()
	v = wait()
	if v.Menu != nil || len(v.Overlays) != 1 {
		t.Fatalf("add: menu %+v overlays %d", v.Menu, len(v.Overlays))
	}
	act(Action{Kind: ActContinue})
	if len(e.party) != 1 || e.party[0].Name() != "No-One" {
		t.Fatalf("added: party %d", len(e.party))
	}
	e.Act(Action{Kind: ActClose})
	ended("Add")
	// Modify: the creation sheet on the member, Keep keeps the changes.
	go func() { e.ModifyCharacter(); done <- true }()
	v = wait()
	if v.Menu == nil || v.Menu.Items[0].Text != "Roll" || v.IconBox == (CellRect{}) {
		t.Fatalf("modify: menu %+v icon %+v", v.Menu, v.IconBox)
	}
	v = act(spot(v, 3, 12)) // charisma typed
	act(Action{Kind: ActText, Text: "7"})
	act(Action{Kind: ActContinue})
	e.Act(Action{Kind: ActPick, N: 2}) // Keep
	ended("Modify")
	if m := e.party[0]; m.Stats[statCha] != 7 || m.Share != 1 {
		t.Errorf("modified: charisma %d share %d", m.Stats[statCha], m.Share)
	}
}
