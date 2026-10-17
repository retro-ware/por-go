package engine

import (
	"image"
	"image/color"
	"strings"
	"sync/atomic"

	"por/ega"
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

// The insides of the main screen's boxes (DrawMainFrame).
var (
	OuterBox    = CellRect{1, 1, 0x26, 0x16}
	ViewWindow  = CellRect{1, 1, 0xF, 0xF}
	InnerWindow = CellRect{3, 3, 0xD, 0xD} // the picture frame
	TextPane    = CellRect{1, 0x11, 0x26, 0x16}
	PromptRows  = CellRect{0, 0x18, 0x27, 0x18} // the menu and prompt line, under the frame
	CombatRows  = CellRect{0, 0x17, 0x27, 0x18} // combat's panel and menu line, under its frame
	CombatPanel = CellRect{0, 0x17, 0x27, 0x17} // the panel alone: who acts, HP, AC, damage
)

// Member is one party member as the party list shows them.
type Member struct {
	Name     string
	HP       int
	MaxHP    int
	AC       int    // as displayed: 10 down to negative
	Status   string // as the sheet names it ("Okay", "Unconscious" ...)
	Selected bool
	Head     image.Image // the paper-doll head the player chose, in EGA colours (nil if missing)
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
	Battle    *Battle     // the combat map (nil outside combat)
	Map       image.Image // the map as the full map draws it (drawMap), 16 squares of MapCell pixels a side (nil where the game has no map)
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

// MapCell is the size of a square in View.Map, in pixels.
const MapCell = mapCell

const mapCell = fmCell

// View returns the last published snapshot; safe from any goroutine.
func (e *Engine) View() View {
	if v := e.snap.view.Load(); v != nil {
		return *v
	}
	return View{}
}

// snapState is the engine's side of the seam.
type snapState struct {
	view     atomic.Pointer[View]
	win      Window                 // set by whatever drew into the view window last
	frame    []byte                 // the frame's border pixels as DrawMainFrame left them
	heads    map[int]image.Image    // decoded HEAD blocks by id
	border   [][2]int               // pixel positions of frame
	menu     *Menu                  // the innermost DisplayInput's menu (Sel unset)
	cursor   image.Point            // drawCursor's last square
	cursorOn bool                   // the cursor icon is on the page (drawn, not yet erased or drawn over)
	pics     map[picKey]image.Image // picRGBA cache; ponytail: keyed by image, so frames patched in place (LoadBackpacIcons) stay stale until the palette changes
	picsPal  [16]uint8
	cmapRow  []byte
	dungeon  bool          // the combat tiles are DungCom's stone walls (else WildCom's outdoors)
	effects  []effect      // sprites on the combat page
	marks    []image.Point // squares markReach framed
	hidden   map[int]bool  // combatants whose icon is off the page (falling, hiding)
	poses    map[int]byte  // the pose each combatant was last drawn in
	click    int           // the square (+1) a SquareKey being translated clicks
}

// frameDrawn remembers the frame's border pixels, so publish can tell it is
// still up (anything full screen draws over it).
func (e *Engine) frameDrawn() {
	s := &e.snap
	if s.border == nil {
		s.border = frameBorder()
	}
	e.S.Lock()
	s.frame = s.frame[:0]
	for _, p := range s.border {
		s.frame = append(s.frame, e.S.Pix[p[1]*ega.W+p[0]])
	}
	e.S.Unlock()
}

// frameBorder lists the pixels of the border cells of the view window and
// the text pane.
func frameBorder() [][2]int {
	cells := map[[2]int]bool{}
	for _, r := range []CellRect{ViewWindow.Framed(), TextPane.Framed()} {
		for x := r.X1; x <= r.X2; x++ {
			cells[[2]int{x, r.Y1}], cells[[2]int{x, r.Y2}] = true, true
		}
		for y := r.Y1; y <= r.Y2; y++ {
			cells[[2]int{r.X1, y}], cells[[2]int{r.X2, y}] = true, true
		}
	}
	var out [][2]int
	for y := 0; y < 25; y++ { // in a fixed order
		for x := 0; x < 40; x++ {
			if cells[[2]int{x, y}] {
				for py := 0; py < 8; py++ {
					for px := 0; px < 8; px++ {
						out = append(out, [2]int{x*8 + px, y*8 + py})
					}
				}
			}
		}
	}
	return out
}

// publish builds a View from the engine's state (on the engine goroutine)
// and makes it the current one.
func (e *Engine) publish() {
	s := &e.snap
	v := &View{
		Window: s.win,
		Area:   AreaID(e.area.Geo),
		X:      e.posX, Y: e.posY,
		Facing:  Dir(e.viewFacing() / 2 & 3),
		Day:     e.isDay(),
		Inside:  e.MapCell(e.posX, e.posY) > 0x7F,
		Command: e.ext.inCommand,
	}
	if s.win == WinThreeD || s.win == WinPicture {
		v.Map = e.mapImage(mapCell)
	}
	if m := s.menu; m != nil {
		c := *m
		c.Sel = min(max(e.menuSel-1, 0), len(c.Items)-1)
		v.Menu = &c
	}
	e.S.Lock()
	v.MainFrame = len(s.frame) > 0
	for i, p := range s.border {
		if i < len(s.frame) && e.S.Pix[p[1]*ega.W+p[0]] != s.frame[i] {
			v.MainFrame = false
			break
		}
	}
	in := TextPane.Pixels()
	for y := in.Min.Y; y < in.Max.Y && !v.TextShown; y++ {
		for x := in.Min.X; x < in.Max.X; x++ {
			if e.S.Pix[y*ega.W+x] != 0 {
				v.TextShown = true
				break
			}
		}
	}
	e.S.Unlock()
	for i, c := range e.party {
		v.Party = append(v.Party, Member{
			Name: c.Name(), HP: int(c.HP), MaxHP: int(c.MaxHP),
			AC: 60 - int(c.AC[0]), Status: e.statusName(c), Selected: i == e.partySel,
			Head: e.charHead(c),
		})
	}
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
	im := e.LoadPic("HEAD"+e.symDAX[4:], id)
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
			c := ega.Palette[p]
			dst.SetRGBA(x+i%w, y+i/w, color.RGBA{c[0], c[1], c[2], 255})
		case p >= ega.AmigaBase:
			dst.SetRGBA(x+i%w, y+i/w, e.S.RGBA(p))
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
			m.Items = append(m.Items, MenuItem{Text: menu[a-1 : b], Key: menu[start[j]-1]})
		}
	}
	if len(m.Items) == 0 {
		m = nil
	}
	e.snap.menu = m
	return old
}
