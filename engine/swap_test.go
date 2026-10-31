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

// Bracers take the armour's slot: readying them swaps the armour off, and
// an older save with both readied keeps the armour.
func TestBracersSwapArmor(t *testing.T) {
	e := &Engine{items: make([]itemBase, itemTypes)}
	e.items[itemPlateMail] = itemBase{Slot: slotArmor, Protection: acGiven | (acTen + 7), Classes: 0xFF}
	e.items[itemBracers] = itemBase{Slot: byte(itemBracers), Protection: acGiven | acTen, Classes: 0xFF}
	c := &Character{}
	c.ClassMask = 1
	e.party = []*Character{c}
	plate := &Item{Type: itemPlateMail, Readied: 1}
	c.Items = []*Item{plate}
	e.ApplyItems(c)
	e.AddItemCopy(c, &Item{Type: itemBracers, Plus: 4})
	b := e.pool[0]
	if dollSlot(e, c, b) != DollArmor || e.inTheWay(c, b) != plate {
		t.Fatalf("bracers: doll slot %d, in the way %v", dollSlot(e, c, b), e.inTheWay(c, b))
	}
	e.swapReady(c, b)
	e.ApplyItems(c)
	if b.Readied == 0 || plate.Readied != 0 || c.Slots[slotArmor] != b {
		t.Fatalf("bracers %d plate %d", b.Readied, plate.Readied)
	}
	plate.Readied = 1 // as an older save: both readied
	c.Items = []*Item{b, plate}
	e.ApplyItems(c)
	if c.Slots[slotArmor] != plate || b.Readied != 0 {
		t.Fatalf("both readied: slot holds %v, bracers readied %d", c.Slots[slotArmor], b.Readied)
	}
}
