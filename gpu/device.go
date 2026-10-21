// Package gpu is the slice of Ebiten's API that por and Blobber use, built
// on gogpu and gogpu/wgpu (pure Go, no cgo): a window, input, images drawn with batched
// triangles, WGSL shaders in the role of Kage, and depth-tested meshes for
// the 3D views. The device code is copied from VoxelLib's wgpu backend.
package gpu

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"time"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// dev is the process-wide WebGPU state: one window, one device. Draws
// record into one command encoder, submitted at the end of each frame (or
// earlier when pixels are written or read back).
var dev struct {
	device *wgpu.Device // gogpu's while RunGame runs, else a headless one
	queue  *wgpu.Queue
	format gputypes.TextureFormat // the window's

	enc  *wgpu.CommandEncoder
	pass *wgpu.RenderPassEncoder
	on   *texture // the pass's target
	pdep bool     // the pass has the target's depth buffer

	uniforms, vertices stream

	uniLayout *wgpu.BindGroupLayout // dynamic uniform block
	texLayout *wgpu.BindGroupLayout // one texture and a sampler
	nearest   *wgpu.Sampler
	linear    *wgpu.Sampler
	repeat    *wgpu.Sampler // nearest, wrapping (mesh textures)
}

// uniformSlot is the bound size of a dynamic uniform block.
const uniformSlot = 1024

// needDevice creates the device on first use: images may be made and drawn
// before RunGame opens the window (package init, tests).
func needDevice() {
	if dev.device == nil {
		if err := initDevice(); err != nil {
			panic(err)
		}
	}
}

// adoptDevice takes gogpu's device, which draws into the window: RunGame calls it at the
// first frame, before anything is drawn (a headless device's images can't go there).
func adoptDevice(d *wgpu.Device, q *wgpu.Queue, format gputypes.TextureFormat) error {
	if dev.device != nil && dev.device != d {
		return fmt.Errorf("gpu: images were made before RunGame opened the window")
	}
	dev.format = format
	if dev.device == nil {
		dev.device, dev.queue = d, q
		if err := initShared(); err != nil {
			return err
		}
	}
	// an sRGB window would brighten the game's colours: the blit undoes it
	wgsl := blitWGSL
	if format == gputypes.TextureFormatBGRA8UnormSrgb || format == gputypes.TextureFormatRGBA8UnormSrgb {
		wgsl = blitSRGBWGSL
	}
	var err error
	pipes.blit, err = newPipeline(pipeSpec{"present", wgsl, []*wgpu.BindGroupLayout{dev.texLayout}, nil, nil, format, false, "", false})
	return err
}

// initDevice makes a device of its own, for images made outside RunGame (tools, tests).
func initDevice() error {
	inst, err := wgpu.CreateInstance(nil)
	if err != nil {
		return fmt.Errorf("wgpu: instance: %w", err)
	}
	// ponytail: the adapter is picked without the surface; fine with one GPU,
	// pass CompatibleSurface if a laptop's other GPU gets picked
	adapter, err := inst.RequestAdapter(&wgpu.RequestAdapterOptions{
		PowerPreference: gputypes.PowerPreferenceHighPerformance,
	})
	if err != nil {
		return fmt.Errorf("wgpu: adapter: %w", err)
	}
	if dev.device, err = adapter.RequestDevice(&wgpu.DeviceDescriptor{Label: "gpu", RequiredLimits: adapter.Limits()}); err != nil {
		return fmt.Errorf("wgpu: device: %w", err)
	}
	dev.queue = dev.device.Queue()
	return initShared()
}

// initShared makes the buffers, layouts, samplers and pipelines on dev.device.
func initShared() error {
	var err error
	dev.uniforms = stream{usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst, chunk: 4 << 20, align: 256, label: "uniforms"}
	dev.vertices = stream{usage: wgpu.BufferUsageVertex | wgpu.BufferUsageCopyDst, chunk: 4 << 20, align: 16, label: "vertices"}
	if dev.uniLayout, err = dev.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{Label: "uniform block",
		Entries: []gputypes.BindGroupLayoutEntry{{Binding: 0, Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
			Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, HasDynamicOffset: true, MinBindingSize: uniformSlot}}},
	}); err != nil {
		return err
	}
	if dev.texLayout, err = dev.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{Label: "texture",
		Entries: []gputypes.BindGroupLayoutEntry{texEntry(0), samplerEntry(1)}}); err != nil {
		return err
	}
	sampler := func(filter gputypes.FilterMode, addr gputypes.AddressMode) *wgpu.Sampler {
		if err != nil {
			return nil
		}
		var s *wgpu.Sampler
		s, err = dev.device.CreateSampler(&wgpu.SamplerDescriptor{Label: "sampler",
			AddressModeU: addr, AddressModeV: addr, AddressModeW: addr,
			MagFilter: filter, MinFilter: filter, MipmapFilter: gputypes.FilterModeNearest, LodMaxClamp: 32})
		return s
	}
	dev.nearest = sampler(gputypes.FilterModeNearest, gputypes.AddressModeClampToEdge)
	dev.linear = sampler(gputypes.FilterModeLinear, gputypes.AddressModeClampToEdge)
	dev.repeat = sampler(gputypes.FilterModeNearest, gputypes.AddressModeRepeat)
	if err != nil {
		return err
	}
	return initPipelines()
}

