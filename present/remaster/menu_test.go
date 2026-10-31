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
