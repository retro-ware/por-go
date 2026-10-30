package blobber

// The 3D view as meshes (phlan-poc). The walls, floor, ceiling and water of
// a map are built once into one vertex buffer (buildWorld) and drawn in a few
// calls a frame with the camera as a uniform; what moves (billboards, glows,
// the clouds) goes into a second buffer rebuilt each frame. The GPU depth
// tests, but the depth written is each face's painter's depth (its centre's
// distance along the view, less a bias), not the pixel's, so the picture is
// the one the painter's sort gave: floors and ceilings never hide anything,
// monsters stand over the side walls of their own cell, a thin wall shows one
// side. A frame is a few dozen draw calls and no geometry on the CPU.

import (
	"image/color"
	"math"
	"sort"

	"gpu"
	"gpu/vector"
)

// meshSrc: the wall shader (look.go's texSrc) for meshes. The vertex's ctr is the face's
// centre (x, 0, z) for the painter's depth, or (0, layer, 0) for a face nothing hides behind.
const meshSrc = `
struct U {
	rect: array<vec4f, 4>,   // the images' places in their atlas
	viewProj: mat4x4f,
	eye: vec4f,    // the eye; Mode (0 shaded, 1 unlit, 2 glow)
	fw: vec4f,     // the view direction (x, z); the projection's a, b: depth = a + b / z
	torch: vec4f,  // Pow, Range (look.go), Dither, Px
	haze: vec4f,   // HazeCol, Haze (cells to full haze; 0 none)
	warm: vec4f,   // the torch's colour; Clamp: 1 the picture's v is not tiled, 2 neither is u (a billboard)
	face: vec4f,   // the faces' normal (x, z; 0 0: two-sided), HasLit, Bias
	uvOff: vec4f,  // added to uv: drifting water, clouds
	lights: array<vec4f, 8>, // the moving lights: position, strength; colour, 0
}
struct VIn { @location(0) pos: vec3f, @location(1) uv: vec2f, @location(2) ctr: vec3f, @location(3) color: vec4f }
struct VOut { @builtin(position) pos: vec4f, @location(0) uv: vec2f, @location(1) color: vec4f, @location(2) world: vec3f }

@vertex
fn vs(v: VIn) -> VOut {
	var o: VOut;
	o.pos = u.viewProj * vec4f(v.pos, 1.0);
	var d = 1.0 + v.ctr.y * 0.0001;
	if (v.ctr.y >= 0.0) {
		let z = max(dot(v.ctr.xz - u.eye.xz, u.fw.xy) - u.face.w, 0.02);
		d = clamp(u.fw.z + u.fw.w / z, 0.0, 0.9999);
	}
	o.pos.z = d * o.pos.w;
	o.uv = v.uv + u.uvOff.xy;
	o.color = v.color;
	o.world = v.pos;
	return o;
}

fn haze(c: vec3f, rel: vec3f) -> vec3f {
	if (u.haze.w == 0.0) {
		return c;
	}
	let h = clamp(length(rel.xz) / u.haze.w, 0.0, 1.0);
	return mix(c, u.haze.xyz, h * h);
}

// torch is the light reaching a point d from the eye (look.go); in 7 hard bands with Dither
fn torch(d: f32, chk: f32) -> f32 {
	let l = u.torch.x / (1.0 + d * d / (u.torch.y * u.torch.y));
	if (u.torch.z == 0.0) {
		return clamp(l, 0.0, 1.0);
	}
	return clamp(ceil(l * 7.0 - chk * 0.5) / 7.0, 0.0, 1.0);
}

fn shade(c0: vec3f, dark: f32) -> vec3f {
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
	return mix(c, fog, clamp((dark - 0.7) / 0.3, 0.0, 1.0) * 0.6);
}

// moving is the moving lights' colour at p (blobber.go's light, per pixel)
fn moving(p: vec3f) -> vec3f {
	var c = vec3f(0.0);
	for (var i = 0; i < 4; i++) {
		let l = u.lights[2 * i];
		if (l.w <= 0.0) {
			break;
		}
		let dv = p - l.xyz;
		let d = dot(dv, dv);
		c += l.w / (1.0 + 0.5 * d * d / l.w) * u.lights[2 * i + 1].xyz;
	}
	return min(c, vec3f(1.0));
}

// texel is where uv samples the picture at r in its atlas (dims the atlas's size): tiled, or clamped to
// the picture's texel centres where Clamp says the picture ends (the interpolated uv reaches a little
// past 1 on the last row of pixels, and tiled that is the first row of texels: a transparent crack)
fn texel(uv: vec2f, r: vec4f, dims: vec2u) -> vec2f {
	var f = fract(uv);
	if (u.warm.w >= 2.0) {
		f.x = clamp(uv.x, 0.0, 1.0);
	}
	if (u.warm.w >= 1.0) {
		f.y = clamp(uv.y, 0.0, 1.0);
	}
	let h = 0.5 / vec2f(dims);
	return clamp(r.xy + f * r.zw, r.xy + h, r.xy + r.zw - h);
}

@fragment
fn fs(i: VOut) -> @location(0) vec4f {
	let c = textureSampleLevel(src0, samp, texel(i.uv, u.rect[0], textureDimensions(src0)), 0.0);
	if (u.eye.w == 2.0) {
		return c * i.color;
	}
	if (c.a == 0.0) {
		discard;
	}
	let rel = i.world - u.eye.xyz;
	if ((u.face.x != 0.0 || u.face.y != 0.0) && dot(u.face.xy, rel.xz) >= 0.0) {
		discard; // a thin wall seen from behind: the cell beyond draws its own
	}
	if (u.eye.w == 1.0) {
		return vec4f(haze(c.rgb / c.a, rel) * c.a, c.a);
	}
	let rgb = c.rgb / c.a;
	let q = floor(i.pos.xy / u.torch.w);
	let chk = ((q.x + q.y) % 2.0) * u.torch.z;
	var lit = 0.0;
	if (u.face.z > 0.0) {
		lit = textureSampleLevel(src2, samp, texel(i.uv, u.rect[2], textureDimensions(src2)), 0.0).r;
	}
	let col = moving(i.world);
	let near = clamp(max(col.r, max(col.g, col.b)), 0.0, 1.0);
	let dark = (1.0 - torch(length(rel), chk)) * (1.0 - 0.7 * lit) * (1.0 - near);
	let o = shade(rgb * mix(u.warm.xyz, vec3f(1.0), max(lit, near)), dark) + rgb * col * 0.3;
	return vec4f(haze(o, rel) * c.a, c.a);
}
`

