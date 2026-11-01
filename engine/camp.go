package engine

import (
	"bytes"
	"slices"
	"strconv"
)

// Encamp, ported from ovr16 (camp menu, magic: cast / memorize / scribe /
// display, alter: order / drop / speed / pics) and the rest of ovr21
// (resting, healing, study).

// campState holds the ovr16/ovr21 globals.
type campState struct {
	rest     [7]int                  // DS:6DC2: rest time left, clock units (DS:6DC4 mins, 6DC6 tens, 6DC8 hours, 6DCA days)
	heal     int                     // DS:6DD8: five-minute steps toward the daily healing
	study    [maxCombatants + 1]byte // DS:6DCF: five-minute steps until member i's next spell is learned
	interval int                     // DS:49FB: steps since the last encounter roll

	scriber map[*Item][3]*Character // extra: who chose to scribe each scroll spell (the party inventory is shared)

	interrupted, rested bool // extra: a rest was interrupted (camp closes); one just ended (camp's actRedraw draws nothing)
}

func (e *Engine) spellLevel(sp byte) byte { return spellTable[sp&0x7F][1] }
func (e *Engine) spellClass(sp byte) byte { return spellTable[sp&0x7F][0] }

// studyTime = ovr16:0000 (char): minutes of study for the spells waiting
// to be memorised (+0x80) or scribed; +0x2C gets the base hours (4, or 6
// for spells above 2nd level).
func (e *Engine) studyTime(c *Character) int {
	maxMem, maxScr, sumMem, sumScr := byte(0), byte(0), 0, 0
	for i := 0; i <= 0x14; i++ {
		if s := c.Memorized[i]; s > 0x7F {
			l := e.spellLevel(s)
			if l > maxMem {
				maxMem = l
			}
			sumMem += int(l)
		}
	}
	for _, it := range e.carried(c) {
		if !e.isScroll(it) {
			continue
		}
		for k := 1; k <= 3; k++ {
			if s := it.Spells[k-1]; s > 0x7F && e.scribesIt(c, it, k-1) {
				l := e.spellLevel(s)
				if l > maxScr {
					maxScr = l
				}
				sumScr += int(l)
			}
		}
	}
	base := 0
	if sumMem > 0 || sumScr > 0 {
		base = 4
	}
	if maxMem > 2 || maxScr > 2 {
		base = 6
	}
	c.Study = byte(base)
	return base*60 + sumMem*15 + sumScr*15
}

// clearMemorize = ovr16:0159 (char): forget the spells still to be memorised.
func clearMemorize(c *Character) {
	for i := 0; i <= 0x14; i++ {
		if c.Memorized[i] > 0x7F {
			c.Memorized[i] = 0
		}
	}
	c.Study = 0
}

// clearScribe = ovr16:019C (char): scroll spells are no longer being scribed.
func (e *Engine) clearScribe(c *Character) {
	for _, it := range e.carried(c) {
		if e.isScroll(it) {
			for k := 1; k <= 3; k++ {
				it.Spells[k-1] &= 0x7F
			}
		}
	}
}

// clearStudy = ovr16:0214: nobody studies any more.
func (e *Engine) clearStudy() {
	e.camp.scriber = nil
	for _, c := range e.party {
		clearMemorize(c)
		e.clearScribe(c)
	}
}

// canStudy = ovr16:036C (mode 1 cast, 2 memorize, 3 scribe): area +0x1CA
// forbids casting; the dead (status 1) and the fallen cannot study.
func (e *Engine) canStudy(mode int) bool {
	msg := ""
	c := e.selected()
	switch {
	case mode == 1 && e.area.NoMagic > 0:
		msg = "cannot cast spells here"
	case c.Status == statusAnimated || c.Okay == 0: // extra: shorter than the original's "is in no condition to cast any spells", so it fits a note
		msg = [...]string{"cannot cast now", "cannot memorize now", "cannot scribe now"}[mode-1]
	}
	if msg == "" {
		return true
	}
	e.CharMessage(c, msg, true)
	return false
}

