package engine

import (
	"fmt"
	"slices"
	"strings"
)

// ECL virtual-machine support, ported from ovr08. Addresses are ECL
// addresses (ovr08:06FB): words of the three script variable records
// (vars.go), the code image (loaded at eclBase, the DS:49DE buffer) and
// special addresses above it.
const (
	areaVarBase = 0x4900 // area record word 0 (to 0x4CFF)
	gameVarBase = 0x6B00 // game record (to 0x6EFF); word 0 is the selected character's name
	auxVarBase  = 0x9700 // aux record (to 0x98FF)
	eclBase     = 0x9900 // the code image (to 0xB6FF)

	eclPicHead, eclPicBody = 0x49FD, 0x49FE // area words: writing one redraws the portrait
	eclAreaScratch         = 0x4A00         // area words 0x4A00..0x4A1F, cleared by a fresh area
	eclGameScratch         = 0x6E79         // game words 0x6E79..0x6E82, likewise
	eclSpecialLow          = 0xBF68         // write-only specials eclSpecialLow+0xE3.. (position, facing, shades)
	eclSpecialHigh         = 0xC04B         // read-only specials: x, y, facing/2, wall ahead, cell
)

// varSpace is what an ECL address refers to (addrKind).
type varSpace int

const (
	spaceArea varSpace = iota
	spaceGame
	spaceAux
	spaceCode
	spaceSpecial
)

// eclState mirrors the DS globals used by the interpreter.
type eclState struct {
	argType                            [64]byte   // DS:7005
	argLo                              [64]byte   // DS:7085
	argHi                              [64]byte   // DS:7045
	str                                [64]string // DS:6FC8 + 0x100*k (k = 1..)
	retStack                           []int      // DS:6F70 chain
	flags                              [6]bool    // DS:6F78..6F7D: eq ne lt gt le ge
	op                                 byte       // DS:6F7F
	prevOp                             byte       // DS:6F7E
	groundShade                        byte       // DS:6F68 (init 0x41)
	skyShade                           byte       // DS:6F69 (init 9)
	specialFB                          uint16     // read-only specials 0xFB/0xFC/0xB1; DS:6F6A
	specialFC                          uint16     // DS:6F6C
	specialB1                          uint16     // DS:6F6E
	cloneIcon                          byte       // CPIC id for ROLF clone (8); DS:6F83
	stop                               bool       // DS:442E
	events                             [5]int     // DS:49E3,49E5,49E7,49E9,49EB
	wallAhead                          byte       // DS:6AB0
	cell                               byte       // DS:6AB1
	encounterShown                     [2]byte    // DS:84C8
	soundVar, scriptVarB8, scriptVarB9 uint16     // DS:84CC/84CE/84D0
	memberDied, memberGone             byte       // DS:84D2/84D3 (DS:84D4 is run.duelMode)
	picShown                           byte       // DS:84E0
	picAnimating                       byte       // DS:84E1
	textInView                         byte       // DS:84E2
	spriteLoaded                       byte       // DS:84E3
	skyChanged                         byte       // DS:84E5
	posChanged                         byte       // DS:84E6
	portraitChanged                    byte       // DS:84E8
	inEncounterMenu                    byte       // DS:84E9
	shownFace                          byte       // DS:84EA
	picHead, picBody                   byte       // DS:49FD/49FE
}

// eclByte / setEclByte read and write the code image at ECL address a.
func (e *Engine) eclByte(a int) byte       { return e.ecl[uint16(a-eclBase)] }
func (e *Engine) setEclByte(a int, v byte) { e.ecl[uint16(a-eclBase)] = v }

// addrKind = ovr08:06FB.
func addrKind(a int) varSpace {
	switch {
	case a >= areaVarBase && a <= 0x4CFF:
		return spaceArea
	case a >= gameVarBase && a <= 0x6EFF:
		return spaceGame
	case a >= auxVarBase && a <= 0x98FF:
		return spaceAux
	case a >= eclBase && a <= 0xB6FF:
		return spaceCode
	}
	return spaceSpecial
}

