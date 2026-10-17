package battle

import (
	"image"
	"image/color"
	"math"

	"gpu"

	"por/engine"
)

// The world: one unit per square, X along the map's columns, Z down its
// rows, Y up. Walls are one square tall.

const (
	atlasCols = 16 // tiles per atlas row
	atlasRows = 17 // the game's 256 tiles, then a row of our own
	tilePx    = 24 // a tile's side in pixels
	wallSlot  = 256
	floorSlot = 257
)

// kind is how a square stands in the world.
type kind int

const (
	plain kind = iota // the floor alone
	block             // a stone wall block
	stand             // a tree, a cactus, a boulder: its tile extruded upright
	flat              // water, a body, grass: its tile lying on the floor
)

// kindOf decides from the square and its tile set: dungeon walls are
// blocks (their pieces only draw the block in the original's perspective,
// so other dungeon squares are floor); outdoors, what blocks sight stands
// up and the rest lies flat.
func kindOf(s engine.Square, dungeon bool, empty func(int) bool) kind {
	switch {
	case dungeon && s.Tile < engine.DungeonTiles:
		if s.Wall {
			return block
		}
		return plain
	case empty(s.Tile):
		return plain
	case s.Wall && s.Height > 0:
		return stand
	}
	return flat
}

// mapMesh builds the whole map as one mesh over the atlas: the floor with
// corner occlusion, the blocks, and the outdoor things.
func mapMesh(g *[25][50]engine.Square, dungeon bool, empty func(int) bool, tiles []image.Image) []gpu.MeshVertex {
	kinds := [25][50]kind{}
	for z := range g {
		for x := range g[z] {
			kinds[z][x] = kindOf(g[z][x], dungeon, empty)
		}
	}
	solid := func(x, z int) bool { return x >= 0 && z >= 0 && x < 50 && z < 25 && kinds[z][x] == block }
	// occlusion of the floor corner at grid point (x, z): blocks among its four squares
	ao := func(x, z int) float32 {
		n := 0
		for _, d := range [4][2]int{{-1, -1}, {0, -1}, {-1, 0}, {0, 0}} {
			if solid(x+d[0], z+d[1]) {
				n++
			}
		}
		return 1 - 0.22*float32(n)
	}
	up := [3]float32{0, 1, 0}
	var vs []gpu.MeshVertex
	for z := range 25 {
		for x := range 50 {
			fx, fz := float32(x), float32(z)
			k := kinds[z][x]
			if k != block {
				vs = shade(quad(vs, [4][3]float32{{fx, 0, fz + 1}, {fx + 1, 0, fz + 1}, {fx + 1, 0, fz}, {fx, 0, fz}}, tileUV(floorSlot), up),
					ao(x, z+1), ao(x+1, z+1), ao(x+1, z), ao(x, z))
			}
			switch k {
			case flat:
				const y = 0.005 // just over the floor
				vs = quad(vs, [4][3]float32{{fx, y, fz + 1}, {fx + 1, y, fz + 1}, {fx + 1, y, fz}, {fx, y, fz}}, tileUV(g[z][x].Tile), up)
			case stand:
				uv := tileUV(g[z][x].Tile)
				u0, v0, du, dv := uv[3][0], uv[3][1], uv[1][0]-uv[3][0], uv[1][1]-uv[3][1]
				for _, v := range extrude(tiles[g[z][x].Tile]) {
					v.X, v.Y, v.Z = v.X+fx+.5, v.Y+.5, v.Z+fz+.5
					v.U, v.V = u0+v.U*du, v0+v.V*dv
					vs = append(vs, v)
				}
			case block:
				side := tileUV(wallSlot)
				vs = quad(vs, [4][3]float32{{fx, 1, fz + 1}, {fx + 1, 1, fz + 1}, {fx + 1, 1, fz}, {fx, 1, fz}}, side, up)
				// sides: darker toward the floor (occlusion where wall meets ground)
				if !solid(x, z+1) {
					vs = shade(quad(vs, [4][3]float32{{fx, 0, fz + 1}, {fx + 1, 0, fz + 1}, {fx + 1, 1, fz + 1}, {fx, 1, fz + 1}}, side, [3]float32{0, 0, 1}), .65, .65, 1, 1)
				}
				if !solid(x, z-1) {
					vs = shade(quad(vs, [4][3]float32{{fx + 1, 0, fz}, {fx, 0, fz}, {fx, 1, fz}, {fx + 1, 1, fz}}, side, [3]float32{0, 0, -1}), .65, .65, 1, 1)
				}
				if !solid(x+1, z) {
					vs = shade(quad(vs, [4][3]float32{{fx + 1, 0, fz + 1}, {fx + 1, 0, fz}, {fx + 1, 1, fz}, {fx + 1, 1, fz + 1}}, side, [3]float32{1, 0, 0}), .65, .65, 1, 1)
				}
				if !solid(x-1, z) {
					vs = shade(quad(vs, [4][3]float32{{fx, 0, fz}, {fx, 0, fz + 1}, {fx, 1, fz + 1}, {fx, 1, fz}}, side, [3]float32{-1, 0, 0}), .65, .65, 1, 1)
				}
			}
		}
	}
	return vs
}

