package gpu

import (
	"fmt"
	"strings"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

var pipes struct {
	std  [blendCount]*wgpu.RenderPipeline // textured triangles into images
	blit *wgpu.RenderPipeline             // the screen into the window
	mesh *wgpu.RenderPipeline             // depth-tested 3D triangles
	// blended 3D triangles, depth tested but not written: MeshAlpha, MeshAdd
	meshBlend [3]*wgpu.RenderPipeline
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

// blitSRGBWGSL is blitWGSL for an sRGB window: the colours go out linear, so the
// window's encoding gives back the game's own.
var blitSRGBWGSL = strings.Replace(blitWGSL, "return vec4f(textureSample(tex, samp, i.uv).rgb, 1.0);",
	"let c = textureSample(tex, samp, i.uv).rgb; return vec4f(select(pow((c + 0.055) / 1.055, vec3f(2.4)), c / 12.92, c <= vec3f(0.04045)), 1.0);", 1)

const meshWGSL = `
struct U {
	viewProj: mat4x4f,
	model: mat4x4f,
	light: vec4f,      // direction toward the sun, ambient share
	tint: vec4f,
	uvRect: vec4f,     // the texture's part: origin, size (0..1)
	eye: vec4f,        // camera position, specular strength
	point: vec4f,      // a point light: position, radius (0: none)
	pointColor: vec4f, // its colour, intensity
}
@group(0) @binding(0) var<uniform> u: U;
@group(1) @binding(0) var tex: texture_2d<f32>;
@group(1) @binding(1) var samp: sampler;
struct VIn { @location(0) pos: vec3f, @location(1) uv: vec2f, @location(2) normal: vec3f, @location(3) color: vec4f }
struct VOut { @builtin(position) pos: vec4f, @location(0) uv: vec2f, @location(1) color: vec4f, @location(2) world: vec3f, @location(3) normal: vec3f }
@vertex
fn vs(v: VIn) -> VOut {
	var o: VOut;
	let w = u.model * vec4f(v.pos, 1.0);
	o.pos = u.viewProj * w;
	o.world = w.xyz;
	o.uv = u.uvRect.xy + v.uv * u.uvRect.zw;
	o.normal = (u.model * vec4f(v.normal, 0.0)).xyz;
	o.color = v.color * u.tint;
	return o;
}
// lit: a warm sun, a cool sky over a dim floor bounce, a point light, a highlight
fn lit(base: vec3f, i: VOut) -> vec3f {
	let n = normalize(i.normal);
	let l = normalize(u.light.xyz);
	let diff = max(dot(n, l), 0.0);
	let sky = mix(vec3f(0.42, 0.40, 0.38), vec3f(0.82, 0.88, 1.0), n.y * 0.5 + 0.5);
	var c = base * (u.light.w * sky + (1.0 - u.light.w) * diff * vec3f(1.0, 0.95, 0.86));
	if (u.point.w > 0.0) {
		let d = u.point.xyz - i.world;
		let dist = length(d);
		let att = clamp(1.0 - dist / u.point.w, 0.0, 1.0);
		c += base * u.pointColor.rgb * u.pointColor.w * att * att * (0.4 + 0.6 * max(dot(n, d / max(dist, 0.001)), 0.0));
	}
	if (u.eye.w > 0.0 && diff > 0.0) {
		let h = normalize(l + normalize(u.eye.xyz - i.world));
		c += vec3f(1.0, 0.97, 0.9) * u.eye.w * pow(max(dot(n, h), 0.0), 28.0);
	}
	return c;
}
@fragment
fn fs(i: VOut) -> @location(0) vec4f {
	let c = textureSample(tex, samp, i.uv);
	if (c.a < 0.5) { discard; }
	return vec4f(lit(c.rgb / c.a * i.color.rgb, i), 1.0);
}
// unlit and blended (shadows, glows): the texture is premultiplied
@fragment
fn fsBlend(i: VOut) -> @location(0) vec4f {
	let c = textureSample(tex, samp, i.uv);
	return vec4f(c.rgb * i.color.rgb * i.color.a, c.a * i.color.a);
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
	fs      string // fragment entry point ("fs" if empty)
	noWrite bool   // depth tested, not written
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
		depth = &wgpu.DepthStencilState{Format: gputypes.TextureFormatDepth32Float, DepthWriteEnabled: !s.noWrite, DepthCompare: gputypes.CompareFunctionLessEqual}
	}
	fs := s.fs
	if fs == "" {
		fs = "fs"
	}
	p, err := dev.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:        s.label,
		Layout:       pl,
		Vertex:       wgpu.VertexState{Module: mod, EntryPoint: "vs", Buffers: s.buffers},
		Primitive:    gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, FrontFace: gputypes.FrontFaceCCW, CullMode: gputypes.CullModeNone},
		DepthStencil: depth,
		Multisample:  gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{Module: mod, EntryPoint: fs, Targets: []gputypes.ColorTargetState{
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
		if pipes.std[b], err = newPipeline(pipeSpec{"triangles", stdWGSL, []*wgpu.BindGroupLayout{dev.texLayout}, vertexLayout, blends[b], rgba, false, "", false}); err != nil {
			return err
		}
	}
	groups := []*wgpu.BindGroupLayout{dev.uniLayout, dev.texLayout}
	if pipes.mesh, err = newPipeline(pipeSpec{"mesh", meshWGSL, groups, meshLayout, nil, rgba, true, "", false}); err != nil {
		return err
	}
	for b, bs := range map[MeshBlend]*gputypes.BlendState{MeshAlpha: blends[BlendSourceOver], MeshAdd: blends[BlendLighter]} {
		if pipes.meshBlend[b], err = newPipeline(pipeSpec{"mesh blend", meshWGSL, groups, meshLayout, bs, rgba, true, "fsBlend", true}); err != nil {
			return err
		}
	}
	return nil
}
