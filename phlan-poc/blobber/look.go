package blobber

// Deepnight's "Labyrinth" look on the 3D quads: everything darkens in hard, dithered bands toward a
// purple fog instead of fading smoothly to black. Light maps (NAME_light.png) keep parts of a wall
// lit and put a glow in front of it; moving lights draw as additive glows with light circles on the
// floor and ceiling.

import (
	"image/color"
	"math"

	"gpu"
)

var fogCol = color.RGBA{0x16, 0x10, 0x23, 255}

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
	let dark = (1.0 - torch(p, chk)) * (1.0 - 0.7 * lit) * (1.0 - near);
	let o = shade(rgb * mix(u.Warm, vec3f(1.0), max(lit, near)), dark, u.Inc * (1.0 - lit) * (1.0 - near)) + rgb * color.rgb * 0.3;
	return vec4f(haze(o, p) * c.a, c.a);
}
`

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
	if openSky {
		u["Haze"] = float32(hazeCells)
	}
	if m := f.m; m != nil && m.lit != nil {
		op.Images[2], u["HasLit"] = m.lit, float32(1)
	}
	if f.mode == modeGlow {
		op.Blend = gpu.BlendLighter
	}
	op.Uniforms = u
	return op
}