func (e *Engine) areaVar(a int) uint16         { return *e.area.word(a - areaVarBase) }
func (e *Engine) setAreaVar(a int, v uint16)   { *e.area.word(a - areaVarBase) = v }
func (e *Engine) globalVar(a int) uint16       { return *e.game.word(a - gameVarBase) }
func (e *Engine) setGlobalVar(a int, v uint16) { *e.game.word(a - gameVarBase) = v }
func (e *Engine) blk3Var(a int) uint16         { return e.aux[a-auxVarBase] }
func (e *Engine) setBlk3Var(a int, v uint16)   { e.aux[a-auxVarBase] = v }

func (e *Engine) selected() *Character {
	if len(e.party) == 0 {
		return &Character{}
	}
	return e.party[e.partySel]
}

func (e *Engine) partyIndex(c *Character) int { // ovr08:0756
	for i, p := range e.party {
		if p == c {
			return i
		}
	}
	return len(e.party)
}

// readSpecialGlobal = ovr08:07BC.
func (e *Engine) readSpecialGlobal(a int) (uint16, bool) {
	c := e.selected()
	switch off := a - gameVarBase; off {
	case 0x18:
		return uint16(c.Stats[statCon]), true
	case 0x9B:
		return uint16(c.Saves[1]), true
	case 0xA0:
		return uint16(int16(int8(c.MaxLevel))), true
	case 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xAB, 0xAC:
		return uint16(c.Thief[off-0xA5]), true
	case 0xB8:
		return uint16(c.Control), true
	case 0xBB:
		return e.purseCoin(0), true
	case 0xBD:
		return e.purseCoin(1), true
	case 0xBF:
		return e.purseCoin(2), true
	case 0xC1:
		return e.purseCoin(3), true
	case 0xC3:
		return e.purseCoin(4), true
	case 0xD8:
		return uint16(int16(int8(c.Alignment))), true
	case 0xF7:
		return c.ExpValue, true
	case 0xF9:
		return uint16(c.ExpPerHP), true
	case 0x100:
		if c.Okay != 0 {
			return 1, true
		}
		return 0x80, true
	case 0x10C:
		switch {
		case c.Side == sideParty && c.Auto != 0:
			return 0x80, true
		case c.Side == sideMonsters:
			return 0x81, true
		}
		return 0, true
	case 0x10D:
		return 0, true // undefined in the original
	case 0x11B:
		return uint16(c.Move), true
	case 0x2B1, 0x2B4:
		return uint16(e.partyIndex(c)), true
	case 0x2CF:
		switch v := c.Stats[statCha]; {
		case v == 3:
			return 0, true
		case v == 4:
			return 5, true
		case v == 5:
			return 0xA, true
		case v == 6:
			return 0xF, true
		case v == 7:
			return 0x14, true
		case v >= 8 && v <= 0xC:
			return 0x19, true
		case v == 0xD:
			return 0x1E, true
		case v == 0xE:
			return 0x23, true
		case v == 0xF:
			return 0x28, true
		case v == 0x10:
			return 0x32, true
		case v == 0x11:
			return 0x37, true
		case v >= 0x12 && v <= 0x19:
			return 0x3C, true
		}
		return 0, true
	case 0x312:
		return uint16(e.set), true
	case 0x33E:
		return uint16(e.partyCount()), true
	}
	return 0, false
}

