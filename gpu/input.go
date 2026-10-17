package gpu

import "gpu/internal/glfw"

// Key is a keyboard key: GLFW's key codes, plus the either-side modifiers.
type Key int

const (
	KeySpace     = Key(glfw.KeySpace)
	KeyComma     = Key(glfw.KeyComma)
	KeyMinus     = Key(glfw.KeyMinus)
	KeyPeriod    = Key(glfw.KeyPeriod)
	KeySlash     = Key(glfw.KeySlash)
	Key0         = Key(glfw.Key0)
	Key1         = Key0 + 1
	Key2         = Key0 + 2
	Key3         = Key0 + 3
	Key4         = Key0 + 4
	Key5         = Key0 + 5
	Key6         = Key0 + 6
	Key7         = Key0 + 7
	Key8         = Key0 + 8
	Key9         = Key0 + 9
	KeyEqual     = Key(glfw.KeyEqual)
	KeyA         = Key(glfw.KeyA)
	KeyB         = KeyA + 1
	KeyC         = KeyA + 2
	KeyD         = KeyA + 3
	KeyE         = KeyA + 4
	KeyF         = KeyA + 5
	KeyG         = KeyA + 6
	KeyH         = KeyA + 7
	KeyI         = KeyA + 8
	KeyJ         = KeyA + 9
	KeyK         = KeyA + 10
	KeyL         = KeyA + 11
	KeyM         = KeyA + 12
	KeyN         = KeyA + 13
	KeyO         = KeyA + 14
	KeyP         = KeyA + 15
	KeyQ         = KeyA + 16
	KeyR         = KeyA + 17
	KeyS         = KeyA + 18
	KeyT         = KeyA + 19
	KeyU         = KeyA + 20
	KeyV         = KeyA + 21
	KeyW         = KeyA + 22
	KeyX         = KeyA + 23
	KeyY         = KeyA + 24
	KeyZ         = KeyA + 25
	KeyEscape    = Key(glfw.KeyEscape)
	KeyEnter     = Key(glfw.KeyEnter)
	KeyTab       = Key(glfw.KeyTab)
	KeyBackspace = Key(glfw.KeyBackspace)
	KeyInsert    = Key(glfw.KeyInsert)
	KeyDelete    = Key(glfw.KeyDelete)
	KeyRight     = Key(glfw.KeyRight)
	KeyLeft      = Key(glfw.KeyLeft)
	KeyDown      = Key(glfw.KeyDown)
	KeyUp        = Key(glfw.KeyUp)
	KeyPageUp    = Key(glfw.KeyPageUp)
	KeyPageDown  = Key(glfw.KeyPageDown)
	KeyHome      = Key(glfw.KeyHome)
	KeyEnd       = Key(glfw.KeyEnd)
	KeyF1        = Key(glfw.KeyF1)
	KeyF2        = KeyF1 + 1
	KeyF3        = KeyF1 + 2
	KeyF4        = KeyF1 + 3
	KeyF5        = KeyF1 + 4
	KeyF6        = KeyF1 + 5
	KeyF7        = KeyF1 + 6
	KeyF8        = KeyF1 + 7
	KeyF9        = KeyF1 + 8
	KeyF10       = KeyF1 + 9
	KeyF11       = KeyF1 + 10
	KeyF12       = KeyF1 + 11
	KeyKP0       = Key(glfw.KeyKP0)
	KeyKP1       = KeyKP0 + 1
	KeyKP2       = KeyKP0 + 2
	KeyKP3       = KeyKP0 + 3
	KeyKP4       = KeyKP0 + 4
	KeyKP5       = KeyKP0 + 5
	KeyKP6       = KeyKP0 + 6
	KeyKP7       = KeyKP0 + 7
	KeyKP8       = KeyKP0 + 8
	KeyKP9       = KeyKP0 + 9
	KeyKPEnter   = Key(glfw.KeyKPEnter)

	KeyArrowUp    = KeyUp
	KeyArrowDown  = KeyDown
	KeyArrowLeft  = KeyLeft
	KeyArrowRight = KeyRight

	keyCount = 512 // GLFW codes stay below

	// either side
	KeyShift Key = keyCount + iota
	KeyControl
	KeyAlt
	KeyMeta
	keyAll
)

