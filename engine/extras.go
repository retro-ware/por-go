package engine

import (
	"strconv"
	"strings"
	"time"
)

// Convenience features that are not in the DOS game. The port is a
// modernized build: they are always on (there is no mode with the
// original's exact behaviour). Most are reached through keys the original
// command bar ignores:
//
//	Tab  minimap of the whole area over the roster panel, where the game's
//	     own Area map was allowed; it replaces Area, which is not offered
//	M    the full-screen map, also a click on the minimap: labels (fullmap.go)
//	     and Walk, which takes the party to a square (walk.go)
//	Down a step backwards (the original turns round)
//	P    party overview: hit points, armour class and status of everyone
//	?      keyboard help, at any prompt
//	F1-F8  the inventory of party member 1-8 (command bar, camp, View);
//	       P switches between it and the View page
//	Ctrl-F next text font (Ctrl-Shift-F the one before; fonts.go)
//	Q    quickstart on the empty party menu: a ready-made party of six
//	F9   named save slots: save and load at any time (slots.go)
//
// One set of navigation keys for every menu, the mouse and the hint line
// below the picture are in nav.go.
//
// Menus also get a selection indicator: the highlighted item sits on a
// coloured background (the original only recolours its text, which is the
// colour the hotkey letters already have).

// Virtual keys sent by the gamepad (scan codes no PC keyboard produces).
const (
	KeyPadSelect   = 0xE000 // choose the highlighted menu item (Enter elsewhere)
	KeyPadMinimap  = 0xE100 // as Tab
	KeyPadOverview = 0xE200 // as P
	KeyPadUp       = 0xE300 // d-pad: walk in the 3D view, move the selection in menus
	KeyPadDown     = 0xE400
	KeyPadLeft     = 0xE500
	KeyPadRight    = 0xE600
	KeyF1          = 0x3B00 // F1; F2..F8 follow at +0x100 (BIOS scan codes the original never reads)
	KeyF9          = 0x4300 // save / load (slots.go)
)

// padArrow is the BIOS arrow key for each d-pad direction.
var padArrow = map[uint16]uint16{KeyPadUp: 0x4800, KeyPadDown: 0x5000, KeyPadLeft: 0x4B00, KeyPadRight: 0x4D00}

// padKey turns a virtual gamepad key into what the game reads, or 0 to
// drop it.
func (e *Engine) padKey(k uint16) uint16 {
	switch k {
	case KeyPadSelect:
		if e.ext.padHot != 0 {
			return uint16(e.ext.padHot)
		}
		return 13
	case KeyPadMinimap:
		e.extraKey(9)
	case KeyPadOverview:
		e.extraKey('P')
	case KeyPadUp, KeyPadDown, KeyPadLeft, KeyPadRight:
		back := k == KeyPadUp || k == KeyPadLeft
		switch {
		case e.ext.inCommand || e.ext.menu == 0:
			return padArrow[k] // walking, or a screen that reads the arrows itself
		case e.ext.list && (k == KeyPadUp || k == KeyPadDown):
			if back {
				return 0x4700 // Home: previous row
			}
			return 0x4F00 // End: next row
		case back:
			return ','
		default:
			return '.'
		}
	}
	return 0
}