// writeSpecialGlobal = ovr08:0A74.
func (e *Engine) writeSpecialGlobal(a int, v uint16) {
	c := e.selected()
	off := a - gameVarBase
	switch {
	case off == 0:
		if v == 0 {
			e.es.memberGone = 1
		}
	case off >= 0x20 && off <= 0x70:
		c.Memorized[(off - 0x1F)] = byte(v)
	case off == 0xB8:
		if v > 0xB2 {
			v -= 0x32
		}
		c.Control = byte(v)
	case off == 0xBB:
		e.setPurseCoin(0, v)
	case off == 0xBD:
		e.setPurseCoin(1, v)
	case off == 0xBF:
		e.setPurseCoin(2, v)
	case off == 0xC1:
		e.setPurseCoin(3, v)
	case off == 0xC3:
		e.setPurseCoin(4, v)
	case off == 0xF7:
		c.ExpValue = v
	case off == 0xF9:
		c.ExpPerHP = byte(v)
	case off == 0x100:
		if v >= 0x80 {
			c.Okay = 0
		}
		if v == 0x87 {
			c.Status = statusStoned
		}
		if v == 0 {
			e.es.memberDied = 1
		}
	case off == 0x10C:
		switch v {
		case 0:
			c.Side, c.Auto = 0, 0
		case 0x80:
			c.Side, c.Auto = 0, 1
		case 0x81:
			c.Side, c.Auto = 1, 1
		}
	case off == 0x312:
		e.set = Set(v) // 0802:094A (it kept the old one in DS:5377, read nowhere here)
	case off == 0x322, off == 0x324, off == 0x326:
		if v > 0x80 {
			e.LoadWalldef(int(v&0x7F), (off-0x322)/2+1)
		}
	}
}

// WriteVar = ovr08:0C77.
func (e *Engine) WriteVar(a int, v uint16) {
	switch addrKind(a) {
	case spaceArea:
		e.setAreaVar(a, v)
		if a == eclPicHead || a == eclPicBody {
			e.es.portraitChanged = 1
		}
	case spaceGame:
		e.setGlobalVar(a, v)
		e.writeSpecialGlobal(a, v)
	case spaceAux:
		e.setBlk3Var(a, v)
	case spaceCode:
		e.setEclByte(a, byte(v))
	case spaceSpecial:
		if a < eclSpecialLow {
			switch a {
			case 0x3DE:
				e.es.soundVar = v
			case 0xB8:
				e.es.scriptVarB8 = v
			case 0xB9:
				e.es.scriptVarB9 = v
			}
			return
		}
		switch a - eclSpecialLow {
		case 0xE3:
			e.es.posChanged = 1
			e.posX = int(v)
		case 0xE4:
			e.posY = int(v)
			e.es.posChanged = 1
		case 0xE5:
			e.facing = [4]int{0, 2, 4, 6}[v%4]
			e.ext.back = false // the script chose the facing
			e.es.posChanged = 1
		case 0xF1:
			e.es.groundShade = byte(v)
			e.es.skyChanged = 1
		case 0xF7:
			e.es.skyShade = byte(v)
			e.es.skyChanged = 1
		}
	}
}

// ReadVar = ovr08:0E0C.
func (e *Engine) ReadVar(a int) uint16 {
	switch addrKind(a) {
	case spaceArea:
		return e.areaVar(a)
	case spaceGame:
		if r, ok := e.readSpecialGlobal(a); ok {
			return r
		}
		return e.globalVar(a)
	case spaceAux:
		return e.blk3Var(a)
	case spaceCode:
		return uint16(e.eclByte(a))
	}
	if a < eclSpecialHigh {
		switch a {
		case 0xB1:
			return e.es.specialB1
		case 0xFB:
			return e.es.specialFB
		case 0xFC:
			return e.es.specialFC
		case 0x33D:
			return uint16(e.facing)
		}
		return 0
	}
	switch a - eclSpecialHigh {
	case 0:
		return uint16(int16(int8(e.posX)))
	case 1:
		return uint16(int16(int8(e.posY)))
	case 2:
		return uint16(e.facing >> 1)
	case 3:
		return uint16(e.es.wallAhead)
	case 4:
		return uint16(e.es.cell)
	}
	return 0 // the original returns its unset result word (stack residue); emu.py pins it to 0
}

// WriteStringVar = ovr08:0F53.
func (e *Engine) WriteStringVar(a int, s string) {
	if addrKind(a) == spaceGame && a == gameVarBase {
		e.selected().SetName(s)
		return
	}
	for i := 0; i <= len(s); i++ {
		var v uint16
		if i < len(s) {
			v = uint16(s[i])
		}
		switch addrKind(a) {
		case spaceArea:
			e.setAreaVar(a+i, v)
		case spaceGame:
			e.setGlobalVar(a+i, v)
		case spaceAux:
			e.setBlk3Var(a+i, v)
		case spaceCode:
			e.setEclByte(a+i, byte(v))
		}
	}
}