var sides = map[Key][2]glfw.Key{
	KeyShift:   {glfw.KeyLeftShift, glfw.KeyRightShift},
	KeyControl: {glfw.KeyLeftControl, glfw.KeyRightControl},
	KeyAlt:     {glfw.KeyLeftAlt, glfw.KeyRightAlt},
	KeyMeta:    {glfw.KeyLeftSuper, glfw.KeyRightSuper},
}

// MouseButton is a mouse button.
type MouseButton int

const (
	MouseButtonLeft   = MouseButton(glfw.MouseButtonLeft)
	MouseButtonRight  = MouseButton(glfw.MouseButtonRight)
	MouseButtonMiddle = MouseButton(glfw.MouseButtonMiddle)
	mouseCount        = 8
)

// GamepadID is a connected gamepad.
type GamepadID int

// StandardGamepadButton is a button of the standard layout, named by place.
type StandardGamepadButton int

const (
	StandardGamepadButtonRightBottom StandardGamepadButton = iota // A
	StandardGamepadButtonRightRight                               // B
	StandardGamepadButtonRightLeft                                // X
	StandardGamepadButtonRightTop                                 // Y
	StandardGamepadButtonFrontTopLeft
	StandardGamepadButtonFrontTopRight
	StandardGamepadButtonFrontBottomLeft  // left trigger
	StandardGamepadButtonFrontBottomRight // right trigger
	StandardGamepadButtonCenterLeft       // back
	StandardGamepadButtonCenterRight      // start
	StandardGamepadButtonLeftStick
	StandardGamepadButtonRightStick
	StandardGamepadButtonLeftTop // d-pad
	StandardGamepadButtonLeftBottom
	StandardGamepadButtonLeftLeft
	StandardGamepadButtonLeftRight
	StandardGamepadButtonCenterCenter
	padButtons
)

// padGLFW is each standard button's GLFW gamepad button (-1: a trigger axis).
var padGLFW = [padButtons]int{0, 1, 2, 3, 4, 5, -4, -5, 6, 7, 9, 10, 11, 13, 14, 12, 8}

// StandardGamepadAxis is an axis of the standard layout.
type StandardGamepadAxis int

const (
	StandardGamepadAxisLeftStickHorizontal StandardGamepadAxis = iota
	StandardGamepadAxisLeftStickVertical
	StandardGamepadAxisRightStickHorizontal
	StandardGamepadAxisRightStickVertical
)

// TouchID is a touch; there are none on the desktop.
type TouchID int

// in is the input as Update sees it: per key and button, the ticks it has
// been held (counted at each tick, so a press shorter than a tick still
// counts once).
var in struct {
	down, hit [keyAll]bool
	dur       [keyAll]int
	mdown     [mouseCount]bool
	mhit      [mouseCount]bool
	mdur      [mouseCount]int
	chars     []rune // typed since the last tick
	tickChars []rune
	wheel     [2]float64
	tickWheel [2]float64
	pads      [16]*glfw.GamepadState
	pdur      [16][padButtons]int
}

func initInput(w *glfw.Window) {
	w.SetKeyCallback(func(_ *glfw.Window, k glfw.Key, _ int, a glfw.Action, _ glfw.ModifierKey) {
		if k < 0 || int(k) >= keyCount || a == glfw.Repeat {
			return
		}
		in.down[k] = a == glfw.Press
		in.hit[k] = in.hit[k] || a == glfw.Press
	})
	w.SetCharCallback(func(_ *glfw.Window, r rune) { in.chars = append(in.chars, r) })
	w.SetMouseButtonCallback(func(_ *glfw.Window, b glfw.MouseButton, a glfw.Action, _ glfw.ModifierKey) {
		if int(b) < mouseCount {
			in.mdown[b] = a == glfw.Press
			in.mhit[b] = in.mhit[b] || a == glfw.Press
		}
	})
	w.SetScrollCallback(func(_ *glfw.Window, dx, dy float64) { in.wheel[0] += dx; in.wheel[1] += dy })
}

