package gpu

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// texture is one GPU texture (RGBA8, premultiplied alpha), drawable and
// sampleable; Images are rectangles of one.
type texture struct {
	tex        *wgpu.Texture
	view       *wgpu.TextureView
	w, h       int
	groups     map[*wgpu.Sampler]*wgpu.BindGroup
	depth      *wgpu.Texture
	dview      *wgpu.TextureView
	clear      bool // the next pass into it clears it first (Clear)
	clearDepth bool
	px         []byte // its pixels until there is a device (images made before RunGame)
	gone       bool   // deallocated
}

func newTexture(w, h int) *texture {
	t := &texture{w: w, h: h} // WebGPU starts it transparent
	if dev.device != nil {
		t.make()
	}
	return t
}

// ensure gives t its GPU texture, uploading the pixels it kept until then. A texture
// used before RunGame adopts the window's device makes a headless device of its own.
func (t *texture) ensure() {
	if t.tex != nil || t.gone {
		return
	}
	t.make()
	if t.px != nil {
		px := t.px
		t.px = nil
		(&Image{t, image.Rect(0, 0, t.w, t.h)}).WritePixels(px)
	}
}

func (t *texture) make() {
	needDevice()
	tex, err := dev.device.CreateTexture(&wgpu.TextureDescriptor{
		Label: "image", Size: wgpu.Extent3D{Width: uint32(t.w), Height: uint32(t.h), DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D, Format: gputypes.TextureFormatRGBA8Unorm,
		Usage: gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst | gputypes.TextureUsageCopySrc | gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		panic(err)
	}
	if t.view, err = dev.device.CreateTextureView(tex, nil); err != nil {
		panic(err)
	}
	t.tex = tex
}

// group is the bind group sampling t with s.
func (t *texture) group(s *wgpu.Sampler) *wgpu.BindGroup {
	if g := t.groups[s]; g != nil {
		return g
	}
	if t.groups == nil {
		t.groups = map[*wgpu.Sampler]*wgpu.BindGroup{}
	}
	t.ensure()
	g, err := dev.device.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "image", Layout: dev.texLayout,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, TextureView: t.view}, {Binding: 1, Sampler: s}}})
	if err != nil {
		panic(err)
	}
	t.groups[s] = g
	return g
}