// Depth layers of the faces nothing hides behind (the painter's passes 0 to 2), back to front.
const (
	layerSky    = -0.5 // the clouds, the sea to the horizon
	layerFloor  = -1
	layerWater  = -2
	layerMud    = -3 // the banks of ponds, the hole of a well
	layerPond   = -4 // and a well's rim
	layerCircle = -5 // light circles
)

// Painter's biases: a face sorts this much nearer than its centre. Side walls none; front walls,
// billboards and glows alike, so they keep their true order; monsters a little more, so they
// stand over the side walls of their own cell and win a tie against the wall ahead.
const biasFront, biasMonster = 0.49, 0.51

const far = 5000.0 // cells; the projection's depth = projA + projB / z runs 0 at near to 1 at far

var projA, projB = float32(far / (far - near)), float32(-near * far / (far - near))

type pt struct{ x, y, z, u, v float64 }

var white4 = [4]float32{1, 1, 1, 1}

func mv(p pt, ctr [3]float32, col [4]float32) gpu.MeshVertex {
	return gpu.MeshVertex{X: float32(p.x), Y: float32(p.y), Z: float32(p.z), U: float32(p.u), V: float32(p.v),
		NX: ctr[0], NY: ctr[1], NZ: ctr[2], R: col[0], G: col[1], B: col[2], A: col[3]}
}

// quad appends the two triangles of a quad.
func quad(vs []gpu.MeshVertex, a, b, c, d pt, ctr [3]float32, col [4]float32) []gpu.MeshVertex {
	return append(vs, mv(a, ctr, col), mv(b, ctr, col), mv(c, ctr, col), mv(a, ctr, col), mv(c, ctr, col), mv(d, ctr, col))
}

func centre(x, z float64) [3]float32 { return [3]float32{float32(x), 0, float32(z)} }

// ahead is a quad of the plane at height y that fills the view from just before the eye to
// reach cells ahead, whatever the roll; its uv is the world position times k. It lies wholly
// in front of the camera: with the painter's depth nothing is clipped at the near plane, and a
// plane centred under the eye (a triangle edge passing beneath it, two corners 4000 cells
// behind) lost its near half for a frame at one facing on Windows 7's AMD.
func ahead(c cam, y, reach, k float64) (a, b, cc, d pt) {
	const near = 0.05
	h := 1.2 * math.Hypot(float64(W), float64(H)) / (2 * focal) // half width per cell ahead, with a margin
	p := func(t, s float64) pt {
		x, z := c.x+c.fwx*t+c.rtx*s, c.z+c.fwz*t+c.rtz*s
		return pt{x, y, z, x * k, z * k}
	}
	return p(near, -near*h), p(near, near*h), p(reach, reach*h), p(reach, -reach*h)
}
func layer(l float32) [3]float32 { return [3]float32{0, l, 0} }

// world is the map's standing geometry on the GPU.
type world struct {
	gen     int
	mesh    *gpu.Mesh
	walls   []wallGroup
	floor   [2]int // first vertex, count
	ceil    [2]int
	water   [2]int
	pond    [2]int
	mud     []texRange
	torches []torch // the glows in front of light-mapped walls
}

type wallGroup struct {
	m     *mat
	side  int
	faces [][4]pt // every face of the map with this material on this side; those in view go into the frame's mesh
}

// wallRange is one group's faces in view this frame: a slice of the frame's mesh.
type wallRange struct{ g, first, n int }

type texRange struct {
	tex      *gpu.Image
	first, n int
}

type torch struct {
	x, y, z, w float64
	col        [3]float64
	phase      float64
	cell       [2]int
}

