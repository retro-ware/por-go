package remaster

import (
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
	var c []command
	if v.CanCast {
		c = append(c, command{"Cast", 'C', engine.ActCastPick})
	}
	return append(c,
		command{"Items", 'I', engine.ActInventory},
		command{"View", 'V', engine.ActMember},
		command{"Camp", 'A', engine.ActCamp}, // not C (Cast) nor E (every screen's Exit)
		command{"Sweep", 'W', engine.ActSweep},
		command{"Search", 'S', engine.ActSearch},
		command{"Map", 'M', engine.ActMap}, // the keys' own commands, for the mouse and the pad
		command{"Overview", 'O', engine.ActOverview},
		command{"Help?", '?', engine.ActHelp})
}

// runCommand sends command i of the command menu.
func (p *Presenter) runCommand(i int) {
	if i >= 0 && i < len(p.cmds) {
		p.send(p.cmds[i].act, -1)
	}
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
		p.send(act, 0)
		return true
	}
	if k >= 128 {
		return false
	}
	for _, cm := range commands(v) {
		if cm.key == c {
			p.send(cm.act, -1)
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
