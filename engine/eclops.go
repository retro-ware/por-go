package engine

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ECL opcode handlers, ported from ovr04. Each is named after its opcode;
// the dispatch table (ovr04:2F2D) maps the byte at the program counter to its handler.

// eclRun holds the ovr04 globals that are not part of the ovr08 interpreter.
type eclRun struct {
	restoreSel     bool                    // DS:4430: put DS:4436 back as the selected character
	savedSel       *Character              // DS:4436 (a pointer: survives the list changing)
	scriptLoad     byte                    // DS:4431
	needFrame      byte                    // DS:4432: main screen not drawn yet
	walldefSet     byte                    // DS:4433
	geoSet         byte                    // DS:4434
	monCount       byte                    // DS:4435: monsters appended to the party list
	areaChanged    bool                    // DS:442F
	quit           bool                    // DS:49FF
	searchSave     byte                    // DS:84DD
	textInBox      byte                    // DS:84DE
	textCleared    byte                    // DS:84E4
	monstersLoaded byte                    // monsters were loaded; DS:84E7
	duelMode       byte                    // DS:84D4
	sprite         int                     // DS:6F81
	closeup        int                     // DS:6F82
	treasure       []*Item                 // DS:6810 list
	doorTry        [3]byte                 // DS:6DDE..6DE0: bash, pick, knock still allowed
	effTick        [maxCombatants + 1]byte // DS:471D: member i has timed effects to run down
	itemType       itemType                // TREASURE: last random item type
	lineHasText    byte                    // DS:84DF
}

type eclOp func(e *Engine)

var eclOps [0x3E]eclOp

func init() {
	eclOps = [0x3E]eclOp{
		0x00: (*Engine).opExit, 0x01: (*Engine).opGoto, 0x02: (*Engine).opGosub,
		0x03: (*Engine).opCompare, 0x04: (*Engine).opArith, 0x05: (*Engine).opArith,
		0x06: (*Engine).opArith, 0x07: (*Engine).opArith, 0x08: (*Engine).opRandom,
		0x09: (*Engine).opSave, 0x0A: (*Engine).opLoadCharacter, 0x0B: (*Engine).opLoadMonster,
		0x0C: (*Engine).opSetupMonster, 0x0D: (*Engine).opApproach, 0x0E: (*Engine).opPicture,
		0x0F: (*Engine).opInputNumber, 0x10: (*Engine).opInputString, 0x11: (*Engine).opPrint,
		0x12: (*Engine).opPrint, 0x13: (*Engine).opReturn, 0x14: (*Engine).opCompareAnd,
		0x15: (*Engine).opVMenu, 0x16: (*Engine).opIf, 0x17: (*Engine).opIf, 0x18: (*Engine).opIf,
		0x19: (*Engine).opIf, 0x1A: (*Engine).opIf, 0x1B: (*Engine).opIf,
		0x1C: (*Engine).opClearMonsters, 0x1D: (*Engine).opPartyStrength,
		0x1E: (*Engine).opCheckParty, 0x20: (*Engine).opNewECL, 0x21: (*Engine).opLoadFiles,
		0x22: (*Engine).opPartySurprise, 0x23: (*Engine).opSurprise, 0x24: (*Engine).opCombat,
		0x2A: (*Engine).opGetTable, 0x2B: (*Engine).opHMenu, 0x2F: (*Engine).opAndOr,
		0x30: (*Engine).opAndOr, 0x35: (*Engine).opSaveTable, 0x37: (*Engine).opLoadFiles,
	}
}

// RunECL = ovr04:3171 (addr): run the script from addr until it stops.
func (e *Engine) RunECL(addr int) {
	e.eclPC = addr
	e.es.stop = false
	for !e.es.stop && !e.run.quit {
		e.es.prevOp = e.es.op
		e.es.op = e.eclByte(e.eclPC)
		if f := e.nativeScript(); f != nil {
			f(e)
			continue
		}
		e.walkOp(e.es.op)
		if int(e.es.op) < len(eclOps) && eclOps[e.es.op] != nil {
			eclOps[e.es.op](e)
		} else {
			// Not ported yet: report it and end this script run.
			fmt.Fprintf(os.Stderr, "ecl: opcode %02X at %04X not ported\n", e.es.op, e.eclPC)
			e.FetchOpcodeArgs()
			e.es.stop = true
		}
	}
	e.es.stop = false // DS:442E is cleared on the way out
}

