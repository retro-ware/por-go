package blobber

// Deepnight's "Labyrinth" look on the 3D quads: everything darkens in hard, dithered bands toward a
// purple fog instead of fading smoothly to black. Light maps (NAME_light.png) keep parts of a wall
// lit and put a glow in front of it; moving lights draw as additive glows with light circles on the
// floor and ceiling.

import (
	"image"
	"image/color"
	"math"

	"gpu"
)

var fogCol = color.RGBA{0x16, 0x10, 0x23, 255}

// slopes holds each wall, floor and ceiling texture's slope map for the normal mapping: red and
// green are the height's slope along u and v, 0.5 flat.
var slopes = map[*gpu.Image]*gpu.Image{}

// addSlopes makes img's slope map from its w×h height field (wrapping, like the texture).
func addSlopes(img *gpu.Image, w, h int, height func(x, y int) float64) {
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
	s := gpu.NewImage(w, h)
	s.WritePixels(pix)
	slopes[img] = s
}

// normalSlopes makes img's slope map from normal map nm: a normal (x, y up, z) leans the height by
// -x/z along u and y/z along v (v runs down), halved to the scale addSlopes' height differences have.
func normalSlopes(img *gpu.Image, nm image.Image) {
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
	s := gpu.NewImage(w, h)
	s.WritePixels(pix)
	slopes[img] = s
}

// mat is a wall material: its front texture, the generated ones' left and right lit variants
// (nil for painted textures, which use img on every face), and an optional light map.
type mat struct {
	img, right, left, lit *gpu.Image
	loaded                bool
	// light map summary (TextureAtlas): average colour weighted by brightness, the glow's height
	// (1 top) and strength (1 when 3% of the texture is lit)
	lightCol           [3]float64
	lightHei, lightPow float64
}

