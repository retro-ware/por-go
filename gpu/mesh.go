package gpu

import (
	"fmt"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/naga"
	"github.com/gogpu/wgpu"
)

// MeshVertex is one vertex of a 3D mesh. U, V run 0..1 over the texture
// image; the colour multiplies the texture.
type MeshVertex struct {
	X, Y, Z    float32
	U, V       float32
	NX, NY, NZ float32
	R, G, B, A float32
}

// Mesh is a triangle list on the GPU.
type Mesh struct {
	buf      *wgpu.Buffer
	off      uint64 // where in buf the vertices start (Set: a slice of the frame's stream)
	n, first int    // the vertices drawn
}

// NewMesh uploads a triangle list.
func NewMesh(vs []MeshVertex) *Mesh {
	data := bytesOf(vs)
	b, err := dev.device.CreateBuffer(&wgpu.BufferDescriptor{Label: "mesh", Size: uint64(max(len(data), 4)),
		Usage: wgpu.BufferUsageVertex | wgpu.BufferUsageCopyDst})
	if err != nil {
		panic(err)
	}
	if len(data) > 0 {
		Stats.Writes++
		if err := dev.queue.WriteBuffer(b, 0, data); err != nil {
			panic(err)
		}
	}
	return &Mesh{buf: b, n: len(vs)}
}

// Dispose frees the mesh.
func (m *Mesh) Dispose() {
	submit()
	m.buf.Release()
}

// Mat4 is a column-major 4x4 matrix with WebGPU's clip space (z 0..1).
type Mat4 [16]float32

// Ident is the identity matrix.
var Ident = Mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}

// MeshBlend is how a mesh meets what is drawn: opaque (alpha-tested,
// lit), or blended unlit and without writing depth.
type MeshBlend int

const (
	MeshOpaque MeshBlend = iota
	MeshAlpha            // over what is there (shadows)
	MeshAdd              // added to it (glows, sparks)
)

// MeshOptions are DrawMesh's options.
type MeshOptions struct {
	Model      Mat4       // zero means Ident
	Light      [3]float32 // direction toward the sun
	Ambient    float32    // the sky's share of the light
	Tint       [4]float32 // zero means white
	Eye        [3]float32 // the camera, for highlights
	Spec       float32    // highlight strength (0: matte)
	Point      [4]float32 // a point light: position, radius (0: none)
	PointColor [4]float32 // its colour, intensity
	Blend      MeshBlend
}

// ClearDepth starts the image's depth buffer afresh for the next meshes.
func (i *Image) ClearDepth() {
	flushBatch()
	if dev.on == i.t {
		endPass()
	}
	i.t.depthView()
	i.t.clearDepth = true
}

// DrawMesh draws m textured with tex, depth tested, into the image's
// rectangle (the viewport).
func (i *Image) DrawMesh(m *Mesh, tex *Image, viewProj Mat4, op *MeshOptions) {
	if m.n == 0 || i.r.Empty() {
		return
	}
	if op == nil {
		op = &MeshOptions{}
	}
	flushBatch()
	model, tint := op.Model, op.Tint
	if model == (Mat4{}) {
		model = Ident
	}
	if tint == ([4]float32{}) {
		tint = [4]float32{1, 1, 1, 1}
	}
	tw, th := float32(tex.t.w), float32(tex.t.h)
	b := appendF32(nil, viewProj[:]...)
	b = appendF32(b, model[:]...)
	b = appendF32(b, op.Light[0], op.Light[1], op.Light[2], op.Ambient)
	b = appendF32(b, tint[:]...)
	b = appendF32(b, float32(tex.r.Min.X)/tw, float32(tex.r.Min.Y)/th, float32(tex.r.Dx())/tw, float32(tex.r.Dy())/th)
	b = appendF32(b, op.Eye[0], op.Eye[1], op.Eye[2], op.Spec)
	b = appendF32(b, op.Point[:]...)
	b = appendF32(b, op.PointColor[:]...)
	p := passOn(i.t, true)
	if op.Blend == MeshOpaque {
		p.SetPipeline(pipes.mesh)
	} else {
		p.SetPipeline(pipes.meshBlend[op.Blend])
	}
	p.SetViewport(gputypes.Viewport{X: float32(i.r.Min.X), Y: float32(i.r.Min.Y), Width: float32(i.r.Dx()), Height: float32(i.r.Dy()), MaxDepth: 1})
	p.SetScissorRect(gputypes.ScissorRect{X: uint32(i.r.Min.X), Y: uint32(i.r.Min.Y), Width: uint32(i.r.Dx()), Height: uint32(i.r.Dy())})
	bindUniforms(p, b)
	p.SetBindGroup(1, tex.t.group(dev.nearest), nil)
	p.SetVertexBuffer(0, m.buf, m.off)
	Stats.Draws++
	p.Draw(gputypes.DrawArgs{VertexCount: uint32(m.n), InstanceCount: 1, FirstVertex: uint32(m.first)})
	// later 2D draws in this pass get the full viewport back
	p.SetViewport(gputypes.Viewport{Width: float32(i.t.w), Height: float32(i.t.h), MaxDepth: 1})
}