func (t *texture) depthView() *wgpu.TextureView {
	t.ensure()
	if t.depth == nil {
		var err error
		if t.depth, err = dev.device.CreateTexture(&wgpu.TextureDescriptor{
			Label: "depth", Size: wgpu.Extent3D{Width: uint32(t.w), Height: uint32(t.h), DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D,
			Format: gputypes.TextureFormatDepth32Float, Usage: gputypes.TextureUsageRenderAttachment,
		}); err != nil {
			panic(err)
		}
		if t.dview, err = dev.device.CreateTextureView(t.depth, nil); err != nil {
			panic(err)
		}
		t.clearDepth = true
	}
	return t.dview
}

func (t *texture) release() {
	t.gone, t.px = true, nil
	if t.tex == nil {
		return
	}
	flushBatch()
	if dev.on == t {
		endPass()
	}
	submit() // draws recorded with it must reach the GPU first
	for _, g := range t.groups {
		g.Release()
	}
	t.view.Release()
	t.tex.Release()
	if t.depth != nil {
		t.dview.Release()
		t.depth.Release()
	}
	t.tex = nil
}

// Image is a rectangle of a GPU texture, like ebiten.Image: a SubImage keeps
// the coordinates of the image it was cut from.
type Image struct {
	t *texture
	r image.Rectangle
}

// NewImage makes a transparent w x h image.
func NewImage(w, h int) *Image {
	return &Image{newTexture(max(w, 1), max(h, 1)), image.Rect(0, 0, w, h)}
}

// NewImageFromImage copies img to the GPU.
func NewImageFromImage(img image.Image) *Image {
	b := img.Bounds()
	rgba, ok := img.(*image.RGBA)
	if !ok || rgba.Stride != b.Dx()*4 {
		rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	}
	i := NewImage(b.Dx(), b.Dy())
	i.WritePixels(rgba.Pix)
	return i
}

// Bounds is the image's rectangle.
func (i *Image) Bounds() image.Rectangle { return i.r }

// Size is the image's width and height.
func (i *Image) Size() (int, int) { return i.r.Dx(), i.r.Dy() }

// ColorModel is premultiplied RGBA, for image.Image.
func (i *Image) ColorModel() color.Model { return color.RGBAModel }

// At reads one pixel back from the GPU (slow: for tests and rare probes).
func (i *Image) At(x, y int) color.Color {
	if !image.Pt(x, y).In(i.r) {
		return color.RGBA{}
	}
	px := make([]byte, 4)
	readTexture(i.t, x, y, 1, 1, px)
	return color.RGBA{px[0], px[1], px[2], px[3]}
}

// SubImage is the part of i inside r, sharing its pixels.
func (i *Image) SubImage(r image.Rectangle) image.Image {
	return &Image{i.t, r.Intersect(i.r)}
}

// WritePixels replaces the image's pixels with px (premultiplied RGBA,
// Dx*Dy*4 bytes).
func (i *Image) WritePixels(px []byte) {
	if i.r.Empty() {
		return
	}
	if i.t.tex == nil && dev.device == nil { // no device yet: kept until there is one
		t := i.t
		if t.px == nil {
			t.px = make([]byte, t.w*t.h*4)
		}
		w := i.r.Dx() * 4
		for y := range i.r.Dy() {
			copy(t.px[((i.r.Min.Y+y)*t.w+i.r.Min.X)*4:][:w], px[y*w:][:w])
		}
		return
	}
	i.t.ensure()
	if i.t.clear {
		passOn(i.t, false) // the pending Clear comes first
	}
	submit() // draws recorded earlier still see the old pixels
	w, h := i.r.Dx(), i.r.Dy()
	if err := dev.queue.WriteTexture(&wgpu.ImageCopyTexture{Texture: i.t.tex, Origin: wgpu.Origin3D{X: uint32(i.r.Min.X), Y: uint32(i.r.Min.Y)}},
		px[:w*h*4], &wgpu.ImageDataLayout{BytesPerRow: uint32(w * 4), RowsPerImage: uint32(h)},
		&wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}); err != nil {
		panic(err)
	}
}

// ReadPixels copies the image's pixels into px (premultiplied RGBA).
func (i *Image) ReadPixels(px []byte) {
	if !i.r.Empty() {
		readTexture(i.t, i.r.Min.X, i.r.Min.Y, i.r.Dx(), i.r.Dy(), px)
	}
}

// Deallocate frees the GPU texture; the image must not be used afterwards.
func (i *Image) Deallocate() { i.t.release() }

// Fill replaces every pixel with c.
func (i *Image) Fill(c color.Color) {
	r, g, b, a := c.RGBA()
	if i.t.tex == nil && dev.device == nil { // no device yet: filled in memory
		px := make([]byte, i.r.Dx()*i.r.Dy()*4)
		for k := 0; k < len(px); k += 4 {
			px[k], px[k+1], px[k+2], px[k+3] = byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8)
		}
		i.WritePixels(px)
		return
	}
	if i.r == image.Rect(0, 0, i.t.w, i.t.h) && a == 0 && dev.on != i.t && !pending(i.t) {
		i.t.clear = true // the next pass clears it
		return
	}
	var op DrawTrianglesOptions
	op.Blend = blendCopy
	k := float32(1) / 0xffff
	x0, y0, x1, y1 := float32(i.r.Min.X), float32(i.r.Min.Y), float32(i.r.Max.X), float32(i.r.Max.Y)
	v := func(x, y float32) Vertex {
		return Vertex{DstX: x, DstY: y, SrcX: 0.5, SrcY: 0.5, ColorR: float32(r) * k, ColorG: float32(g) * k, ColorB: float32(b) * k, ColorA: float32(a) * k}
	}
	i.DrawTriangles([]Vertex{v(x0, y0), v(x1, y0), v(x0, y1), v(x1, y1)}, []uint16{0, 1, 2, 1, 3, 2}, white(), &op)
}

