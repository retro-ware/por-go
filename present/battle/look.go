package battle

import (
	"image"
	"image/color"
	"math"

	"gpu"

	"por/engine"
)

// The tabletop's own props: the stone texture, the bases, shadows, the
// board's edge and the table under it.

// stoneTexture is the dungeon wall's face: wall tile 3, nearly plain
// stone in both the DOS and the Amiga art, with its bevel (the triangle
// from the diagonal x-y = 16 to the top right corner) replaced by the stone
// mirrored across the main diagonal, from the clean bottom left. The
// ground-coloured gaps between stones (transparent in the tiles) show the
// ground's colour, as the original draws them.
func stoneTexture(tiles []image.Image, mortar color.RGBA) *image.RGBA {
	const base, bevel = 3, 16
	im := image.NewRGBA(image.Rect(0, 0, tilePx, tilePx))
	if len(tiles) <= base || tiles[base] == nil {
		return im
	}
	t := tiles[base]
	for y := range tilePx {
		for x := range tilePx {
			sx, sy := x, y
			if x-y >= bevel {
				sx, sy = y, x
			}
			r, g, b, a := t.At(t.Bounds().Min.X+sx, t.Bounds().Min.Y+sy).RGBA()
			c := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
			if a>>8 != 255 {
				c = mortar
			}
			im.SetRGBA(x, y, c)
		}
	}
	return im
}

// disc is a round base: radius 0.5, Y 0..1.
func disc(n int) []gpu.MeshVertex {
	var vs []gpu.MeshVertex
	uv := [4][2]float32{}
	for i := range n {
		a0, a1 := 2*math.Pi*float64(i)/float64(n), 2*math.Pi*float64(i+1)/float64(n)
		x0, z0 := float32(math.Sin(a0)/2), float32(math.Cos(a0)/2)
		x1, z1 := float32(math.Sin(a1)/2), float32(math.Cos(a1)/2)
		vs = quad(vs, [4][3]float32{{0, 1, 0}, {x0, 1, z0}, {x1, 1, z1}, {0, 1, 0}}, uv, [3]float32{0, 1, 0})
		am := (a0 + a1) / 2
		vs = quad(vs, [4][3]float32{{x0, 0, z0}, {x1, 0, z1}, {x1, 1, z1}, {x0, 1, z0}}, uv,
			[3]float32{float32(math.Sin(am)), 0, float32(math.Cos(am))})
	}
	return vs
}

// blob is a soft round spot, white with falling alpha: shadows tint it black, lights their colour.
func blob(n int) *image.NRGBA {
	im := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			dx, dy := (float64(x)+.5)/float64(n)*2-1, (float64(y)+.5)/float64(n)*2-1
			a := max(0, 1-math.Hypot(dx, dy))
			im.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(255 * math.Sqrt(a))})
		}
	}
	return im
}

// woodTexture is a dark oak plank: lengthwise grain.
func woodTexture(n int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			g := 0.5 + 0.5*math.Sin(float64(y)*0.9+math.Sin(float64(x)*0.11)*3)
			k := 0.8 + 0.2*g
			if y%16 == 0 { // the joint between planks
				k = 0.55
			}
			im.SetRGBA(x, y, color.RGBA{uint8(92 * k), uint8(60 * k), uint8(36 * k), 255})
		}
	}
	return im
}

// tableMesh is the table round the board, under its edge.
func tableMesh() []gpu.MeshVertex {
	var vs []gpu.MeshVertex
	const y, s = -boardEdge, 4 // plank squares of s units
	full := [4][2]float32{{0, 1}, {1, 1}, {1, 0}, {0, 0}}
	for z := float32(-24); z < 49; z += s {
		for x := float32(-24); x < 74; x += s {
			vs = quad(vs, [4][3]float32{{x, y, z + s}, {x + s, y, z + s}, {x + s, y, z}, {x, y, z}}, full, [3]float32{0, 1, 0})
		}
	}
	return vs
}

// boardSides are the board's edges down to the table, in the floor's colour, darkened.
func boardSides() []gpu.MeshVertex {
	uv := tileUV(floorSlot)
	var vs []gpu.MeshVertex
	const y = -boardEdge
	side := func(p [4][3]float32, n [3]float32) { vs = shade(quad(vs, p, uv, n), .4, .4, .6, .6) }
	side([4][3]float32{{0, y, 25}, {50, y, 25}, {50, 0, 25}, {0, 0, 25}}, [3]float32{0, 0, 1})
	side([4][3]float32{{50, y, 0}, {0, y, 0}, {0, 0, 0}, {50, 0, 0}}, [3]float32{0, 0, -1})
	side([4][3]float32{{50, y, 25}, {50, y, 0}, {50, 0, 0}, {50, 0, 25}}, [3]float32{1, 0, 0})
	side([4][3]float32{{0, y, 0}, {0, y, 25}, {0, 0, 25}, {0, 0, 0}}, [3]float32{-1, 0, 0})
	return vs
}

// standShadows are soft spots under what stands up and round the blocks' feet.
func standShadows(g *[25][50]engine.Square, dungeon bool, empty func(int) bool) []gpu.MeshVertex {
	full := [4][2]float32{{0, 1}, {1, 1}, {1, 0}, {0, 0}}
	var vs []gpu.MeshVertex
	for z := range g {
		for x := range g[z] {
			if k := kindOf(g[z][x], dungeon, empty); k == stand || k == block {
				const y, r = 0.003, 0.75
				cx, cz := float32(x)+.62, float32(z)+.6 // a little away from the sun
				vs = quad(vs, [4][3]float32{{cx - r, y, cz + r}, {cx + r, y, cz + r}, {cx + r, y, cz - r}, {cx - r, y, cz - r}}, full, [3]float32{0, 1, 0})
			}
		}
	}
	return vs
}
