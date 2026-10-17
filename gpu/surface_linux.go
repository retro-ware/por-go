//go:build linux && !android

package gpu

import (
	"unsafe"

	"github.com/gogpu/wgpu"

	"gpu/internal/glfw"
)

// surfaceTarget is the window's X11 handle (XWayland on Wayland desktops).
func surfaceTarget(win *glfw.Window) wgpu.SurfaceTargetUnsafe {
	return wgpu.SurfaceTargetFromXlibWindow(uintptr(unsafe.Pointer(glfw.GetX11Display())), uintptr(win.GetX11Window()))
}
