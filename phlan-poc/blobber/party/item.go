package party

import "fmt"

// Item model after Legacy: one shared party inventory of stacks; equipped items stay in it,
// tagged with their wearer, and never stack.

type Kind int

const (
	Weapon Kind = iota
	Armor
	Accessory
	Consumable
	Scroll
	Misc
	Key
)

var KindNames = []string{"weapon", "armor", "accessory", "consumable", "scroll", "misc", "key"}

type Slot string

const (
	RightHand Slot = "right hand"
	LeftHand  Slot = "left hand"
	Ranged    Slot = "ranged"
	Helmet    Slot = "helmet"
	Body      Slot = "body"
	Shoes     Slot = "shoes"
	RingL     Slot = "ring left"
	RingR     Slot = "ring right"
	Amulet1   Slot = "amulet one"
	Amulet2   Slot = "amulet two"
)

// Slots in display order.
var Slots = []Slot{RightHand, LeftHand, Ranged, Helmet, Body, Shoes, RingL, RingR, Amulet1, Amulet2}

type Item struct {
	Base   string // catalog name, e.g. "steel_dagger"
	Title  string // display name, e.g. "Steel Dagger"; Base when empty
	Kind   Kind
	Slot   Slot // preferred slot; "" if not equippable
	Icon   string
	Bonus  int // enchantment
	AC     int
	Damage int
	Heal   int // consumables: HP, scrolls: SP
	Value  int
	Wearer *Character `json:"-"`
	Worn   int        `json:",omitempty"` // save only: 1-based wearer slot, see Party.Prepare / Relink
	WornAt Slot       `json:",omitempty"`
}

// Catalog holds the item templates scripts can name.
var Catalog = map[string]Item{
	"helmet":     {Kind: Armor, Slot: Helmet, Icon: "a003", AC: 1, Value: 30},
	"leather":    {Title: "Leather Armor", Kind: Armor, Slot: Body, Icon: "a100", AC: 2, Value: 40},
	"chainmail":  {Kind: Armor, Slot: Body, Icon: "a109", AC: 4, Value: 120},
	"plate":      {Title: "Plate Armor", Kind: Armor, Slot: Body, Icon: "a110", AC: 7, Value: 400},
	"boots":      {Kind: Armor, Slot: Shoes, Icon: "a403", AC: 1, Value: 15},
	"wizard_hat": {Title: "Wizard Hat", Kind: Armor, Slot: Helmet, Icon: "wizard_hat", AC: 1, Value: 25},
	"ring":       {Kind: Accessory, Slot: RingL, Icon: "ring", AC: 1, Value: 90},
	"amulet":     {Kind: Accessory, Slot: Amulet1, Icon: "amulet", AC: 1, Value: 70},
	"potion":     {Kind: Consumable, Icon: "potion", Heal: 10, Value: 10},
	"scroll":     {Kind: Scroll, Icon: "scroll", Heal: 10, Value: 15},
	"gem":        {Kind: Misc, Icon: "gem", Value: 50},
}

// NewItem makes an item from the catalog; unknown names become misc items with a bag icon.
func NewItem(base string, bonus int) *Item {
	it, ok := Catalog[base]
	if !ok {
		it = Item{Kind: Misc, Icon: "bag"}
	}
	it.Base, it.Bonus = base, bonus
	return &it
}

// NewKey makes a key item named "<name> key".
func NewKey(name string) *Item {
	return &Item{Base: name + " key", Kind: Key, Icon: "key"}
}

func (it *Item) Name() string {
	n := it.Base
	if it.Title != "" {
		n = it.Title
	}
	if it.Bonus > 0 {
		return fmt.Sprintf("%s +%d", n, it.Bonus)
	}
	return n
}

func (it *Item) Equippable() bool { return it.Slot != "" }

func (it *Item) CanStackWith(o *Item) bool {
	return it.Wearer == nil && o.Wearer == nil && it.Base == o.Base && it.Bonus == o.Bonus && it.Kind == o.Kind
}

// Info is the tooltip text.
func (it *Item) Info() string {
	s := it.Name() + " (" + KindNames[it.Kind] + ")"
	if it.Damage > 0 {
		s += fmt.Sprintf("  dmg %d", it.Damage+it.Bonus)
	}
	if it.AC > 0 {
		s += fmt.Sprintf("  AC +%d", it.AC+it.Bonus)
	}
	if it.Heal > 0 && it.Kind == Consumable {
		s += fmt.Sprintf("  heals %d HP", it.Heal)
	}
	if it.Heal > 0 && it.Kind == Scroll {
		s += fmt.Sprintf("  restores %d SP", it.Heal)
	}
	if it.Value > 0 {
		s += fmt.Sprintf("  worth %d gold", it.Value)
	}
	if it.Wearer != nil {
		s += "  worn by " + it.Wearer.Name
	}
	return s
}

