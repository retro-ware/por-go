package engine

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"por/dax"
)

// Trace mode: the Go side of the emulator comparison (por-go/decomp/emu).
// The emulator runs the original with the hardware primitives intercepted
// and appends one record per call; the hooks below emit the same records
// from the Go functions that port those primitives, keyed by the original
// address. Records are (addr, name, args...) with the same arg order and
// encoding as decomp/emu/prims.py.
//
// e.Trace is nil in normal play: every hook site tests it first, so the
// cost is one nil check and no argument boxing.
//
// A trace run also replaces the clock and keyboard with the emulator's
// models (StartTrace): virtual time advances only by Delay(ms) and by 10 ms
// per Ticks() call; a scripted key becomes visible to KeyPressed on the
// poll after one that saw none; ReadKey with an empty script, or too many
// idle polls, ends the run by panicking with TraceDone.

// TraceFunc receives one trace record: the original's address, the
// primitive's name and its arguments.
type TraceFunc func(addr, name string, args ...any)

// TraceDone is the panic value that ends a trace run; Reason says why.
type TraceDone struct{ Reason string }

type traceState struct {
	keys  []uint16
	ready bool
	idle  int
	ms    int64
}

const traceMaxIdle = 200 // = emu.Machine.max_idle

// StartTrace switches the engine to trace mode with the given seed and key script.
func (e *Engine) StartTrace(fn TraceFunc, seed uint32, keys []uint16) {
	e.Trace = fn
	e.FixedSeed = &seed
	e.tr = &traceState{keys: append([]uint16(nil), keys...)}
}

// TraceMS is the virtual clock in milliseconds.
func (e *Engine) TraceMS() int64 { return e.tr.ms }

// TraceKeysLeft is the number of unread scripted keys.
func (e *Engine) TraceKeysLeft() int { return len(e.tr.keys) }

func (t *traceState) keyPressed(e *Engine) bool {
	if len(t.keys) > 0 && t.ready {
		e.Trace("INT16", "KeyPressed", 1)
		return true
	}
	e.Trace("INT16", "KeyPressed", 0)
	if len(t.keys) > 0 {
		t.ready = true
	} else if t.idle++; t.idle > traceMaxIdle {
		panic(TraceDone{fmt.Sprintf("key script exhausted (KeyPressed polled %d times)", t.idle)})
	}
	return false
}

func (t *traceState) readKey(e *Engine) uint16 {
	if len(t.keys) == 0 {
		panic(TraceDone{"key script exhausted (ReadKey)"})
	}
	k := t.keys[0]
	t.keys = t.keys[1:]
	t.ready, t.idle = false, 0
	if k >= KeyMouse { // (extra) a scripted click goes through nav.go as in a window
		if k = e.translate(k); k == 0 {
			k = keyNone
		}
	}
	e.Trace("INT16", "ReadKey", int(k))
	return k
}

// picID = prims.pic_id: "HxW:" + md5 of the frame's colour indices (one
// byte per pixel, rows top to bottom), first 10 hex digits.
func picID(im *dax.Image, frame int) string {
	if im == nil {
		return "nil"
	}
	if frame < 0 || frame >= im.Count {
		return fmt.Sprintf("%dx%d:?", im.Height, im.Width*8)
	}
	px := append([]byte(nil), im.Frame(frame)...)
	for i := range px {
		px[i] &= 15 // dax.Clear reads as 0, as in the original's colour planes
	}
	s := md5.Sum(px)
	return fmt.Sprintf("%dx%d:%s", im.Height, im.Width*8, hex.EncodeToString(s[:])[:10])
}

// pageSet mirrors a write to the original's drawing page DS:4A20 (shown at
// screen pixel 8,8; wider in combat), so Flip can hash the page as the
// emulator does and sprites can restore what they hid. It keeps the whole
// pixel: Amiga pixels (ega.AmigaBase up) lost their palette slot to & 15,
// and the restored floor came back EGA light red.
func (e *Engine) pageSet(x, y int, c uint8) {
	if x >= 0 && y >= 0 && x < pageStride && y < 168 {
		e.page[y*pageStride+x] = c
	}
}

// Flip = 0464:1B52: show the drawing page. The port draws straight to the
// shown screen (Hold/Release frame the 3D view), so only a trace run does
// anything: it records the hash of the drawing page.
func (e *Engine) Flip() {
	if e.Trace != nil {
		e.flips++
		s := md5.Sum(e.page[:])
		e.Trace("0464:1B52", "Flip", "168x168:"+hex.EncodeToString(s[:])[:10])
	}
}

// SetPaletteColor = 0464:2A0D (colour, index): EGA palette register.
func (e *Engine) SetPaletteColor(colour, index int) {
	if e.Trace != nil {
		e.Trace("0464:2A0D", "SetPaletteColor", index, colour) // Pascal order: (index, colour)
	}
	e.S.SetPalette(index, colour)
}

// TraceState is the heap records in the layout of the emulator's dump
// (emu.REGIONS): hex of game, area, aux, ecl and each party member.
func (e *Engine) TraceState() map[string]string {
	area := e.area // the original's record: the walls as last saved or loaded, and the animation flag
	area.WallsA, area.WallsB = e.fileWallsA, e.fileWallsB
	area.WallsB[2] = e.viewAnim
	m := map[string]string{
		"game": hex.EncodeToString(recordBytes(&e.game)), "area": hex.EncodeToString(recordBytes(&area)),
		"aux": hex.EncodeToString(recordBytes(&e.aux)), "ecl": hex.EncodeToString(e.ecl),
	}
	i := 0
	for _, c := range e.party {
		if c != e.tempSel {
			m[fmt.Sprintf("char%d", i)] = hex.EncodeToString(c.record())
			i++
		}
	}
	if len(e.party) > 0 {
		m["selected_char"] = hex.EncodeToString(e.selected().record())
	}
	return m
}
