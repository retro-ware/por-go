package engine

import (
	"strconv"
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
	voiced    map[string]bool     // voice.go: the recordings this save has heard
	quick     *quickPick          // CreateCharacter answers its own menus (quickstart)
	minStats  [6]int              // "Minimal stats": per-stat floor, STR as st*100+str00
	guarded   bool                // slots.go: a load can unwind to adventure
	inGame    bool                // slots.go: GameLoop has started: saving allowed
	aoe       int                 // aoe.go: radius+1 of the spell being aimed, 0 none
	bolt      [2]int              // aoe.go: the reach and bounce of the bolt being aimed (reach 0: not a bolt)
	aoeSpell  int                 // aoe.go: the spell being aimed

	// nav.go: keys and mouse
	held          Action                  // the action read next (Engine.input): seen by a poll, or one a choice leaves to follow it
	nav           navCtx                  // the waiting menu
	regions       []region                // clickable parts of the screen
	castCancelled bool                    // CastSpell: the player cancelled the aim (the turn goes on)
	raw           bool                    // the screen reads the arrows itself
	typing        bool                    // InputString is waiting
	modal         int                     // help or overview shown: any key closes
	enemiesOnly   bool                    // aiming a weapon or hostile spell: Tab cycles through foes only (playerAim)
	learned       map[*Character][21]byte // the spells each caster held before using some (camp.go noteSpells)
	picker        bool                    // the list has no menu words: a click picks (listmenu.go)
	floor         int                     // regions below belong to the screen under the modal one
	menuFloor     int                     // menu bars open under the modal screen
	viewing       bool                    // the View page's own menu is waiting
	toItems       bool                    // the next View opens on the inventory (F1-F8)
	throwing      *Item                   // combat: Throw left the inventory to aim this item
	buying        bool                    // the shop's Buy list is waiting: F1-F8 change the buyer
	campBar       bool                    // a camp menu bar is waiting: F1-F8 change the member
	sels          map[string]int          // the highlighted item each menu was left with
	listWord      map[string]Choice       // a list menu's highlighted word, kept across pages
	selSet        bool                    // the caller set the highlight (YesNo): keep it
	skip          bool                    // a key finished the text being printed: no more slow text until input is read

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
	e.mapPage(mmX, mmY, mmCell, e.labels(), nil)
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

// toggleMinimap (Tab on the command bar) puts the minimap over the
// roster or takes it away, where the game's Area map was allowed.
func (e *Engine) toggleMinimap() {
	if !e.minimapAllowed() {
		e.DisplayMessageWait(colYellow, "Not Here")
		return
	}
	e.ext.minimap = !e.ext.minimap
	e.rosterPanel()
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
		e.fill(px, py, px+0x20*4, py+3, 8)
		e.fill(px, py, px+w*4, py+3, uint8(hpc))
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
		switch a := e.anyKey(); {
		case a.Kind == ActClose:
			p = len(pages)
		case a.Kind == ActMove && (a.N == DirUp || a.N == DirPgUp || a.N == DirLeft):
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
