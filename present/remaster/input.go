package remaster

import (
	"image"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gpu"

	"por/phlan-poc/blobber"

	"por/engine"
	"por/present/input"
)

// memberKey is the member key k picks where a member applies: F1-F8 that
// member, the pad's stick buttons the previous and next of n after cur
// (none selected: the first), and with lr Left and Right too (where the
// screen has no other use for them). ok: k is a member key.
func memberKey(k uint16, cur, n int, lr bool) (int, bool) {
	switch a := actOf(k); {
	case a >= input.Member1 && a < input.Member1+input.Members:
		i := int(a - input.Member1)
		return i, i < n
	case n == 0:
	case a == input.PrevMember || lr && a == input.Left:
		if cur <= 0 {
			return n - 1, true
		}
		return cur - 1, true
	case a == input.NextMember || lr && a == input.Right:
		return (cur + 1) % n, true
	}
	return -1, false
}

// globalKey: k works on every screen (keyAct): help, the minimap, save and
// load, the sound, the font. A screen passes it on rather than swallowing it.
func globalKey(k uint16) bool {
	switch actOf(k) {
	case input.Help, input.Minimap, input.SaveLoad, input.Sound, input.Font, input.FontPrev:
		return true
	}
	return false
}

// arrow turns a cursor key a into its decision on v: the full map's cursor
// (ActPoint), the member pick's candidate (ActLead), else a pager's page
// (ActPage -1 or +1).
func (p *Presenter) arrow(a input.Act, v engine.View) {
	back := a == input.Up || a == input.Left || a == input.PgUp || a == input.Home
	on := a == input.Down || a == input.Right || a == input.PgDn || a == input.End
	if fm := v.FullMap; fm != nil {
		c := fm.Cursor
		d := map[input.Act]image.Point{input.Up: {0, -1}, input.Down: {0, 1}, input.Left: {-1, 0}, input.Right: {1, 0}}[a]
		if d != (image.Point{}) {
			c := c.Add(d)
			p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActPoint, X: min(max(c.X, 0), 15), Y: min(max(c.Y, 0), 15)})
		}
		return
	}
	if n := len(v.Party); n > 0 && choosing(v) && (back || on) {
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
			p.typed, p.typedCur, p.typing = t.Text, len(t.Text), true
		}
		switch done, cancel, ok := editKey(k, &p.typed, &p.typedCur, t.Max, t.Kind == engine.AskNumber); {
		case done:
			p.send(engine.ActOK, 0)
			p.typing = false
		case cancel:
			p.send(engine.ActClose, 0)
			p.typing = false
		case !ok:
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
	if screenUp(v) && p.campKey(k, v) {
		return
	}
	if v.Exploring && p.exploreKey(k, v) {
		return
	}
	if n, ok := memberKey(k, selectedIndex(v), len(v.Party), false); ok { // F1-F8, the stick buttons: select the member (exploring: and open their context menu, as a card's click)
		if p.command {
			p.cardClick(n)
			return
		}
		p.send(engine.ActLead, n)
		return
	}
	c := byte(k)
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	switch a := actOf(k); {
	case a == input.Enter || a == input.PadA:
		p.send(engine.ActOK, 0)
	case a == input.Esc:
		p.send(engine.ActClose, 0)
	case a == input.Minimap:
		p.send(engine.ActMinimap, 0)
	case a == input.Overview:
		p.send(engine.ActOverview, 0)
	case a == input.Font:
		p.send(engine.ActFont, 1)
	case a == input.FontPrev:
		p.send(engine.ActFont, -1)
	case a == input.Sound:
		p.send(engine.ActSound, 0)
	case a == input.SaveLoad:
		p.send(engine.ActSlots, 0)
	case input.Cursor(a):
		p.arrow(a, v)
	case a == input.Help:
		p.send(engine.ActHelp, 0)
	case k >= 128 || c < ' ': // another key the table binds (or none): nothing here
	case menuIndex(askMenu(v), c) >= 0:
		p.send(engine.ActChoose, menuIndex(askMenu(v), c))
	case a == input.Map:
		p.send(engine.ActMap, 0)
	case a == input.Party:
		p.send(engine.ActOverview, 0)
	case keypad(k) != input.None:
		p.arrow(keypad(k), v)
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
	case v.Portrait != (engine.CellRect{}) || v.Inventory != nil && v.Inventory.Wares == nil: // the sheet or the member's inventory
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
	if p.journal.open { // its rows on the window's pixels (Hover, Click)
		p.journalMouse(p.camp.pointer, false, right, wheel)
		return
	}
	for ; wheel != 0; wheel -= unit(wheel) {
		if p.fight && p.fightLog { // wheel up: up, a line a step, as every list
			p.logTop = max(0, p.logTop-unit(wheel))
			continue
		}
		if p.fight {
			p.battle.Zoom(wheel > 0, blobber.Settings.Zoom)
			continue
		}
		if v.Pages != nil { // a pager: the wheel pages
			p.send(engine.ActPage, -unit(wheel))
			continue
		}
		if c := &p.camp; screenUp(v) && c.screen == campBestiary && c.pointer.In(c.beastPane) { // over the entry: it scrolls
			c.beastY = min(max(c.beastY-10*unit(wheel), 0), c.beastMax)
			continue
		}
		if c := &p.camp; screenUp(v) && c.screen == campBestiary { // over the list: a line at most a pause, the rest of a fling dropped
			if time.Since(c.wheeled) < 2*wheelPause {
				break
			}
			c.wheeled = time.Now()
		}
		if screenUp(v) || askList(v) != nil || p.pageMenu { // a list or menu (camp-style, the list popup, the page's: save slots, drop-downs, the sheet): the wheel moves the highlight, a line a step, up for up
			k := keyOf(input.Down)
			if wheel > 0 {
				k = keyOf(input.Up)
			}
			p.key(k, v)
		}
	}
	if pageScreen(v) && p.pageMenu { // the page screens' own pointer (screens.go): the right button is Esc
		if right {
			p.screenKey(keyOf(input.Esc), v)
			right = false
		}
		if v.Picker != nil || v.Creation != nil || v.IconEditor != nil {
			if in {
				p.screenMouse(cx, cy, left, v)
			}
			return
		}
	}
	switch { // the right button: on a card that member's sheet, else it closes the context menu, else Esc
	case !right:
	case confirm(v) != "": // a yes/no question: No
		p.confirmPick(1)
	case p.cardHover > 0 && (p.active || p.main):
		p.ctx = 0
		p.send(engine.ActMember, p.cardHover-1)
	case p.fight && p.pointer.In(p.fightCard): // the active member's sheet
		p.send(engine.ActMember, 0)
	case p.fight: // as Esc: cancel, else the options
		p.fightKey(keyOf(input.Esc))
	case p.ctx > 0:
		p.ctx = 0
	case askMenu(v) != nil && len(askMenu(v).Items) == 1 && typing(v) == nil: // "PRESS <ENTER> TO CONTINUE" takes no Esc: the right button continues too
		p.menu = false
		p.send(engine.ActOK, 0)
	case p.menu && v.Exploring: // the command bar's popup: it alone closes (key)
		p.menu = false
	case screenUp(v): // back one level, as Esc
		p.campBack(v)
	case v.Exploring && !p.menu && p.active && p.view != nil: // as Esc: the options
		p.view.OpenOptions()
	default:
		p.send(engine.ActClose, 0)
	}
	if r := v.Portrait; left && in && gpu.IsKeyPressed(gpu.KeyShift) && image.Pt(cx, cy).In(image.Rect(r.X1, r.Y1, r.X2+1, r.Y2+1)) { // Shift-click on the sheet's portrait: the next head
		p.send(engine.ActHead, -1)
		return
	}
	if pg := v.Pages; pg != nil && left && in { // a pager's ◀ ▶ marks page
		w, h := pageSize(v)
		for _, d := range []int{-1, 1} {
			if x, ok := pagerMark(pg, w/8, d); ok && cx == x && cy == h/8-2 {
				p.send(engine.ActPage, d)
				return
			}
		}
	}
	if fm := v.FullMap; fm != nil && in && !left && typing(v) == nil && !p.mapMenu && image.Pt(cx, cy) != p.mapPointer { // the pointer moved over a square: the cursor follows (the arrows keep it while it rests)
		p.mapPointer = image.Pt(cx, cy)
		px, py := cx*8+4-fm.At.X, cy*8+4-fm.At.Y
		if sq := image.Pt(px/fm.Cell, py/fm.Cell); px >= 0 && py >= 0 && sq.In(image.Rect(0, 0, 16, 16)) && sq != fm.Cursor {
			p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActPoint, X: sq.X, Y: sq.Y})
		}
	}
	if fm := v.FullMap; fm != nil && left && in && typing(v) == nil { // the full map: a square takes the cursor and opens its context menu; a legend line goes to its first square; elsewhere: back
		px, py := cx*8+4-fm.At.X, cy*8+4-fm.At.Y
		sq := image.Pt(px/fm.Cell, py/fm.Cell)
		switch i := cy - fm.LegendAt.Y; {
		case px >= 0 && py >= 0 && sq.In(image.Rect(0, 0, 16, 16)) && cx*8 < fm.At.X+16*fm.Cell && cy*8 < fm.At.Y+16*fm.Cell:
			p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActPoint, X: sq.X, Y: sq.Y})
			p.mapMenu, p.sel = true, 0
		case cx >= fm.LegendAt.X && i >= 0 && i < len(fm.Legend):
			s := fm.Legend[i].First
			p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActPoint, X: s.X, Y: s.Y})
			p.mapMenu = false
		case p.mapMenu: // off the cell's menu: it closes
			p.mapMenu = false
		default:
			p.send(engine.ActClose, 0)
		}
		return
	}
	switch {
	case !left || !in:
	case typing(v) == nil && (askMenu(v) == nil || len(askMenu(v).Items) == 1): // no bar, or one with nothing to choose (the scripts' "PRESS <ENTER> TO CONTINUE")
		p.send(engine.ActOK, 0)
	}
}

