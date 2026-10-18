// Package ega is the 320x200 16-colour frame buffer the game draws into,
// plus the keyboard queue, modelled on the BIOS services the DOS build used.
package ega

import (
	"image"
	"image/color"
	"sync"
	"sync/atomic"
	"time"
)

const (
	W = 320
	H = 200
)

// Palette is the standard 16-colour EGA/CGA palette (RGB).
var Palette = [16][3]uint8{
	{0, 0, 0}, {0, 0, 173}, {0, 173, 0}, {0, 173, 173},
	{173, 0, 0}, {173, 0, 173}, {173, 82, 0}, {173, 173, 173},
	{82, 82, 82}, {82, 82, 255}, {82, 255, 82}, {82, 255, 255},
	{255, 82, 82}, {255, 82, 255}, {255, 255, 82}, {255, 255, 255},
}

// Amiga holds the Amiga Pool of Radiance palettes (12-bit RGB, from the
// program's data at 0x54080, 0x54040 and 0x54020). Pixel values from AmigaBase up
// are colour AmigaBase+16*slot+index of these, drawn in their own RGB.
var Amiga = [3][16]uint16{
	AmigaView:     {0x000, 0x124, 0x953, 0xb97, 0xecb, 0x710, 0xa42, 0xdb0, 0xff5, 0x074, 0x053, 0x369, 0x555, 0x777, 0xaaa, 0xfff},
	AmigaPortrait: {0x000, 0xdb9, 0xb97, 0x964, 0x521, 0x742, 0xa84, 0xcb6, 0xff9, 0x007, 0x44b, 0x444, 0x555, 0x777, 0xaaa, 0xfff},
	AmigaTerrain:  {0x000, 0x06b, 0x096, 0x0aa, 0x000, 0x940, 0xb44, 0xfaa, 0xfe9, 0x59f, 0x0d8, 0x5ff, 0x555, 0x777, 0xaaa, 0xfff},
}

// Amiga palette slots: PIC/FINAL pictures; portraits; the area map,
// combat ground tiles and combat icons.
const (
	AmigaView = iota
	AmigaPortrait
	AmigaTerrain
	AmigaBase = 32
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

	state atomic.Int32 // what the engine goroutine does: Working, Waiting, Pausing

	Overlays []Overlay // drawn over the picture; set and read under Lock
}

// Engine states (SetState): the window draws a key's result once the engine
// is done with it (Settled).
const (
	Working = iota // handling input
	Waiting        // waiting for input
	Pausing        // a deliberate delay (Delay)
)

// SetState records what the engine does now (no screen: tests).
func (s *Screen) SetState(st int32) {
	if s != nil {
		s.state.Store(st)
	}
}

// Settled reports whether the engine has taken the keys sent and finished with them, or
// pauses on purpose; the picture is worth drawing.
func (s *Screen) Settled() bool {
	st := s.state.Load()
	return st == Pausing || st == Waiting && len(s.Keys) == 0
}

// Idle reports whether the engine waits for input and none is queued (held keys repeat
// only then, so they never pile up).
func (s *Screen) Idle() bool { return s.state.Load() == Waiting && len(s.Keys) == 0 }

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

// Set writes one pixel; 17..AmigaBase-1 are ignored (transparent).
func (s *Screen) Set(x, y int, c uint8) {
	if c == 16 { // dax.Clear: a masked-out pixel drawn plain
		c = 0
	}
	if (c < 16 || c >= AmigaBase) && x >= 0 && x < W && y >= 0 && y < H {
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
	if c >= 16 && c < AmigaBase { // transparent: Set ignores it
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
func (s *Screen) RGBA(i uint8) color.RGBA {
	if i >= AmigaBase {
		c := Amiga[int(i-AmigaBase)/16%len(Amiga)][i%16]
		return saturate(float64(c>>8&15)*17, float64(c>>4&15)*17, float64(c&15)*17)
	}
	p := Palette[s.Pal[i&15]&15]
	return color.RGBA{p[0], p[1], p[2], 255}
}

// AmigaSaturation scales the Amiga colours' distance from grey, to sit
// closer to the vivid EGA set (1 = the Amiga's own values).
var AmigaSaturation = 1.4

func saturate(r, g, b float64) color.RGBA {
	l := 0.299*r + 0.587*g + 0.114*b
	f := func(v float64) uint8 { return uint8(max(0, min(255, l+(v-l)*AmigaSaturation)) + 0.5) }
	return color.RGBA{f(r), f(g), f(b), 255}
}

// SetPalette remaps palette register index to EGA colour.
func (s *Screen) SetPalette(index, colour int) {
	s.Pal[index&15] = uint8(colour & 15)
}

// --- keyboard ---------------------------------------------------------------

// KeyPressed reports whether a key is waiting (INT 16h/01h).
func (s *Screen) KeyPressed() bool { return len(s.Keys) > 0 }

// ReadKey blocks for the next key (INT 16h/00h).
func (s *Screen) ReadKey() uint16 {
	s.state.Store(Waiting)
	k := <-s.Keys
	s.state.Store(Working)
	return k
}

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

// Delay sleeps; the engine calls this instead of busy-waiting on the timer.
func Delay(ms int) {
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}
