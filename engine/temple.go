package engine

// The temple, ported from ovr05. ECL COMBAT with the temple flag (game
// +0x5C4) set runs Temple instead of a fight.

import "strconv"

// templeNotNeeded = ovr05:0013 (message): "<name> is not ..." and the
// question whether to pay anyway.
func (e *Engine) templeNotNeeded(msg string) bool {
	c := e.selected()
	e.CharMessage(c, msg, false)
	k := e.YesNo(colLightMagenta, colLightGreen, colWhite, "cast cure anyway: ")
	e.ClearMessageArea()
	return k
}

// templePay = ovr05:00BF (cure, price): the price from the party's gold.
// True when paid.
func (e *Engine) templePay(cure string, price uint16) bool {
	e.DisplayTextBlock(1, msgTop, winRight, winBottom, colLightGreen, true,
		cut(cure+" will only cost "+strconv.Itoa(int(price))+" gold pieces.", 0xFF))

	k := e.YesNo(colLightMagenta, colWhite, colLightGreen, "pay for cure ") && e.pay(uint32(price))
	if k {
		e.ClearMessageArea()
		c := e.selected()
		e.CharMessage(c, "is cured.", true)
	}
	return k
}

// templeCure asks "not needed" when needed is false, then for the price.
func (e *Engine) templeCure(needed bool, notMsg, cure string, price uint16) bool {
	return (needed || e.templeNotNeeded(notMsg)) && e.templePay(cure, price)
}

// cureBlindness = ovr05:0249.
func (e *Engine) cureBlindness() {
	c := e.selected()
	if e.templeCure(e.FindAffect(c, affBlind) != nil, "is not blind.", "Cure Blindness", 1000) {
		e.RemoveAffect(c, affBlind, nil)
	}
}

// diseases = DS:0114: the affects Cure Disease removes.
var diseases = [6]affectType{affHelpless, affCauseDisease, affWeakened, affSick, affMummyDisease, affRotting}

// cureDisease = ovr05:02ED.
func (e *Engine) cureDisease() {
	c := e.selected()
	ill := false
	for _, d := range diseases {
		if e.FindAffect(c, d) != nil {
			ill = true
		}
	}
	if e.templeCure(ill, "is not Diseased.", "Cure Disease", 1000) {
		e.rv.cure = 1
		for _, d := range diseases {
			e.RemoveAffect(c, d, nil)
		}
		e.rv.cure = 0
	}
}

// cureWounds = ovr05:0401 (kind): light, serious or critical.
func (e *Engine) cureWounds(kind int) {
	c := e.selected()
	switch kind {
	case 1:
		if e.templePay("Cure Light Wounds", 100) {
			e.Heal(c, e.Dice(1, 8), false)
		}
	case 2:
		if e.templePay("Cure Serious Wounds", 350) {
			e.Heal(c, e.Dice(2, 8)+1, false)
		}
	case 3:
		if e.templePay("Cure Critical Wounds", 600) {
			e.Heal(c, e.Dice(3, 8)+3, false)
		}
	}
}

// neutralizePoison = ovr05:0751.
func (e *Engine) neutralizePoison() {
	c := e.selected()
	if e.templeCure(e.FindAffect(c, affPoisoned) != nil, "is not poisoned.", "Neutralize Poison", 1000) {
		e.rv.cure = 1
		e.RemoveAffect(c, affPoisoned, nil)
		e.RemoveAffect(c, affSlowPoison, nil)
		e.RemoveAffect(c, affPoisonDamage, nil)
		e.rv.cure = 0
	}
}

