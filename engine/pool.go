package engine

import (
	"slices"
	"strconv"
)

// The party inventory (extra): the party shares one list of every item no
// member has readied, limited by slot count (a stack is one slot); item
// weight and encumbrance are gone. A member's own list (Character.Items)
// holds only their readied items. Saved games keep the format: the pool is
// written with the first member's items, unreadied, and ApplyItems moves
// unreadied items of party members into it again on loading.

// poolSlots is the party inventory's size, poolBag what a readied bag of
// holding adds (balanced against the original's 16 items a member could
// carry with only a minor encumbrance penalty).
const (
	poolSlots = 60
	poolBag   = 10
)

// partyMember: c is one of the player's party (not a monster or the clone
// the scripts add).
func (e *Engine) partyMember(c *Character) bool {
	return c.Side == sideParty && slices.Contains(e.party, c)
}

// PoolCap is the number of slots of the party inventory.
func (e *Engine) poolCap() int {
	for _, c := range e.party {
		for _, it := range c.Items {
			if it.Readied != 0 && it.Words[0] == 0xBA { // bag of holding
				return poolSlots + poolBag
			}
		}
	}
	return poolSlots
}

// carried is everything c can reach: their readied items and, for a party
// member, the party inventory.
func (e *Engine) carried(c *Character) []*Item {
	if !e.partyMember(c) {
		return c.Items
	}
	return append(c.Items[:len(c.Items):len(c.Items)], e.pool...)
}

// poolFull: the party inventory has no room for it (no stack of it to join).
func (e *Engine) poolFull(it *Item) bool {
	if len(e.pool) < e.poolCap() {
		return false
	}
	if it.Count > 0 && it.Spells[0] < 2 {
		for _, p := range e.pool {
			if sameStack(p, it) && int(p.Count)+int(it.Count) <= 0xFF {
				return false
			}
		}
	}
	return true
}

// stow moves a party member's unreadied items into the party inventory.
// Items that need no slot (potions, wands, scrolls, keys ...) are only
// readied when they have a slot on the paper doll (readyItem).
func (e *Engine) stow(c *Character) {
	keep := c.Items[:0]
	var moved []*Item
	for _, it := range c.Items {
		if it.Readied != 0 && dollSlot(e, c, it) < 0 {
			it.Readied = 0
			if it.Spells[2] > 0x7F {
				e.itemAffect(it, c, 1)
			}
		}
		if it.Readied != 0 {
			keep = append(keep, it)
		} else {
			moved = append(moved, it)
		}
	}
	clear(c.Items[len(keep):])
	c.Items = keep
	for _, it := range moved {
		e.addToPool(it)
	}
}

// addToPool puts the item in the party inventory, joining a like stack.
// It goes in even when the pool is full: callers test poolFull first where
// the player may refuse (the overflow only stops further pick-ups).
func (e *Engine) addToPool(it *Item) *Item {
	it.Readied = 0
	if it.Count > 0 && it.Spells[0] < 2 {
		for _, p := range e.pool {
			if sameStack(p, it) && int(p.Count)+int(it.Count) <= 0xFF {
				p.Count += it.Count
				return p
			}
		}
	}
	e.pool = append(e.pool, it)
	return it
}

// takeFromPool moves an item of the party inventory into c's own list.
func (e *Engine) takeFromPool(c *Character, it *Item) {
	if i := slices.Index(e.pool, it); i >= 0 {
		e.pool = slices.Delete(e.pool, i, i+1)
		c.Items = append(c.Items, it)
	}
}

// Doll slots: where a readied item shows on the paper doll. 0..12 are the
// game's own slots (Character.Slots); the bracers have none of their own.
const (
	DollWeapon = iota
	DollShield
	DollArmor
	DollGauntlets
	DollHelm
	DollBelt
	DollRobe
	DollCloak
	DollBoots
	DollRing1
	DollRing2
	DollArrows
	DollBolts
	DollBracers
	DollSlots
)