// opExit = ovr04:0001 (EXIT).
func (e *Engine) opExit() {
	if e.es.textInView != 0 {
		e.DrawBoxTitled(1, 1, 15, 15, 0, colWhite, "")
		if e.area.In3D != 0 {
			e.DrawBoxTitled(3, 3, 13, 13, 0, colWhite, "")
		}
	}
	if e.run.restoreSel {
		e.selectChar(e.run.savedSel)
		e.run.restoreSel = false
	}
	e.es.encounterShown = [2]byte{}
	e.es.picShown, e.es.textInView = 0, 0
	e.es.stop = true
	e.eclPC++
	e.es.retStack = nil
	e.msgRow, e.msgCol = 0x11, 1
	e.run.textInBox = 0
}

// opGoto = ovr04:00F4.
func (e *Engine) opGoto() {
	e.FetchArgs(1)
	e.eclPC = e.ArgAddr(1)
}

// opGosub = ovr04:0111.
func (e *Engine) opGosub() {
	e.FetchArgs(1)
	e.Gosub(1)
}

// opCompare = ovr04:0126: strings when either operand is one.
func (e *Engine) opCompare() {
	e.FetchArgs(2)
	if e.es.argType[1] >= 0x80 || e.es.argType[2] >= 0x80 {
		e.CompareStrings(e.es.str[1], e.es.str[2])
		return
	}
	a := e.ArgValue(1)
	b := e.ArgValue(2)
	if (e.es.argType[1] == 1 || e.es.argType[1] == 3) && e.ArgAddr(1) == 0x49C9 && e.es.argType[2] == 0 && b == 14 {
		// extra: the scripts pick the day sky by hour < 14; day is 6:00-20:00 instead
		a = 14
		if e.isDay() {
			a = 0
		}
	}
	e.CompareWords(a, b)
}

// opArith = ovr04:01A3 (ADD, SUB, DIV, MUL): result to operand 3.
func (e *Engine) opArith() {
	e.FetchArgs(3)
	a, b := e.ArgValue(1), e.ArgValue(2)
	dst := e.ArgAddr(3)
	var r uint16
	switch e.es.op {
	case 4:
		r = a + b
	case 5:
		r = b - a
	case 6:
		if b != 0 {
			r = a / b
		}
	case 7:
		r = a * b
	}
	e.WriteVar(dst, r)
}

// opRandom = ovr04:0234: Random(v+1) into operand 2 (v capped at 0xFF).
func (e *Engine) opRandom() {
	e.FetchArgs(2)
	v := byte(e.ArgValue(1))
	if v < 0xFF {
		v++
	}
	dst := e.ArgAddr(2)
	e.WriteVar(dst, uint16(byte(e.Random(int(v)))))
}

// opSave = ovr04:0292.
func (e *Engine) opSave() {
	e.FetchArgs(2)
	dst := e.ArgAddr(2)
	if e.es.argType[1] < 0x80 {
		e.WriteVar(dst, e.ArgValue(1))
	} else {
		e.WriteStringVar(dst, e.es.str[1])
	}
}

// opLoadCharacter = ovr04:02E0: select party member n (bit 7: also refresh).
func (e *Engine) opLoadCharacter() {
	e.run.restoreSel = true
	e.FetchArgs(1)
	v := byte(e.ArgValue(1))
	refresh := v&0x80 != 0
	n := int(v & 0x7F)
	if n < len(e.party) {
		e.partySel = n
	}
	if refresh && e.es.memberDied != 0 && e.es.memberGone != 0 {
		if e.run.savedSel == e.selected() {
			e.run.restoreSel = false
		}
		e.RemoveSelected(false, true) // ovr17:3291
		e.DisplayPartyList()
		e.es.memberDied, e.es.memberGone = 0, 0
	}
}

// opLoadMonster = ovr04:0450 (monster, count, icon block): append count
// copies of the monster to the party list for the coming fight.
func (e *Engine) opLoadMonster() {
	e.FetchArgs(3)
	if e.run.monCount >= 0x3F {
		return
	}
	e.addMonsters(int(e.ArgValue(1)), int(byte(e.ArgValue(2))), int(byte(e.ArgValue(3))))
}

