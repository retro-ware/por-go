package engine

import (
	"slices"
	"strconv"
	"strings"
)

// Character creation and dropping, ported from ovr17.
//
// extra: the creation is one screen, the character sheet of a sample
// character: its name, gender, race, alignment and class are drop-down
// lists (a grey entry forces the dependants to their first valid value),
// the name and the ability scores are typed where they stand (inlineInput:
// the value gives way to the cursor), the portrait's head and body and the
// combat icon open pickers, and the bar rolls, rolls to minimums or keeps.
// A drop-down is a framed menu over the sheet with the current value
// first, on the field's text, and the others below; the pointer on any of
// them shows the selection background (region.glow). The original's four
// menus, name prompt, Head/Body stepping and icon editor states are gone.

// freeCharacter = ovr17:0000 (var c): disposes the combat block, the item
// and affect lists and the record, then nils the pointer. Go's collector
// does the freeing.
func (e *Engine) freeCharacter(c **Character) {
	if *c != nil {
		(*c).Combat, (*c).Items, (*c).Affects = nil, nil, nil
	}
	*c = nil
}

// selectTemp makes c the selected character (DS:5D92 = c) although it is not
// in the party; the returned func restores the old selection.
func (e *Engine) selectTemp(c *Character) func() {
	sel, n := e.partySel, len(e.party)
	e.party = append(e.party, c)
	e.partySel = n
	e.tempSel = c
	return func() {
		e.party = e.party[:n]
		e.partySel = sel
		e.tempSel = nil
	}
}

// kind is what the player chooses; the rest of the record follows from it.
type kind struct{ race, sex, class, align byte }

// The fields of a kind, for enforce.
const (
	fRace = iota
	fSex
	fClass
	fAlign
)

// pickRaces are the races offered, in list order (no half-orcs, as the original).
var pickRaces = []byte{raceDwarf, raceElf, raceGnome, raceHalfElf, raceHalfling, raceHuman}

// classFor: the race may be the class (DS:3E17).
func classFor(race, class byte) bool {
	rc := raceClasses[race]
	return slices.Contains(rc[1:1+rc[0]], class)
}

// alignFor: the class may be of the alignment (DS:3FE7).
func alignFor(class, align byte) bool {
	ca := classAlignments[class]
	return slices.Contains(ca[1:1+ca[0]], align)
}

// pickClasses are the classes any offered race may be, in class order.
func pickClasses() []byte {
	var out []byte
	for cl := byte(0); cl < byte(len(classNames)); cl++ {
		if slices.ContainsFunc(pickRaces, func(r byte) bool { return classFor(r, cl) }) {
			out = append(out, cl)
		}
	}
	return out
}

// enforce applies the rules after field f changed: a dependant that
// became invalid takes its first valid value. A class picked for a race
// that may not be it changes the race; an alignment the class may not be
// changes the class (every race has the fighter, who may be anything).
func (k kind) enforce(f int) kind {
	switch f {
	case fClass:
		if !classFor(k.race, k.class) {
			k.race = pickRaces[slices.IndexFunc(pickRaces, func(r byte) bool { return classFor(r, k.class) })]
		}
	case fAlign:
		if rc := raceClasses[k.race]; !alignFor(k.class, k.align) {
			k.class = rc[1+slices.IndexFunc(rc[1:1+rc[0]], func(cl byte) bool { return alignFor(cl, k.align) })]
		}
		return k
	}
	if !classFor(k.race, k.class) {
		k.class = raceClasses[k.race][1]
	}
	if !alignFor(k.class, k.align) {
		k.align = classAlignments[k.class][1]
	}
	return k
}

