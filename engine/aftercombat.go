package engine

import (
	"strconv"
	"strings"
)

// After combat, ported from ovr06: the fight's outcome, experience, the
// hired NPCs' share and the treasure screen. ECL COMBAT without monsters
// runs this directly, which is how scripts hand out treasure.

// afterCombat holds the ovr06 globals.
type afterCombat struct {
	monsters bool  // DS:443A: there were monsters to beat
	fled     bool  // DS:443B
	idle     int   // DS:84D5: members who get no experience
	exp      int32 // DS:84D6
	won      bool  // DS:84DA
}

func (e *Engine) duel() bool { return e.run.duelMode != 0 }

// combatParty returns the party members before the first combat monster
// (Combat +0x13 = 1).
func (e *Engine) combatParty() []*Character {
	for i, c := range e.party {
		if c.Combat != nil && c.Combat.Extra == 1 {
			return e.party[:i]
		}
	}
	return e.party
}

// computeExp = ovr06:0000 (var exp): the beaten monsters' worth, their
// money and (some of) their items go to the treasure, and the treasure's
// worth counts too; the sum is split over the members that earn it.
func (e *Engine) computeExp() int32 {
	sel := e.selected()
	if e.duel() && e.game.Duel != 0 {
		return int32(int16(int8(sel.MaxLevel)) * 100)
	}
	var total int32
	taken := 0
	for _, c := range e.party {
		if c.Side != sideMonsters || c.Status == statusRunning {
			continue
		}
		e.ac.monsters = true
		var coins [7]uint16
		for i := range coins {
			coins[i] = c.Coins[i]
		}
		e.loot.add(coinsToPurse(coins))
		// the byte product is a 16-bit mul, sign-extended
		total += int32(c.ExpValue) + int32(int16(uint16(c.HPRoll)*uint16(c.ExpPerHP)))
		if e.game.NoLoot == 1 {
			continue
		}
		for _, it := range c.Items {
			if it.Price != 0 || taken < 8 && e.Dice(1, 10) <= 3 {
				taken++
				e.ItemName(c, it, 0, 0, false, false)
				cp := *it
				cp.Readied = 0
				e.run.treasure = append([]*Item{&cp}, e.run.treasure...)
			}
		}
	}
	total += int32(e.loot.gold + e.loot.gems*250 + e.loot.jewelry*2200)
	// (the original stops at DS:5D9A, an item the combat code marks; it is
	// never set outside a fight)
	for _, it := range e.run.treasure {
		if p := int8(it.Plus); p > 0 {
			total += int32(p) * 400
		}
	}
	// ponytail: the original divides by zero when d is 0
	if d := int32(uint16(e.game.PartyCount - uint16(e.ac.idle))); d > 0 {
		total /= d
	}
	return total
}

// giveExp = ovr06:0355 (exp): every member still standing gets it, with
// a tenth more for a high prime requisite; multi-class characters split it.
func (e *Engine) giveExp(exp int32) {
	for _, c := range e.party {
		if c.Okay == 0 || c.Status == statusAnimated {
			continue
		}
		g := exp
		prime := -1
		switch c.Class {
		case clsCleric:
			prime = statWis
		case clsFighter:
			prime = statStr
		case clsMage:
			prime = statInt
		case clsThief:
			prime = statDex
		case clsPaladin, clsRanger: // CotAB: paladin str+wis, ranger str+int+wis over 15
			if c.Stats[statStr] > 15 && c.Stats[statWis] > 15 && (c.Class == clsPaladin || c.Stats[statInt] > 15) {
				g += g / 10
			}
		case clsClericFighter, clsClericRanger, clsClericMage, clsClericThief, clsFighterMage, clsFighterThief, clsMageThief:
			g = exp / 2
		case clsClericFighterMage, clsFighterMageThief:
			g = exp / 3
		}
		if prime >= 0 && c.Stats[prime] > 15 {
			g += g / 10
		}
		c.Exp = uint32(int32(c.Exp) + g)
	}
}

