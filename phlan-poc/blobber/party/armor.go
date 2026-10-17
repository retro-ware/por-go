package party

import "fmt"

// Armor pieces from the 16x16 armor sheet: icon aPCC, P = piece row, CC = colour column.
// ponytail: gloves (row 2) and leggings (row 3) are sliced but unused, there is no slot for them.
var armorColour = [...]int{6, 9, 10, 13, 16} // per material: orange, grey, silver, gold, red

func init() {
	for _, pc := range []struct {
		name      string
		row       int
		slot      Slot
		ac, value int
	}{{"Helm", 0, Helmet, 1, 30}, {"Cuirass", 1, Body, 4, 120}, {"Boots", 4, Shoes, 1, 25}} {
		for t, m := range Materials {
			it := Item{Title: m.Name + " " + pc.name, Kind: Armor, Slot: pc.slot, AC: pc.ac + m.Dmg/2,
				Value: pc.value * m.Value, Icon: fmt.Sprintf("a%d%02d", pc.row, armorColour[t])}
			Catalog[key(it.Title)] = it
		}
	}
}
