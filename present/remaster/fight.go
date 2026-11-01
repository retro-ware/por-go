package remaster

import (
	"fmt"
	"image"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"gpu"

	"por/engine"
	"por/pal"
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
	}
	p.drawFightRows(dst, note, b, k)
	cx := dst.Bounds().Dx() / 2
	switch {
	case p.battle.Picking && b.Turn != nil && p.pick == nil: // the bar's Cast list (View.Ask (AskSpell) is drawn by Draw)
		p.drawPickList(dst, glyph, k, cx, int(k)*16)
	case p.fightLog:
		var lines []string
		for _, m := range b.Log {
			lines = append(lines, wrap(m, 36)...)
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
	rows := append(wrap(title, 24), "")
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
	const w, h = 40 * 8, fightRows * 8
	glyph := note.Glyph
	if p.rowsImg == nil {
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
	case b.Ask != "":
		put(b.Ask, 13, 0)
	case note.Show: // a refusal (why the attack is not possible ...)
		put(note.Text, 12, 0)
	case b.Turn != nil && b.Turn.Aim && b.AimWhy != "":
		put(b.AimWhy, 12, 0)
	case b.Turn != nil && b.Turn.Aim:
		put("Aim: Tab, click=target  Esc=cancel", 10, 0)
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
	if key := fmt.Sprint(puts); key != p.rowsKey {
		p.rowsKey = key
		im := image.NewRGBA(image.Rect(0, 0, w, h))
		for i := 3; i < len(im.Pix); i += 4 {
			im.Pix[i] = 255
		}
		x = 0
		for _, t := range puts {
			drawText(im, glyph, t.s, x, 0, pal.EGA(t.c))
			x += len(t.s)*8 + 8
		}
		p.rowsImg.WritePixels(im.Pix)
	}
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate((float64(dst.Bounds().Dx())-k*w)/2, float64(p.mapBottom))
	dst.DrawImage(p.rowsImg, op)
}

// fightKey takes key k on the combat screen: the spell list's, L for the
// Log, else the 3D map's and bar's.
func (p *Presenter) fightKey(k uint16) bool {
	if p.pickKey(k) {
		return true
	}
	if k == 'l' || k == 'L' || p.fightLog && k == 27 {
		p.fightLog = !p.fightLog && k != 27
		p.logTop = math.MaxInt32 // it opens on the latest lines
		return true
	}
	if d, ok := map[uint16]int{0x4800: -1, 0x5000: 1, 0x4900: -logRows, 0x5100: logRows, 0x4700: -math.MaxInt32, 0x4F00: math.MaxInt32}[k]; ok && p.fightLog { // Up/Down, PgUp/PgDn, Home/End scroll the Log
		p.logTop = max(0, min(p.logTop, math.MaxInt32-logRows)+d) // clamped when drawn
		return true
	}
	if k == KeyPadSelect { // pad A: as Enter
		k = 13
	}
	if p.battle.Key(k) {
		return true
	}
	if k == 27 && p.view != nil { // Esc with nothing to give up: the options
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

// wrap breaks s into lines of at most n characters at spaces.
func wrap(s string, n int) (out []string) {
	l := ""
	for _, w := range strings.Fields(s) {
		if l != "" && len(l)+1+len(w) > n {
			out, l = append(out, l), ""
		}
		if l != "" {
			l += " "
		}
		l += w
	}
	return append(out, l)
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