// combatResult = ovr06:04C9: who won, experience, and what happens to the
// fallen. quit (DS:49FF) is left set when the whole party is lost.
func (e *Engine) combatResult() {
	e.ac.idle = 0
	e.run.quit = true
	e.ac.fled = false
	for _, c := range e.combatParty() {
		if c.Status == statusRunning {
			e.ac.fled = true
		}
	}
	if e.duel() {
		e.run.quit = false
	}
	e.ac.won = false
	if !e.duel() {
		for _, c := range e.combatParty() {
			st := c.Status
			if (st == statusRunning || st == statusAnimated || st == statusOkay || e.game.Duel == 1) &&
				c.Side == sideParty && c.Control < 0x80 {
				e.run.quit = false
			}
			if st == statusAnimated || st == statusOkay {
				e.ac.won = true
				e.ac.fled = false
			}
			if c.Okay == 0 || st == statusAnimated {
				e.ac.idle++
			}
		}
		if e.ac.won {
			e.ac.exp = e.computeExp()
			e.giveExp(e.ac.exp)
		}
		if !e.run.quit {
			if !e.ac.fled {
				for _, c := range e.combatParty() {
					switch st := c.Status; {
					case st == statusRunning:
						c.Status, c.Okay = statusOkay, 1
					case st == statusDying:
						c.Status = statusUnconscious
					case st == statusUnconscious && c.HP > 0:
						c.Status, c.Okay = statusOkay, 1
					}
				}
			} else {
				e.game.CombatEnd = 0x81
				for i := 0; i < len(e.combatParty()); {
					c := e.party[i]
					if c.Status == statusRunning {
						c.Status, c.Okay = statusOkay, 1
						i++
						continue
					}
					e.partySel = i
					e.RemoveSelected(false, true)
				}
			}
		} else {
			for n := len(e.combatParty()); n > 0; n-- {
				e.partySel = 0
				e.RemoveSelected(false, true)
			}
			e.game.PartyCount = 0
		}
		return
	}
	for _, c := range e.party {
		if c.Okay == 1 && c.Status == statusOkay && c.Side != sideMonsters {
			e.ac.won = true
			e.ac.exp = e.computeExp()
			e.giveExp(e.ac.exp)
		}
	}
	for _, c := range e.party {
		switch c.Status {
		case statusOkay, statusAnimated:
			c.Okay = 1
		case statusDying:
			c.Status = statusUnconscious
		}
	}
}

// cleanupCombatants = ovr06:11AC: the monsters leave the party list
// (game +0x590 counts the ones that were beaten); the members drop their
// combat blocks.
func (e *Engine) cleanupCombatants() {
	e.game.Beaten = 0
	for i := 0; i < len(e.party); {
		c := e.party[i]
		monster := c.Combat != nil && c.Combat.Extra == 1
		if monster || c.Side == sideMonsters {
			if c.Okay != 1 {
				e.game.Beaten = e.game.Beaten + 1
			}
			e.partySel = i
			e.RemoveSelected(monster, true)
			continue
		}
		c.Combat = nil
		i++
	}
	e.run.monCount = 0
	e.partySel = 0
}

// hiredShare = ovr06:12E8: hired NPCs (+0x84 > 0x7F) take their shares
// (+0x85 & 7) of every coin before the party gets to the treasure.
func (e *Engine) hiredShare() {
	shares, parts := 0, 0
	for _, c := range e.party {
		if c.Control > 0x7F && c.Status == statusOkay {
			shares += int(c.Share & 7)
			parts += int(c.Share & 7)
		} else {
			parts++
		}
	}
	if shares == 0 || e.loot.empty() {
		return
	}
	for _, n := range []*uint32{&e.loot.gold, &e.loot.copper, &e.loot.gems, &e.loot.jewelry} {
		*n -= *n / uint32(parts) * uint32(shares)
	}
	e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colWhite, "")
	k := 0
	for _, c := range e.party {
		if c.Control > 0x7F && c.Status == statusOkay && c.Share > 0 {
			e.DisplayTextBlock(5, 5+2*k, 34, winBottom, colLightGreen, true, c.Name()+" takes and hides his share.")
			k++
		}
	}
	e.DisplayInput("", pressEnter, colWhite, colWhite, colWhite, true, false)
}

