package engine

import (
	"image"
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
//	F1-F8  select party member 1-8 (Left/Right the previous/next on the
//	       menu screens); P switches between items and the View page
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
	minimap  bool                // roster panel shows the minimap
	seen     map[int]*[256]uint8 // per map (areaKey): 1 seen, 2 visited
	voiced   map[string]bool     // voice.go: the recordings this save has heard
	met      map[string]metIcon  // bestiary.go: the monster kinds this save has met
	beasts   []Beast             // bestiary.go: their entries, as last published
	creating bool                // CreateCharacter's sheet is up: no level, experience, party items or status on it
	minStats [6]int              // "Minimal stats": per-stat floor, STR as st*100+str00
	inGame   bool                // slots.go: GameLoop has started: saving allowed
	aoe      int                 // aoe.go: radius+1 of the spell being aimed, 0 none
	bolt     [2]int              // aoe.go: the reach and bounce of the bolt being aimed (reach 0: not a bolt)
	aoeSpell int                 // aoe.go: the spell being aimed

	// nav.go: keys and mouse
	held          Action      // the action read next (Engine.input): seen by a poll, or one a choice leaves to follow it
	castCancelled bool        // CastSpell: the player cancelled the aim (the turn goes on)
	castYes       bool        // castAsks: yes to "Lose it?" / "Use it?"
	help          [][2]string // the help's lines (SetHelp)
	castOn        *Character  // castAsks: the member the spell goes on (nil: none)
	enemiesOnly   bool        // aiming a weapon or hostile spell: Tab cycles through foes only (playerAim)

	// fullmap.go
	labels  map[int]map[int]mapLabel // per area: square -> label
	fullMap *mapScreen               // the full-screen map, while up (modeMap)
	pager   *pager                   // help's or the journal's pager, while up
	walk    *walkState               // walk.go: the party walks to a square
	back    bool                     // Down stepped backwards: Move turns the party round again

	// fonts.go
	font   int      // 0: the game's font, else fonts[font-1]
	fonts  []pcFont // the fonts in data/assets/fonts (*.f8), then the PC fonts
	redraw func()   // draws the screen up again at the page's size (resizePage); nil: it stays as drawn until its next draw
	note   string   // shown over the picture for a moment
	noteAt time.Time

	// journal.go
	printed   string     // the text window's text since it was cleared
	refsShown int        // journal references in it already shown
	battleLog []string   // the combat messages of this battle (Log)
	panel     *Character // whose line the combat status row shows
}

const (
	seenSeen    = 1
	seenVisited = 2
)

// areaKey identifies the current map: data set and GEO block (New Phlan's
// three scripts share one map).
func (e *Engine) areaKey() int {
	return int(e.set)<<16 | int(e.area.Geo)
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

// MinimapAt is the minimap's top left on the page, MinimapCell its
// square's side in pixels.
var (
	MinimapAt   = image.Pt(mmX, mmY)
	minimapRect = image.Rect(mmX, mmY, mmX+16*mmCell, mmY+16*mmCell)
)

const MinimapCell = mmCell

// DrawMinimap puts the minimap up in the party panel (View.Minimap: the
// presenter draws View.Map there). Like the game's Area view it shows
// every wall; walls of squares not yet seen are dim.
func (e *Engine) DrawMinimap() {
	e.blank(17, 1, paneRight, 14)
	e.pg.tx.minimap = true
}

// rosterPanel redraws the panel the minimap shares with the roster.
func (e *Engine) rosterPanel() {
	if e.ext.minimap && e.minimapAllowed() {
		e.DrawMinimap()
		return
	}
	e.blank(17, 1, paneRight, 14)
	e.drawPartyList()
}

// minimapFile in the save folder marks the minimap as switched on (Tab).
const minimapFile = "MINIMAP.ON"

// toggleMinimap (Tab on the command bar) puts the minimap over the
// roster or takes it away, where the game's Area map was allowed.
func (e *Engine) toggleMinimap() {
	if !e.minimapAllowed() {
		e.DisplayMessageWait(colYellow, "Not Here")
		return
	}
	e.ext.minimap = !e.ext.minimap
	e.mark(minimapFile, e.ext.minimap)
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

// PartyOverview puts up the party overview (View.Overview: every member's
// state from View.Party) until a key is pressed, then the adventuring
// screen again.
func (e *Engine) PartyOverview() {
	e.push(modeOverview, e.pressed, e.DrawMainScreen)
}

// viewDrawn runs after the 3D view is drawn: remember what the party saw
// and keep the minimap current.
func (e *Engine) viewDrawn() {
	e.markSeen()
	if e.ext.minimap && e.minimapAllowed() && e.beginFlag != 0 {
		e.DrawMinimap()
	}
}

// SetHelp gives the help ("?") its lines, the front end's binding table
// (a heading has no description): the engine only pages them.
func (e *Engine) SetHelp(lines [][2]string) { e.ext.help = lines }

// Help puts up the keyboard reference (View.Pages) over whatever is on
// screen: a key shows the next page, ActPage -1 the one before, Esc closes.
func (e *Engine) Help() { e.pushPager(modeHelp, &pager{title: "Help"}) }

// helpPage is page p of the help: headings in white, keys in yellow.
func (e *Engine) helpPage(p int) *Pages {
	pages := helpPages(e.ext.help)
	pg := &Pages{Title: e.boxTitle("Help"), Top: 2, Page: p, Pages: len(pages)}
	for _, l := range pages[p] {
		if l[1] == "" {
			pg.Lines = append(pg.Lines, []PaneRun{e.textRun(2, colWhite, l[0])})
			continue
		}
		pg.Lines = append(pg.Lines, []PaneRun{e.textRun(3, colYellow, l[0]), e.textRun(helpCol, colLightGreen, l[1])})
	}
	if len(pages) > 1 {
		more := "page " + strconv.Itoa(p+1) + "/" + strconv.Itoa(len(pages))
		pg.Hint = e.textRun(winRight-1-len(more), colLightGreen, more)
	}
	return pg
}

// helpCol is where the descriptions start; they end before the border.
const helpCol = 0x10

// helpRows lines fit above the page number.
const helpRows = 0x14

// helpPages cuts lines into pages, between sections where it can; a page
// does not start with a blank line. There is always a page.
func helpPages(lines [][2]string) [][][2]string {
	pages := [][][2]string{}
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
	if len(pages) == 0 {
		pages = append(pages, nil)
	}
	return pages
}
