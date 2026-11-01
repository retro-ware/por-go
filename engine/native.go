package engine

import (
	"math"
	"strconv"
)

// Native scripts (extra): stretches of the ECL scripts rewritten in Go,
// where the script's own logic cannot follow the port (one party purse in
// gold and copper). RunECL runs one instead of the commands at its label;
// it leaves eclPC at the label the script would have gone on to.

type scriptAt struct {
	area  int    // the area script
	label string // its label
}

var nativeScripts = map[scriptAt]func(*Engine){
	{0, "TavernGamble"}:  (*Engine).tavernGamble,
	{0, "TavernTale"}:    (*Engine).tavernTale,
	{0, "PhlanHistory1"}: func(e *Engine) { e.phlanHistory(0) },
	{0, "PhlanHistory2"}: func(e *Engine) { e.phlanHistory(1) },
	{0, "PhlanHistory3"}: func(e *Engine) { e.phlanHistory(2) },
	{0, "PhlanHistory4"}: func(e *Engine) { e.phlanHistory(3) },
	{0, "PhlanHistory5"}: func(e *Engine) { e.phlanHistory(4) },
	{0, "PhlanHistory6"}: func(e *Engine) { e.phlanHistory(5) },
}

func (e *Engine) nativeScript() func(*Engine) { return e.native[e.eclPC] }

// New Phlan's tavern game labels (area 0): TavernEvent is the "pickpocket"
// check and tavern tales, ResetVisits clears the visit flags, closes the
// picture and exits.
func (e *Engine) scriptLabel(l string) int { return e.script.Labels[l] }

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
		e.eclPC = e.scriptLabel("TavernEvent")
		return
	}
	e.tavernRound()
}

// tavernRound is one spin, each question asked of the player in turn
// (the asks complete on the answers; RunECL waits for the last).
func (e *Engine) tavernRound() {
	e.scriptText(true, "YOU HAVE ")
	e.scriptText(false, strconv.Itoa(int(e.purse.gold)))
	e.scriptText(false, " GP.")
	e.askContinue(func() {
		e.scriptText(true, "HOW MANY GOLD PIECES WILL YOU WAGER? (HOUSE LIMIT IS "+strconv.Itoa(tavernHouseLimit)+")")
		e.askNumber(func(n uint16) {
			wager := uint32(n)
			r := 0
			if tavernWagerOK(e.purse.gold, wager) {
				r = e.Random(10)
			}
			var text string
			e.purse.gold, text = tavernSpin(e.purse.gold, wager, r)
			e.scriptText(true, text)
			e.askContinue(func() {
				e.scriptText(true, "WILL YOU GO AGAIN?")
				e.askChoice("", "~YES ~NO", false, false, func(c int) {
					e.clearPrompt()
					if c == 0 {
						e.tavernRound()
						return
					}
					e.eclPC = e.scriptLabel("ResetVisits")
				})
			})
		})
	})
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

// tavernTale = ECL3 block 0 TavernTale: the original printed "YOU OVERHEAR
// TAVERN TALE n" for the player to look up; the port shows (and voices)
// the tale itself, so that line is gone. Then one in three a drunken brawl.
func (e *Engine) tavernTale() {
	n := e.Random(22) + 1
	e.scriptText(true, "")
	e.showJournal(journalBox, journalLines([]string{"Tavern Tale " + strconv.Itoa(n)}))
	if e.Random(3) == 1 {
		e.eclPC = e.scriptLabel("DrunkenBrawl")
		return
	}
	e.opExit()
}