// extras holds the convenience-feature state.
type extras struct {
	inCommand bool                // the adventuring command bar is waiting
	padHot    byte                // hotkey of the highlighted menu-bar item
	minimap   bool                // roster panel shows the minimap
	onSelect  func(hot byte)      // menus drawn elsewhere (party menu) show the selection
	menu      int                 // DisplayInput calls in progress
	list      bool                // the menu belongs to a scrolling list
	choosing  bool                // SelectCharacter is waiting: highlight the roster entry
	seen      map[int]*[256]uint8 // per area: 1 seen, 2 visited
	quick     *quickPick          // CreateCharacter answers its own menus (quickstart)
	minStats  [6]int              // "Minimal stats": per-stat floor, STR as st*100+str00
	guarded   bool                // slots.go: a load can unwind to adventure
	inGame    bool                // slots.go: GameLoop has started: saving allowed
	aoe       int                 // aoe.go: radius+1 of the spell being aimed, 0 none

	// nav.go: keys and mouse
	ahead        []uint16                // translated keys not yet read
	nav          navCtx                  // the waiting menu
	regions      []region                // clickable parts of the screen
	mouseAt      int                     // cell under the pointer + 1 at the last poll
	jump         *[2]int8                // aimManual: a clicked square for the cursor
	autoPath     []int                   // the main combat menu's clicked square: moveAttack walks there, then hands back
	autoAim      *Character              // the main combat menu's clicked foe: aim at it, attack it if it can be
	aimAfterWalk *Character              // the clicked foe autoPath walks up to: attacked when the menu comes back
	autoJumped   bool                    // autoAim's jump to it was tried
	raw          string                  // the screen reads the arrows itself; its hint line
	typing       bool                    // InputString is waiting
	modal        int                     // help or overview shown: any key closes
	hiddenKeys   string                  // DisplayInput also takes these keys, not on the bar
	aimMarks     map[[2]int][]int        // Aim: the squares of the foes Next/Prev stop on, framed as Move frames its reach
	enemiesOnly  bool                    // aiming a weapon or hostile spell: Next/Prev skip allies (combatturn2.go)
	learned      map[*Character][21]byte // the spells each caster held before using some (camp.go noteSpells)
	picker       bool                    // the list has no menu words: a click picks (listmenu.go)
	floor        int                     // regions below belong to the screen under the modal one
	menuFloor    int                     // menu bars open under the modal screen
	viewing      bool                    // the View page's own menu is waiting
	items        bool                    // the inventory's item list is waiting
	toItems      bool                    // the next View opens on the inventory (F1-F8)
	toSheet      bool                    // P left the inventory for the View page
	switchTo     bool                    // F1-F8 left the inventory for another member
	throwing     *Item                   // combat: Throw left the inventory to aim this item
	buying       bool                    // the shop's Buy list is waiting: F1-F8 change the buyer
	campBar      bool                    // a camp menu bar is waiting: F1-F8 change the member
	later        []uint16                // keys handed to the game one at a time (Engine.ReadKey)
	sels         map[string]int          // the highlighted item each menu was left with
	listWord     map[string]byte         // a list menu's highlighted word, by its hot letter, kept across pages
	selSet       bool                    // the caller set the highlight (YesNo): keep it
	skip         bool                    // a key finished the text being printed: no more slow text until input is read

	// fullmap.go
	labels  map[int]map[int]mapLabel // per area: square -> label
	fullMap bool                     // the full-screen map is up
	walk    *walkState               // walk.go: the party walks to a square
	back    bool                     // Down stepped backwards: Move turns the party round again

	// fonts.go
	font   int              // 0: the game's font, else fonts[font-1]
	fonts  []pcFont         // the game folder's fonts (FONTS/*.f8), then the PC fonts
	cells  [25][40]textCell // the text drawn into each cell
	note   string           // shown over the picture for a moment
	noteAt time.Time
	hint   string // the hint line last sent to the window

	// journal.go
	printed   string     // the text window's text since it was cleared
	refsShown int        // journal references in it already shown
	paging    bool       // a journal text is shown
	battleLog []string   // the combat messages of this battle (Log)
	panel     *Character // whose line the combat status row shows
}

const (
	seenSeen    = 1
	seenVisited = 2
)

// areaKey identifies the current map: script area plus GEO block.
func (e *Engine) areaKey() int {
	return int(e.gs.areaKind)<<16 | int(e.area.Geo)
}

// markSeen records the party's cell as visited and the cells in view
// (straight ahead until a wall, up to 3) as seen.
func (e *Engine) markSeen() {
	if e.viewMode != 1 {
		return
	}
	if e.ext.seen == nil {
		e.ext.seen = map[int]*[256]uint8{}
	}
	k := e.areaKey()
	m := e.ext.seen[k]
	if m == nil {
		m = &[256]uint8{}
		e.ext.seen[k] = m
	}
	x, y := e.posX, e.posY
	if !onMap(x, y) {
		return
	}
	m[y<<4|x] = seenVisited
	for i := 0; i < 3; i++ {
		if e.WallAt(x, y, e.facing) != 0 {
			break
		}
		x, y = x+dirDX[e.facing], y+dirDY[e.facing]
		if !onMap(x, y) {
			break
		}
		if m[y<<4|x] == 0 {
			m[y<<4|x] = seenSeen
		}
	}
}

// minimapAllowed follows the game's own rule for its Area command.
// In camp the panel keeps the party.
func (e *Engine) minimapAllowed() bool {
	return e.viewMode == 1 && e.area.NoCoords == 0 && e.beginFlag != 2
}

// Minimap geometry: 16x16 cells of 7 pixels inside the roster panel
// (cells 0x11..0x26 x rows 1..0xE).
const (
	mmCell = 7
	mmX    = 0x11*8 + (22*8-16*mmCell)/2
	mmY    = 8
)