func (e *Engine) addMonsters(idx, count, icon int) {
	sel := e.partySel
	tmpl := &Character{}
	e.LoadMonster(idx, tmpl) // ovr08:0411
	if count == 0 {
		count = 1
	}
	e.LoadIconPic("CPIC", icon, int(e.es.cloneIcon)) // ovr33:01C8
	// The template itself is the first monster (ovr04:055B); each clone is
	// a byte copy of it, so its Slots still point at the template's items
	// until ApplyItems, and its item and affect lists are copied in reverse
	// (each new node is pushed onto the head, ovr04:063E..07E7).
	tmpl.Order = e.es.cloneIcon
	e.party = append(e.party, tmpl)
	e.run.monCount++
	for i := 2; i <= count && e.run.monCount < 0x3F; i++ {
		c := &Character{charRecord: tmpl.charRecord, Slots: tmpl.Slots, Combat: tmpl.Combat}
		for k := len(tmpl.Items) - 1; k >= 0; k-- {
			it := *tmpl.Items[k]
			c.Items = append(c.Items, &it)
		}
		for k := len(tmpl.Affects) - 1; k >= 0; k-- {
			a := *tmpl.Affects[k]
			c.Affects = append(c.Affects, &a)
		}
		e.party = append(e.party, c)
		e.run.monCount++
	}
	e.es.cloneIcon++
	e.run.monstersLoaded = 1
	e.partySel = sel
}

// opSetupMonster = ovr04:03B4 (sprite, max distance, picture).
func (e *Engine) opSetupMonster() {
	e.FetchArgs(3)
	e.run.sprite = int(byte(e.ArgValue(1)))
	e.game.MaxDist = e.ArgValue(2)
	e.run.closeup = int(byte(e.ArgValue(3)))
	e.game.MonsterDist = uint16(e.FreeSteps(e.posX, e.posY, e.facing))
	if e.game.MaxDist < e.game.MonsterDist {
		e.game.MonsterDist = e.game.MaxDist
	}
	e.ShowEncounter(e.run.sprite, e.run.closeup, int(byte(e.game.MonsterDist)), &e.es.encounterShown)
}

// opApproach = ovr04:0817: the monster comes one step closer.
func (e *Engine) opApproach() {
	if d := e.game.MonsterDist; d > 0 {
		e.game.MonsterDist = d - 1
		e.ShowEncounter(e.run.sprite, e.run.closeup, int(byte(e.game.MonsterDist)), &e.es.encounterShown)
	}
	e.eclPC++
}

// opPicture = ovr04:0856 (block): show a picture, or with 0xFF put the
// view back.
func (e *Engine) opPicture() {
	e.FetchArgs(1)
	v := int(byte(e.ArgValue(1)))
	if e.area.In3D == 0 && v != 0xFF {
		e.DrawBoxTitled(3, 3, 13, 13, 0, colWhite, "")
	}
	if v != 0xFF {
		if w := e.ext.walk; w != nil {
			w.event = true // (extra, walk.go)
		}
		e.es.encounterShown[1] = 1
		e.es.picShown = 1
		if face := e.game.Portrait; face == 0xFF {
			e.LoadAnimPic("PIC", v, false)
			e.DrawAnimFirst()
		} else {
			e.ShowPortrait(int(face), v)
		}
		return
	}
	if e.prevViewMode > 1 || e.viewMode == 1 {
		if e.es.picShown != 0 || e.es.spriteLoaded != 0 {
			e.Redraw()
			e.es.picShown, e.es.spriteLoaded = 0, 0
			e.es.picAnimating = 1
		}
	}
	e.es.encounterShown = [2]byte{}
}

// opInputNumber = ovr04:0951.
func (e *Engine) opInputNumber() {
	e.FetchArgs(2)
	dst := e.ArgAddr(2)
	e.WriteVar(dst, e.InputNumber("", colLightGreen))
}

// opInputString = ovr04:0994.
func (e *Engine) opInputString() {
	e.FetchArgs(2)
	dst := e.ArgAddr(2)
	s := e.InputString("", colLightGreen, 0x28)
	if s == "" {
		s = " "
	}
	e.WriteStringVar(dst, s)
}

// platinumPrices (extra) reprices the scripts' platinum in gold: the purse
// holds gold and copper, and a script's platinum is 5 of its gold.
var platinumPrices = strings.NewReplacer(
	"1 PLATINUM PIECE", "5 GOLD PIECES", // inn, harbor master (ECL3)
	"ENOUGH PLATINUM", "ENOUGH GOLD")