// fresh builds a new character of kind k: the record the original filled
// in as its menus were answered. keep carries the name, portrait and icon
// over from the character it replaces (the abilities are rolled again:
// their limits are the race's and class's).
func (e *Engine) fresh(k kind, keep *Character) *Character {
	c := &Character{}
	for i := 1; i <= 6; i++ {
		t := defaultIconColours[i]
		c.IconColour[i-1] = byte((t+8)<<4 + t)
	}
	c.BaseAC = 0x32
	c.HitDice = 0x28
	c.Status = statusOkay
	c.Okay = 1
	c.PortHead = 1
	c.PortBody = 1
	c.Size = 1
	c.RandomID = byte(e.Random(0x100))
	c.SetName("No-One")
	if keep != nil {
		c.PName, c.PortHead, c.PortBody, c.Portrait = keep.PName, keep.PortHead, keep.PortBody, keep.Portrait
		c.IconHead, c.IconBody, c.IconColour, c.Icon = keep.IconHead, keep.IconBody, keep.IconColour, keep.Icon
	}
	c.Race, c.Sex, c.Class, c.Alignment = k.race, k.sex, k.class, k.align
	switch c.Race {
	case raceHalfling:
		c.IconSize = 1
		e.AddAffect(c, affSaveBonusVsType0, 0, 0xFF, 0)
		e.AddAffect(c, affSaveBonusVsType42, 0, 0xFF, 0)
	case raceDwarf:
		c.IconSize = 1
		for _, t := range []affectType{affSaveBonusVsType0, affSaveBonusVsType42, affHitBonusVsTargetNames, affMinus4VsNames} {
			e.AddAffect(c, t, 0, 0xFF, 0)
		}
	case raceGnome:
		c.IconSize = 1
		for _, t := range []affectType{affSaveBonusVsType42, affHitBonusVsNames, affMinus4VsNames, affMinus4VsBugbearGnoll} {
			e.AddAffect(c, t, 0, 0xFF, 0)
		}
	case raceElf:
		c.IconSize = 2
		e.AddAffect(c, affBlock90, 0, 0xFF, 0)
	case raceHalfElf:
		c.IconSize = 2
		e.AddAffect(c, affBlock30, 0, 0xFF, 0)
	default:
		c.IconSize = 2
	}
	c.MaxLevel = 1
	class := int(int8(c.Class))
	switch {
	case class >= clsCleric && class <= clsMonk:
		c.Levels[class] = 1
	case class == clsClericFighter:
		c.Levels[clsCleric], c.Levels[clsFighter] = 1, 1
	case class == clsClericFighterMage:
		c.Levels[clsCleric], c.Levels[clsFighter], c.Levels[clsMage] = 1, 1, 1
	case class == clsClericRanger:
		c.Levels[clsCleric], c.Levels[clsRanger] = 1, 1
	case class == clsClericMage:
		c.Levels[clsCleric], c.Levels[clsMage] = 1, 1
	case class == clsClericThief:
		c.Levels[clsCleric], c.Levels[clsThief] = 1, 1
	case class == clsFighterMage:
		c.Levels[clsFighter], c.Levels[clsMage] = 1, 1
	case class == clsFighterThief:
		c.Levels[clsFighter], c.Levels[clsThief] = 1, 1
	case class == clsFighterMageThief:
		c.Levels[clsFighter], c.Levels[clsMage], c.Levels[clsThief] = 1, 1, 1
	case class == clsMageThief:
		c.Levels[clsMage], c.Levels[clsThief] = 1, 1
	}
	if class == clsPaladin {
		e.AddAffect(c, affProtEvil, 0, 0xFF, 0) // CotAB: a paladin's lasting protection from evil
	}
	if int8(c.Levels[clsThief]) > 0 {
		e.thiefSkills(c) // ovr24:0329
	}
	c.ClassMask = 0
	c.HitDice = 0
	for k := 0; k <= 7; k++ {
		if int8(c.Levels[k]) > 0 {
			if t := hitDiceTable[k][int8(c.Levels[k])]; t > c.HitDice {
				c.HitDice = t
			}
			c.ClassMask += classMask[k]
		}
	}
	e.savingThrows(c) // ovr24:0255

	// Starting age: single classes roll the dice, multi-classes use the
	// maximum.
	age := 0
	ages := &startAge[c.Race]
	if class <= clsMonk {
		a := ages[class]
		age = int(a.Base) + int(byte(e.Dice(int(a.Count), int(a.Sides))))
	} else {
		o := -1
		switch class {
		case 8, 9, 0xA, 0xB, 0xC: // 0xA (cleric/ranger) as in CotAB
			o = clsCleric
		case 0xD, 0xF, 0x10:
			o = clsMage
		case 0xE:
			o = clsFighter
		}
		if o >= 0 {
			age = int(ages[o].Base) + int(ages[o].Count)*int(ages[o].Sides)
		}
	}
	c.Age = uint16(age)
	return c
}

