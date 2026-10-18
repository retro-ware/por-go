module por

go 1.26.2

require (
	github.com/go-gl/glfw/v3.3/glfw v0.0.0-20250301202403-da16c1255728 // indirect
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gpucontext v0.31.3 // indirect
	github.com/gogpu/gputypes v0.8.0 // indirect
	github.com/gogpu/naga v0.19.0 // indirect
	github.com/gogpu/wgpu v0.34.5 // indirect
)

require (
	blobber v0.0.0
	github.com/ebitengine/oto/v3 v3.5.0
	github.com/ebitengine/purego v0.11.0 // indirect
	github.com/jfreymuth/pulse v0.1.3 // indirect
	github.com/ulikunitz/xz v0.5.17
	golang.org/x/sys v0.47.0 // indirect
)

replace blobber => ./phlan-poc/blobber

require (
	golang.org/x/image v0.45.0
	gpu v0.0.0
)

replace gpu => ./gpu

replace github.com/gogpu/wgpu => ../gogpu-wgpu
