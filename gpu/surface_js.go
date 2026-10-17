//go:build js

package gpu

import (
	"github.com/gogpu/wgpu"

	"gpu/internal/glfw"
)

// surfaceTarget is the page's first canvas (glfw's window).
func surfaceTarget(*glfw.Window) wgpu.SurfaceTargetUnsafe { return wgpu.SurfaceTargetFromWebCanvasID(0) }
