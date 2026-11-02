package battle

import (
	"image"
	"image/color"
	"math"
	"strconv"

	"gpu"
)

// Floating damage numbers (Graphics, Damage Numbers): a unit whose hit
// points drop shows the loss over its head, rising and fading.

const floatLife = 1.2 // seconds a number shows

// floater is one number in the air.
type floater struct {
	at   vec // over the head when it was hit
	img  *gpu.Image
	life float64
}

// noteDamage starts a number for each unit that lost hit points since the last frame.
func (p *Presenter) noteDamage(heads []head) {
	if p.hp == nil {
		p.hp = map[int][2]int{}
	}
	for _, hd := range heads {
		u := hd.u
		was, ok := p.hp[u.ID]
		p.hp[u.ID] = [2]int{u.HP, u.MaxHP}
		if !ok || was[1] != u.MaxHP || u.HP >= was[0] || !p.DamageNumbers || p.Glyph == nil {
			continue
		}
		p.floats = append(p.floats, floater{at: hd.top, img: gpu.NewImageFromImage(textImage(p.Glyph, strconv.Itoa(was[0]-u.HP)))})
	}
}

// textImage is s in the game's font, yellow on a dark outline.
func textImage(glyph func(byte) [8]byte, s string) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, len(s)*8+2, 10))
	set := func(dx, dy int, c color.RGBA) {
		for i := 0; i < len(s); i++ {
			g := glyph(s[i])
			for y := range 8 {
				for x := range 8 {
					if g[y]&(0x80>>x) != 0 {
						im.SetRGBA(1+i*8+x+dx, 1+y+dy, c)
					}
				}
			}
		}
	}
	for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		set(d[0], d[1], color.RGBA{0, 0, 0, 255})
	}
	set(0, 0, color.RGBA{255, 255, 85, 255})
	return im
}

// drawDamage moves, fades and draws the numbers at the UI's glyph size.
func (p *Presenter) drawDamage(dst *gpu.Image, vp gpu.Mat4, dt float64) {
	r := dst.Bounds()
	k := float64(max(1, min(r.Dx()/400, r.Dy()/250)))
	live := p.floats[:0]
	for _, f := range p.floats {
		f.life += dt
		if f.life >= floatLife {
			f.img.Deallocate()
			continue
		}
		live = append(live, f)
		x, y, ok := project(dst, vp, f.at.add(vec{0, f.life * .6, 0}))
		if !ok {
			continue
		}
		a := float32(math.Min(1, 3*(1-f.life/floatLife)))
		b := f.img.Bounds()
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(math.Round(x-float64(b.Dx())*k/2), math.Round(y-float64(b.Dy())*k))
		op.ColorScale.Scale(a, a, a, a)
		dst.DrawImage(f.img, op)
	}
	p.floats = live
}