// Clear makes every pixel transparent.
func (i *Image) Clear() { i.Fill(color.Transparent) }

var whiteImg *Image

// white is a 1x1 white image, the source of solid shapes.
func white() *Image {
	if whiteImg == nil {
		whiteImg = NewImage(1, 1)
		whiteImg.WritePixels([]byte{255, 255, 255, 255})
	}
	return whiteImg
}

// GeoM is a 2D affine transform, like ebiten.GeoM.
type GeoM struct {
	a_1, b, c, d_1, tx, ty float64 // a and d stored minus one, so the zero value is the identity
}

func (g *GeoM) Reset() { *g = GeoM{} }

// Apply transforms x, y.
func (g *GeoM) Apply(x, y float64) (float64, float64) {
	return (g.a_1+1)*x + g.b*y + g.tx, g.c*x + (g.d_1+1)*y + g.ty
}

// Concat applies o after g.
func (g *GeoM) Concat(o GeoM) {
	a, b, c, d := g.a_1+1, g.b, g.c, g.d_1+1
	oa, ob, oc, od := o.a_1+1, o.b, o.c, o.d_1+1
	g.a_1 = oa*a + ob*c - 1
	g.b = oa*b + ob*d
	g.c = oc*a + od*c
	g.d_1 = oc*b + od*d - 1
	g.tx, g.ty = oa*g.tx+ob*g.ty+o.tx, oc*g.tx+od*g.ty+o.ty
}

func (g *GeoM) Scale(x, y float64)     { g.Concat(GeoM{a_1: x - 1, d_1: y - 1}) }
func (g *GeoM) Translate(x, y float64) { g.tx += x; g.ty += y }

// ColorScale multiplies a draw's premultiplied colour, like ebiten.ColorScale.
type ColorScale struct{ r_1, g_1, b_1, a_1 float32 } // minus one: the zero value is no change

func (c *ColorScale) Scale(r, g, b, a float32) {
	c.r_1, c.g_1, c.b_1, c.a_1 = (c.r_1+1)*r-1, (c.g_1+1)*g-1, (c.b_1+1)*b-1, (c.a_1+1)*a-1
}
func (c *ColorScale) ScaleAlpha(a float32) { c.Scale(a, a, a, a) }
func (c *ColorScale) ScaleWithColor(clr color.Color) {
	r, g, b, a := clr.RGBA()
	c.Scale(float32(r)/0xffff, float32(g)/0xffff, float32(b)/0xffff, float32(a)/0xffff)
}
func (c *ColorScale) rgba() (float32, float32, float32, float32) {
	return c.r_1 + 1, c.g_1 + 1, c.b_1 + 1, c.a_1 + 1
}

// Filter picks how a scaled source is sampled.
type Filter int

const (
	FilterNearest Filter = iota
	FilterLinear
)

// Blend is how a draw combines with what is there.
type Blend int

const (
	BlendSourceOver Blend = iota
	BlendLighter          // additive
	blendCopy             // replaces (Fill)
	blendCount
)

// DrawImageOptions are DrawImage's options.
type DrawImageOptions struct {
	GeoM       GeoM
	ColorScale ColorScale
	Filter     Filter
	Blend      Blend
}

// DrawImage draws src (its bounds at the origin) transformed by op.GeoM.
func (i *Image) DrawImage(src *Image, op *DrawImageOptions) {
	if op == nil {
		op = &DrawImageOptions{}
	}
	b := src.r
	if b.Empty() {
		return
	}
	r, g, bl, a := op.ColorScale.rgba()
	w, h := float64(b.Dx()), float64(b.Dy())
	v := func(x, y float64) Vertex {
		dx, dy := op.GeoM.Apply(x, y)
		return Vertex{DstX: float32(dx), DstY: float32(dy), SrcX: float32(b.Min.X) + float32(x), SrcY: float32(b.Min.Y) + float32(y),
			ColorR: r, ColorG: g, ColorB: bl, ColorA: a}
	}
	i.DrawTriangles([]Vertex{v(0, 0), v(w, 0), v(0, h), v(w, h)}, []uint16{0, 1, 2, 1, 3, 2}, src,
		&DrawTrianglesOptions{Filter: op.Filter, Blend: op.Blend})
}