// limitStat keeps v within the race limits and the class minimum
// (DS:3DA3.., DS:3F81..); a cleric multi-class has wisdom 13 at least.
func limitStat(c *Character, i int, v byte) byte {
	lo, hi := raceLimits[c.Race].limits(i)
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	if m := int(classMinStats[c.Class][i]); int(v) < m {
		v = byte(m)
	}
	if i == statWis && v < 0xD && c.Class >= clsClericFighter && c.Class <= clsClericThief { // set [8..12] at cs:0525
		v = 0xD
	}
	return v
}

// adjustStat: age, race limits and class minimums on a rolled stat.
func adjustStat(c *Character, i int, v byte) byte {
	// Age steps DS:3F31 + race*10 (4 words) against the age word.
	older := func(j int) bool { return int(ageSteps[c.Race][j]) < int(c.Age) }
	switch i {
	case 0:
		if v != 0 && older(0) {
			v++
		}
		if v != 0 && older(1) {
			v--
		}
		if v != 0 && older(2) {
			v -= 2
		}
		if v != 0 && older(3) {
			v--
		}
	case 1:
		if older(1) {
			v++
		}
		if older(3) {
			v++
		}
	case 2:
		if !older(0) {
			v--
		}
		if older(1) {
			v++
		}
		if older(2) {
			v++
		}
		if older(3) {
			v++
		}
	case 3:
		if older(2) {
			v -= 2
		}
		if older(3) {
			v--
		}
	case 4:
		if older(1) {
			v--
		}
		if older(2) {
			v--
		}
		if older(3) {
			v--
		}
	}
	return limitStat(c, i, v)
}

// bestStats (extra: Minimal stats): the most each stat can reach, from a
// roll of 18, in the units of minStats (STR as strScore).
func bestStats(c *Character) [6]int {
	var best [6]int
	for i := range best {
		v := adjustStat(c, i, 18)
		best[i] = int(v)
		if i == 0 {
			best[0] = strScore(v, 0)
			if v == 0x12 && fighterKind(c) {
				best[0] = strScore(v, min(100, raceLimits[c.Race].Str00Max))
			}
		}
	}
	return best
}

// rollStats rolls the abilities: the best of six 3d6 each, then the age,
// race and class limits. (The original's race adjustments on the dice
// never applied: their switch read the race menu's loop variable, left at
// 7, so they are not here.)
func (e *Engine) rollStats(c *Character) {
	c.Str00 = 0
	for i := 0; i <= 5; i++ {
		st := &c.Stats[i]
		*st = 0
		for n := 1; n <= 6; n++ {
			if roll := byte(e.Dice(3, 6)); *st < roll {
				*st = roll
			}
		}
		*st = adjustStat(c, i, *st)
		if i == 0 && *st == 0x12 && fighterKind(c) {
			c.Str00 = byte(e.Random(100) + 1)
			if m := raceLimits[c.Race].Str00Max; m < c.Str00 {
				c.Str00 = m
			}
		}
	}
}

// rollHP rolls the hit points (one die per class, averaged, with the
// constitution adjustment), the starting money and the first spells.
func (e *Engine) rollHP(c *Character) {
	c.HP = c.MaxHP
	c.BaseAttack[0], c.BaseDice[0], c.BaseSides[0], c.StrBonus, c.BaseMove = 2, 1, 2, 1, 0xC
	var sum, count byte
	for k := 0; k <= 7; k++ {
		if int8(c.Levels[k]) <= 0 {
			continue
		}
		if k == 0 {
			c.SpellSlots[0] = 1
		} else if k == 5 {
			c.SpellSlots[3] = 1
		}
		sum += byte(e.Dice(int(startHPDice[k][0]), int(startHPDice[k][1])))
		count++
		if k == 0 {
			e.wisdomSlots(c) // ovr24:01AF
			for s := 1; s <= lastSpell; s++ {
				if rec := spellTable[s][:2]; rec[0] == 0 && rec[1] == 1 {
					c.SpellBook[s-1] = 1
				}
			}
		} else if k == 5 {
			c.SpellBook[10], c.SpellBook[17], c.SpellBook[18], c.SpellBook[20] = 1, 1, 1, 1
		}
	}
	c.Coins[3] = uint16(int16(int(sum) * 10 / int(count)))
	c.HPRoll = byte(e.rollHitDice(c, 0xFF))
	c.MaxHP = c.HPRoll
	adj := int(int8(e.conHPBonus(c)))
	if adj < 0 {
		if int(c.MaxHP) > -adj+int(count) {
			c.MaxHP = byte((int(c.MaxHP) + adj) / int(count))
		} else {
			c.MaxHP = 1
		}
	} else {
		c.MaxHP = byte((int(c.MaxHP) + adj) / int(count))
	}
	c.HP = c.MaxHP
	c.HPRoll = byte(int(c.HPRoll) / int(count))
}