// worldGen counts the changes to the map, its faces, flats or textures; the world is rebuilt when it moved.
var worldGen int

func worldChanged() { worldGen++ }

// sides of a floor cell, from its corner x0 z0 to x1 z1: a to b as seen from inside, and the normal (into the cell)
type side struct{ ax, az, bx, bz, nx, nz float64 }

func sides(x0, z0, x1, z1 float64) [4]side {
	return [4]side{{x0, z0, x1, z0, 0, 1}, {x1, z0, x1, z1, -1, 0}, {x1, z1, x0, z1, 0, -1}, {x0, z1, x0, z0, 1, 0}}
}

func buildWorld() *world {
	w := &world{gen: worldGen}
	var vs []gpu.MeshVertex
	h, wd := len(level), 0
	for _, r := range level {
		wd = max(wd, len(r))
	}
	m := 0 // a dungeon's rays run this far off the map through a gap in its edge; outdoors the sea plane is beyond it
	if !openSky {
		m = 2 * radius
	}
	// the floor and ceiling: a quad per cell, so their edges share the walls' base vertices exactly (one
	// quad over the map left cracks a pixel wide along the walls); off the map one quad under it all
	flat := func(y float64, l float32) [2]int {
		first := len(vs)
		for cz := 0; cz < h; cz++ {
			for cx := 0; cx < wd; cx++ {
				x0, z0 := float64(cx), float64(cz)
				x1, z1 := x0+1, z0+1
				vs = quad(vs, pt{x0, y, z0, x0, z0}, pt{x1, y, z0, x1, z0}, pt{x1, y, z1, x1, z1}, pt{x0, y, z1, x0, z1}, layer(l), white4)
			}
		}
		if m > 0 {
			x0, z0, x1, z1 := float64(-m), float64(-m), float64(wd+m), float64(h+m)
			vs = quad(vs, pt{x0, y, z0, x0, z0}, pt{x1, y, z0, x1, z0}, pt{x1, y, z1, x1, z1}, pt{x0, y, z1, x0, z1}, layer(layerSky), white4)
		}
		return [2]int{first, len(vs) - first}
	}
	w.floor = flat(0.5, layerFloor)
	if !openSky {
		w.ceil = flat(-0.5, layerFloor)
	}
	// water: the water cells, and off the map when the map has water or a sea
	first := len(vs)
	wq := func(cx, cz int) {
		x0, z0 := float64(cx), float64(cz)
		x1, z1 := x0+1, z0+1
		vs = quad(vs, pt{x0, 0.5, z0, x0 / 2, z0 / 2}, pt{x1, 0.5, z0, x1 / 2, z0 / 2}, pt{x1, 0.5, z1, x1 / 2, z1 / 2}, pt{x0, 0.5, z1, x0 / 2, z1 / 2}, layer(layerWater), white4)
	}
	for cell := range Water {
		wq(cell[0], cell[1])
	}
	if !openSky && (len(Water) > 0 || Sea) {
		for cz := -m; cz < h+m; cz++ {
			for cx := -m; cx < wd+m; cx++ {
				if !inMap(cx, cz) {
					wq(cx, cz)
				}
			}
		}
	}
	w.water = [2]int{first, len(vs) - first}
	// the banks of the ponds, by tile
	muds := map[*gpu.Image][][2]int{}
	var mudOrder []*gpu.Image
	for cz := -1; cz <= h; cz++ {
		for cx := -1; cx <= wd; cx++ {
			if t := pondMud([2]int{cx, cz}); t != nil {
				if muds[t] == nil {
					mudOrder = append(mudOrder, t)
				}
				muds[t] = append(muds[t], [2]int{cx, cz})
			}
		}
	}
	for _, t := range mudOrder {
		first := len(vs)
		for _, cell := range muds[t] {
			x0, z0 := float64(cell[0]), float64(cell[1])
			x1, z1 := x0+1, z0+1
			vs = quad(vs, pt{x0, 0.5, z0, 0, 0}, pt{x1, 0.5, z0, 1, 0}, pt{x1, 0.5, z1, 1, 1}, pt{x0, 0.5, z1, 0, 1}, layer(layerMud), white4)
		}
		w.mud = append(w.mud, texRange{t, first, len(vs) - first})
	}
	// the ponds: a pool per cell (pondH), joined to a neighbouring pond cell by a neck
	first = len(vs)
	for cell := range Pond {
		const n = 48
		wv := func(x, z float64) pt { return pt{x, 0.5, z, x / 2, z / 2} }
		mx, mz := float64(cell[0])+0.5, float64(cell[1])+0.5
		for i := 0; i < n; i++ {
			ax, az := pondEdge(2 * math.Pi * float64(i) / n)
			bx, bz := pondEdge(2 * math.Pi * float64(i+1) / n)
			vs = append(vs, mv(wv(mx, mz), layer(layerPond), white4), mv(wv(mx+ax, mz+az), layer(layerPond), white4), mv(wv(mx+bx, mz+bz), layer(layerPond), white4))
		}
		for _, d := range [][2]int{{1, 0}, {0, 1}} { // each join once, from the west / north cell
			if Pond[[2]int{cell[0] + d[0], cell[1] + d[1]}] {
				ex, ez := float64(d[0]), float64(d[1])
				px, pz := ez*pondNeck, ex*pondNeck
				vs = quad(vs, wv(mx-px, mz-pz), wv(mx+px, mz+pz), wv(mx+px+ex, mz+pz+ez), wv(mx-px+ex, mz-pz+ez), layer(layerPond), white4)
			}
		}
	}
	w.pond = [2]int{first, len(vs) - first}
	// the walls, grouped by material and side (the texture is picked per side as the camera turns)
	type key struct {
		m    *mat
		side int
	}
	faces := map[key][][4]pt{}
	var order []key
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < len(level[cy]); cx++ {
			if wall(cx, cy) {
				continue
			}
			for i, s := range sides(float64(cx), float64(cy), float64(cx+1), float64(cy+1)) {
				parts := Faces[[3]int{cx, cy, i}] // phlan-poc: a face of its own, also in front of a passable cell
				if parts == nil {
					if !wall(cx+dirs[i][0], cy+dirs[i][1]) || !inMap(cx+dirs[i][0], cy+dirs[i][1]) { // phlan-poc: no wall around the map
						continue
					}
					parts = []FacePart{{Mat: 0, T1: 1, U1: 1}}
				}
				var m *mat
				for _, p := range parts {
					m = mats[p.Mat]
					if p.Tex != "" {
						m = texMat(p.Tex)
					}
					if m == nil {
						m = material(cx+dirs[i][0], cy+dirs[i][1])
					}
					m = m.load()
					ax, az := s.ax+(s.bx-s.ax)*p.T0, s.az+(s.bz-s.az)*p.T0
					bx, bz := s.ax+(s.bx-s.ax)*p.T1, s.az+(s.bz-s.az)*p.T1
					k := key{m, i}
					if faces[k] == nil {
						order = append(order, k)
					}
					faces[k] = append(faces[k], [4]pt{{ax, wallTop, az, p.U0, 0}, {bx, wallTop, bz, p.U1, 0}, {bx, 0.5, bz, p.U1, 1}, {ax, 0.5, az, p.U0, 1}})
				}
				if m.lightPow > 0 { // a flickering glow in front of a light-mapped face
					d := dirs[i]
					k := 0.5 + 0.1*m.lightPow
					wx, wz := float64(cx+d[0])+0.5, float64(cy+d[1])+0.5
					w.torches = append(w.torches, torch{wx - float64(d[0])*k, 0.5 - m.lightHei, wz - float64(d[1])*k, 1.5 * m.lightPow, m.lightCol, float64(cx*7 + cy*13 + i), [2]int{cx, cy}})
				}
			}
		}
	}
	for _, k := range order { // the walls go into the frame's mesh, those in view only (draw3D)
		w.walls = append(w.walls, wallGroup{k.m, k.side, faces[k]})
	}
	w.mesh = gpu.NewMesh(vs)
	return w
}

