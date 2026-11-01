//go:build !darwin && !windows

package gpu

// readPads: no pads read on this platform yet.
// ponytail: Linux pads want evdev (/dev/input) and a mapping table; add when someone plays there
func readPads(*[maxPads]padState) {}