// raiseDead = ovr05:0510: back with 1 hp and a point of constitution
// less; the constitution bonus to the maximum hit points shrinks with it.
func (e *Engine) raiseDead() {
	c := e.selected()
	dead := c.Status == statusDead || c.Status == statusAnimated
	if !e.templeCure(dead, "is not dead.", "Raise Dead", 5500) || !dead {
		return
	}
	e.rv.cure = 1
	e.RemoveAffect(c, affAnimated, nil)
	e.RemoveAffect(c, affPoisoned, nil)
	e.rv.cure = 0
	c.HP = 1
	c.Status = statusOkay
	c.Okay = 1
	if c.Stats[statCon] > 0 {
		c.Stats[statCon]--
	}
	var excess byte
	if c.MaxHP > c.HPRoll {
		excess = c.MaxHP - c.HPRoll
	}
	if c.Stats[statCon] < 0xE {
		return
	}
	var levels byte
	for i := 0; i <= 7; i++ {
		lvl := int8(c.Levels[i])
		if lvl <= 0 {
			continue
		}
		switch {
		case i == 2: // fighters get the exceptional bonus
			levels += byte(int(lvl) * (int(c.Stats[statCon]) - 0xE))
		case c.Stats[statCon] > 0xF:
			levels += byte(lvl) * 2
		default:
			levels += byte(lvl)
		}
	}
	if levels > 0 {
		excess = byte(int(excess) / int(levels))
	}
	if c.Stats[statCon] < 0x11 || fighterKind(c) {
		c.MaxHP -= excess
	}
}

// removeCurse = ovr05:083A: cursed items or the curse affect; the work is
// the Remove Curse spell (ovr23:2540) on the selected character.
func (e *Engine) removeCurse() {
	c := e.selected()
	cursed := false
	for _, it := range c.Items {
		if it.Magic != 0 {
			cursed = true
			break
		}
	}
	if !cursed && e.FindAffect(c, affBestowCurse) != nil {
		cursed = true
	}
	if e.templeCure(cursed, "is not cursed.", "Remove Curse", 3500) {
		e.spellTarget = c
		e.spellRemoveCurse()
	}
}

// stoneToFlesh = ovr05:0928.
func (e *Engine) stoneToFlesh() {
	c := e.selected()
	if e.templeCure(c.Status == statusStoned, "is not stoned.", "Stone to Flesh", 2000) && c.Status == statusStoned {
		c.Status = statusOkay
		c.Okay = 1
		c.HP = 1
	}
}

// templeHeal = ovr05:09D7: the list of services (DS:011A).
func (e *Engine) templeHeal() {
	list := make([]*ListNode, 10)
	for i := range list {
		list[i] = &ListNode{Text: cut(templeServices[i+1], textCols)}
	}
	e.ClearBox(0, promptRow, lastCol, promptRow)
	redraw := true
	e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colWhite, "")
	e.DrawBoxTitled(1, 1, winRight, 1, 0, colWhite, "")
	index := 0
	for done := false; !done; {
		e.DisplayString(1, 1, colWhite, cut(e.selected().Name()+", how can we help you?", textCols))
		e.DrawBoxTitled(1, msgTop, winRight, winBottom, 0, colWhite, "")
		var sel *ListNode
		const heal Choice = 0
		switch e.ListMenu("", Bar{{"Heal", heal}, {"Exit", PickExit}}, colLightMagenta, colLightGreen, colWhite, 2, 4, 38, 15, list, false, &redraw, &index, &sel).ID {
		case heal, PickEnter:
			switch index {
			case 0:
				e.cureBlindness()
			case 1:
				e.cureDisease()
			case 2, 3, 4:
				e.cureWounds(index - 1)
			case 5:
				e.neutralizePoison()
			case 6:
				e.raiseDead()
			case 7:
				e.removeCurse()
			case 8:
				e.stoneToFlesh()
			case 9:
				done = true
			}
		case NoPick:
			done = true
		}
	}
	e.ClearBox(0, 0, lastCol, 24) // ovr05:0C18
	e.DrawMainScreen()
	e.DisplayPartyList()
}

// Temple = ovr05:0D09.
func (e *Engine) Temple() {
	e.beginFlag = 1
	e.DrawMainScreen()
	e.DisplayPartyList()
	for leave := false; !leave; {
		e.shopMoney()
		const (
			heal Choice = iota
			view
			appraise
			exit
		)
		p := e.DisplayInput("", Bar{{"Heal", heal}, {"View", view}, {"Appraise", appraise}, {"Exit", exit}}, colLightMagenta, colLightGreen, colWhite, true, false)
		switch p.ID {
		case heal:
			e.templeHeal()
		case view:
			e.ViewCharacter()
		case appraise:
			if e.Appraise() {
				e.DrawMainScreen()
			}
		case exit:
			leave = true
		case PickMove:
			if homeEnd(p) {
				e.partyKeypad(p)
			}
		}
		e.DisplayPartyList()
	}
}
