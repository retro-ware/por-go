package engine

import (
	"os"
	"slices"
	"strconv"

	"por/dax"
	"por/pal"
)

// ovr17 (second half): Modify, Train and Add on the party screen, the
// icon copy the designer uses (icondesign.go), and the hit point helpers
// they share.

// partyExp = es:[game+0x550]: the classes this training hall teaches
// (a mask that is ANDed with DS:5DE; the party menu only offers Train
// when it is set).
func (e *Engine) partyExp() int { return int(e.game.PartyExp) }

// ModifyCharacter = ovr17:209E, redone (extra) as the creation sheet on
// the selected character, experienced or not: Roll, Minimal, the fields
// and the pictures change it as at the creation; Keep keeps it, Esc puts
// the old character back. (The original raised and lowered the abilities
// and hit points within the race and class limits, and only for a
// character without experience.)
func (e *Engine) ModifyCharacter() {
	c := e.selected()
	i, was := e.partySel, *c
	k := kind{race: c.Race, sex: c.Sex, class: c.Class, align: c.Alignment}
	e.pushCreation(c, k, func(_, new *Character) { e.party[i] = new }, "Discard the changes?", func(n *Character, kept bool) {
		if kept {
			n.Share = 1
		} else {
			*c = was
			e.party[i] = c
			n = c
		}
		e.ApplyItems(n) // 00BA:0BB8
		e.BuildCharIcon(n, true)
	})
}

// conHPBonus = ovr17:3F31 (char): hit point bonus per class from
// constitution (DS:408E), plus one or two for a class 2 character with
// constitution above 16 and 17. Returned as a signed byte.
func (e *Engine) conHPBonus(c *Character) int {
	var sum byte
	for i := 0; i <= 7; i++ {
		if int8(c.Levels[i]) <= 0 {
			continue
		}
		adj := conHP[c.Stats[statCon]]
		if c.Stats[statCon] > 0x10 && c.Class >= clsFighter && c.Class <= clsRanger {
			adj++
		}
		if c.Stats[statCon] > 0x11 && c.Class >= clsFighter && c.Class <= clsRanger {
			adj++
		}
		if i == clsRanger && c.Levels[i] == 1 {
			adj *= 2 // the ranger's two first level dice (CotAB ovr018)
		}
		sum += byte(adj)
	}
	return int(int8(sum))
}

// hitPointsFrom = the tail of ovr17:3FB3/408C: sum plus the constitution
// bonus, averaged over the classes (count 0 would be a divide error in
// the original; 0 here).
func (e *Engine) hitPointsFrom(c *Character, sum byte, count int) byte {
	if count == 0 {
		return 0
	}
	adj := e.conHPBonus(c)
	if adj < 0 {
		if int(sum) > count+(-adj) {
			return byte((int(sum) + adj) / count)
		}
		return 1
	}
	return byte((int(sum) + adj) / count)
}

// minHitPoints = ovr17:3FB3 (char): the least hit points the levels allow.
func (e *Engine) minHitPoints(c *Character) byte {
	var sum byte
	count := 0
	for i := 0; i <= 7; i++ {
		if lv := int8(c.Levels[i]); lv > 0 {
			sum += byte(lv)
			count++
			if i == 4 || i == 7 {
				sum++
			}
		}
	}
	return e.hitPointsFrom(c, sum, count)
}

// maxHitPoints = ovr17:408C (char): the most hit points the levels allow
// (a full hit die per level, the die of each class).
func (e *Engine) maxHitPoints(c *Character) byte {
	var sum byte
	count := 0
	for i := 0; i <= 7; i++ {
		lv := int(int8(c.Levels[i]))
		if lv <= 0 {
			continue
		}
		count++
		switch i {
		case 0, 1:
			sum += byte(lv << 3)
		case 2, 3:
			sum += byte(lv * 10)
		case 4:
			sum += byte((lv + 1) << 3)
		case 5:
			sum += byte(lv << 2)
		case 6:
			sum += byte(lv * 6)
		case 7:
			sum += byte((lv + 1) << 2)
		}
	}
	return e.hitPointsFrom(e.selected(), sum, count) // the original reads the bonus of the selected character, not c
}

// rollHitDice = ovr17:4235 (char, mask): hit dice rolled for the classes in
// mask (DS:5DE bits); a first level die is at least two thirds of the die.
func (e *Engine) rollHitDice(c *Character, mask int) int {
	var total byte
	for i := 0; i <= 7; i++ {
		if int8(c.Levels[i]) <= 0 || hpClassMask[i]&byte(mask) == 0 {
			continue
		}
		n, sides := int(hitDieCount[i]), int(hitDieSides[i])
		if i == clsRanger && c.Levels[i] > 1 {
			n = 1 // two dice at first level only (CotAB hp_calc)
		}
		r := e.Dice(n, sides) // ovr25:0DE3
		if c.Levels[i] == 1 && r < (sides<<1)/3 {
			r = (sides << 1) / 3
		}
		total += byte(r)
	}
	return int(total)
}