func texEntry(binding uint32) gputypes.BindGroupLayoutEntry {
	return gputypes.BindGroupLayoutEntry{Binding: binding, Visibility: gputypes.ShaderStageFragment,
		Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D}}
}

func samplerEntry(binding uint32) gputypes.BindGroupLayoutEntry {
	return gputypes.BindGroupLayoutEntry{Binding: binding, Visibility: gputypes.ShaderStageFragment,
		Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}}
}

// encoder returns the frame's command encoder, creating it if needed.
func encoder() *wgpu.CommandEncoder {
	if dev.enc == nil {
		var err error
		if dev.enc, err = dev.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "frame"}); err != nil {
			panic(err)
		}
	}
	return dev.enc
}

// passOn returns a render pass drawing into t (with its depth buffer if
// depth), beginning one if the current pass draws elsewhere.
func passOn(t *texture, depth bool) *wgpu.RenderPassEncoder {
	if dev.pass != nil && dev.on == t && dev.pdep == depth {
		return dev.pass
	}
	endPass()
	t.ensure()
	color := wgpu.RenderPassColorAttachment{View: t.view, LoadOp: gputypes.LoadOpLoad, StoreOp: gputypes.StoreOpStore}
	if t.clear {
		color.LoadOp, color.ClearValue, t.clear = gputypes.LoadOpClear, gputypes.Color{}, false
	}
	desc := &wgpu.RenderPassDescriptor{Label: "draw", ColorAttachments: []wgpu.RenderPassColorAttachment{color}}
	if depth {
		desc.DepthStencilAttachment = &wgpu.RenderPassDepthStencilAttachment{View: t.depthView(),
			DepthLoadOp: gputypes.LoadOpLoad, DepthStoreOp: gputypes.StoreOpStore}
		if t.clearDepth {
			desc.DepthStencilAttachment.DepthLoadOp, desc.DepthStencilAttachment.DepthClearValue, t.clearDepth = gputypes.LoadOpClear, 1, false
		}
	}
	p, err := encoder().BeginRenderPass(desc)
	if err != nil {
		panic(err)
	}
	dev.pass, dev.on, dev.pdep = p, t, depth
	return p
}

func endPass() {
	if dev.pass != nil {
		if err := dev.pass.End(); err != nil {
			panic(err)
		}
		dev.pass, dev.on = nil, nil
	}
}

// submit sends everything recorded so far to the GPU.
func submit() {
	flushBatch()
	if dev.enc == nil {
		return
	}
	endPass()
	dev.uniforms.flush()
	dev.vertices.flush()
	cmd, err := dev.enc.Finish()
	if err != nil {
		panic(err)
	}
	dev.enc = nil
	if _, err := dev.queue.Submit(cmd); err != nil {
		panic(err)
	}
	dev.uniforms.reset()
	dev.vertices.reset()
}

// stream is a set of per-frame buffer chunks for data written each frame:
// allocations are copied to a CPU shadow and written with one
// Queue.WriteBuffer per chunk before submit.
type stream struct {
	usage  gputypes.BufferUsage
	chunk  int
	align  int
	label  string
	chunks []*streamChunk
	cur    int
}

type streamChunk struct {
	buf     *wgpu.Buffer
	shadow  []byte
	used    int
	flushed int
	bg      *wgpu.BindGroup // uniform chunks: bound at dynamic offsets
}