// rollCharacter is one roll: abilities, then hit points.
func (e *Engine) rollCharacter(c *Character) {
	e.rollStats(c)
	e.rollHP(c)
}

// rollMinimal (extra: Minimal stats): asks the minimums, then rolls on
// until they are met or a key stops it; impossible minimums are refused.
func (e *Engine) rollMinimal(c *Character) {
	best := bestStats(c)
	e.askMinStats()
	for i, name := range statNames {
		if e.ext.minStats[i] > best[i] {
			max := strconv.Itoa(best[i])
			if i == 0 {
				max = fmtStr(best[0])
			}
			msg := name + " " + fmtMin(i, e.ext.minStats[i]) + " impossible here, max " + max
			if i == 0 && best[0] == 1800 {
				msg = "STR 18/xx is for fighters only, max 18"
			}
			e.PromptRow24(colYellow, cut(msg, textCols))
			e.ClearRow24()
			return
		}
	}
	for tries := 1; ; tries++ {
		e.rollCharacter(c)
		if e.meetsMinStats(c) {
			break
		}
		if tries%100 == 0 {
			for i := 0; i <= 5; i++ {
				e.showStat(i, false)
			}
			e.ClearRow24()
			e.DisplayString(0, promptRow, colYellow, "Rolling "+strconv.Itoa(tries)+"... any key stops")
			e.Delay(16) // let the window draw and see keys
			if e.keyHit() {
				break
			}
		}
	}
	e.ClearRow24()
}

var statNames = [6]string{"STR", "INT", "WIS", "DEX", "CON", "CHA"}

// setStatTyped (extra) gives ability i the value typed on the sheet (STR as
// 18/xx), kept within the race and class limits; what is not a value keeps
// it. Wisdom and constitution carry their spell slots and hit points
// along, as Modify does.
func (e *Engine) setStatTyped(c *Character, i int, s string) {
	v, ok := parseMinStat(s, i == 0)
	if !ok {
		return
	}
	st, x := byte(v), byte(0)
	if i == 0 {
		st = byte(v / 100)
		if v > 1800 && v <= 1900 {
			st, x = 18, byte(v-1800)
		}
	}
	c.Stats[i] = limitStat(c, i, st)
	if i == 0 {
		if c.Stats[0] != 18 || !fighterKind(c) {
			x = 0
		}
		c.Str00 = min(x, raceLimits[c.Race].Str00Max)
	}
	switch i {
	case statWis:
		if c.SpellSlots[0] > 0 {
			c.SpellSlots[0] = 1
		}
		e.wisdomSlots(c)
	case statCon:
		c.MaxHP = min(max(c.MaxHP, e.minHitPoints(c)), e.maxHitPoints(c))
		c.HP = c.MaxHP
	}
}

// The sheet's clickable parts (what a click wants, besides the bar).
const (
	wantNone = iota
	wantName
	wantSex
	wantRace
	wantAlign
	wantClass
	wantHead
	wantBody
	wantIcon
	wantStat // + the ability's index (six regions)
)

// Where the sheet shows the combat icon's poses: under the portrait box.
const sheetIconRow = 13

// sheetTallRow is the creation sheet's blank row the 4:3 window repeats:
// under the combat stats, so the drop-downs' boxes (bottom border at
// sheetTallRow-1 at most) never cross it.
const sheetTallRow = 20

var sheetIconCol = [2]int{29, 34}

