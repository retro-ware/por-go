package engine

import (
	"image"
	"image/color"
	"por/pal"
	"strings"
	"sync/atomic"
)

// The read-only seam for presenters other than the classic screen: a View
// is the game as the player sees it, published by the engine each time it
// waits for a key. Nothing in it points back into the engine's state.

// Dir is a facing: N, E, S, W.
type Dir int

const (
	North Dir = iota
	East
	South
	West
)

// AreaID is the GEO block a 3D area is drawn from (the same area in every
// disk's GEO file that has it).
type AreaID int

// Window is what the view window (cells ViewWindow) shows.
type Window int

const (
	WinEmpty   Window = iota
	WinThreeD         // the 3D view of Area at X, Y, Facing
	WinAreaMap        // the wilderness or city map
	WinPicture        // a picture, portrait or approaching sprite
	WinText           // text in the whole view window (opClearBox: the Sokal Keep scroll's runes)
)

// CellRect is a rectangle of 8x8 text cells, both corners included. For a
// box it is the inside, as DrawBorder takes it: the border is the ring of
// cells around it (Framed).
type CellRect struct{ X1, Y1, X2, Y2 int }

// Pixels is the rectangle in screen pixels.
func (r CellRect) Pixels() image.Rectangle {
	return image.Rect(r.X1*8, r.Y1*8, r.X2*8+8, r.Y2*8+8)
}

// Framed is the box with its border.
func (r CellRect) Framed() CellRect { return CellRect{r.X1 - 1, r.Y1 - 1, r.X2 + 1, r.Y2 + 1} }

// The insides of the main screen's boxes (DrawMainFrame), the original's
// 40x25 places whatever the page's size: the remaster lays that screen
// out itself (the text pane as a pop-up, the party as cards).
var (
	OuterBox    = CellRect{1, 1, paneRight, paneBottom}
	ViewWindow  = CellRect{1, 1, 0xF, 0xF}
	InnerWindow = CellRect{3, 3, 0xD, 0xD} // the picture frame
	TextPane    = CellRect{1, msgTop, paneRight, paneBottom}
)

// Member is one party member as the party list shows them.
type Member struct {
	Name     string
	HP       int
	MaxHP    int
	AC       int         // as displayed: 10 down to negative
	Status   string      // as the sheet names it ("Okay", "Unconscious" ...)
	Class    string      // as the sheet names it ("Fighter/Thief" ...)
	Selected bool        // the selected member, or the candidate while a member is picked (SelectCharacter)
	Head     image.Image // the paper-doll head the player chose, in EGA colours (nil if missing)
	Portrait image.Image // extra: the PNG the player dropped on the sheet, in place of Head (nil: none)
	Powers   []Power     // extra: what the member can do now out of combat (a card's context menu): View, Items, then powers'; Kind with the member's index acts
	Exp      int         // extra: experience, what the level took (LevelExp) and what the next takes (NextExp; 0: it can rise no further)
	LevelExp int
	NextExp  int
}

// View is one published snapshot.
type View struct {
	MainFrame bool // the main screen's frame is up (not combat, a sheet, a shop, the title)
	Window    Window
	Area      AreaID
	X, Y      int
	Facing    Dir
	TextShown bool // the bottom text pane holds text
	Day       bool // 6:00 to 20:00, when the scripts show the day sky
	Inside    bool // the party stands on an inside square (the original's inside sky colour)
	Menu      *Menu
	Command   bool // Menu is the adventuring command bar
	Party     []Member
	Border    [3]image.Image // the window border's corner, side and top cells (DrawBorder style 0) in EGA colours; nil before the symbols load
	Battle    *Battle        // the combat map (nil outside combat)
	Inventory *Inventory     // the inventory screen is up (nil otherwise)
	Map       image.Image    // the map as the full map draws it (drawMap), 16 squares of MapCell pixels a side (nil where the game has no map)
	Page      []Sprite       // the screen's pictures, symbols and fills, bottom first (page.go)
	Text      [][]Cell       // the text cells, over Page: the page's rows of columns (40x25 at least; a larger window gives more, ActResize)
	Overlays  []Overlay      // pictures over everything at the window's resolution (the journal's scans)
	Spots     []CellRect     // the parts of the page that take clicks (ActSpot, ActHover N), the topmost last
	Portrait  CellRect       // the sheet's portrait box (zero elsewhere): Shift-click takes the next head (ActHead), a dropped PNG becomes the portrait (ActPortrait)
	IconDrop  bool           // the combat icon editor is up: a dropped PNG becomes the member's icon (ActIcon)
	IconBox   CellRect       // the creation sheet's combat icon (zero elsewhere): a PNG dropped on it becomes the icon (ActIcon)
	Ordering  bool           // the camp's Party Order bar is up: a card dragged onto another's place moves the member (ActOrder)
	Typing    *Typing        // a line is being typed: the presenter edits it (ActText)
	Choosing  bool           // a party member is being picked (SelectCharacter): Menu asks whom, Party's Selected is the candidate; ActMember or ActInventory N takes one, ActMove Up/Down steps, Enter takes the candidate
	MainMenu  []string       // the party menu's functions, as Menu.Items (nil when it is not up)
	Logo      image.Image    // the title's lettering on transparent black, with MainMenu
	Backdrop  image.Image    // the Amiga painted title under MainMenu (nil: black)
	MainAsk   bool           // Menu is a question over the main menu (Quit to DOS)
	SpellPick []Spell        // a spell to cast is being picked (pickSpell): the combat's Cast list on any screen; ActCast with its ID casts, ActClose gives up
	Seq       uint64         // counts the Views published: a presenter redraws the page on a new one
	Working   bool           // an action sent is still being carried out: what the View shows is on its way out (a menu still up is not asked again)
}

