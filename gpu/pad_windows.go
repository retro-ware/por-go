package gpu

import (
	"syscall"
	"unsafe"
)

var xinputGetState = syscall.NewLazyDLL("xinput1_4.dll").NewProc("XInputGetState")

// readPads reads the XInput pads.
func readPads(p *[maxPads]padState) {
	if xinputGetState.Find() != nil {
		return
	}
	for i := range p {
		var s xinputState
		if r, _, _ := xinputGetState.Call(uintptr(i), uintptr(unsafe.Pointer(&s))); r != 0 {
			continue
		}
		p[i] = xinputPad(s)
	}
}
