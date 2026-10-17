package party

import (
	"fmt"
	"strings"
)

// Materials, worst to best. A weapon's material sets its name affix, damage bonus and price.
var Materials = []struct {
	Name, Wood string // Wood names the first tier of wooden families
	Dmg, Value int    // damage bonus, price multiplier
}{
	{"Bronze", "Oak", 0, 1},
	{"Iron", "", 1, 2},
	{"Steel", "", 2, 4},
	{"Orichalcum", "", 4, 10},
	{"Bloodforged", "", 6, 25},
}

// family is one weapon design from the 16x16 weapons pack; each material tier has its own icon.
// Icons 96-103 are arrows, 105/109/113/117 bolts, 107-108 (+4 per tier) drawn-bow frames: not items.
type family struct {
	name       string
	icon       int  // icon number (wNNN) of its first tier
	from, n    int  // first material tier and number of tiers
	stride     int  // icon step between tiers
	wood       bool // first tier is wooden
	kind       Kind
	slot       Slot
	dmg, value int // at Bronze; shields: AC
}

var families = []family{
	{"Sword", 1, 0, 5, 1, false, Weapon, RightHand, 6, 60},
	{"Longsword", 6, 0, 5, 1, false, Weapon, RightHand, 8, 90},
	{"Scimitar", 11, 0, 5, 1, false, Weapon, RightHand, 7, 80},
	{"Dagger", 16, 0, 5, 1, false, Weapon, RightHand, 3, 20},
	{"Rapier", 21, 0, 5, 1, false, Weapon, RightHand, 5, 70},
	{"Sickle", 26, 0, 5, 1, false, Weapon, RightHand, 4, 25},
	{"Axe", 31, 0, 5, 1, false, Weapon, RightHand, 7, 60},
	{"Shield", 36, 0, 5, 1, true, Armor, LeftHand, 2, 40},
	{"Staff", 41, 0, 5, 1, true, Weapon, RightHand, 4, 15},
	{"Mace", 46, 0, 5, 1, false, Weapon, RightHand, 6, 50},
	{"War Pick", 51, 0, 5, 1, false, Weapon, RightHand, 7, 65},
	{"Morningstar", 56, 0, 5, 1, true, Weapon, RightHand, 8, 85},
	{"Warhammer", 61, 0, 5, 1, false, Weapon, RightHand, 9, 100},
	{"Spear", 66, 0, 5, 1, false, Weapon, RightHand, 7, 40},
	{"Halberd", 71, 0, 5, 1, false, Weapon, RightHand, 10, 120},
	{"Greatsword", 76, 0, 5, 1, false, Weapon, RightHand, 12, 150},
	{"Cudgel", 81, 0, 1, 1, true, Weapon, RightHand, 3, 5},
	{"Trident", 82, 1, 1, 1, false, Weapon, RightHand, 8, 90},
	{"Katana", 83, 0, 5, 1, true, Weapon, RightHand, 9, 140},
	{"Pike", 88, 1, 4, 1, false, Weapon, RightHand, 9, 70},
	{"Glaive", 92, 1, 4, 1, false, Weapon, RightHand, 10, 110},
	{"Longbow", 104, 0, 1, 1, true, Weapon, Ranged, 6, 80},
	{"Bow", 106, 0, 4, 4, true, Weapon, Ranged, 5, 50},
}

// key is the script name of a catalog entry: "Steel Longsword" -> "steel_longsword".
func key(title string) string { return strings.ReplaceAll(strings.ToLower(title), " ", "_") }

func init() {
	for _, f := range families {
		for t := range f.n {
			m := Materials[f.from+t]
			affix := m.Name
			if f.wood && f.from+t == 0 {
				affix = m.Wood
			}
			it := Item{Title: affix + " " + f.name, Kind: f.kind, Slot: f.slot, Value: f.value * m.Value,
				Icon: fmt.Sprintf("w%03d", f.icon+t*f.stride)}
			if f.kind == Armor {
				it.AC = f.dmg + m.Dmg/2
			} else if f.kind == Weapon {
				it.Damage = f.dmg + m.Dmg
			}
			Catalog[key(it.Title)] = it
		}
	}
}

// Tier is an item's material tier (0 = Bronze/Oak), or -1 for items without a material.
func Tier(it Item) int {
	for i, m := range Materials {
		if strings.HasPrefix(it.Title, m.Name+" ") || (m.Wood != "" && strings.HasPrefix(it.Title, m.Wood+" ")) {
			return i
		}
	}
	return -1
}
