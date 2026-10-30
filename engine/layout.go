package engine

// The 16 EGA text colours the game draws with.
const (
	colBlack = iota
	colBlue
	colGreen
	colCyan
	colRed
	colMagenta
	colBrown
	colLightGrey
	colDarkGrey
	colLightBlue
	colLightGreen
	colLightCyan
	colLightRed
	colLightMagenta
	colYellow
	colWhite
)

// The text screen: the original's 40x25, the floor; a larger window gives
// more cells (resizePage), and every screen lays out to them (textCols,
// textRows and the rows and columns derived from them). The 3D screen's
// boxes (ViewWindow, TextPane) keep their places: the remaster draws that
// screen itself.
const (
	minCols  = 40
	minRows  = 25
	noteCols = 36 // a note (say) fits the bottom line's box with a blank each side
	msgTop   = 17 // first row of the message pane under the portrait
	// The 3D screen's text pane and party panel keep the original's edges
	// whatever the page's size: the remaster shows them as a pop-up and
	// cards, and a clear past the fixed pane would tear the main frame
	// (page.go draw) and bring the page up in its place.
	paneRight  = 0x26
	paneBottom = 0x16
)

// ponytail: package-level page size, one Engine per process; move onto
// Engine if two of different sizes ever share one.
var (
	textCols   = minCols // characters per line
	textRows   = minRows
	lastCol    = textCols - 1
	winRight   = textCols - 2 // inner right edge of the framed text window
	winBottom  = textRows - 3 // its inner bottom row
	messageRow = textRows - 2 // the bottom message line
	promptRow  = textRows - 1 // the menu and prompt line
)

// setPageSize sets the text screen's size in cells (never under 40x25).
func setPageSize(cols, rows int) {
	textCols, textRows = max(cols, minCols), max(rows, minRows)
	lastCol, winRight, winBottom, messageRow, promptRow = textCols-1, textCols-2, textRows-3, textRows-2, textRows-1
}

// newCells is an empty text cell grid of the page's size.
func newCells() [][]textCell {
	all := make([]textCell, textRows*textCols)
	cells := make([][]textCell, textRows)
	for y := range cells {
		cells[y] = all[y*textCols : (y+1)*textCols]
	}
	return cells
}

// cells is the text cell grid, made at the page's size on first use.
func (e *Engine) cells() [][]textCell {
	if len(e.ext.cells) != textRows || len(e.ext.cells[0]) != textCols {
		e.ext.cells = newCells()
	}
	return e.ext.cells
}

// cellsCopy is a copy of the text cells (saveScreen, the shops' greeting).
func (e *Engine) cellsCopy() [][]textCell {
	c := newCells()
	for y, row := range e.cells() {
		copy(c[y], row)
	}
	return c
}
