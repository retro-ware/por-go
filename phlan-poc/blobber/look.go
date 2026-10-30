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

// Face modes for the mesh shader (mesh.go).
const (
	modeShaded = iota // darkened by distance, lit by the moving lights
	modeUnlit         // drawn as is (signs, self-lit sprites)
	modeGlow          // additive, tinted by the vertex colour
)

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