// ---------- inventory ----------

// AddItem puts an item into the shared inventory, stacking with equal unequipped items.
func (p *Party) AddItem(it *Item) {
	for i, st := range p.Inventory {
		if st[0].CanStackWith(it) {
			p.Inventory[i] = append(st, it)
			return
		}
	}
	p.Inventory = append(p.Inventory, []*Item{it})
}

// RemoveItem takes an item out of the inventory (unequipping it first).
func (p *Party) RemoveItem(it *Item) bool {
	if it.Wearer != nil {
		p.Unequip(it)
	}
	for i, st := range p.Inventory {
		for j, x := range st {
			if x == it {
				st = append(st[:j], st[j+1:]...)
				if len(st) == 0 {
					p.Inventory = append(p.Inventory[:i], p.Inventory[i+1:]...)
				} else {
					p.Inventory[i] = st
				}
				return true
			}
		}
	}
	return false
}

// FindItem returns an item whose base or full name matches, preferring unequipped ones.
func (p *Party) FindItem(name string) *Item {
	var worn *Item
	for _, st := range p.Inventory {
		for _, it := range st {
			if it.Base == name || it.Name() == name {
				if it.Wearer == nil {
					return it
				}
				worn = it
			}
		}
	}
	return worn
}

// restack re-files an item after its wearer changed, so equipped items leave their stack.
func (p *Party) restack(it *Item) {
	for i, st := range p.Inventory {
		for j, x := range st {
			if x == it {
				st = append(st[:j], st[j+1:]...)
				if len(st) == 0 {
					p.Inventory = append(p.Inventory[:i], p.Inventory[i+1:]...)
				} else {
					p.Inventory[i] = st
				}
				p.AddItem(it)
				return
			}
		}
	}
}

// slotFor picks the slot an item goes into for c: rings and amulets use the free one of their pair.
func slotFor(c *Character, it *Item) Slot {
	pairs := map[Slot]Slot{RingL: RingR, Amulet1: Amulet2}
	if other, ok := pairs[it.Slot]; ok && c.Equipped[it.Slot] != nil && c.Equipped[other] == nil {
		return other
	}
	return it.Slot
}

// Fits reports whether it can go into slot s: its own slot, or the other of a ring/amulet pair.
func Fits(it *Item, s Slot) bool {
	pair := map[Slot]Slot{RingL: RingR, RingR: RingL, Amulet1: Amulet2, Amulet2: Amulet1}
	return it.Slot != "" && (it.Slot == s || pair[it.Slot] == s)
}

// Equip puts it on c in its natural slot, displacing whatever is there. Armor and accessories add AC.
func (p *Party) Equip(c *Character, it *Item) error { return p.EquipAt(c, it, slotFor(c, it)) }

// EquipAt puts it on c in slot s.
func (p *Party) EquipAt(c *Character, it *Item, s Slot) error {
	if !Fits(it, s) {
		return fmt.Errorf("%s does not go in the %s slot", it.Name(), s)
	}
	if it.Wearer != nil {
		p.Unequip(it)
	}
	if old := c.Equipped[s]; old != nil {
		p.Unequip(old)
	}
	c.Equipped[s] = it
	it.Wearer = c
	if it.Kind != Weapon {
		c.AC += it.AC + it.Bonus
	}
	p.restack(it)
	return nil
}

func (p *Party) Unequip(it *Item) {
	c := it.Wearer
	if c == nil {
		return
	}
	for s, x := range c.Equipped {
		if x == it {
			delete(c.Equipped, s)
		}
	}
	if it.Kind != Weapon {
		c.AC -= it.AC + it.Bonus
	}
	it.Wearer = nil
	p.restack(it)
}

// Use consumes a potion or scroll on c and describes the effect.
func (p *Party) Use(c *Character, it *Item) (string, error) {
	switch it.Kind {
	case Consumable:
		before := c.HP
		c.HP = min(c.MaxHP, c.HP+it.Heal+it.Bonus)
		p.RemoveItem(it)
		return fmt.Sprintf("%s drinks the %s: +%d HP", c.Name, it.Name(), c.HP-before), nil
	case Scroll:
		before := c.SP
		c.SP = min(c.MaxSP, c.SP+it.Heal+it.Bonus)
		p.RemoveItem(it)
		return fmt.Sprintf("%s reads the %s: +%d SP", c.Name, it.Name(), c.SP-before), nil
	}
	return "", fmt.Errorf("%s cannot be used", it.Name())
}