// Menu is the menu bar waiting for a choice on the prompt line.
type Menu struct {
	Prompt string
	Items  []MenuItem
	Sel    int // index into Items of the highlighted item
}

// MenuItem is one choice: its word as the bar shows it and the key that picks it.
type MenuItem struct {
	Text string
	Key  byte
}

// Typing is the line being typed and how long it may get.
type Typing struct {
	Text   string
	Max    int
	Prompt string // a script's question (the text window's text): the remaster asks it in a pop-up
}

// MapCell is the size of a square in View.Map, in pixels.
const MapCell = mapCell

const mapCell = 10

// View returns the last published snapshot; safe from any goroutine.
func (e *Engine) View() View {
	if v := e.snap.view.Load(); v != nil {
		return *v
	}
	return View{}
}

// snapState is the engine's side of the seam.
type snapState struct {
	view         atomic.Pointer[View]
	seq          uint64      // Views published
	invOn        bool        // ItemsMenu waits: View.Inventory is up
	shop         bool        // ... as the shop's Browse: View.Inventory.Wares
	loot         bool        // ... as the Loot screen: the treasure is the Wares, free
	main         []string    // PartyMenu waits: View.MainMenu
	mainAsk      bool        // ... and asks View.Menu over it
	logo         image.Image // titleLogo, once cut
	backdrop     image.Image // titleBackdrop, once read
	backdropRead bool
	win          Window                 // set by whatever drew into the view window last
	heads        map[int]image.Image    // decoded HEAD blocks by id
	tiles        [3]image.Image         // View.Border, once decoded
	typing       *Typing                // the line being typed (typed)
	own          bool                   // menu is a modal screen's own (slots), not the one under it
	menu         *Menu                  // the innermost DisplayInput's menu (Sel unset)
	cursor       image.Point            // drawCursor's last square
	cursorOn     bool                   // the cursor icon is on the page (drawn, not yet erased or drawn over)
	pics         map[picKey]image.Image // picRGBA cache; ponytail: keyed by image, so frames patched in place (LoadBackpacIcons) stay stale until the palette changes
	picsPal      pal.Regs
	cmapOn       bool                   // the combat map is set up (cmapShown) and shown
	sheetOn      bool                   // a character sheet is up over the combat map
	portrait     CellRect               // the sheet's portrait box while a sheet is up
	iconDrop     func(image.Image)      // the icon editor, or the creation sheet, takes a dropped PNG while it is up (nil elsewhere)
	iconBox      CellRect               // the creation sheet's icon: the cells that take the drop (zero: the whole screen)
	dungeon      bool                   // the combat tiles are DungCom's stone walls (else WildCom's outdoors)
	effects      []effect               // sprites on the combat page
	marks        []image.Point          // squares markReach framed
	reach        []image.Point          // squares a click on the main combat menu walks to
	paths        map[image.Point][]Step // the walk to each of them
	hits         map[int]int            // each of targets' chance to be hit, in percent
	area         []image.Point          // squares aoeMarks covers
	skipped      []image.Point          // squares of allies in the area the spell passes over (Bless: an enemy next to them)
	aimWhy       string                 // aiming: why the aimed square cannot be taken ("" when it can)
	turn         *Turn                  // the player's turn waits in act
	ask          string                 // a yes/no question waits in act (ask)
	spellPick    []Spell                // a spell to cast waits in act (pickSpell)
	targets      []int                  // combatants ActAttack takes; aiming: the foes to cycle through
	hidden       map[int]bool           // combatants whose icon is off the page (falling, hiding)
	poses        map[int]byte           // the pose each combatant was last drawn in
	strike       Strike                 // the last weapon attack (From/To filled in by battle)
	striker      [2]*Character          // its attacker and target
}

