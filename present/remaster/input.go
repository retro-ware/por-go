package remaster

import (
	"image"
	"path/filepath"
	"strings"

	"gpu"

	"por/engine"
)

// The window's keys as cmd/por sends them, BIOS style: low byte ASCII,
// high byte scan code; the presenter turns them into actions (Key).
const (
	KeyF1          = 0x3B00 // F1; F2..F8 follow at +0x100
	KeyF9          = 0x4300 // save / load
	KeyFontPrev    = 0xFD00 // Ctrl-Shift-F (Ctrl-F is ASCII 6)
	KeyPadSelect   = 0xE000 // gamepad: choose the highlighted (as Enter)
	KeyPadMinimap  = 0xE100 // as Tab
	KeyPadOverview = 0xE200 // as P
)

// scanDir is the ActMove direction of each scan code.
var scanDir = map[uint16]int{0x4800: engine.DirUp, 0x5000: engine.DirDown, 0x4B00: engine.DirLeft, 0x4D00: engine.DirRight,
	0x4700: engine.DirHome, 0x4F00: engine.DirEnd, 0x4900: engine.DirPgUp, 0x5100: engine.DirPgDn, 0x4C00: engine.DirCentre}

// keypadDir: digits a menu does not offer step as the keypad's keys.
var keypadDir = [...]int{engine.DirEnd, engine.DirDown, engine.DirPgDn, engine.DirLeft, engine.DirCentre, engine.DirRight, engine.DirHome, engine.DirUp, engine.DirPgUp}

func (p *Presenter) send(kind engine.ActKind, n int) {
	if kind == engine.ActPick || kind == engine.ActContinue { // a question answered: the next menu pops up, the same one again too
		p.asked = ""
	}
	p.inv.acts = append(p.inv.acts, engine.Action{Kind: kind, N: n})
}

// keyAct turns key k into the action it stands for on the screen v shows.
func (p *Presenter) keyAct(k uint16, v engine.View) {
	if t := v.Typing; t != nil { // the line being typed is edited here
		if !p.typing {
			p.typed, p.typing = t.Text, true
		}
		switch {
		case k == 13:
			p.send(engine.ActContinue, 0)
			p.typing = false
		case k == 27:
			p.send(engine.ActClose, 0)
			p.typing = false
		case k == 8 && len(p.typed) > 0:
			p.typed = p.typed[:len(p.typed)-1]
		case k >= 0x20 && k < 0x7F && len(p.typed) < t.Max:
			p.typed += string(rune(k))
		default:
			return
		}
		if p.typing {
			p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActText, Text: p.typed})
		}
		return
	}
	p.typing = false
	c := byte(k)
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	switch {
	case k == 13 || k == KeyPadSelect:
		p.send(engine.ActContinue, 0)
	case k == 27:
		p.send(engine.ActClose, 0)
	case k == 9 || k == KeyPadMinimap:
		p.send(engine.ActMinimap, 0)
	case k == KeyPadOverview:
		p.send(engine.ActOverview, 0)
	case k == 6:
		p.send(engine.ActFont, 1)
	case k == KeyFontPrev:
		p.send(engine.ActFont, -1)
	case k == 0x13:
		p.send(engine.ActSound, 0)
	case k >= KeyF1 && k <= KeyF1+7<<8 && k&0xFF == 0:
		p.send(engine.ActInventory, int(k-KeyF1)>>8)
	case k == KeyF9:
		p.send(engine.ActSlots, 0)
	case k&0xFF == 0:
		if d, ok := scanDir[k]; ok {
			p.send(engine.ActMove, d)
		}
	case k == '?':
		p.send(engine.ActHelp, 0)
	case k == ',':
		p.send(engine.ActMove, engine.DirBack)
	case k == '.':
		p.send(engine.ActMove, engine.DirOn)
	case k >= 128 || c < ' ':
	case menuIndex(v.Menu, c) >= 0:
		p.send(engine.ActPick, menuIndex(v.Menu, c))
	case c == 'M':
		p.send(engine.ActMap, 0)
	case c == 'P':
		p.send(engine.ActOverview, 0)
	case c >= '1' && c <= '9':
		p.send(engine.ActMove, keypadDir[c-'1'])
	case c == ' ' || v.Menu == nil: // "press any key"
		p.send(engine.ActContinue, 0)
	}
}

// Drop takes a file dropped on the window: a PNG while the sheet is up
// becomes the member's portrait.
func (p *Presenter) Drop(path string, v engine.View) {
	if v.Portrait != (engine.CellRect{}) && strings.EqualFold(filepath.Ext(path), ".png") {
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActPortrait, N: -1, Text: path})
	}
}

// menuIndex is the item of m that hotkey c picks, or -1.
func menuIndex(m *engine.Menu, c byte) int {
	if m != nil {
		for i, it := range m.Items {
			if it.Key == c {
				return i
			}
		}
	}
	return -1
}

// Mouse takes the pointer on text cell (cx, cy) of the page (in: on the
// page at all), a left or right click and wheel steps (up positive): a
// spot of the page hovered or clicked, Esc on the right button, the
// highlight on the wheel; a click elsewhere answers "press a key".
func (p *Presenter) Mouse(cx, cy int, in, left, right bool, wheel int, v engine.View) {
	for ; wheel != 0; wheel -= sign(wheel) {
		if p.fight {
			p.battle.Zoom(wheel > 0)
			continue
		}
		d := engine.DirWheelDown
		if wheel > 0 {
			d = engine.DirWheelUp
		}
		p.send(engine.ActMove, d)
	}
	switch { // the right button: a card's context menu opens (drawCardTip) or closes, else Esc
	case !right:
	case p.ctx > 0:
		p.ctx = 0
	case p.cardHover > 0 && p.active && v.Command:
		p.ctx, p.ctxSel = p.cardHover, -1
	default:
		p.send(engine.ActClose, 0)
	}
	if r := v.Portrait; left && in && gpu.IsKeyPressed(gpu.KeyShift) && image.Pt(cx, cy).In(image.Rect(r.X1, r.Y1, r.X2+1, r.Y2+1)) { // Shift-click on the sheet's portrait: the next head
		p.send(engine.ActHead, -1)
		return
	}
	spot := -1
	for i, r := range v.Spots { // the topmost: the last
		if in && image.Pt(cx, cy).In(image.Rect(r.X1, r.Y1, r.X2+1, r.Y2+1)) {
			spot = i
		}
	}
	at := [3]int{spot, cx, cy}
	if spot >= 0 && at != p.hovered {
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActHover, N: spot, X: cx, Y: cy})
	}
	p.hovered = at
	switch {
	case !left || !in:
	case spot >= 0:
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActSpot, N: spot, X: cx, Y: cy})
	case v.Menu == nil && v.Typing == nil:
		p.send(engine.ActContinue, 0)
	}
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}
