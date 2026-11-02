package remaster

import (
	"gpu"
	"image"

	"por/engine"
	"por/present/input"
)

// The adventuring commands: the presenter's own list, each a decision
// sent for the selected member (View.Exploring).

type command struct {
	text string
	key  byte
	act  engine.ActKind
}

// commands are the decisions the command menu offers on v.
func commands(v engine.View) []command {
	return []command{
		{"Camp", 'A', engine.ActCamp}, // not C (Cast) nor E (every screen's Exit)
		{"Sweep", 'W', engine.ActSweep},
		{"Search", 'S', engine.ActSearch},
		{"Map", 'M', engine.ActMap}, // the keys' own commands, for the mouse and the pad
		{"Overview", 'O', engine.ActOverview},
		{"Journal", 'J', actJournal},
		{"Clue Book", 'B', engine.ActClueBook},
		{"Help?", '?', engine.ActHelp}}
}

// memberCommands are the selected member's keys while exploring, kept
// off the command menu (a card's context menu has them for the mouse and
// the pad).
func memberCommands(v engine.View) []command {
	c := []command{{"Items", 'I', engine.ActInventory}, {"View", 'V', engine.ActMember}}
	if v.CanCast {
		c = append(c, command{"Cast", 'C', engine.ActCastPick})
	}
	return c
}

// actJournal is the Journal command's: the presenter's own page (journal.go), no decision.
const actJournal engine.ActKind = -1

// runCommand sends command i of the command menu.
func (p *Presenter) runCommand(i int) {
	if i >= 0 && i < len(p.cmds) {
		p.runAct(p.cmds[i].act)
	}
}

// runAct sends a command's decision, or opens the journal.
func (p *Presenter) runAct(a engine.ActKind) {
	if a == actJournal {
		p.openJournal(p.shownView)
		return
	}
	p.send(a, -1)
}

// exploreKey turns key k into an adventuring decision; false when k is
// not one.
func (p *Presenter) exploreKey(k uint16, v engine.View) bool {
	c := byte(k)
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	a := actOf(k)
	if kp := keypad(k); kp != input.None { // the digits: the keypad's arrows
		a = kp
	}
	if v.Window == engine.WinAreaMap { // the area map: a step each way
		if d, ok := map[input.Act]int{input.Up: 0, input.Right: 1, input.Down: 2, input.Left: 3}[a]; ok {
			p.send(engine.ActGo, d)
			return true
		}
	} else if act, ok := map[input.Act]engine.ActKind{input.Up: engine.ActForward, input.Down: engine.ActBackward, input.Left: engine.ActTurnLeft, input.Right: engine.ActTurnRight}[a]; ok {
		if gpu.IsKeyPressed(gpu.KeyShift) && (a == input.Left || a == input.Right) { // Shift: a step aside instead of a turn
			act = map[input.Act]engine.ActKind{input.Left: engine.ActStrafeLeft, input.Right: engine.ActStrafeRight}[a]
		}
		p.send(act, 0)
		return true
	}
	if k >= 128 {
		return false
	}
	for _, cm := range append(commands(v), memberCommands(v)...) {
		if cm.key == c {
			p.runAct(cm.act)
			return true
		}
	}
	return false
}

// exploreClick takes a left click at (x, y) on the 3D view while the party
// explores (no menu up): the minimap opens the full map, anywhere else
// opens the command menu. The mouse never walks or turns.
func (p *Presenter) exploreClick(x, y int, v engine.View) {
	if v.Map != nil && p.mapAt.Min.X < p.mapAt.Max.X && image.Pt(x, y).In(p.mapAt) {
		p.send(engine.ActMap, -1)
		return
	}
	p.menu, p.sel = true, 0
}