// Vertex is one corner of a triangle: where it lands (DstX, DstY), the
// source pixel it samples (SrcX, SrcY), the colour it is multiplied by, and
// four free values for shaders.
type Vertex struct {
	DstX, DstY                     float32
	SrcX, SrcY                     float32
	ColorR, ColorG, ColorB, ColorA float32
	Custom0, Custom1               float32
	Custom2, Custom3               float32
}

// DrawTrianglesOptions are DrawTriangles' options.
type DrawTrianglesOptions struct {
	Filter Filter
	Blend  Blend
}

// DrawTriangles draws indexed triangles textured from img (nil: white).
func (i *Image) DrawTriangles(vs []Vertex, is []uint16, img *Image, op *DrawTrianglesOptions) {
	if op == nil {
		op = &DrawTrianglesOptions{}
	}
	if img == nil {
		img = white()
	}
	s := dev.nearest
	if op.Filter == FilterLinear {
		s = dev.linear
	}
	queue(batchKey{dst: i.t, clip: i.r, pipe: pipes.std[op.Blend], group: img.t.group(s)}, i, vs, is, img.t)
}

// batch is the run of triangles waiting to be drawn with one draw call:
// DrawImage calls with the same target, source and state join it.
type batchKey struct {
	dst   *texture
	clip  image.Rectangle
	pipe  *wgpu.RenderPipeline
	group *wgpu.BindGroup
	uni   string // the shader's uniform block
}

// gpuVertex is a Vertex as the pipelines read it: clip-space position,
// source position in pixels, colour, custom values.
type gpuVertex struct {
	X, Y, SrcX, SrcY, R, G, B, A, C0, C1, C2, C3 float32
}

var batch struct {
	key   batchKey
	verts []gpuVertex
	src   *texture
}

func pending(t *texture) bool {
	return len(batch.verts) > 0 && (batch.key.dst == t || batch.src == t)
}

// queue adds triangles to the batch, drawing the batch first if its key
// differs.
func queue(k batchKey, dst *Image, vs []Vertex, is []uint16, src *texture) {
	if src == dst.t {
		panic("gpu: an image drawn into itself")
	}
	if k != batch.key {
		flushBatch()
		batch.key, batch.src = k, src
	}
	sx, sy := 2/float32(dst.t.w), 2/float32(dst.t.h)
	for _, j := range is {
		v := vs[j]
		batch.verts = append(batch.verts, gpuVertex{v.DstX*sx - 1, 1 - v.DstY*sy, v.SrcX, v.SrcY,
			v.ColorR, v.ColorG, v.ColorB, v.ColorA, v.Custom0, v.Custom1, v.Custom2, v.Custom3})
	}
}

// flushBatch records the batch's draw call.
func flushBatch() {
	if len(batch.verts) == 0 {
		return
	}
	k := batch.key
	clip := k.clip.Intersect(image.Rect(0, 0, k.dst.w, k.dst.h))
	if !clip.Empty() {
		p := passOn(k.dst, false)
		p.SetPipeline(k.pipe)
		p.SetScissorRect(gputypes.ScissorRect{X: uint32(clip.Min.X), Y: uint32(clip.Min.Y), Width: uint32(clip.Dx()), Height: uint32(clip.Dy())})
		if k.uni != "" {
			bindUniforms(p, []byte(k.uni))
			p.SetBindGroup(1, k.group, nil)
		} else {
			p.SetBindGroup(0, k.group, nil)
		}
		c, off := dev.vertices.alloc(bytesOf(batch.verts), 0)
		p.SetVertexBuffer(0, c.buf, uint64(off))
		p.Draw(gputypes.DrawArgs{VertexCount: uint32(len(batch.verts)), InstanceCount: 1})
	}
	batch.verts = batch.verts[:0]
	batch.key, batch.src = batchKey{}, nil
}