// CastMenu = ovr16:04A1: cast memorised spells until none is picked.
func (e *Engine) CastMenu() {
	e.spellCaster = nil // DS:6DB5
	if !e.canStudy(1) {
		return
	}
	if !e.buildSpellList(0) {
		e.CharMessage(e.selected(), "has no spells memorized", true)
		return
	}
	// extra: the pick list (View.Ask (AskSpell), as the combat's Cast) instead of the
	// spell page, and one cast: the original lists again until Exit
	e.pickSpell(func(spell byte) {
		if spell == 0 { // the main screen is still up: no message box to redraw (the original's spell page had replaced it)
			return
		}
		e.castAsks(spell, 0, false, func() {
			var quit bool
			e.CastSpell(spell, 0, 1, &quit)
			e.DrawMainScreen()
		})
	})
}

// partyStudy: the longest study time of the party in minutes (studyTime
// also sets each member's study hours, which Rest counts down).
// scribesIt (extra): spell k of scroll it is being scribed by c, not by
// another member sharing the scroll.
func (e *Engine) scribesIt(c *Character, it *Item, k int) bool {
	by := e.camp.scriber[it][k]
	return it.Spells[k] > 0x80 && (by == nil || by == c)
}

// markScribe (extra): c scribes spell k of scroll it (c nil: nobody).
func (e *Engine) markScribe(c *Character, it *Item, k int) {
	if e.camp.scriber == nil {
		e.camp.scriber = map[*Item][3]*Character{}
	}
	by := e.camp.scriber[it]
	by[k] = c
	e.camp.scriber[it] = by
}

// willStudy (extra): a rest would have someone memorize or scribe a
// spell, the planned spells taken up first as ActRest does (on copies).
func (e *Engine) willStudy() bool {
	for _, c := range e.party {
		mem, study := c.Memorized, c.Study
		e.planSpells(c)
		t := e.studyTime(c)
		c.Memorized, c.Study = mem, study
		if t > 0 {
			return true
		}
	}
	return false
}

func (e *Engine) partyStudy() int {
	most := 0
	for _, c := range e.party {
		most = max(most, e.studyTime(c))
	}
	return most
}

// restFor (extra) rests mins minutes or until stop says so (nil: never);
// then then gets whether a monster interrupted.
func (e *Engine) restFor(mins int, stop func() bool, then func(interrupted bool)) {
	if stop == nil {
		stop = func() bool { return false }
	}
	r := &e.camp.rest
	*r = [7]int{}
	r[1] = mins
	e.normalizeRest()
	e.Rest(stop, func(interrupted bool) {
		e.camp.rest = [7]int{}
		then(interrupted)
	})
}

const spellCureLight = 3

// healParty (extra) is the later games' Fix: clerics cast their Cure Light
// Wounds on the most wounded; while that is not enough they memorize Cure
// Light Wounds in every first-level slot, rest to learn it and cast again.
// Then they rest to learn their planned spells (Plan) again, and natural
// healing (1 HP a day) does the rest.
// The rests follow one another (restFor's then); then gets whether one
// was interrupted.
func (e *Engine) healParty(then func(interrupted bool)) {
	var clerics []*Character
	for _, c := range e.party {
		if c.spellSlot(clsCleric, 1) > 0 && c.SpellBook[spellCureLight-1] != 0 {
			clerics = append(clerics, c)
		}
	}
	natural := func() {
		if e.healMinutes() > 0 {
			e.restFor(e.healMinutes(), func() bool { return e.healMinutes() == 0 }, then)
			return
		}
		then(false)
	}
	planned := func() {
		if len(clerics) == 0 {
			natural()
			return
		}
		for _, c := range clerics { // the cures left make way for the planned spells
			for i, v := range c.Memorized {
				if v == spellCureLight {
					c.Memorized[i] = 0
				}
			}
			e.planSpells(c)
		}
		if most := e.partyStudy(); most > 0 {
			e.restFor(most, nil, func(interrupted bool) {
				if interrupted {
					then(true)
					return
				}
				natural()
			})
			return
		}
		natural()
	}
	var cures func()
	cures = func() {
		if !(len(clerics) > 0 && e.healMinutes() > 0 && e.spellsAllowed()) {
			planned()
			return
		}
		for _, c := range clerics {
			e.memorizeCures(c)
		}
		before := e.healMinutes()
		e.restFor(e.partyStudy(), nil, func(interrupted bool) {
			if interrupted {
				then(true)
				return
			}
			e.castCures()
			if e.healMinutes() >= before {
				planned() // the cures do not help (nobody could cast)
				return
			}
			cures()
		})
	}
	e.castCures()
	cures()
}

