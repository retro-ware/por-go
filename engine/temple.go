package engine

// The temple, ported from ovr05. ECL COMBAT with the temple flag (game
// +0x5C4) set runs Temple instead of a fight. The questions (not needed,
// pay?) are the front end's: ActTempleCure comes agreed.

// templeCures = the services of DS:011A, their prices and the "is not"
// messages of ovr05:0013 (none: always needed).
var templeCures = [9]struct {
	price uint16
	not   string
}{
	{1000, "is not blind."}, {1000, "is not Diseased."}, {100, ""}, {350, ""}, {600, ""},
	{1000, "is not poisoned."}, {5500, "is not dead."}, {3500, "is not cursed."}, {2000, "is not stoned."},
}

// diseases = DS:0114: the affects Cure Disease removes.
var diseases = [6]affectType{affHelpless, affCauseDisease, affWeakened, affSick, affMummyDisease, affRotting}

// cureNeeded: c has what cure i cures (the tests of ovr05:0249 .. 0928).
func (e *Engine) cureNeeded(c *Character, i int) bool {
	switch i {
	case 0:
		return e.FindAffect(c, affBlind) != nil
	case 1:
		for _, d := range diseases {
			if e.FindAffect(c, d) != nil {
				return true
			}
		}
		return false
	case 5:
		return e.FindAffect(c, affPoisoned) != nil
	case 6:
		return c.Status == statusDead || c.Status == statusAnimated
	case 7:
		for _, it := range c.Items {
			if it.Magic != 0 {
				return true
			}
		}
		return e.FindAffect(c, affBestowCurse) != nil
	case 8:
		return c.Status == statusStoned
	}
	return true
}

// templeView is the temple's state for the View.
func (e *Engine) templeView() *Temple {
	v := &Temple{Gold: int(e.purse.gold), Appraise: e.appraiseView()}
	c := e.selected()
	for i, t := range templeCures {
		v.Cures = append(v.Cures, Cure{Name: templeServices[i+1], Price: int(t.price), Needed: c == nil || e.cureNeeded(c, i), Not: t.not})
	}
	return v
}

// applyTemple applies a decision in the temple; done when the party leaves.
func (e *Engine) applyTemple(a Action) (done bool) {
	switch a.Kind {
	case ActClose:
		e.sellValued()
		return true
	case ActLead:
		e.actor(a.N)
	case ActMember:
		e.actor(a.N)
		e.pushSheet()
	case ActAppraise, ActKeep:
		e.applyAppraise(a)
	case ActTempleCure:
		c := e.actor(a.N)
		if a.X < 0 || a.X >= len(templeCures) || !e.pay(uint32(templeCures[a.X].price)) { // ovr05:00BF
			break
		}
		e.dasp("recover")
		e.say(c.Name() + " is cured.")
		e.templeCure(c, a.X)
	}
	return false
}

// templeCure is the work of cure i on c, paid for.
func (e *Engine) templeCure(c *Character, i int) {
	switch i {
	case 0: // ovr05:0249
		e.RemoveAffect(c, affBlind, nil)
	case 1: // ovr05:02ED
		e.rv.cure = 1
		for _, d := range diseases {
			e.RemoveAffect(c, d, nil)
		}
		e.rv.cure = 0
	case 2: // ovr05:0401: light, serious, critical
		e.Heal(c, e.Dice(1, 8), false)
	case 3:
		e.Heal(c, e.Dice(2, 8)+1, false)
	case 4:
		e.Heal(c, e.Dice(3, 8)+3, false)
	case 5: // ovr05:0751
		e.rv.cure = 1
		e.RemoveAffect(c, affPoisoned, nil)
		e.RemoveAffect(c, affSlowPoison, nil)
		e.RemoveAffect(c, affPoisonDamage, nil)
		e.rv.cure = 0
	case 6:
		e.raiseDead(c)
	case 7: // ovr05:083A: the Remove Curse spell (ovr23:2540)
		e.spellTarget = c
		e.spellRemoveCurse()
	case 8: // ovr05:0928
		if c.Status == statusStoned {
			c.Status = statusOkay
			c.Okay = 1
			c.HP = 1
		}
	}
}

// raiseDead = ovr05:0510: back with 1 hp and a point of constitution
// less; the constitution bonus to the maximum hit points shrinks with it.
func (e *Engine) raiseDead(c *Character) {
	if c.Status != statusDead && c.Status != statusAnimated {
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

// Temple = ovr05:0D09: the temple takes decisions until the party leaves
// (it puts the temple up and returns).
func (e *Engine) Temple() {
	greeting := e.paneCopy() // extra: the priests' greeting stays, as the shop
	e.beginFlag = 1
	e.DrawMainScreen()
	e.keepPane(greeting)
	e.DisplayPartyList()
	e.push(modeTemple, func(a Action) bool {
		done := e.applyTemple(a)
		e.keepPane(greeting)
		e.DisplayPartyList()
		return done
	}, func() {
		e.blank(0, 0, lastCol, promptRow) // ovr05:0C18
		e.DrawMainScreen()
		e.DisplayPartyList()
	})
}
