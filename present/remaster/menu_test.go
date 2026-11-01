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
