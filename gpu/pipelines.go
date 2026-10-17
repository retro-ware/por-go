package gpu

import (
	"fmt"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

var pipes struct {
	std  [blendCount]*wgpu.RenderPipeline // textured triangles into images
	blit *wgpu.RenderPipeline             // the screen into the window
	mesh *wgpu.RenderPipeline             // depth-tested 3D triangles
}

const vertexWGSL = `
struct VIn { @location(0) pos: vec2f, @location(1) src: vec2f, @location(2) color: vec4f, @location(3) custom: vec4f }
struct VOut { @builtin(position) pos: vec4f, @location(0) src: vec2f, @location(1) color: vec4f, @location(2) custom: vec4f }
@vertex
fn vs(v: VIn) -> VOut {
	var o: VOut;
	o.pos = vec4f(v.pos, 0.0, 1.0);
	o.src = v.src;
	o.color = v.color;
	o.custom = v.custom;
	return o;
}
`

const stdWGSL = `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
` + vertexWGSL + `
@fragment
fn fs(i: VOut) -> @location(0) vec4f {
	return textureSample(tex, samp, i.src / vec2f(textureDimensions(tex))) * i.color;
}
`

const blitWGSL = `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
struct VOut { @builtin(position) pos: vec4f, @location(0) uv: vec2f }
@vertex
fn vs(@builtin(vertex_index) id: u32) -> VOut {
	let p = vec2f(f32((id << 1u) & 2u), f32(id & 2u));
	var o: VOut;
	o.pos = vec4f(p * vec2f(2.0, -2.0) + vec2f(-1.0, 1.0), 0.0, 1.0);
	o.uv = p;
	return o;
}
@fragment
fn fs(i: VOut) -> @location(0) vec4f { return vec4f(textureSample(tex, samp, i.uv).rgb, 1.0); }
`

const meshWGSL = `
struct U {
	viewProj: mat4x4f,
	model: mat4x4f,
	light: vec4f,  // direction toward the light, ambient
	tint: vec4f,
	uvRect: vec4f, // the texture's part: origin, size (0..1)
}
@group(0) @binding(0) var<uniform> u: U;
@group(1) @binding(0) var tex: texture_2d<f32>;
@group(1) @binding(1) var samp: sampler;
struct VIn { @location(0) pos: vec3f, @location(1) uv: vec2f, @location(2) normal: vec3f, @location(3) color: vec4f }
struct VOut { @builtin(position) pos: vec4f, @location(0) uv: vec2f, @location(1) color: vec4f }
@vertex
fn vs(v: VIn) -> VOut {
	var o: VOut;
	o.pos = u.viewProj * u.model * vec4f(v.pos, 1.0);
	o.uv = u.uvRect.xy + v.uv * u.uvRect.zw;
	let n = normalize((u.model * vec4f(v.normal, 0.0)).xyz);
	let l = u.light.w + (1.0 - u.light.w) * max(dot(n, normalize(u.light.xyz)), 0.0);
	o.color = vec4f(v.color.rgb * l, v.color.a) * u.tint;
	return o;
}
@fragment
fn fs(i: VOut) -> @location(0) vec4f {
	let c = textureSample(tex, samp, i.uv);
	if (c.a < 0.5) { discard; }
	return vec4f(c.rgb / c.a, 1.0) * i.color;
}
`

var blends = [blendCount]*gputypes.BlendState{
	BlendSourceOver: {
		Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
		Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
	},
	BlendLighter: {
		Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOne, Operation: gputypes.BlendOperationAdd},
		Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOne, Operation: gputypes.BlendOperationAdd},
	},
	blendCopy: nil,
}

func attr(loc uint32, f gputypes.VertexFormat, off uint64) gputypes.VertexAttribute {
	return gputypes.VertexAttribute{Format: f, Offset: off, ShaderLocation: loc}
}

var vertexLayout = []gputypes.VertexBufferLayout{{ArrayStride: 48, StepMode: gputypes.VertexStepModeVertex,
	Attributes: []gputypes.VertexAttribute{attr(0, gputypes.VertexFormatFloat32x2, 0), attr(1, gputypes.VertexFormatFloat32x2, 8),
		attr(2, gputypes.VertexFormatFloat32x4, 16), attr(3, gputypes.VertexFormatFloat32x4, 32)}}}

var meshLayout = []gputypes.VertexBufferLayout{{ArrayStride: 48, StepMode: gputypes.VertexStepModeVertex,
	Attributes: []gputypes.VertexAttribute{attr(0, gputypes.VertexFormatFloat32x3, 0), attr(1, gputypes.VertexFormatFloat32x2, 12),
		attr(2, gputypes.VertexFormatFloat32x3, 20), attr(3, gputypes.VertexFormatFloat32x4, 32)}}}

// pipeSpec describes one pipeline.
type pipeSpec struct {
	label   string
	wgsl    string
	groups  []*wgpu.BindGroupLayout
	buffers []gputypes.VertexBufferLayout
	blend   *gputypes.BlendState
	format  gputypes.TextureFormat
	depth   bool
}

func newPipeline(s pipeSpec) (*wgpu.RenderPipeline, error) {
	mod, err := dev.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: s.label, WGSL: s.wgsl})
	if err != nil {
		return nil, fmt.Errorf("shader %s: %w", s.label, err)
	}
	pl, err := dev.device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{Label: s.label, BindGroupLayouts: s.groups})
	if err != nil {
		return nil, err
	}
	var depth *wgpu.DepthStencilState
	if s.depth {
		depth = &wgpu.DepthStencilState{Format: gputypes.TextureFormatDepth32Float, DepthWriteEnabled: true, DepthCompare: gputypes.CompareFunctionLessEqual}
	}
	p, err := dev.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:        s.label,
		Layout:       pl,
		Vertex:       wgpu.VertexState{Module: mod, EntryPoint: "vs", Buffers: s.buffers},
		Primitive:    gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, FrontFace: gputypes.FrontFaceCCW, CullMode: gputypes.CullModeNone},
		DepthStencil: depth,
		Multisample:  gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{Module: mod, EntryPoint: "fs", Targets: []gputypes.ColorTargetState{
			{Format: s.format, Blend: s.blend, WriteMask: gputypes.ColorWriteMaskAll}}},
	})
	if err != nil {
		return nil, fmt.Errorf("pipeline %s: %w", s.label, err)
	}
	return p, nil
}

func initPipelines() error {
	var err error
	rgba := gputypes.TextureFormatRGBA8Unorm
	for b := range pipes.std {
		if pipes.std[b], err = newPipeline(pipeSpec{"triangles", stdWGSL, []*wgpu.BindGroupLayout{dev.texLayout}, vertexLayout, blends[b], rgba, false}); err != nil {
			return err
		}
	}
	pipes.mesh, err = newPipeline(pipeSpec{"mesh", meshWGSL, []*wgpu.BindGroupLayout{dev.uniLayout, dev.texLayout}, meshLayout, nil, rgba, true})
	return err
}
