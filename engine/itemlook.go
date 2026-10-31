package engine

import (
	"strconv"
	"strings"
)

// Item Look (extra): the shop's Look shows what an item does under the
// game's rules, read from its base record (DS:5582 + 16*type) and the
// affect handlers that test for it.

// classBits are the base record's +0xD class bits in the DS:0D38 order
// (the DS:05D6 masks).
var classBits = [8]byte{0x02, 0x10, 0x08, 0x40, 0x80, 0x01, 0x04, 0x20}

// itemLines is the Look text for an item, at most 0x1E wide. With vs (the
// item readied in the slot it would take) the damage and armour class lines
// end with the difference to it, " (+1)" better, " (-2)" worse.
func (e *Engine) itemLines(it, vs *Item) []string {
	typ := it.Type
	b := e.base(typ)
	delta := func(v, o float64) string {
		if vs == nil || v == o {
			return ""
		}
		sign := ""
		if v > o {
			sign = "+"
		}
		return " (" + sign + strings.TrimSuffix(strconv.FormatFloat(v-o, 'f', 1, 64), ".0") + ")" // one decimal at most
	}
	dice := func(n, d byte, bonus int8) string {
		s := strconv.Itoa(int(n)) + "d" + strconv.Itoa(int(d))
		if bonus > 0 {
			s += "+"
		}
		if bonus != 0 {
			s += strconv.Itoa(int(bonus))
		}
		return s
	}
	var l []string
	add := func(s string) { l = append(l, s) }
	slot, flags := b.Slot, b.Usage
	words := func(w byte) bool { return it.Words[0] == w || it.Words[1] == w || it.Words[2] == w }
	if b.Dice > 0 && b.Sides > 0 {
		hands := "one hand"
		if b.Hands >= 2 {
			hands = "two hands"
		}
		add("Weapon, " + hands)
		add("Damage " + dice(b.Dice, b.Sides, int8(b.Bonus)) + delta(e.damageAvg(it), e.damageAvg(vs)))
		if b.LargeDice > 0 && (b.LargeDice != b.Dice || b.LargeSides != b.Sides || b.LargeBonus != b.Bonus) {
			add(" vs large " + dice(b.LargeDice, b.LargeSides, int8(b.LargeBonus)))
		}
		if r := int(b.Range); r > 1 {
			add("Range " + strconv.Itoa(r-1) + ", worse past " + strconv.Itoa((r-1)/3))
			switch n := max(int(b.Speed), 2); {
			case n == 2:
				add("1 shot per round")
			case n%2 == 0:
				add(strconv.Itoa(n/2) + " shots per round")
			default:
				add(strconv.Itoa(n) + " shots per 2 rounds")
			}
		}
		switch {
		case typ == itemArrow || typ == itemQuarrel: // before the flags: quarrels carry the crossbows' 0x80
			add("Ammunition")
		case flags&0x14 == 0x14:
			add("Thrown, or used in melee")
		case flags&0x10 != 0:
			add("Thrown, used up; not with")
			add("a foe adjacent")
		case flags&0x80 != 0:
			add("Needs quarrels")
		case flags&1 != 0 && flags&8 != 0:
			add("Needs arrows")
		}
		if flags&4 != 0 {
			add("Strength adds to hit, damage")
		} else if flags&2 != 0 {
			add("Dexterity adds to hit")
		}
	}
	if v := b.acBetter(); b.acPoints() > 0 {
		switch {
		case slot == slotArmor:
			add("Armor, AC " + strconv.Itoa(10-v) + delta(e.protection(it), e.protection(vs)))
			switch w := it.Weight; {
			case w > 0x18F:
				add("Movement 6")
			case w > 0x96:
				add("Movement 9")
			}
		case slot == slotShield:
			add("Shield, AC " + strconv.Itoa(v) + " better" + delta(e.protection(it), e.protection(vs)))
		default: // bracers: on the armour's scale and in its place, the better one counts (itemAC's ac[4])
			add("AC " + strconv.Itoa(10-v-int(e.knownPlus(it))) + ", as armor" + delta(e.protection(it), e.protection(vs)))
			add("Worn in place of armor")
		}
	}
	switch {
	case typ == itemHolyWater:
		add("Hurts undead: 1d6+1 a hit")
	case typ == itemOil:
		add("Burns some monsters: 3d8")
	case words(0x76):
		add("Readied: turns gaze attacks")
		add("back, wards some monsters")
	case words(0x98) && words(0xFC):
		add("Readied: wards some monsters")
	case slot > 0xA && slot < 0xE:
		add("Scroll: read to cast its")
		add("spells, once each")
	case slot == 0xA && b.Hands == 0:
		add("Treasure: worth selling")
	}
	if words(0xB1) && b.Dice > 0 {
		add("Silver: hurts monsters that")
		add("ignore plain weapons")
	}
	if it.Hidden == 0 && int8(it.Plus) > 0 && (b.Dice > 0 || b.acPoints() > 0) {
		add("Magic +" + strconv.Itoa(int(int8(it.Plus))))
	}
	add("Price " + strconv.Itoa(int(it.Price))) // extra: no weight (the party inventory counts slots)
	if slot != 0xA || b.Hands != 0 {
		var who []string
		for k, m := range classBits {
			if b.Classes&m != 0 {
				who = append(who, strings.TrimSpace(classNames[k]))
			}
		}
		switch {
		case len(who) == 0:
			add("No class can use it")
		case len(who) >= 7:
			add("Most classes can use it")
		default:
			s := "For"
			for _, w := range who {
				if len(s)+1+len(w) > 0x1E {
					add(s)
					s = "   "
				}
				s += " " + w
			}
			add(s)
		}
	}
	return l
}

