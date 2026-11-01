package remaster

import (
	"image"
	"image/color"
	"math"

	"gpu"

	"por/engine"
)

// drawGeneric draws a page that has no screen of its own (a script's
// picture and text, a story page) over the whole window from its data:
// its pictures (the art, never the engine's frames or fills) as large as
// whole times their pixels fit over the text, centred; the text pane and
// the text over the view window in the remaster's boxes; its question as
// every page's. It returns the window.
func (p *Presenter) drawGeneric(dst *gpu.Image, v engine.View, note Note) Rect {
	dst.Fill(color.Black)
	b := dst.Bounds()
	cx := b.Dx() / 2
	k := scale(dst)
	var art []engine.Picture
	var box image.Rectangle
	for _, pc := range v.Pictures {
		if pc.Img != nil && pc.File != "" {
			art, box = append(art, pc), box.Union(pc.At)
		}
	}
	if v.Full && len(art) > 0 { // a whole-screen picture: fitted, its aspect kept (bars only on a screen of another shape)
		s := math.Min(float64(b.Dx())/float64(box.Dx()), float64(b.Dy())/float64(box.Dy()))
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(s, s)
		op.GeoM.Translate((float64(b.Dx())-s*float64(box.Dx()))/2, (float64(b.Dy())-s*float64(box.Dy()))/2)
		dst.DrawImage(pictureBox(&p.pg.pic, art, box), op)
		p.drawPageMenu(dst, note, v)
		return Rect{0, 0, float64(b.Dx()), float64(b.Dy())}
	}
	txt := engine.TextPane.Framed()
	th := 0.0
	if v.Pane != nil && v.Pane.Shown() {
		th = k * float64(txt.Pixels().Dy())
	}
	gap := int(k * 8)
	if len(v.ViewText) > 0 { // text over the view window (WinText): in its box, in the art's place
		vw := engine.ViewWindow.Framed()
		r := vw.Pixels()
		vk := math.Max(1, math.Min(k, math.Floor((float64(b.Dy())-th-3*float64(gap))/float64(r.Dy()))))
		drawBox(dst, viewTextImage(&p.pg.pic, v.ViewText, r.Inset(8), note), p.border.tiles, vw, vk, cx, gap+int(vk*float64(r.Dy())/2))
	} else if len(art) > 0 {
		s := math.Max(1, math.Floor(math.Min(float64(b.Dx()-2*gap)/float64(box.Dx()), (float64(b.Dy())-th-3*float64(gap))/float64(box.Dy()))))
		img := pictureBox(&p.pg.pic, art, box)
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(float64(cx)-s*float64(box.Dx())/2, float64(gap))
		dst.DrawImage(img, op)
	}
	if th > 0 { // the text along the bottom
		drawBox(dst, paneImage(&p.pg.pane, v.Pane, note), p.border.tiles, txt, k, cx, b.Dy()-gap-int(th/2))
	}
	p.drawPageMenu(dst, note, v)
	return Rect{0, 0, float64(b.Dx()), float64(b.Dy())}
}
