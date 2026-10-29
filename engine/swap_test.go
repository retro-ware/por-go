package engine

import "testing"

// Readying an item whose slot is taken swaps the two.
func TestSwapReady(t *testing.T) {
	e := &Engine{items: make([]itemBase, itemTypes)}
	e.items[itemLongSword] = itemBase{Hands: 1, Classes: 0xFF}
	e.items[itemDagger] = itemBase{Hands: 1, Classes: 0xFF}
	c := &Character{}
	c.ClassMask = 1
	e.party = []*Character{c}
	sword := &Item{Type: itemLongSword, Readied: 1}
	c.Items = []*Item{sword}
	e.ApplyItems(c)
	e.AddItemCopy(c, &Item{Type: itemDagger})
	d := e.pool[0]
	e.swapReady(c, d)
	e.ApplyItems(c)
	if d.Readied == 0 || sword.Readied != 0 || c.Slots[slotWeapon] != d {
		t.Fatalf("dagger %d sword %d", d.Readied, sword.Readied)
	}
}
