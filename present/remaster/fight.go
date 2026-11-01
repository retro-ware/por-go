package remaster

import (
	"fmt"
	"image"
	"math"
	"slices"
	"strconv"
	"time"

	"gpu"

	"por/engine"
	"por/pal"
	"por/present/input"
)

// The combat screen's own rows (extra): the original's two rows under the
// map (the panel and the message line) drawn from View.Battle in its font,
// colours and border; the spell list Cast opens and the battle Log.

const fightRows = 1 // cells: the status line

const logRows = 20 // the battle Log's lines shown at a time

// drawFight draws the rows, the bar, whose turn it is, the spell list and the Log over the 3D map.
func (p *Presenter) drawFight(dst *gpu.Image, note Note, v engine.View) {
	glyph := note.Glyph
	b, k := v.Battle, scale(dst)
	p.mapBottom = dst.Bounds().Dy() - int(k)*8*fightRows
	if b.Turn != nil {
		for _, m := range v.Party { // whose turn: the portrait card, bottom left
			if m.Selected {
				ck := max(1, int(k)/2) // as the bar
				y := p.mapBottom - (cardH+cardGap)*ck + cardFoot*ck
				p.fightCard = image.Rect(cardGap*ck, y, cardGap*ck+cardW*ck, y+cardH*ck)
				p.drawParty(dst, []engine.Member{m}, func(int, int) image.Rectangle { return p.fightCard })
			}
		}
	}
	p.battle.Icons, p.battle.IconKey = nil, ""
	if b.Turn != nil { // the Attack button shows the weapon in hand, Ranged what it shoots
		p.inv.load()
		for kind, it := range map[engine.ActKind]*engine.InvItem{engine.ActAttack: b.Turn.Weapon, engine.ActRanged: b.Turn.Missile} {
			if it == nil {
				continue
			}
			if n := p.inv.icon(it); n >= 0 {
				at := image.Pt(n%atlasCols*atlasCell, n/atlasCols*atlasCell)
				if p.battle.Icons == nil {
					p.battle.Icons = map[engine.ActKind]image.Image{}
				}
				p.battle.Icons[kind] = p.inv.atlas.(interface {
					SubImage(image.Rectangle) image.Image
				}).SubImage(image.Rectangle{at, at.Add(image.Pt(atlasCell, atlasCell))})
				p.battle.IconKey += strconv.Itoa(int(kind)) + ":" + strconv.Itoa(n) + " "
			}
		}
	}
	p.battle.Pointer = p.pointer
	p.battle.DrawBar(dst, glyph, b, max(1, int(k)/2), p.mapBottom)
	if kind, r, ok := p.battle.Hovered(); ok && b.Turn != nil { // the readied weapon over Attack and Ranged: what it does, its rules
		if it := map[engine.ActKind]*engine.InvItem{engine.ActAttack: b.Turn.Weapon, engine.ActRanged: b.Turn.Missile}[kind]; it != nil {
			p.drawTipOver(dst, it.Name, append(slices.Clone(it.Look), it.Rules...), max(1, int(k)/2), r)
		}
		if kind == engine.ActBandage {
			p.drawTipOver(dst, "Bandage", []string{
				"Binds the wounds of the first",
				"party member who is Dying.",
				"They become Unconscious and",
				"stop bleeding; the Dying die",
				"after 10 rounds unbandaged.",
				"Uses this character's turn.",
			}, max(1, int(k)/2), r)
		}
	}
	p.drawFightRows(dst, note, b, k)
	cx := dst.Bounds().Dx() / 2
	switch {
	case p.battle.Picking && b.Turn != nil && p.pick == nil: // the bar's Cast list (View.Ask (AskSpell) is drawn by Draw)
		p.drawPickList(dst, glyph, k, cx, int(k)*16)
	case p.fightLog:
		var lines []string
		for _, m := range b.Log {
			lines = append(lines, engine.WrapWords(m, 36)...)
		}
		if len(lines) == 0 {
			lines = []string{"Nothing has happened yet."}
		}
		n := len(lines)
		p.logTop = min(max(p.logTop, 0), max(0, n-logRows))
		lines = lines[p.logTop:min(n, p.logTop+logRows)]
		p.drawMenu(dst, glyph, "Battle Log", lines, make([]byte, len(lines)), nil, -1, k, cx, int(k)*16)
		p.menuBar(dst, p.menuRects, n, len(lines), &p.logTop, nil, k)
		p.menuRects = p.menuRects[:0]
	}
}

// drawTipOver draws a tooltip (title, lines) at scale k above r, kept on screen.
func (p *Presenter) drawTipOver(dst *gpu.Image, title string, lines []string, k int, r image.Rectangle) {
	rows := append(engine.WrapWords(title, 24), "")
	rows = append(rows, lines...)
	w := 0
	for _, l := range rows {
		w = max(w, len(l))
	}
	w, h := (w+4)*8*k, (len(rows)+2)*8*k
	cx := min(max(r.Min.X+r.Dx()/2, w/2), dst.Bounds().Dx()-w/2)
	p.tipMenu(dst, title, lines, make([]byte, len(lines)), -1, float64(k), cx, r.Min.Y-h-4*k)
}

