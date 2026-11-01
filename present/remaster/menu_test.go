package remaster

import (
	"testing"

	"blobber"
	"por/engine"
)

// Esc closes the command menu (Enter opened it).
func TestEscClosesCommandMenu(t *testing.T) {
	p := &Presenter{active: true, view: &blobber.View{}}
	v := engine.View{Exploring: true}
	if !p.key(13, v) || !p.menu {
		t.Fatal("Enter did not open the menu")
	}
	if !p.key(27, v) || p.menu {
		t.Fatal("Esc did not close the menu")
	}
	p.key(13, v)
	p.Mouse(0, 0, false, false, true, 0, v)
	if p.menu {
		t.Fatal("a right click did not close the menu")
	}
}

// menuNav: Esc is back and never picks; a hotkey picks in either case; a
// hotkey letter not in the text is shown in front.
func TestMenuNav(t *testing.T) {
	sel := 0
	keys := []byte{'C', 'E'}
	if i, back, _ := menuNav(27, &sel, keys); i != -1 || !back {
		t.Fatal("Esc picked a row")
	}
	if i, _, _ := menuNav('e', &sel, keys); i != 1 || sel != 1 {
		t.Fatal("hotkey e did not pick row 1")
	}
	if menuNav(0x5000, &sel, keys); sel != 0 {
		t.Fatal("Down did not wrap")
	}
	if s, at := hotText("Camp", 'E'); s != "[E] Camp" || at != 1 {
		t.Fatal(s)
	}
}

// digitKeys: rows with no letter and that can be picked get 1-9, skipping
// digits already taken, then none.
func TestDigitKeys(t *testing.T) {
	keys := digitKeys([]byte{'B', 0, '2', 0, 0}, func(i int) bool { return i != 3 })
	if string(keys) != "B12\x003" {
		t.Fatalf("%q", keys)
	}
	many := digitKeys(make([]byte, 12), func(int) bool { return true })
	if many[8] != '9' || many[9] != 0 {
		t.Fatalf("%q", many)
	}
}

// editKey: the cursor moves, Backspace and Delete edit at it, typing
// inserts at it; caretLine shows it.
func TestEditCursor(t *testing.T) {
	s, c := "abd", 3
	for _, k := range []uint16{0x4B00, 'c', 0x4700, 0x5300, 'A', 0x4F00, 8} {
		editKey(k, &s, &c, 10, false)
	}
	if s != "Abc" || c != 3 {
		t.Fatalf("%q %d", s, c)
	}
	if editKey(0x4B00, &s, &c, 10, false); c != 2 {
		t.Fatal(c)
	}
	if l := caretLine(s, 1, 4, true); l != "A_c  " {
		t.Fatalf("%q", l)
	}
}
