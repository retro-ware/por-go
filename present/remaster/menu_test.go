package remaster

import (
	"testing"

	"blobber"
	"por/engine"
)

// Esc closes the command bar's popup menu (Enter opened it) without asking
// the game to leave it: the bar has no Exit item.
func TestEscClosesCommandMenu(t *testing.T) {
	p := &Presenter{active: true, view: &blobber.View{}}
	v := engine.View{Command: true, Menu: &engine.Menu{Items: []engine.MenuItem{{Text: "Move", Key: 'M'}}}}
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
