package engine

import (
	"image"
	"strings"
)

// The game's text (extra): the engine draws no text. What its routines
// wrote is kept as typed text where it goes: the text pane's lines, the
// prompt row and the view window's text (opProtection's runes), published
// as View.Pane, View.Prompt and View.ViewText; the screens' own text is
// their View data (the status line, the roster, after-combat, the credits).

// textCell is a character the game wrote: upcased (DisplayString's way)
// or not, in colour on bg.
type textCell struct {
	set, upcase   bool
	c, colour, bg byte
}

// texts is the text the game wrote, by where it goes.
type texts struct {
	pane   [TextPaneRows][]textCell // the text pane's rows, from column 0 of the page
	prompt []textCell               // the prompt row
	view   [0xF][]textCell          // the view window's rows 1..15, from column 0
}

// row is the text row of the page's row y (nil: text written there is not kept).
func (t *texts) row(x, y int) []textCell {
	switch {
	case y == promptRow:
		if len(t.prompt) != textCols {
			t.prompt = make([]textCell, textCols)
		}
		return t.prompt
	case y >= msgTop && y <= paneBottom && x <= paneRight:
		r := &t.pane[y-msgTop]
		if *r == nil {
			*r = make([]textCell, paneRight+1)
		}
		return *r
	case y >= ViewWindow.Y1 && y <= ViewWindow.Y2 && x <= ViewWindow.X2:
		r := &t.view[y-1]
		if *r == nil {
			*r = make([]textCell, ViewWindow.X2+1)
		}
		return *r
	}
	return nil
}

// put writes ch at cell (x, y).
func (e *Engine) put(x, y, colour, bg int, ch byte, upcase bool) {
	if x < 0 || x > lastCol || y < 0 || y > promptRow {
		return
	}
	if r := e.pg.tx.row(x, y); x < len(r) {
		r[x] = textCell{set: true, upcase: upcase, c: ch, colour: byte(colour), bg: byte(bg)}
	}
}

// unwrite takes the text off the page rectangle r (pixels).
func (e *Engine) unwrite(r image.Rectangle) {
	r = r.Intersect(image.Rect(0, 0, textCols*8, textRows*8))
	if r.Empty() {
		return
	}
	x1, y1, x2, y2 := r.Min.X/8, r.Min.Y/8, (r.Max.X-1)/8, (r.Max.Y-1)/8
	for y := y1; y <= y2; y++ {
		row := e.pg.tx.row(x1, y)
		for x := x1; x <= x2 && x < len(row); x++ {
			row[x] = textCell{}
		}
	}
}

// The party panel's cells: the roster (or the minimap) and the status line.
var (
	rosterPanel = CellRect{0x11, 2, paneRight, 14}
	statusLine  = CellRect{0x11, 15, paneRight, 15}
)

// char = 0709:01DF display_char(upcase, ch, count, colour, y, x):
// ch at cell (x,y) in colour on black. Glyph index = upcase(ch) mod 0x40
// into 8X8D1 block 201 (the presenter's: Engine.FontGlyph).
func (e *Engine) char(x, y, colour int, ch byte, upcase bool) {
	e.put(x, y, colour, colBlack, ch, upcase)
}

// text = 0709:0626 displayString(x, y, colour, str): characters left to
// right, each upcased, no wrapping (x stops at lastCol).
func (e *Engine) text(x, y, colour int, s string) {
	for i := 0; i < len(s) && i < 255; i++ {
		e.char(x+i, y, colour, s[i], true)
	}
}

// blank = ClearCells (03F1:0137), ClearBox (0709:0DD3), 0709:00B7: the
// cells [x1..x2] x [y1..y2] black, their text gone.
func (e *Engine) blank(x1, y1, x2, y2 int) {
	e.fill(x1*8, y1*8, x2*8+8, y2*8+8, 0)
}

// frame = DrawBoxTitled (03F1:02E8) with DrawBorder (03F1:0000) style 0:
// the inside cleared, a border round it (View.Border's cells, drawn by the
// presenter), the title centred on row y1 in colour.
func (e *Engine) frame(x1, y1, x2, y2, colour int, title string) {
	if len(title) > 0x28 {
		title = title[:0x28]
	}
	e.blank(x1, y1, x2, y2)
	e.border(x1, y1, x2, y2, title, colour)
}

