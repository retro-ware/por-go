package engine

// Items: names, lists and the START helpers around them.

import (
	"slices"
	"strconv"
)

// itemWord = DS:10A7 + 0x15*n.
func (e *Engine) itemWord(n byte) string { return itemWords[n] }

func (it *Item) Name() string {
	n := int(it.PName[0])
	if n > 0x28 {
		n = 0x28
	}
	return string(it.PName[1 : n+1])
}

func (it *Item) setName(s string) {
	s = cut(s, textCols)
	it.PName[0] = byte(len(s))
	copy(it.PName[1:41], s)
}

// ItemName = 00BA:0442 (char, item, x, y, readied, display): builds the
// item's shown name into its name field ("Yes"/"No" readied column, "*"
// for magic when someone detects magic, count, the known words with the
// plural "s" on the right one) and optionally prints it.
func (e *Engine) ItemName(c *Character, it *Item, x, y int, readied, display bool) {
	name := ""
	if readied {
		if it.Readied != 0 {
			name = " Yes  "
		} else {
			name = " No   "
		}
	}
	detect := false
	for _, p := range e.party {
		if detect {
			break
		}
		if e.FindAffect(p, affDetectMagic) != nil {
			detect = true
		}
	}
	if detect && (int8(it.Plus) > 0 || int8(it.Plus2) > 0 || it.Magic != 0) {
		name = cut(name+"* ", textCols)
	}
	if it.Count > 0 {
		name = cut(name+strconv.Itoa(int(it.Count))+" ", textCols)
	}
	mask := 0
	for i := 1; i <= 3; i++ {
		if it.Words[i-1] != 0 && (it.Hidden>>(3-i))&1 == 0 {
			mask += 1 << (i - 1)
		}
	}
	plural := false
	for i := 3; i >= 1; i-- {
		if (mask>>(i-1))&1 == 0 {
			continue
		}
		name = cut(name+e.itemWord(it.Words[i-1]), textCols)
		s := " "
		if it.Count >= 2 && !plural {
			typ := it.Type
			switch {
			case 1<<(i-1) == mask,
				i == 1 && mask > 4 && typ != itemOil,
				i == 2 && mask&1 == 0,
				i == 3 && typ == itemOil,
				(typ == itemArrow || typ == itemQuarrel) && it.Words[2] != 0xB1:
				s = "s "
				plural = true
			}
		}
		name = cut(name+s, textCols)
	}
	it.setName(name + e.itemStats(it.Type))
	if display {
		e.DisplayString(x, y, colLightGreen, it.Name())
	}
}

// itemStats (extra): Angband-style base stats after the name, "(1d8) "
// for weapon damage, "<6> " for the armour class it gives (the font has
// no square brackets); magic pluses stay in the name.
func (e *Engine) itemStats(typ itemType) string {
	s := ""
	if n, d := e.base(typ).Dice, e.base(typ).Sides; n > 0 && d > 0 {
		s = "(" + strconv.Itoa(int(n)) + "d" + strconv.Itoa(int(d))
		if b := int8(e.base(typ).Bonus); b > 0 {
			s += "+" + strconv.Itoa(int(b))
		} else if b < 0 {
			s += strconv.Itoa(int(b))
		}
		s += ") "
	}
	if v := int(e.base(typ).Protection); v > 0x80 {
		if v &= 0x7F; v >= 50 {
			v -= 50 // body armour and bracers count up from AC 10 = 50: plain bracers are [0], their plus in the name
		}
		if v > 0 || int(e.base(typ).Protection&0x7F) >= 50 {
			s += "[" + strconv.Itoa(v) + "] "
		}
	}
	return s
}

// RemoveItem = 00BA:1509 (char, item): unlinks the item and frees it.
func (e *Engine) RemoveItem(c *Character, it *Item) {
	if it == nil {
		e.PromptRow24(colYellow, "Nil Item pointer...")
		return
	}
	for i, x := range c.Items {
		if x == it {
			c.Items = append(c.Items[:i:i], c.Items[i+1:]...)
			return
		}
	}
	if i := slices.Index(e.pool, it); i >= 0 && e.partyMember(c) { // extra: the party inventory
		e.pool = slices.Delete(e.pool, i, i+1)
		return
	}
	e.PromptRow24(colYellow, "Tried to Lose item & couldn't find it!")
}

// AddItemCopy = 00BA:15F7 (char, item): a copy of the item at the end of
// the character's list.
func (e *Engine) AddItemCopy(c *Character, it *Item) *Item {
	n := *it
	if n.Readied == 0 && e.partyMember(c) { // extra: into the party inventory
		return e.addToPool(&n)
	}
	c.Items = append(c.Items, &n)
	// extra: a stack joins the first like stack the character has, so no
	// two exist below the 255 a stack holds (the byte count's limit)
	if n.Count == 0 || n.Spells[0] >= 2 {
		return &n
	}
	for _, p := range c.Items {
		if sameStack(p, &n) {
			e.joinItems(c, p)
			return p
		}
	}
	return &n
}

// sameStack: joinItems's test for two stacks of the same thing.
func sameStack(p, it *Item) bool {
	return p.Count > 0 && p.Spells[0] < 2 && p.Words == it.Words && p.Type == it.Type &&
		p.Plus == it.Plus && p.Plus2 == it.Plus2 && p.Magic == it.Magic && p.Weight == it.Weight
}

// BottomMessage = 00BA:1693: the string on row 24 for the message delay.
func (e *Engine) BottomMessage(s string) { e.bottomMessage(s, colLightGreen) }

// refuse (extra): BottomMessage for a refusal, the text red on the combat
// status row; outside combat the original's green on row 24.
func (e *Engine) refuse(s string) { e.bottomMessage(s, colLightRed) }

// bottomMessage: BottomMessage with the colour of the combat status row.
// Extra: in combat the message goes to the View's Log, elsewhere it is the
// window's note (a toast the remaster shows on every page; the original's
// row 24 is under the text box there, and its delay held the game).
func (e *Engine) bottomMessage(s string, colour int) {
	s = cut(s, textCols)
	e.say(s)
	if e.beginFlag == 5 && e.cmap != nil {
		e.combatSay(nil, s, true)
		e.MsgDelay()
	}
}
