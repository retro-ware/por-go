module blobber

go 1.26.2

require golang.org/x/image v0.45.0

require (
	github.com/ebitengine/purego v0.11.0 // indirect
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gpucontext v0.31.3 // indirect
	github.com/gogpu/gputypes v0.8.0 // indirect
	github.com/gogpu/naga v0.19.0 // indirect
	github.com/gogpu/wgpu v0.34.5 // indirect
	github.com/jfreymuth/pulse v0.1.3 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

require (
	gpu v0.0.0
)

replace gpu => ../../gpu

replace github.com/gogpu/wgpu => ../../../gogpu-wgpu
