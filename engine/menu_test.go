package engine

import "testing"

func TestItemSpan(t *testing.T) {
	menu := "Parts color-1 color-2 Size Exit"
	start, end := menuItems(menu)
	want := []string{"", "Parts", "color-1", "color-2", "Size", "Exit"}
	for j := 1; j < len(start); j++ {
		if a, b := itemSpan(menu, start, end, j); menu[a-1:b] != want[j] {
			t.Errorf("item %d: %q, want %q", j, menu[a-1:b], want[j])
		}
	}
	// a word with two hotkeys keeps the original spans
	start, end = menuItems("OK No")
	if a, b := itemSpan("OK No", start, end, 1); a != 1 || b != 1 {
		t.Errorf("OK: %d..%d", a, b)
	}
}

func TestItemStats(t *testing.T) {
	e := &Engine{items: make([]itemBase, 4)}
	e.items[1].Dice, e.items[1].Sides, e.items[1].Bonus = 2, 4, 1 // type 1: 2d4+1
	e.items[2].Protection = 0x80 + 1                              // type 2: shield 1
	e.items[3].Protection = 0x80 + 57                             // type 3: plate mail 7
	for typ, want := range []string{"", "(2d4+1) ", "[1] ", "[7] "} {
		if s := e.itemStats(itemType(typ)); s != want {
			t.Errorf("type %d: %q, want %q", typ, s, want)
		}
	}
}

func TestMenuGlyph(t *testing.T) {
	menu := "Cast sWeep ahead Search here"
	got := make([]byte, len(menu))
	for i := range got {
		got[i] = menuGlyph(menu, i+1)
	}
	if string(got) != "Cast Sweep ahead Search here" {
		t.Errorf("shown %q", got)
	}
	start, end := menuItems(menu)
	if a, b := itemSpan(menu, start, end, 2); menu[a-1:b] != "sWeep ahead" {
		t.Errorf("span %q", menu[a-1:b])
	}
	if a, b := itemSpan(menu, start, end, 3); menu[a-1:b] != "Search here" {
		t.Errorf("span %q", menu[a-1:b])
	}
}

// A modal screen publishes its own spots only, not the screen's under it.
func TestModalFloor(t *testing.T) {
	e := &Engine{}
	e.addRegion(region{x0: 0, y0: 0, x1: 39, y1: 24, click: func(int, int) Action { return Action{Kind: actHotkey, N: 'A'} }})
	off := e.modalOn()
	if s := e.spots(); len(s) != 0 {
		t.Errorf("spots through the modal: %v", s)
	}
	e.addRegion(region{x0: 5, y0: 5, x1: 5, y1: 5, click: func(int, int) Action { return Action{Kind: actHotkey, N: 'Y'} }})
	if k := e.take(Action{Kind: ActSpot, N: 0, X: 5, Y: 5}); k.N != 'Y' {
		t.Errorf("modal's own spot: %v", k)
	}
	off()
	if k := e.take(Action{Kind: ActSpot, N: 0, X: 1, Y: 1}); k.N != 'A' {
		t.Errorf("after the modal: %v", k)
	}
}

// An ECL picker list (opVMenu) has an empty bar: menuItems gives one item
// with no word, which ListMenu must not index.
func TestMenuItemsEmpty(t *testing.T) {
	start, _ := menuItems("")
	if len(start) != 2 || start[1] != 0 {
		t.Fatalf("start = %v", start)
	}
}

// A menu bar decides on actions: a pick hands back the item's Choice, Esc
// the Exit item, Right moves the highlight, the keypad a direction.
func TestMenuBarDecide(t *testing.T) {
	e := &Engine{}
	const (
		look Choice = iota
		take
		exit
	)
	bar := Bar{{"Look", look}, {"Take", take}, {"Exit", exit}}
	menu := bar.String()
	start, end := menuItems(menu)
	e.ext.menu, e.menuSel = 1, 1
	e.menuShown("", menu, start, end)
	e.ext.nav = navCtx{menu: menu, exit: menuHasExit(menu)}
	m := &menuBar{e: e, menu: menu, start: start, ids: bar.ids(), count: len(start) - 1, allowExtra: true, allowEnter: true, redraw: func() {}}
	if p, ok := m.decide(Action{Kind: ActPick, N: 1}); p.ID != take || !ok || e.menuSel != 2 {
		t.Errorf("pick: %v %v sel %d", p, ok, e.menuSel)
	}
	if p, ok := m.decide(Action{Kind: ActClose}); p.ID != exit || !ok {
		t.Errorf("close: %v %v", p, ok)
	}
	if _, ok := m.decide(Action{Kind: ActMove, N: DirRight}); ok || e.menuSel != 1 {
		t.Errorf("right: %v sel %d", ok, e.menuSel)
	}
	if p, ok := m.decide(Action{Kind: ActMove, N: DirHome}); p.ID != PickMove || p.Dir != DirHome || !ok {
		t.Errorf("home: %v %v", p, ok)
	}
}