// DrawMinimap draws the current map with the party. Like the game's Area
// view it shows every wall; walls of squares not yet seen are dim.
func (e *Engine) DrawMinimap() {
	e.ClearCells(17, 1, winRight, 14)
	e.S.Lock()
	defer e.S.Unlock()
	e.drawMap(e.S, mmX, mmY, mmCell, e.labels(), nil)
}

// rosterPanel redraws the panel the minimap shares with the roster.
func (e *Engine) rosterPanel() {
	if e.ext.minimap && e.minimapAllowed() {
		e.DrawMinimap()
		return
	}
	e.ClearCells(17, 1, winRight, 14)
	e.drawPartyList()
}

// extraKey handles a convenience key while the command bar waits.
// It reports whether the key was used.
func (e *Engine) extraKey(k byte) bool {
	if !e.ext.inCommand {
		return false
	}
	switch k {
	case 9: // Tab
		if !e.minimapAllowed() {
			e.DisplayMessageWait(colYellow, "Not Here")
			return true
		}
		e.ext.minimap = !e.ext.minimap
		e.rosterPanel()
		return true
	case 'p', 'P':
		e.PartyOverview()
		return true
	case 'm', 'M':
		e.FullMap()
		return true
	}
	return false
}

// viewKey: F1..F8 (n = 0..7) on the command bar or in a menu with View
// select member n and open their inventory (View, starting on Items). On
// the View page or in the inventory they switch to member n (Exit, then
// View again), staying on the page they were on. Not in combat, where View is the fighter's.
func (e *Engine) viewKey(n int) uint16 {
	if n >= len(e.party) || e.beginFlag == 5 || e.ext.menu == 0 {
		return 0
	}
	switch {
	case e.ext.viewing || e.ext.items:
		if n == e.partySel { // already theirs: P switches pages
			return 0
		}
		e.partySel, e.ext.toItems = n, e.ext.items // the same page for member n
		e.ext.switchTo = e.ext.items
		e.ext.later = append(e.ext.later, 'V')
		return 'E'
	case e.ext.campBar: // camp: F1-F8 pick the member the menu is for
		if n == e.partySel {
			return 0
		}
		e.partySel = n
		return keyRelist
	case e.ext.buying: // the shop's Buy list: F1-F8 pick the buyer
		if n == e.partySel {
			return 0
		}
		e.partySel = n
		return keyRelist
	case e.ext.inCommand || strings.Contains(e.ext.nav.menu, "View"):
		e.partySel, e.ext.toItems = n, true
		return 'V'
	}
	return 0
}

// sheetKey: P toggles between the View page and the inventory.
func (e *Engine) sheetKey() uint16 {
	switch {
	case e.ext.items:
		e.ext.toSheet = true
		return 'E'
	case len(e.selected().Items) > 0:
		return 'I'
	}
	return 0
}

// statusName = DS:1047 + 0xD*n, the status words. extra: dead with the
// poison still in (0x37, Slow Poison can still raise) reads "Poisoned".
func (e *Engine) statusName(c *Character) string {
	if c.Status > statusGone {
		return ""
	}
	if c.Status == statusDead && e.FindAffect(c, affPoisoned) != nil {
		return "Poisoned"
	}
	return statusNames[c.Status]
}

// PartyOverview shows every member's state on one screen until a key is
// pressed, then puts the adventuring screen back.
func (e *Engine) PartyOverview() {
	defer e.modalOn()()
	e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colWhite, "Party")
	y := 3
	e.DisplayString(2, y, colWhite, "Name")
	e.DisplayString(18, y, colWhite, "HP")
	e.DisplayString(26, y, colWhite, "AC")
	e.DisplayString(30, y, colWhite, "Status")
	y += 2
	for i, c := range e.party {
		if y > 0x14 {
			break
		}
		colour := colLightCyan
		if i == e.partySel {
			colour = colWhite
		}
		e.DisplayString(2, y, colour, cut(c.Name(), 15))
		hp, max := int(c.HP), int(c.MaxHP)
		hpc := 0xA
		switch {
		case hp <= 0:
			hpc = 0xC
		case hp*2 < max:
			hpc = 0xE
		}
		e.DisplayString(18, y, hpc, strconv.Itoa(hp)+"/"+strconv.Itoa(max))
		e.displayAC(c, 0x1A, y)
		e.DisplayString(30, y, colLightGreen, e.statusName(c))
		// Hit point bar, 32 pixels wide, under the name.
		px, py := 2*8, (y+1)*8+2
		w := 0
		if max > 0 && hp > 0 {
			w = (hp*0x20 + max - 1) / max
			if w > 0x20 {
				w = 0x20
			}
		}
		e.S.Lock()
		e.S.Fill(px, py, px+0x20*4, py+3, 8)
		e.S.Fill(px, py, px+w*4, py+3, uint8(hpc))
		e.S.Unlock()
		y += 2
	}
	e.PromptRow24(colWhite, "press any key to continue")
	e.ClearRow24()
	e.DrawMainScreen()
}