// unit is -1 for a negative n, else 1.
func unit(n int) int {
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

// editKey is the one line editor over *text with its cursor *cur: a
// printable character (a digit only, for a number) is raw input and goes
// in at the cursor, up to max; the other keys are the table's: Enter is
// done, Esc cancels, Left/Right/Home/End move the cursor, Backspace takes
// the character before it and Delete the one under it. ok: k was taken.
func editKey(k uint16, text *string, cur *int, max int, digits bool) (done, cancel, ok bool) {
	t, c := *text, min(len(*text), *cur)
	if k >= 0x20 && k < 0x7F { // a character
		if digits && (k < '0' || k > '9') || len(t) >= max {
			return false, false, false
		}
		*text, *cur = t[:c]+string(rune(k))+t[c:], c+1
		return false, false, true
	}
	switch a := actOf(k); {
	case a == input.Enter:
		return true, false, true
	case a == input.Esc:
		return false, true, true
	case a == input.Left:
		c = c - min(c, 1)
	case a == input.Right:
		c = min(len(t), c+1)
	case a == input.Home:
		c = 0
	case a == input.End:
		c = len(t)
	case a == input.Backspace && c > 0:
		t, c = t[:c-1]+t[c:], c-1
	case a == input.Delete && c < len(t):
		t = t[:c] + t[c+1:]
	default:
		return false, false, false
	}
	*text, *cur = t, c
	return false, false, true
}

// caretLine is text with its cursor at cur, in a field w+1 cells wide: a
// '_' over the character at cur while on (the blink), scrolled to keep
// the cursor in the field, padded with blanks.
func caretLine(text string, cur, w int, on bool) string {
	cur = min(cur, len(text))
	if on {
		if cur < len(text) {
			text = text[:cur] + "_" + text[cur+1:]
		} else {
			text += "_"
		}
	}
	off := max(0, cur-w)
	text = text[min(off, len(text)):min(len(text), off+w+1)]
	return text + strings.Repeat(" ", w+1-len(text))
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

// confirm is v's yes/no question (AskConfirm), "" for none.
func confirm(v engine.View) string {
	if a := v.Ask; a != nil && a.Kind == engine.AskConfirm {
		return a.Prompt
	}
	return ""
}

// yesNoKeys are every yes/no question's hotkeys.
var yesNoKeys = []byte{'Y', 'N'}

// confirmKey takes k on the engine's yes/no question as on every Yes/No:
// the arrows, Enter, Space, pad A, Y and N; Esc (pad B) is No. Only the
// global keys go on.
func (p *Presenter) confirmKey(k uint16) bool {
	switch i, back, ok := menuNav(k, &p.confirmSel, yesNoKeys); {
	case back:
		p.confirmPick(1)
	case i >= 0:
		p.confirmPick(i)
	case !ok:
		return !globalKey(k)
	}
	return true
}

// confirmPick answers the yes/no question: row 0 is Yes, anything else No.
func (p *Presenter) confirmPick(i int) {
	if i == 0 {
		p.send(engine.ActYes, 0)
		return
	}
	p.send(engine.ActNo, 0)
}
