package blobber

// Gamepad support: every connected standard-layout gamepad presses the keyboard keys the game
// already reads, so all screens work without knowing about pads. Read keys through KeyJustPressed
// and KeyPressed instead of inpututil/ebiten directly.
//
//	d-pad / left stick   arrows: walk and turn, move through menus
//	LB / RB              A / D: strafe; Page Up / Down in windows: previous / next character
//	A                    F (use) in the dungeon; Enter, Space in menus and windows
//	B                    Esc
//	X                    Space: wait a turn
//	Y                    I: inventory
//	LT / RT              C: spellbook, J: journal
//	Back / Start         M: map, Esc: menu

import (
	"gpu"
	"os"
)

// padHeld are the keys the pads hold; padJust those whose button went down this frame. A press
// counts only when the physical button goes down: a button held across a change of context (A
// opening a window that A then confirms in) must not fire again.
var (
	padHeld, padJust = map[gpu.Key]bool{}, map[gpu.Key]bool{}
	padWas           = map[[2]int]bool{} // gamepad id, button (or stick direction 100+) held last frame
	padDur           = map[gpu.Key]int{} // frames each pad key has been held
)

// pollPads works out which keys the pads hold this frame; context is where input goes:
// 'w' the dungeon, anything else a menu, window or prompt.
func pollPads(context byte) {
	padHeld, padJust = map[gpu.Key]bool{}, map[gpu.Key]bool{}
	was := padWas
	padWas = map[[2]int]bool{}
	var id gpu.GamepadID
	// src names the physical input behind the next hold, for the down-this-frame test
	hold := func(src int, on bool, ks ...gpu.Key) {
		if !on {
			return
		}
		key := [2]int{int(id), src}
		padWas[key] = true
		for _, k := range ks {
			padHeld[k] = true
			if !was[key] {
				padJust[k] = true
			}
		}
	}
	for _, id = range gpu.AppendGamepadIDs(nil) {
		if !gpu.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		b := func(s gpu.StandardGamepadButton, ks ...gpu.Key) {
			hold(int(s), gpu.IsStandardGamepadButtonPressed(id, s), ks...)
		}
		x := gpu.StandardGamepadAxisValue(id, gpu.StandardGamepadAxisLeftStickHorizontal)
		y := gpu.StandardGamepadAxisValue(id, gpu.StandardGamepadAxisLeftStickVertical)
		b(gpu.StandardGamepadButtonLeftTop, gpu.KeyUp)
		b(gpu.StandardGamepadButtonLeftBottom, gpu.KeyDown)
		b(gpu.StandardGamepadButtonLeftLeft, gpu.KeyLeft)
		b(gpu.StandardGamepadButtonLeftRight, gpu.KeyRight)
		hold(100, y < -0.5, gpu.KeyUp)
		hold(101, y > 0.5, gpu.KeyDown)
		hold(102, x < -0.5, gpu.KeyLeft)
		hold(103, x > 0.5, gpu.KeyRight)
		b(gpu.StandardGamepadButtonCenterRight, gpu.KeyEscape)
		switch context {
		case 'w':
			b(gpu.StandardGamepadButtonFrontTopLeft, gpu.KeyA)
			b(gpu.StandardGamepadButtonFrontTopRight, gpu.KeyD)
			b(gpu.StandardGamepadButtonRightBottom, gpu.KeyF)
			b(gpu.StandardGamepadButtonRightLeft, gpu.KeySpace)
			b(gpu.StandardGamepadButtonRightTop, gpu.KeyI)
			b(gpu.StandardGamepadButtonFrontBottomLeft, gpu.KeyC)
			b(gpu.StandardGamepadButtonFrontBottomRight, gpu.KeyJ)
			b(gpu.StandardGamepadButtonCenterLeft, gpu.KeyM)
		default:
			b(gpu.StandardGamepadButtonRightBottom, gpu.KeyEnter, gpu.KeySpace)
			b(gpu.StandardGamepadButtonRightRight, gpu.KeyEscape)
			b(gpu.StandardGamepadButtonCenterLeft, gpu.KeyM) // closes the map
			b(gpu.StandardGamepadButtonFrontTopLeft, gpu.KeyPageUp)
			b(gpu.StandardGamepadButtonFrontTopRight, gpu.KeyPageDown)
		}
	}
	for k := range padDur {
		if !padHeld[k] {
			delete(padDur, k)
		}
	}
	for k := range padHeld {
		padDur[k]++
	}
}

// KeyJustPressed reports a key pressed this frame on the keyboard or a gamepad.
var shooting = os.Getenv("BLOBBER_SHOT") != "" // phlan-poc: screenshots ignore the keyboard

func KeyJustPressed(k gpu.Key) bool {
	return !shooting && gpu.IsKeyJustPressed(k) || padJust[k]
}

// KeyRepeated is KeyJustPressed with auto-repeat while held, for moving through lists.
func KeyRepeated(k gpu.Key) bool {
	rep := func(d int) bool { return d > 20 && d%4 == 0 }
	return KeyJustPressed(k) || rep(gpu.KeyPressDuration(k)) || rep(padDur[k])
}

// KeyPressed reports a key held on the keyboard or a gamepad.
func KeyPressed(k gpu.Key) bool { return !shooting && gpu.IsKeyPressed(k) || padHeld[k] }
