package remaster

import "por/engine"

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
		command{"Search", 'S', engine.ActSearch})
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
	if v.Window == engine.WinAreaMap { // the area map: a step each way
		if d, ok := map[uint16]int{0x4800: 0, 0x4D00: 1, 0x5000: 2, 0x4B00: 3, '8': 0, '6': 1, '2': 2, '4': 3}[k]; ok {
			p.send(engine.ActGo, d)
			return true
		}
	} else if a, ok := map[uint16]engine.ActKind{0x4800: engine.ActForward, 0x5000: engine.ActBackward, 0x4B00: engine.ActTurnLeft, 0x4D00: engine.ActTurnRight,
		'8': engine.ActForward, '2': engine.ActBackward, '4': engine.ActTurnLeft, '6': engine.ActTurnRight}[k]; ok {
		p.send(a, 0)
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