// planSpells (extra): the spells of c.Plan that c does not hold are set to
// be memorized on the next rest, as far as the free slots go.
func (e *Engine) planSpells(c *Character) {
	clearMemorize(c)
	held := map[byte]int{}
	for _, v := range c.Memorized {
		if v != 0 {
			held[v]++
		}
	}
	free := map[byte]int{} // per level and class, lazily from the slots
	for _, s := range c.Plan {
		if s == 0 {
			continue
		}
		if held[s] > 0 {
			held[s]--
			continue
		}
		lvl, cls := e.spellLevel(s), e.spellClass(s)
		k := 3*cls + lvl
		if _, ok := free[k]; !ok {
			free[k] = int(c.spellSlot(int(cls), int(lvl)))
			for _, v := range c.Memorized {
				if v != 0 && e.spellLevel(v) == lvl && e.spellClass(v) == cls {
					free[k]--
				}
			}
		}
		if free[k] <= 0 {
			continue
		}
		free[k]--
		if i := slices.Index(c.Memorized[:], 0); i >= 0 {
			c.Memorized[i] = s | 0x80
		}
	}
	e.sortSpells(c)
}

// memorizeCures replaces c's first-level cleric spells by Cure Light
// Wounds to be memorized, one per slot.
func (e *Engine) memorizeCures(c *Character) {
	var m [21]byte
	k := 0
	for _, v := range c.Memorized {
		if v != 0 && !(e.spellClass(v) == clsCleric && e.spellLevel(v) == 1) {
			m[k] = v
			k++
		}
	}
	for n := c.spellSlot(clsCleric, 1); n > 0 && k < len(m); n-- {
		m[k] = spellCureLight | 0x80
		k++
	}
	c.Memorized = m
}

// spellsAllowed: the area allows spells.
func (e *Engine) spellsAllowed() bool { return e.area.NoMagic == 0 }

// castCures (extra): every cleric able to cast spends memorized Cure Light
// Wounds on the most wounded member while someone needs it.
func (e *Engine) castCures() {
	if !e.spellsAllowed() {
		return
	}
	for _, c := range e.party {
		if c.Status == statusAnimated || c.Okay == 0 {
			continue
		}
		for i, v := range c.Memorized {
			if v != spellCureLight {
				continue
			}
			var t *Character
			for _, p := range e.party {
				if healable(p) && p.HP < p.MaxHP && e.FindAffect(p, affMummyDisease) == nil && (t == nil || int(p.MaxHP)-int(p.HP) > int(t.MaxHP)-int(t.HP)) {
					t = p
				}
			}
			if t == nil {
				return
			}
			c.Memorized[i] = 0
			e.spellCaster, e.spellTarget, e.nSpellTargets = c, t, 1
			e.spellCureLightWounds()
		}
	}
	e.spellCaster = nil
}

// healable: Heal works on c (its status).
func healable(c *Character) bool {
	switch c.Status {
	case 0, 1, 4, 5:
		return true
	}
	return false
}

// clockMinutes (extra): the time of day in minutes.
func (e *Engine) clockMinutes() int {
	return 60*int(e.area.Clock[3]) + 10*int(e.area.Clock[2]) + int(e.area.Clock[1])
}

// isDay (extra): day runs from 6:00 to 20:00.
func (e *Engine) isDay() bool {
	m := e.clockMinutes()
	return m >= 6*60 && m < 20*60
}

