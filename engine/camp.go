package engine

import (
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
			if s := it.Spells[k-1]; s > 0x7F {
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
// hasPending: spells are chosen to be memorized.
func hasPending(c *Character) bool {
	return slices.ContainsFunc(c.Memorized[:], func(v byte) bool { return v > 0x7F })
}

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
	for _, c := range e.party {
		clearMemorize(c)
		e.clearScribe(c)
	}
}

// slotsLeft = ovr16:025C (level, class): spells of the level the selected
// character may still memorise (+0xB1 + 3*class + level holds the total).
func (e *Engine) slotsLeft(level, class byte) int {
	c := e.selected()
	n := 0
	for i := 0; i <= 0x14; i++ {
		if s := c.Memorized[i]; s > 0 && e.spellLevel(s) == level && e.spellClass(s) == class {
			n++
		}
	}
	return int(byte(int(c.spellSlot(int(class), int(level))) - n))
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
	// extra: the pick list (View.SpellPick, as the combat's Cast) instead of the
	// spell page, and one cast: the original lists again until Exit
	if spell := e.pickSpell(); spell != 0 { // the main screen is still up: no message box to redraw (the original's spell page had replaced it)
		var quit bool
		e.CastSpell(spell, 0, 1, &quit)
		e.DrawMainScreen()
	}
}

// memorizeInfo = ovr16:05A4: "can memorize:" with the free slots per class
// and level in the message box; false when there are none at all.
func (e *Engine) memorizeInfo() bool {
	c := e.selected()
	var grid [2][4]string
	var has [2]bool
	any := false
	for cls := 0; cls <= 1; cls++ {
		for lvl := 1; lvl <= 3; lvl++ {
			s := strconv.Itoa(int(int8(e.slotsLeft(byte(lvl), byte(cls)))))
			if len(s) > 2 {
				s = s[:2]
			}
			grid[cls][lvl] = s
			if c.spellSlot(cls, lvl) == 0 {
				grid[cls][lvl] = " "
			} else {
				has[cls] = true
				any = true
			}
		}
	}
	if !any {
		return false
	}
	e.CharMessage(c, "can memorize:", false)
	row := 3
	for cls := 0; cls <= 1; cls++ {
		if !has[cls] {
			continue
		}
		label := "    Cleric Spells:"
		if cls == 1 {
			label = "Magic-User Spells:"
		}
		e.DisplayString(1, row+0x11, colLightGreen, label)
		x := 0x14 // ovr16:0723: [bp-0x41]=0x13, passed +1
		for lvl := 1; lvl <= 3; lvl++ {
			e.DisplayString(x, row+0x11, colLightGreen, grid[cls][lvl])
			x += 3
		}
		row++
	}
	return true
}

// restMenu = ovr16:077A (var interrupted): the longest study time of the
// party becomes the proposed rest time, then rest (ovr21:0C9A).
func (e *Engine) restMenu() (interrupted bool) {
	for {
		// extra: rest for a purpose instead of setting days, hours and minutes
		dayNight, until := "Night", 20*60
		if !e.isDay() {
			dayNight, until = "Day", 6*60
		}
		const (
			heal Choice = iota
			memorize
			wait
			untilIt
			exit
		)
		switch e.DisplayInput("Rest: ", Bar{{"Heal", heal}, {"Memorize", memorize}, {"Wait", wait}, {dayNight, untilIt}, {"Exit", exit}}, colLightMagenta, colLightGreen, colWhite, false, false).ID {
		case heal:
			if e.healMinutes() == 0 {
				e.BottomMessage("No one needs healing")
				continue
			}
			return e.healParty()
		case memorize:
			most := e.partyStudy()
			if most == 0 {
				e.BottomMessage("No spells to memorize")
				continue
			}
			return e.restFor(most, nil)
		case wait:
			return e.restFor(60, nil)
		case untilIt:
			return e.restFor(((until-e.clockMinutes())%(24*60)+24*60)%(24*60), nil)
		}
		return false
	}
}

// partyStudy: the longest study time of the party in minutes (studyTime
// also sets each member's study hours, which Rest counts down).
func (e *Engine) partyStudy() int {
	most := 0
	for _, c := range e.party {
		most = max(most, e.studyTime(c))
	}
	return most
}

// restFor (extra) rests mins minutes or until stop says so (nil: never).
func (e *Engine) restFor(mins int, stop func() bool) (interrupted bool) {
	if stop == nil {
		stop = func() bool { return false }
	}
	r := &e.camp.rest
	*r = [7]int{}
	r[1] = mins
	e.normalizeRest()
	interrupted = e.Rest(stop)
	e.camp.rest = [7]int{}
	e.StatusLine()
	return interrupted
}

const spellCureLight = 3

// healParty (extra) is the later games' Fix: clerics cast their Cure Light
// Wounds on the most wounded; while that is not enough they memorize Cure
// Light Wounds in every first-level slot, rest to learn it and cast again.
// Then they rest to learn their own spells again, and natural healing (1 HP
// a day) does the rest.
func (e *Engine) healParty() (interrupted bool) {
	saved := map[*Character][21]byte{}
	for _, c := range e.party {
		if c.spellSlot(clsCleric, 1) > 0 && c.SpellBook[spellCureLight-1] != 0 {
			saved[c] = c.Memorized
		}
	}
	e.castCures()
	for len(saved) > 0 && e.healMinutes() > 0 && !interrupted && e.spellsAllowed() {
		for c := range saved {
			e.memorizeCures(c)
		}
		before := e.healMinutes()
		if interrupted = e.restFor(e.partyStudy(), nil); !interrupted {
			e.castCures()
		}
		if e.healMinutes() >= before {
			break // the cures do not help (nobody could cast)
		}
	}
	if len(saved) > 0 && !interrupted {
		for c, m := range saved { // relearn what was cast or replaced, nothing else
			r := restoreSpells(m, c.Memorized)
			for i, v := range m {
				if v > 0x7F {
					r[i] = 0
				}
			}
			c.Memorized = r
		}
		if most := e.partyStudy(); most > 0 {
			interrupted = e.restFor(most, nil)
		}
	}
	for c, m := range saved {
		if interrupted { // the rest is left to be memorized
			c.Memorized = restoreSpells(m, c.Memorized)
		} else {
			c.Memorized = m // exactly the spells of before
			e.setLearned(c, m)
		}
	}
	if !interrupted && e.healMinutes() > 0 {
		interrupted = e.restFor(e.healMinutes(), func() bool { return e.healMinutes() == 0 })
	}
	return interrupted
}

// restoreSpells: the spells of was, those no longer in now (cast or
// replaced) to be memorized again.
func restoreSpells(was, now [21]byte) [21]byte {
	left := map[byte]int{}
	for _, v := range now {
		if v > 0 && v < 0x80 {
			left[v]++
		}
	}
	for i, v := range was {
		if v > 0 && v < 0x80 {
			if left[v] > 0 {
				left[v]--
			} else {
				was[i] = v | 0x80
			}
		}
	}
	return was
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

// sortSpells = ovr16:0824: the selected character's spell slots in spell
// order (empty slots first).
func (e *Engine) sortSpells() {
	c := e.selected()
	s := c.Memorized[:]
	for i := 0; i <= 0x13; i++ {
		for j := i; j <= 0x14; j++ {
			if s[i]&0x7F > s[j]&0x7F {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// noteSpells (extra): before c first uses a spell, the spells it holds are
// remembered, so that Memorize can offer to learn the used ones again.
func (e *Engine) noteSpells(c *Character) {
	if _, ok := e.ext.learned[c]; !ok {
		e.setLearned(c, c.Memorized)
	}
}

func (e *Engine) setLearned(c *Character, m [21]byte) {
	if e.ext.learned == nil {
		e.ext.learned = map[*Character][21]byte{}
	}
	e.ext.learned[c] = m
}

// relearnSpells (extra): offer to memorize again the spells c used since
// they were learned, when nothing else is waiting to be memorized.
func (e *Engine) relearnSpells(c *Character) {
	was, ok := e.ext.learned[c]
	if !ok || slices.ContainsFunc(c.Memorized[:], func(v byte) bool { return v > 0x7F }) {
		return
	}
	r := restoreSpells(was, c.Memorized)
	if !slices.ContainsFunc(r[:], func(v byte) bool { return v > 0x7F }) {
		return // nothing used
	}
	if e.YesNo(colYellow, colLightGreen, colWhite, "Relearn the spells used? ") {
		c.Memorized = r
		e.sortSpells()
	}
}

// memorizeMenu = ovr16:090A.
func (e *Engine) memorizeMenu() {
	if !e.canStudy(2) {
		return
	}
	e.relearnSpells(e.selected())
	// extra: no "Spells to be memorized" list and "Memorize These Spells?"
	// before or after: the book's list marks the chosen spells, Clear drops them
	done, drew := false, false
	index := -1
	var any bool
	for !done {
		done = !e.memorizeInfo()
		if done {
			c := e.selected()
			e.CharMessage(c, "cannot memorize any spells", true)
			continue
		}
		spell := e.SpellList(1, 2, &index, &any)
		drew = true
		if e.spellCleared { // extra: Clear drops the chosen spells
			clearMemorize(e.selected())
			continue
		}
		if spell == 0 {
			done = true
			continue
		}
		if e.slotsLeft(e.spellLevel(spell), e.spellClass(spell)) > 0 { // unsigned test (jbe) in the original
			c := e.selected()
			k := 0
			for c.Memorized[k] != 0 {
				k++
			}
			c.Memorized[k] = spell + 0x80
			e.sortSpells()
		}
	}
	if drew {
		e.DrawMainScreen()
	}
}

// scribeMenu = ovr16:0B5B.
func (e *Engine) scribeMenu() {
	if !e.canStudy(3) {
		return
	}
	done := false
	index := -1
	var any bool
	e.SpellList(6, 0, &index, &any)
	drew := true
	if any {
		if !e.YesNo(colYellow, colLightGreen, colWhite, "Scribe These Spells? ") {
			e.clearScribe(e.selected())
		} else {
			done = true
		}
	} else {
		drew = false
	}
	index = -1
	for !done {
		spell := e.SpellList(3, 3, &index, &any)
		c := e.selected()
		if spell == 0 {
			done = true
			if !any {
				e.CharMessage(c, "has no copyable scrolls", true)
			} else {
				drew = true
			}
			continue
		}
		drew = true
		if c.SpellBook[int(spell)-1] != 0 {
			e.BottomMessage("You already know that spell")
			continue
		}
		found := false
		for _, it := range e.carried(c) {
			if found {
				break
			}
			if !e.isScroll(it) {
				continue
			}
			for k := 1; k <= 3; k++ {
				if s := it.Spells[k-1]; s > 0x7F && s&0x7F == spell {
					e.BottomMessage("You are already scibing that spell")
					found = true
				}
			}
		}
		if found {
			continue
		}
		if c.spellSlot(int(e.spellClass(spell)), int(e.spellLevel(spell))) == 0 {
			e.BottomMessage("You can not scribe that spell.")
			continue
		}
		// (any item's spell bytes count here, not only scrolls)
		for _, it := range e.carried(c) {
			if found {
				break
			}
			for k := 1; k <= 3 && !found; k++ {
				if it.Spells[k-1] == spell {
					it.Spells[k-1] |= 0x80
					found = true
				}
			}
		}
	}
	if index != -1 {
		index = -1
		e.SpellList(6, 0, &index, &any)
		if any && !e.YesNo(colYellow, colLightGreen, colWhite, "Scribe these spells? ") {
			e.clearScribe(e.selected())
		}
	}
	if drew {
		e.DrawMainScreen()
	}
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
// (text, flag). The caller redraws the screen.
func (e *Engine) displayAffects(title string, members []*Character) {
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
	e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colWhite, title)
	e.ListMenu("", nil, colLightCyan, colLightGreen, colWhite, 1, 4, 38, 22, list, true, &redraw, &index, &sel)
}

// magicMenu = ovr16:13F1 (var interrupted).
func (e *Engine) magicMenu(interrupted *bool) {
	for key := magicRest; !*interrupted && key != NoPick && key != magicExit; {
		// the original's Display is left out: the View page shows the spell effects
		var again bool
		if key, again = e.campInput("", append(e.magicCommands(), Entry{"Rest", magicRest}, Entry{"Exit", magicExit})); again {
			continue
		}
		switch key {
		case magicCast:
			e.CastMenu()
		case magicMemorize:
			e.memorizeMenu()
		case magicScribe:
			e.scribeMenu()
		case magicRest:
			*interrupted = e.restMenu()
		}
	}
}

// campInput (extra) waits on a camp menu bar. The keypad and F1-F8 pick
// the member the menu is for; again is set when the menu is to be rebuilt.
func (e *Engine) campInput(prompt string, menu Bar) (key Choice, again bool) {
	e.ext.campBar = true
	p := e.DisplayInput(prompt, menu, colLightMagenta, colLightGreen, colWhite, true, true)
	e.ext.campBar = false
	if !p.extra() {
		return p.ID, false
	}
	if p.ID != PickRelist {
		e.partyKeypad(p)
	}
	e.DisplayPartyList()
	return p.ID, true // not an item: the menu loops go on
}

// The items of the Magic menu.
const (
	magicCast Choice = iota
	magicMemorize
	magicScribe
	magicRest
	magicExit
)

// magicCommands (extra): the Magic menu items the selected member can use.
func (e *Engine) magicCommands() Bar {
	c := e.selected()
	if c.Status == statusAnimated || c.Okay == 0 {
		return nil
	}
	var m Bar
	if e.area.NoMagic == 0 && slices.ContainsFunc(c.Memorized[:], func(v byte) bool { return v > 0 && v < 0x80 }) {
		m = append(m, Entry{"Cast", magicCast})
	}
	caster, mu := false, false
	for lvl := 1; lvl <= 3; lvl++ {
		caster = caster || c.spellSlot(0, lvl) > 0 || c.spellSlot(1, lvl) > 0
		mu = mu || c.spellSlot(1, lvl) > 0
	}
	if caster {
		m = append(m, Entry{"Memorize", magicMemorize})
	}
	if mu && slices.ContainsFunc(e.carried(c), e.isScroll) {
		m = append(m, Entry{"Scribe", magicScribe})
	}
	return m
}

// movePrev = ovr16:14A9: the selected member swaps places with the one
// before; the first goes to the end.
func (e *Engine) movePrev() {
	i := e.partySel
	if i == 0 {
		c := e.party[0]
		e.party = append(e.party[1:], c)
		e.partySel = len(e.party) - 1
		return
	}
	e.party[i-1], e.party[i] = e.party[i], e.party[i-1]
	e.partySel = i - 1
}

// moveNext = ovr16:1601: swaps with the one after; the last goes first.
func (e *Engine) moveNext() {
	i := e.partySel
	if i == len(e.party)-1 {
		c := e.party[i]
		e.party = append([]*Character{c}, e.party[:i]...)
		e.partySel = 0
		return
	}
	e.party[i+1], e.party[i] = e.party[i], e.party[i+1]
	e.partySel = i + 1
}

// orderMenu = ovr16:1780: Select a member, then Place it with Home/End.
func (e *Engine) orderMenu() {
	const (
		selectIt Choice = iota // Select, or Place while placing
		exit
	)
	placing := false
	for key := selectIt; key != NoPick && key != exit; {
		menu := Bar{{"Select", selectIt}, {"Exit", exit}}
		if placing {
			menu[0].Word = "Place"
		}
		p := e.DisplayInput("Party Order: ", menu, colLightMagenta, colLightGreen, colWhite, true, true)
		key = p.ID
		if p.extra() {
			switch {
			case !placing:
				e.partyKeypad(p)
			case p.ID == PickMove && p.Dir == DirHome:
				e.movePrev()
			case p.ID == PickMove && p.Dir == DirEnd:
				e.moveNext()
			}
			e.DisplayPartyList()
			continue
		}
		if key == PickEnter || key == selectIt {
			placing = !placing
			if placing {
				c := e.selected()
				e.CharMessage(c, "has been selected", false)
			} else {
				e.ClearMessageArea()
			}
		}
	}
}

// dropMenu = ovr16:18E5: drop the selected member (the last one quits).
func (e *Engine) dropMenu() {
	c := e.selected()
	if len(e.party) == 1 {
		if e.YesNo(colYellow, colLightGreen, colWhite, "quit TO DOS: ") {
			e.RemoveSelected(false, true)
			e.Exit()
		}
		return
	}
	e.CharMessage(c, "will be gone", false)
	if !e.YesNo(colYellow, colLightGreen, colWhite, "Drop from party? ") {
		e.CharMessage(c, "Breathes A sigh of relief", true)
		return
	}
	if c.Okay != 0 {
		e.CharMessage(c, "bids you farewell", true)
	} else {
		e.CharMessage(c, "is dumped in a ditch", true)
	}
	e.RemoveSelected(false, true)
	e.ClearCells(17, 1, winRight, 11)
	e.DisplayPartyList()
}

// campSpeedMenu = ovr16:1A97: the message delay (DS:49E2), 0 fastest.
func (e *Engine) campSpeedMenu() {
	const (
		faster Choice = iota
		slower
		exit
	)
	for key := faster; key != NoPick && key != exit; {
		e.DisplayString(1, 18, colLightGreen, "Game Speed = "+strconv.Itoa(int(e.msgDelay))+" (0=fastest 9=slowest)")
		menu := Bar{{}} // the original's bar starts with a space
		if e.msgDelay > 0 {
			menu = append(menu, Entry{"Faster", faster})
		}
		if e.msgDelay < 9 {
			menu = append(menu, Entry{"Slower", slower})
		}
		menu = append(menu, Entry{"Exit", exit})
		p := e.DisplayInput("Game Speed:", menu, colLightMagenta, colLightGreen, colWhite, true, true)
		key = p.ID
		switch {
		case p.ID == PickMove && p.Dir == DirDown && e.msgDelay > 0:
			e.msgDelay--
		case p.ID == PickMove && p.Dir == DirUp && e.msgDelay < 9:
			e.msgDelay++
		case key == faster:
			e.msgDelay--
		case key == slower:
			e.msgDelay++
		}
	}
	e.ClearMessageArea()
}

// alterMenu = ovr16:1C59.
func (e *Engine) alterMenu() {
	const (
		order Choice = iota
		drop
		speed
		icon
		pics
		exit
		monsters
		portraits
	)
	for key := order; key != NoPick && key != exit; {
		p := e.DisplayInput("Alter: ", Bar{{"Order", order}, {"Drop", drop}, {"Speed", speed}, {"Icon", icon}, {"Pics", pics}, {"Exit", exit}}, colLightMagenta, colLightGreen, colWhite, true, true)
		key = p.ID
		if p.extra() {
			e.partyKeypad(p)
			e.DisplayPartyList()
			continue
		}
		switch key {
		case order:
			e.orderMenu()
		case drop:
			e.dropMenu()
		case speed:
			e.campSpeedMenu()
		case icon:
			e.iconEditor()
			e.DrawMainScreen()
		case pics:
			for k := pics; k != NoPick && k != exit; {
				menu := Bar{{"Monsters off ", monsters}, {"Portraits off ", portraits}, {"Exit", exit}} // two spaces after each
				if e.viewBit1 != 0 {
					menu[0].Word = "Monsters on "
				}
				if e.viewBit0 != 0 {
					menu[1].Word = "Portraits on "
				}
				k = e.DisplayInput("", menu, colLightMagenta, colLightGreen, colWhite, false, true).ID
				if k == monsters {
					e.viewBit1 ^= 1
				} else if k == portraits {
					e.viewBit0 ^= 1
				}
			}
		}
	}
}

// CampMenu = ovr16:1E6F: true when resting was interrupted (the caller
// then runs the encounter).
func (e *Engine) CampMenu() bool {
	e.Music(1) // Amiga 0x17FBC .. 0x1813C
	defer e.Music(0)
	saved := e.beginFlag
	e.beginFlag = 2
	e.camp.heal = 0
	e.camp.rest = [7]int{}
	e.DrawMainScreen()
	e.ClearCells(1, msgTop, winRight, winBottom)
	e.DisplayString(1, 18, colLightGreen, "The party makes camp...")
	e.clearStudy()
	interrupted := false
	const (
		view Choice = iota
		magic
		rest
		alter
		exit
	)
	for key := view; !interrupted && key != NoPick && key != exit; {
		var menu Bar
		menu = append(menu, Entry{"View", view}) // (extra) no Save: F9 saves (slots.go)
		if len(e.magicCommands()) > 0 {
			menu = append(menu, Entry{"Magic", magic})
		}
		menu = append(menu, Entry{"Rest", rest}, Entry{"Alter", alter}, Entry{"Exit", exit})
		var again bool
		if key, again = e.campInput("Camp: ", menu); again {
			continue
		}
		switch key {
		case view:
			e.ViewCharacter()
		case magic:
			e.magicMenu(&interrupted)
		case rest:
			interrupted = e.restMenu()
		case alter:
			e.alterMenu()
		}
	}
	e.clearStudy()
	e.spellCaster = nil
	e.beginFlag = saved
	e.StatusLine()
	e.ClearMessageArea()
	e.ClearRow24()
	return interrupted
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

// showRest = ovr21:05F0 (unit): "Rest Time: dd:hh:mm", the unit being
// changed in white.
func (e *Engine) showRest(unit int) {
	var col [7]int
	for i := range col {
		col[i] = 0xA
	}
	col[unit] = 0xF
	r := &e.camp.rest
	e.DisplayString(1, 17, colLightGreen, "Rest Time:")
	x := 0xB
	for i := 4; i >= 3; i-- {
		e.DisplayString(x+1, 17, col[i], twoDigits(r[i]))
		e.DisplayString(x+3, 17, colLightGreen, ":")
		x += 3
	}
	e.DisplayString(x+1, 17, col[2], twoDigits(10*r[2]+r[1]))
}

// healTick = ovr21:0871: every 24 hours of rest everyone heals 1 HP.
func (e *Engine) healTick() {
	e.camp.heal++
	if e.camp.heal < 0x120 {
		return
	}
	healed := false
	for _, c := range e.party {
		if e.Heal(c, 1, false) {
			healed = true
		}
	}
	e.showRest(0)
	e.DisplayString(1, 19, colLightGreen, "The Whole Party Is Healed")
	if healed {
		e.DisplayPartyList()
	}
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
		if !slices.ContainsFunc(c.Memorized[:], func(v byte) bool { return v > 0x7F }) {
			e.setLearned(c, c.Memorized) // extra: the full set, to relearn (memorizeMenu)
		}
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
			if s <= 0x80 {
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
// +0x5A6 percent chance). Returns true when interrupted.
func (e *Engine) Rest(stop func() bool) bool {
	interrupted := false
	for i := range e.camp.study {
		e.camp.study[i] = 0
	}
	for i := 1; i < len(e.run.effTick); i++ {
		e.run.effTick[i] = 1
	}
	hours, shown := 0, 0
	e.ClearBox(1, msgTop, winRight, winBottom)
	e.showRest(0)
	e.resting = 1
	done := false
	r := &e.camp.rest
	for !done {
		if r[4] == 0 && r[3] == 0 && r[2] == 0 && r[1] == 0 || stop() {
			break
		}
		if e.keyHit() {
			e.showRest(0)
			if e.YesNo(colLightMagenta, colLightGreen, colWhite, "Stop Resting? ") {
				done = true
			} else {
				e.ClearRow24()
			}
		}
		if done {
			continue
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
					e.DisplayString(1, 19, colWhite, "The Party is rudely interrupted!")
					done = true
					interrupted = true
					e.MsgDelay()
				}
			}
		}
		e.idle() // let the window breathe; the DOS loop ran flat out
	}
	e.ClearBox(1, msgTop, winRight, winBottom)
	e.resting = 0
	return interrupted
}
