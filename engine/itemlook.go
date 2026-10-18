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

// itemLines is the Look text for an item, at most 0x1E wide.
func (e *Engine) itemLines(it *Item) []string {
	typ := it.Type
	b := e.base(typ)
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
		add("Damage " + dice(b.Dice, b.Sides, int8(b.Bonus)))
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
		case flags&0x14 == 0x14:
			add("Thrown, or used in melee")
		case flags&0x10 != 0:
			add("Thrown, used up")
		case flags&0x80 != 0:
			add("Needs quarrels")
		case flags&1 != 0 && flags&8 != 0:
			add("Needs arrows")
		case typ == itemArrow || typ == itemQuarrel:
			add("Ammunition")
		}
		if flags&4 != 0 {
			add("Strength adds to hit, damage")
		} else if flags&2 != 0 {
			add("Dexterity adds to hit")
		}
	}
	if v := int(b.Protection); v > 0x80 {
		v &= 0x7F
		switch {
		case slot == 2:
			add("Armor, AC " + strconv.Itoa(10-(v-50)))
			switch w := it.Weight; {
			case w > 0x18F:
				add("Movement 6")
			case w > 0x96:
				add("Movement 9")
			}
		case slot == 1:
			add("Shield, AC " + strconv.Itoa(v) + " better")
		case v > 0:
			add("AC " + strconv.Itoa(v) + " better")
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
	if it.Hidden == 0 && int8(it.Plus) > 0 && (b.Dice > 0 || b.Protection > 0x80) {
		add("Magic +" + strconv.Itoa(int(int8(it.Plus))))
	}
	add("Weight " + strconv.Itoa(int(it.Weight)) + ", price " + strconv.Itoa(int(it.Price)))
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
	l := e.itemLines(it)
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
