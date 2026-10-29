package remaster

import (
	"image"
	"image/color"

	"gpu"

	"por/engine"
)

// The main menu (the party menu, View.MainMenu): the title's lettering on
// black, the functions under it in a box wrapped round them, the party's
// cards where the 3D view has them.

// drawMain draws the main menu.
func (p *Presenter) drawMain(dst *gpu.Image, v engine.View) {
	dst.Fill(color.Black)
	b := dst.Bounds()
	mk := scale(dst)
	keys := make([]byte, len(v.MainMenu))
	for i, s := range v.MainMenu {
		keys[i] = s[0]
	}
	menuH := float64(len(v.MainMenu)+2) * 8 * mk
	var lh, lk int
	if v.Logo != nil {
		lb := v.Logo.Bounds()
		lk = max(1, min(b.Dx()*3/4/lb.Dx(), (b.Dy()-int(menuH))*2/3/lb.Dy()))
		lh = lb.Dy() * lk
	}
	gap := int(16 * mk)
	top := (b.Dy() - lh - gap - int(menuH)) / 2
	if v.Logo != nil {
		if p.logoSrc != v.Logo {
			if p.logo != nil {
				p.logo.Deallocate()
			}
			p.logo, p.logoSrc = gpu.NewImageFromImage(v.Logo), v.Logo
		}
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(float64(lk), float64(lk))
		op.GeoM.Translate(float64((b.Dx()-v.Logo.Bounds().Dx()*lk)/2), float64(top))
		dst.DrawImage(p.logo, op)
	}
	sel := -1
	if v.Menu != nil {
		sel = v.Menu.Sel
	}
	p.drawMenu(dst, p.glyph, "", v.MainMenu, keys, sel, mk, b.Dx()/2, top+lh+gap)
	p.drawParty(dst, v.Party, cardAt(dst))
	p.cards = p.cards[:0]
	for i := range v.Party {
		p.cards = append(p.cards, cardAt(dst)(i, len(v.Party)))
	}
}

// mainClick takes a click on the main menu: a function or a portrait.
func (p *Presenter) mainClick(x, y int) bool {
	for i, r := range p.cards {
		if image.Pt(x, y).In(r) {
			p.send(engine.ActMember, i)
			return true
		}
	}
	for i, r := range p.menuRects {
		if image.Pt(x, y).In(r) {
			p.send(engine.ActPick, i)
			return true
		}
	}
	return false
}

// mainHover highlights the function under the pointer when it moves.
func (p *Presenter) mainHover(x, y int) {
	if image.Pt(x, y) == p.pointer {
		return
	}
	p.pointer = image.Pt(x, y)
	for i, r := range p.menuRects {
		if p.pointer.In(r) {
			p.send(engine.ActSelect, i)
		}
	}
}
