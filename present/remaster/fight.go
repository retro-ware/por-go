package remaster

import (
	"image"
	"strconv"
	"strings"

	"gpu"

	"por/engine"
	"por/pal"
)

// The combat screen's own rows (extra): the original's two rows under the
// map (the panel and the message line) drawn from View.Battle in its font,
// colours and border; the spell list Cast opens and the battle Log.

const fightRows = 1 // cells: the status line

// drawFight draws the rows, the bar, whose turn it is, the spell list and the Log over the 3D map.
func (p *Presenter) drawFight(dst *gpu.Image, glyph func(byte) [8]byte, v engine.View) {
	b, k := v.Battle, scale(dst)
	p.mapBottom = dst.Bounds().Dy() - int(k)*8*fightRows
	if b.Turn != nil {
		for _, m := range v.Party { // whose turn: the portrait card, bottom left
			if m.Selected {
				ck := max(1, int(k)/2) // as the bar
				p.drawParty(dst, []engine.Member{m}, func(int, int) image.Rectangle {
					y := p.mapBottom - (cardH+cardGap)*ck + cardFoot*ck
					return image.Rect(cardGap*ck, y, cardGap*ck+cardW*ck, y+cardH*ck)
				})
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
	p.battle.DrawBar(dst, glyph, b, max(1, int(k)/2), p.mapBottom)
	p.drawFightRows(dst, glyph, b, k)
	cx := dst.Bounds().Dx() / 2
	switch {
	case p.battle.Picking && b.Turn != nil && p.pick == nil: // the bar's Cast list (View.SpellPick is drawn by Draw)
		p.drawPickList(dst, glyph, k, cx, int(k)*16)
	case p.fightLog:
		var lines []string
		for _, m := range b.Log {
			lines = append(lines, wrap(m, 36)...)
		}
		if len(lines) == 0 {
			lines = []string{"Nothing has happened yet."}
		}
		lines = lines[max(0, len(lines)-20):] // ponytail: the last 20 lines, no scrolling
		p.drawMenu(dst, glyph, "Battle Log", lines, make([]byte, len(lines)), -1, k, cx, int(k)*16)
		p.menuRects = p.menuRects[:0]
	}
}

// drawFightRows draws the status line along the bottom: the question
// asked, the aiming help, else the panel (the unit under the pointer, else
// the one the game names); between the player's turns the last message.
// All of them are in the Log (L).
func (p *Presenter) drawFightRows(dst *gpu.Image, glyph func(byte) [8]byte, b *engine.Battle, k float64) {
	const w, h = 40 * 8, fightRows * 8
	if p.rowsImg == nil {
		p.rowsImg = gpu.NewImage(w, h)
	}
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 3; i < len(im.Pix); i += 4 {
		im.Pix[i] = 255
	}
	rgb := pal.EGA
	x := 0
	put := func(s string, c int, y int) {
		s = s[:min(len(s), max(0, (w-x)/8))]
		drawText(im, glyph, s, x, y, rgb(c))
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
	p.rowsImg.WritePixels(im.Pix)
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
		return true
	}
	return p.battle.Key(k)
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
	return p.battle.Click(x, y)
}

func (p *Presenter) lastBattle() *engine.Battle { return p.fightView }

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