// opPrint = ovr04:0A09 (PRINT 0x11, PRINTCLEAR 0x12).
func (e *Engine) opPrint() {
	e.FetchArgs(1)
	if e.es.argType[1] < 0x80 {
		e.es.str[1] = strconv.Itoa(int(e.ArgValue(1))) // 00BA:1073
	}
	e.scriptText(e.es.op == 0x12, e.es.str[1])
}

// scriptText is opPrint's text: clear starts the text window over.
func (e *Engine) scriptText(clear bool, s string) {
	e.run.textCleared = 0
	e.slowText = true
	s = platinumPrices.Replace(s)
	e.es.str[1] = s
	if w := e.ext.walk; w != nil && strings.TrimSpace(s) != "" {
		w.event = true // (extra, walk.go) blank text only clears the window
	}
	if clear {
		e.msgRow, e.msgCol = 0x11, 1
	}
	e.DisplayTextBlock(1, msgTop, winRight, winBottom, colLightGreen, clear, s)
	e.slowText = false
	e.journalPrinted(s, clear)
}

// opReturn = ovr04:0A9F.
func (e *Engine) opReturn() {
	e.eclPC++
	if n := len(e.es.retStack); n > 0 {
		e.eclPC = e.es.retStack[n-1]
		e.es.retStack = e.es.retStack[:n-1]
	}
}

// opCompareAnd = ovr04:0AED: eq when a==b and c==d, else ne.
func (e *Engine) opCompareAnd() {
	e.es.flags = [6]bool{}
	e.FetchArgs(4)
	var v [5]uint16
	for i := 1; i <= 4; i++ {
		v[i] = e.ArgValue(i)
	}
	if v[1] == v[2] && v[3] == v[4] {
		e.es.flags[0] = true
	} else {
		e.es.flags[1] = true
	}
}

// opVMenu = ovr04:0F08 (result, prompt, count, items...): a vertical list.
func (e *Engine) opVMenu() {
	e.run.textCleared = 0
	redraw := true
	index := 0
	e.FetchArgs(3)
	dst := e.ArgAddr(1)
	prompt := e.es.str[1]
	n := int(byte(e.ArgValue(3)))
	e.eclPC--
	e.FetchArgs(n)
	list := newList(n)
	e.msgCol, e.msgRow = 1, 0x11
	e.DisplayTextBlock(1, msgTop, winRight, winBottom, colLightGreen, true, prompt)
	for i, p := range list {
		s := e.es.strAt(i + 1)
		if len(s) > 0x28 {
			s = s[:0x28]
		}
		p.Text, p.Flag = s, 0
	}
	var sel *ListNode
	e.ListMenu("", "", colLightMagenta, colLightGreen, colWhite, 1, e.msgRow+1, 38, 22, list, false, &redraw, &index, &sel) // ovr08:1788
	e.WriteVar(dst, uint16(index))
	e.ClearBox(1, msgTop, winRight, winBottom)
}

// opIf = ovr04:0B52 (IF= IF<> IF< IF> IF<= IF>=): skip the next
// instruction when the flag is clear.
func (e *Engine) opIf() {
	e.eclPC++
	if !e.es.flags[e.es.op-0x16] {
		e.FetchOpcodeArgs()
	}
}

// opNewECL = ovr04:0BD1 (area): switch to another area's script.
func (e *Engine) opNewECL() {
	e.FetchArgs(1)
	e.area.AreaKind = uint16(e.gs.areaKind)
	v := byte(e.ArgValue(1))
	e.gs.areaKind = v
	e.eclFile = e.set.file("ECL") + ".dax" // DS:547C
	// ovr18:22D5 asked for another game disk when the file was missing; all
	// files are in one folder here.
	e.LoadArea(int(v))
	e.InitAreaScript()
	e.es.stop = true
	e.run.areaChanged = true
	e.run.scriptLoad = 0
}

