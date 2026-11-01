package engine

import "testing"

func TestItemStats(t *testing.T) {
	e := &Engine{items: make([]itemBase, 5)}
	e.items[1].Dice, e.items[1].Sides, e.items[1].Bonus = 2, 4, 1 // type 1: 2d4+1
	e.items[2].Protection = acGiven | 1                           // type 2: shield 1
	e.items[3].Protection = acGiven | (acTen + 7)                 // type 3: plate mail 7
	e.items[4].Protection = acGiven | acTen                       // type 4: bracers 0
	for typ, want := range []string{"", "(2d4+1) ", "[1] ", "[7] ", "[0] "} {
		if s := e.itemStats(&Item{Type: itemType(typ)}); s != want {
			t.Errorf("type %d: %q, want %q", typ, s, want)
		}
	}
	for _, c := range []struct { // the known plus counts in; a hidden one does not
		typ    itemType
		plus   int8
		hidden byte
		want   string
	}{{4, 4, 0, "[4] "}, {4, 4, 1, "[0] "}, {3, 2, 0, "[9] "}, {2, 1, 0, "[2] "}, {1, 3, 0, "(2d4+1) "}} {
		if s := e.itemStats(&Item{Type: c.typ, Plus: byte(c.plus), Hidden: c.hidden}); s != c.want {
			t.Errorf("type %d %+d hidden %d: %q, want %q", c.typ, c.plus, c.hidden, s, c.want)
		}
	}
}

func TestModalSpots(t *testing.T) {
	e := &Engine{}
	e.addRegion(region{x0: 0, y0: 0, x1: 39, y1: 24, click: func(int, int) Action { return Action{Kind: actHotkey, N: 'A'} }})
	e.push(modeHelp, func(Action) bool { return true }, nil)
	if s := e.spots(); len(s) != 0 {
		t.Errorf("spots through the modal: %v", s)
	}
	if k := e.take(Action{Kind: ActSpot, N: 0, X: 1, Y: 1}); k.Kind != ActNone {
		t.Errorf("a spot under the modal: %v", k)
	}
	e.screens = nil
	if k := e.take(Action{Kind: ActSpot, N: 0, X: 1, Y: 1}); k.N != 'A' {
		t.Errorf("after the modal: %v", k)
	}
}

// An ECL picker list (opVMenu) has an empty bar: menuItems gives one item
// with no word, which ListMenu must not index.
func TestOptionText(t *testing.T) {
	for in, want := range map[string]string{"sWeep": "Sweep", "aiM": "Aim", "HEAL": "HEAL", "Exit": "Exit"} {
		if got := optionText(in); got != want {
			t.Errorf("optionText(%q) = %q, want %q", in, got, want)
		}
	}
}
