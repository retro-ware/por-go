package engine

import "testing"

// Every base type makes an item: the table records RandomItem reads lie
// inside itemTable (the rod's record at 0x91 ran past it).
func TestRandomItemAllTypes(t *testing.T) {
	e := &Engine{}
	for typ := 0; typ < 0x100; typ++ {
		for range 50 {
			e.RandomItem(itemType(typ))
		}
	}
	if it := e.RandomItem(0x43); it.Price != 6000 || it.Spells != [3]byte{0, 0x48, 0x86} {
		t.Errorf("rod record 0x91: price %d, spells %v", it.Price, it.Spells)
	}
}
