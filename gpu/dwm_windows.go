//go:build windows

package gpu

import "syscall"

var dwmEnableComposition = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmEnableComposition")

// setComposition turns Windows 7's desktop composition off for a fullscreen
// game (and on again after): the compositor copies every frame of a window
// once more and waits for the screen to do it, half the frame time on a slow
// machine. Composition comes back when the process ends, too. Windows 8 and
// later refuse, composition being always on there, and that is fine.
func setComposition(on bool) {
	var v uintptr
	if on {
		v = 1
	}
	dwmEnableComposition.Call(v)
}
