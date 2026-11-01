package engine

import (
	"image"
	"image/color"
	"por/pal"
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
	WinThreeD  Window = iota // the 3D view of Area at X, Y, Facing
	WinAreaMap               // the wilderness or city map
	WinPicture               // a picture, portrait or approaching sprite
	WinText                  // text in the whole view window (opClearBox: the Sokal Keep scroll's runes)
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

// The insides of the original's main screen boxes, at their
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
	Colour   int         // the roster's EGA colour for the name when not Selected: red not okay, yellow a monster, magenta ready for a level, else cyan
	Head     image.Image // the paper-doll head the player chose, in EGA colours (nil if missing)
	Portrait image.Image // extra: the PNG the player dropped on the sheet, in place of Head (nil: none)
	Powers   []Power     // extra: what the member can do now out of combat (a card's context menu): View, Items, then powers'; Kind with the member's index acts
	Exp      int         // extra: experience, what the level took (LevelExp) and what the next takes (NextExp; 0: it can rise no further)
	LevelExp int
	NextExp  int
	Spells   string // extra: a spellcaster's spells ready over its slots ("0/3", the overview); "" otherwise
	Memo     string // extra: in camp a spellcaster's spells ready over its slots ("0/3"); "" otherwise
}

// Pane is the text under the 3D view (TextPane): what DisplayTextBlock,
// the messages and the debug values put there, a line per row.
type Pane struct {
	Lines [TextPaneRows][]PaneRun
}

// TextPaneRows is the text pane's rows.
const TextPaneRows = paneBottom - msgTop + 1

// PaneRun is text in one colour on a pane line from column X of the pane
// (0: its left edge); Upcase picks the glyphs as DisplayChar does.
type PaneRun struct {
	X      int
	Text   string
	Fg, Bg color.RGBA
	Upcase bool
}

// Pages is help's or the journal's page (View.Pages), in a box round the
// whole page: ActPage -1 and +1 page back and on (the journal: scroll a
// tall picture first), ActClose closes, any other key the next page.
type Pages struct {
	Title       PaneRun     // the box's title, on row 1
	Top         int         // the row of Lines[0]
	Lines       [][]PaneRun // the page's text, a line a row, X the page's column
	Scans       []Scan      // the journal's pictures on the page
	Hint        PaneRun     // on the message row: the page count or the keys
	Page, Pages int
}

// Scan is a journal picture (the file Name in journal/): the part Src of
// Img scaled into the page rectangle At, on paper of colour Paper.
type Scan struct {
	Name    string
	Img     image.Image
	Src, At image.Rectangle
	Paper   color.RGBA
}

// FullMap is the full-screen map (View.FullMap), the cursor's cell's
// context menu its question (Walk, Mark, Remove): ActPoint X, Y moves the cursor.
type FullMap struct {
	Name     string      // the map's name, top left
	Map      *MapSquares // the squares, the legend's letter on each labelled square
	At       image.Point // the map's top left on the page, in pixels
	Cell     int         // pixels a square
	Cursor   image.Point // the cursor's square
	Legend   []MapLabel  // the legend's lines, from cell LegendAt down
	LegendAt image.Point
	Info     string  // the cursor's square and its label, on the message row
	Label    *string // the label being typed (nil: none), on the prompt row
}

// MapSquares is a map as the maps show it (minimap, View.Map, the full
// map): squares [y][x], the party's square (off the map: not shown) and
// the step towards its facing.
type MapSquares struct {
	Squares [16][16]MapSquare
	Party   image.Point
	Ahead   image.Point
}

// MapSquare is a square of a map: how far the party has seen it, its
// walls (N E S W) and its label (Ink 0: none).
type MapSquare struct {
	Seen   Seen
	Walls  [4]MapWall
	Ink    int  // the label's EGA colour, by kind
	Letter byte // the label's legend letter
}

// Seen is how much of a square the party has seen.
type Seen uint8

const (
	Unseen  Seen = iota
	SeenFar      // seen from afar
	Visited      // walked on
)

// MapWall is a wall on a map.
type MapWall uint8

