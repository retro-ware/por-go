package battle

import (
	"math"

	"gpu"

	"por/engine"
)

// The world: one unit per square, X along the map's columns, Z down its
// rows, Y up. Walls are one square tall.

const (
	atlasCols = 16 // tiles per atlas row (256 tiles: 16 rows)
	tilePx    = 24 // a tile's side in pixels
	wallTile  = 3  // the plain stone wall (the others are pieces): every wall block's faces
	lastPiece = 24 // tiles 1..lastPiece: the wall's sides as the original paints them in perspective
)

// mapMesh builds the floor and the wall blocks, textured from the tile
// atlas: floors show their square's own tile, blocks the plain stone wall
// on every face (the original's wall pieces have its perspective drawn in;
// a block is what they draw).
func mapMesh(g *[25][50]engine.Square) []gpu.MeshVertex {
	wall := func(x, z int) bool { return x >= 0 && z >= 0 && x < 50 && z < 25 && g[z][x].Wall }
	floor := plainFloor(g)
	var vs []gpu.MeshVertex
	for z := range 25 {
		for x := range 50 {
			s := g[z][x]
			fx, fz := float32(x), float32(z)
			if !s.Wall {
				t := s.Tile
				if t <= lastPiece { // a painted wall side: the blocks show it now
					t = floor
				}
				vs = quad(vs, [4][3]float32{{fx, 0, fz + 1}, {fx + 1, 0, fz + 1}, {fx + 1, 0, fz}, {fx, 0, fz}}, tileUV(t), [3]float32{0, 1, 0})
				continue
			}
			side := tileUV(wallTile)
			vs = quad(vs, [4][3]float32{{fx, 1, fz + 1}, {fx + 1, 1, fz + 1}, {fx + 1, 1, fz}, {fx, 1, fz}}, side, [3]float32{0, 1, 0})
			if !wall(x, z+1) { // south
				vs = quad(vs, [4][3]float32{{fx, 0, fz + 1}, {fx + 1, 0, fz + 1}, {fx + 1, 1, fz + 1}, {fx, 1, fz + 1}}, side, [3]float32{0, 0, 1})
			}
			if !wall(x, z-1) { // north
				vs = quad(vs, [4][3]float32{{fx + 1, 0, fz}, {fx, 0, fz}, {fx, 1, fz}, {fx + 1, 1, fz}}, side, [3]float32{0, 0, -1})
			}
			if !wall(x+1, z) { // east
				vs = quad(vs, [4][3]float32{{fx + 1, 0, fz + 1}, {fx + 1, 0, fz}, {fx + 1, 1, fz}, {fx + 1, 1, fz + 1}}, side, [3]float32{1, 0, 0})
			}
			if !wall(x-1, z) { // west
				vs = quad(vs, [4][3]float32{{fx, 0, fz}, {fx, 0, fz + 1}, {fx, 1, fz + 1}, {fx, 1, fz}}, side, [3]float32{-1, 0, 0})
			}
		}
	}
	return vs
}

// plainFloor is the map's most common walkable tile.
func plainFloor(g *[25][50]engine.Square) int {
	n := map[int]int{}
	best := 0
	for z := range g {
		for _, s := range g[z] {
			if !s.Wall {
				if n[s.Tile]++; n[s.Tile] > n[best] {
					best = s.Tile
				}
			}
		}
	}
	return best
}

// tileUV is tile t's corners in the atlas, in quad order: bottom left,
// bottom right, top right, top left (half a texel in, so nearest sampling
// never reaches the neighbour).
func tileUV(t int) [4][2]float32 {
	const n = atlasCols * tilePx
	const in = 0.5 / n
	u0, v0 := float32(t%atlasCols*tilePx)/n+in, float32(t/atlasCols*tilePx)/n+in
	u1, v1 := u0+tilePx/float32(n)-2*in, v0+tilePx/float32(n)-2*in
	return [4][2]float32{{u0, v1}, {u1, v1}, {u1, v0}, {u0, v0}}
}

// box is a unit box: X and Z -0.5..0.5, Y 0..1.
func box() []gpu.MeshVertex {
	uv := [4][2]float32{}
	var vs []gpu.MeshVertex
	vs = quad(vs, [4][3]float32{{-.5, 1, .5}, {.5, 1, .5}, {.5, 1, -.5}, {-.5, 1, -.5}}, uv, [3]float32{0, 1, 0})
	vs = quad(vs, [4][3]float32{{-.5, 0, .5}, {.5, 0, .5}, {.5, 1, .5}, {-.5, 1, .5}}, uv, [3]float32{0, 0, 1})
	vs = quad(vs, [4][3]float32{{.5, 0, -.5}, {-.5, 0, -.5}, {-.5, 1, -.5}, {.5, 1, -.5}}, uv, [3]float32{0, 0, -1})
	vs = quad(vs, [4][3]float32{{.5, 0, .5}, {.5, 0, -.5}, {.5, 1, -.5}, {.5, 1, .5}}, uv, [3]float32{1, 0, 0})
	vs = quad(vs, [4][3]float32{{-.5, 0, -.5}, {-.5, 0, .5}, {-.5, 1, .5}, {-.5, 1, -.5}}, uv, [3]float32{-1, 0, 0})
	return vs
}

// mul is a*b (column-major: b applies first).
func mul(a, b gpu.Mat4) gpu.Mat4 {
	var r gpu.Mat4
	for c := range 4 {
		for row := range 4 {
			for k := range 4 {
				r[c*4+row] += a[k*4+row] * b[c*4+k]
			}
		}
	}
	return r
}

// place is translate(x, y, z) * rotateY(yaw) * scale(sx, sy, sz).
func place(x, y, z, yaw, sx, sy, sz float32) gpu.Mat4 {
	c, s := float32(math.Cos(float64(yaw))), float32(math.Sin(float64(yaw)))
	return gpu.Mat4{c * sx, 0, -s * sx, 0, 0, sy, 0, 0, s * sz, 0, c * sz, 0, x, y, z, 1}
}

// perspective is a right-handed projection with z 0..1.
func perspective(fovy, aspect, near, far float32) gpu.Mat4 {
	f := 1 / float32(math.Tan(float64(fovy)/2))
	return gpu.Mat4{f / aspect, 0, 0, 0, 0, f, 0, 0, 0, 0, far / (near - far), -1, 0, 0, near * far / (near - far), 0}
}

// lookAt is the view from eye toward at, Y up.
func lookAt(eye, at [3]float32) gpu.Mat4 {
	sub := func(a, b [3]float32) [3]float32 { return [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
	cross := func(a, b [3]float32) [3]float32 {
		return [3]float32{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
	}
	dot := func(a, b [3]float32) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
	norm := func(a [3]float32) [3]float32 {
		l := float32(math.Sqrt(float64(dot(a, a))))
		return [3]float32{a[0] / l, a[1] / l, a[2] / l}
	}
	z := norm(sub(eye, at))
	x := norm(cross([3]float32{0, 1, 0}, z))
	y := cross(z, x)
	return gpu.Mat4{x[0], y[0], z[0], 0, x[1], y[1], z[1], 0, x[2], y[2], z[2], 0, -dot(x, eye), -dot(y, eye), -dot(z, eye), 1}
}
