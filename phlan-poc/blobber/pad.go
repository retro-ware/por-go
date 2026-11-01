package blobber

import (
	"gpu"
	"os"
)

// padKeys are the keys the host's gamepad sent the Esc menu this frame
// (View.PadKey), its repeat already applied.
var padKeys = map[gpu.Key]bool{}

// KeyJustPressed reports a key pressed this frame on the keyboard or a gamepad.
var shooting = os.Getenv("BLOBBER_SHOT") != "" // phlan-poc: screenshots ignore the keyboard

func KeyJustPressed(k gpu.Key) bool {
	return !shooting && gpu.IsKeyJustPressed(k) || padKeys[k]
}

// Repeats reports whether a key held d ticks repeats this tick; a host with its own
// key repeat sets it, so the Esc menu repeats at the same rate as the rest of the game.
var Repeats = func(d int) bool { return d > 20 && d%4 == 0 }

// KeyRepeated is KeyJustPressed with auto-repeat while held (Repeats), for moving through lists.
func KeyRepeated(k gpu.Key) bool {
	return KeyJustPressed(k) || Repeats(gpu.KeyPressDuration(k))
}