const (
	NoWall MapWall = iota
	Wall
	Door
	LockedDoor
)

// DoorColour is a door's colour on the maps (an index for pal.Regs.RGBA).
const DoorColour = doorColour

// MapLabel is a line of the full map's legend: the letter on its squares
// (in EGA colour Colour, by kind), its text and its first square.
type MapLabel struct {
	Letter byte
	Text   string
	Colour int
	First  image.Point
}

// Shown reports whether the pane holds text.
func (p *Pane) Shown() bool {
	for _, l := range p.Lines {
		if len(l) > 0 {
			return true
		}
	}
	return false
}

// View is one published snapshot.
type View struct {
	Pane       *Pane // the text pane under the 3D view: out adventuring (Adventure), or it holds text (nil otherwise)
	Window     Window
	AreaMap    *AreaMap // with WinAreaMap: the whole map, for a screen of its own
	Area       AreaID
	X, Y       int
	Facing     Dir
	Day        bool        // 6:00 to 20:00, when the scripts show the day sky
	Inside     bool        // the party stands on an inside square (the original's inside sky colour)
	Exploring  bool        // the party explores: the adventuring decisions (ActForward .. ActCastPick) are taken
	Sweeping   bool        // ... at the slow, searching pace (ActSweep): the HUD says so
	CanCast    bool        // ... and the selected member has a spell to cast (ActCastPick)
	Camp       *Camp       // the party is in camp: the camp decisions (ActRest .. ActIconEdit) are taken; ActClose leaves
	Temple     *Temple     // a temple: ActTempleCure, ActAppraise, ActKeep, ActMember, ActLead; ActClose leaves
	Shop       *Shop       // a shop: ActInventory (the goods, ActBuy, ActSell), ActIdentify, ActAppraise, ActKeep, ActLead; ActClose leaves
	Training   *Training   // the training hall for the selected member: ActTrain, ActLearn; ActClose leaves
	Sheet      *Sheet      // the selected member's sheet (the page): ActInventory, ActSpells .. ActCure, ActMember (another member); ActClose leaves
	IconEditor *IconEditor // the combat icon editor is up (the page; IconDrop): the presenter lays it out and takes its clicks
	SheetPage  *SheetPage  // the character sheet's page is up (the sheet, the creation's, the spell lists over it): the presenter draws it
	Creation   *Creation   // the creation (or Modify) sheet (the page): ActRoll, ActSetField, ActName, ActStat, ActPicker, ActIconEdit, ActKeep; ActClose abandons (the player agreed to Abandon)
	Picker     *Picker     // a grid picker (the page): ActPicked, ActPage; ActClose leaves
	Slots      *Slots      // the save slots: ActSave, ActLoad, ActDelete with the slot's name as Text; ActClose leaves
	Ask        *Ask        // a question waits for its answer (eclask.go): menu, list, typed line, member, key, pause (the presenter's to pace) or spell
	Party      []Member
	Border     [3]image.Image // the window border's corner, side and top cells (DrawBorder style 0) in EGA colours; nil before the symbols load
	Battle     *Battle        // the combat map (nil outside combat)
	Events     []CombatEvent  // what happened in the fight since the last Apply, in order (Apply's View only)
	Inventory  *Inventory     // the inventory screen is up (nil otherwise)
	Map        *MapSquares    // the current map, drawn by the presenter at MapCell pixels a square (nil where the game has no map)
	Pictures   []Picture      // the screen's pictures, symbols and fills, bottom first (page.go)
	Adventure  bool           // the party is out in the world: the 3D view (or the overland map) is the screen
	Prompt     []PaneRun      // the prompt row's text (the last row), X the page's column
	ViewText   [][]PaneRun    // text over the view window (WinText: opProtection's runes), rows from 1, X the page's column
	Results    *Results       // the after-combat page (in a box round the window)
	Credits    bool           // the title's credits page (Credits, RemasterCredit)
	Full       bool           // Pictures are one whole-screen picture (the title's): fitted to the window
	Cells      image.Point    // the page's size in text cells (40x25 at least; a larger window gives more, ActResize)
	Ink        [16]color.RGBA // the text colours (EGA 0..15) in the page's palette now
	Portrait   CellRect       // the sheet's portrait box (zero elsewhere): Shift-click takes the next head (ActHead), a dropped PNG becomes the portrait (ActPortrait)
	IconDrop   bool           // the combat icon editor is up: a dropped PNG becomes the member's icon (ActIcon)
	IconBox    CellRect       // the creation sheet's combat icon (zero elsewhere): a PNG dropped on it becomes the icon (ActIcon)
	Pages      *Pages         // help or the journal is up (a box round the whole page)
	FullMap    *FullMap       // the full-screen map is up
	Overview   bool           // the party overview is up (from Party): any key closes it
	Purse      *Purse         // with Overview: the party's money and party items
	Ordering   bool           // the camp's Party Order bar is up: a card dragged onto another's place moves the member (ActOrder)
	MainMenu   []ActKind      // the party menu is up: the functions open now (ActCreate .. ActQuit, ActMember, ActDropMember); ActLead selects a member
	Logo       image.Image    // the title's lettering on transparent black, with MainMenu
	Backdrop   image.Image    // the Amiga painted title under MainMenu (nil: black)
	Hall       bool           // MainMenu is the training hall's: over the 3D view, Begin leaves the hall
	Seq        uint64         // counts the Views published: a presenter redraws the page on a new one
	Ticks      bool           // the engine is in a timed loop (an animation, the auto-walk, a pause it times itself): send ActTick every frame
}

