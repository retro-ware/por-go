package engine

import "testing"

// A thrown weapon readied beside the weapon in hand goes into the Missiles
// slot (Readied 2), out of the hands, and comes back out of it; without a
// weapon in hand it is readied in hand as the original does.
func TestMissileSlot(t *testing.T) {
	e, err := New("../data")
	if err != nil {
		t.Skip(err)
	}
	e.items = e.readItems()
	if !e.LoadSlotNamed("in-Slums") {
		t.Skip("no in-Slums save")
	}
	e.selectMember(1)
	c := e.selected()
	e.ApplyItems(c)
	w := c.Slots[slotWeapon]
	if w == nil {
		t.Fatal("no weapon in hand")
	}
	j := e.AddItemCopy(c, e.RandomItem(itemJavelin))
	if dollSlot(e, c, j) != DollArrows {
		t.Errorf("doll slot %d, want Missiles", dollSlot(e, c, j))
	}
	e.readyItem(j)
	e.ApplyItems(c)
	if j.Readied != 2 || c.Slots[slotArrows] != j || e.missile(c) != j || c.Slots[slotWeapon] != w {
		t.Errorf("readied %d, missile %v, weapon kept %v", j.Readied, e.missile(c) == j, c.Slots[slotWeapon] == w)
	}
	e.readyItem(j) // out again
	e.ApplyItems(c)
	if j.Readied != 0 || e.missile(c) != nil {
		t.Errorf("unready: readied %d", j.Readied)
	}
	e.readyItem(w) // no weapon in hand: the javelin is the weapon
	e.ApplyItems(c)
	e.readyItem(j)
	e.ApplyItems(c)
	if j.Readied != 1 || c.Slots[slotWeapon] != j {
		t.Errorf("in hand: readied %d, weapon %v", j.Readied, c.Slots[slotWeapon] == j)
	}
}