// drawFightRows draws the status line along the bottom: the question
// asked, the aiming help, else the panel (the unit under the pointer, else
// the one the game names); between the player's turns the last message.
// All of them are in the Log (L).
func (p *Presenter) drawFightRows(dst *gpu.Image, note Note, b *engine.Battle, k float64) {
	w, h := max(40, int(float64(dst.Bounds().Dx())/k)/8)*8, fightRows*8 // as many cells as the window has
	glyph := note.Glyph
	if p.rowsImg == nil || p.rowsImg.Bounds().Dx() != w {
		p.rowsKey = ""
		p.rowsImg = gpu.NewImage(w, h)
	}
	var puts []struct { // the line's pieces: drawn only when they change (a texture upload a frame otherwise)
		s string
		c int
	}
	x := 0
	put := func(s string, c int, y int) {
		s = s[:min(len(s), max(0, (w-x)/8))]
		puts = append(puts, struct {
			s string
			c int
		}{s, c})
		x += len(s)*8 + 8
	}
	panel := b.Panel
	if b.Turn != nil && panel == 0 {
		panel = b.Turn.Unit
	}
	hover := false
	if mx, my, ok := p.battle.Pick(gpu.CursorPosition()); ok && my < p.mapBottom {
		for _, u := range b.Units {
			for _, s := range u.Squares {
				if s == image.Pt(mx, my) {
					panel, hover = u.ID, true
				}
			}
		}
	}
	switch {
	case note.Show: // a refusal (why the attack is not possible ...): the toast, as every view's (noteLine)
	case b.Turn != nil && b.Turn.Aim && b.AimWhy != "":
		put(b.AimWhy, 12, 0)
	case b.Turn != nil && b.Turn.Aim:
		put("Aim: W A S D, Tab; Enter, click; Esc", 10, 0)
	case b.Turn == nil && b.Panel == 0 && !hover && len(b.Log) > 0: // between the player's turns: what happens
		put(b.Log[len(b.Log)-1], 10, 0)
	default:
		for _, u := range b.Units {
			if u.ID != panel {
				continue
			}
			name := 10
			if !u.Party {
				name = 12
			}
			put(u.Name, name, 0)
			x -= 8
			put(" HP", 15, 0)
			x -= 8
			put(" "+strconv.Itoa(u.HP), 10, 0)
			put("AC", 15, 0)
			x -= 8
			put(" "+strconv.Itoa(u.AC), 10, 0)
			if u.Status != "" {
				put(u.Status, 15, 0)
				break
			}
			put("Dmg", 15, 0)
			x -= 8
			put(" "+u.Dmg, 10, 0)
			if u.Hit > 0 {
				put("CtH", 15, 0)
				x -= 8
				put(" "+strconv.Itoa(u.Hit), 10, 0)
			}
		}
	}
	if key := fmt.Sprint(note.Show, note.Text, puts); key != p.rowsKey {
		p.rowsKey = key
		im := image.NewRGBA(image.Rect(0, 0, w, h))
		for i := 3; i < len(im.Pix); i += 4 {
			im.Pix[i] = 255
		}
		x = 0
		if note.Show {
			noteLine(im, note, 0)
		}
		for _, t := range puts {
			drawText(im, glyph, t.s, x, 0, pal.EGA(t.c))
			x += len(t.s)*8 + 8
		}
		p.rowsImg.WritePixels(im.Pix)
	}
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate((float64(dst.Bounds().Dx())-k*float64(w))/2, float64(p.mapBottom))
	dst.DrawImage(p.rowsImg, op)
}

// fightKey takes key k on the combat screen: the spell list's, L for the
// Log, else the 3D map's and bar's.
func (p *Presenter) fightKey(k uint16) bool {
	if p.pickKey(k) {
		return true
	}
	a := combatAct(k)
	if a == input.Log || p.fightLog && a == input.Esc {
		p.fightLog = !p.fightLog && a != input.Esc
		p.logTop = math.MaxInt32 // it opens on the latest lines
		return true
	}
	if d, ok := map[input.Act]int{input.Up: -1, input.Down: 1, input.PgUp: -logRows, input.PgDn: logRows, input.Home: -math.MaxInt32, input.End: math.MaxInt32}[actOf(k)]; ok && p.fightLog { // Up/Down, PgUp/PgDn, Home/End scroll the Log, as every list
		p.logTop = max(0, min(p.logTop, math.MaxInt32-logRows)+d) // clamped when drawn
		return true
	}
	if a == input.Options && p.view != nil {
		p.view.OpenOptions()
		return true
	}
	if a == input.PadA { // pad A: as Enter
		a = input.Enter
	}
	if p.battle.Key(a, k) {
		return true
	}
	if a == input.Esc && p.view != nil { // Esc with nothing to give up: the options
		p.view.OpenOptions()
		return true
	}
	return false
}

