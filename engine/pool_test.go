package engine

import "testing"

// The party inventory: unreadied items of members go into one pool with a
// slot count; stacks join; a readied bag of holding adds slots; a saved
// game keeps it with the first member's items.
func TestPartyPool(t *testing.T) {
	e := &Engine{}
	a, b := &Character{}, &Character{}
	e.party = []*Character{a, b}
	sword := &Item{Type: itemLongSword, Readied: 1}
	a.Items = []*Item{sword, {Type: itemDagger}}
	e.ApplyItems(a)
	if len(a.Items) != 1 || len(e.pool) != 1 {
		t.Fatalf("stow: own %d pool %d", len(a.Items), len(e.pool))
	}
	e.AddItemCopy(b, &Item{Type: itemArrow, Count: 10})
	e.AddItemCopy(a, &Item{Type: itemArrow, Count: 5})
	if len(e.pool) != 2 || e.pool[1].Count != 15 || len(b.Items) != 0 {
		t.Fatalf("arrows: pool %d", len(e.pool))
	}
	for len(e.pool) < poolSlots {
		e.AddItemCopy(b, &Item{Type: itemDagger})
	}
	if !e.overloaded(&Item{Type: itemDagger}, a) || e.overloaded(&Item{Type: itemArrow, Count: 1}, a) {
		t.Error("full pool: a new item must not fit, arrows join their stack")
	}
	a.Items = append(a.Items, &Item{Readied: 1, Words: [3]byte{0xBA}}) // bag of holding
	if e.poolCap() != poolSlots+poolBag || e.overloaded(&Item{Type: itemDagger}, a) {
		t.Error("bag of holding")
	}
	if len(e.savedItems(a, "CHRDATA1")) != len(a.Items)+len(e.pool) || len(e.savedItems(b, "CHRDATA2")) != 0 || len(e.savedItems(a, "")) != len(a.Items) {
		t.Error("saved items")
	}
	d := e.pool[2]
	e.RemoveItem(b, d)
	if len(e.pool) != poolSlots-1 {
		t.Error("remove from pool")
	}
}
