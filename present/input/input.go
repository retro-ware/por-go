// Package input names what a key does. The binding table
// (present/remaster/bindings.go) gives every key code its Act by context,
// and the screens (remaster, battle) switch on the Act, never on the code.
package input

// Act is what a key does where the table binds it.
type Act uint8

const (
	None Act = iota

	// Menus (and the cursor keys everywhere)
	Up
	Down
	Left
	Right
	PgUp
	PgDn
	Home
	End
	Enter
	Space
	PadA // the pad's A: Enter, except where Enter alone opens a menu
	Esc
	Member1 // F1; Member1+i is F(i+1), up to F8
	_
	_
	_
	_
	_
	_
	_
	PrevMember // the pad's stick buttons
	NextMember
	More // a camp spell list: one more, one less
	Less

	// Adventuring
	Minimap
	Overview // the pad's X
	Map      // M, after the menu's hotkeys
	Party    // P, after the menu's hotkeys

	// Any time
	Help
	SaveLoad
	Sound
	Font
	FontPrev
	Backspace
	Delete

	// Combat
	CursorUp
	CursorDown
	CursorLeft
	CursorRight
	TurnLeft
	TurnRight
	Tab
	EndTurn
	Options // the pad's Start: the options, even while aiming
	Log
)

// Members is how many member keys there are (F1-F8).
const Members = 8

// Cursor: a is one of the cursor keys (arrows, PgUp/PgDn, Home/End).
func Cursor(a Act) bool { return a >= Up && a <= End }