// border = DrawBorder (03F1:0000) style 0: a border round the cells
// x1..x2, y1..y2, with title (or none) centred on row y1 in colour.
func (e *Engine) border(x1, y1, x2, y2 int, title string, colour int) {
	if x1 <= 1 && y1 <= 1 { // a new page: the sheet's extras end with it
		e.newPage()
	}
	e.draw(layer{at: CellRect{x1, y1, x2, y2}.Framed().Pixels(), border: true, title: title, fill: byte(colour)})
}

// punct = DS:0D16 set: characters that end a word for textBlock.
func punct(c byte) bool { return strings.IndexByte("!,-.:;?", c) >= 0 }

// flow lays s out as 0709:0768 does in the box x1..x2, y1..y2 from the
// cursor (*col, *row): put gets each character's cell; when the box is
// full, full is called and the box starts over.
func flow(s string, x1, y1, x2, y2 int, col, row *int, put func(x, y int, c byte), full func()) {
	n := min(len(s), 255)
	if n == 0 {
		return
	}
	at := func(k int) byte { return s[k-1] } // Pascal 1-based
	i := 1
	// printSeg = 0709:06A7: s[i..j] at the cursor, i = j+1.
	printSeg := func(j int) {
		for ; i <= j; i++ {
			put(*col, *row, at(i)) // at once: the original's slow typing (DS:4A00) is left out
			*col++
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
		if j-i+*col > x2 {
			// (0919 also drops a trailing space when the word ends exactly
			// at x2, but that test can never hold after the > above.)
			*col = x1
			*row++
			for i < n && at(i) == ' ' { // 0709:0714
				i++
			}
			if *row > y2 && i < n {
				*col, *row = x1, y1
				full()
				printSeg(j)
			}
		} else {
			printSeg(j)
		}
		if i > n {
			break
		}
	}
	if *col > x2 {
		*col = x1
		*row++
	}
}

// Wrap is s word-wrapped as the game's text boxes do (0709:0768) between
// columns x1 and x2, from x1: the rows' text, each from column x1.
func Wrap(s string, x1, x2 int) []string {
	var rows [][]byte
	col, row := x1, 0
	flow(s, x1, 0, x2, 1<<30, &col, &row, func(x, y int, c byte) {
		for len(rows) <= y {
			rows = append(rows, nil)
		}
		for len(rows[y]) < x-x1 {
			rows[y] = append(rows[y], ' ')
		}
		rows[y] = append(rows[y], c)
	}, func() {})
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = string(r)
	}
	return out
}

// textBlock = 0709:0768 DisplayTextBlock (x1, y1, x2, y2, colour, clear,
// s): word wrapped text in the cell box, continuing at the text cursor
// (DS:5E24 x, DS:5E25 y) when it lies inside the box. When the box is full
// the player is asked to press a key and the box starts over.
func (e *Engine) textBlock(x1, y1, x2, y2, colour int, clear bool, s string) {
	if x1 > lastCol || y1 > promptRow || x2 > lastCol || y2 > lastCol {
		return
	}
	if e.msgCol < x1 || e.msgCol > x2 || e.msgRow < y1 || e.msgRow > y2 {
		e.msgCol, e.msgRow = x1, y1
	}
	if clear {
		e.blank(x1, y1, x2, y2)
		e.msgCol, e.msgRow = x1, y1
	}
	flow(s, x1, y1, x2, y2, &e.msgCol, &e.msgRow, func(x, y int, c byte) { e.char(x, y, colour, c, true) }, func() {
		e.PromptRow24(colLightMagenta, "Press any key to continue")
		e.ClearKeyboard()
		e.blank(x1, y1, x2, y2)
	})
}

// paneText is textBlock in the text pane.
func (e *Engine) paneText(colour int, clear bool, s string) {
	e.textBlock(1, msgTop, paneRight, paneBottom, colour, clear, s)
}

// clearPane blanks the text pane.
func (e *Engine) clearPane() { e.blank(1, msgTop, paneRight, paneBottom) }

// clearPrompt = ClearRow24 (ovr26:06FB): blank the message/menu row.
func (e *Engine) clearPrompt() { e.pg.tx.prompt = nil }

// PromptRow24 = 0709:0BF5: clear row 24, show, wait for a key.
func (e *Engine) PromptRow24(colour int, s string) {
	e.prompt(colour, s)
	e.pause(0, true)
}

// prompt shows s alone on the prompt row.
func (e *Engine) prompt(colour int, s string) {
	e.clearPrompt()
	e.text(0, promptRow, colour, s)
}
