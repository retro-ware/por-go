package remaster

import (
	"image"
	"path/filepath"
	"slices"
	"strings"

	"gpu"

	"blobber"

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

// keypadScan: digits a menu does not offer are the keypad's keys (1 End .. 9 PgUp).
var keypadScan = [...]uint16{0x4F00, 0x5000, 0x5100, 0x4B00, 0x4C00, 0x4D00, 0x4700, 0x4800, 0x4900}

// arrow turns a cursor key (scan code k) into its decision on v: the full
// map's cursor (ActPoint), the member pick's candidate or another member
// (Home/End: ActLead), else a pager's page (ActPage -1 or +1).
func (p *Presenter) arrow(k uint16, v engine.View) {
	back := k == 0x4800 || k == 0x4B00 || k == 0x4900 || k == 0x4700
	on := k == 0x5000 || k == 0x4D00 || k == 0x5100 || k == 0x4F00
	if c := v.MapCursor; c != nil {
		d := map[uint16]image.Point{0x4800: {0, -1}, 0x5000: {0, 1}, 0x4B00: {-1, 0}, 0x4D00: {1, 0}}[k]
		if d != (image.Point{}) {
			c := c.Add(d)
			p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActPoint, X: min(max(c.X, 0), 15), Y: min(max(c.Y, 0), 15)})
		}
		return
	}
	if n := len(v.Party); n > 0 && (v.Choosing || k == 0x4700 || k == 0x4F00) && (back || on) {
		i := slices.IndexFunc(v.Party, func(m engine.Member) bool { return m.Selected })
		switch {
		case on:
			i = (i + 1) % n // none: the first
		case i <= 0:
			i = n - 1
		default:
			i--
		}
		p.send(engine.ActLead, i)
		return
	}
	switch {
	case back:
		p.send(engine.ActPage, -1)
	case on:
		p.send(engine.ActPage, 1)
	}
}

func (p *Presenter) send(kind engine.ActKind, n int) {
	if kind == engine.ActChoose || kind == engine.ActOK { // a question answered: the next menu pops up, the same one again too
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
			p.send(engine.ActOK, 0)
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
	if v.List != nil && v.Menu != nil && !v.Choosing && p.listKey(k, v) {
		return
	}
	if screenUp(v) {
		p.campKey(k, v)
		return
	}
	if v.Exploring && p.exploreKey(k, v) {
		return
	}
	c := byte(k)
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	switch {
	case k == 13 || k == KeyPadSelect:
		p.send(engine.ActOK, 0)
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
	case k >= KeyF1 && k <= KeyF1+7<<8 && k&0xFF == 0: // F1-F8: as a click on the card (the command bar: select, the context menu), else the member's items
		if n := int(k-KeyF1) >> 8; p.active && p.command && n < len(p.cards) {
			p.selectCard(n)
		} else {
			p.send(engine.ActInventory, n)
		}
	case k == KeyF9:
		p.send(engine.ActSlots, 0)
	case k&0xFF == 0:
		p.arrow(k, v)
	case k == '?':
		p.send(engine.ActHelp, 0)
	case k >= 128 || c < ' ':
	case menuIndex(v.Menu, c) >= 0:
		p.send(engine.ActChoose, menuIndex(v.Menu, c))
	case c == 'M':
		p.send(engine.ActMap, 0)
	case c == 'P':
		p.send(engine.ActOverview, 0)
	case c >= '1' && c <= '9':
		p.arrow(keypadScan[c-'1'], v)
	case c == ' ' || v.Menu == nil: // "press any key"
		p.send(engine.ActOK, 0)
	}
}

// Drop takes a file dropped on the window: a PNG while the sheet is up
// becomes the member's portrait, on the icon editor (or dropped on the
// creation sheet's icon, cell cx, cy) the combat icon.
func (p *Presenter) Drop(path string, cx, cy int, in bool, v engine.View) {
	if !strings.EqualFold(filepath.Ext(path), ".png") {
		return
	}
	r := v.IconBox
	switch {
	case v.IconDrop || in && r != (engine.CellRect{}) && image.Pt(cx, cy).In(image.Rect(r.X1, r.Y1, r.X2+1, r.Y2+1)):
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActIcon, Text: path})
	case v.Portrait != (engine.CellRect{}):
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
			p.battle.Zoom(wheel > 0, blobber.Settings.Zoom)
			continue
		}
	}
	if pageScreen(v) && p.pageMenu { // the page screens' own pointer (screens.go): the right button is Esc
		if right {
			p.screenKey(27, v)
			right = false
		}
		if v.Picker != nil || v.Creation != nil { // the icon editor keeps its spots
			if in {
				p.screenMouse(cx, cy, left, v)
			}
			return
		}
	}
	switch { // the right button: on a card that member's inventory, else it closes the context menu, else Esc
	case !right:
	case p.cardHover > 0 && p.active && v.Exploring:
		p.ctx = 0
		p.send(engine.ActInventory, p.cardHover-1)
	case p.fight && p.pointer.In(p.fightCard): // the active member's sheet
		p.send(engine.ActMember, 0)
	case p.ctx > 0:
		p.ctx = 0
	case v.Menu != nil && len(v.Menu.Items) == 1 && v.Typing == nil: // "PRESS <ENTER> TO CONTINUE" takes no Esc: the right button continues too
		p.menu = false
		p.send(engine.ActOK, 0)
	case p.menu && v.Exploring: // the command bar's popup: it alone closes (key)
		p.menu = false
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
	if at != p.hovered && (spot >= 0 || p.hovered[0] >= 0) { // onto a spot, or off them
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActHover, N: spot, X: cx, Y: cy})
	}
	p.hovered = at
	switch {
	case !left || !in:
	case spot >= 0:
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActSpot, N: spot, X: cx, Y: cy})
	case v.Typing == nil && (v.Menu == nil || len(v.Menu.Items) == 1): // no bar, or one with nothing to choose (the scripts' "PRESS <ENTER> TO CONTINUE")
		p.send(engine.ActOK, 0)
	}
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}
