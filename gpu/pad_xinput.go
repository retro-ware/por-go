package gpu

// XInput's state in the standard layout; pad_windows.go reads it.

// xinputState is XINPUT_STATE.
type xinputState struct {
	packet         uint32
	buttons        uint16
	lt, rt         uint8
	lx, ly, rx, ry int16
}

// xinputButtons: XInput's button bits in StandardGamepadButton order (the triggers are axes).
var xinputButtons = [padButtons]uint16{
	StandardGamepadButtonRightBottom:   0x1000,
	StandardGamepadButtonRightRight:    0x2000,
	StandardGamepadButtonRightLeft:     0x4000,
	StandardGamepadButtonRightTop:      0x8000,
	StandardGamepadButtonFrontTopLeft:  0x0100,
	StandardGamepadButtonFrontTopRight: 0x0200,
	StandardGamepadButtonCenterLeft:    0x0020,
	StandardGamepadButtonCenterRight:   0x0010,
	StandardGamepadButtonLeftStick:     0x0040,
	StandardGamepadButtonRightStick:    0x0080,
	StandardGamepadButtonLeftTop:       0x0001,
	StandardGamepadButtonLeftBottom:    0x0002,
	StandardGamepadButtonLeftLeft:      0x0004,
	StandardGamepadButtonLeftRight:     0x0008,
}

// xinputPad is an XInput state in the standard layout (y down, as the browser's).
func xinputPad(s xinputState) padState {
	q := padState{ok: true}
	for b, m := range xinputButtons {
		q.btn[b] = m != 0 && s.buttons&m != 0
	}
	q.btn[StandardGamepadButtonFrontBottomLeft] = float64(s.lt)/255 > triggerDown
	q.btn[StandardGamepadButtonFrontBottomRight] = float64(s.rt)/255 > triggerDown
	q.axis = [4]float64{float64(s.lx) / 32768, -float64(s.ly) / 32768, float64(s.rx) / 32768, -float64(s.ry) / 32768}
	return q
}
