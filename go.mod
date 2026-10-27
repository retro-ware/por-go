module por

go 1.26.2

require (
	github.com/andybalholm/brotli v1.2.2 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/sevenzip v1.6.5 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gogpu v0.54.0 // indirect
	github.com/gogpu/gpucontext v0.31.3 // indirect
	github.com/gogpu/gputypes v0.8.0 // indirect
	github.com/gogpu/naga v0.19.0 // indirect
	github.com/gogpu/wgpu v0.34.5 // indirect
	github.com/hajimehoshi/go-mp3 v0.3.4 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/klauspost/compress v1.19.0 // indirect
	github.com/nwaples/rardecode/v2 v2.4.5 // indirect
	github.com/pierrec/lz4/v4 v4.1.27 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/stangelandcl/ppmd v0.1.1 // indirect
	go4.org v0.0.0-20260112195520-a5071408f32f // indirect
	golang.org/x/text v0.41.0 // indirect
)

require (
	blobber v0.0.0
	github.com/ebitengine/oto/v3 v3.5.0
	github.com/ebitengine/purego v0.11.0 // indirect
	github.com/jfreymuth/pulse v0.1.3 // indirect
	github.com/ulikunitz/xz v0.5.17 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace blobber => ./phlan-poc/blobber

require (
	github.com/retro-ware/gameimport v0.0.0
	golang.org/x/image v0.45.0
	gpu v0.0.0
)

replace gpu => ./gpu

replace github.com/gogpu/wgpu => ../gogpu-wgpu

replace github.com/retro-ware/gameimport => ../gameimport

replace github.com/go-webgpu/goffi => ../goffi

replace github.com/gogpu/gogpu => ../goplay/third_party/gogpu // macOS fullscreen fixes, see goplay/third_party/README.md