// shade darkens the last quad's corners (in quad order) by a, b, c, d.
func shade(vs []gpu.MeshVertex, a, b, c, d float32) []gpu.MeshVertex {
	q := vs[len(vs)-6:]
	for i, k := range [6]float32{a, b, c, a, c, d} {
		q[i].R, q[i].G, q[i].B = k, k, k
	}
	return vs
}

// tileUV is atlas slot t's corners in quad order: bottom left, bottom
// right, top right, top left (a hair inside, so nearest sampling never
// reaches the neighbour).
func tileUV(t int) [4][2]float32 {
	const w, h = atlasCols * tilePx, atlasRows * tilePx
	const in = 0.01
	u0, v0 := (float32(t%atlasCols*tilePx)+in)/w, (float32(t/atlasCols*tilePx)+in)/h
	u1, v1 := u0+(tilePx-2*in)/w, v0+(tilePx-2*in)/h
	return [4][2]float32{{u0, v1}, {u1, v1}, {u1, v0}, {u0, v0}}
}

// floorTile is the ground's texture: its colour with a little grain, and
// a faint edge so the squares read (a pixel darker all round).
func floorTile(c color.RGBA) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, tilePx, tilePx))
	for y := range tilePx {
		for x := range tilePx {
			h := uint32(x/2*73856093^y/2*19349663) % 97 // 2x2 grain, the same every square
			k := 0.94 + float64(h)/97*0.1
			if x == 0 || y == 0 || x == tilePx-1 || y == tilePx-1 {
				k *= 0.82
			}
			f := func(v uint8) uint8 { return uint8(min(255, float64(v)*k)) }
			im.SetRGBA(x, y, color.RGBA{f(c.R), f(c.G), f(c.B), 255})
		}
	}
	return im
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

type vec [3]float64

func (a vec) sub(b vec) vec     { return vec{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func (a vec) add(b vec) vec     { return vec{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func (a vec) mul(k float64) vec { return vec{a[0] * k, a[1] * k, a[2] * k} }
func (a vec) dot(b vec) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func (a vec) norm() vec         { return a.mul(1 / math.Sqrt(a.dot(a))) }
func (a vec) cross(b vec) vec {
	return vec{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

// camera is a view of the map: where from, toward what, how wide.
type camera struct {
	eye, at vec
	fovy    float64
	aspect  float64
}

// basis is the camera's right, up and forward directions.
func (c camera) basis() (r, u, f vec) {
	f = c.at.sub(c.eye).norm()
	r = f.cross(vec{0, 1, 0}).norm()
	return r, r.cross(f), f
}

func (c camera) viewProj() gpu.Mat4 {
	r, u, f := c.basis()
	z := f.mul(-1)
	e := c.eye
	view := gpu.Mat4{float32(r[0]), float32(u[0]), float32(z[0]), 0, float32(r[1]), float32(u[1]), float32(z[1]), 0,
		float32(r[2]), float32(u[2]), float32(z[2]), 0, float32(-r.dot(e)), float32(-u.dot(e)), float32(-z.dot(e)), 1}
	return mul(perspective(float32(c.fovy), float32(c.aspect), 0.1, 200), view)
}

// pick is the map square under screen point (nx, ny) in -1..1 (y up), on
// the floor; ok is false off the map.
func (c camera) pick(nx, ny float64) (x, z int, ok bool) {
	r, u, f := c.basis()
	t := math.Tan(c.fovy / 2)
	d := f.add(r.mul(nx * t * c.aspect)).add(u.mul(ny * t))
	if d[1] >= 0 {
		return 0, 0, false
	}
	p := c.eye.add(d.mul(-c.eye[1] / d[1]))
	x, z = int(math.Floor(p[0])), int(math.Floor(p[2]))
	return x, z, x >= 0 && z >= 0 && x < 50 && z < 25
}
