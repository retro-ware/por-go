package engine

import (
	"encoding/binary"
	"strconv"
)

// The party's money. The original gave every character a purse of seven
// coin kinds plus a treasure pool (ovr22); here the party shares one purse
// and every coin is counted in gold pieces plus the copper left over.

// purse is the party's money: gold pieces, copper (under 200, a gold piece),
// and gems and jewelry to appraise.
type purse struct{ gold, copper, gems, jewelry uint32 }

func (e *Engine) clearLoot() { e.loot = purse{} } // DS:67F4

func (p *purse) add(q purse) {
	p.gold += q.gold
	p.addCopper(q.copper)
	p.gems += q.gems
	p.jewelry += q.jewelry
}

// addCopper adds n copper, changing every 200 into a gold piece.
func (p *purse) addCopper(n uint32) {
	p.copper += n
	p.gold += p.copper / 200
	p.copper %= 200
}

// moneyText is gold and copper as "12 gold" / "12 gold and 50 copper".
func moneyText(gold, copper uint32) string {
	switch {
	case copper == 0:
		return strconv.Itoa(int(gold)) + " gold"
	case gold == 0:
		return strconv.Itoa(int(copper)) + " copper"
	}
	return strconv.Itoa(int(gold)) + " gold and " + strconv.Itoa(int(copper)) + " copper"
}

func (p purse) empty() bool { return p == purse{} }

// coinCopper is the worth of copper, silver, electrum, gold and platinum
// in copper pieces (DS:0D24); 200 copper make a gold piece (DS:0D30).
var coinCopper = [5]uint32{1, 10, 100, 200, 1000}

// coinsToPurse counts coins of the seven kinds (the last two gems and
// jewelry) as gold and copper.
func coinsToPurse(c [7]uint16) purse {
	var t uint32
	for i, v := range coinCopper {
		t += uint32(c[i]) * v
	}
	return purse{gold: t / 200, copper: t % 200, gems: uint32(c[5]), jewelry: uint32(c[6])}
}

// takeRecordCoins moves the coins in a character record (starting gold,
// money in an old save) into the party's purse.
func (e *Engine) takeRecordCoins(c *Character) {
	var coins [7]uint16
	for i := range coins {
		coins[i] = c.Coins[i]
		c.Coins[i] = 0
	}
	e.purse.add(coinsToPurse(coins))
}

// pay takes gold from the purse; false (and a message) when there is not enough.
func (e *Engine) pay(gold uint32) bool {
	if gold > e.purse.gold {
		e.BottomMessage("Not enough money.")
		return false
	}
	e.purse.gold -= gold
	return true
}

// purseCoin is what ECL scripts see in the selected character's coin words
// (DS:6BBB.. copper to platinum): the purse as platinum, gold and copper.
func (e *Engine) purseCoin(i int) uint16 {
	switch i {
	case 0:
		return uint16(e.purse.copper)
	case 3:
		return uint16(e.purse.gold % 5)
	case 4:
		return uint16(min(e.purse.gold/5, 0xFFFF))
	}
	return 0
}

// setPurseCoin is a script setting one of those coin words.
func (e *Engine) setPurseCoin(i int, v uint16) {
	var c [7]uint16
	for k := 0; k <= 4; k++ {
		c[k] = e.purseCoin(k)
	}
	c[i] = v
	p := coinsToPurse(c)
	e.purse.gold, e.purse.copper = p.gold, p.copper
}

// appendPurse / readPurse keep the purse after the original save game data.
func appendPurse(b []byte, p purse) []byte {
	for _, v := range []uint32{p.gold, p.gems, p.jewelry, p.copper} {
		b = binary.LittleEndian.AppendUint32(b, v)
	}
	return b
}

func readPurse(b []byte) purse {
	if len(b) < 12 {
		return purse{}
	}
	v := func(i int) uint32 { return binary.LittleEndian.Uint32(b[4*i:]) }
	p := purse{gold: v(0), gems: v(1), jewelry: v(2)}
	if len(b) >= 16 { // copper came later: older saves end after jewelry
		p.copper = v(3)
	}
	return p
}

// showMoney = ovr20:050D: the party's purse on the character sheet.
func (e *Engine) showMoney() {
	e.ClearCells(12, 7, 26, 14)
	y := 7
	for _, l := range []struct {
		name string
		n    uint32
	}{{"Gold", e.purse.gold}, {"Copper", e.purse.copper}, {"Gems", e.purse.gems}, {"Jewelry", e.purse.jewelry}} {
		if l.n == 0 {
			continue
		}
		e.DisplayString(0x14-len(l.name), y, colLightGreen, l.name)
		e.DisplayString(21, y, colLightGreen, strconv.Itoa(int(l.n)))
		y++
	}
}

// shopMoney (extra) puts the party's gold under the shop's and temple's menus.
func (e *Engine) shopMoney() {
	e.ClearBox(1, 21, winRight, winBottom)
	e.DisplayString(1, 21, colYellow, e.moneyLine())
}

func (e *Engine) moneyLine() string { return "Party money " + moneyText(e.purse.gold, e.purse.copper) }

// randomItemClass = ovr22:0FD2: d20 1-14 → 1, 15-19 → 2, 20 → 3.
func (e *Engine) randomItemClass() int {
	switch r := e.Dice(1, 20); {
	case r >= 1 && r <= 14:
		return 1
	case r >= 15 && r <= 19:
		return 2
	}
	return 3
}