// levelBlocked: the race's level limit stops class i at level lv.
func levelBlocked(race, str byte, i int, lv byte) bool {
	// Level limits of the races.
	skip := false
	switch race {
	case 1:
		skip = i == 2 && (lv == 8 && str == 0x11 || lv == 7 && str < 0x11)
	case 2:
		skip = i == 2 && (lv == 7 || lv == 6 && str == 0x11 || lv == 5 && str < 0x11)
	case 3:
		skip = i == 2 && (lv == 6 || lv == 5 && str < 0x12)
	case 4:
		if i == 0 && lv == 5 {
			skip = true
		} else if i == 2 || i == clsRanger { // the half-elf ranger as the fighter (CotAB)
			skip = lv == 8 || lv == 7 && str == 0x11 || lv == 6 && str < 0x11
		}
	case 5:
		skip = i == 2 && (lv == 6 || lv == 5 && str == 0x11 || lv == 4 && str < 0x11)
	}
	return skip
}

// levelReady (extra): c has the experience for a level in one of its
// classes; the roster shows the name in purple.
func levelReady(c *Character) bool {
	for i := 0; i <= 7; i++ {
		lv := c.Levels[i]
		if int8(lv) <= 0 || levelBlocked(c.Race, c.Stats[statStr], i, lv) {
			continue
		}
		if need := xpTable[i][int8(lv+1)]; need > 0 && need <= int32(c.Exp) {
			return true
		}
	}
	return false
}

// levelSpan (extra): the experience c's level took and its next level
// takes, for the class with the nearest next level; to is 0 when it can
// rise no further.
func levelSpan(c *Character) (from, to int32) {
	for i := 0; i <= 7; i++ {
		if lv := int8(c.Levels[i]); lv > 0 && lv < 9 { // columns 2..9 hold levels 2..9; the rest overlap other tables
			if n := xpTable[i][lv+1]; n > 0 && (to == 0 || n < to) {
				from, to = 0, n
				if lv > 1 {
					from = xpTable[i][lv]
				}
			}
		}
	}
	return from, to
}

// trainPrice = ovr17:29AC's fee in gold.
const trainPrice = 1000

// trainCheck = the tests of ovr17:29AC on c: the classes it would rise
// in (DS:5DE bits, the hall's game +0x550 among them), why not when the
// hall refuses, and the experience to hold it at below a second level
// (0: none).
func (e *Engine) trainCheck(c *Character) (mask byte, why string, best int32) {
	if c.Status != statusOkay && !e.debug {
		return 0, "we only train conscious people", 0
	}
	if e.purse.gold < trainPrice && !e.debug {
		return 0, "Training costs 1000 gp.", 0
	}
	var have, can byte
	race, str := c.Race, c.Stats[statStr]
	for i := 0; i <= 7; i++ {
		lv := c.Levels[i]
		if int8(lv) <= 0 {
			continue
		}
		have += hpClassMask[i]
		if levelBlocked(race, str, i, lv) {
			continue
		}
		exp := int32(c.Exp)
		need := xpTable[i][int8(lv+1)]
		if need > exp && !e.debug || need <= 0 {
			continue
		}
		can += hpClassMask[i]
		next := xpTable[i][int8(lv+2)]
		if next <= 0 || exp < next {
			continue
		}
		// Enough for two levels: the experience is held below the second.
		if next > best {
			best = next - 1
		}
	}
	if e.debug {
		return can, "", best
	}
	hall := e.partyExp()
	switch {
	case byte(int(have)&hall) == 0:
		return 0, "We don't train that class here", best
	case byte(int(can)&hall) == 0:
		return 0, "Not Enough Experience", best
	}
	return byte(int(can) & hall), "", best
}

// pushTraining = ovr17:29AC: the training hall takes decisions for the
// selected member until it leaves (or has trained).
func (e *Engine) pushTraining() {
	c := e.selected()
	if _, _, best := e.trainCheck(c); best > 0 {
		c.Exp = uint32(best) // as the original, before its question and refusals
	}
	e.learning = false
	e.push(modeTraining, func(a Action) bool {
		done := e.applyTraining(a)
		return done
	}, nil)
}