// opLoadFiles = ovr04:0C74 (0x21 LOAD FILES, 0x37 LOAD PIECES).
func (e *Engine) opLoadFiles() {
	e.FetchArgs(3)
	e.run.scriptLoad = 1
	var a [4]int
	for i := 1; i <= 3; i++ {
		a[i] = int(byte(e.ArgValue(i)))
	}
	if e.es.op == 0x21 {
		e.run.geoSet = 1
		if a[1] != 0xFF && a[1] != 0x7F && e.area.In3D != 0 {
			e.area.Geo = uint16(a[1])
			e.LoadGeo(a[1])
			e.game.HoldMove = 0
		}
		if a[3] != 0xFF && e.area.In3D == 0 {
			e.LoadBackpacIcons("BACPAC", 0, 0x28) // ovr33:0022
		}
	} else {
		e.run.walldefSet = 1
		switch {
		case a[1] == 0x7F:
			e.LoadWalldef(0, 1)
		case e.area.WallSplitA != 0 && e.area.WallSplitB != 0:
			e.LoadWalldef(a[1], 1)
			e.LoadWalldef(a[3], 3)
		default:
			for i := 1; i <= 3; i++ {
				if a[i] != 0xFF {
					e.LoadWalldef(a[i], i)
				} else {
					e.mapPairA[i-1], e.mapPairB[i-1] = -1, -1
				}
			}
		}
	}
	if e.run.needFrame != 0 && e.run.walldefSet != 0 && e.run.geoSet != 0 {
		e.DrawMainScreen()
		e.DisplayPartyList()
		e.run.needFrame = 0
	}
}

// opAndOr = ovr04:0DEF: bytewise AND / OR into operand 3, flags set
// against 0.
func (e *Engine) opAndOr() {
	e.FetchArgs(3)
	a, b := e.ArgValue(1), e.ArgValue(2)
	dst := e.ArgAddr(3)
	var r byte
	if e.es.op == 0x2F {
		r = byte(a & b)
	} else {
		r = byte(a | b)
	}
	e.CompareWords(0, uint16(r))
	e.WriteVar(dst, uint16(r))
}

// opGetTable = ovr04:0E5E (base, index, dst).
func (e *Engine) opGetTable() {
	e.FetchArgs(3)
	base := e.ArgAddr(1)
	i := byte(e.ArgValue(2))
	dst := e.ArgAddr(3)
	e.WriteVar(dst, e.ReadVar((base+int(i))&0xFFFF))
}

// opSaveTable = ovr04:0EBC (value, base, index).
func (e *Engine) opSaveTable() {
	e.FetchArgs(3)
	v := e.ArgValue(1)
	base := e.ArgAddr(2)
	i := e.ArgValue(3)
	e.WriteVar((base+int(i))&0xFFFF, v)
}

// opHMenu = ovr04:10D1 (result, count, items...): a one-line hotkey menu.
func (e *Engine) opHMenu() {
	e.FetchArgs(2)
	dst := e.ArgAddr(1)
	n := int(byte(e.ArgValue(2)))
	e.eclPC--
	e.FetchArgs(n)
	allowEnter := false
	hi, normal := colWhite, colLightGreen
	if n == 1 {
		allowEnter, normal = true, 0xF
		if e.es.str[1] == "PRESS <RETURN> OR BUTTON TO CONTINUE" {
			e.es.str[1] = "PRESS <ENTER>/<RETURN> TO CONTINUE"
		}
	}
	anim := e.es.picShown != 0 && e.es.picAnimating != 0
	menu := ""
	for i := 1; i <= n-1; i++ {
		menu = cut(menu+"~"+e.es.strAt(i)+" ", 0x32)
	}
	menu = cut(menu+"~"+e.es.strAt(n), 0x32)
	e.animInput = anim
	r := e.MenuInput(allowEnter, hi, normal, colLightMagenta, menu, "")
	e.animInput = false
	e.WriteVar(dst, uint16(byte(r)))
	e.ClearBox(0, promptRow, lastCol, promptRow)
}

