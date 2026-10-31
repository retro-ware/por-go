package engine

import (
	"slices"
	"testing"
)

// TestBracersLook: bracers are on the armour's scale (50 is AC 10), so
// Bracers +4 read AC 6, not "AC 50 better".
func TestBracersLook(t *testing.T) {
	e, err := New("../data")
	if err != nil {
		t.Skip("no game files")
	}
	e.items = e.readItems()
	if e.base(itemBracers).Protection != acGiven|acTen {
		t.Skip("bracers not in the item table")
	}
	l := e.itemLines(&Item{Type: itemBracers, Plus: 4}, nil)
	if !slices.Contains(l, "AC 6, as armor") {
		t.Errorf("Look %q, want AC 6, as armor", l)
	}
	if p := e.protection(&Item{Type: itemBracers, Plus: 4}); p != 4 {
		t.Errorf("protection %v, want 4", p)
	}
}