// Results is the after-combat page (ovr06): the hired NPCs hiding their
// shares (hiredShare), else the outcome (showResult), or the party
// destroyed; the presenter lays it out in the window's box.
type Results struct {
	Shares  []string // hiredShare: "X takes and hides his share.", each wrapped from column 5 to 34 on rows 5, 7 ...
	Outcome string   // "The party has won." ... on row 3
	Exp     string   // "Each character receives 120" on row 5, "experience points." on row 7
	Taken   string   // "The party takes ...", wrapped from row 9 ("" none)
	Harmed  []Harm   // the members out of the fight dead or worse, or with harmful effects (extra), from row 12
	End     string   // the party is destroyed: the text under "The END!" (the box's title), wrapped in EndAt
	EndAt   CellRect
}

// Harm is a member's line on the after-combat page: its name and what
// harms it.
type Harm struct{ Name, What string }

// SheetPage is what the character sheet's page shows (CharacterSheet),
// laid out by the presenter as the original's ovr20 did, in the colours of
// View.Ink. The DAX portrait is among View.Pictures.
type SheetPage struct {
	Name       string
	NameColour int    // red when not okay, yellow a monster, else cyan (displayName)
	NPC        bool   // "(NPC)" after the name
	Spells     string // a spell list's title over the name row ("Spells in Memory"); "" none
	Sex, Race  string
	Age        int
	Alignment  string
	Class      string
	Stats      [6]int // STR .. CHA
	Str00      string // the exceptional strength after STR: "(78)", ""
	Creating   bool   // the creation's sheet: no level, experience, party items, items or status
	Levels     string // "1/2"
	Exp        int
	Next       int // the experience the next level takes (0: none)
	AC, HP     int // AC as displayed (10 down to negative)
	MaxHP      int // HP below it shows yellow
	THAC0      int
	Damage     string
	Move       int
	Status     string
	Effects    string         // the spell effects, as many as fit the row
	PortraitAt CellRect       // the portrait's box (zero: no portrait)
	Portrait   image.Image    // a dropped PNG in the box, in place of the DAX portrait (nil: none)
	Icons      [2]image.Image // the creation's combat icon from a PNG, ready and action poses, in View.IconBox (nil: the DAX icon among the pictures)
}