// readStringVar = ovr08:12FB.
func (e *Engine) readStringVar(a int) string {
	if addrKind(a) == spaceGame && a == gameVarBase {
		return e.selected().Name()
	}
	var b []byte
	for i := 0; i < 255; i++ {
		var v uint16
		switch addrKind(a) {
		case spaceArea:
			v = e.areaVar(a + i)
		case spaceGame:
			v = e.globalVar(a + i)
		case spaceAux:
			v = e.blk3Var(a + i)
		case spaceCode:
			v = uint16(e.eclByte(a + i))
		}
		if v == 0 {
			break
		}
		b = append(b, byte(v))
	}
	return string(b)
}

func unpackChar(c byte) byte { // ovr08:06D3
	if c <= 0x1F {
		return c + 0x40
	}
	return c
}

// decodePackedString = ovr08:1153: 6-bit packed text at pc+1.., length n chars.
func (e *Engine) decodePackedString(n int) string {
	var out []byte
	count, state := 0, 1
	var prev, cur byte
	for count < n {
		if state < 4 {
			e.eclPC++
			prev, cur = cur, e.eclByte(e.eclPC)
			count++
		}
		var c byte
		switch state {
		case 1:
			c = cur >> 2
		case 2:
			c = (prev&3)<<4 | cur>>4
		case 3:
			c = (prev&0xF)<<2 | cur>>6
		case 4:
			c = cur & 0x3F
		}
		if state < 4 {
			state++
		} else {
			state = 1
		}
		if c != 0 && len(out) < 255 {
			out = append(out, unpackChar(c))
		}
	}
	if state == 4 {
		if c := cur & 0x3F; c != 0 && len(out) < 255 {
			out = append(out, unpackChar(c))
		}
	}
	return string(out)
}

// FetchArgs = ovr08:0000 (n): decode n operands following the opcode.
func (e *Engine) FetchArgs(n int) {
	k := 0
	for i := 1; i <= n; i++ {
		e.es.argType[i] = e.eclByte(e.eclPC + 1)
		e.es.argLo[i] = e.eclByte(e.eclPC + 2)
		e.eclPC += 2
		switch e.es.argType[i] {
		case 1, 2, 3:
			e.eclPC++
			e.es.argHi[i] = e.eclByte(e.eclPC)
		case 0x80:
			k++
			if l := int(e.es.argLo[i]); l > 0 {
				e.es.str[k] = e.decodePackedString(l)
			} else {
				e.es.str[k] = ""
			}
		case 0x81:
			k++
			e.eclPC++
			e.es.argHi[i] = e.eclByte(e.eclPC)
			e.es.str[k] = e.readStringVar(int(e.es.argHi[i])<<8 | int(e.es.argLo[i]))
		}
	}
	e.eclPC++
}

// ArgAddr returns hi<<8|lo of operand i (ovr08:06AF mkaddr).
func (e *Engine) ArgAddr(i int) int { return int(e.es.argHi[i])<<8 | int(e.es.argLo[i]) }

// ArgValue = ovr08:0136.
func (e *Engine) ArgValue(i int) uint16 {
	switch e.es.argType[i] {
	case 0:
		return uint16(e.es.argLo[i])
	case 1, 3, 0x80:
		return e.ReadVar(e.ArgAddr(i))
	}
	return uint16(e.ArgAddr(i))
}