// creationSheet draws the sheet for the creation: without level, experience,
// party items and status, with the combat icon under the portrait.
func (e *Engine) creationSheet(c *Character) {
	e.CharacterSheet()
	e.snap.tallRow = sheetTallRow
	e.BuildCharIcon(c, true)
	for p := 0; p < 2; p++ {
		if c.Icon != nil { // a PNG: over the page at the window's resolution
			e.pg.overlays = append(e.pg.overlays, Overlay{Img: iconPose(c.Icon, p, false), At: iconRect(sheetIconCol[p], sheetIconRow)})
		} else if im := e.icons[c.Order][p]; im != nil {
			e.picture(sheetIconCol[p]*8, sheetIconRow*8, im.Width*8, im.Height, im.Frame(0), im.Mask)
		}
	}
}

// sheetRegions makes the sheet's parts clickable: each click records what
// it wants and ends the bar's wait (PickRedraw). The texts that are
// drop-downs glow under the pointer.
func (e *Engine) sheetRegions(c *Character, want *int) {
	at := func(x0, y0, x1, y1, w int, glow bool) {
		e.addRegion(region{x0: x0, y0: y0, x1: x1, y1: y1, glow: glow, click: func(int, int) Action {
			*want = w
			return Action{Kind: actRedraw}
		}})
	}
	at(1, 1, len(c.Name()), 1, wantName, true)
	g := len(genderNames[c.Sex])
	at(1, 3, g, 3, wantSex, true)
	at(g+2, 3, g+1+len(raceNames[int8(c.Race)]), 3, wantRace, true)
	at(1, 4, len(alignmentNames[int8(c.Alignment)]), 4, wantAlign, true)
	at(1, 5, len(className(c.Class)), 5, wantClass, true)
	for i := range statNames {
		x1 := 6 // "STR 18", "(00)" after the strength
		if i == 0 {
			x1 += len(str00Text(c))
		}
		at(1, 7+i, x1, 7+i, wantStat+i, true)
	}
	at(28, 1, winRight, 5, wantHead, false)
	at(28, 6, winRight, 11, wantBody, false)
	at(sheetIconCol[0], sheetIconRow, sheetIconCol[1]+2, sheetIconRow+2, wantIcon, false)
}

// dropDown (extra) opens a sheet field's menu: a framed box from column
// x, row y holding every item, the current one first, over the field's
// text, the rest below it in their order, going on into further columns
// where they would reach the repeated row; green where green says so, grey
// otherwise (a grey item still picks). The arrows, Enter and Esc, the
// pointer and a click work as in a list. The index picked, or -1.
func (e *Engine) dropDown(x, y int, items []string, cur int, green func(i int) bool) int {
	order := []int{max(cur, 0)}
	for i := range items {
		if i != order[0] {
			order = append(order, i)
		}
	}
	rows := min(len(order), sheetTallRow-1-y) // the bottom border above the repeated row
	cols, w := (len(order)+rows-1)/rows, 0
	for _, s := range items {
		w = max(w, len(s))
	}
	x2, y2 := x+cols*(w+1)-2, y+rows-1
	e.DrawBoxTitled(x, y, x2, y2, 0, colBlack, "")
	at := func(k int) (int, int) { return x + k/rows*(w+1), y + k%rows }
	draw := func(k int, hi bool) {
		cx, cy := at(k)
		col, bg := colLightGreen, colBlack
		if !green(order[k]) {
			col = dimColour
		}
		if hi {
			bg = selBg(col, colWhite)
		}
		for j, ch := range []byte(items[order[k]]) {
			e.displayCharBg(cx+j, cy, col, bg, ch, true)
		}
	}
	for k := range order {
		draw(k, false)
	}
	index := 0
	hit := func(cx, cy int) int { // the item at a cell, or -1
		for k := range order {
			if ix, iy := at(k); cy == iy && cx >= ix && cx <= ix+w {
				return k
			}
		}
		return -1
	}
	move := func(k int) {
		draw(index, false)
		index = k
		draw(index, true)
	}
	mark := e.regionMark()
	defer e.regionCut(mark)
	e.addRegion(region{x0: x, y0: y, x1: x2, y1: y2, click: func(cx, cy int) Action {
		k := hit(cx, cy)
		if k < 0 {
			return Action{}
		}
		move(k) // a click picks at once, as in a list without words
		return Action{Kind: ActContinue}
	}, hover: func(cx, cy int) {
		if k := hit(cx, cy); k >= 0 && k != index {
			move(k)
		}
	}})
	for {
		draw(index, true)
		e.ext.list, e.ext.picker = true, true
		p := e.DisplayInput("", Bar{{"Exit", PickExit}}, colLightMagenta, colLightGreen, colWhite, true, false)
		e.ext.list, e.ext.picker = false, false
		n := len(order)
		switch p.ID {
		case PickEnter:
			return order[index]
		case PickMove:
			switch p.Dir {
			case DirUp, DirWheelUp:
				move((index + n - 1) % n)
			case DirDown, DirWheelDown:
				move((index + 1) % n)
			case DirHome:
				move(0)
			case DirEnd:
				move(n - 1)
			case DirLeft:
				move(max(index-rows, 0))
			case DirRight:
				move(min(index+rows, n-1))
			}
		case PickRedraw: // a click moved the highlight
		default:
			return -1
		}
	}
}