// Creation is the creation sheet's parts: the drop-down fields, and the
// cells a click on the name, an ability, the portrait or the icon takes.
type Creation struct {
	Fields   [4]Field // FieldSex, FieldRace, FieldAlign, FieldClass
	Name     CellRect
	Stats    [6]CellRect // STR .. CHA
	Head     CellRect    // the portrait's head (PickHead) ...
	Body     CellRect    // ... and body (PickBody)
	Icon     CellRect    // the combat icon (ActIconEdit)
	Abandon  string      // what Esc asks before ActClose
	Minimums string      // the minimums Minimal last rolled to (ActRoll N 1)
}

// The creation's fields (ActSetField N).
const (
	FieldSex = iota
	FieldRace
	FieldAlign
	FieldClass
)

// Field is a drop-down of the creation sheet.
type Field struct {
	Name    string
	At      CellRect // its text on the sheet
	Options []Option
	Cur     int // the option it holds
}

// Option is one of a Field's values; one that does not Fit changes the
// fields depending on it.
type Option struct {
	Text string
	Fits bool
}

// Picker is a grid picker's page as the engine drew it.
type Picker struct {
	N, Page, Pages, PerPage, Cols int
	Cur                           int           // the picture it opened on
	Cells                         []CellRect    // the pictures on this page: Page*PerPage on
	Prev, Next                    CellRect      // the pager's arrows (zero: one page)
	Bottom                        int           // the box's inner bottom row (the box: from cell 1, 1 to the page's right)
	Labels                        []string      // the pictures' names (the saved characters)
	Classes                       []string      // ... their classes ("": the files are gone)
	Portraits                     []image.Image // ... their PNG portraits, fitted into the head's place (nil: none; the DAX head is among the pictures)
	Ask                           string        // the question before a pick (%s: the label), "" none
}

// Slots is the save slot screen: the saves, newest first.
type Slots struct {
	Names, Times []string
	CanSave      bool   // ActSave is taken (else loading only, or not now)
	LoadOnly     bool   // the party menu's Load
	Current      string // the last save ("" none): the list starts on it
}

// Menu is a question waiting for the player: a prompt and its options.
type Menu struct {
	Prompt string
	Items  []MenuItem // the options (ActChoose N); none: a message to acknowledge (ActOK)
}

// ListLine is a line of View.Ask (AskList).
type ListLine struct {
	Text    string
	Header  bool     // a heading
	Dim     bool     // greyed out
	Off     bool     // not chosen (headings, some greyed lines)
	Without []string // options (MenuItem.Text) left out while this line is chosen
}

// MenuItem is one option: its text and its hotkey.
type MenuItem struct {
	Text string
	Key  byte
}

// Purse is the party's money and party inventory, on the overview.
type Purse struct {
	Gold, Copper, Gems, Jewelry int
	Items, Slots                int // the party items held and their room ("12/60")
}

// Sheet is what the character sheet offers besides the page.
type Sheet struct {
	Powers []Power // Spells, Magic, Heal, Cure
}

// Temple is a temple's services for the selected member.
type Temple struct {
	Cures    []Cure // ActTempleCure X is the index
	Gold     int    // the party's gold
	Appraise Appraise
}

// Cure is a temple service.
type Cure struct {
	Name   string
	Price  int    // gold
	Needed bool   // false: Not says why, and the player is asked to pay anyway
	Not    string // "is not blind."
}

// Appraise is the gems and jewelry to appraise (ActAppraise N 0 a gem,
// 1 a jewel).
type Appraise struct {
	Gems, Jewelry int
	Valued        int  // the gold piece value of the one appraised, to sell or keep (ActKeep N 0 sells, 1 keeps); 0: none
	Jewel         bool // ... it is a jewel
	CanKeep       bool // ... there is room to keep it
}

// Shop is a shop's services besides its goods (Inventory.Wares).
type Shop struct {
	Unknown     []Unknown // the party's items Id tells something about
	IDPrice     int       // gold an Id costs
	Gold        int
	Appraise    Appraise
	Identified  *InvItem // (extra) the item Id last revealed, its Look lines what it does
	IdentifiedN int      // counts the reveals: the presenter shows a new number once
}