// InitAreaScript = ovr08:01B6.
func (e *Engine) InitAreaScript() {
	s := &e.es
	s.picShown, s.textInView, s.memberDied, s.memberGone = 0, 0, 0, 0
	s.groundShade, s.skyShade, s.skyChanged = 0x41, 9, 1
	s.encounterShown = [2]byte{}
	s.cloneIcon = 8
	e.eclPC = eclBase
	s.retStack = nil
	s.flags = [6]bool{}
	e.game.Portrait = 0xFF
	e.game.RestEvery = 0
	e.game.RestChance = 0
	e.area.NoMagic = 0
	for i := 0; i < 5; i++ {
		e.FetchArgs(1)
		s.events[i] = e.ArgAddr(1)
	}
	e.area.In3D = 1
	if !e.loadedGame {
		for i := 1; i <= 0x20; i++ {
			e.setAreaVar(eclAreaScratch-1+i, 0)
		}
		for i := 0; i <= 9; i++ {
			e.setGlobalVar(eclGameScratch+i, 0)
		}
	} else {
		e.loadedGame = false
	}
}

// LoadArea = ovr08:0341: ECL<n>.DAX block areaId → ECL image (first word skipped).
func (e *Engine) LoadArea(areaID int) {
	for i := range e.ecl {
		e.ecl[i] = 0
	}
	e.loadingMonster = false
	e.ClearRow24()
	e.DisplayString(0, promptRow, colWhite, "Loading...Please Wait")
	b := e.L.Block(e.set.file("ECL"), areaID)
	if len(b) >= 2 {
		copy(e.ecl, b[2:])
	}
	e.ClearRow24()
}

// FreeSteps = ovr08:0482.
func (e *Engine) FreeSteps(x, y, dir int) int {
	if e.area.In3D == 0 {
		e.game.MonsterDist = 2
		return 2
	}
	n := 0
	for n < 2 {
		if e.WallAt(x, y, dir) != 0 {
			break
		}
		n++
		switch dir {
		case 0:
			y--
		case 2:
			x++
		case 4:
			y++
		case 6:
			x--
		}
	}
	return n
}

// CompareWords = ovr08:18EB; CompareStrings = ovr08:1815.
func (e *Engine) CompareWords(a, b uint16) {
	e.es.flags = [6]bool{a == b, a != b, a < b, a > b, a <= b, a >= b}
}
func (e *Engine) CompareStrings(a, b string) {
	e.es.flags = [6]bool{a == b, a != b, a < b, a > b, a <= b, a >= b}
}

// Gosub = ovr08:1951 (i).
func (e *Engine) Gosub(i int) {
	e.es.retStack = append(e.es.retStack, e.eclPC)
	e.eclPC = e.ArgAddr(i)
}

// StepForward = ovr08:19DB: one square ahead. Within a map it never wraps:
// at the edge the party stays put. A step out through an open side
// (checkEdge set LeftMap) lands on the far edge as in the original, since
// that is the entry square the next area keeps (west out of Phlan, in at
// the east edge).
func (e *Engine) StepForward() {
	x, y := e.posX+dirDX[e.facing], e.posY+dirDY[e.facing]
	if e.game.LeftMap != 0 {
		x, y = (x+16)%16, (y+16)%16
	}
	if x >= 0 && x <= 15 && y >= 0 && y <= 15 {
		e.posX, e.posY = x, y
	}
	e.es.cell = e.MapCell(e.posX, e.posY)
	e.es.wallAhead = e.WallAt(e.posX, e.posY, e.facing)
}

// ItemSave = ovr08:1DF0 (c, pct): each item is lost with chance pct.
// extra: no reductions for heavy items (items weigh nothing), and the
// party inventory takes its chance with the first member.
// ponytail: the pool goes with party[0] only, so a hazard to another member alone spares it.
func (e *Engine) ItemSave(c *Character, pct int) {
	list := c.Items
	if len(e.party) > 0 && c == e.party[0] {
		list = e.carried(c)
	}
	for _, it := range slices.Clone(list) {
		if e.Dice(1, 100) <= pct {
			e.RemoveItem(c, it) // 00BA:1509
		}
	}
}