// applyTraining applies a decision in the training hall; done when it is
// left or the training is over.
func (e *Engine) applyTraining(a Action) (done bool) {
	c := e.selected()
	switch a.Kind {
	case ActClose:
		return !e.learning
	case ActTrain:
		mask, why, _ := e.trainCheck(c)
		if e.learning || why != "" {
			e.say(why)
			return false
		}
		e.trainLevels(c, mask)
		e.learning = len(e.learnable(c)) > 0 && e.learning
		return !e.learning
	case ActLearn:
		if e.learning && slices.Contains(e.learnable(c), byte(a.X)) {
			c.SpellBook[a.X-1] = 1
			e.learning = false
			return true
		}
	}
	return false
}

// learnable = SpellList kind 4 (ovr20:299C): the spells c has slots for
// and not in the book.
func (e *Engine) learnable(c *Character) []byte {
	var s []byte
	for i := 1; i <= lastSpell; i++ {
		if c.spellSlot(int(e.spellRec(byte(i), 0)), int(e.spellRec(byte(i), 1))) > 0 && c.SpellBook[i-1] == 0 {
			s = append(s, byte(i))
		}
	}
	return s
}

// trainLevels = the training of ovr17:29AC: a level in each class of
// mask for the fee, and the hit points; a mage level gained sets
// learning (the original's spell pick, ovr20:299C, came before the hit
// points; it rolls no dice).
func (e *Engine) trainLevels(c *Character, mask byte) {
	e.dasp("level")
	e.say("Congratulations...")
	if !e.debug {
		e.purse.gold -= trainPrice
	}
	count := 0
	mage := int8(c.Levels[clsMage])
	for i := 0; i <= 7; i++ {
		if int8(c.Levels[i]) <= 0 {
			continue
		}
		count++
		if hpClassMask[i]&mask > 0 {
			c.Levels[i]++
			if c.Drained > 0 {
				c.DrainedHP -= c.DrainedHP / c.Drained
				c.Drained--
			}
		}
	}
	e.RecomputeCharacter(c)                     // ovr24:0000
	e.learning = int8(c.Levels[clsMage]) > mage // (the original, with nothing to choose, set +0x32, the max HP, to 1)
	roll := e.rollHitDice(c, int(mask))
	per := roll / count
	if per == 0 {
		per = 1
	}
	c.HPRoll += byte(per)
	per = (roll + e.conHPBonus(c)) / count
	if per < 1 {
		per = 1
	}
	damage := c.MaxHP - c.HP
	c.MaxHP += byte(per)
	c.HP = c.MaxHP - damage
}

// trainingView is the hall's offer for the View.
func (e *Engine) trainingView() *Training {
	c := e.selected()
	if c == nil {
		return &Training{Why: "No one to train"}
	}
	v := &Training{Name: c.Name(), Price: trainPrice, Gold: int(e.purse.gold)}
	if e.learning {
		for _, s := range e.learnable(c) {
			v.Learn = append(v.Learn, BookSpell{ID: int(s), Name: e.spellName(int(s)), Level: spellGroups[e.spellRec(s, 1)]})
		}
		return v
	}
	mask, why, _ := e.trainCheck(c)
	v.Why = why
	for i := 0; i <= 7; i++ {
		if int8(c.Levels[i]) > 0 && hpClassMask[i]&mask > 0 {
			v.Becomes = append(v.Becomes, "a level "+strconv.Itoa(int(int8(c.Levels[i]))+1)+" "+classNames[i])
		}
	}
	return v
}

// AddCharacter = ovr17:306D: pick saved characters from the roster list
// and add them to the party (at most 6 characters and 8 with hired NPCs).
// Chosen entries get a "* " in the list (extra: the list is pickSaved's
// cards).
func (e *Engine) AddCharacter() {
	e.blank(1, 1, winRight, winBottom)
	names := e.BuildCharList() // ovr18:0010
	if len(names) == 0 {
		return
	}
	chars := e.loadSaved(names)
	count := 0 // uninitialised until the first add in the original
	e.pickSaved(&chars, &names, func(i int) bool {
		defer e.WriteCharList(names) // ovr18:01B8 (extra: on each pick)
		if c := chars[i]; c != nil && (names[i] == "" || names[i][0] != '*') {
			names[i] = cut("* "+names[i], textCols)
			count = 0
			if len(e.party) == 0 {
				e.game.PartyCount = 0
				e.AddToParty(c) // ovr18:1446
				e.BuildCharIcon(c, true)
			} else {
				var dup *Character
				for _, m := range e.party {
					if m.Name() == c.Name() && m.RandomID == c.RandomID {
						dup = m
						break
					}
					if m.Control < 0x80 {
						count++
					}
				}
				npc := c.Control >= 0x80
				refused := dup == nil && e.joinRefused(c)
				if refused {
					names[i] = names[i][2:] // unmarked again, as in CotAB
				}
				if dup == nil && !refused && (!npc && count < 6 || npc && e.game.PartyCount < 8) {
					e.AddToParty(c)
					e.BuildCharIcon(c, true)
					if !npc {
						count++
					}
				}
			}
		}
		return count > 5 || e.game.PartyCount > 7
	})
}