// dollSlot is the doll slot a readied item of c fills, -1 for an item
// that cannot be readied (no slot: it is used straight from the pool).
func dollSlot(e *Engine, c *Character, it *Item) int {
	switch s := int(e.base(it.Type).Slot); {
	case it.Type == itemArrow:
		return DollArrows
	case it.Type == itemQuarrel:
		return DollBolts
	case it.Type == itemBracers:
		return DollBracers
	case s <= 8:
		return s
	case s == 9:
		if c.Slots[slotRing1] == it || c.Slots[slotRing1] == nil {
			return DollRing1
		}
		return DollRing2
	}
	return -1
}

// poolText is the party inventory's fill, "12/60".
func (e *Engine) poolText() string {
	return strconv.Itoa(len(e.pool)) + "/" + strconv.Itoa(e.poolCap())
}

// Inventory is the inventory screen (ItemsMenu) as the remastered one
// draws it. Its items' Index is what an Action's N names.
type Inventory struct {
	Member int                 // index into View.Party
	Doll   [DollSlots]*InvItem // the readied items by doll slot
	Pool   []InvItem           // the party inventory
	Cap    int                 // the party inventory's slots
	Use    bool                // items can be used here (ActUse)
	Throw  bool                // combat: unreadied throwables can be thrown (ActThrow)
	Sell   bool                // a shop buys them (ActSell)
	Switch bool                // ActMember shows another member (not in combat)
}

// InvItem is one item of the inventory.
type InvItem struct {
	Index int      // its place in the member's items (Action.N)
	Name  string   // as the list shows it
	Type  byte     // base item type
	Words []string // the item's name words, known or not (for its icon)
	Count int
	Slot  bool     // it has a doll slot (it can be readied)
	Wrong bool     // the member's class cannot ready it
	Use   bool     // it has a use (a scroll, a spell of its own)
	Throw bool     // it can be thrown
	Look  []string // what it does (the shop's Look text)
	Price string   // what selling it pays
}

// inventoryView builds the snapshot of the inventory screen.
func (e *Engine) inventoryView(c *Character, items []*Item) *Inventory {
	hired := c.Control >= 0x80 && c.Okay != 0 && c.Status != statusAnimated
	v := &Inventory{Member: slices.Index(e.party, c), Cap: e.poolCap(),
		Use: c.Okay != 0 && e.area.NoMagic == 0 && (e.beginFlag == 2 || e.beginFlag == 3 || e.beginFlag == 4 ||
			e.beginFlag == 5 && c.Combat != nil && c.Combat.CanUse != 0),
		Throw:  e.beginFlag == 5 && c.Okay != 0,
		Sell:   !hired && e.beginFlag == 1,
		Switch: e.beginFlag != 5,
	}
	for i, it := range items {
		e.ItemName(c, it, 0, 0, false, false)
		v2 := sellValue(it)
		x := InvItem{Index: i, Name: it.Name(), Type: byte(it.Type), Count: int(it.Count), Slot: dollSlot(e, c, it) >= 0,
			Wrong: dollSlot(e, c, it) >= 0 && e.base(it.Type).Classes&c.ClassMask == 0,
			Use:   e.usable(it), Throw: it.Readied == 0 && e.throwableItem(it),
			Look: e.itemLines(it), Price: moneyText(v2/200, v2%200)}
		for _, w := range it.Words {
			if w != 0 {
				x.Words = append(x.Words, itemWords[w])
			}
		}
		if it.Readied != 0 {
			if s := dollSlot(e, c, it); s >= 0 && v.Doll[s] == nil {
				v.Doll[s] = &x
				continue
			}
		}
		v.Pool = append(v.Pool, x)
	}
	return v
}

// savedItems is what SaveCharacter writes to the .itm file: in a saved
// game the first member's items carry the party inventory (unreadied).
func (e *Engine) savedItems(c *Character, name string) []*Item {
	if name != "" && len(e.party) > 0 && c == e.party[0] {
		return e.carried(c)
	}
	return c.Items
}
