package gpu

import (
	"github.com/gogpu/gputypes"
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
	buf *wgpu.Buffer
	n   int
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
		if err := dev.queue.WriteBuffer(b, 0, data); err != nil {
			panic(err)
		}
	}
	return &Mesh{b, len(vs)}
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
	p.SetVertexBuffer(0, m.buf, 0)
	p.Draw(gputypes.DrawArgs{VertexCount: uint32(m.n), InstanceCount: 1})
	// later 2D draws in this pass get the full viewport back
	p.SetViewport(gputypes.Viewport{Width: float32(i.t.w), Height: float32(i.t.h), MaxDepth: 1})
}