// CreateCharacter = ovr17:056C, redone as one screen (see the top of the
// file): the sheet of a sample character whose parts are picked in place;
// Roll and Minimal make the abilities, Keep saves it. Esc asks, then
// abandons it.
func (e *Engine) CreateCharacter() {
	k := kind{race: raceHuman, sex: 0, class: clsFighter, align: 0}
	q := e.ext.quick
	if q != nil {
		k = q.kind()
	}
	c := e.fresh(k, nil)
	e.rollCharacter(c)
	undo := e.selectTemp(c)
	if q != nil { // quickstart: rolled until its prime abilities are high, named and pictured from the template
		for !q.good(c) {
			e.rollCharacter(c)
		}
		c.SetName(q.name)
		c.PortHead, c.PortBody = q.head, q.body
		e.SaveCharacter(c, "") // ovr18:069E
		undo()
		e.freeCharacter(&c)
		return
	}
	e.ext.creating = true
	defer func() { e.ext.creating = false }()
	// rekind replaces c by a fresh character of kind k, rolled.
	rekind := func() {
		undo()
		c = e.fresh(k, c)
		e.rollCharacter(c)
		undo = e.selectTemp(c)
	}
	const (
		roll Choice = iota
		minimal
		keep
	)
	for {
		e.creationSheet(c)
		want := wantNone
		mark := e.regionMark()
		e.sheetRegions(c, &want)
		p := e.DisplayInput("", Bar{{"Roll", roll}, {"Minimal", minimal}, {"Keep", keep}}, colLightMagenta, colLightGreen, colWhite, true, false)
		e.regionCut(mark)
		switch {
		case p.ID == keep:
			e.SaveCharacter(c, "") // ovr18:069E
			undo()
			e.freeCharacter(&c)
			return
		case p.ID == NoPick:
			if e.YesNo(colYellow, colLightGreen, colWhite, "Abandon this character? ") {
				undo()
				e.freeCharacter(&c)
				return
			}
		case p.ID == roll:
			e.rollCharacter(c)
		case p.ID == minimal:
			e.rollMinimal(c)
		case p.ID != PickRedraw:
		case want == wantName:
			if s, ok := e.inlineInput(1, 1, 0xF, colLightCyan, func(c byte) bool { return c >= 0x20 && c <= 0x7A }); ok && s != "" { // 0709:09E3 (extra: case kept)
				c.SetName(s)
			}
		case want == wantSex:
			if i := e.dropDown(1, 3, genderNames[:], int(c.Sex), func(int) bool { return true }); i >= 0 {
				c.Sex = byte(i)
			}
		case want == wantRace:
			names := make([]string, len(pickRaces))
			for i, r := range pickRaces {
				names[i] = raceNames[r]
			}
			if i := e.dropDown(len(genderNames[c.Sex])+2, 3, names, slices.Index(pickRaces, c.Race), func(i int) bool { return classFor(pickRaces[i], c.Class) }); i >= 0 && pickRaces[i] != c.Race {
				k.race = pickRaces[i]
				k = k.enforce(fRace)
				rekind()
			}
		case want == wantClass:
			classes := pickClasses()
			names := make([]string, len(classes))
			for i, cl := range classes {
				names[i] = classNames[cl]
			}
			if i := e.dropDown(1, 5, names, slices.Index(classes, c.Class), func(i int) bool { return classFor(c.Race, classes[i]) && alignFor(classes[i], c.Alignment) }); i >= 0 && classes[i] != c.Class {
				k.class = classes[i]
				k = k.enforce(fClass)
				rekind()
			}
		case want == wantAlign:
			if i := e.dropDown(1, 4, alignmentNames[:], int(c.Alignment), func(i int) bool { return alignFor(c.Class, byte(i)) }); i >= 0 && byte(i) != c.Alignment {
				k.align = byte(i)
				if k = k.enforce(fAlign); k.class != c.Class {
					rekind()
				} else {
					c.Alignment = k.align
				}
			}
		case want == wantHead:
			if i := e.pickHead(int(c.PortHead)); i > 0 {
				c.PortHead, c.Portrait = byte(i), nil
			}
		case want == wantBody:
			if i := e.pickBody(int(c.PortBody)); i > 0 {
				c.PortBody = byte(i)
			}
		case want == wantIcon:
			e.iconEditor()
		case want >= wantStat:
			i := want - wantStat
			if s, ok := e.inlineInput(5, 7+i, 5, colLightGreen, func(c byte) bool { return c >= '0' && c <= '9' || c == '/' }); ok {
				e.setStatTyped(c, i, s)
			}
		}
	}
}