// healMinutes (extra): the rest until every member that resting can heal
// is at full hit points (1 HP a day, see healTick); 0 when none is.
func (e *Engine) healMinutes() int {
	days := 0
	for _, c := range e.party {
		if healable(c) && e.FindAffect(c, affMummyDisease) == nil {
			days = max(days, int(c.MaxHP)-int(c.HP))
		}
	}
	if days == 0 {
		return 0
	}
	return days*24*60 - e.camp.heal*5
}

// sortSpells = ovr16:0824 (char): c's spell slots in spell order (empty
// slots first).
func (e *Engine) sortSpells(c *Character) {
	s := c.Memorized[:]
	for i := 0; i <= 0x13; i++ {
		for j := i; j <= 0x14; j++ {
			if s[i]&0x7F > s[j]&0x7F {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// planned: how many spells of the level and class c's plan holds.
func (e *Engine) planned(c *Character, lvl, cls byte) int {
	n := 0
	for _, v := range c.Plan {
		if v != 0 && e.spellLevel(v) == lvl && e.spellClass(v) == cls {
			n++
		}
	}
	return n
}

// listedAffect = the affect types ovr16:0FBC names after the spell that
// gives them (cs:0x1050 checks).
func listedAffect(t affectType) bool {
	switch {
	case t == affBless, t == affCurse, t == affDetectMagic, t >= affProtEvil && t <= affFriends,
		t == affReadMagic, t == affShield, t >= affFindTraps && t <= affInvisible, t == affMirrorImage,
		t == affEnfeebled, t >= affAnimated && t <= affCauseDisease, t >= affBestowCurse && t <= affHaste,
		t == affProtMissiles, t == affSlow, t == affProtEvilRadius, t == affProtGoodRadius,
		t == affPrayer, t >= affSnakeCharm && t <= affAsleep:
		return true
	}
	return false
}

// affectNames = ovr16:0FBC's named affects that no spell list entry covers.
var affectNames = map[affectType]string{
	affStudyingManual:   "Studying Manual of Bodily Health",
	affTrainingManual:   "Training with Manual of Bodily Health",
	affFeatherFall:      "Feather Fall",
	affHelpless:         "Helpless",
	affPrayerBonus:      "Prayer",
	affSick:             "Cause Disease",
	affMummyDisease:     "Dreaded Mummy Disease",
	affRepulsed:         "Repulsed",
	affPoisoned:         "Poisoned",
	affRegenerating:     "Regenerating",
	affFireResistance:   "Fire Resistance",
	affMonsterInvisible: "Invisible",
	affCamouflaged:      "Camouflaged",
	affDisplaced:        "Displaced",
	affLaidOnHands:      "Laid on Hands today",
	affCuredDisease:     "Cured Disease this week",
}

// affectList = the names ovr16:0FBC lists for c: its spell effects.
func (e *Engine) affectList(c *Character) []string {
	var names []string
	for _, a := range c.Affects {
		t := a.Type
		name, known := "", true
		if listedAffect(t) {
			for s := 1; s <= lastSpell && name == ""; s++ {
				if affectType(spellTable[s][10]) == t {
					name = e.spellName(s)
				}
			}
			if name == "" {
				name = "Funky--" + strconv.Itoa(int(t))
			}
		} else if name, known = affectNames[t]; !known {
			continue
		}
		names = append(names, name)
	}
	return names
}

// displayAffects = ovr16:0FBC: the members with the spell effects on them,
// under a box titled title. The list appends are the nested ovr16:0E37
// (text, flag). The caller redraws the screen in then.
func (e *Engine) displayAffects(title string, members []*Character, then func()) {
	list := []*ListNode{{Text: " "}}
	for _, c := range members {
		list = append(list, &ListNode{Text: c.Name(), Flag: 1})
		names := e.affectList(c)
		for _, name := range names {
			list = append(list, &ListNode{Text: cut(" "+name, textCols)})
		}
		if len(names) == 0 {
			list = append(list, &ListNode{Text: " <No Spell Effects>"})
		}
		list = append(list, &ListNode{Text: " "})
	}
	redraw := true
	index := 0
	var sel *ListNode
	e.newPage()
	e.blank(1, 1, winRight, winBottom)
	e.ListMenu("", nil, colLightCyan, colLightGreen, colWhite, 1, 4, 38, 22, list, true, &redraw, &index, &sel, func(Pick) { then() })
}

// dropMember = ovr16:18E5 (the player confirmed): member n leaves the
// party; the last one ends the game.
func (e *Engine) dropMember(n int) {
	if n < 0 || n >= len(e.party) {
		return
	}
	e.selectMember(n)
	c := e.selected()
	if len(e.party) == 1 {
		e.RemoveSelected(false, true)
		e.Exit()
	}
	if c.Okay != 0 {
		e.say(c.Name() + " bids you farewell")
	} else {
		e.say(c.Name() + " is dumped in a ditch")
	}
	e.RemoveSelected(false, true)
}

// Rest choices (ActRest N).
const (
	RestFix   = iota // until everyone is healed (healParty) and the planned spells are learned
	RestWait         // an hour
	RestUntil        // until day (at night) or night (by day)
)

// CampMenu = ovr16:1E6F, remade: camp takes decisions until the party
// leaves (ActClose) or a rest is interrupted; then then (nil: nothing)
// gets whether it was (the caller then runs the encounter).
func (e *Engine) CampMenu(then func(interrupted bool)) {
	e.Music(1) // Amiga 0x17FBC .. 0x1813C
	saved := e.beginFlag
	e.beginFlag = 2
	e.camp.heal = 0
	e.camp.rest = [7]int{}
	e.DrawMainScreen()
	e.clearStudy()
	e.camp.interrupted, e.camp.rested = false, false
	anim := e.animInput
	e.animInput = true // the camp picture's fire burns while camp is up (the camp bar's anim, ovr06:101F)
	e.push(modeCamp, func(a Action) bool {
		left := e.campDecision(a)
		return left || e.camp.interrupted
	}, func() {
		for _, c := range e.party { // the plans hold: the next rest learns them
			e.planSpells(c)
		}
		e.spellCaster = nil
		e.beginFlag = saved
		e.animInput = anim
		e.ClearMessageArea()
		e.clearPrompt()
		e.Music(0)
		if then != nil {
			then(e.camp.interrupted)
		}
	})
}

// rested ends a rest in camp: an interruption closes camp.
func (e *Engine) rested(interrupted bool) {
	e.camp.interrupted, e.camp.rested = interrupted, true
}

// campDecision carries out a decision in camp; true when the party leaves.
// A rest is a screen of its own (modeRest), which ends in rested.
func (e *Engine) campDecision(a Action) (left bool) {
	member := func() *Character { // member N (< 0: the selected one) acts and becomes the selected one
		if a.N >= 0 && a.N < len(e.party) {
			e.selectMember(a.N)
		}
		return e.selected()
	}
	switch a.Kind {
	case ActClose:
		return true
	case ActLead:
		member()
	case ActMember:
		member()
		e.pushSheet()
	case ActInventory:
		if c := member(); len(e.carried(c)) == 0 {
			e.say("No items")
		} else {
			e.pushSheetOn(true)
		}
	case ActCastPick:
		member()
		e.CastMenu()
	case ActHeal, ActCure, ActSpells, ActMagic:
		c := member()
		if slices.ContainsFunc(e.powers(c), func(p Power) bool { return p.Kind == a.Kind }) {
			e.power(c, a.Kind, func(bool) { e.DrawMainScreen() })
		}
	case ActPlan:
		e.planSpell(member(), byte(a.X), a.Y)
	case ActScribe:
		e.scribe(member(), byte(a.X))
	case ActDropMember:
		e.dropMember(a.N)
	case ActIconEdit:
		member()
		e.pushIcon()
	case actRedraw: // back from the icon editor (a rest draws nothing again)
		if e.camp.rested {
			e.camp.rested = false
		} else if e.mode() == modeCamp {
			e.DrawMainScreen()
		}
	case ActRest:
		for _, c := range e.party { // extra: the planned spells are learned on any rest
			e.planSpells(c)
		}
		switch a.N {
		case RestFix:
			if e.healMinutes() == 0 && e.partyStudy() == 0 {
				e.say("Nothing to fix")
				return false
			}
			study := func(interrupted bool) { // healing ends with the planned spells only when there are clerics
				if most := e.partyStudy(); !interrupted && most > 0 {
					e.restFor(most, nil, e.rested)
					return
				}
				e.rested(interrupted)
			}
			if e.healMinutes() == 0 {
				study(false)
			} else {
				e.healParty(study)
			}
		case RestWait:
			e.restFor(60, nil, e.rested)
		case RestUntil:
			until := 20 * 60
			if !e.isDay() {
				until = 6 * 60
			}
			e.restFor(((until-e.clockMinutes())%(24*60)+24*60)%(24*60), nil, e.rested)
		}
	}
	return false
}

// campView is camp's state for the View.
func (e *Engine) campView() *Camp {
	c := e.selected()
	v := &Camp{CanCast: e.canCastNow(), Healing: e.healMinutes() > 0, Studying: e.willStudy(), Bestiary: e.beasts()}
	if c != nil {
		v.Book, v.Scrolls = e.spellLists(c)
	}
	return v
}

// spellLists are c's spell book (how often each is memorized on a rest)
// and the spells on c's scrolls not in it (Count 1 while being scribed).
func (e *Engine) spellLists(c *Character) (book, scrolls []BookSpell) {
	spell := func(s byte, n int) BookSpell {
		lvl, cls := e.spellLevel(s), e.spellClass(s)
		return BookSpell{ID: int(s), Name: e.spellName(int(s)), Level: spellGroups[e.spellRec(s, 1)], Count: n,
			Planned: e.planned(c, lvl, cls), Slots: int(c.spellSlot(int(cls), int(lvl)))}
	}
	for i := 1; i <= 0x37 && i <= len(c.SpellBook); i++ {
		if c.SpellBook[i-1] != 0 {
			book = append(book, spell(byte(i), bytes.Count(c.Plan[:], []byte{byte(i)})))
		}
	}
	for _, it := range e.carried(c) {
		if !e.isScroll(it) {
			continue
		}
		for j, s := range it.Spells {
			if s > 0x80 && !e.scribesIt(c, it, j) {
				continue // another member scribes it
			}
			if k := int(s & 0x7F); k != 0 && k <= len(c.SpellBook) && c.SpellBook[k-1] == 0 && c.spellSlot(int(e.spellClass(byte(k))), int(e.spellLevel(byte(k)))) > 0 { // only what c can scribe (scribe refuses the rest)
				n := 0
				if s > 0x7F {
					n = 1
				}
				scrolls = append(scrolls, spell(s&0x7F, n))
			}
		}
	}
	return book, scrolls
}

// planSpell (extra) memorizes spell s on every rest once more (delta > 0)
// or once less, as far as c's slots for its level go.
func (e *Engine) planSpell(c *Character, s byte, delta int) {
	if s == 0 || int(s) > len(c.SpellBook) || c.SpellBook[s-1] == 0 || !e.canStudy(2) {
		return
	}
	if delta < 0 {
		if i := bytes.LastIndexByte(c.Plan[:], s); i >= 0 {
			c.Plan[i] = 0
		}
		return
	}
	if e.planned(c, e.spellLevel(s), e.spellClass(s)) < int(c.spellSlot(int(e.spellClass(s)), int(e.spellLevel(s)))) {
		if i := slices.Index(c.Plan[:], 0); i >= 0 {
			c.Plan[i] = s
		}
	}
}

// scribe = the pick of ovr16:0B5B: c scribes spell s from a scroll on
// the next rest, or no longer when it is being scribed.
func (e *Engine) scribe(c *Character, s byte) {
	if s == 0 || int(s) > len(c.SpellBook) || !e.canStudy(3) {
		return
	}
	for _, it := range e.carried(c) {
		if !e.isScroll(it) {
			continue
		}
		for k := range it.Spells {
			if it.Spells[k] == s|0x80 && e.scribesIt(c, it, k) {
				it.Spells[k] = s
				e.markScribe(nil, it, k)
				return
			}
		}
	}
	switch {
	case c.SpellBook[s-1] != 0:
		e.say("You already know that spell")
		return
	case c.spellSlot(int(e.spellClass(s)), int(e.spellLevel(s))) == 0:
		e.say("You can not scribe that spell")
		return
	}
	for _, it := range e.carried(c) { // (any item's spell bytes count here, not only scrolls)
		for k := range it.Spells {
			if it.Spells[k] == s {
				it.Spells[k] |= 0x80
				e.markScribe(c, it, k)
				return
			}
		}
	}
}

// ---- ovr21: resting ----

// subRest = ovr21:047A (count, unit): take count units off the rest time,
// borrowing from the larger units; all of it when that runs out.
func (e *Engine) subRest(count, unit int) {
	r := &e.camp.rest
	if r[4] == 0 && r[3] == 0 && r[2] == 0 && r[1] == 0 {
		return
	}
	for count > r[unit] {
		k := unit + 1
		for r[k] == 0 && k < 5 {
			k++
		}
		if k == 5 {
			*r = [7]int{}
			count = 0
			continue
		}
		for j := k; j >= unit+1; j-- {
			r[j]--
			r[j-1] += clockLimits[j-1]
		}
	}
	r[unit] -= count
	e.normalizeRest()
}

// normalizeRest = ovr21:03A3: carries, months fold into days, at most 99.
func (e *Engine) normalizeRest() {
	r := &e.camp.rest
	for i := 0; i < 5; i++ {
		if r[i] >= clockLimits[i] {
			r[i+1] += r[i] / clockLimits[i]
			r[i] %= clockLimits[i]
		}
	}
	if r[5] > 0 {
		r[4] += clockLimits[4] * r[5]
		r[5] = 0
		if r[4] > 0x63 {
			r[4] = 0x63
		}
	}
}

// twoDigits = ovr21:0565.
func twoDigits(n int) string {
	s := strconv.Itoa(n)
	if n < 10 {
		s = "0" + s
	}
	return s[:2]
}

// showRest = ovr21:05F0 (unit): "Rest time dd:hh:mm" (the original: "Rest Time:"), the unit being
// changed in white.
func (e *Engine) showRest(unit int) {
	var col [7]int
	for i := range col {
		col[i] = 0xA
	}
	col[unit] = 0xF
	r := &e.camp.rest
	e.text(1, 17, colLightGreen, "Rest time")
	x := 0xB
	for i := 4; i >= 3; i-- {
		e.text(x+1, 17, col[i], twoDigits(r[i]))
		e.text(x+3, 17, colLightGreen, ":")
		x += 3
	}
	e.text(x+1, 17, col[2], twoDigits(10*r[2]+r[1]))
}

// healTick = ovr21:0871: every 24 hours of rest everyone heals 1 HP.
func (e *Engine) healTick() {
	e.camp.heal++
	if e.camp.heal < 0x120 {
		return
	}
	for _, c := range e.party {
		e.Heal(c, 1, false)
	}
	e.showRest(0)
	e.text(1, 19, colLightGreen, "The Whole Party Is Healed")
	e.say("The Whole Party Is Healed")
	e.MsgDelay()
	e.ClearMessageArea()
	e.camp.heal = 0
}

// memorizeOne = ovr21:0925 (char, var check): with check clear the first
// waiting spell is memorised (and check set); returns the level of the
// next waiting spell, 0 when none is left.
func (e *Engine) memorizeOne(c *Character, check *bool) int {
	found := 0
	for i := 0; i <= 0x14 && found == 0; i++ {
		s := c.Memorized[i]
		if s <= 0x7F {
			continue
		}
		if *check {
			found = int(e.spellLevel(s))
			continue
		}
		c.Memorized[i] -= 0x80
		e.showRest(0)
		e.announceCast(c, "has memorized", int(c.Memorized[i]))
		*check = true
	}
	return found
}

// scribeOne = ovr21:09E7 (char, var check): memorizeOne for scrolls being
// scribed; the spell goes into the book and off the scroll.
func (e *Engine) scribeOne(c *Character, check *bool) int {
	found := 0
	for _, it := range e.carried(c) {
		if found != 0 {
			break
		}
		if !e.isScroll(it) {
			continue
		}
		for k := 1; k <= 3 && found == 0; k++ {
			s := it.Spells[k-1]
			if s <= 0x80 || !e.scribesIt(c, it, k-1) {
				continue
			}
			if *check {
				found = int(e.spellLevel(s))
				continue
			}
			sp := s & 0x7F
			c.SpellBook[int(sp)-1] = 1
			e.scrollUsed(c, it, sp)
			e.showRest(0)
			e.announceCast(c, "has scribed", int(sp))
			*check = true
		}
	}
	return found
}

// studyStep = the shared tail of ovr21:0B0A / 0BB2: learn what is due and
// time the next spell (3 steps per spell level).
func (e *Engine) studyStep(i int, c *Character, check bool) {
	lvl := e.scribeOne(c, &check)
	if lvl == 0 {
		lvl = e.memorizeOne(c, &check)
	}
	e.camp.study[i] = byte(lvl * 3)
}

// memTick = ovr21:0B0A: every five minutes.
func (e *Engine) memTick() {
	for i, c := range e.party {
		if i+1 >= len(e.camp.study) {
			break
		}
		e.camp.study[i+1]--
		if e.camp.study[i+1] == 0 && c.Study == 0 {
			e.studyStep(i+1, c, false)
		}
	}
}

// hourTick = ovr21:0BB2 (var count): every hour the study hours run down.
func (e *Engine) hourTick(count *int) {
	*count++
	if *count < 12 {
		return
	}
	*count = 0
	for i, c := range e.party {
		if i+1 >= len(e.camp.study) {
			break
		}
		if int8(c.Study) > 0 {
			c.Study--
			if c.Study == 0 {
				e.studyStep(i+1, c, true)
			}
		}
	}
}

// Rest = ovr21:0C9A: rest in five-minute steps until the time is up, a key
// stops it, or a monster interrupts (game +0x5A4 steps between rolls,
// +0x5A6 percent chance); then then gets whether it was interrupted. The
// loop is a timed screen (modeRest): one step a frame, as the DOS loop
// ran flat out.
func (e *Engine) Rest(stop func() bool, then func(interrupted bool)) {
	interrupted := false
	for i := range e.camp.study {
		e.camp.study[i] = 0
	}
	for i := 1; i < len(e.run.effTick); i++ {
		e.run.effTick[i] = 1
	}
	hours, shown, stopped := 0, 0, false
	e.clearPane()
	e.showRest(0)
	e.resting = 1
	r := &e.camp.rest
	e.pushTimed(modeRest, func(a Action) bool {
		if r[4] == 0 && r[3] == 0 && r[2] == 0 && r[1] == 0 || stop() || stopped {
			return true
		}
		if a.Kind != ActTick { // keyHit: the step waits for the next frame
			if e.pressed(a) {
				e.showRest(0)
				e.YesNo("Stop resting?", func(yes bool) {
					if stopped = yes; !yes { // the rest ends on the actRedraw that follows
						e.clearPrompt()
					}
				})
			}
			return false
		}
		e.subRest(5, 1)
		if shown++; shown >= 5 {
			e.showRest(0)
			shown = 0
		}
		e.AdvanceClock(5, 1)
		e.healTick()
		e.memTick()
		e.hourTick(&hours)
		if every := e.game.RestEvery; every > 0 {
			e.camp.interval++
			if e.camp.interval >= int(every) {
				e.camp.interval = 0
				if e.Dice(1, 100) <= int(e.game.RestChance) {
					e.ClearMessageArea()
					e.showRest(0)
					e.text(1, 19, colWhite, "The Party is rudely interrupted!")
					e.say("The Party is rudely interrupted!")
					interrupted = true
					e.MsgDelay()
					return true
				}
			}
		}
		return false
	}, func() {
		e.clearPane()
		e.resting = 0
		then(interrupted)
	})
}
