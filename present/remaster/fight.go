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

const fightRows = 5 // cells: a border, the panel, two message lines, a border

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
	p.battle.DrawBar(dst, glyph, b, max(1, int(k)/2), p.mapBottom)
	p.drawFightRows(dst, glyph, b, k)
	cx := dst.Bounds().Dx() / 2
	switch {
	case p.battle.Picking:
		lines := make([]string, len(b.Turn.Spells))
		for i, s := range b.Turn.Spells {
			lines[i] = s.Name
		}
		p.sel = min(max(p.sel, 0), len(lines)-1)
		p.drawMenu(dst, glyph, "Cast:", lines, make([]byte, len(lines)), p.sel, k, cx, int(k)*16)
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

// drawFightRows draws the panel (the unit under the pointer, else the one
// the game names) and the last message, or the question asked, in a box
// of the game's border along the bottom.
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
	x := 8
	put := func(s string, c int, y int) {
		s = s[:min(len(s), max(0, (w-8-x)/8))]
		drawText(im, glyph, s, x, y, rgb(c))
		x += len(s)*8 + 8
	}
	panel := b.Panel
	if b.Turn != nil && panel == 0 {
		panel = b.Turn.Unit
	}
	if mx, my, ok := p.battle.Pick(gpu.CursorPosition()); ok && my < p.mapBottom {
		for _, u := range b.Units {
			for _, s := range u.Squares {
				if s == image.Pt(mx, my) {
					panel = u.ID
				}
			}
		}
	}
	if b.Turn != nil && b.Turn.Aim && b.AimWhy != "" {
		put(b.AimWhy, 12, 8)
	} else {
		for _, u := range b.Units {
			if u.ID != panel {
				continue
			}
			name := 10
			if !u.Party {
				name = 12
			}
			put(u.Name, name, 8)
			x -= 8
			put(" HP", 15, 8)
			x -= 8
			put(" "+strconv.Itoa(u.HP), 10, 8)
			put("AC", 15, 8)
			x -= 8
			put(" "+strconv.Itoa(u.AC), 10, 8)
			if u.Status != "" {
				put(u.Status, 15, 8)
				break
			}
			put("Dmg", 15, 8)
			x -= 8
			put(" "+u.Dmg, 10, 8)
			if u.Hit > 0 {
				put("CtH", 15, 8)
				x -= 8
				put(" "+strconv.Itoa(u.Hit), 10, 8)
			}
		}
	}
	var lines []string
	switch {
	case b.Ask != "":
		lines = []string{b.Ask}
	case b.Turn != nil && b.Turn.Aim:
		lines = []string{"Aim: Tab, click=target  Esc=cancel"}
	case len(b.Log) > 0:
		lines = wrap(b.Log[len(b.Log)-1], 38)
		lines = lines[max(0, len(lines)-2):]
	}
	for i, l := range lines {
		x = 8
		c := 10
		if b.Ask != "" {
			c = 13
		}
		put(l, c, 16+i*8)
	}
	p.rowsImg.WritePixels(im.Pix)
	drawBorder(p.rowsImg, p.border.tiles, gpu.GeoM{}, 40, fightRows)
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate((float64(dst.Bounds().Dx())-k*w)/2, float64(p.mapBottom))
	dst.DrawImage(p.rowsImg, op)
}

// fightKey takes key k on the combat screen: the spell list's, L for the
// Log, else the 3D map's and bar's.
func (p *Presenter) fightKey(k uint16) bool {
	if p.battle.Picking {
		n := len(p.lastBattle().Turn.Spells)
		switch k {
		case 0x4800:
			p.sel = (p.sel + n - 1) % n
		case 0x5000:
			p.sel = (p.sel + 1) % n
		case 13:
			p.cast(p.sel)
		case 27:
			p.battle.Picking = false
		}
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
	if p.battle.Picking {
		p.battle.Picking = false
		for i, r := range p.menuRects {
			if image.Pt(x, y).In(r) {
				p.cast(i)
			}
		}
		return true
	}
	return p.battle.Click(x, y)
}

// cast casts spell i of the list.
func (p *Presenter) cast(i int) {
	p.battle.Picking = false
	if b := p.lastBattle(); b != nil && b.Turn != nil && i >= 0 && i < len(b.Turn.Spells) {
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActCast, N: b.Turn.Spells[i].ID})
	}
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
