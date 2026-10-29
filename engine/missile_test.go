package engine

import (
	"slices"
	"testing"
)

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
	// A launcher shoots the Missiles slot's matching ammunition: a bow the
	// arrows (not quarrels), a sling nothing; one kind of ammunition at a time.
	i := slices.IndexFunc(e.party, func(c *Character) bool {
		return e.base(itemShortBow).Classes&e.base(itemSling).Classes&c.ClassMask != 0
	})
	if i < 0 {
		t.Fatal("nobody for a bow and a sling")
	}
	e.selectMember(i)
	c = e.selected()
	e.ApplyItems(c)
	for _, s := range []int{slotWeapon, slotShield, slotArrows, slotBolts} {
		if c.Slots[s] != nil {
			e.readyItem(c.Slots[s])
			e.ApplyItems(c)
		}
	}
	bow, arrows, quarrels, sling := e.AddItemCopy(c, e.RandomItem(itemShortBow)), e.AddItemCopy(c, e.RandomItem(itemArrow)), e.AddItemCopy(c, e.RandomItem(itemQuarrel)), e.AddItemCopy(c, e.RandomItem(itemSling))
	e.readyItem(bow)
	e.ApplyItems(c)
	if c.Slots[slotWeapon] != bow || e.missile(c) != nil {
		t.Errorf("bow readied %d, missile without arrows %v", bow.Readied, e.missile(c))
	}
	e.readyItem(quarrels)
	e.ApplyItems(c)
	if dollSlot(e, c, quarrels) != DollArrows || e.missile(c) != nil {
		t.Errorf("quarrels: doll slot %d, missile for a bow %v", dollSlot(e, c, quarrels), e.missile(c))
	}
	e.readyItem(arrows) // the Missiles slot holds the quarrels
	if arrows.Readied != 0 {
		t.Error("arrows readied beside quarrels")
	}
	e.readyItem(quarrels)
	e.ApplyItems(c)
	e.readyItem(arrows)
	e.ApplyItems(c)
	if e.missile(c) != arrows {
		t.Errorf("bow and arrows: missile %v (readied %d, %s)", e.missile(c), arrows.Readied, e.ext.note)
	}
	e.readyItem(bow)
	e.ApplyItems(c)
	e.readyItem(sling)
	e.ApplyItems(c)
	if e.missile(c) != sling {
		t.Errorf("sling: missile %v (readied %d, %s)", e.missile(c), sling.Readied, e.ext.note)
	}
}
