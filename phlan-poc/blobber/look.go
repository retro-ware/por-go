package blobber

// Deepnight's "Labyrinth" look on the 3D quads: everything darkens in hard, dithered bands toward a
// purple fog instead of fading smoothly to black. Light maps (NAME_light.png) keep parts of a wall
// lit and put a glow in front of it; moving lights draw as additive glows with light circles on the
// floor and ceiling.

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

var fogCol = color.RGBA{0x16, 0x10, 0x23, 255}

// slopes holds each wall, floor and ceiling texture's slope map for the normal mapping: red and
// green are the height's slope along u and v, 0.5 flat.
var slopes = map[*ebiten.Image]*ebiten.Image{}

// addSlopes makes img's slope map from its w×h height field (wrapping, like the texture).
func addSlopes(img *ebiten.Image, w, h int, height func(x, y int) float64) {
	at := func(x, y int) float64 { return height((x+w)%w, (y+h)%h) }
	pix := make([]byte, 4*w*h)
	for y := range h {
		for x := range w {
			i := 4 * (y*w + x)
			pix[i] = u8(128 + 127*(at(x+1, y)-at(x-1, y)))
			pix[i+1] = u8(128 + 127*(at(x, y+1)-at(x, y-1)))
			pix[i+3] = 255
		}
	}
	s := ebiten.NewImage(w, h)
	s.WritePixels(pix)
	slopes[img] = s
}

// normalSlopes makes img's slope map from normal map nm: a normal (x, y up, z) leans the height by
// -x/z along u and y/z along v (v runs down), halved to the scale addSlopes' height differences have.
func normalSlopes(img *ebiten.Image, nm image.Image) {
	b := nm.Bounds()
	w, h := b.Dx(), b.Dy()
	pix := make([]byte, 4*w*h)
	for y := range h {
		for x := range w {
			r, g, bl, _ := nm.At(b.Min.X+x, b.Min.Y+y).RGBA()
			nx, ny, nz := float64(r)/32767.5-1, float64(g)/32767.5-1, math.Max(float64(bl)/32767.5-1, 0.2)
			i := 4 * (y*w + x)
			pix[i] = u8(128 + 127*math.Max(-1, math.Min(1, -nx/nz*0.5)))
			pix[i+1] = u8(128 + 127*math.Max(-1, math.Min(1, ny/nz*0.5)))
			pix[i+3] = 255
		}
	}
	s := ebiten.NewImage(w, h)
	s.WritePixels(pix)
	slopes[img] = s
}

// mat is a wall material: its front texture, the generated ones' left and right lit variants
// (nil for painted textures, which use img on every face), and an optional light map.
type mat struct {
	img, right, left, lit *ebiten.Image
	loaded                bool
	// light map summary (TextureAtlas): average colour weighted by brightness, the glow's height
	// (1 top) and strength (1 when 3% of the texture is lit)
	lightCol           [3]float64
	lightHei, lightPow float64
}

// face picks the texture for a face whose normal's lateral component in camera space is nx.
func (m *mat) face(nx float64) *ebiten.Image {
	if nx > 0.5 && m.right != nil {
		return m.right
	} else if nx < -0.5 && m.left != nil {
		return m.left
	}
	return m.img
}

func lum(r, g, b byte) float64 { return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 255 }

// load reads the light map back once for its summary; a light map of another size is dropped.
func (m *mat) load() *mat {
	if m.loaded {
		return m
	}
	m.loaded = true
	b := m.img.Bounds()
	w, h := b.Dx(), b.Dy()
	if m.lit == nil || m.lit.Bounds().Size() != b.Size() {
		m.lit = nil
		return m
	}
	pix := make([]byte, 4*w*h)
	m.lit.ReadPixels(pix)
	var n, ys float64
	for i := 0; i < w*h; i++ {
		if l := lum(pix[4*i], pix[4*i+1], pix[4*i+2]); l > 0.1 {
			n += l
			ys += l * float64(i/w)
			for c := range 3 {
				m.lightCol[c] += l * float64(pix[4*i+c]) / 255
			}
		}
	}
	if n > 0 {
		for c := range 3 {
			m.lightCol[c] /= n
		}
		m.lightHei = 1 - ys/n/float64(h)
		m.lightPow = math.Min(1, n/float64(w*h)/0.03)
	}
	return m
}

// darkMul is how strongly distance darkens: 1 at torch 1, less with a brighter torch or a Light
// spell, 0 in daylight.
func (g *Game) darkMul() float64 {
	if openSky {
		return 0
	}
	return 1 / (g.torch + math.Min(1.5, lightLeft)*0.5)
}

// torchPos is the torch in camera space: exactly at the eye.
func (g *Game) torchPos() [3]float64 { return [3]float64{} }

// flicker is the torch's brightness factor: a few incommensurate sines, so it never visibly repeats.
func (g *Game) flicker() float64 {
	t := g.time
	return 1 + 0.06*math.Sin(t*7.3) + 0.04*math.Sin(t*13.1+1) + 0.03*math.Sin(t*23.7+2)
}