// Unknown is an item to identify: ActIdentify N Member, X Item.
type Unknown struct {
	Member, Item int // the member and the index in its items (member 0: with the party inventory)
	Owner, Name  string
	Icon         *InvItem // its type and words: the icon
}

// Training is the training hall's offer to the selected member.
type Training struct {
	Name    string
	Why     string      // the hall refuses: why
	Becomes []string    // "a level 3 Fighter" (ActTrain trains)
	Price   int         // gold
	Gold    int         // the party's gold
	Learn   []BookSpell // a mage level was gained: the spell for the book (ActLearn X), chosen before leaving
}

// Camp is what camp shows of the selected member and the party.
type Camp struct {
	CanCast  bool        // the selected member has a spell to cast (ActCastPick)
	Book     []BookSpell // the selected member's spell book: how often each is memorized on a rest (ActPlan)
	Scrolls  []BookSpell // the spells on their scrolls: Count 1 while being scribed (ActScribe)
	Healing  bool        // someone resting can heal (RestHeal)
	Studying bool        // someone resting would memorize or scribe a spell (RestMemorize)
	Bestiary []Beast     // the monster kinds met
}

// BookSpell is a spell with a count, and the slots of its level.
type BookSpell struct {
	ID      int // the spell (ActPlan, ActScribe X)
	Name    string
	Level   string // its group, as "Cleric Level 1"
	Count   int
	Planned int // spells planned of its level
	Slots   int // slots of its level
}

// Beast is a bestiary entry.
type Beast struct {
	Name   string
	Danger int // I..VIII
	Text   string
	Icon   image.Image // the combat icon (nil: none)
	Pic    image.Image // the picture window's when first met (nil: none)
}

// MapCell is the size of a square in View.Map, in pixels.
const MapCell = mapCell

const mapCell = 10

// View returns the last published snapshot; safe from any goroutine.
func (e *Engine) View() View {
	var v View
	if e.snap.view != nil {
		v = *e.snap.view
	}
	v.Ticks = e.timed() || e.ext.walk != nil || e.animating()
	return v
}

// snapState is the engine's side of the seam.
type snapState struct {
	view         *View
	seq          uint64      // Views published
	invOn        bool        // ItemsMenu waits: View.Inventory is up
	shop         bool        // ... as the shop's Browse: View.Inventory.Wares
	identified   *InvItem    // the item the shop's Id last revealed (View.Shop.Identified)
	identN       int         // ... counted: a new reveal is a new number
	loot         bool        // ... as the Loot screen: the treasure is the Wares, free
	saves        bool        // there are saved games (the party menu's Load)
	logo         image.Image // titleLogo, once cut
	backdrop     image.Image // titleBackdrop, once read
	backdropRead bool
	win          Window      // set by whatever drew into the view window last
	areaImg      image.Image // View.AreaMap's picture, for areaTiles in areaPal
	areaTiles    [36][44]byte
	areaPal      pal.Regs
	heads        map[int]image.Image    // decoded HEAD blocks by id
	tiles        [3]image.Image         // View.Border, once decoded
	list         []*ListNode            // the innermost ListMenu's lines
	cursor       image.Point            // drawCursor's last square
	cursorOn     bool                   // the cursor icon is on the page (drawn, not yet erased or drawn over)
	pics         map[picKey]image.Image // picRGBA cache; ponytail: keyed by image, so frames patched in place (LoadBackpacIcons) stay stale until the palette changes
	picsPal      pal.Regs
	cmapOn       bool                   // the combat map is set up (cmapShown) and shown
	sheetOn      bool                   // a character sheet is up over the combat map
	portrait     CellRect               // the sheet's portrait box while a sheet is up
	sheet        *SheetPage             // the character sheet's page while it is up (View.SheetPage)
	results      *Results               // the after-combat page while it is up (View.Results)
	credits      bool                   // the credits page is up (View.Credits)
	full         bool                   // a whole-screen picture is up (View.Full)
	icon         *IconEditor            // the icon editor's page while it is up (View.IconEditor)
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
	targets      []int                  // combatants ActAttack takes; aiming: the foes to cycle through
	hidden       map[int]bool           // combatants whose icon is off the page (falling, hiding)
	poses        map[int]byte           // the pose each combatant was last drawn in
	strike       Strike                 // the last weapon attack (From/To filled in by battle)
	striker      [2]*Character          // its attacker and target
}

