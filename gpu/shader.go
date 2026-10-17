package gpu

import (
	"fmt"
	"image"
	"regexp"
	"strings"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/naga"
	"github.com/gogpu/wgpu"
)

// Shader is a WGSL fragment shader in the role of a Kage one (pixel units).
// The source declares
//
//	struct Uniforms { Name: f32, Other: vec3f, ... }
//	fn Fragment(dstPos: vec4f, srcPos: vec2f, color: vec4f, custom: vec4f) -> vec4f
//
// and reads its uniforms as u.Name. It may call imageSrcNAt(pos) (N 0..3,
// pos in source pixels), imageSrc0Origin(), imageSrc0Size() and
// imageDstOrigin(), as Kage has them.
type Shader struct {
	fields []field
	pipes  [blendCount]*wgpu.RenderPipeline
	src    string
	groups map[[4]*texture]*wgpu.BindGroup
}

type field struct {
	name        string
	size, align int // bytes
}

var (
	structRe = regexp.MustCompile(`(?s)struct\s+Uniforms\s*\{(.*?)\}`)
	fieldRe  = regexp.MustCompile(`(\w+)\s*:\s*(f32|vec2f|vec3f|vec4f)`)
)

// wgslSize is the size and alignment of each uniform type.
var wgslSize = map[string][2]int{"f32": {4, 4}, "vec2f": {8, 8}, "vec3f": {12, 16}, "vec4f": {16, 16}}

var imagesLayout *wgpu.BindGroupLayout

// builtins come first in every Uniforms block.
const builtins = "_dstOrigin: vec2f, _srcOrigin: vec2f, _srcSize: vec2f, _pad: vec2f, "

const shaderPrelude = `
@group(1) @binding(0) var src0: texture_2d<f32>;
@group(1) @binding(1) var src1: texture_2d<f32>;
@group(1) @binding(2) var src2: texture_2d<f32>;
@group(1) @binding(3) var src3: texture_2d<f32>;
@group(1) @binding(4) var samp: sampler;
fn imageSrc0At(p: vec2f) -> vec4f { return textureSampleLevel(src0, samp, p / vec2f(textureDimensions(src0)), 0.0); }
fn imageSrc1At(p: vec2f) -> vec4f { return textureSampleLevel(src1, samp, p / vec2f(textureDimensions(src1)), 0.0); }
fn imageSrc2At(p: vec2f) -> vec4f { return textureSampleLevel(src2, samp, p / vec2f(textureDimensions(src2)), 0.0); }
fn imageSrc3At(p: vec2f) -> vec4f { return textureSampleLevel(src3, samp, p / vec2f(textureDimensions(src3)), 0.0); }
fn imageSrc0Origin() -> vec2f { return u._srcOrigin; }
fn imageSrc0Size() -> vec2f { return u._srcSize; }
fn imageDstOrigin() -> vec2f { return u._dstOrigin; }
@group(0) @binding(0) var<uniform> u: Uniforms;
` + vertexWGSL + `
@fragment
fn fs(i: VOut) -> @location(0) vec4f { return Fragment(i.pos, i.src, i.color, i.custom); }
`

// NewShader checks src (see Shader); it needs no window yet.
func NewShader(src []byte) (*Shader, error) {
	code := string(src)
	m := structRe.FindStringSubmatchIndex(code)
	if m == nil {
		code = "struct Uniforms {}\n" + code
		m = structRe.FindStringSubmatchIndex(code)
	}
	s := &Shader{groups: map[[4]*texture]*wgpu.BindGroup{}}
	body := builtins + code[m[2]:m[3]]
	for _, f := range fieldRe.FindAllStringSubmatch(body, -1) {
		sa := wgslSize[f[2]]
		s.fields = append(s.fields, field{f[1], sa[0], sa[1]})
	}
	s.src = code[:m[2]] + body + code[m[3]:] + shaderPrelude
	if _, err := naga.Compile(s.src); err != nil {
		return nil, fmt.Errorf("gpu: shader: %w", err)
	}
	return s, nil
}

