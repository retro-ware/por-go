package battle

// Copied from VoxelLib items/extrude and owned here: it emits gpu vertices.

import (
	"image"

	"gpu"
)

// alphaCut is the alpha below which a texel counts as transparent.
const alphaCut = 128

// extrude turns a sprite into a slab one texel thick, the way Minecraft
// draws items: front and back are quads with the whole sprite (the mesh
// shader discards transparent texels), and every opaque texel on an edge
// gets side faces in its own colour. The slab fits the unit square keeping
// the aspect ratio, centred on the origin, front facing +Z. Its thickness
// is half a texel of a sprite texels wide: a sharper picture standing in
// for one (a PNG icon) is as thick as the original would be.
func extrude(img image.Image, texels int) []gpu.MeshVertex {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil
	}
	px := float32(1) / float32(max(w, h))
	x0, y0 := -float32(w)*px/2, float32(h)*px/2
	t := 1 / float32(2*max(texels, 1))
	opaque := func(x, y int) bool {
		if x < 0 || y < 0 || x >= w || y >= h {
			return false
		}
		_, _, _, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
		return a>>8 >= alphaCut
	}
	pos := func(gx, gy int, z float32) [3]float32 {
		return [3]float32{x0 + float32(gx)*px, y0 - float32(gy)*px, z}
	}
	uvAt := func(gx, gy int) [2]float32 { return [2]float32{float32(gx) / float32(w), float32(gy) / float32(h)} }
	var vs []gpu.MeshVertex
	vs = quad(vs, [4][3]float32{pos(0, h, t), pos(w, h, t), pos(w, 0, t), pos(0, 0, t)},
		[4][2]float32{uvAt(0, h), uvAt(w, h), uvAt(w, 0), uvAt(0, 0)}, [3]float32{0, 0, 1})
	vs = quad(vs, [4][3]float32{pos(w, h, -t), pos(0, h, -t), pos(0, 0, -t), pos(w, 0, -t)},
		[4][2]float32{uvAt(w, h), uvAt(0, h), uvAt(0, 0), uvAt(w, 0)}, [3]float32{0, 0, -1})
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !opaque(x, y) {
				continue
			}
			c := [2]float32{(float32(x) + 0.5) / float32(w), (float32(y) + 0.5) / float32(h)}
			uv := [4][2]float32{c, c, c, c}
			if !opaque(x, y-1) {
				vs = quad(vs, [4][3]float32{pos(x, y, t), pos(x+1, y, t), pos(x+1, y, -t), pos(x, y, -t)}, uv, [3]float32{0, 1, 0})
			}
			if !opaque(x, y+1) {
				vs = quad(vs, [4][3]float32{pos(x, y+1, -t), pos(x+1, y+1, -t), pos(x+1, y+1, t), pos(x, y+1, t)}, uv, [3]float32{0, -1, 0})
			}
			if !opaque(x-1, y) {
				vs = quad(vs, [4][3]float32{pos(x, y+1, -t), pos(x, y+1, t), pos(x, y, t), pos(x, y, -t)}, uv, [3]float32{-1, 0, 0})
			}
			if !opaque(x+1, y) {
				vs = quad(vs, [4][3]float32{pos(x+1, y+1, t), pos(x+1, y+1, -t), pos(x+1, y, -t), pos(x+1, y, t)}, uv, [3]float32{1, 0, 0})
			}
		}
	}
	return vs
}

// quad appends two counter-clockwise triangles (seen from the normal side), white.
func quad(vs []gpu.MeshVertex, p [4][3]float32, uv [4][2]float32, n [3]float32) []gpu.MeshVertex {
	for _, i := range [6]int{0, 1, 2, 0, 2, 3} {
		vs = append(vs, gpu.MeshVertex{X: p[i][0], Y: p[i][1], Z: p[i][2], U: uv[i][0], V: uv[i][1],
			NX: n[0], NY: n[1], NZ: n[2], R: 1, G: 1, B: 1, A: 1})
	}
	return vs
}
