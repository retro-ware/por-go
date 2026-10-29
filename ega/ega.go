// Package ega is the 320x200 16-colour frame buffer the game draws into,
// plus the keyboard queue, modelled on the BIOS services the DOS build used.
package ega

import (
	"image"
	"image/color"

	"por/pal"
	"sync"
)

const (
	W = pal.W
	H = pal.H
)

// Overlay is a picture the window draws over the screen at its own
// resolution: the part Src of Img (empty: all of it) scaled into the
// screen rectangle At (the journal's scans).
type Overlay struct {
	Img     image.Image
	Src, At image.Rectangle
}

// Part is the rectangle of Img the overlay shows.
func (o Overlay) Part() image.Rectangle {
	if o.Src.Empty() {
		return o.Img.Bounds()
	}
	return o.Src
}

// Screen is the shared frame buffer. The engine goroutine writes it; the
// renderer snapshots it each frame under the lock.
type Screen struct {
	mu   sync.Mutex
	Pix  [W * H]uint8 // colour index per pixel
	Pal  [16]uint8    // palette register -> colour (SetPalette remaps)
	Keys chan uint16  // BIOS style: low byte ASCII, high byte scan code

	held  int          // Hold depth: while > 0 the window shows shown
	shown [W * H]uint8 // last released frame

	mouse int    // text cell under the pointer + 1, 0 outside the screen
	hint  string // key hint for the window's status line

	Overlays []Overlay // drawn over the picture; set and read under Lock
}

// Hold freezes what the window shows until the matching Release, so a
// picture built in several passes (the 3D view) appears at once like the
// DOS back buffer flip (0464:1B52).
func (s *Screen) Hold() {
	s.mu.Lock()
	if s.held == 0 {
		s.shown = s.Pix
	}
	s.held++
	s.mu.Unlock()
}

// Release ends a Hold.
func (s *Screen) Release() {
	s.mu.Lock()
	if s.held > 0 {
		s.held--
	}
	s.mu.Unlock()
}

// Visible returns the pixels the window should show; call under Lock.
func (s *Screen) Visible() *[W * H]uint8 {
	if s.held > 0 {
		return &s.shown
	}
	return &s.Pix
}

// New creates a screen with the identity palette.
func New() *Screen {
	s := &Screen{Keys: make(chan uint16, 64)}
	for i := range s.Pal {
		s.Pal[i] = uint8(i)
	}
	return s
}

// Lock guards Pix for the renderer; Unlock releases it.
func (s *Screen) Lock()   { s.mu.Lock() }
func (s *Screen) Unlock() { s.mu.Unlock() }

// Set writes one pixel; 17..pal.AmigaBase-1 are ignored (transparent).
func (s *Screen) Set(x, y int, c uint8) {
	if c == 16 { // dax.Clear: a masked-out pixel drawn plain
		c = 0
	}
	if (c < 16 || c >= pal.AmigaBase) && x >= 0 && x < W && y >= 0 && y < H {
		s.Pix[y*W+x] = c
	}
}

// Get reads one pixel.
func (s *Screen) Get(x, y int) uint8 {
	if x < 0 || x >= W || y < 0 || y >= H {
		return 0
	}
	return s.Pix[y*W+x]
}

// Fill paints a rectangle [x0,x1) x [y0,y1).
func (s *Screen) Fill(x0, y0, x1, y1 int, c uint8) {
	if c == 16 {
		c = 0
	}
	if c >= 16 && c < pal.AmigaBase { // transparent: Set ignores it
		return
	}
	x0, y0, x1, y1 = max(x0, 0), max(y0, 0), min(x1, W), min(y1, H)
	for y := y0; y < y1 && x0 < x1; y++ {
		row := s.Pix[y*W+x0 : y*W+x1]
		for i := range row {
			row[i] = c
		}
	}
}

// RGBA returns the displayed colour of palette register i.
func (s *Screen) RGBA(i uint8) color.RGBA { r := pal.Regs(s.Pal); return r.RGBA(i) }

// SetPalette remaps palette register index to EGA colour.
func (s *Screen) SetPalette(index, colour int) {
	s.Pal[index&15] = uint8(colour & 15)
}

// --- keyboard ---------------------------------------------------------------

// KeyPressed reports whether a key is waiting (INT 16h/01h).
func (s *Screen) KeyPressed() bool { return len(s.Keys) > 0 }

// PushKey enqueues a key from the window layer.
func (s *Screen) PushKey(k uint16) {
	select {
	case s.Keys <- k:
	default:
	}
}

// --- mouse and hints -------------------------------------------------------

// SetMouse records the text cell (40x25) under the pointer; ok false when
// the pointer is off the game picture.
func (s *Screen) SetMouse(cx, cy int, ok bool) {
	s.mu.Lock()
	s.mouse = 0
	if ok && cx >= 0 && cx < W/8 && cy >= 0 && cy < H/8 {
		s.mouse = cy*(W/8) + cx + 1
	}
	s.mu.Unlock()
}

// Mouse returns the cell under the pointer.
func (s *Screen) Mouse() (cx, cy int, ok bool) {
	s.mu.Lock()
	m := s.mouse
	s.mu.Unlock()
	if m == 0 {
		return 0, 0, false
	}
	return (m - 1) % (W / 8), (m - 1) / (W / 8), true
}

// SetHint sets the key hint the window shows under the picture.
func (s *Screen) SetHint(h string) {
	s.mu.Lock()
	s.hint = h
	s.mu.Unlock()
}

// Hint returns the key hint.
func (s *Screen) Hint() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hint
}