// pipe is the shader's pipeline for blend b, made on first use.
func (s *Shader) pipe(b Blend) *wgpu.RenderPipeline {
	if s.pipes[b] != nil {
		return s.pipes[b]
	}
	var err error
	if imagesLayout == nil {
		if imagesLayout, err = dev.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{Label: "shader images",
			Entries: []gputypes.BindGroupLayoutEntry{texEntry(0), texEntry(1), texEntry(2), texEntry(3), samplerEntry(4)}}); err != nil {
			panic(err)
		}
	}
	if s.pipes[b], err = newPipeline(pipeSpec{"shader", s.src, []*wgpu.BindGroupLayout{dev.uniLayout, imagesLayout},
		vertexLayout, blends[b], gputypes.TextureFormatRGBA8Unorm, false, "", false}); err != nil {
		panic(err)
	}
	return s.pipes[b]
}

// pack lays the uniform values out as WGSL does.
func (s *Shader) pack(builtin [8]float32, vals map[string]any) []byte {
	var b []byte
	bi := 0
	for _, f := range s.fields {
		for len(b)%f.align != 0 {
			b = append(b, 0)
		}
		var fs []float32
		if strings.HasPrefix(f.name, "_") {
			fs = builtin[bi : bi+2]
			bi += 2
		} else {
			switch v := vals[f.name].(type) {
			case float32:
				fs = []float32{v}
			case float64:
				fs = []float32{float32(v)}
			case int:
				fs = []float32{float32(v)}
			case []float32:
				fs = v
			}
		}
		n := f.size / 4
		for j := range n {
			x := float32(0)
			if j < len(fs) {
				x = fs[j]
			}
			b = appendF32(b, x)
		}
	}
	return b
}

func (s *Shader) group(ts [4]*texture) *wgpu.BindGroup {
	if g := s.groups[ts]; g != nil {
		return g
	}
	es := []wgpu.BindGroupEntry{{Binding: 4, Sampler: dev.nearest}}
	for j, t := range ts {
		es = append(es, wgpu.BindGroupEntry{Binding: uint32(j), TextureView: t.view})
	}
	g, err := dev.device.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "shader images", Layout: imagesLayout, Entries: es})
	if err != nil {
		panic(err)
	}
	s.groups[ts] = g
	return g
}

// DrawTrianglesShaderOptions are DrawTrianglesShader's options.
type DrawTrianglesShaderOptions struct {
	Uniforms map[string]any
	Images   [4]*Image
	Blend    Blend
}

// DrawTrianglesShader draws indexed triangles coloured by s.
func (i *Image) DrawTrianglesShader(vs []Vertex, is []uint16, s *Shader, op *DrawTrianglesShaderOptions) {
	if op == nil {
		op = &DrawTrianglesShaderOptions{}
	}
	var ts [4]*texture
	src := image.Rectangle{}
	for j, im := range op.Images {
		if im == nil {
			im = white()
		}
		ts[j] = im.t
		if j == 0 && op.Images[0] != nil {
			src = im.r
		}
	}
	builtin := [8]float32{float32(i.r.Min.X), float32(i.r.Min.Y), float32(src.Min.X), float32(src.Min.Y), float32(src.Dx()), float32(src.Dy())}
	k := batchKey{dst: i.t, clip: i.r, pipe: s.pipe(op.Blend), group: s.group(ts), uni: string(s.pack(builtin, op.Uniforms))}
	queue(k, i, vs, is, ts[0])
}

// DrawRectShaderOptions are DrawRectShader's options.
type DrawRectShaderOptions struct {
	GeoM     GeoM
	Uniforms map[string]any
	Images   [4]*Image
	Blend    Blend
}

// DrawRectShader fills a w x h rectangle, transformed by op.GeoM, with s;
// srcPos runs over image 0's bounds.
func (i *Image) DrawRectShader(w, h int, s *Shader, op *DrawRectShaderOptions) {
	if op == nil {
		op = &DrawRectShaderOptions{}
	}
	var o image.Point
	if op.Images[0] != nil {
		o = op.Images[0].r.Min
	}
	v := func(x, y float64) Vertex {
		dx, dy := op.GeoM.Apply(x, y)
		return Vertex{DstX: float32(dx), DstY: float32(dy), SrcX: float32(o.X) + float32(x), SrcY: float32(o.Y) + float32(y),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}
	}
	fw, fh := float64(w), float64(h)
	i.DrawTrianglesShader([]Vertex{v(0, 0), v(fw, 0), v(0, fh), v(fw, fh)}, []uint16{0, 1, 2, 1, 3, 2}, s,
		&DrawTrianglesShaderOptions{Uniforms: op.Uniforms, Images: op.Images, Blend: op.Blend})
}
