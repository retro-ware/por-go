package engine

import (
	"strconv"
	"strings"
)

// DisplayChar = 0709:01DF display_char(upcase, ch, count, colour, y, x).
// The glyph is 8x8 at cell (x,y); set bits take the colour, clear bits are
// black (the EGA path writes every plane byte, so the cell is overwritten).
// Glyph index = upcase(ch) mod 0x40 into 8X8D1 block 201.
func (e *Engine) DisplayChar(x, y, colour int, ch byte, upcase bool) {
	e.displayCharBg(x, y, colour, colBlack, ch, upcase)
}

// displayCharBg is DisplayChar on a coloured background (the selection
// indicator of the menus; the original always draws on black).
func (e *Engine) displayCharBg(x, y, colour, bg int, ch byte, upcase bool) {
	e.drawChar(x, y, colour, bg, ch, upcase)
}

// displayCharN = 0709:01DF with a repeat count: ch drawn count times from x on.
// ponytail: cells past lastCol are dropped; the original writes on into the next row's video memory.
func (e *Engine) displayCharN(x, y, colour, count int, ch byte, upcase bool) {
	if x > lastCol || y > promptRow {
		return
	}
	for i := 0; i < count; i++ {
		e.drawChar(x+i, y, colour, 0, ch, upcase)
	}
}

func (e *Engine) drawChar(x, y, colour, bg int, ch byte, upcase bool) {
	if x > lastCol || y > promptRow || e.font == nil {
		return
	}
	e.S.Lock()
	defer e.S.Unlock()
	e.paintChar(x, y, colour, bg, ch, upcase)
}

// paintChar draws the glyph (fonts.go picks it) with the screen locked.
func (e *Engine) paintChar(x, y, colour, bg int, ch byte, upcase bool) {
	g, ok := e.glyph(ch, upcase)
	if !ok {
		return
	}
	e.ext.cells[y][x] = textCell{set: true, upcase: upcase, c: ch, colour: byte(colour), bg: byte(bg), glyph: g}
	for r := 0; r < 8; r++ {
		bits := g[r]
		for b := 0; b < 8; b++ {
			c := uint8(bg)
			if bits&(0x80>>b) != 0 {
				c = uint8(colour)
			}
			e.S.Set(x*8+b, y*8+r, c)
		}
	}
}

// DisplayString = 0709:0626 displayString(x, y, colour, str): characters
// are drawn left to right, each upcased, no wrapping (x stops at lastCol).
func (e *Engine) DisplayString(x, y, colour int, s string) {
	if x > lastCol || y > lastCol {
		return
	}
	for i := 0; i < len(s) && i < 255; i++ {
		e.DisplayChar(x+i, y, colour, s[i], true)
	}
}

// ClearCells = 03F1:0137: fill cells [x1..x2] x [y1..y2] with black.
func (e *Engine) ClearCells(x1, y1, x2, y2 int) {
	e.fillCells(x1, y1, x2, y2)
}

// fillCells = 0709:00B7 (x1, y1, x2, y2, page, 0) for both EGA pages.
func (e *Engine) fillCells(x1, y1, x2, y2 int) {
	e.S.Lock()
	e.fill(x1*8, y1*8, x2*8+8, y2*8+8, 0)
	e.S.Unlock()
}

// DrawBorder = 03F1:0000: draw a frame around the cell rectangle using
// symbols 0x114+style*3 (+0 corner, +1 vertical, +2 horizontal).
func (e *Engine) DrawBorder(x1, y1, x2, y2, style int) {
	x1, y1, x2, y2 = x1-1, y1-1, x2+1, y2+1
	base := 0x114 + style*3
	e.Put8x8Symbol(x1, y1, base)
	e.Put8x8Symbol(x2, y1, base)
	e.Put8x8Symbol(x1, y2, base)
	e.Put8x8Symbol(x2, y2, base)
	for x := x1 + 1; x <= x2-1; x++ {
		e.Put8x8Symbol(x, y1, base+2)
		e.Put8x8Symbol(x, y2, base+2)
	}
	for y := y1 + 1; y <= y2-1; y++ {
		e.Put8x8Symbol(x1, y, base+1)
		e.Put8x8Symbol(x2, y, base+1)
	}
}