// alloc copies data into the stream (reserving at least reserve bytes) and
// returns the chunk and the offset.
func (s *stream) alloc(data []byte, reserve int) (*streamChunk, uint32) {
	need := max(len(data), reserve)
	for {
		if s.cur >= len(s.chunks) {
			size := max(s.chunk, need)
			b, err := dev.device.CreateBuffer(&wgpu.BufferDescriptor{Label: s.label, Size: uint64(size), Usage: s.usage})
			if err != nil {
				panic(err)
			}
			c := &streamChunk{buf: b, shadow: make([]byte, size)}
			if s.usage&wgpu.BufferUsageUniform != 0 {
				if c.bg, err = dev.device.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: s.label, Layout: dev.uniLayout,
					Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: b, Size: uniformSlot}}}); err != nil {
					panic(err)
				}
			}
			s.chunks = append(s.chunks, c)
		}
		c := s.chunks[s.cur]
		off := (c.used + s.align - 1) &^ (s.align - 1)
		if off+need <= len(c.shadow) {
			copy(c.shadow[off:], data)
			clear(c.shadow[off+len(data) : off+need])
			c.used = off + need
			return c, uint32(off)
		}
		s.cur++
	}
}

func (s *stream) flush() {
	for _, c := range s.chunks[:min(s.cur+1, len(s.chunks))] {
		if c.used > c.flushed {
			lo, hi := c.flushed&^3, (c.used+3)&^3
			if err := dev.queue.WriteBuffer(c.buf, uint64(lo), c.shadow[lo:hi]); err != nil {
				panic(err)
			}
			c.flushed = c.used
		}
	}
}

func (s *stream) reset() {
	for _, c := range s.chunks {
		c.used, c.flushed = 0, 0
	}
	s.cur = 0
}

// bindUniforms writes a uniform block and binds it as group 0.
func bindUniforms(p *wgpu.RenderPassEncoder, b []byte) {
	c, off := dev.uniforms.alloc(b, uniformSlot)
	p.SetBindGroup(0, c.bg, []uint32{off})
}

func appendF32(b []byte, vs ...float32) []byte {
	for _, v := range vs {
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
	}
	return b
}

func bytesOf[T any](s []T) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&s[0])), len(s)*int(unsafe.Sizeof(s[0])))
}

// readTexture copies the w x h pixels at x, y of t into px (RGBA, top-down).
// It waits for the GPU: for screenshots and tests.
func readTexture(t *texture, x, y, w, h int, px []byte) {
	t.ensure()
	submit()
	row := (uint32(w)*4 + 255) &^ 255
	size := uint64(row) * uint64(h)
	buf, err := dev.device.CreateBuffer(&wgpu.BufferDescriptor{Label: "readback", Size: size, Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead})
	if err != nil {
		panic(err)
	}
	defer buf.Release()
	encoder().CopyTextureToBuffer(t.tex, buf, []wgpu.BufferTextureCopy{{
		BufferLayout: wgpu.ImageDataLayout{BytesPerRow: row, RowsPerImage: uint32(h)},
		TextureBase:  wgpu.ImageCopyTexture{Texture: t.tex, Origin: wgpu.Origin3D{X: uint32(x), Y: uint32(y)}},
		Size:         wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
	}})
	submit()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := buf.Map(ctx, wgpu.MapModeRead, 0, size); err != nil {
		panic(err)
	}
	rng, err := buf.MappedRange(0, size)
	if err != nil {
		panic(err)
	}
	src := rng.Bytes()
	for r := 0; r < h; r++ {
		copy(px[r*w*4:(r+1)*w*4], src[uint32(r)*row:])
	}
	buf.Unmap()
}

// present draws screen into view, the window's frame (w x h framebuffer pixels), at the
// largest size that keeps its aspect ratio; gogpu shows the frame.
func present(screen *Image, view *wgpu.TextureView, w, h int) {
	flushBatch()
	if w <= 0 || h <= 0 {
		submit()
		return
	}
	endPass()
	p, err := encoder().BeginRenderPass(&wgpu.RenderPassDescriptor{Label: "present",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore,
			ClearValue: gputypes.Color{A: 1}}}})
	if err != nil {
		panic(err)
	}
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	k := min(float64(w)/float64(sw), float64(h)/float64(sh))
	vw, vh := float32(float64(sw)*k), float32(float64(sh)*k)
	p.SetViewport(gputypes.Viewport{X: (float32(w) - vw) / 2, Y: (float32(h) - vh) / 2, Width: vw, Height: vh, MaxDepth: 1})
	p.SetPipeline(pipes.blit)
	sampler := dev.nearest
	if k != math.Trunc(k) {
		sampler = dev.linear
	}
	p.SetBindGroup(0, screen.t.group(sampler), nil)
	p.Draw(gputypes.DrawArgs{VertexCount: 3, InstanceCount: 1})
	if err := p.End(); err != nil {
		panic(err)
	}
	submit()
}
