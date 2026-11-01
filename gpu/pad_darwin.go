package gpu

import (
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// The pads come from Apple's GameController framework, through the
// Objective-C runtime as gogpu's own window does.
var gc struct {
	once                       sync.Once
	ok                         bool
	send, getClass, sel        unsafe.Pointer
	push, pop                  unsafe.Pointer
	cifPtr, cifIdx, cifF, cifB types.CallInterface // id(id,SEL), id(id,SEL,NSUInteger), float(id,SEL), BOOL(id,SEL)
	cifArg, cifVoid, cifNone   types.CallInterface // ptr(ptr), void(ptr), ptr(void)
	sels                       map[string]uintptr
	controllers                uintptr // the GCController class
}

func gcInit() bool {
	gc.once.Do(func() {
		objc, err := ffi.LoadLibrary("/usr/lib/libobjc.A.dylib")
		if err != nil {
			return
		}
		if _, err := ffi.LoadLibrary("/System/Library/Frameworks/GameController.framework/GameController"); err != nil {
			return
		}
		for _, s := range []struct {
			p    *unsafe.Pointer
			name string
		}{{&gc.send, "objc_msgSend"}, {&gc.getClass, "objc_getClass"}, {&gc.sel, "sel_registerName"}, {&gc.push, "objc_autoreleasePoolPush"}, {&gc.pop, "objc_autoreleasePoolPop"}} {
			if *s.p, err = ffi.GetSymbol(objc, s.name); err != nil {
				return
			}
		}
		P := types.PointerTypeDescriptor
		for _, c := range []struct {
			cif  *types.CallInterface
			ret  *types.TypeDescriptor
			args []*types.TypeDescriptor
		}{{&gc.cifPtr, P, []*types.TypeDescriptor{P, P}}, {&gc.cifIdx, P, []*types.TypeDescriptor{P, P, types.UInt64TypeDescriptor}},
			{&gc.cifF, types.FloatTypeDescriptor, []*types.TypeDescriptor{P, P}}, {&gc.cifB, types.UInt8TypeDescriptor, []*types.TypeDescriptor{P, P}},
			{&gc.cifArg, P, []*types.TypeDescriptor{P}}, {&gc.cifNone, P, nil}, {&gc.cifVoid, types.VoidTypeDescriptor, []*types.TypeDescriptor{P}}} {
			if ffi.PrepareCallInterface(c.cif, types.DefaultCall, c.ret, c.args) != nil {
				return
			}
		}
		gc.sels = map[string]uintptr{}
		gc.controllers = cstrCall(gc.getClass, "GCController")
		gc.ok = gc.controllers != 0
	})
	return gc.ok
}

// cstrCall calls fn(const char *s), as objc_getClass and sel_registerName.
func cstrCall(fn unsafe.Pointer, s string) uintptr {
	b := append([]byte(s), 0)
	p := unsafe.Pointer(&b[0])
	var r uintptr
	ffi.CallFunction(&gc.cifArg, fn, unsafe.Pointer(&r), []unsafe.Pointer{unsafe.Pointer(&p)})
	return r
}

func selector(name string) uintptr {
	s, ok := gc.sels[name]
	if !ok {
		s = cstrCall(gc.sel, name)
		gc.sels[name] = s
	}
	return s
}

// msg sends a message with no arguments, the result into r (a pointer to the return type).
func msg(cif *types.CallInterface, id uintptr, name string, r unsafe.Pointer) {
	sel := selector(name)
	ffi.CallFunction(cif, gc.send, r, []unsafe.Pointer{unsafe.Pointer(&id), unsafe.Pointer(&sel)})
}

func obj(id uintptr, name string) uintptr {
	var r uintptr
	if id != 0 {
		msg(&gc.cifPtr, id, name, unsafe.Pointer(&r))
	}
	return r
}

func pressed(button uintptr) bool {
	var r uint8
	if button != 0 {
		msg(&gc.cifB, button, "isPressed", unsafe.Pointer(&r))
	}
	return r != 0
}

func value(input uintptr) float64 {
	var r float32
	if input != 0 {
		msg(&gc.cifF, input, "value", unsafe.Pointer(&r))
	}
	return float64(r)
}

// gcButtons: the extended gamepad's buttons in StandardGamepadButton order
// ("" for the triggers, read as values, and the d-pad, read from "dpad").
var gcButtons = [padButtons]string{"buttonA", "buttonB", "buttonX", "buttonY", "leftShoulder", "rightShoulder", "", "",
	"buttonOptions", "buttonMenu", "leftThumbstickButton", "rightThumbstickButton", "", "", "", "", "buttonHome"}

// readPads reads the extended gamepads; on the main thread.
func readPads(p *[maxPads]padState) {
	if !gcInit() {
		return
	}
	var pool uintptr
	ffi.CallFunction(&gc.cifNone, gc.push, unsafe.Pointer(&pool), nil)
	defer ffi.CallFunction(&gc.cifVoid, gc.pop, nil, []unsafe.Pointer{unsafe.Pointer(&pool)})
	list := obj(gc.controllers, "controllers")
	var n uint64
	msg(&gc.cifPtr, list, "count", unsafe.Pointer(&n))
	for i := range min(int(n), maxPads) {
		var c uintptr
		idx := uint64(i)
		sel := selector("objectAtIndex:")
		ffi.CallFunction(&gc.cifIdx, gc.send, unsafe.Pointer(&c), []unsafe.Pointer{unsafe.Pointer(&list), unsafe.Pointer(&sel), unsafe.Pointer(&idx)})
		g := obj(c, "extendedGamepad")
		if g == 0 {
			continue
		}
		q := &p[i]
		q.ok = true
		for b, name := range gcButtons {
			if name != "" {
				q.btn[b] = pressed(obj(g, name))
			}
		}
		q.btn[StandardGamepadButtonFrontBottomLeft] = value(obj(g, "leftTrigger")) > triggerDown
		q.btn[StandardGamepadButtonFrontBottomRight] = value(obj(g, "rightTrigger")) > triggerDown
		d := obj(g, "dpad")
		for b, name := range map[StandardGamepadButton]string{StandardGamepadButtonLeftTop: "up", StandardGamepadButtonLeftBottom: "down", StandardGamepadButtonLeftLeft: "left", StandardGamepadButtonLeftRight: "right"} {
			q.btn[b] = pressed(obj(d, name))
		}
		l, r := obj(g, "leftThumbstick"), obj(g, "rightThumbstick")
		q.axis = [4]float64{value(obj(l, "xAxis")), -value(obj(l, "yAxis")), value(obj(r, "xAxis")), -value(obj(r, "yAxis"))} // up is positive there, down here
	}
}
