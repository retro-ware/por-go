package remaster

import (
	"math"

	"gpu"

	"por/engine"
)

// drawAreaMap draws the overland's whole map over the window: whole times
// its pixels, at least the page's scale and as much more as the map needs
// to cover the window, centred on the party as far as the map's edges let
// it (no bars), the party's marker on its square.
func (p *Presenter) drawAreaMap(dst *gpu.Image, m *engine.AreaMap) {
	if p.areaSrc != m.Img {
		if p.areaImg != nil {
			p.areaImg.Deallocate()
		}
		p.areaSrc, p.areaImg = m.Img, gpu.NewImageFromImage(m.Img)
	}
	b, mb := dst.Bounds(), m.Img.Bounds()
	k := scale(dst)
	k = max(k, math.Ceil(float64(b.Dx())/float64(mb.Dx())), math.Ceil(float64(b.Dy())/float64(mb.Dy())))
	n := k * float64(m.Tile)
	cx, cy := (float64(m.Party.X)+0.5)*n, (float64(m.Party.Y)+0.5)*n
	x := min(max(cx-float64(b.Dx())/2, 0), k*float64(mb.Dx())-float64(b.Dx()))
	y := min(max(cy-float64(b.Dy())/2, 0), k*float64(mb.Dy())-float64(b.Dy()))
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(-x, -y)
	dst.DrawImage(p.areaImg, op)
	if m.Marker == nil {
		return
	}
	if p.areaMarkSrc != m.Marker {
		if p.areaMark != nil {
			p.areaMark.Deallocate()
		}
		p.areaMarkSrc, p.areaMark = m.Marker, gpu.NewImageFromImage(m.Marker)
	}
	mk := m.Marker.Bounds()
	op = &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(m.Party.X)*n+(n-k*float64(mk.Dx()))/2-x, float64(m.Party.Y)*n+(n-k*float64(mk.Dy()))/2-y)
	dst.DrawImage(p.areaMark, op)
}
