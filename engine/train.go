package engine

import (
	"image"
	"os"
	"slices"
	"strconv"

	"por/dax"
	"por/pal"
)

// ovr17 (second half): Modify, Train and Add on the party screen, the
// portrait and icon editors, and the hit point helpers they share.

// partyExp = es:[game+0x550]: the classes this training hall teaches
// (a mask that is ANDed with DS:5DE; the party menu only offers Train
// when it is set).
func (e *Engine) partyExp() int { return int(e.game.PartyExp) }

// modifyDraw = ovr17:2033 (idx, highlight): the ability line idx of the
// selected character, or its hit points for idx 6.
func (e *Engine) modifyDraw(idx int, hi bool) {
	if idx < 6 {
		e.showStat(idx, hi) // ovr20:0901
	} else {
		e.displayHP(e.selected(), 4, 0x12, hi) // 00BA:0973
	}
}

// ModifyCharacter = ovr17:209E: change the abilities and hit points of a
// character that has no experience yet, within the race and class limits
// (DS:3DA3.., DS:3F81..). Left/right change the line, up/down move,
// Keep (K) accepts, Exit (E) puts the old values back.
func (e *Engine) ModifyCharacter() {
	c := e.selected()
	if int32(c.Exp) > 0 {
		e.DisplayMessageWait(colYellow, c.Name()+" can't be modified.")
		return
	}
	e.CharacterSheet() // ovr20:004A
	var saved [6]byte
	copy(saved[:], c.Stats[:])
	savedStr00, savedHP := c.Str00, c.MaxHP
	race, class := int(c.Race), int(c.Class)
	idx := 6
	e.modifyDraw(idx, false)
	idx = 0
	e.modifyDraw(idx, true)
	// hpToDraw redraws the hit point line, leaving the cursor on idx.
	hpToDraw := func() {
		c.HP = c.MaxHP
		save := idx
		idx = 6
		e.modifyDraw(idx, false)
		idx = save
	}
	// slots: when wisdom changes, a first level cleric slot is capped at 1.
	wisdom := func() {
		if c.SpellSlots[0] > 0 {
			c.SpellSlots[0] = 1
		}
		e.wisdomSlots(c) // ovr24:01AF
	}
	lower := func(i int) {
		c.Stats[i]--
		if i == 0 && c.Str00 > 0 {
			c.Str00--
			c.Stats[statStr]++
		} else if lo, _ := raceLimits[race].limits(i); c.Stats[i] < lo || c.Stats[i] < classMinStats[class][i] {
			// Quirk: below the class minimum the race minimum is stored.
			c.Stats[i] = lo
		}
		switch i {
		case 2:
			wisdom()
		case 4:
			if m := e.maxHitPoints(c); m < c.MaxHP {
				c.MaxHP = m
			}
			hpToDraw()
		}
	}
	raise := func(i int) {
		c.Stats[i]++
		if _, hi := raceLimits[race].limits(i); c.Stats[i] > hi {
			if i == 0 && c.Stats[statStr] > 0x12 && fighterKind(c) && raceLimits[race].Str00Max > c.Str00 {
				c.Str00++ // exceptional strength for fighters
			}
			c.Stats[i] = hi
		}
		switch i {
		case 2:
			wisdom()
		case 4:
			if m := e.minHitPoints(c); m > c.MaxHP {
				c.MaxHP = m
			}
			hpToDraw()
		}
	}
	defer e.rawArrows()()
	for {
		// extra (nav.go): a click on a score or the hit points selects it;
		// Raise and Lower on the menu bar change it
		mark := e.regionMark()
		pick := func(i int) {
			e.modifyDraw(idx, false)
			idx = i
		}
		e.addRegion(region{x0: 0, y0: 7, x1: 0xB, y1: 12, click: func(_, cy int) Action { pick(cy - 7); return Action{Kind: actRedraw} }})
		e.addRegion(region{x0: 0, y0: 0x12, x1: 0xB, y1: 0x12, click: func(int, int) Action { pick(6); return Action{Kind: actRedraw} }})
		const (
			keep Choice = iota
			raiseIt
			lowerIt
			exit
		)
		p := e.DisplayInput("Modify character: ", Bar{{"Keep", keep}, {"Raise", raiseIt}, {"Lower", lowerIt}, {"Exit", exit}}, colLightMagenta, colLightGreen, colWhite, true, false)
		e.regionCut(mark)
		switch p.ID {
		case raiseIt:
			p = Pick{ID: PickMove, Dir: DirRight}
		case lowerIt:
			p = Pick{ID: PickMove, Dir: DirLeft}
		}
		e.modifyDraw(idx, false)
		if p.ID == exit || p.ID == NoPick {
			copy(c.Stats[:], saved[:])
			c.Str00, c.MaxHP = savedStr00, savedHP
			c.HP = c.MaxHP
			e.ApplyItems(c) // 00BA:0BB8
			return
		}
		if p.ID == PickMove {
			switch p.Dir {
			case DirDown:
				if idx++; idx > 6 {
					idx = 0
				}
			case DirUp:
				if idx--; idx < 0 {
					idx = 6
				}
			case DirLeft:
				if idx < 6 {
					lower(idx)
				} else {
					c.MaxHP--
					if m := e.minHitPoints(c); m > c.MaxHP {
						c.MaxHP = m
					}
					c.HP = c.MaxHP
				}
			case DirRight:
				if idx < 6 {
					raise(idx)
				} else {
					c.MaxHP++
					if m := e.maxHitPoints(c); m < c.MaxHP {
						c.MaxHP = m
					}
					c.HP = c.MaxHP
				}
			}
		}
		e.ApplyItems(c)
		e.showCombatStats() // ovr20:0608
		e.modifyDraw(idx, true)
		if p.ID == keep {
			c.Share = 1
			return
		}
	}
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

// TrainCharacter = ovr17:29AC: raise the classes whose experience
// (DS:409B table) allows it and which this hall teaches (game +0x550),
// for 1000 gp. Race level limits apply; a mage level gained offers a
// new spell for the book.
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

func (e *Engine) TrainCharacter() {
	c := e.selected()
	if c.Status != statusOkay && !e.debug {
		e.DisplayMessageWait(colYellow, "we only train conscious people")
		return
	}
	if e.purse.gold < 1000 && !e.debug {
		e.DisplayMessageWait(colYellow, "Training costs 1000 gp.")
		return
	}
	classMask := func(i int) byte { return hpClassMask[i] }
	var have, can byte
	var best int32
	race, str := c.Race, c.Stats[statStr]
	for i := 0; i <= 7; i++ {
		lv := c.Levels[i]
		if int8(lv) <= 0 {
			continue
		}
		have += classMask(i)
		skip := levelBlocked(race, str, i, lv)
		if skip {
			continue
		}
		exp := int32(c.Exp)
		need := xpTable[i][int8(lv+1)]
		if need > exp && !e.debug || need <= 0 {
			continue
		}
		can += classMask(i)
		next := xpTable[i][int8(lv+2)]
		if next <= 0 || exp < next {
			continue
		}
		// Enough for two levels: the experience is held below the second.
		if next > best {
			best = next - 1
		}
	}
	if best > 0 {
		c.Exp = uint32(best)
	}
	hall := e.partyExp()
	if !e.debug {
		if byte(int(have)&hall) == 0 {
			e.DisplayMessageWait(colYellow, "We don't train that class here")
			return
		}
		if byte(int(can)&hall) == 0 {
			e.DisplayMessageWait(colYellow, "Not Enough Experience")
			return
		}
	}
	mask := can
	if !e.debug {
		mask = byte(int(can) & hall)
	}
	e.ClearCells(1, 1, winRight, winBottom)
	y := 4
	e.displayName(c, 4, y, false) // 00BA:1816
	e.DisplayString(len(c.Name())+4, y, colLightGreen, " will become:")
	for i := 0; i <= 7; i++ {
		if int8(c.Levels[i]) > 0 && classMask(i)&mask > 0 {
			y++
			pre := "and a level "
			if y == 5 {
				pre = "    a level "
			}
			e.DisplayString(6, y, colLightGreen, pre+strconv.Itoa(int(int8(c.Levels[i]))+1)+" "+classNames[i])
		}
	}
	if !e.YesNo(colLightMagenta, colLightGreen, colWhite, "Do you wish to train? ") {
		return
	}
	e.BottomMessage("Congratulations...")
	if !e.debug {
		e.purse.gold -= 1000
	}
	count := 0
	mage := int8(c.Levels[clsMage])
	for i := 0; i <= 7; i++ {
		if int8(c.Levels[i]) <= 0 {
			continue
		}
		count++
		if classMask(i)&mask > 0 {
			c.Levels[i]++
			if c.Drained > 0 {
				c.DrainedHP -= c.DrainedHP / c.Drained
				c.Drained--
			}
		}
	}
	e.RecomputeCharacter(c) // ovr24:0000
	if int8(c.Levels[clsMage]) > mage {
		idx := -1
		var spell byte
		any := true
		for {
			spell = e.SpellList(4, 4, &idx, &any) // ovr20:299C
			if spell > 0 || !any {
				break
			}
		}
		if spell > 0 { // (the original, with nothing to choose, set +0x32, the max HP, to 1)
			c.SpellBook[spell-1] = 1
		}
	}
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

// AddCharacter = ovr17:306D: pick saved characters from the roster list
// and add them to the party (at most 6 characters and 8 with hired NPCs).
// Chosen entries get a "* " in the list.
func (e *Engine) AddCharacter() {
	e.ClearCells(1, 1, winRight, winBottom)
	names := e.BuildCharList() // ovr18:0010
	if len(names) == 0 {
		return
	}
	list := make([]*ListNode, len(names))
	for i, n := range names {
		list[i] = &ListNode{Text: n}
	}
	index := 0
	redraw := true
	var sel *ListNode
	count := 0 // uninitialised until the first add in the original
	for {
		// The show-exit argument is an uninitialised local; false here.
		const add Choice = 0
		key := e.ListMenu("Add a character: ", Bar{{"Add", add}, {"Exit", PickExit}}, colLightMagenta, colLightGreen, colWhite, 1, 2, 38, 22, list, false, &redraw, &index, &sel).ID
		if (key == PickEnter || key == add) && sel != nil && (sel.Text == "" || sel.Text[0] != '*') {
			c := &Character{}
			if e.LoadCharacter(sel.Text, c, false) { // ovr18:0AEC
				sel.Text = cut("* "+sel.Text, textCols)
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
						sel.Text = sel.Text[2:] // unmarked again, as in CotAB
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
		}
		if key == NoPick || count > 5 || e.game.PartyCount > 7 {
			break
		}
	}
	for i, n := range list {
		names[i] = n.Text
	}
	e.WriteCharList(names) // ovr18:01B8
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
		e.DisplayMessageWait(colYellow, msg)
	}
	return msg != ""
}

// ForgetCharacter (extra): pick a saved character and delete its files and
// its line in charlist.txt. Party members keep their copy in the party.
func (e *Engine) ForgetCharacter() {
	e.ClearCells(1, 1, winRight, winBottom)
	names := e.BuildCharList()
	list := make([]*ListNode, len(names))
	for i, n := range names {
		list[i] = &ListNode{Text: n}
	}
	index, redraw := 0, true
	var sel *ListNode
	for len(list) > 0 {
		const forget Choice = 0
		key := e.ListMenu("Forget a character: ", Bar{{"Forget", forget}, {"Exit", PickExit}}, colLightMagenta, colLightGreen, colWhite, 1, 2, 38, 22, list, false, &redraw, &index, &sel).ID
		if key == NoPick {
			break
		}
		if (key != PickEnter && key != forget) || sel == nil {
			continue
		}
		redraw = true
		if !e.YesNo(colYellow, colLightGreen, colWhite, cut("Forget "+sel.Text+" forever? ", textCols)) {
			continue
		}
		base := makeFileName(removeSubstr(sel.Text, "."))
		for _, ext := range []string{".cha", ".itm", ".spc"} {
			os.Remove(e.savePath(base + ext))
		}
		list = slices.DeleteFunc(list, func(n *ListNode) bool { return n == sel })
		sel, index = nil, min(index, max(len(list)-1, 0))
		names = names[:0]
		for _, n := range list {
			names = append(names, n.Text)
		}
		e.WriteCharList(names)
	}
	e.ClearCells(1, 1, winRight, winBottom)
}

// choosePortrait = ovr17:33E7: choose the portrait: Head (1..14) and Body
// (1..12) step through the pictures, Keep accepts. There is no way out
// without Keep (Esc just asks again).
func (e *Engine) choosePortrait() {
	c := e.selected()
	head, body := c.PortHead, c.PortBody
	for {
		e.SheetPortrait() // ovr29:04F9
		const (
			headIt Choice = iota
			bodyIt
			keep
		)
		key := e.DisplayInput("", Bar{{"Head", headIt}, {"Body", bodyIt}, {"Keep", keep}}, colBlack, colWhite, colLightGreen, false, false).ID
		{
			switch key {
			case headIt:
				if c.PortHead < 0xE {
					c.PortHead++
				} else {
					c.PortHead = 1
				}
			case bodyIt:
				if c.PortBody < 0xC {
					c.PortBody++
				} else {
					c.PortBody = 1
				}
			case keep:
				head, body = c.PortHead, c.PortBody
			}
		}
		if key == keep {
			break
		}
	}
	c.PortHead, c.PortBody = head, body
	e.ClearRow24()
}

// iconPane = ovr17:34DF (x, y) nested in the icon editor: the two poses
// of the icon in slot 0xC at icon square (x,y), on a cleared 12 cell
// panel (on EGA over the slot 0x19 background icons).
// png (extra): a dropped PNG stands in for the icon: the pane stays bare
// under its overlay (iconEditor).
func (e *Engine) iconPane(x, y int, png bool) {
	if e.Cfg.Graphics != 'E' && e.Cfg.Graphics != 'T' { // DS:4A1E == 0 (1 EGA, 2 Tandy)
		e.FillRect(x*3, y*0x18, 0xC, 0x18, 3)
	} else {
		e.FillRect(x*3, y*0x18, 0xC, 0x18, 0)
		e.drawIcon(x, y, 0, 0, 0x19)
		e.drawIcon(x+3, y, 0, 1, 0x19)
	}
	if png {
		return
	}
	e.drawIcon(x, y, 0, 0, 0xC)
	e.drawIcon(x+3, y, 0, 1, 0xC)
}

// iconCopy = ovr17:35BE (src, dst, recolour) nested in the icon editor:
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
// pixels (extra) take the same end result through amigaIconColour.
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
		am[amigaIconColour[c]] = amigaIconColour[x]
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
// default icon and each step shows another Amiga colour.
var amigaIconColour = [16]byte{0, 9, 10, 11, 6, 4, 5, 15, 12, 1, 2, 3, 7, 13, 8, 14}

// iconEditor = ovr17:382D: the combat icon editor (called from the
// character creation and the camp). States (menus DS:7BF + 0x29*state):
// 1 Parts/color-1/color-2/Size/Exit, 2 Head/Weapon, 3 the part to colour,
// 4 Size, 5 the Next/Prev/Keep/Exit stepping for the choice made before.
// The old icon is shown above the new one; Is this icon ok? ends it.
func (e *Engine) iconEditor() {
	c := e.selected()
	e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colBlack, "")
	e.combatViewLeave() // ovr32:00E3
	menu3 := slices.Clone(iconMenus[3])
	var state, parent, part int // parent and part are uninitialised until set
	var sub, key Choice
	var face bool
	old := c.Icon // extra: a dropped PNG (iconpng.go) shows over the panes, the old one above the new
	show := func() {
		var o []Overlay
		for y, im := range map[int]image.Image{2: old, 4: c.Icon} {
			for p := 0; p < 2 && im != nil; p++ {
				o = append(o, Overlay{Img: iconPose(im, p, false), At: iconBox(y, p)})
			}
		}
		e.pg.overlays = o
	}
	e.snap.iconDrop = func(im image.Image) {
		if im == nil || !c.setIconPNG(im) {
			e.say("Not a PNG, square or twice as wide as high")
			return
		}
		show()
	}
	defer func() { e.snap.iconDrop, e.pg.overlays = nil, nil }()
	for {
		e.BuildCharIcon(c, false) // ovr18:0482
		state = 1
		var colours [6]byte
		copy(colours[:], c.IconColour[:])
		order := c.Order
		c.Order = 0xC // the working copy is built into slot 0xC
		e.BuildCharIcon(c, false)
		c.Order = order
		head, weapon, size := c.IconHead, c.IconBody, c.IconSize
		e.iconCopy(order, 0xC, true)
		e.iconPane(1, 2, old != nil)
		show()
		e.DisplayString(8, 6, colWhite, "old")
		e.DisplayString(3, 10, colWhite, "ready   action")
		e.DisplayString(8, 12, colWhite, "new")
		e.DisplayString(3, 16, colWhite, "ready   action")
		for {
			e.iconPane(1, 4, c.Icon != nil)
			m := iconMenus[state]
			switch state {
			case 3:
				m = menu3
			case 4:
				m = slices.Clone(m)
				m[0] = Entry{"Large", icLarge}
				if c.IconSize == 2 {
					m[0] = Entry{"Small", icSmall} // as in the original
				}
			}
			key = e.DisplayInput("", m, colLightMagenta, colLightGreen, colWhite, false, false).ID
			{
				done := key == NoPick || key == icExit
				switch state {
				case 1:
					parent = 1
					switch key {
					case icParts:
						state = 2
					case icColour1, icColour2:
						state, face = 3, key == icColour2
						label := "Hair"
						if face {
							label = "Face"
						}
						// Delete(s, 13, 4); Insert(label, s, 13) on DS:83A.
						menu3[2].Word = label
					case icSize:
						state = 4
					}
				case 2:
					parent = 2
					if done {
						state = 1
					} else {
						sub, state = key, 5
					}
				case 3:
					parent = 3
					part = 1
					switch key {
					case icWeapon:
						part = 6
					case icBody:
						part = 1
					case icHairFace:
						part = 4
					case icShield:
						part = 5
					case icArm:
						part = 2
					case icLeg:
						part = 3
					}
					if done {
						state = 1
					} else {
						state = 5
					}
				case 4:
					switch {
					case key == icLarge:
						c.IconSize = 2
						e.BuildCharIcon(c, false)
					case key == icSmall:
						c.IconSize = 1
						e.BuildCharIcon(c, false)
					case key == icKeep:
						size, state, key = c.IconSize, 1, icStay
					case done:
						c.IconSize = size
						state, key = 1, icStay
					}
					e.BuildCharIcon(c, false)
				case 5:
					switch parent {
					case 2:
						// Step head (0..13) or weapon (0..31) pictures.
						pic, saved, top := &c.IconHead, &head, byte(0xD)
						if sub == icWeapon {
							pic, saved, top = &c.IconBody, &weapon, 0x1F
						}
						if sub == icHead || sub == icWeapon {
							switch {
							case key == icPrev:
								if *pic > 0 {
									*pic--
								} else {
									*pic = top
								}
							case key == icNext:
								if *pic < top {
									*pic++
								} else {
									*pic = 0
								}
							case key == icKeep:
								*saved, state, key = *pic, parent, icStay
							case done:
								*pic = *saved
								state, key = parent, icStay
							}
							e.BuildCharIcon(c, false)
						}
					case 3:
						lo, hi := c.IconColour[part-1]&0xF, c.IconColour[part-1]>>4
						switch {
						case key == icNext, key == icPrev:
							d := byte(1)
							if key == icPrev {
								d = 0xF // (n-1) and 15
							}
							if face {
								hi = (hi + d) & 0xF
							} else {
								lo = (lo + d) & 0xF
							}
							c.IconColour[part-1] = hi<<4 + lo
						case key == icKeep:
							copy(colours[:], c.IconColour[:])
							state, key = parent, icStay
						case done:
							copy(c.IconColour[:], colours[:])
							state, key = parent, icStay
						}
					}
				}
			}
			e.iconCopy(c.Order, 0xC, true)
			if parent == 1 && (key == NoPick || key == icExit) {
				break
			}
		}
		c.IconHead, c.IconBody, c.IconSize = head, weapon, size
		copy(c.IconColour[:], colours[:])
		e.iconCopy(c.Order, 0xC, true)
		e.iconCopy(0xC, c.Order, false)
		e.ClearRow24()
		e.freeIcon(0xC) // ovr33:014E
		if e.viewMode > 1 {
			e.LoadIconPic("ICON", 0, 0xB)
			e.LoadIconPic("ICON", 1, 0xC)
		}
		if e.YesNo(colLightMagenta, colLightGreen, colWhite, "Is this icon ok? ") {
			e.Music(3) // Amiga 0x2F33E
			break
		}
	}
	e.combatViewEnter() // ovr32:0105
}