// face picks the texture for a face whose normal's lateral component in camera space is nx.
func (m *mat) face(nx float64) *gpu.Image {
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
const texSrc = `
struct Uniforms {
	Mode: f32,
	Pow: f32,     // torch intensity (flicker included)
	Range: f32,   // torch falloff range
	Torch: vec3f, // torch position in camera space
	Warm: vec3f,  // torch colour (white in daylight)
	Inc: f32,     // constant extra fade toward the fog: 0.3 sides, 0.4 floors, 0 fronts
	HasLit: f32,  // 1 when image 2 is a light map
	Dither: f32,  // 1: checker dither and hard darkness bands; 0: smooth darkness
	Center: vec2f, // screen centre
	Focal: f32,
	Px: f32,      // screen pixels per view pixel of theirs (500 wide), for the checker dither
	HasBump: f32, // 1 when image 1 is a slope map
	Bump: f32,    // normal mapping strength
	T: vec3f,     // the face's u and v directions and normal, in camera space (x right, y down, z ahead)
	B: vec3f,
	N: vec3f,
	Lamp: vec3f,  // the moving light reaching this face most, in camera space
	Haze: f32,    // distance fog: cells to full haze (0 = none)
	HazeCol: vec3f, // the sky's colour at the horizon
}

// haze fades c toward the horizon's colour with distance: a little within the map, full Haze cells out.
fn haze(c: vec3f, p: vec3f) -> vec3f {
	if (u.Haze == 0.0) {
		return c;
	}
	let h = clamp(length(p.xz) / u.Haze, 0.0, 1.0);
	return mix(c, u.HazeCol, h * h);
}

fn tex(uv: vec2f) -> vec4f { return imageSrc0At(imageSrc0Origin() + fract(uv) * imageSrc0Size()); }

// torch is the light reaching camera-space point p: inverse-square falloff by distance only, so the
// floor, ceiling and walls at one distance get the same light (no angle term: with the torch at the eye
// it would leave floor and ceiling always darker). In 7 hard bands with Dither.
fn torch(p: vec3f, chk: f32) -> f32 {
	let d = length(u.Torch - p);
	let l = u.Pow / (1.0 + d * d / (u.Range * u.Range));
	if (u.Dither == 0.0) {
		return clamp(l, 0.0, 1.0);
	}
	return clamp(ceil(l * 7.0 - chk * 0.5) / 7.0, 0.0, 1.0);
}

fn shade(c0: vec3f, dark: f32, inc: f32) -> vec3f {
	let fog = vec3f(22.0, 16.0, 35.0) / 255.0;
	let r = clamp(dark / 0.6, 0.0, 1.0);
	let l = dot(c0, vec3f(0.299, 0.587, 0.114));
	var t = fog;
	if (l * 2.0 > r) {
		t = vec3f(37.0, 31.0, 50.0) / 255.0;
	} else if (l * 4.0 > r) {
		t = vec3f(30.0, 24.0, 43.0) / 255.0;
	}
	let c = mix(c0, t, r);
	return mix(c, fog, clamp(clamp((dark - 0.7) / 0.3, 0.0, 1.0) * 0.6 + inc, 0.0, 1.0));
}

// relief is how much slopes s turn the surface toward light direction l, relative to the flat face.
fn relief(s: vec2f, l: vec3f) -> f32 {
	return (s.x * dot(l, u.T) + s.y * dot(l, u.B)) / max(dot(u.N, l), 0.3);
}

fn Fragment(dstPos: vec4f, srcPos: vec2f, color: vec4f, custom: vec4f) -> vec4f {
	let z = 1.0 / custom.z;
	let uv = custom.xy * z;
	let c = tex(uv);
	if (u.Mode == 2.0) {
		return c * color;
	}
	if (c.a == 0.0) {
		discard;
	}
	if (u.Mode == 1.0) {
		return vec4f(haze(c.rgb / c.a, vec3f(0.0, 0.0, z)) * c.a, c.a);
	}
	var rgb = c.rgb / c.a;
	let x = (dstPos.x - imageDstOrigin().x - u.Center.x) / u.Focal * z;
	let p = vec3f(x, custom.w * z, z);
	let q = floor(dstPos.xy / u.Px);
	let chk = ((q.x + q.y) % 2.0) * u.Dither;
	var lit = 0.0;
	if (u.HasLit > 0.0) {
		lit = imageSrc2At(imageSrc0Origin() + fract(uv) * imageSrc0Size()).r;
	}
	// the moving lights (vertex colour) push the darkness back and tint what they reach
	let near = clamp(max(color.r, max(color.g, color.b)), 0.0, 1.0);
	if (u.HasBump > 0.0) {
		// normal mapping, lit by the torch at the eye: a slope facing the light brightens, one facing
		// away darkens, by how far the tilt turns the surface toward the light relative to the flat face
		// (linear in the slope, so on average the surface keeps its brightness)
		let s = imageSrc1At(imageSrc0Origin() + fract(uv) * imageSrc0Size()).rg * 2.0 - 1.0;
		// the torch, and where a lantern reaches, that lantern
		let r = mix(relief(s, normalize(u.Torch - p)), relief(s, normalize(u.Lamp - p)), near);
		rgb *= clamp(1.0 + u.Bump * r, 0.4, 1.6);
	}
	let dark = (1.0 - torch(p, chk)) * (1.0 - 0.7 * lit) * (1.0 - near);
	let o = shade(rgb * mix(u.Warm, vec3f(1.0), max(lit, near)), dark, u.Inc * (1.0 - lit) * (1.0 - near)) + rgb * color.rgb * 0.3;
	return vec4f(haze(o, p) * c.a, c.a);
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

// hazeCells is how far the distance fog reaches full strength (phlan-poc: past the map, out to the horizon).
const hazeCells = 128

// hazeCol is the sky's colour just above the horizon, day or night.
func hazeCol() []float32 {
	if Night {
		return []float32{10.0 / 255, 32.0 / 255, 70.0 / 255}
	}
	return []float32{165.0 / 255, 195.0 / 255, 220.0 / 255}
}

func btof(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// faceOptions sets the texture shader's images and uniforms for face f seen from c.
func (g *Game) faceOptions(c cam, f *face) *gpu.DrawTrianglesShaderOptions {
	op := &gpu.DrawTrianglesShaderOptions{}
	op.Images[0], op.Images[1], op.Images[2] = f.tex, f.tex, f.tex
	tp, warm := g.torchPos(), []float32{1.08, 0.94, 0.78}
	if openSky {
		warm = []float32{1, 1, 1}
		if Night && f.mode != modeUnlit { // phlan-poc: moonlight, dim and blue
			warm = nightTint
		}
	}
	u := map[string]any{"Mode": float32(f.mode), "Dither": float32(btof(g.dither)), "Pow": float32(g.torchPow()), "Range": float32(g.torchRange()), "Inc": float32(f.inc),
		"Torch": []float32{float32(tp[0]), float32(tp[1]), float32(tp[2])}, "Warm": warm,
		"Haze": float32(0), "HazeCol": hazeCol(),
		"Center": []float32{float32(W) / 2, float32(H) / 2}, "Focal": float32(focal), "Px": float32(math.Max(1, float64(W)/500))}
	if openSky && Settings.Haze {
		u["Haze"] = float32(hazeCells)
	}
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
		op.Blend = gpu.BlendLighter
	}
	op.Uniforms = u
	return op
}