// DrawBoxTitled = 03F1:02E8: clear the interior, draw the border, then
// centre the title on row y1 in the given colour.
func (e *Engine) DrawBoxTitled(x1, y1, x2, y2, style, colour int, title string) {
	if len(title) > 0x28 {
		title = title[:0x28]
	}
	e.ClearCells(x1, y1, x2, y2)
	e.DrawBorder(x1, y1, x2, y2, style)
	if title != "" {
		e.DisplayString((x1+x2-len(title))/2, y1, colour, title)
	}
}

// punct = DS:0D16 set: characters that end a word for DisplayTextBlock.
func punct(c byte) bool { return strings.IndexByte("!,-.:;?", c) >= 0 }

// DisplayTextBlock = 0709:0768 (x1, y1, x2, y2, colour, clear, s): word
// wrapped text in the cell box, continuing at the text cursor (DS:5E24 x,
// DS:5E25 y) when it lies inside the box. When the box is full the player
// is asked to press a key and the box starts over.
func (e *Engine) DisplayTextBlock(x1, y1, x2, y2, colour int, clear bool, s string) {
	if x1 > lastCol || y1 > promptRow || x2 > lastCol || y2 > lastCol {
		return
	}
	if e.msgCol < x1 || e.msgCol > x2 || e.msgRow < y1 || e.msgRow > y2 {
		e.msgCol, e.msgRow = x1, y1
	}
	if clear {
		e.ClearBox(x1, y1, x2, y2)
		e.msgCol, e.msgRow = x1, y1
	}
	n := len(s)
	if n > 255 {
		n = 255
	}
	if n == 0 {
		return
	}
	at := func(k int) byte { return s[k-1] } // Pascal 1-based
	i := 1
	// printSeg = 0709:06A7: s[i..j] at the cursor, i = j+1.
	printSeg := func(j int) {
		for ; i <= j; i++ {
			e.DisplayChar(e.msgCol, e.msgRow, colour, at(i), true)
			if e.slowText && !e.skipText() {
				e.Delay(int(e.msgDelay) * 10)
			}
			e.msgCol++
		}
	}
	for {
		j := i
		for j < n && punct(at(j)) {
			j++
		}
		for j < n && !punct(at(j)) && at(j) != ' ' {
			j++
		}
		if at(j) != ' ' {
			for j+1 < n && punct(at(j+1)) {
				j++
			}
		}
		if j-i+e.msgCol > x2 {
			// (0919 also drops a trailing space when the word ends exactly
			// at x2, but that test can never hold after the > above.)
			e.msgCol = x1
			e.msgRow++
			for i < n && at(i) == ' ' { // 0709:0714
				i++
			}
			if e.msgRow > y2 && i < n {
				e.msgCol, e.msgRow = x1, y1
				e.PromptRow24(colLightMagenta, "Press any key to continue")
				e.ClearKeyboard()
				e.ClearBox(x1, y1, x2, y2)
				printSeg(j)
			}
		} else {
			printSeg(j)
		}
		if i > n {
			break
		}
	}
	if e.msgCol > x2 {
		e.msgCol = x1
		e.msgRow++
	}
}

// ClearBox = 0709:0DD3 (x1, y1, x2, y2): blank the cell rectangle.
func (e *Engine) ClearBox(x1, y1, x2, y2 int) {
	e.fillCells(x1, y1, x2, y2)
}

// InputNumber = 0709:0B66 (prompt, colour): a 6-character line read until
// it parses as 0..65536; the low word is returned.
func (e *Engine) InputNumber(prompt string, colour int) uint16 {
	for {
		s := e.InputString(prompt, colour, 6)
		if n, err := strconv.ParseInt(strings.TrimLeft(s, " "), 10, 32); err == nil && n >= 0 && n <= 0x10000 {
			return uint16(n)
		}
	}
}

// skipText (extra): a key pressed while text is
// printed slowly shows the rest at once; the key is used up.
func (e *Engine) skipText() bool {
	if e.msgDelay == 0 {
		return false
	}
	if !e.ext.skip && e.KeyPressed() {
		e.ext.ahead = e.ext.ahead[1:]
		e.ext.skip = true
	}
	return e.ext.skip
}