// fightClick takes a click at (x, y) on the combat screen.
func (p *Presenter) fightClick(x, y int) bool {
	if p.fightLog {
		p.fightLog = false
		return true
	}
	if p.pickClick(x, y) {
		return true
	}
	if image.Pt(x, y).In(p.fightCard) && p.fightView != nil && p.fightView.Turn != nil { // the active member's inventory
		p.send(engine.ActItems, 0)
		return true
	}
	return p.battle.Click(x, y)
}

// replay is the events still to play (Play): one held for Ms (or until a
// key: Key) shows the fight or the page (Scene) it carries; the engine waits until all are played (no
// ActTick, no decisions: Playing), then the live View is drawn again.
type replay struct {
	q    []engine.CombatEvent
	from time.Time // the shown event began
	skip bool      // a key cut the shown event short
}

// Play queues the fight's events Apply handed out.
func (p *Presenter) Play(evs []engine.CombatEvent) { p.replay.q = append(p.replay.q, evs...) }

// Playing: the fight's events are still being played.
func (p *Presenter) Playing() bool { return len(p.replay.q) > 0 }

// playing is the event to show now (nil: none), the ones played dropped.
func (p *Presenter) playing() *engine.CombatEvent {
	r := &p.replay
	for len(r.q) > 0 {
		if ev := &r.q[0]; ev.Kind == engine.EvSound && p.Sound != nil && r.from.IsZero() {
			p.Sound(*ev)
		}
		if ev := &r.q[0]; (ev.Ms > 0 || ev.Key) && (ev.Battle != nil || ev.Scene != nil) && !r.skip {
			if r.from.IsZero() {
				r.from = time.Now()
			}
			if ev.Key || time.Since(r.from) < time.Duration(ev.Ms)*time.Millisecond {
				return ev
			}
		}
		r.q, r.from, r.skip = r.q[1:], time.Time{}, false
	}
	return nil
}

// CombatPad: the combat map is up with no menu over it (no question, Log
// or spell list): the gamepad takes the combat bindings (PadKeys).
func (p *Presenter) CombatPad() bool {
	return p.fight && confirm(p.shownView) == "" && !p.fightLog && p.pickList() == nil && !p.Options()
}

// PadStick hands over the pad's right stick (x right, y down, -1..1): in
// combat it pans the camera.
func (p *Presenter) PadStick(x, y float64) {
	dead := func(v float64) float64 { // a resting stick drifts a little
		if math.Abs(v) < 0.25 {
			return 0
		}
		return v
	}
	p.battle.Stick = [2]float64{dead(x), dead(y)}
}

// pickList is the spell list up for a choice: View.Ask (AskSpell) anywhere, else
// the bar's Cast in combat (Picking); nil when none.
func (p *Presenter) pickList() []engine.Spell {
	switch {
	case p.pick != nil:
		return p.pick
	case p.fight && p.battle.Picking && p.fightView != nil && p.fightView.Turn != nil:
		return p.fightView.Turn.Spells
	}
	return nil
}

// drawPickList draws the spell list (pickList) as a menu with hotkeys, its
// box centred on cx with its top at top.
func (p *Presenter) drawPickList(dst *gpu.Image, glyph func(byte) [8]byte, k float64, cx, top int) {
	lines := make([]string, len(p.pickList()))
	for i, s := range p.pickList() {
		lines[i] = s.Name
	}
	p.pickSel = min(max(p.pickSel, 0), len(lines)-1)
	p.drawMenu(dst, glyph, "Cast", lines, spellKeys(len(lines)), nil, p.pickSel, k, cx, top)
	if p.pickSel >= 0 && len(p.menuRects) > 0 {
		p.drawSpellTip(dst, p.pickList()[p.pickSel].ID, k, cx)
	}
}

// spellKeys are a spell list's hotkeys, as every spell list's (camp,
// training): the digits 1-9 (digitKeys).
func spellKeys(n int) []byte { return digitKeys(make([]byte, n), func(int) bool { return true }) }

// pickKey takes key k while a spell is picked: the arrows, Enter, a hotkey, Esc.
func (p *Presenter) pickKey(k uint16) bool {
	spells := p.pickList()
	if spells == nil || globalKey(k) {
		return false
	}
	switch i, back, _ := menuNav(k, &p.pickSel, spellKeys(len(spells))); {
	case back:
		p.cast(-1)
	case i >= 0:
		p.cast(i)
	}
	return true
}

// pickClick takes a click while a spell is picked: a line casts, elsewhere gives up.
func (p *Presenter) pickClick(x, y int) bool {
	if p.pickList() == nil {
		return false
	}
	p.cast(slices.IndexFunc(p.menuRects, func(r image.Rectangle) bool { return image.Pt(x, y).In(r) }))
	return true
}

// cast casts spell i of the list (-1: none, giving up).
func (p *Presenter) cast(i int) {
	spells, fromBar := p.pickList(), p.pick == nil
	p.battle.Picking = false
	switch {
	case i >= 0 && i < len(spells):
		p.send(engine.ActCast, spells[i].ID)
	case !fromBar: // the game waits for the pick: give it up
		p.send(engine.ActClose, 0)
	}
}