// DropCharacter = ovr17:1F37: asks twice, then the selected member leaves
// the party (a hired NPC bids farewell, anyone else is dumped).
func (e *Engine) DropCharacter() {
	if len(e.party) == 0 {
		return
	}
	c := e.selected()
	if e.YesNo(colYellow, colLightGreen, colWhite, "Drop "+c.Name()+" forever? ") &&
		e.YesNo(colYellow, colLightGreen, colWhite, "Are you sure? ") {
		if c.Okay == 0 {
			e.BottomMessage("You dump " + c.Name() + " out back.")
		} else {
			e.BottomMessage(c.Name() + " bids you farewell.")
		}
		e.RemoveSelected(false, true) // ovr17:3291
		return
	}
	e.BottomMessage(c.Name() + " breathes a sigh of relief.")
}

// strScore folds 18/xx into one number: 18/01 = 1801, 18/00 = 1900.
func strScore(st, x byte) int {
	if st == 18 && x > 0 {
		return 1800 + int(x)
	}
	return int(st) * 100
}

func (e *Engine) meetsMinStats(c *Character) bool {
	for i, m := range e.ext.minStats {
		v := int(c.Stats[i])
		if i == 0 {
			v = strScore(c.Stats[0], c.Str00)
		}
		if v < m {
			return false
		}
	}
	return true
}

// askMinStats (extra): one prompt per stat; blank keeps the last value.
func (e *Engine) askMinStats() {
	for i, name := range statNames {
		for {
			cur := strconv.Itoa(e.ext.minStats[i])
			if i == 0 {
				cur = fmtStr(e.ext.minStats[0])
			}
			s := strings.TrimSpace(e.InputString("Minimum "+name+" ("+cur+"): ", colLightMagenta, 6))
			if s == "" {
				break
			}
			if v, ok := parseMinStat(s, i == 0); ok {
				e.ext.minStats[i] = v
				break
			}
		}
	}
}

func fmtMin(i, v int) string {
	if i == 0 {
		return fmtStr(v)
	}
	return strconv.Itoa(v)
}

func fmtStr(v int) string {
	if v <= 1800 || v > 1900 {
		return strconv.Itoa(v / 100)
	}
	x := v - 1800
	if x == 100 {
		return "18/00"
	}
	if x < 10 {
		return "18/0" + strconv.Itoa(x)
	}
	return "18/" + strconv.Itoa(x)
}

func parseMinStat(s string, str bool) (int, bool) {
	a, b, slash := strings.Cut(s, "/")
	n, err := strconv.Atoi(a)
	if err != nil || n < 0 || n > 25 {
		return 0, false
	}
	if !slash {
		if str {
			return n * 100, true
		}
		return n, true
	}
	x, err := strconv.Atoi(b)
	if !str || n != 18 || err != nil || x < 0 || x > 99 {
		return 0, false
	}
	if x == 0 {
		x = 100
	}
	return strScore(18, byte(x)), true
}
