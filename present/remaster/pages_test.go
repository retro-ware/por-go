package remaster

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"por/engine"
)

// The screens the presenter draws from their View data (the sheets, the
// pickers, the slots, the icon editor): their text and pictures, recorded
// in testdata/*.txt from the engine's own drawing before the presenter took
// it over (-update writes them anew), must come out of the presenter's
// layout (screenPage) cell for cell.

var update = flag.Bool("update", false, "record the screens' goldens")

func headless(t *testing.T, explore bool) *engine.Engine {
	t.Helper()
	if _, err := os.Stat("../../data/dos/TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	e, err := engine.New("../../data")
	if err != nil {
		t.Fatal(err)
	}
	seed := uint32(7)
	e.FixedSeed = &seed
	e.Cfg.SaveDir = t.TempDir()
	e.Explore = explore
	e.Init()
	e.Start(e.Play)
	return e
}

func settled(e *engine.Engine, as ...engine.Action) engine.View {
	v := e.View()
	for _, a := range as {
		v = e.Apply(a)
		for t0 := time.Now(); v.Ticks && time.Since(t0) < 300*time.Millisecond; {
			time.Sleep(time.Millisecond)
			v = e.Apply(engine.Action{Kind: engine.ActTick})
		}
	}
	return v
}

// partyMenu: past the title to the party menu.
func partyMenu(t *testing.T) (*engine.Engine, engine.View) {
	e := headless(t, false)
	v := settled(e)
	for i := 0; v.MainMenu == nil && i < 20; i++ {
		v = settled(e, engine.Action{Kind: engine.ActOK})
	}
	if v.MainMenu == nil {
		t.Fatal("no party menu")
	}
	return e, v
}

// dump is the screen's text and pictures, a line each: the text as runs
// (row, column, colours, upcase, text), the pictures (DAX file, block,
// frame, place; fills by colour; the border's 8x8 symbols left out) and the
// PNGs over the page (size, place).
func dump(lines [][]engine.PaneRun, pics []engine.Picture, overs []overlay) string {
	var b strings.Builder
	hex := func(c color.RGBA) string { return fmt.Sprintf("%02x%02x%02x", c.R, c.G, c.B) }
	for y, l := range lines {
		for _, r := range l {
			if strings.TrimSpace(r.Text) == "" && r.Bg == (color.RGBA{A: 255}) {
				continue
			}
			fmt.Fprintf(&b, "text %d,%d %s/%s %v %q\n", y, r.X, hex(r.Fg), hex(r.Bg), r.Upcase, r.Text)
		}
	}
	for _, p := range pics {
		switch {
		case p.Fill && p.Colour == color.RGBA{A: 255}: // the page's black
		case p.Fill:
			fmt.Fprintf(&b, "fill %v %s\n", p.At, hex(p.Colour))
		case p.At.Dx() == 8 && p.At.Dy() == 8:
		default:
			fmt.Fprintf(&b, "pic %s %d %d %v\n", p.File, p.Block, p.Frame, p.At)
		}
	}
	for _, o := range overs {
		fmt.Fprintf(&b, "png %v %v\n", o.Part().Size(), o.At)
	}
	return b.String()
}

// golden compares the screen's dump with testdata/name.txt.
func golden(t *testing.T, name string, v engine.View) {
	t.Helper()
	got := screenDump(v)
	path := filepath.Join("testdata", name+".txt")
	if *update {
		os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := 0; i < max(len(gl), len(wl)); i++ {
			g, w := "", ""
			if i < len(gl) {
				g = gl[i]
			}
			if i < len(wl) {
				w = wl[i]
			}
			if g != w {
				t.Errorf("%s line %d:\n got %s\nwant %s", name, i+1, g, w)
				break
			}
		}
	}
}

func writePNG(t *testing.T, w, h int) string {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range m.Pix {
		m.Pix[i] = byte(i * 7)
	}
	path := filepath.Join(t.TempDir(), "p.png")
	f, _ := os.Create(path)
	png.Encode(f, m)
	f.Close()
	return path
}

func TestSheetPage(t *testing.T) {
	e := headless(t, true)
	v := settled(e, engine.Action{Kind: engine.ActMember, N: -1})
	if v.Sheet == nil {
		t.Fatal("no sheet")
	}
	golden(t, "sheet", v)
	v = settled(e, engine.Action{Kind: engine.ActMember, N: 1})
	golden(t, "sheet2", v)
	v = settled(e, engine.Action{Kind: engine.ActPortrait, N: -1, Text: writePNG(t, 40, 60)})
	golden(t, "sheetpng", v)
}

func TestCreationPage(t *testing.T) {
	e, _ := partyMenu(t)
	v := settled(e, engine.Action{Kind: engine.ActCreate})
	if v.Creation == nil {
		t.Fatal("no creation")
	}
	golden(t, "creation", v)
	v = settled(e, engine.Action{Kind: engine.ActIcon, Text: writePNG(t, 48, 24)})
	golden(t, "creationpng", v)
	v = settled(e, engine.Action{Kind: engine.ActPicker, N: engine.PickHead})
	if v.Picker == nil {
		t.Fatal("no picker")
	}
	golden(t, "pickhead", v)
	v = settled(e, engine.Action{Kind: engine.ActClose}, engine.Action{Kind: engine.ActIconEdit, N: -1})
	if !v.IconDrop {
		t.Fatal("no icon editor")
	}
	golden(t, "icon", v)
	ed := v.IconEditor
	for _, k := range []struct {
		x, y int
		want engine.Action
	}{
		{ed.Head.X1, ed.Head.Y1, engine.Action{Kind: engine.ActPicker, N: engine.PickIconHead}},
		{ed.PoseAt[1].X1, ed.PoseAt[1].Y1 + 5, engine.Action{Kind: engine.ActPicker, N: engine.PickWeapon}},
		{ed.Size.X2, ed.Size.Y1, engine.Action{Kind: engine.ActIconSize}},
		{ed.Parts.X1 + 3, ed.Parts.Y1 + 2, engine.Action{Kind: engine.ActIconColour, N: 2, X: 1, Y: -1}},
		{ed.Palette.X1 + 3, ed.Palette.Y1 + 1, engine.Action{Kind: engine.ActIconColour, N: ed.Part, X: ed.Shade, Y: ed.Colours[9]}},
	} {
		if a, ok := iconClick(ed, k.x, k.y); !ok || a != k.want {
			t.Errorf("click %d,%d: %+v, want %+v", k.x, k.y, a, k.want)
		}
	}
	if v = settled(e, engine.Action{Kind: engine.ActIconSize}); v.IconEditor == nil || v.IconEditor.Large == ed.Large {
		t.Error("Size not taken")
	}
	settled(e, engine.Action{Kind: engine.ActIconSize})
	v = settled(e, engine.Action{Kind: engine.ActPicker, N: engine.PickWeapon})
	golden(t, "pickweapon", v)
}

func TestPartyPages(t *testing.T) {
	e, _ := partyMenu(t)
	v := settled(e, engine.Action{Kind: engine.ActQuickStart})
	// The party menu's roster (drawPartyList, recorded in party.txt) is the
	// presenter's from View.Party: the names, AC and HP.
	want, err := os.ReadFile("testdata/party.txt")
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, l := range strings.Split(string(want), "\n") {
		var y, x int
		var fg, q string
		if n, _ := fmt.Sscanf(l, "text %d,%d %s true %q", &y, &x, &fg, &q); n == 4 && y >= 4 {
			for len(rows) <= y-4 {
				rows = append(rows, "")
			}
			rows[y-4] += " " + q
		}
	}
	var got []string
	for _, m := range v.Party {
		got = append(got, fmt.Sprintf(" %s %d %d", m.Name, m.AC, m.HP))
	}
	if !slices.Equal(got, rows) {
		t.Errorf("party %q, the roster showed %q", got, rows)
	}
	v = settled(e, engine.Action{Kind: engine.ActRemove, N: 0}, engine.Action{Kind: engine.ActAdd})
	if v.Picker == nil {
		t.Fatal("no saved characters")
	}
	golden(t, "picksaved", v)
	v = settled(e, engine.Action{Kind: engine.ActClose}, engine.Action{Kind: engine.ActSlots})
	if v.Slots == nil {
		t.Fatal("no slots")
	}
	golden(t, "slots", v)
}

func screenDump(v engine.View) string {
	c := screenPage(v)
	return dump(c.lines(), c.pics, c.overs)
}

// The spell lists' title replaces the sheet's name row (SpellList: a box
// over row 1, the name's, the title after it), and the PNG portrait goes.
func TestSheetSpellsTitle(t *testing.T) {
	var ink [16]color.RGBA
	for i := range ink {
		ink[i] = col(i)
	}
	v := engine.View{Cells: image.Pt(40, 25), Ink: ink, SheetPage: &engine.SheetPage{Name: "Mirelle", NameColour: inkCyan, Spells: "Spells in Memory"}}
	c := screenPage(v)
	got := c.lines()[1]
	want := []engine.PaneRun{{X: 1, Text: "Mirelle's", Fg: ink[inkCyan], Bg: ink[0], Upcase: true}, {X: 11, Text: "Spells in Memory", Fg: ink[inkGreen], Bg: ink[0], Upcase: true}}
	if !slices.Equal(got, want) {
		t.Errorf("row 1: %+v, want %+v", got, want)
	}
	if !slices.Contains(c.boxes, engine.CellRect{X1: 1, Y1: 1, X2: 38, Y2: 1}) {
		t.Errorf("no box round the title: %v", c.boxes)
	}
}

// devScene: adventuring with dev scene name (engine.DevScene) put up.
func devScene(t *testing.T, name string) (*engine.Engine, engine.View) {
	t.Helper()
	if _, err := os.Stat("../../data/dos/TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	e, err := engine.New("../../data")
	if err != nil {
		t.Fatal(err)
	}
	seed := uint32(7)
	e.FixedSeed = &seed
	e.Cfg.SaveDir = t.TempDir()
	e.Explore = true
	if name != "" {
		e.DevAction = e.DevScene(name)
	}
	e.Init()
	e.Start(e.Play)
	return e, settled(e)
}

// The adventuring screens (the main frame, the roster, the status line,
// the text pane, after-combat, rest, casting, the runes' window text),
// recorded from the engine's drawing (step 4 part 3).
func TestAdventurePages(t *testing.T) {
	for _, sc := range []struct {
		file, name string
		acts       []engine.Action
	}{
		{"adv_", "", nil},
		{"adv_loot", "loot", nil},
		{"adv_cast", "cast", nil},
		{"adv_runes", "runes", nil},
		{"adv_journal", "journal", nil},
		{"adv_parlay", "parlay", nil},
		{"adv_password", "password", nil},
		{"adv_who", "who", nil},
		{"adv_minimap", "", []engine.Action{{Kind: engine.ActMinimap}}},
		{"adv_rest", "", []engine.Action{{Kind: engine.ActCamp}, {Kind: engine.ActRest, N: engine.RestWait}}},
	} {
		e, _ := devScene(t, sc.name)
		v := settled(e, sc.acts...)
		golden(t, sc.file, v)
	}
}

// The after-combat page and the roster laid out from their data
// (showResult, the party destroyed, drawPartyList's colours).
func TestResultsRoster(t *testing.T) {
	var ink [16]color.RGBA
	for i := range ink {
		ink[i] = col(i)
	}
	row := func(c *cellPage, y int) string {
		var s []string
		for _, r := range c.lines()[y] {
			s = append(s, fmt.Sprintf("%d:%s", r.X, r.Text))
		}
		return strings.Join(s, " ")
	}
	v := engine.View{Cells: image.Pt(40, 25), Ink: ink, Results: &engine.Results{Outcome: "The party has won.", Exp: "Each character receives 120",
		Taken: "The party takes 156 gold pieces, 7 gems and a great many other things besides.", Harmed: []engine.Harm{{Name: "Pip", What: "Dead"}}}}
	c := screenPage(v)
	for y, want := range map[int]string{3: "1:The party has won.", 5: "1:Each character receives 120", 7: "1:experience points.",
		9: "1:The party takes 156 gold pieces, 7 ", 10: "1:gems and a great many other things ", 11: "1:besides.", 12: "1:Pip 18:Dead"} {
		if got := row(c, y); got != want {
			t.Errorf("row %d: %q, want %q", y, got, want)
		}
	}
	v.Results = &engine.Results{End: "The entire party is killed!", EndAt: engine.CellRect{X1: 1, Y1: 1, X2: 38, Y2: 22}}
	if got := row(screenPage(v), 1); got != "1:The entire party is killed!" {
		t.Errorf("end: %q", got)
	}
	v = engine.View{Cells: image.Pt(40, 25), Ink: ink, Roster: true, Status: "15,1 W 00:00",
		Party: []engine.Member{{Name: "Pip", AC: -2, HP: 3, MaxHP: 8, Colour: inkMagenta}, {Name: "Thorgrim", AC: 10, HP: 9, MaxHP: 9, Selected: true}}}
	c = screenPage(v)
	for y, want := range map[int]string{2: "17:Name 33:AC  HP", 4: "17:Pip 33:-2 38:3", 5: "17:Thorgrim 33:10 38:9", 15: "17:15,1 W 00:00"} {
		if got := row(c, y); got != want {
			t.Errorf("row %d: %q, want %q", y, got, want)
		}
	}
	if r := c.lines()[4][0]; r.Fg != ink[inkMagenta] {
		t.Errorf("Pip in %v", r.Fg)
	}
	if i := rosterRow(v, 20, 5); i != 1 {
		t.Errorf("roster click: %d", i)
	}
}