// showResult = ovr06:0916 (exp).
func (e *Engine) showResult(exp int32) {
	e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colWhite, "")
	switch {
	case e.ac.monsters || e.duel():
		switch {
		case e.duel() && !e.ac.won:
			e.DisplayString(1, 3, colLightGreen, "You have lost the duel.")
			exp = 0
		case e.duel():
			e.DisplayString(1, 3, colLightGreen, "You have won the duel.")
		default:
			e.DisplayString(1, 3, colLightGreen, "The party has won.")
		}
	case e.ac.fled:
		e.DisplayString(1, 3, colLightGreen, "The party has fled.")
		exp = 0
	default:
		e.DisplayString(1, 3, colLightGreen, "The party has found Treasure!")
	}
	s := "Each character receives "
	if e.duel() {
		s = "The duelist receives "
	}
	e.DisplayString(1, 5, colLightGreen, cut(s+strconv.Itoa(int(exp)), 0xFF))
	e.DisplayString(1, 7, colLightGreen, "experience points.")
	if l := e.loot; !l.empty() {
		var got []string
		if l.gold > 0 || l.copper > 0 {
			got = append(got, moneyText(l.gold, l.copper))
		}
		for _, p := range []struct {
			n    uint32
			name string
		}{{l.gems, " gems"}, {l.jewelry, " jewelry"}} {
			if p.n > 0 {
				got = append(got, strconv.Itoa(int(p.n))+p.name)
			}
		}
		e.DisplayTextBlock(1, 9, winRight, winBottom, colLightGreen, true, "The party takes "+strings.Join(got, ", ")+".")
	}
	y := 0xC // extra: who came out of the fight dead or worse, or with harmful effects on them
	for _, c := range e.party {
		var what []string
		if st := e.statusName(c); c.Status != statusOkay && c.Status != statusRunning && st != "" {
			what = append(what, st)
		}
		for _, a := range c.Affects {
			if name := e.harmfulAffect(a.Type); name != "" {
				what = append(what, name)
			}
		}
		if len(what) > 0 && y <= 0x15 {
			e.DisplayString(1, y, colLightCyan, cut(c.Name(), 15))
			e.DisplayString(18, y, colLightRed, cut(strings.Join(what, ", "), winRight-18))
			y++
		}
	}
	e.DisplayInput("", pressEnter, colWhite, colWhite, colWhite, true, false)
}

// lootTakeItem is the Take of the loot list.
const lootTakeItem Choice = 0

// lootList = ovr06:0B14 (var index, var item): the treasure in a "Loot" box.
func (e *Engine) lootList(index *int, item **Item) Pick {
	e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colWhite, "Loot")
	e.snap.tallRow = 22 // the remaster fills a 4:3 window: the list's last row repeats while it is blank, else the row under the name
	if len(e.run.treasure) >= 19 {
		e.snap.tallRow = 3
	}
	if c := e.selected(); c != nil { // extra: who takes (F1-F8, a portrait change it)
		e.displayName(c, 2, 2, false)
		w := e.poolText() // extra: the party inventory's slots, against the window's right edge
		e.DisplayString(winRight-len("Items "+w)+1, 2, colWhite, "Items ")
		e.DisplayString(winRight-len(w)+1, 2, colLightGreen, w)
	}
	list := make([]*ListNode, len(e.run.treasure))
	for i, it := range e.run.treasure {
		e.ItemName(nil, it, 0, 0, false, false)
		list[i] = &ListNode{Text: it.Name()}
	}
	redraw := true
	var sel *ListNode
	e.ext.buying = true
	key := e.ListMenu("Items: ", Bar{{"Take", lootTakeItem}}, colLightMagenta, colLightGreen, colWhite, 1, 4, 38, 22, list, true, &redraw, index, &sel)
	e.ext.buying = false
	*item = nil
	for i, n := range list {
		if n == sel {
			*item = e.run.treasure[i]
		}
	}
	return key
}