// publish builds a View from the engine's state (inside Apply)
// and makes it the current one.
func (e *Engine) publish() {
	if e.pg.held > 0 { // the 3D view is half drawn
		return
	}
	s := &e.snap
	s.seq++
	v := &View{
		Seq:    s.seq,
		Window: s.win,
		Area:   AreaID(e.area.Geo),
		X:      e.posX, Y: e.posY,
		Facing:    Dir(e.viewFacing() / 2 & 3),
		Day:       e.isDay(),
		Inside:    e.MapCell(e.posX, e.posY) > 0x7F,
		Exploring: e.exploring(),
		Sweeping:  e.exploring() && e.game.Search&1 != 0,
	}
	if s.win == WinAreaMap {
		v.AreaMap = e.areaMapView()
	}
	if (s.win == WinThreeD || s.win == WinPicture || s.win == WinText || e.choosing()) && e.minimapAllowed() {
		v.Map = e.mapSquares(e.labels())
	}
	if s.invOn {
		c := e.selected()
		v.Inventory = e.inventoryView(c, e.carried(c))
		if s.shop {
			e.shopWares(v.Inventory, c)
			v.Inventory.Loot = s.loot
		}
	}
	if v.Exploring {
		v.CanCast = e.canCastNow()
	}
	if e.mode() == modeCamp || e.mode() == modeRest {
		v.Camp = e.campView()
	}
	if !s.invOn {
		switch e.mode() {
		case modeTemple:
			v.Temple = e.templeView()
		case modeShop:
			v.Shop = e.shopView()
		case modeTraining:
			v.Training = e.trainingView()
		case modeSheet:
			v.Sheet = e.sheetView()
		case modeParty:
			if !e.modal() {
				v.MainMenu, v.Hall = e.partyFunctions(), e.ext.hall
				if !v.Hall {
					v.Logo, v.Backdrop = e.titleLogo(), e.titleBackdrop()
				}
			}
		}
	}
	switch e.mode() {
	case modeCreate:
		v.Creation = e.creationView()
	case modePicker:
		v.Picker = e.pickerView()
	case modeSlots:
		v.Slots = e.slotsView()
	case modeConfirm:
		a := confirmAsk(e.snap.ask)
		v.Ask = &a
	case modeScript:
		if w := e.eclWait; w != nil {
			a := w.ask
			v.Ask = &a
		}
	}
	v.Portrait = s.portrait
	v.IconDrop, v.IconBox, v.Ordering = s.iconDrop != nil && s.iconBox == (CellRect{}), s.iconBox, e.up(modeCamp)
	switch {
	case e.ext.pager != nil:
		v.Pages, _ = e.pagerPage(e.ext.pager)
	case e.ext.fullMap != nil:
		v.FullMap = e.fullMapView(e.ext.fullMap)
	case e.mode() == modeOverview:
		v.Overview = true
		v.Purse = &Purse{Gold: int(e.purse.gold), Copper: int(e.purse.copper), Gems: int(e.purse.gems), Jewelry: int(e.purse.jewelry), Items: len(e.pool), Slots: e.poolCap()}
	}
	if e.modal() { // the screens under take no drop
		v.Portrait, v.IconDrop, v.IconBox = CellRect{}, false, CellRect{}
	} else if s.sheet != nil {
		sp := *s.sheet
		v.SheetPage = &sp
	}
	v.Results, v.Credits, v.Full = s.results, s.credits, s.full
	if s.icon != nil && e.mode() == modeIcon {
		ed := *s.icon
		v.IconEditor = &ed
	}
	for i := range v.Ink {
		v.Ink[i] = e.pal.RGBA(byte(i))
	}
	e.pageView(v)
	v.Party = e.members()
	v.Border = e.borderTiles()
	v.Battle = e.battle()
	s.view = v
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
