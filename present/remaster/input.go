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
	if fm := v.FullMap; fm != nil {
		c := fm.Cursor
		d := map[uint16]image.Point{0x4800: {0, -1}, 0x5000: {0, 1}, 0x4B00: {-1, 0}, 0x4D00: {1, 0}}[k]
		if d != (image.Point{}) {
			c := c.Add(d)
			p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActPoint, X: min(max(c.X, 0), 15), Y: min(max(c.Y, 0), 15)})
		}
		return
	}
	if n := len(v.Party); n > 0 && (choosing(v) || k == 0x4700 || k == 0x4F00) && (back || on) {
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
	if t := typing(v); t != nil { // the line being typed is edited here
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
	if askList(v) != nil && askMenu(v) != nil && !choosing(v) && p.listKey(k, v) {
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
	case menuIndex(askMenu(v), c) >= 0:
		p.send(engine.ActChoose, menuIndex(askMenu(v), c))
	case c == 'M':
		p.send(engine.ActMap, 0)
	case c == 'P':
		p.send(engine.ActOverview, 0)
	case c >= '1' && c <= '9':
		p.arrow(keypadScan[c-'1'], v)
	case c == ' ' || askMenu(v) == nil: // "press any key"
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
// page at all), a left or right click and wheel steps (up positive): the
// clicks the screens take, Esc on the right button, the highlight on the
// wheel; a click elsewhere answers "press a key".
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
		if v.Picker != nil || v.Creation != nil || v.IconEditor != nil {
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
	case askMenu(v) != nil && len(askMenu(v).Items) == 1 && typing(v) == nil: // "PRESS <ENTER> TO CONTINUE" takes no Esc: the right button continues too
		p.menu = false
		p.send(engine.ActOK, 0)
	case p.menu && v.Exploring: // the command bar's popup: it alone closes (key)
		p.menu = false
	case screenUp(v): // back one level, as Esc
		p.campBack(v)
	default:
		p.send(engine.ActClose, 0)
	}
	if r := v.Portrait; left && in && gpu.IsKeyPressed(gpu.KeyShift) && image.Pt(cx, cy).In(image.Rect(r.X1, r.Y1, r.X2+1, r.Y2+1)) { // Shift-click on the sheet's portrait: the next head
		p.send(engine.ActHead, -1)
		return
	}
	if fm := v.FullMap; fm != nil && left && in && typing(v) == nil { // the full map: a square takes the cursor, a second click names it; a legend line goes to its first square
		px, py := cx*8+4-fm.At.X, cy*8+4-fm.At.Y
		sq := image.Pt(px/fm.Cell, py/fm.Cell)
		switch i := cy - fm.LegendAt.Y; {
		case px >= 0 && py >= 0 && sq.In(image.Rect(0, 0, 16, 16)) && cx*8 < fm.At.X+16*fm.Cell && cy*8 < fm.At.Y+16*fm.Cell:
			if sq == fm.Cursor {
				p.send(engine.ActChoose, menuIndex(askMenu(v), 'L'))
			} else {
				p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActPoint, X: sq.X, Y: sq.Y})
			}
		case cx >= fm.LegendAt.X && i >= 0 && i < len(fm.Legend):
			s := fm.Legend[i].First
			p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActPoint, X: s.X, Y: s.Y})
		}
		return
	}
	switch i := rosterRow(v, cx, cy); {
	case !left || !in:
	case i >= 0 && choosing(v): // a click on a member in the roster picks it, as its portrait
		p.send(engine.ActMember, i)
	case typing(v) == nil && (askMenu(v) == nil || len(askMenu(v).Items) == 1): // no bar, or one with nothing to choose (the scripts' "PRESS <ENTER> TO CONTINUE")
		p.send(engine.ActOK, 0)
	}
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}

// askMenu is the question's prompt and options when v asks for an option,
// a line of a list or a member (nil otherwise).
func askMenu(v engine.View) *engine.Menu {
	if a := v.Ask; a != nil && (a.Kind == engine.AskMenu || a.Kind == engine.AskList || a.Kind == engine.AskMember) {
		return &a.Menu
	}
	return nil
}

// askList is the list v asks a line of (nil: none).
func askList(v engine.View) []engine.ListLine {
	if a := v.Ask; a != nil && a.Kind == engine.AskList {
		return a.List
	}
	return nil
}

// typing is the line v asks to be typed (nil: none).
func typing(v engine.View) *engine.Ask {
	if a := v.Ask; a != nil && (a.Kind == engine.AskText || a.Kind == engine.AskNumber) {
		return a
	}
	return nil
}

// choosing: v asks for a party member.
func choosing(v engine.View) bool { return v.Ask != nil && v.Ask.Kind == engine.AskMember }

// spellPick is the spells v asks to pick from (nil: none).
func spellPick(v engine.View) []engine.Spell {
	if a := v.Ask; a != nil && a.Kind == engine.AskSpell {
		return a.Spells
	}
	return nil
}
