package engine

import (
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
)

// Character creation and dropping, ported from ovr17.
//
// extra: the creation is one screen taking decisions, the character sheet
// of a sample character (View.Creation): its gender, race, alignment and
// class are drop-downs (an option that does not fit forces the dependants
// to their first valid value: ActSetField), the name and the ability
// scores are typed (ActName, ActStat), the portrait's head and body and
// the combat icon open pickers (ActPicker, ActIconEdit), and Roll rolls,
// to minimums too (ActRoll), Keep keeps (ActKeep). The drop-downs, the
// typed lines and the questions are the presenter's. The original's four
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
// in as its menus were answered. keep carries the name, portrait, icon and
// items over from the character it replaces (the abilities are rolled again:
// their limits are the race's and class's).
func (e *Engine) fresh(k kind, keep *Character) *Character {
	c := &Character{}
	for i := 1; i <= 6; i++ {
		t := defaultIconColours[i]
		c.IconColour[i-1] = byte((t+8)<<4 + t)
	}
	c.BaseAC = acTen
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
		c.Items = keep.Items // Modify: the gear stays with the character
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

// rollMinimal (extra: Minimal stats): takes the minimums (Text, as
// minimumsText; a field that is not a value keeps the last one), then rolls
// on until they are met; impossible minimums are refused.
func (e *Engine) rollMinimal(c *Character, text string) {
	for i, f := range strings.Fields(text) {
		if v, ok := parseMinStat(f, i == 0); ok && i < 6 {
			e.ext.minStats[i] = v
		}
	}
	best := bestStats(c)
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
			e.say(msg)
			return
		}
	}
	const most = 200000 // ponytail: a cap in place of "any key stops"; minimums this rare are given up
	for tries := 1; ; tries++ {
		e.rollCharacter(c)
		if e.meetsMinStats(c) {
			return
		}
		if tries == most {
			e.say("Not met in " + strconv.Itoa(most) + " rolls")
			return
		}
	}
}