// torchPow is the torch's intensity, flickering; it saturates within arm's length.
func (g *Game) torchPow() float64 {
	if openSky {
		return 1e3
	}
	return 1.6 * g.flicker()
}

// torchRange is the distance where the inverse-square falloff 1/(1+d²/R²) has halved: further with
// the torch setting or a Light spell.
func (g *Game) torchRange() float64 {
	if openSky {
		return 1e3
	}
	return 2.4 / math.Sqrt(g.darkMul())
}

// torchLight is how much torch light reaches distance d, facing it head on (0..1). The shader has a copy.
func (g *Game) torchLight(d float64) float64 {
	r := g.torchRange()
	return math.Min(1, g.torchPow()/(1+d*d/(r*r)))
}

// Face modes for the texture shader.
const (
	modeShaded = iota // darkened by distance, lit by the moving lights
	modeUnlit         // drawn as is (signs, self-lit sprites)
	modeGlow          // additive, tinted by the vertex colour
)

// texSrc: perspective-correct texturing (custom = u/z, v/z, 1/z, y/z), lit by the flickering torch
// (banded with Dither) and faded to Deepnight's fog tint.
const texSrc = `//kage:unit pixels
package main

var Mode float
var Pow float     // torch intensity (flicker included)
var Range float   // torch falloff range
var Torch vec3    // torch position in camera space
var Warm vec3     // torch colour (white in daylight)
var Inc float     // constant extra fade toward the fog: 0.3 sides, 0.4 floors, 0 fronts
var HasLit float  // 1 when image 2 is a light map
var Dither float  // 1: checker dither and hard darkness bands; 0: smooth darkness
var Center vec2   // screen centre
var Focal float
var Px float      // screen pixels per view pixel of theirs (500 wide), for the checker dither
var HasBump float // 1 when image 1 is a slope map
var Bump float    // normal mapping strength
var T vec3        // the face's u and v directions and normal, in camera space (x right, y down, z ahead)
var B vec3
var N vec3
var Lamp vec3     // the moving light reaching this face most, in camera space

func tex(uv vec2) vec4 { return imageSrc0At(imageSrc0Origin() + fract(uv)*imageSrc0Size()) }

// torch is the light reaching camera-space point p: inverse-square falloff by distance only, so the
// floor, ceiling and walls at one distance get the same light (no angle term: with the torch at the eye
// it would leave floor and ceiling always darker). In 7 hard bands with Dither.
func torch(p vec3, chk float) float {
	lv := Torch - p
	d := length(lv)
	l := Pow / (1 + d*d/(Range*Range))
	if Dither == 0 {
		return clamp(l, 0, 1)
	}
	return clamp(ceil(l*7-chk*0.5)/7, 0, 1)
}

func shade(c vec3, dark, inc float) vec3 {
	fog := vec3(0x16, 0x10, 0x23) / 255
	r := clamp(dark/0.6, 0, 1)
	l := dot(c, vec3(0.299, 0.587, 0.114))
	t := fog
	if l*2 > r {
		t = vec3(0x25, 0x1f, 0x32) / 255
	} else if l*4 > r {
		t = vec3(0x1e, 0x18, 0x2b) / 255
	}
	c = mix(c, t, r)
	return mix(c, fog, clamp(clamp((dark-0.7)/0.3, 0, 1)*0.6+inc, 0, 1))
}

// relief is how much slopes s turn the surface toward light direction l, relative to the flat face.
func relief(s vec2, l vec3) float {
	return (s.x*dot(l, T) + s.y*dot(l, B)) / max(dot(N, l), 0.3)
}

func Fragment(dstPos vec4, srcPos vec2, color vec4, custom vec4) vec4 {
	z := 1 / custom.z
	uv := custom.xy * z
	if Mode == 2 {
		return tex(uv) * color
	}
	c := tex(uv)
	if c.a == 0 {
		discard()
	}
	if Mode == 1 {
		return c
	}
	rgb := c.rgb / c.a
	x := (dstPos.x - imageDstOrigin().x - Center.x) / Focal * z // dstPos counts from the atlas, not our image
	p := vec3(x, custom.w*z, z)
	q := floor(dstPos.xy / Px)
	chk := mod(q.x+q.y, 2) * Dither
	lit := 0.0
	if HasLit > 0 {
		lit = imageSrc2At(imageSrc0Origin() + fract(uv)*imageSrc0Size()).r
	}
	// the moving lights (vertex colour) push the darkness back and tint what they reach
	near := clamp(max(color.r, max(color.g, color.b)), 0, 1)
	if HasBump > 0 {
		// normal mapping, lit by the torch at the eye: a slope facing the light brightens, one facing
		// away darkens, by how far the tilt turns the surface toward the light relative to the flat face
		// (linear in the slope, so on average the surface keeps its brightness)
		s := imageSrc1At(imageSrc0Origin()+fract(uv)*imageSrc0Size()).rg*2 - 1
		// the torch, and where a lantern reaches, that lantern
		r := mix(relief(s, normalize(Torch-p)), relief(s, normalize(Lamp-p)), near)
		rgb *= clamp(1+Bump*r, 0.4, 1.6)
	}
	dark := (1 - torch(p, chk)) * (1 - 0.7*lit) * (1 - near)
	out := shade(rgb*mix(Warm, vec3(1), max(lit, near)), dark, Inc*(1-lit)*(1-near)) + rgb*color.rgb*0.3
	return vec4(out*c.a, c.a)
}
`

