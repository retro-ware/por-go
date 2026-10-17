//go:build darwin

package gpu

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore
#import <Cocoa/Cocoa.h>
#import <QuartzCore/CAMetalLayer.h>

static void *gpu_metal_layer(void *win) {
	NSWindow *w = (__bridge NSWindow *)win;
	NSView *v = [w contentView];
	CAMetalLayer *l = [CAMetalLayer layer];
	l.contentsScale = w.backingScaleFactor;
	[v setLayer:l];
	[v setWantsLayer:YES];
	return (__bridge void *)l;
}
*/
import "C"

import (
	"github.com/gogpu/wgpu"

	"gpu/internal/glfw"
)

// surfaceTarget attaches a CAMetalLayer to the window's content view.
func surfaceTarget(win *glfw.Window) wgpu.SurfaceTargetUnsafe {
	return wgpu.SurfaceTargetFromMetalLayer(uintptr(C.gpu_metal_layer(win.GetCocoaWindow())))
}