func cut(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// opClearMonsters = ovr04:125D.
func (e *Engine) opClearMonsters() {
	e.eclPC++
	e.run.monCount = 0
	e.run.monstersLoaded = 0
	e.es.cloneIcon = 8
	e.clearLoot()
	e.run.treasure = nil
}

// opPartyStrength = ovr04:12C6.
func (e *Engine) opPartyStrength() {
	e.FetchArgs(1)
	var total byte
	for _, c := range e.party {
		ac, th := int(c.AC[0]), int(c.THAC0)
		if ac > 0x3C {
			ac -= 0x3C
		} else {
			ac = 0
		}
		if th > 0x27 {
			th -= 0x27
		} else {
			th = 0
		}
		v := int(c.HP) + 5*ac + 5*th + 8*int(c.Levels[clsMage]) + 4*int(c.Levels[clsCleric])
		total += byte(v / 10)
	}
	e.WriteVar(e.ArgAddr(1), uint16(total))
}

// opCheckParty = ovr04:1435 (what, special, min, max, avg, found): a party
// census. what 0 looks for a special ability, 0x6BA7 averages +0x79 and
// 0x6C1B averages +0x11C.
func (e *Engine) opCheckParty() {
	e.FetchArgs(6)
	var what uint16
	if e.es.argType[1] == 1 {
		what = uint16(e.ArgAddr(1))
	} else {
		what = e.ArgValue(1)
	}
	special := affectType(e.ArgValue(2))
	var dst [7]int
	for i := 3; i <= 6; i++ {
		dst[i] = e.ArgAddr(i)
	}
	lo, hi, avg, found := byte(0xFF), byte(0), byte(0), false
	store := func() { // ovr04:13D7
		e.WriteVar(dst[3], uint16(lo))
		e.WriteVar(dst[4], uint16(hi))
		e.WriteVar(dst[5], uint16(avg))
		f := uint16(0)
		if found {
			f = 1
		}
		e.WriteVar(dst[6], f)
	}
	census := func(field func(*Character) byte) {
		sum, n := 0, 0
		for _, c := range e.party {
			n++
			v := field(c)
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
			sum += int(v)
		}
		if n > 0 {
			avg = byte(sum / n)
		}
		store()
	}
	switch what {
	case 0:
		for _, c := range e.party {
			if found {
				break
			}
			found = e.FindAffect(c, special) != nil // 00BA:2199
		}
		lo, hi, avg = 0, 0, 0
		store()
	case 0x6BA7:
		census(func(c *Character) byte { return c.Thief[2] })
	case 0x6C1B:
		census(func(c *Character) byte { return c.Move })
	}
}

// opPartySurprise = ovr04:1629 (a, b): a = someone has +0x2F of 4 or 10;
// b is always 0.
func (e *Engine) opPartySurprise() {
	e.FetchArgs(2)
	var a byte
	for _, c := range e.party {
		if c.Class == clsRanger || c.Class == clsClericRanger {
			a = 1
		}
	}
	e.WriteVar(e.ArgAddr(1), uint16(a))
	e.WriteVar(e.ArgAddr(2), 0)
}

// opSurprise = ovr04:16C9 (a, b, c, d): roll who is surprised into
// DS:6DCB (0 none, 1 party, 2 monsters; the 3 "both" result is always
// overwritten by 2 in the original).
func (e *Engine) opSurprise() {
	e.FetchArgs(4)
	var v [5]int
	for i := 1; i <= 4; i++ {
		v[i] = int(byte(e.ArgValue(i)))
	}
	s1 := int(int8(byte(v[4] + 2 - v[1])))
	s2 := int(int8(byte(v[2] + 2 - v[3])))
	d1, d2 := e.Dice(1, 6), e.Dice(1, 6)
	r := 0
	if d1 <= s1 {
		if d2 <= s2 {
			r = 3
		} else {
			r = 1
		}
	}
	if d2 <= s2 {
		r = 2
	}
	e.WriteVar(0x6DCB, uint16(r))
}

// opCombat = ovr04:179B: run the fight (or the shop/temple/after-combat
// screen when no fight is pending).
func (e *Engine) opCombat() {
	e.eclPC++
	if e.run.monstersLoaded == 0 && e.run.duelMode == 0 {
		switch {
		case e.game.Shop == 1:
			e.game.Shop = 0
			e.Shop() // ovr07:053E
		case e.game.Temple == 1:
			e.game.Temple = 0
			e.Temple() // ovr05:0D09
		default:
			e.AfterCombat() // ovr06:1527
		}
	} else {
		d := uint16(e.FreeSteps(e.posX, e.posY, e.facing))
		if d < e.game.MonsterDist {
			e.game.MonsterDist = d
		}
		e.Combat()      // ovr09:007A
		e.AfterCombat() // ovr06:1527
		if e.area.In3D == 0 {
			e.LoadBackpacIcons("BACPAC", 0, 0x28)
			e.LoadIconPic("ICON", 0, 9)
			e.LoadIconPic("ICON", 1, 0xA)
			e.LoadIconPic("ICON", 2, 0xB)
		}
		e.run.duelMode = 0
	}
	if e.viewMode == 1 {
		e.beginFlag = 4
	} else {
		e.beginFlag = 3
	}
	e.game.Search = e.game.Search & 1
	e.es.encounterShown = [2]byte{}
	e.es.picShown = 0
	e.DrawMainScreen()
}

// strAt returns string operand k (DS:6FC8 + 0x100*k).
func (s *eclState) strAt(k int) string {
	if k >= 0 && k < len(s.str) {
		return s.str[k]
	}
	return ""
}