// viewDrawn runs after the 3D view is drawn: remember what the party saw
// and keep the minimap current.
func (e *Engine) viewDrawn() {
	e.markSeen()
	if e.ext.minimap && e.minimapAllowed() && e.beginFlag != 0 {
		e.DrawMinimap()
	}
}

// selBg picks the selection background: blue, or the next colour that is
// neither the text nor the hotkey colour.
func selBg(normal, highlight int) int {
	for _, c := range []int{1, 5, 4, 2} {
		if c != normal && c != highlight {
			return c
		}
	}
	return 1
}

// helpLines is the keyboard reference ("?"): a heading has no key.
var helpLines = [][2]string{
	{"Menus", ""},
	{"Letter", "choose that item"},
	{"Arrows , .", "move the highlight"},
	{"Enter", "pick the highlighted"},
	{"Home End", "prev. / next member"},
	{"Esc", "back"},
	{"Adventuring", ""},
	{"Arrows", "walk, turn; Down=back"},
	{"Home End", "prev. / next member"},
	{"F1 - F8", "items of member 1 - 8; in camp: pick member"},
	{"P", "items  <->  character"},
	{"W", "sweep ahead: search as you walk"},
	{"S", "search here: this square, once"},
	{"Tab", "minimap"},
	{"M", "map: walk, labels"},
	{"P", "party overview"},
	{"Combat", ""},
	{"Keypad 1-9", "move and aim"},
	{"Q   D", "quick   done"},
	{"Mouse", ""},
	{"Point  Click", "highlight  choose"},
	{"Wheel", "move the highlight"},
	{"Right button", "back"},
	{"", ""},
	{"F9", "save / load game"},
	{"Ctrl F", "font (+Shift: back)"},
	{"?", "this help, any time"},
}

// Help shows the keyboard reference over whatever is on screen and puts
// the screen back when closed: headings in white, keys in yellow. It runs
// over pages: a key shows the next, Up/PgUp/Left the one before, Esc closes.
func (e *Engine) Help() {
	off := e.modalOn()
	e.ext.paging = true
	defer func() { off(); e.ext.paging = false }()
	restore := e.saveScreen()
	pages := helpPages()
	for p := 0; p < len(pages); {
		e.DrawBoxTitled(1, 1, winRight, 23, 0, colWhite, "Help")
		e.ClearCells(2, 2, 37, 23)
		for i, l := range pages[p] {
			if l[1] == "" {
				e.DisplayString(2, 2+i, colWhite, l[0])
				continue
			}
			e.DisplayString(3, 2+i, colYellow, l[0])
			e.DisplayString(helpCol, 2+i, colLightGreen, l[1])
		}
		if len(pages) > 1 {
			more := "page " + strconv.Itoa(p+1) + "/" + strconv.Itoa(len(pages))
			e.DisplayString(winRight-1-len(more), messageRow, colLightGreen, more)
		}
		switch e.ReadKey() {
		case 27:
			p = len(pages)
		case 0x4800, 0x4900, 0x4B00: // Up, PgUp, Left
			p = max(p-1, 0)
		default:
			p++
		}
	}
	restore()
}

// helpCol is where the descriptions start; they end before the border.
const helpCol = 0x10

// helpRows lines fit above the page number.
const helpRows = 0x14

// helpPages cuts helpLines into pages, between sections where it can; a
// page does not start with a blank line.
func helpPages() [][][2]string {
	var pages [][][2]string
	lines := helpLines
	for len(lines) > 0 {
		for len(lines) > 0 && lines[0][0] == "" {
			lines = lines[1:]
		}
		n := min(helpRows, len(lines))
		if n < len(lines) { // break before the last section that starts on the page
			for k := n; k > 1; k-- {
				if lines[k][1] == "" {
					n = k
					break
				}
			}
		}
		if n > 0 {
			pages = append(pages, lines[:n])
		}
		lines = lines[n:]
	}
	return pages
}
