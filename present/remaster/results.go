package remaster

import (
	"image"

	"gpu"

	"por/engine"
)

// drawResults draws the after-combat page (View.Results) as a screen of
// its own: the outcome and experience, the hirelings' shares, what the
// party takes, the members harmed (name cyan, what red) and, when the party
// is destroyed, the end; its question as every page's.
func (p *Presenter) drawResults(dst *gpu.Image, v engine.View, note Note) Rect {
	r := v.Results
	title := "After Combat"
	if r.End != "" {
		title = "The END!"
	}
	p.fullPage(dst, &p.resultsPage, note, title, func(m *image.RGBA, cw, ch int) {
		w, y := cw-4, 2
		para := func(s string, c int) {
			for _, l := range engine.WrapWords(s, w) {
				if y < ch-2 {
					textAt(m, note, l, 2, y, c)
				}
				y++
			}
			y++
		}
		if r.Outcome != "" {
			para(r.Outcome, inkGreen)
			para(r.Exp+" experience points.", inkGreen)
		}
		for _, s := range r.Shares {
			para(s, inkGreen)
		}
		if r.Taken != "" {
			para(r.Taken, inkGreen)
		}
		for _, h := range r.Harmed {
			if y < ch-2 {
				textAt(m, note, h.Name[:min(len(h.Name), 15)], 2, y, inkCyan)
				textAt(m, note, h.What[:min(len(h.What), max(0, w-17))], 19, y, 12)
			}
			y++
		}
		if r.End != "" {
			y++
			para(r.End, inkGreen)
		}
	})
	p.drawPageMenu(dst, note, v)
	b := dst.Bounds()
	return Rect{0, 0, float64(b.Dx()), float64(b.Dy())}
}
