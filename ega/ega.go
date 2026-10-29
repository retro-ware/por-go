// Package ega is the keyboard queue between the window and the engine,
// modelled on the BIOS services the DOS build used, with the pointer's text
// cell and the key hint.
package ega

import (
	"sync"

	"por/pal"
)

// Screen carries keys from the window to the engine goroutine.
type Screen struct {
	mu   sync.Mutex
	Keys chan uint16 // BIOS style: low byte ASCII, high byte scan code

	mouse int    // text cell under the pointer + 1, 0 outside the screen
	hint  string // key hint for the window's status line
}

// New creates the queue.
func New() *Screen { return &Screen{Keys: make(chan uint16, 64)} }

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
	if ok && cx >= 0 && cx < pal.W/8 && cy >= 0 && cy < pal.H/8 {
		s.mouse = cy*(pal.W/8) + cx + 1
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
	return (m - 1) % (pal.W / 8), (m - 1) / (pal.W / 8), true
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