// tickInput advances the held counts; called before each Update.
func tickInput() {
	for _, k := range []Key{KeyShift, KeyControl, KeyAlt, KeyMeta} {
		s := sides[k]
		in.down[k] = in.down[s[0]] || in.down[s[1]]
		in.hit[k] = in.hit[s[0]] || in.hit[s[1]]
	}
	for k := range in.dur {
		if in.down[k] || in.hit[k] && in.dur[k] == 0 {
			in.dur[k]++
		} else {
			in.dur[k] = 0
		}
		in.hit[k] = false
	}
	for b := range in.mdur {
		if in.mdown[b] || in.mhit[b] && in.mdur[b] == 0 {
			in.mdur[b]++
		} else {
			in.mdur[b] = 0
		}
		in.mhit[b] = false
	}
	in.tickChars, in.chars = append(in.tickChars[:0], in.chars...), in.chars[:0]
	in.tickWheel, in.wheel = in.wheel, [2]float64{}
	for j := range in.pads {
		in.pads[j] = nil
		if js := glfw.Joystick(int(glfw.Joystick1) + j); js.IsGamepad() {
			in.pads[j] = js.GetGamepadState()
		}
		for b := range in.pdur[j] {
			if padPressed(j, StandardGamepadButton(b)) {
				in.pdur[j][b]++
			} else {
				in.pdur[j][b] = 0
			}
		}
	}
}

func padPressed(j int, b StandardGamepadButton) bool {
	s := in.pads[j]
	if s == nil {
		return false
	}
	if g := padGLFW[b]; g >= 0 {
		return s.Buttons[g] == glfw.Press
	} else {
		return s.Axes[-g] > 0
	}
}

func IsKeyPressed(k Key) bool { return k >= 0 && k < keyAll && in.dur[k] > 0 }
func IsKeyJustPressed(k Key) bool { return KeyPressDuration(k) == 1 }
func KeyPressDuration(k Key) int {
	if k < 0 || k >= keyAll {
		return 0
	}
	return in.dur[k]
}

// AppendJustPressedKeys appends the keys pressed this tick.
func AppendJustPressedKeys(ks []Key) []Key {
	for k := range Key(keyCount) {
		if in.dur[k] == 1 {
			ks = append(ks, k)
		}
	}
	return ks
}

// AppendInputChars appends the characters typed since the last tick.
func AppendInputChars(rs []rune) []rune { return append(rs, in.tickChars...) }

func IsMouseButtonPressed(b MouseButton) bool { return b >= 0 && b < mouseCount && in.mdur[b] > 0 }
func IsMouseButtonJustPressed(b MouseButton) bool {
	return b >= 0 && b < mouseCount && in.mdur[b] == 1
}

// CursorPosition is the pointer's position on the screen image.
func CursorPosition() (int, int) {
	if win.w == nil || win.k == 0 {
		return 0, 0
	}
	x, y := win.w.GetCursorPos()
	s := Monitor().DeviceScaleFactor()
	return int((x*s - win.ox) / win.k), int((y*s - win.oy) / win.k)
}

// Wheel is the wheel movement since the last tick.
func Wheel() (float64, float64) { return in.tickWheel[0], in.tickWheel[1] }

// AppendGamepadIDs appends the connected gamepads.
func AppendGamepadIDs(ids []GamepadID) []GamepadID {
	for j, s := range in.pads {
		if s != nil {
			ids = append(ids, GamepadID(j))
		}
	}
	return ids
}

func IsStandardGamepadLayoutAvailable(id GamepadID) bool {
	return id >= 0 && int(id) < len(in.pads) && in.pads[id] != nil
}

func IsStandardGamepadButtonPressed(id GamepadID, b StandardGamepadButton) bool {
	return StandardGamepadButtonPressDuration(id, b) > 0
}

func StandardGamepadButtonPressDuration(id GamepadID, b StandardGamepadButton) int {
	if !IsStandardGamepadLayoutAvailable(id) || b < 0 || b >= padButtons {
		return 0
	}
	return in.pdur[id][b]
}

func StandardGamepadAxisValue(id GamepadID, a StandardGamepadAxis) float64 {
	if !IsStandardGamepadLayoutAvailable(id) || a < 0 || a > 3 {
		return 0
	}
	return float64(in.pads[id].Axes[a])
}

// AppendJustPressedTouchIDs: no touches on the desktop.
func AppendJustPressedTouchIDs(ids []TouchID) []TouchID { return ids }
func TouchPosition(TouchID) (int, int)                  { return 0, 0 }