// frame is face f's u and v directions and its normal toward the eye, in camera space.
func frame(c cam, f *face) (t, b, n []float32, ok bool) {
	p0, p1, p3 := f.v[0], f.v[1], f.v[3]
	e1 := [3]float64{p1.x - p0.x, p1.y - p0.y, p1.z - p0.z}
	e2 := [3]float64{p3.x - p0.x, p3.y - p0.y, p3.z - p0.z}
	du1, dv1, du2, dv2 := p1.u-p0.u, p1.v-p0.v, p3.u-p0.u, p3.v-p0.v
	det := du1*dv2 - du2*dv1
	if math.Abs(det) < 1e-9 {
		return nil, nil, nil, false
	}
	var tv, bv [3]float64
	for i := range 3 {
		tv[i] = (e1[i]*dv2 - e2[i]*dv1) / det
		bv[i] = (e2[i]*du1 - e1[i]*du2) / det
	}
	// to camera space (y stays), unit length
	cs := func(v [3]float64) [3]float64 {
		r := [3]float64{v[0]*c.rtx + v[2]*c.rtz, v[1], v[0]*c.fwx + v[2]*c.fwz}
		l := math.Hypot(math.Hypot(r[0], r[1]), r[2])
		return [3]float64{r[0] / l, r[1] / l, r[2] / l}
	}
	tv, bv = cs(tv), cs(bv)
	nv := [3]float64{tv[1]*bv[2] - tv[2]*bv[1], tv[2]*bv[0] - tv[0]*bv[2], tv[0]*bv[1] - tv[1]*bv[0]}
	nl := math.Hypot(math.Hypot(nv[0], nv[1]), nv[2])
	dx, dz := p0.x-c.x, p0.z-c.z
	if nv[0]*(dx*c.rtx+dz*c.rtz)+nv[1]*p0.y+nv[2]*(dx*c.fwx+dz*c.fwz) > 0 { // facing away: flip toward the eye
		nl = -nl
	}
	f32 := func(v [3]float64, l float64) []float32 {
		return []float32{float32(v[0] / l), float32(v[1] / l), float32(v[2] / l)}
	}
	return f32(tv, 1), f32(bv, 1), f32(nv, nl), true
}

// lamp is the moving light that reaches face f's centre most, in camera space (the eye when none does).
func lamp(c cam, f *face) []float32 {
	var x, y, z float64
	for _, v := range f.v {
		x, y, z = x+v.x/4, y+v.y/4, z+v.z/4
	}
	best, out := 0.0, []float32{0, 0, 0}
	for _, e := range ents {
		if e.light <= 0 {
			continue
		}
		d := (x-e.x)*(x-e.x) + (y-e.y)*(y-e.y) + (z-e.z)*(z-e.z)
		if a := e.light / (1 + 0.5*d*d/e.light); a > best { // as light() weighs it
			dx, dz := e.x-c.x, e.z-c.z
			best, out = a, []float32{float32(dx*c.rtx + dz*c.rtz), float32(e.y), float32(dx*c.fwx + dz*c.fwz)}
		}
	}
	return out
}

func btof(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// faceOptions sets the texture shader's images and uniforms for face f seen from c.
func (g *Game) faceOptions(c cam, f *face) *ebiten.DrawTrianglesShaderOptions {
	op := &ebiten.DrawTrianglesShaderOptions{}
	op.Images[0], op.Images[1], op.Images[2] = f.tex, f.tex, f.tex
	tp, warm := g.torchPos(), []float32{1.08, 0.94, 0.78}
	if openSky {
		warm = []float32{1, 1, 1}
	}
	u := map[string]any{"Mode": float32(f.mode), "Dither": float32(btof(g.dither)), "Pow": float32(g.torchPow()), "Range": float32(g.torchRange()), "Inc": float32(f.inc),
		"Torch": []float32{float32(tp[0]), float32(tp[1]), float32(tp[2])}, "Warm": warm,
		"Center": []float32{float32(W) / 2, float32(H) / 2}, "Focal": float32(focal), "Px": float32(math.Max(1, float64(W)/500))}
	if m := f.m; m != nil && m.lit != nil && Settings.LightMaps {
		op.Images[2], u["HasLit"] = m.lit, float32(1)
	}
	if f.mode == modeShaded {
		if t, b, n, ok := frame(c, f); ok { // the normal is for the torch's Lambert term too
			u["T"], u["B"], u["N"] = t, b, n
			if s := slopes[f.tex]; s != nil && Settings.NormalMaps {
				op.Images[1], u["HasBump"], u["Bump"], u["Lamp"] = s, float32(1), float32(bump*2), lamp(c, f)
			}
		}
	}
	if f.mode == modeGlow {
		op.Blend = ebiten.BlendLighter
	}
	op.Uniforms = u
	return op
}