// joinRefused (CotAB ovr018): paladins do not join the evil, the evil do not
// join a paladin, and a party takes at most three rangers.
func (e *Engine) joinRefused(c *Character) bool {
	evil := func(c *Character) bool { return (c.Alignment+1)%3 == 0 }
	msg, rangers := "", 0
	for _, m := range e.party {
		switch {
		case int8(c.Levels[clsPaladin]) > 0 && evil(m):
			msg = "paladins do not join with evil scum"
		case evil(c) && int8(m.Levels[clsPaladin]) > 0:
			msg = m.Name() + " will tolerate no evil!"
		}
		if int8(m.Levels[clsRanger]) > 0 {
			rangers++
		}
	}
	if msg == "" && int8(c.Levels[clsRanger]) > 0 && rangers > 2 {
		msg = "too many rangers in party"
	}
	if msg != "" {
		e.say(msg)
	}
	return msg != ""
}

// ForgetCharacter (extra): pick a saved character (pickSaved's cards) and
// delete its files and its line in charlist.txt. Party members keep their
// copy in the party.
func (e *Engine) ForgetCharacter() {
	e.blank(1, 1, winRight, winBottom)
	names := e.BuildCharList()
	if len(names) == 0 {
		return
	}
	chars := e.loadSaved(names)
	p := e.pickSaved(&chars, &names, func(i int) bool {
		base := makeFileName(removeSubstr(names[i], "."))
		for _, ext := range []string{".cha", ".itm", ".spc", ".mem", ".png", ".icon.png"} {
			os.Remove(e.savePath(base + ext))
		}
		names, chars = slices.Delete(names, i, i+1), slices.Delete(chars, i, i+1)
		e.WriteCharList(names)
		return len(names) == 0
	})
	p.ask = "Forget %s forever?"
}

// iconCopy = ovr17:35BE (src, dst, recolour) nested in the icon editor (now the designer):
// copy both poses of icon slot src into slot dst (the original also
// copies the mask planes, which the port keeps in the image); with
// recolour the selected character's six colour pairs (+0xC1..) replace
// the DS:3CF5 colours in the copy.
func (e *Engine) iconCopy(src, dst byte, recolour bool) {
	for p := 0; p < 2; p++ {
		if im := e.icons[src][p]; im != nil {
			e.icons[dst][p] = im.Clone()
		}
	}
	if !recolour {
		return
	}
	c := e.selected()
	var to [16]byte
	for i := range to {
		to[i] = byte(i)
	}
	pos := defaultIconColours
	for i := 1; i <= 6; i++ {
		v := c.IconColour[i-1]
		to[pos[i]] = v & 0xF
		to[pos[i]+8] = v >> 4
	}
	for p := 0; p < 2; p++ {
		recolourIcon(e.icons[dst][p], to)
	}
}

// recolourIcon = 0464:1E5C (identity, to, 0, 0, icon): colour i becomes
// to[i] for i = 0..15 in turn, so a pixel already moved to a later colour
// moves again (3 -> 15 -> to[15]). Mask pixels are left alone. Amiga
// pixels (extra) go through amigaIconColour, and once only: the Amiga's
// routine (0x63240) looks each pixel's colour up a single time.
func recolourIcon(im *dax.Image, to [16]byte) {
	if im == nil {
		return
	}
	var f, am [16]byte
	for c := range f {
		x := byte(c)
		for i := x; i < 16; i++ {
			if x == i {
				x = to[i]
			}
		}
		f[c] = x
		am[amigaIconColour[c]] = amigaIconColour[to[c]]
	}
	for k, v := range im.Data {
		switch {
		case v < 16 && int(v) != im.Mask:
			im.Data[k] = f[v]
		case v >= pal.AmigaBase:
			im.Data[k] = v&^0xF | am[v&0xF]
		}
	}
}

// amigaIconColour (extra): the Amiga palette slot for each EGA colour
// number a character's icon colours hold. The Amiga icon editor (its slot
// table at 0x8B10E, against DS:3CF5) colours the parts in slots 9/1,
// 10/2, 11/3, 6/7, 5/8 and 15/14 where DOS uses 1/9 .. 7/15; the four
// left over pair by shade. So the DOS default colours give the Amiga
// default icon. The Amiga editor (0x1F11A) steps a part's colour through
// slots 1..15 only, never slot 0 (a second black): the designer offers
// those 15 in slot order on Amiga art.
var amigaIconColour = [16]byte{0, 9, 10, 11, 6, 4, 5, 15, 12, 1, 2, 3, 7, 13, 8, 14}
