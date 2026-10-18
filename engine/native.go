package engine

import (
	"math"
	"strconv"
)

// Native scripts (extra): stretches of the ECL scripts rewritten in Go,
// where the script's own logic cannot follow the port (one party purse in
// gold and copper). RunECL runs one instead of the opcodes at its address;
// it leaves eclPC at the label the script would have gone on to.

type scriptAt struct {
	disk, area byte // ECL<disk>.DAX block area
	pc         int
}

var nativeScripts = map[scriptAt]func(*Engine){
	{3, 0, 0xA501}: (*Engine).tavernGamble,
}

func (e *Engine) nativeScript() func(*Engine) {
	return nativeScripts[scriptAt{byte(e.set), e.gs.areaKind, e.eclPC}]
}

// New Phlan's tavern game labels (ECL3 block 0).
const (
	tavernTalk  = 0xA3B1 // "pickpocket" check, tavern tales
	tavernLeave = 0xAE6D // clear the visit flags, close the picture, exit
)

// tavernHouseLimit is the original 1000 platinum in gold.
const tavernHouseLimit = 5000

// tavernGamble = ECL3 A501..A643, after "WHO WILL GAMBLE?" (opWho picks the
// gambler: the purse is shared): spin a top for
// a gold wager. 8-9 of a d10 (from 0) is green, the wager won; 3-7 blue,
// the wager back; 0-2 red, the wager lost. The script counted the selected
// member's platinum in a 16-bit word; here it is the party's gold.
func (e *Engine) tavernGamble() {
	if e.selected().Okay == 0 { // [6C00] is 1 for an okay member
		e.scriptText(true, "NO ONE IS WILLING TO GAMBLE WITH SOMEONE WHO LOOKS THAT BAD.")
		e.eclPC = tavernTalk
		return
	}
	for again := true; again; {
		e.scriptText(true, "YOU HAVE ")
		e.scriptText(false, strconv.Itoa(int(e.purse.gold)))
		e.scriptText(false, " GP.")
		e.pressReturn()
		e.scriptText(true, "HOW MANY GOLD PIECES WILL YOU WAGER? (HOUSE LIMIT IS "+strconv.Itoa(tavernHouseLimit)+")")
		wager := uint32(e.InputNumber("", 0xA))
		r := 0
		if tavernWagerOK(e.purse.gold, wager) {
			r = e.Random(10)
		}
		var text string
		e.purse.gold, text = tavernSpin(e.purse.gold, wager, r)
		e.scriptText(true, text)
		e.pressReturn()
		e.scriptText(true, "WILL YOU GO AGAIN?")
		again = e.scriptMenu("~YES ~NO") == 0
	}
	e.eclPC = tavernLeave
}

// pressReturn is the scripts' one-item "PRESS <RETURN>" menu (opHMenu).
func (e *Engine) pressReturn() {
	e.MenuInput(true, 0xF, 0xF, 0xD, "~PRESS <ENTER>/<RETURN> TO CONTINUE", "")
	e.ClearBox(0, 0x18, 0x27, 0x18)
}

// scriptMenu is opHMenu's menu: the index of the choice.
func (e *Engine) scriptMenu(menu string) int {
	r := e.MenuInput(false, 0xF, 0xA, 0xD, menu, "")
	e.ClearBox(0, 0x18, 0x27, 0x18)
	return int(byte(r))
}

func tavernWagerOK(gold, wager uint32) bool {
	return wager > 0 && wager <= gold && wager <= tavernHouseLimit
}

// tavernSpin is one spin of the top (r = Random(10)): the gold after it and
// the text.
func tavernSpin(gold, wager uint32, r int) (uint32, string) {
	switch {
	case !tavernWagerOK(gold, wager):
		return gold, "I CAN'T ACCEPT THAT WAGER."
	case r >= 8:
		return gold + min(wager, math.MaxUint32-gold), "IT'S GREEN.  YOU GET DOUBLE YOUR MONEY BACK."
	case r > 2:
		return gold, "IT'S BLUE.  YOU WIN."
	}
	return gold - wager, "RED. SORRY, YOU LOSE."
}