// opArgCount = ovr08:1EA6 table.
var opArgCount = map[byte]int{
	0x01: 1, 0x02: 1, 0x0A: 1, 0x0E: 1, 0x11: 1, 0x12: 1, 0x1D: 1, 0x20: 1, 0x2D: 1, 0x32: 1, 0x34: 1, 0x36: 1, 0x38: 1, 0x39: 1, 0x3C: 1,
	0x03: 2, 0x08: 2, 0x09: 2, 0x0C: 2, 0x0F: 2, 0x10: 2, 0x1F: 2, 0x22: 2,
	0x04: 3, 0x05: 3, 0x06: 3, 0x07: 3, 0x0B: 3, 0x21: 3, 0x28: 3, 0x2A: 3, 0x2F: 3, 0x30: 3, 0x35: 3, 0x37: 3, 0x3B: 3,
	0x14: 4, 0x23: 4, 0x2E: 5, 0x1E: 6, 0x2C: 6, 0x27: 8, 0x29: 14,
}

// FetchOpcodeArgs = ovr08:1EA6.
func (e *Engine) FetchOpcodeArgs() {
	e.es.op = e.eclByte(e.eclPC)
	if n, ok := opArgCount[e.es.op]; ok {
		e.FetchArgs(n)
	} else {
		e.eclPC++
	}
}

// PartyHpRange = ovr08:1FB1 (&hi, &lo): highest and lowest +0x11C in the
// party, doubled (special 0x27) or halved (0x2A) in byte arithmetic.
func (e *Engine) PartyHpRange() (hi, lo int) {
	if len(e.party) == 0 {
		return 0, 0
	}
	h8 := e.party[0].Move
	l8 := h8
	for _, c := range e.party {
		h := c.Move
		if e.FindAffect(c, affHaste) != nil {
			h *= 2
		} else if e.FindAffect(c, affSlow) != nil {
			h /= 2
		}
		if h > h8 {
			h8 = h
		}
		if h < l8 {
			l8 = h
		}
	}
	return int(h8), int(l8)
}

// extractHotkeys = ovr08:156D: "~Yes ~No" → menu "yes no", hotkeys "YN".
func extractHotkeys(menu string) (hot, out string) {
	b := []byte(menu)
	var res []byte
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c == '~' {
			if i+1 < len(b) {
				h := upper(b[i+1])
				if len(hot) < 0x28 {
					hot += string(h)
				}
				res = append(res, h)
				i++
			}
			continue
		}
		if isHotkey(c) {
			c += 'a' - 'A' // digits too: '0'..'9' turn into 'P'..'Y', as in the original
		}
		res = append(res, c)
	}
	return hot, string(res)
}

// MenuInput = ovr08:1684 (anim, allowEnter, c3, c2, c1, menu, prompt): index
// of the chosen hotkey, 0 for Enter, 0xFF for unknown.
func (e *Engine) MenuInput(allowEnter bool, c3, c2, c1 int, menu, prompt string) int {
	hot, shown := extractHotkeys(menu)
	for {
		key, extra := e.DisplayInput(prompt, shown, c1, c2, c3, true, e.animInput) // ovr08:16DC: the anim parameter
		if extra {
			e.partyKeypad(key)
			e.DisplayPartyList()
			key = 0
		}
		if isHotkey(key) {
			if i := strings.IndexByte(hot, key); i >= 0 {
				return i
			}
			return 0xFF
		}
		if key == 13 && allowEnter {
			return 0
		}
	}
}

// HitMessage = ovr08:20CC (c, dmg).
func (e *Engine) HitMessage(c *Character, dmg int) {
	if c.Status == statusDead {
		return
	}
	var msg string
	if int(c.HP)+10 < dmg {
		msg = "  " + c.Name() + " dies. "
	} else {
		msg = "  " + c.Name() + " is hit FOR " + fmt.Sprintf("%3d", dmg) + " points of Damage."
	}
	clear := false
	if e.msgRow > 0x16 {
		e.msgRow = 0x11
		clear = true
		e.PromptRow24(colWhite, "press <enter>/<return> to continue")
	}
	e.msgCol = 0x26
	e.DisplayTextBlock(1, msgTop, winRight, winBottom, colWhite, clear, msg)
	e.ApplyDamage(c, dmg&0xFF) // 00BA:2228
	e.ClearBox(17, 1, winRight, 15)
	e.DisplayPartyList()
}
