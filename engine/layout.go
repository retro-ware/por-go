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

// The 40x25 text screen.
const (
	textCols   = 40 // characters per line
	noteCols   = 36 // a note (say) fits the bottom line's box with a blank each side
	lastCol    = 39
	winRight   = 38 // inner right edge of the framed text window
	winBottom  = 22 // its inner bottom row
	msgTop     = 17 // first row of the message pane under the portrait
	messageRow = 23 // the bottom message line
	promptRow  = 24 // the menu and prompt line
)