// minimumsText is the minimums as rollMinimal reads them.
func (e *Engine) minimumsText() string {
	var f []string
	for i, v := range e.ext.minStats {
		f = append(f, fmtMin(i, v))
	}
	return strings.Join(f, " ")
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

// sheetScale is the creation sheet's portrait and combat icon scale: the
// 40x25 page's sizes times what the right of the page leaves room for (the
// combat stats reach column 37, the prompt row stays free).
func sheetScale() int { return max(1, min((winRight-37)/11, (winBottom-3)/14)) }

// sheetPortraitBox is the creation sheet's portrait box, against the right
// border: 11x11 cells at 1x (the head 5 rows, the body 6), times sheetScale.
func sheetPortraitBox() CellRect {
	s := sheetScale()
	return CellRect{winRight - 11*s + 1, 1, winRight, 11 * s}
}

// Where the sheet shows the combat icon's poses: under the portrait box.
func sheetIconRow() int { return 2 + 11*sheetScale() }

// sheetIconCol is the column of pose p's icon under the portrait box, as
// the engine draws it (at 1x: the presenter scales it by sheetScale from
// the icon box's corner).
func sheetIconCol(p int) int { return winRight - 10*sheetScale() + 1 + 5*p }

// sheetIconBox is the icons' cells at sheetScale.
func sheetIconBox() CellRect {
	s := sheetScale()
	x, y := sheetIconCol(0), sheetIconRow()
	return CellRect{x, y, x + 8*s - 1, y + 3*s - 1}
}

// creationSheet draws the sheet for the creation: without level, experience,
// party items and status, with the combat icon under the portrait. A PNG
// dropped on the icon becomes the icon, as in the icon editor (one dropped
// on the portrait the portrait, as on the sheet in play: ActPortrait).
func (e *Engine) creationSheet(c *Character) {
	e.CharacterSheet()
	e.BuildCharIcon(c, true)
	e.snap.iconBox = sheetIconBox()
	e.snap.iconDrop = func(im image.Image) {
		if e.takeIconPNG(c, im) {
			e.creationSheet(c) // the sheet draws itself anew
		}
	}
	for p := 0; p < 2; p++ {
		r := iconRect(sheetIconCol(p), sheetIconRow())
		e.erase(r)
		if c.Icon != nil { // a PNG: the presenter's, over the page at the window's resolution
			if sp := e.snap.sheet; sp != nil {
				sp.Icons[p] = iconPose(c.Icon, p, false)
			}
		} else if im := e.icons[c.Order][p]; im != nil {
			e.picture(sheetIconCol(p)*8, sheetIconRow()*8, im.Width*8, im.Height, im.Frame(0), im.Mask, im, 0)
		}
	}
}

// creation is the creation sheet up: c, a character of kind k.
type creation struct {
	c       *Character
	k       kind
	replace func(old, new *Character) // puts a fresh character where c was
	abandon string
}

// creationView is the sheet's parts for the View.
func (e *Engine) creationView() *Creation {
	cr := e.creation
	if cr == nil {
		return nil
	}
	c := cr.c
	v := &Creation{Abandon: cr.abandon, Minimums: e.minimumsText()}
	v.Name = CellRect{1, 1, max(1, len(c.Name())), 1}
	g := len(genderNames[c.Sex])
	for f := range v.Fields {
		names, cur, fits := cr.options(f)
		fl := &v.Fields[f]
		fl.Name, fl.Cur = [4]string{"Gender", "Race", "Alignment", "Class"}[f], cur
		for i, n := range names {
			fl.Options = append(fl.Options, Option{n, fits(i)})
		}
	}
	v.Fields[FieldSex].At = CellRect{1, 3, g, 3}
	v.Fields[FieldRace].At = CellRect{g + 2, 3, g + 1 + len(raceNames[int8(c.Race)]), 3}
	v.Fields[FieldAlign].At = CellRect{1, 4, len(alignmentNames[int8(c.Alignment)]), 4}
	v.Fields[FieldClass].At = CellRect{1, 5, len(className(c.Class)), 5}
	for i := range statNames {
		x1 := 6 // "STR 18", "(00)" after the strength
		if i == 0 {
			x1 += len(str00Text(c))
		}
		v.Stats[i] = CellRect{1, 7 + i, x1, 7 + i}
	}
	b, s := sheetPortraitBox(), sheetScale()
	v.Head, v.Body = CellRect{b.X1, 1, winRight, 5 * s}, CellRect{b.X1, 5*s + 1, winRight, b.Y2}
	v.Icon, v.Scale = sheetIconBox(), s
	return v
}

// options are field f's option names, the one c holds and which fit.
func (cr *creation) options(f int) ([]string, int, func(int) bool) {
	c := cr.c
	switch f {
	case FieldSex:
		return genderNames[:], int(c.Sex), func(int) bool { return true }
	case FieldRace:
		names := make([]string, len(pickRaces))
		for i, r := range pickRaces {
			names[i] = raceNames[r]
		}
		return names, slices.Index(pickRaces, c.Race), func(i int) bool { return classFor(pickRaces[i], c.Class) }
	case FieldAlign:
		return alignmentNames[:], int(c.Alignment), func(i int) bool { return alignFor(c.Class, byte(i)) }
	}
	classes := pickClasses()
	names := make([]string, len(classes))
	for i, cl := range classes {
		names[i] = classNames[cl]
	}
	return names, slices.Index(classes, c.Class), func(i int) bool { return classFor(c.Race, classes[i]) && alignFor(classes[i], c.Alignment) }
}

// CreateCharacter = ovr17:056C, redone as one screen (see the top of the
// file): the sheet of a sample character whose parts are picked in place;
// Keep saves it, ActClose abandons it.
func (e *Engine) CreateCharacter() {
	k := kind{race: raceHuman, sex: 0, class: clsFighter, align: 0}
	c := e.fresh(k, nil)
	e.rollCharacter(c)
	undo := e.selectTemp(c)
	e.pushCreation(c, k, func(old, new *Character) { undo(); undo = e.selectTemp(new) }, "Abandon this character?", func(c *Character, kept bool) {
		if kept {
			e.SaveCharacter(c, "") // ovr18:069E
		}
		undo()
		e.freeCharacter(&c)
	})
}

// quickCreate makes and saves the quickstart character q: rolled until its
// prime abilities are high, named and pictured from the template.
func (e *Engine) quickCreate(q *quickPick) {
	c := e.fresh(q.kind(), nil)
	e.rollCharacter(c)
	undo := e.selectTemp(c)
	for !q.good(c) {
		e.rollCharacter(c)
	}
	c.SetName(q.name)
	c.PortHead, c.PortBody = q.head, q.body
	e.SaveCharacter(c, "") // ovr18:069E
	undo()
	e.freeCharacter(&c)
}

// pushCreation (extra) puts the creation sheet up on c, a character of
// kind k (applyCreation); done gets the character as it ends up, kept by
// Keep or abandoned.
func (e *Engine) pushCreation(c *Character, k kind, replace func(old, new *Character), abandon string, done func(c *Character, kept bool)) {
	cr := &creation{c: c, k: k, replace: replace, abandon: abandon}
	kept := false
	e.creation = cr
	e.ext.creating = true
	off := e.redrawWith(func() { e.creationSheet(cr.c) })
	e.push(modeCreate, func(a Action) bool {
		d, k := e.applyCreation(cr, a)
		kept = k
		return d
	}, func() {
		off()
		e.creation, e.ext.creating = nil, false
		e.snap.iconDrop, e.snap.iconBox = nil, CellRect{}
		done(cr.c, kept)
	})
	e.creationSheet(c)
}

// applyCreation applies a decision on the creation sheet: done by Keep
// (kept) or ActClose.
func (e *Engine) applyCreation(cr *creation, a Action) (done, kept bool) {
	c := cr.c
	// rekind replaces c by a fresh character of kind k, rolled.
	rekind := func() {
		old := c
		c = e.fresh(cr.k, c)
		e.rollCharacter(c)
		cr.c = c
		cr.replace(old, c)
	}
	switch a.Kind {
	case ActKeep:
		return true, true
	case ActClose:
		return true, false
	case ActRoll:
		if a.N == 1 {
			e.rollMinimal(c, a.Text)
		} else {
			e.rollCharacter(c)
		}
	case ActName:
		if s := strings.Map(func(r rune) rune { // 0709:09E3 (extra: case kept)
			if r >= 0x20 && r <= 0x7A {
				return r
			}
			return -1
		}, a.Text); s != "" {
			c.SetName(cut(s, 0xF))
		}
	case ActStat:
		if a.N >= 0 && a.N < 6 {
			e.setStatTyped(c, a.N, a.Text)
		}
	case ActSetField:
		if a.N < 0 || a.N > FieldClass {
			return false, false
		}
		names, cur, _ := cr.options(a.N)
		if a.X < 0 || a.X >= len(names) || a.X == cur {
			return false, false
		}
		switch a.N {
		case FieldSex:
			c.Sex = byte(a.X)
		case FieldRace:
			cr.k.race = pickRaces[a.X]
			cr.k = cr.k.enforce(fRace)
			rekind()
		case FieldClass:
			cr.k.class = pickClasses()[a.X]
			cr.k = cr.k.enforce(fClass)
			rekind()
		case FieldAlign:
			cr.k.align = byte(a.X)
			if cr.k = cr.k.enforce(fAlign); cr.k.class != c.Class {
				rekind()
			} else {
				c.Alignment = cr.k.align
			}
		}
	case ActPicker:
		switch a.N {
		case PickHead:
			e.pickHead(int(c.PortHead), func(i int) { c.PortHead, c.Portrait = byte(i), nil })
		case PickBody:
			e.pickBody(int(c.PortBody), func(i int) { c.PortBody = byte(i) })
		}
		return false, false
	case ActIconEdit:
		e.pushIcon()
		return false, false
	case actRedraw:
	default:
		return false, false
	}
	e.creationSheet(c)
	return false, false
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
	return fmt.Sprintf("18/%02d", (v-1800)%100)
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
