package engine

import "strings"

// keyWaiting = 0A40:02FD as Turbo Pascal's Crt has it: true while the
// scan code of an extended key is still to be read, else the BIOS check.
// keyWaiting = Turbo Pascal KeyPressed: a pending scan code counts.
func (e *Engine) keyWaiting() bool { return e.pendingScan != 0 || e.KeyPressed() }

// crtReadKey = 0A40:030F, Turbo Pascal's ReadKey: an extended key (zero
// ASCII) reads as 0, then its scan code on the next call. The convenience
// keys (F1, gamepad, mouse, extras.go and nav.go) are translated by
// Engine.ReadKey and never reach the game as such.
func (e *Engine) crtReadKey() byte {
	if e.pendingScan != 0 {
		c := e.pendingScan
		e.pendingScan = 0
		return c
	}
	k := e.ReadKey()
	if k&0xFF == 0 {
		e.pendingScan = byte(k >> 8)
		return 0
	}
	return byte(k)
}

// ReadKeyByte = 0899:0059: one key as the game reads it. Ctrl-S toggles the
// sound; with the STING parameter Ctrl-D toggles the debug display and
// Ctrl-C quits. After a non-zero key, keys typed ahead are drained and the
// last one wins.
func (e *Engine) ReadKeyByte() byte {
	k := e.crtReadKey()
	if k == 0x13 {
		e.soundToggle()
	}
	if e.Cfg.Param == "STING" {
		if k == 4 {
			e.debugShow ^= 1
		}
		if k == 3 {
			e.Exit() // 0899:0000
		}
	}
	if k != 0 {
		e.draining = true
		for e.keyWaiting() {
			k = e.crtReadKey()
		}
		e.draining = false
	}
	return k
}

// DisplaySpaces = 0709:05EB (x, y, colour, count): count blank cells.
func (e *Engine) DisplaySpaces(x, y, colour, count int) {
	if e.Trace != nil {
		e.Trace("0709:05EB", "DisplaySpaces", x, y, colour, count)
	}
	if x > lastCol || y > promptRow {
		return
	}
	e.displayCharN(x, y, colour, count, ' ', true)
}

// ClearRow24 = ovr26:06FB: blank the message/menu row.
func (e *Engine) ClearRow24() { e.DisplaySpaces(0, promptRow, colBlack, 40) }

// InputString = 0709:09E3 (prompt, colour, maxlen): line editor on row 24.
// Characters 0x20..0x7A are accepted while len < maxlen and echoed in white
// one cell right of the last; backspace erases; Enter or Esc ends; the
// result is upcased.
func (e *Engine) InputString(prompt string, colour, maxlen int) string {
	return strings.ToUpper(e.inputLine(prompt, colour, maxlen))
}

// inputLine is InputString keeping the case as typed (extra: character names).
func (e *Engine) inputLine(prompt string, colour, maxlen int) string {
	if len(prompt) > 40 {
		prompt = prompt[:40]
	}
	e.ClearRow24()
	e.DisplayString(0, promptRow, colour, prompt)
	e.ext.typing = true // hint line and mouse (nav.go)
	defer func() { e.ext.typing = false }()
	x := len(prompt)
	var s []byte
	for {
		k := e.ReadKeyByte()
		switch {
		case k >= 0x20 && k <= 0x7A:
			if len(s) < maxlen {
				s = append(s, k)
				x++
				e.DisplayString(x, promptRow, colWhite, string(k))
			}
		case k == 8:
			if len(s) > 0 {
				s = s[:len(s)-1]
				e.DisplaySpaces(x, promptRow, colBlack, 1)
				x--
			}
		}
		if k == 13 || k == 27 {
			break
		}
	}
	e.ClearRow24()
	return string(s)
}

// DisplayMessageWait = 0709:0D68 (str, colour): message on row 24 for
// msgDelay*225 ms, then cleared.
func (e *Engine) DisplayMessageWait(colour int, s string) {
	if e.Trace != nil {
		e.Trace("0709:0D68", "DisplayMessageWait", s, colour)
	}
	if len(s) > 40 {
		s = s[:40]
	}
	e.ClearRow24()
	e.DisplayString(0, promptRow, colour, s)
	e.Delay(int(e.msgDelay) * 225) // 0709:0DBE
	e.ClearRow24()
}