// lootTake = ovr06:0C06: take items until Exit or nothing is left.
func (e *Engine) lootTake() {
	index := 0
	for len(e.run.treasure) > 0 {
		var it *Item
		key := e.lootList(&index, &it)
		if key.ID == PickRelist {
			continue // F1-F8 picked another taker
		}
		if key.ID != lootTakeItem && key.ID != PickEnter || it == nil {
			break
		}
		if !e.buyItem(it) {
			for i, t := range e.run.treasure {
				if t == it {
					e.run.treasure = append(e.run.treasure[:i], e.run.treasure[i+1:]...)
					break
				}
			}
		}
	}
	e.DrawMainScreen()
}

// lootMenu = ovr06:0ECD: the treasure screen.
func (e *Engine) lootMenu() {
	e.DrawMainScreen()
	for {
		items := len(e.run.treasure) > 0
		const (
			view Choice = iota
			take
			detectIt
			exit
		)
		withDetect := false
		detect := byte(0)
		if items {
			if c := e.selected(); c != nil {
				for i := 0; i <= 0x14; i++ {
					if s := c.Memorized[i]; s == spDetectMagicCl || s == spDetectMagicMU {
						detect = s
						withDetect = true
						break
					}
				}
			}
		}
		menu := Bar{{"View", view}}
		if items {
			menu = append(menu, Entry{"Take", take})
			if withDetect {
				menu = append(menu, Entry{"Detect", detectIt})
			}
		}
		menu = append(menu, Entry{"Exit", exit})
		key, again := e.campInput("", menu) // ovr06:101F: anim; extra: F1-F8 pick the member too
		if again {
			continue
		}
		switch key {
		case view:
			e.ViewCharacter()
		case take:
			e.lootTake()
		case detectIt:
			quit := false
			e.CastSpell(detect, 0, 0, &quit)
		case exit, NoPick:
			if len(e.run.treasure) == 0 {
				return
			}
			e.DisplayTextBlock(1, msgTop, winRight, winBottom, colLightGreen, true, "There is still treasure left.  ")
			e.DisplayTextBlock(1, msgTop, winRight, winBottom, colWhite, false, "Do you want to go back and claim your treasure?")
			if e.MenuInput(false, colWhite, colLightGreen, colLightMagenta, "~Yes ~No", "") == 1 {
				return
			}
			e.ClearBox(1, msgTop, winRight, winBottom)
		}
	}
}

// AfterCombat = ovr06:1527.
func (e *Engine) AfterCombat() {
	e.game.CombatEnd = 0
	e.ac.monsters = false
	e.beginFlag = 6
	e.combatResult()
	e.cleanupCombatants()
	if !e.run.quit || e.duel() {
		e.hiredShare()
		e.showResult(e.ac.exp)
		e.purse.add(e.loot)
		e.loot = purse{}
		e.lootMenu()
		e.run.treasure = nil
	} else {
		e.game.CombatEnd = 0x80
		e.Music(5) // Amiga 0x4602E
		e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colWhite, "The END!")
		e.msgCol, e.msgRow = 2, 6
		e.DisplayTextBlock(2, 5, 37, winBottom, colLightGreen, true, "The monsters rejoice for the party has been destroyed")
		e.PromptRow24(colLightMagenta, "Press any key to continue")
	}
	g := &e.game
	g.MonsterHit, g.PartyHit, g.PartyMove, g.NoLoot, g.Duel = 0, 0, 0, 0, 0
}

// harmfulAffect (extra) names affect t when it hurts its bearer: one a
// hostile spell gives (spell table +7 is 0, as Aim reads it) or one of the
// named harms; "" otherwise.
func (e *Engine) harmfulAffect(t affectType) string {
	switch t {
	case affHelpless, affSick, affMummyDisease, affRepulsed, affPoisoned:
		return affectNames[t]
	}
	if listedAffect(t) {
		for s := 1; s <= lastSpell; s++ {
			if affectType(spellTable[s][10]) == t && spellTable[s][7] == 0 {
				return e.spellName(s)
			}
		}
	}
	return ""
}
