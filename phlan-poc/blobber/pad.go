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
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"os"
)

// padHeld are the keys the pads hold; padJust those whose button went down this frame. A press
// counts only when the physical button goes down: a button held across a change of context (A
// opening a window that A then confirms in) must not fire again.
var (
	padHeld, padJust = map[ebiten.Key]bool{}, map[ebiten.Key]bool{}
	padWas           = map[[2]int]bool{}    // gamepad id, button (or stick direction 100+) held last frame
	padDur           = map[ebiten.Key]int{} // frames each pad key has been held
)

// pollPads works out which keys the pads hold this frame; context is where input goes:
// 'w' the dungeon, anything else a menu, window or prompt.
func pollPads(context byte) {
	padHeld, padJust = map[ebiten.Key]bool{}, map[ebiten.Key]bool{}
	was := padWas
	padWas = map[[2]int]bool{}
	var id ebiten.GamepadID
	// src names the physical input behind the next hold, for the down-this-frame test
	hold := func(src int, on bool, ks ...ebiten.Key) {
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
	for _, id = range ebiten.AppendGamepadIDs(nil) {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		b := func(s ebiten.StandardGamepadButton, ks ...ebiten.Key) {
			hold(int(s), ebiten.IsStandardGamepadButtonPressed(id, s), ks...)
		}
		x := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		y := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		b(ebiten.StandardGamepadButtonLeftTop, ebiten.KeyUp)
		b(ebiten.StandardGamepadButtonLeftBottom, ebiten.KeyDown)
		b(ebiten.StandardGamepadButtonLeftLeft, ebiten.KeyLeft)
		b(ebiten.StandardGamepadButtonLeftRight, ebiten.KeyRight)
		hold(100, y < -0.5, ebiten.KeyUp)
		hold(101, y > 0.5, ebiten.KeyDown)
		hold(102, x < -0.5, ebiten.KeyLeft)
		hold(103, x > 0.5, ebiten.KeyRight)
		b(ebiten.StandardGamepadButtonCenterRight, ebiten.KeyEscape)
		switch context {
		case 'w':
			b(ebiten.StandardGamepadButtonFrontTopLeft, ebiten.KeyA)
			b(ebiten.StandardGamepadButtonFrontTopRight, ebiten.KeyD)
			b(ebiten.StandardGamepadButtonRightBottom, ebiten.KeyF)
			b(ebiten.StandardGamepadButtonRightLeft, ebiten.KeySpace)
			b(ebiten.StandardGamepadButtonRightTop, ebiten.KeyI)
			b(ebiten.StandardGamepadButtonFrontBottomLeft, ebiten.KeyC)
			b(ebiten.StandardGamepadButtonFrontBottomRight, ebiten.KeyJ)
			b(ebiten.StandardGamepadButtonCenterLeft, ebiten.KeyM)
		default:
			b(ebiten.StandardGamepadButtonRightBottom, ebiten.KeyEnter, ebiten.KeySpace)
			b(ebiten.StandardGamepadButtonRightRight, ebiten.KeyEscape)
			b(ebiten.StandardGamepadButtonCenterLeft, ebiten.KeyM) // closes the map
			b(ebiten.StandardGamepadButtonFrontTopLeft, ebiten.KeyPageUp)
			b(ebiten.StandardGamepadButtonFrontTopRight, ebiten.KeyPageDown)
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

func KeyJustPressed(k ebiten.Key) bool {
	return !shooting && inpututil.IsKeyJustPressed(k) || padJust[k]
}

// KeyRepeated is KeyJustPressed with auto-repeat while held, for moving through lists.
func KeyRepeated(k ebiten.Key) bool {
	rep := func(d int) bool { return d > 20 && d%4 == 0 }
	return KeyJustPressed(k) || rep(inpututil.KeyPressDuration(k)) || rep(padDur[k])
}

// KeyPressed reports a key held on the keyboard or a gamepad.
func KeyPressed(k ebiten.Key) bool { return !shooting && ebiten.IsKeyPressed(k) || padHeld[k] }