// lookItem (extra) shows the item's Look text in a box until a key.
func (e *Engine) lookItem(it *Item) {
	if it == nil {
		return
	}
	e.ItemName(nil, it, 0, 0, false, false)
	l := e.itemLines(it, nil)
	y1 := 4
	y2 := min(y1+len(l)+3, 0x16)
	e.DrawBoxTitled(1, y1, winRight, y2, 0, colWhite, "")
	e.DisplayString(3, y1+1, colLightCyan, cut(it.Name(), 0x22))
	for i, s := range l {
		if y1+3+i < y2 {
			e.DisplayString(3, y1+3+i, colLightGreen, s)
		}
	}
	e.PromptRow24(colLightMagenta, "Press any key")
}

// damageAvg is the item's average damage (against small foes) with its
// known magic plus, for the Look's comparison; 0 for nil.
func (e *Engine) damageAvg(it *Item) float64 {
	if it == nil {
		return 0
	}
	b := e.base(it.Type)
	return float64(b.Dice)*float64(b.Sides+1)/2 + float64(int8(b.Bonus)) + e.knownPlus(it)
}

// protection is the armour class points the item is worth with its known
// magic plus (armour's and bracers' over AC 10), for the Look's comparison; 0 for nil.
func (e *Engine) protection(it *Item) float64 {
	if it == nil {
		return 0
	}
	return float64(e.base(it.Type).acBetter()) + e.knownPlus(it)
}

// knownPlus is the item's magic plus when identified, else 0.
func (e *Engine) knownPlus(it *Item) float64 {
	if it.Hidden != 0 {
		return 0
	}
	return float64(int8(it.Plus))
}

// readiedLike (extra) is the item c has readied in the doll slot it would
// take, for the Look's comparison: nil for a readied item or one without a slot.
func (e *Engine) readiedLike(c *Character, it *Item) *Item {
	s := dollSlot(e, c, it)
	if it.Readied != 0 || s < 0 {
		return nil
	}
	for _, o := range c.Items {
		if o.Readied != 0 && o != it && dollSlot(e, c, o) == s {
			return o
		}
	}
	return nil
}