// Set replaces the mesh's triangles with this frame's (a mesh rebuilt every
// frame); they live in the frame's vertex stream, so Set each frame before drawing.
func (m *Mesh) Set(vs []MeshVertex) {
	data := bytesOf(vs)
	m.n, m.first = len(vs), 0
	if len(data) == 0 {
		return
	}
	c, off := dev.vertices.alloc(data, 0)
	m.buf, m.off = c.buf, uint64(off)
}

// Sub is the part of m from vertex first, n vertices long, sharing its buffer.
func (m *Mesh) Sub(first, n int) *Mesh { return &Mesh{buf: m.buf, off: m.off, n: n, first: first} }

// MeshShader is a WGSL vertex and fragment shader of its own for meshes:
// vs(VIn) -> VOut and fs(VOut) over the MeshVertex layout, with
//
//	@group(0) @binding(0) var<uniform> u: U;   // U begins with rect: array<vec4f, 4>, the images' places in their atlas (origin, size, 0..1)
//	@group(1) @binding(0..3) var src0..src3: texture_2d<f32>; @group(1) @binding(4) var samp: sampler;
//
// Depth tested; the blends write depth for BlendSourceOver and only test it otherwise.
type MeshShader struct {
	pipes  [blendCount]*wgpu.RenderPipeline
	src    string
	groups map[[4]*texture]*wgpu.BindGroup
}

// NewMeshShader checks src (see MeshShader).
func NewMeshShader(src string) (*MeshShader, error) {
	s := &MeshShader{src: src + meshShaderPrelude, groups: map[[4]*texture]*wgpu.BindGroup{}}
	if _, err := naga.Compile(s.src); err != nil {
		return nil, fmt.Errorf("gpu: mesh shader: %w", err)
	}
	return s, nil
}

const meshShaderPrelude = `
@group(1) @binding(0) var src0: texture_2d<f32>;
@group(1) @binding(1) var src1: texture_2d<f32>;
@group(1) @binding(2) var src2: texture_2d<f32>;
@group(1) @binding(3) var src3: texture_2d<f32>;
@group(1) @binding(4) var samp: sampler;
@group(0) @binding(0) var<uniform> u: U;
`

func (s *MeshShader) pipe(b Blend) *wgpu.RenderPipeline {
	if s.pipes[b] != nil {
		return s.pipes[b]
	}
	var err error
	if s.pipes[b], err = newPipeline(pipeSpec{"mesh shader", s.src, []*wgpu.BindGroupLayout{dev.uniLayout, imagesGroupLayout()},
		meshLayout, blends[b], gputypes.TextureFormatRGBA8Unorm, true, "", b != BlendSourceOver}); err != nil {
		panic(err)
	}
	return s.pipes[b]
}

// DrawMeshShader draws m with s: the images bound as src0..3 (nil: white),
// uniforms laid out as s's U after the images' rects, blended by blend.
func (i *Image) DrawMeshShader(m *Mesh, s *MeshShader, imgs [4]*Image, uniforms []float32, blend Blend) {
	if m.n == 0 || i.r.Empty() {
		return
	}
	flushBatch()
	var ts [4]*texture
	var b []byte
	for j, im := range imgs {
		if im == nil {
			im = white()
		}
		ts[j] = im.t
		tw, th := float32(im.t.w), float32(im.t.h)
		b = appendF32(b, float32(im.r.Min.X)/tw, float32(im.r.Min.Y)/th, float32(im.r.Dx())/tw, float32(im.r.Dy())/th)
	}
	b = appendF32(b, uniforms...)
	p := passOn(i.t, true)
	p.SetPipeline(s.pipe(blend))
	p.SetViewport(gputypes.Viewport{X: float32(i.r.Min.X), Y: float32(i.r.Min.Y), Width: float32(i.r.Dx()), Height: float32(i.r.Dy()), MaxDepth: 1})
	p.SetScissorRect(gputypes.ScissorRect{X: uint32(i.r.Min.X), Y: uint32(i.r.Min.Y), Width: uint32(i.r.Dx()), Height: uint32(i.r.Dy())})
	bindUniforms(p, b)
	p.SetBindGroup(1, (&Shader{groups: s.groups}).group(ts), nil)
	p.SetVertexBuffer(0, m.buf, m.off)
	Stats.Draws++
	p.Draw(gputypes.DrawArgs{VertexCount: uint32(m.n), InstanceCount: 1, FirstVertex: uint32(m.first)})
	p.SetViewport(gputypes.Viewport{Width: float32(i.t.w), Height: float32(i.t.h), MaxDepth: 1})
}
