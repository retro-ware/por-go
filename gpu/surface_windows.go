//go:build windows

package gpu

import (
	"syscall"
	"unsafe"

	"github.com/gogpu/wgpu"

	"gpu/internal/glfw"
)

func surfaceTarget(win *glfw.Window) wgpu.SurfaceTargetUnsafe {
	inst, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	return wgpu.SurfaceTargetFromWindowsHWND(inst, uintptr(unsafe.Pointer(win.GetWin32Window())))
}