// publish builds a View from the engine's state (on the engine goroutine)
// and makes it the current one.
func (e *Engine) publish() {
	if e.pg.held > 0 { // the 3D view is half drawn
		return
	}
	s := &e.snap
	s.seq++
	v := &View{
		Seq:     s.seq,
		Working: e.Busy(),
		Window:  s.win,
		Area:    AreaID(e.area.Geo),
		X:       e.posX, Y: e.posY,
		Facing:  Dir(e.viewFacing() / 2 & 3),
		Day:     e.isDay(),
		Inside:  e.MapCell(e.posX, e.posY) > 0x7F,
		Command: e.ext.inCommand,
	}
	if s.win == WinThreeD || s.win == WinPicture || s.win == WinText || e.ext.choosing {
		v.Map = e.mapImage(mapCell)
	}
	if s.invOn {
		c := e.selected()
		v.Inventory = e.inventoryView(c, e.carried(c))
		if s.shop {
			e.shopWares(v.Inventory, c)
			v.Inventory.Loot = s.loot
		}
	}
	if m := s.menu; m != nil && (e.ext.modal == 0 || e.ext.menu > e.ext.menuFloor || s.own) { // not the menu under help or the overview
		c := *m
		c.Sel = min(max(e.menuSel-1, 0), len(c.Items)-1)
		v.Menu = &c
		if e.ext.modal == 0 && s.main != nil {
			v.MainMenu, v.Logo, v.Backdrop, v.MainAsk = s.main, e.titleLogo(), e.titleBackdrop(), s.mainAsk
		}
	}
	v.Typing, v.Spots, v.Portrait, v.Choosing, v.SpellPick = s.typing, e.spots(), s.portrait, e.ext.choosing, s.spellPick
	v.IconDrop, v.IconBox, v.Ordering = s.iconDrop != nil && s.iconBox == (CellRect{}), s.iconBox, e.ext.ordering
	e.pageView(v)
	sel := e.rosterSel() // a member pick shows its candidate
	for i, c := range e.party {
		if !isParty(c) { // the monsters loaded for a fight sit on the same list: no cards for them
			continue
		}
		v.Party = append(v.Party, Member{
			Name: c.Name(), HP: int(c.HP), MaxHP: int(c.MaxHP),
			AC: 60 - int(c.AC[0]), Status: e.statusName(c), Selected: i == sel,
			Class: className(c.Class), Head: e.charHead(c), Portrait: c.Portrait, Powers: e.memberPowers(c),
		})
		m := &v.Party[len(v.Party)-1]
		from, to := levelSpan(c)
		m.Exp, m.LevelExp, m.NextExp = int(int32(c.Exp)), int(from), int(to)
	}
	v.Border = e.borderTiles()
	v.Battle = e.battle()
	s.view.Store(v)
}

// charHead is the head the player chose for c, as the sheet shows it
// (SheetPortrait), or nil without one.
func (e *Engine) charHead(c *Character) image.Image {
	if c.PortHead == 0 || int(c.PortHead) >= len(portraitHead) {
		return nil
	}
	return e.head(int(portraitHead[c.PortHead]))
}

// head decodes HEAD block id in the EGA colours (cached; nil if missing).
func (e *Engine) head(id int) image.Image {
	if im, ok := e.snap.heads[id]; ok {
		return im
	}
	if e.snap.heads == nil {
		e.snap.heads = map[int]image.Image{}
	}
	var out image.Image
	im := e.LoadPic(charArt.file("HEAD"), id)
	if im != nil && im.Count > 0 {
		out = e.toRGBA(im.Frame(0), im.Width*8, im.Height, image.NewRGBA(image.Rect(0, 0, im.Width*8, im.Height)), 0, 0)
	}
	e.snap.heads[id] = out
	return out
}

// toRGBA draws the w by h picture pixels d into dst at x, y in the EGA
// colours (17..AmigaBase-1 stay transparent, as blit skips them) and returns dst.
func (e *Engine) toRGBA(d []byte, w, h int, dst *image.RGBA, x, y int) *image.RGBA {
	for i, p := range d[:w*h] {
		switch {
		case p == 16:
			p = 0
			fallthrough
		case p < 16:
			c := pal.Palette[p]
			dst.SetRGBA(x+i%w, y+i/w, color.RGBA{c[0], c[1], c[2], 255})
		case p >= pal.AmigaBase:
			dst.SetRGBA(x+i%w, y+i/w, e.pal.RGBA(p))
		}
	}
	return dst
}

// menuShown records the menu DisplayInput waits on, and returns the one
// it replaces (an outer DisplayInput's) for restoring.
func (e *Engine) menuShown(prompt, menu string, start, end []int) *Menu {
	old := e.snap.menu
	m := &Menu{Prompt: strings.TrimSpace(prompt)}
	for j := 1; j < len(start); j++ {
		if a, b := itemSpan(menu, start, end, j); a > 0 {
			text := make([]byte, 0, b-a+1)
			for i := a; i <= b; i++ {
				text = append(text, menuGlyph(menu, i)) // "aiM" reads "Aim"
			}
			m.Items = append(m.Items, MenuItem{Text: string(text), Key: menu[start[j]-1]})
		}
	}
	if len(m.Items) == 0 {
		m = nil
	}
	e.snap.menu = m
	return old
}

// borderTiles decodes DrawBorder's style 0 cells (0x114 corner, 0x115
// side, 0x116 top) once the symbols are loaded.
func (e *Engine) borderTiles() [3]image.Image {
	if e.snap.tiles[0] != nil || e.sym[4] == nil {
		return e.snap.tiles
	}
	im := e.sym[4]
	for i := range e.snap.tiles {
		e.snap.tiles[i] = e.toRGBA(im.Frame(0x114+i-symFix[4]), im.Width*8, im.Height, image.NewRGBA(image.Rect(0, 0, im.Width*8, im.Height)), 0, 0)
	}
	return e.snap.tiles
}