// viewProj is the camera's matrix: world to camera space (x right, y down, z forward), the roll,
// then the projection (focal, W, H), to WebGPU clip space.
func viewProj(c cam) gpu.Mat4 {
	cr, sr := math.Cos(c.roll), math.Sin(c.roll)
	sx, sy := 2*focal/float64(W), 2*focal/float64(H)
	v := [4][4]float64{{c.rtx, 0, c.rtz, -(c.rtx*c.x + c.rtz*c.z)}, {0, 1, 0, 0}, {c.fwx, 0, c.fwz, -(c.fwx*c.x + c.fwz*c.z)}, {0, 0, 0, 1}}
	r := [4][4]float64{{cr, -sr, 0, 0}, {sr, cr, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}}
	p := [4][4]float64{{sx, 0, 0, 0}, {0, -sy, 0, 0}, {0, 0, float64(projA), float64(projB)}, {0, 0, 1, 0}}
	m := mul(p, mul(r, v))
	var out gpu.Mat4
	for col := 0; col < 4; col++ {
		for row := 0; row < 4; row++ {
			out[col*4+row] = float32(m[row][col])
		}
	}
	return out
}

func mul(a, b [4][4]float64) (c [4][4]float64) {
	for i := range 4 {
		for j := range 4 {
			for k := range 4 {
				c[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return c
}

// Offsets into the frame's uniforms (meshSrc's U after rect) of what changes per draw.
const (
	uMode  = 16 + 3
	uClamp = 32 + 3
	uFace  = 36
	uUVOff = 40
	uLight = 44
)

// frameUniforms fills g.uni with the frame's share of the shader's uniforms: the camera, the torch,
// the haze and the four nearest moving lights.
func (g *Game) frameUniforms(c cam, vp gpu.Mat4) {
	u := append(g.uni[:0], vp[:]...)
	u = append(u, float32(c.x), 0, float32(c.z), 0)
	u = append(u, float32(c.fwx), float32(c.fwz), projA, projB)
	u = append(u, float32(g.torchPow()), float32(g.torchRange()), float32(btof(g.dither)), float32(math.Max(1, float64(W)/500)))
	hc, haze := hazeCol(), float32(0)
	if openSky {
		haze = hazeCells
	}
	u = append(u, hc[0], hc[1], hc[2], haze)
	warm := []float32{1.08, 0.94, 0.78}
	if openSky {
		warm = []float32{1, 1, 1}
		if Night { // phlan-poc: moonlight, dim and blue
			warm = nightTint
		}
	}
	u = append(u, warm[0], warm[1], warm[2], 0)
	u = append(u, 0, 0, 0, 0, 0, 0, 0, 0) // face, uvOff: per draw
	g.lights = g.lights[:0]
	for _, e := range ents {
		if e.light > 0 {
			g.lights = append(g.lights, e)
		}
	}
	if len(g.lights) > 4 {
		d := func(e *ent) float64 { return (e.x-c.x)*(e.x-c.x) + (e.z-c.z)*(e.z-c.z) }
		sort.Slice(g.lights, func(i, j int) bool { return d(g.lights[i]) < d(g.lights[j]) })
	}
	for i := 0; i < 4; i++ {
		if i < len(g.lights) {
			e := g.lights[i]
			u = append(u, float32(e.x), float32(e.y), float32(e.z), float32(e.light), float32(e.lr), float32(e.lg), float32(e.lb), 0)
		} else {
			u = append(u, 0, 0, 0, 0, 0, 0, 0, 0)
		}
	}
	g.uni = u
}

// bill is one dynamic draw: a billboard, a glow, a well's rim.
type bill struct {
	first, n int
	tex      *gpu.Image
	bias     float64
	mode     int
}

// glowSrc is a light's glow before it is laid out: w wide at x y z, tinted col at alpha, darkened
// by distance dm times; flat lays it on the floor or ceiling (en_LightGlow).
type glowSrc struct {
	x, y, z, w float64
	col        [3]float64
	alpha, dm  float64
	flat       bool
}

// draw3D draws the scene from camera c: the 3D view alone, at the size W, H.
func (g *Game) draw3D(screen *gpu.Image, c cam) {
	dst := screen
	if g.dither {
		dst = g.scene
	}
	if !gpu.Skip("fill") {
		dst.Fill(fogCol)
	}
	if texWater == nil {
		texWater, texClouds = FlatTexture("floor", "phlan_water"), FlatTexture("ceiling", "phlan_clouds")
		texNightSky, texStars = FlatTexture("ceiling", "sky_night"), FlatTexture("ceiling", "night_stars")
	}
	if openSky && !gpu.Skip("sky") {
		g.drawSky(dst, c)
		if texWater != nil && (len(Water) > 0 || Sea) { // phlan-poc: the sea runs on to the horizon past the last cells drawn
			vector.DrawFilledRect(dst, 0, float32(H)/2, float32(W), float32(H)/2, color.RGBA{40, 98, 160, 255}, false)
		}
	}
	if g.world == nil || g.world.gen != worldGen {
		if g.world != nil {
			g.world.mesh.Dispose()
		}
		if texPondWater == nil && len(Pond) > 0 {
			b := texWater.Bounds()
			texPondWater = gpu.NewImage(b.Dx(), b.Dy())
			op := &gpu.DrawImageOptions{}
			op.ColorScale.Scale(1.25, 1.35, 1.7, 1)
			texPondWater.DrawImage(texWater, op)
		}
		g.world = buildWorld()
	}
	w := g.world
	vis := visibleCells(c)
	vp := viewProj(c)
	g.frameUniforms(c, vp)

	// ---- what moves, into the frame's buffer ----
	dyn, bills := g.dyn[:0], g.bills[:0]
	board := func(x, y, z, wid, hgt float64, tex *gpu.Image, bias float64, mode int) { // wid by hgt, its foot centred at x y z, facing us
		hx, hz := c.rtx*wid/2, c.rtz*wid/2
		first := len(dyn)
		dyn = quad(dyn, pt{x - hx, y - hgt, z - hz, 0, 0}, pt{x + hx, y - hgt, z + hz, 1, 0}, pt{x + hx, y, z + hz, 1, 1}, pt{x - hx, y, z - hz, 0, 1}, centre(x, z), white4)
		bills = append(bills, bill{first, 6, tex, bias, mode})
	}
	clouds := [2]int{}
	if openSky { // phlan-poc: a cloud layer high above, so the sky moves with every step and turn
		const hgt, span, size = -6.0, 130.0, 14.0 // height, reach and cloud tile size in cells
		a, b, cc, d := ahead(c, hgt, span, 1/size)
		clouds[0] = len(dyn)
		dyn = quad(dyn, a, b, cc, d, layer(layerSky), white4)
		clouds[1] = 6
	}
	ground := [2]int{} // under everything, out to the horizon (the sea on a coast), so no sky shows below it
	groundK := 1.0
	if openSky {
		const span = 4000.0 // the plane's far edge sits under a pixel below the horizon
		if len(Water) > 0 || Sea {
			groundK = 0.5
		}
		a, b, cc, d := ahead(c, 0.5, span, groundK)
		ground[0] = len(dyn)
		dyn = quad(dyn, a, b, cc, d, layer(layerSky), white4)
		ground[1] = 6
	}
	// the lights' glows: the torches on the walls and the moving lights, with their light circles
	glows := g.glows[:0]
	for _, t := range w.torches {
		if vis[t.cell] {
			fl := 0.8 + 0.2*math.Sin(t.phase+g.time*2.4)
			glows = append(glows, glowSrc{t.x, t.y, t.z, t.w, t.col, 0.6 * fl, 0.75, false})
		}
	}
	for _, e := range ents { // billboards, only in cells a ray reached
		if !vis[[2]int{int(math.Floor(e.x)), int(math.Floor(e.z))}] {
			continue
		}
		if e.light > 0 { // a moving light (en_FloatingLight): its glow and light circles below and above
			col := [3]float64{e.lr, e.lg, e.lb}
			if e.kind == Fireball {
				col = [3]float64{1, 0.45, 0.15}
			}
			glows = append(glows, glowSrc{e.x, e.y, e.z, 0.2 + e.size*3, col, 1, 0.85, false})
			r := math.Min(1.5, 0.6*e.light+0.3)
			glows = append(glows, glowSrc{e.x, 0.499, e.z, r, col, 0.30, 0.85, true})
			if !openSky {
				glows = append(glows, glowSrc{e.x, -0.499, e.z, r, col, 0.25, 0.85, true})
			}
			continue
		}
		board(e.x, e.y+e.size/2, e.z, e.size, e.size, e.tex, biasFront, modeShaded)
	}
	g.glows = glows
	glowQuads := func(flat bool) [2]int {
		first := len(dyn)
		for _, s := range glows {
			if s.flat != flat {
				continue
			}
			dx, dz := s.x-c.x, s.z-c.z
			lat, dep := dx*c.rtx+dz*c.rtz, dx*c.fwx+dz*c.fwz
			k := s.alpha * (1 - (1-g.torchLight(math.Sqrt(lat*lat+s.y*s.y+dep*dep)))*s.dm)
			tint := [4]float32{float32(s.col[0] * k), float32(s.col[1] * k), float32(s.col[2] * k), 0}
			r := s.w / 2
			if flat {
				dyn = quad(dyn, pt{s.x - r, s.y, s.z - r, 0, 0}, pt{s.x + r, s.y, s.z - r, 1, 0}, pt{s.x + r, s.y, s.z + r, 1, 1}, pt{s.x - r, s.y, s.z + r, 0, 1}, layer(layerCircle), tint)
			} else {
				hx, hz := c.rtx*r, c.rtz*r
				dyn = quad(dyn, pt{s.x - hx, s.y - r, s.z - hz, 0, 0}, pt{s.x + hx, s.y - r, s.z + hz, 1, 0}, pt{s.x + hx, s.y + r, s.z + hz, 1, 1}, pt{s.x - hx, s.y + r, s.z - hz, 0, 1}, centre(s.x, s.z), tint)
			}
		}
		return [2]int{first, len(dyn) - first}
	}
	circles, halos := glowQuads(true), glowQuads(false)
	for _, o := range Objects { // props and floor items: static floor-standing billboards
		if o.Img == nil {
			continue
		}
		ox, oz := float64(o.X)+0.5+c.fwx*eyeBack, float64(o.Y)+0.5+c.fwz*eyeBack
		if o.At != nil { // phlan-poc: stands where it stands, seen like a wall (no culling by cell)
			ox, oz = o.At[0], o.At[1]
		} else if !vis[[2]int{o.X, o.Y}] {
			continue
		}
		b := o.Img.Bounds()
		rx, rz, fx, fz := 1.0, 0.0, 0.0, 1.0 // a ring: axis-aligned, unless it turns with us on our square
		if o.Front > 0 && o.At != nil {
			px, pz := math.Floor(c.x+c.fwx*eyeBack), math.Floor(c.z+c.fwz*eyeBack) // the party's cell
			if ox == px+0.5 && oz == pz+0.5 {
				ox, oz, rx, rz, fx, fz = ox+c.fwx*o.Front, oz+c.fwz*o.Front, c.rtx, c.rtz, c.fwx, c.fwz
			}
		}
		if r := o.Ring; r != nil { // an oval hole: black inside, the rim's stones laid around it (tiled, not stretched)
			if texBlack == nil {
				texBlack = gpu.NewImage(4, 4)
				texBlack.Fill(color.Black)
			}
			const n = 36
			at := func(i int, ext float64) (float64, float64) {
				t := 2 * math.Pi * float64(i) / n
				a, d := (r[0]-r[2]+ext)*math.Cos(t), (r[1]-r[2]+ext)*math.Sin(t)
				return ox + rx*a + fx*d, oz + rz*a + fz*d
			}
			tile := r[2] * float64(b.Dx()) / float64(b.Dy()) // one strip's length along the rim at its aspect
			first := len(dyn)
			for i := 0; i < n; i++ {
				x0, z0 := at(i, 0)
				x1, z1 := at(i+1, 0)
				dyn = append(dyn, mv(pt{ox, 0.5, oz, 0, 0}, layer(layerMud), white4), mv(pt{x0, 0.5, z0, 0, 0}, layer(layerMud), white4), mv(pt{x1, 0.5, z1, 0, 0}, layer(layerMud), white4))
			}
			bills = append(bills, bill{first, len(dyn) - first, texBlack, 0, modeUnlit})
			first = len(dyn)
			u := 0.0
			for i := 0; i < n; i++ {
				x0, z0 := at(i, 0)
				x1, z1 := at(i+1, 0)
				X0, Z0 := at(i, r[2])
				X1, Z1 := at(i+1, r[2])
				du := math.Hypot((X1+x1-X0-x0)/2, (Z1+z1-Z0-z0)/2) / tile
				dyn = quad(dyn, pt{x0, 0.5, z0, u, 0}, pt{x1, 0.5, z1, u + du, 0}, pt{X1, 0.5, Z1, u + du, 1}, pt{X0, 0.5, Z0, u, 1}, layer(layerPond), white4)
				u += du
			}
			bills = append(bills, bill{first, len(dyn) - first, o.Img, 0, modeShaded})
			continue
		}
		k := o.Size / math.Max(float64(b.Dx()), float64(b.Dy()))
		board(ox, 0.5+o.Sink, oz, k*float64(b.Dx()), k*float64(b.Dy()), o.Img, biasFront, modeShaded)
	}
	crowd := map[[2]int]int{} // monsters per cell, so a group stands side by side
	for _, a := range actors {
		crowd[[2]int{a.x, a.y}]++
	}
	placed := map[[2]int]int{}
	for _, a := range actors { // monsters: floor-standing billboards
		cell := [2]int{a.x, a.y}
		slot := formation(crowd[cell], placed[cell])
		placed[cell]++
		ax, az := a.pos()
		if !vis[[2]int{int(math.Floor(ax)), int(math.Floor(az))}] {
			continue
		}
		if a.x == g.x && a.y == g.y { // in melee: as far in front of our centre as we stand behind it, whichever way we face
			ax, az = g.tx+c.fwx*2*eyeBack, g.tz+c.fwz*2*eyeBack
		}
		ax, az = ax-c.fwx*eyeBack, az-c.fwz*eyeBack // mirror the eye: we stand at the back of our cell, the monster at the front of its, facing us
		ax, az = ax+c.rtx*slot[0]+c.fwx*slot[1], az+c.rtz*slot[0]+c.fwz*slot[1]
		img := a.img()
		b := img.Bounds()
		size := a.sheet.Size
		if size == 0 {
			size = 0.6
		}
		k := math.Min(size/float64(b.Dy()), math.Max(1, size)/float64(b.Dx()))
		if a.sheet.density { // one pixel density for every painted monster; poses keep their own extent
			k = 1.0 / PxPerTile
		}
		board(ax, 0.5, az, k*float64(b.Dx()), k*float64(b.Dy()), img, biasMonster, modeShaded)
	}
	if sign.tex != nil {
		d := dirs[sign.dir]
		fwd, y0 := eyeBack, 0.0 // mid-cell, centred at eye height
		b := sign.tex.Bounds()
		wid, hgt := float64(b.Dx())/400, float64(b.Dy())/400 // 400 texture px per tile
		if sign.top {
			fwd, y0 = 0.48, -0.5+0.04+hgt/2 // just off the wall ahead, hanging from the top
		}
		board(float64(sign.x)+0.5+float64(d[0])*fwd, y0+hgt/2, float64(sign.y)+0.5+float64(d[1])*fwd, wid, hgt, sign.tex, biasFront, modeUnlit)
	}
	// the walls in view: the faces that face the eye (the shader discards the others, after sampling
	// them) and reach into the view, in the order they were built: their textures' soft edges blend
	// over what was drawn before them, so the order shows
	walls := g.wallRanges[:0]
	for gi, wg := range w.walls {
		s := sides(0, 0, 1, 1)[wg.side]
		first := len(dyn)
		for _, q := range wg.faces {
			mx, mz := (q[0].x+q[2].x)/2, (q[0].z+q[2].z)/2
			if s.nx*(mx-c.x)+s.nz*(mz-c.z) >= 0 || !inView(vp, q) {
				continue
			}
			dyn = quad(dyn, q[0], q[1], q[2], q[3], centre(mx, mz), white4)
		}
		if n := len(dyn) - first; n > 0 {
			walls = append(walls, wallRange{gi, first, n})
		}
	}
	g.wallRanges = walls
	g.dyn, g.bills = dyn, bills
	if g.dyn2 == nil {
		g.dyn2 = &gpu.Mesh{}
	}
	g.dyn2.Set(dyn)

	// ---- the draws, in the painter's order ----
	dst.ClearDepth()
	clampUV := 0 // U.Clamp for the draws that follow
	draw := func(m *gpu.Mesh, r [2]int, tex, lit *gpu.Image, mode int, nx, nz, bias, du float64, blend gpu.Blend) {
		if r[1] == 0 {
			return
		}
		u := g.uni
		u[uMode] = float32(mode)
		u[uClamp] = float32(clampUV)
		u[uFace], u[uFace+1], u[uFace+2], u[uFace+3] = float32(nx), float32(nz), float32(btof(lit != nil)), float32(bias)
		u[uUVOff], u[uUVOff+1] = float32(du), 0
		dst.DrawMeshShader(m.Sub(r[0], r[1]), g.meshShader, [4]*gpu.Image{tex, nil, lit, nil}, u, blend)
	}
	wind := g.time * 0.02
	// the floors before the planes beyond them: the depth test spares the shader the ground under
	// them (the planes share a depth, so the clouds keep coming before the ground); the walls after
	// both, as their textures' soft edges blend over what is behind. The planes lie beyond every
	// face's depth (0.9999 at most), so writing theirs would hide nothing: tested only, which lets
	// the GPU test before it shades (a shader that discards puts the test after a write)
	if !gpu.Skip("floors") {
		draw(w.mesh, w.floor, texFloor, nil, modeShaded, 0, 0, 0, 0, gpu.BlendSourceOver)
		draw(w.mesh, w.ceil, texCeil, nil, modeShaded, 0, 0, 0, 0, gpu.BlendSourceOver)
		draw(w.mesh, w.water, texWater, nil, modeShaded, 0, 0, 0, wind, gpu.BlendSourceOver)
		for _, r := range w.mud {
			draw(w.mesh, [2]int{r.first, r.n}, r.tex, nil, modeShaded, 0, 0, 0, 0, gpu.BlendSourceOver)
		}
		draw(w.mesh, w.pond, texPondWater, nil, modeShaded, 0, 0, 0, wind, gpu.BlendSourceOver)
	}
	if openSky && !gpu.Skip("planes") {
		ct := texClouds
		if Night {
			ct = texStars
		}
		back := gpu.BlendSourceOverBack
		if gpu.Skip("back") {
			back = gpu.BlendSourceOver
		}
		draw(g.dyn2, clouds, ct, nil, modeUnlit, 0, 0, 0, g.time*0.03, back)
		gt, gw := texFloor, 0.0
		if groundK < 1 {
			gt, gw = texWater, wind
		}
		draw(g.dyn2, ground, gt, nil, modeShaded, 0, 0, 0, gw, back)
	}
	clampUV = 1 // tiled along, not up
	for _, wr := range g.wallRanges {
		if gpu.Skip("walls") {
			break
		}
		wg := w.walls[wr.g]
		s := sides(0, 0, 1, 1)[wg.side]
		nx := s.nx*c.rtx + s.nz*c.rtz // the normal's lateral component in camera space: the lit variant, and front faces' bias
		bias := 0.0
		if math.Abs(nx) <= 0.5 {
			bias = biasFront
		}
		draw(g.dyn2, [2]int{wr.first, wr.n}, wg.m.face(nx), wg.m.lit, modeShaded, s.nx, s.nz, bias, 0, gpu.BlendSourceOver)
	}
	draw(g.dyn2, circles, texGlow, nil, modeGlow, 0, 0, 0, 0, gpu.BlendLighter)
	clampUV = 2 // a picture
	if gpu.Skip("bills") {
		bills = nil
	}
	for _, b := range bills {
		draw(g.dyn2, [2]int{b.first, b.n}, b.tex, nil, b.mode, 0, 0, b.bias, 0, gpu.BlendSourceOver)
	}
	draw(g.dyn2, halos, texGlow, nil, modeGlow, 0, 0, biasFront, 0, gpu.BlendLighter)
	if g.dither && !gpu.Skip("dither") {
		op := &gpu.DrawRectShaderOptions{}
		op.Images[0] = g.scene
		screen.DrawRectShader(W, H, g.shader, op)
	}
}

// inView reports whether a face can reach the view: a quad with all four corners beyond the same
// clip plane draws nothing. The painter's depth never clips at the near plane, but behind the eye
// w < 0 turns it negative and the z >= 0 plane clips; a hair of slack keeps rounding from culling
// a face on the edge.
func inView(vp gpu.Mat4, q [4]pt) bool {
	var out [5]int
	for _, p := range q {
		x := float64(vp[0])*p.x + float64(vp[4])*p.y + float64(vp[8])*p.z + float64(vp[12])
		y := float64(vp[1])*p.x + float64(vp[5])*p.y + float64(vp[9])*p.z + float64(vp[13])
		w := float64(vp[3])*p.x + float64(vp[7])*p.y + float64(vp[11])*p.z + float64(vp[15])
		m := math.Abs(w)*1e-5 + 1e-9
		if w < -m {
			out[4]++
		}
		if x < -w-m {
			out[0]++
		}
		if x > w+m {
			out[1]++
		}
		if y < -w-m {
			out[2]++
		}
		if y > w+m {
			out[3]++
		}
	}
	for _, n := range out {
		if n == 4 {
			return false
		}
	}
	return true
}
