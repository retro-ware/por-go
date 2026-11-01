package engine

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
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

// The creation sheet takes decisions: a race that does not fit the class
// forces it, a stat typed, the head picker paged, dropped PNGs, Keep
// saving the character under its default name; then Add, Modify, Forget,
// the icon editor and the party menu.
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
	act := func(a Action) View {
		if a = e.take(a); a.Kind != ActNone {
			e.feed(a, 0)
		}
		e.publish()
		return e.View()
	}
	view := func() View { e.publish(); return e.View() }
	option := func(v View, f int, text string) int {
		i := slices.IndexFunc(v.Creation.Fields[f].Options, func(o Option) bool { return o.Text == text })
		if i < 0 {
			t.Fatalf("no %q in field %d", text, f)
		}
		return i
	}
	e.CreateCharacter()
	v := view()
	if v.Creation == nil || v.Ask != nil || v.Creation.Fields[FieldClass].At.Y1 != 5 {
		t.Fatalf("creation %+v ask %+v", v.Creation, v.Ask)
	}
	c := e.selected()
	if c.Name() != "No-One" || c.Race != raceHuman || c.Class != clsFighter || c.Stats[0] == 0 || c.MaxHP == 0 {
		t.Fatalf("sample: %s %d %d stats %v hp %d", c.Name(), c.Race, c.Class, c.Stats, c.MaxHP)
	}
	v = act(Action{Kind: ActSetField, N: FieldClass, X: option(v, FieldClass, classNames[clsMage])})
	if c = e.selected(); c.Class != clsMage {
		t.Fatalf("class %d, want a magic-user", c.Class)
	}
	// Dwarf does not fit a magic-user: it forces the class to fighter.
	dw := option(v, FieldRace, raceNames[raceDwarf])
	if v.Creation.Fields[FieldRace].Options[dw].Fits {
		t.Error("a dwarf magic-user fits")
	}
	v = act(Action{Kind: ActSetField, N: FieldRace, X: dw})
	if c = e.selected(); c.Race != raceDwarf || c.Class != clsFighter || c.IconSize != 1 {
		t.Fatalf("race %d class %d size %d, want a dwarf fighter", c.Race, c.Class, c.IconSize)
	}
	act(Action{Kind: ActStat, N: statCha, Text: "9"})
	if c.Stats[statCha] != 9 {
		t.Errorf("charisma %d, want 9", c.Stats[statCha])
	}
	act(Action{Kind: ActName, Text: "Brok"})
	act(Action{Kind: ActRoll, N: 1, Text: "3 3 3 3 3 3"})
	if c.Name() != "Brok" || !e.meetsMinStats(c) {
		t.Errorf("name %q, minimums %v", c.Name(), e.ext.minStats)
	}
	// The head picker: 14 heads, 12 a page on 25 rows (three columns of
	// 11 from column 2), the pager's arrows on row 23; ActPage turns it.
	v = act(Action{Kind: ActPicker, N: PickHead})
	if p := v.Picker; p == nil || v.Creation != nil || p.N != 14 || p.Pages != 2 || p.PerPage != 12 || p.Cells[1].X1 != 14 || p.Next.Y1 != 23 {
		t.Fatalf("head picker %+v", v.Picker)
	}
	if v = act(Action{Kind: ActPage, N: 1}); len(v.Picker.Cells) != 2 {
		t.Fatalf("page 2: %+v", v.Picker)
	}
	if v = act(Action{Kind: ActPicked, N: 13}); c.PortHead != 14 || v.Creation == nil {
		t.Fatalf("head %d, want 14; creation %v", c.PortHead, v.Creation != nil)
	}
	e.resizePage(40, 30) // a window of 30 rows holds them all
	if v = act(Action{Kind: ActPicker, N: PickHead}); v.Picker.Pages != 1 || v.Cells.Y != 30 {
		t.Fatalf("30 rows: %+v", v.Picker)
	}
	act(Action{Kind: ActPicked, N: 1})
	e.resizePage(40, 25)
	if c.PortHead != 2 {
		t.Errorf("head %d, want 2", c.PortHead)
	}
	// The icon editor: Size, then Exit puts the old icon back.
	if v = act(Action{Kind: ActIconEdit, N: -1}); !v.IconDrop {
		t.Fatal("no icon editor")
	}
	act(Action{Kind: ActIconSize})
	if v = act(Action{Kind: ActClose}); c.IconSize != 1 || v.Creation == nil {
		t.Fatalf("icon size %d, creation %v", c.IconSize, v.Creation != nil)
	}
	// Dropped PNGs: one on the icon's cells (IconBox) is the combat icon,
	// one on the portrait box the portrait; the sheet shows both at once.
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
	if v = act(Action{Kind: ActKeep}); v.Creation != nil || len(e.screens) != 0 {
		t.Fatalf("after Keep: creation %v screens %d", v.Creation != nil, len(e.screens))
	}
	var d Character
	if !e.LoadCharacter("Brok", &d, false) || d.Race != raceDwarf || d.PortHead != 2 || d.Icon == nil || d.Portrait == nil {
		t.Errorf("saved: %v race %d head %d icon %v portrait %v", d.Name(), d.Race, d.PortHead, d.Icon != nil, d.Portrait != nil)
	}
	// Add: the saved characters as cards (the portrait PNG an overlay);
	// a pick adds one, the picker stays until ActClose.
	e.AddCharacter()
	if v = view(); v.Picker == nil || v.Picker.N != 1 || len(v.Overlays) != 1 {
		t.Fatalf("add: picker %+v overlays %d", v.Picker, len(v.Overlays))
	}
	act(Action{Kind: ActPicked, N: 0})
	if len(e.party) != 1 || e.party[0].Name() != "Brok" || e.View().Picker.Labels[0] != "* Brok" {
		t.Fatalf("added: party %d", len(e.party))
	}
	act(Action{Kind: ActClose})
	// Modify: the creation sheet on the member, experienced or not, Keep
	// keeps the changes.
	e.party[0].Exp = 500
	e.ModifyCharacter()
	if v = view(); v.Creation == nil || v.Creation.Abandon != "Discard the changes?" {
		t.Fatalf("modify: %+v", v.Creation)
	}
	act(Action{Kind: ActStat, N: statCha, Text: "7"})
	act(Action{Kind: ActKeep})
	if m := e.party[0]; m.Stats[statCha] != 7 || m.Share != 1 {
		t.Errorf("modified: charisma %d share %d", m.Stats[statCha], m.Share)
	}
	// The party menu takes decisions: View opens the sheet (View.Sheet)
	// over it, the sheet's Exit brings the menu back, Begin starts play.
	e.push(modeParty, e.applyParty, nil)
	if v = view(); !slices.Contains(v.MainMenu, ActMember) || !slices.Contains(v.MainMenu, ActBegin) {
		t.Fatalf("party menu %v", v.MainMenu)
	}
	if v = act(Action{Kind: ActMember, N: -1}); v.Sheet == nil || v.MainMenu != nil {
		t.Fatalf("view: sheet %+v menu %v", v.Sheet, v.MainMenu)
	}
	if v = act(Action{Kind: ActClose}); v.MainMenu == nil {
		t.Fatal("back from the sheet: no party menu")
	}
	act(Action{Kind: ActBegin})
	if e.beginFlag != 4 || len(e.screens) != 0 {
		t.Errorf("begin flag %d, screens %d", e.beginFlag, len(e.screens))
	}
	// Forget: the picker asks (the presenter's question), a pick deletes.
	e.SaveCharacter(e.party[0], "") // as Remove
	e.party = nil
	e.ForgetCharacter()
	if v = view(); v.Picker == nil || v.Picker.Ask == "" {
		t.Fatalf("forget: %+v", v.Picker)
	}
	act(Action{Kind: ActPicked, N: 0})
	if len(e.BuildCharList()) != 0 || len(e.screens) != 0 {
		t.Errorf("forgotten: still there, screens %d", len(e.screens))
	}
}
