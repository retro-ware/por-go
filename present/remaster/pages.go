package remaster

import (
	"image"
	"image/color"
	"image/draw"
	"slices"
	"strconv"

	"por/engine"
)

// The screens the engine publishes as data (the character sheet, the
// creation's, the pickers, the save slots, the icon editor) are laid out
// here on the page's text cells as the original's code put them there.

// EGA text colours.
const (
	inkBlack   = 0
	inkGreen   = 10 // light green
	inkCyan    = 11
	inkMagenta = 13
	inkYellow  = 14
	inkWhite   = 15
)

// overlay is a picture drawn over the page at the window's resolution:
// the part Src of Img (empty: all of it) scaled into the page rectangle At
// (a portrait, a journal scan).
type overlay struct {
	Img     image.Image
	Src, At image.Rectangle
	Crisp   bool // pixel art: nearest neighbour, its aspect kept, centred in At
}

// Part is the rectangle of Img the overlay shows.
func (o overlay) Part() image.Rectangle {
	if o.Src.Empty() {
		return o.Img.Bounds()
	}
	return o.Src
}

type cell struct {
	set, upcase bool
	ch          byte
	fg, bg      color.RGBA
}

// cellPage is the page as the presenter draws it: the View's pictures and
// text, and the screens' text, borders and portraits over them.
type cellPage struct {
	cols, rows int
	ink        [16]color.RGBA
	cells      [][]cell
	pics       []engine.Picture  // the View's pictures, then the cells cleared (fills)
	boxes      []engine.CellRect // DrawBorder's insides: the border's cells round them (View.Border)
	overs      []overlay
}

func newCellPage(v engine.View) *cellPage {
	c := &cellPage{cols: max(v.Cells.X, 40), rows: max(v.Cells.Y, 25), ink: v.Ink}
	c.cells = make([][]cell, c.rows)
	for y := range c.cells {
		c.cells[y] = make([]cell, c.cols)
	}
	c.pics = append(c.pics, v.Pictures...)
	return c
}

// run puts r's text on row y.
func (c *cellPage) run(r engine.PaneRun, y int) {
	if y < 0 || y >= c.rows {
		return
	}
	for i := 0; i < len(r.Text) && r.X+i < c.cols; i++ {
		if r.X+i >= 0 {
			c.cells[y][r.X+i] = cell{true, r.Upcase, r.Text[i], r.Fg, r.Bg}
		}
	}
}

// str = DisplayString: s from cell (x, y) in colour on black, upcased.
func (c *cellPage) str(x, y, colour int, s string) {
	if x > c.cols-1 || y > c.cols-1 {
		return
	}
	for i := 0; i < len(s) && x+i < c.cols && y < c.rows; i++ {
		c.cells[y][x+i] = cell{true, true, s[i], c.ink[colour], c.ink[inkBlack]}
	}
}

// clear = ClearCells: the cells black.
func (c *cellPage) clear(x1, y1, x2, y2 int) {
	r := image.Rect(x1*8, y1*8, x2*8+8, y2*8+8)
	c.pics = append(c.pics, engine.Picture{At: r, Fill: true, Colour: c.ink[inkBlack]})
	c.unset(r)
}

func (c *cellPage) unset(r image.Rectangle) {
	r = r.Intersect(image.Rect(0, 0, c.cols*8, c.rows*8))
	for y := r.Min.Y / 8; y < (r.Max.Y+7)/8; y++ {
		for x := r.Min.X / 8; x < (r.Max.X+7)/8; x++ {
			c.cells[y][x].set = false
		}
	}
}

// border = DrawBorder: the border's cells round the cells x1..x2, y1..y2.
func (c *cellPage) border(x1, y1, x2, y2 int) {
	r := engine.CellRect{X1: x1, Y1: y1, X2: x2, Y2: y2}
	c.boxes = append(c.boxes, r)
	f := r.Framed().Pixels()
	for _, e := range []image.Rectangle{{f.Min, image.Pt(f.Max.X, f.Min.Y+8)}, {image.Pt(f.Min.X, f.Max.Y-8), f.Max}, {f.Min, image.Pt(f.Min.X+8, f.Max.Y)}, {image.Pt(f.Max.X-8, f.Min.Y), f.Max}} {
		c.unset(e)
	}
}

// box = DrawBoxTitled: cleared, bordered, the title centred on row y1.
func (c *cellPage) box(x1, y1, x2, y2, colour int, title string) {
	if len(title) > 0x28 {
		title = title[:0x28]
	}
	c.clear(x1, y1, x2, y2)
	c.border(x1, y1, x2, y2)
	if title != "" {
		c.str((x1+x2-len(title))/2, y1, colour, title)
	}
}

// lines are the cells as runs of one colour, a row each.
func (c *cellPage) lines() [][]engine.PaneRun {
	ls := make([][]engine.PaneRun, c.rows)
	for y, row := range c.cells {
		var l []engine.PaneRun
		for x, k := range row {
			if !k.set {
				continue
			}
			if n := len(l) - 1; n >= 0 && l[n].X+len(l[n].Text) == x && l[n].Fg == k.fg && l[n].Bg == k.bg && l[n].Upcase == k.upcase {
				l[n].Text += string(k.ch)
				continue
			}
			l = append(l, engine.PaneRun{X: x, Text: string(k.ch), Fg: k.fg, Bg: k.bg, Upcase: k.upcase})
		}
		ls[y] = l
	}
	return ls
}

// screenPage is v's page as the presenter draws it: the page's pictures
// and boxes and the screens' text over them.
// contentPictures are the original art a screen of its own shows from the
// engine's page: the picker's pictures in their cells, the portrait and
// combat icon on the sheets; never a frame or a fill (the credits sit on
// black, not over the title picture).
func contentPictures(v engine.View) []engine.Picture {
	var in []image.Rectangle
	switch {
	case v.Slots != nil, v.IconEditor != nil && v.Picker == nil:
	case v.Picker != nil:
		for _, r := range v.Picker.Cells {
			in = append(in, r.Pixels())
		}
	default:
		in = append(in, v.Portrait.Pixels(), v.IconBox.Pixels())
		if cr := v.Creation; cr != nil {
			in = append(in, cr.Head.Pixels(), cr.Body.Pixels(), cr.Icon.Pixels())
		}
	}
	var out []engine.Picture
	for _, pc := range v.Pictures {
		if pc.Img == nil || pc.File == "" {
			continue
		}
		if slices.ContainsFunc(in, func(r image.Rectangle) bool { return !r.Empty() && pc.At.Overlaps(r) }) {
			out = append(out, pc)
		}
	}
	return out
}

func screenPage(v engine.View) *cellPage {
	v.Pictures = contentPictures(v) // nothing of the engine's page: only the art in the screen's own places
	if v.Slots != nil {
		v.SheetPage, v.Picker, v.IconEditor = nil, nil, nil
	}
	if v.IconEditor != nil && v.Picker == nil {
		v.Ink[inkBlack] = color.RGBA{A: 255} // the combat palette shows black dark grey (for the icon's black): the page black as every screen
	}
	c := newCellPage(v)
	c.texts(v)
	if s := v.SheetPage; s != nil {
		c.sheet(s, v)
	}
	if pk := v.Picker; pk != nil {
		c.picker(pk)
	}
	if ed := v.IconEditor; ed != nil {
		c.iconEditor(ed)
	}
	if (v.Creation != nil || v.IconEditor != nil) && v.Picker == nil && v.Slots == nil { // the bar on the prompt row
		for _, w := range barWords(v) {
			c.str(w.x, c.rows-1, inkWhite, w.label[:1])
			c.str(w.x+1, c.rows-1, inkGreen, w.label[1:])
		}
	}
	if sl := v.Slots; sl != nil {
		c.box(1, 1, c.cols-2, c.rows-2, inkWhite, "Saved Games")
		if !sl.CanSave && !sl.LoadOnly {
			c.str(2, c.rows-4, inkYellow, "No saving now") // promptRow-3
		}
		for _, w := range slotFooter(sl, c.cols) {
			c.str(w.x, c.rows-3, inkYellow, w.label) // promptRow-2
		}
	}
	return c
}

// footWord is a word of the slot page's footer: its text, the key it
// stands for (footPick, L, D, footBack) and its column.
type footWord struct {
	label string
	key   byte
	x     int
}

// The footer words that are no letter: Enter (pick the row) and Esc (back).
const footPick, footBack byte = 1, 2

// slotFooter is the slot page's footer on a page cols wide, as the
// original-style F10 page's: spread over the row inside the frame.
func slotFooter(sl *engine.Slots, cols int) []footWord {
	ws := []footWord{{"Enter=save", footPick, 0}, {"L=load", 'L', 0}, {"D=delete", 'D', 0}, {"Esc=back", footBack, 0}}
	switch {
	case sl.LoadOnly:
		ws = append([]footWord{{"Enter=load", footPick, 0}}, ws[2:]...)
	case !sl.CanSave:
		ws = ws[1:]
	}
	gap := cols - 4
	for _, w := range ws {
		gap -= len(w.label)
	}
	gap /= len(ws) - 1
	x := 2
	for i := range ws {
		ws[i].x, x = x, x+len(ws[i].label)+gap
	}
	return ws
}

// texts lays out the game's text: the text pane, the prompt row, the view
// window's text, the party panel (roster, status line), after-combat and
// the credits.
func (c *cellPage) texts(v engine.View) {
	for _, r := range v.Prompt {
		c.run(r, c.rows-1)
	}
	if v.Credits { // drawCredits' two boxes: the title's, the lines'
		c.border(1, 1, c.cols-2, 3)
		c.border(1, 5, c.cols-2, c.rows-2)
		for _, l := range engine.Credits {
			c.str(l.X, l.Y, l.Colour, l.Text)
		}
		if last := engine.Credits[len(engine.Credits)-1]; c.rows-3 >= last.Y+2 { // the remaster's own lines, on a page tall enough
			for i, s := range engine.RemasterCredit {
				c.str((c.cols-len(s))/2, c.rows-4+i, inkYellow, s)
			}
		}
	}
}

// cellBorder draws View.Border's cells into m round the cells of r. The
// page boxes are text-grid cells drawn into the page's CPU image (uploaded
// once per View, cropped by the adventure frame), not window()'s GPU pass.
func cellBorder(m *image.RGBA, v engine.View, r engine.CellRect) {
	f := r.Framed()
	tile := func(i, x, y int) {
		if t := v.Border[i]; t != nil {
			draw.Draw(m, image.Rect(x*8, y*8, x*8+8, y*8+8), t, t.Bounds().Min, draw.Over)
		}
	}
	for _, p := range [][2]int{{f.X1, f.Y1}, {f.X2, f.Y1}, {f.X1, f.Y2}, {f.X2, f.Y2}} {
		tile(0, p[0], p[1])
	}
	for x := f.X1 + 1; x < f.X2; x++ {
		tile(2, x, f.Y1)
		tile(2, x, f.Y2)
	}
	for y := f.Y1 + 1; y < f.Y2; y++ {
		tile(1, f.X1, y)
		tile(1, f.X2, y)
	}
}

// draw draws the page into m (its pictures, the borders' cells, the text
// in the game's font).
func (c *cellPage) draw(m *image.RGBA, v engine.View, note Note) {
	drawPictures(m, c.pics, image.Point{})
	for _, r := range c.boxes {
		cellBorder(m, v, r)
	}
	for y, row := range c.cells {
		for x, k := range row {
			if k.set {
				glyphAt(m, note.Font(k.ch, k.upcase), x*8, y*8, k.fg, k.bg)
			}
		}
	}
}

// fit is the largest rectangle of b's shape inside r, centred across and
// on r's bottom edge (a portrait stands on its frame's foot).
func fit(b, r image.Rectangle) image.Rectangle {
	w, h := r.Dx(), b.Dy()*r.Dx()/max(b.Dx(), 1)
	if h > r.Dy() {
		w, h = b.Dx()*r.Dy()/max(b.Dy(), 1), r.Dy()
	}
	return image.Rect(0, 0, w, h).Add(r.Min.Add(image.Pt((r.Dx()-w)/2, r.Dy()-h)))
}

// sheet lays the character sheet out (ovr20:004A CharacterSheet).
func (c *cellPage) sheet(s *engine.SheetPage, v engine.View) {
	right, bottom := c.cols-2, c.rows-3
	c.unset(image.Rect(0, 0, c.cols*8, (bottom+2)*8)) // the sheet's page: ClearScreen came first (the 3D view's status line once wrote over it)
	c.border(1, 1, right, bottom)
	c.str(1, 1, s.NameColour, s.Name)
	if s.NPC {
		c.str(len(s.Name)+3, 1, inkGreen, "(NPC)")
	}
	x := 1
	c.str(x, 3, inkWhite, s.Sex)
	x += len(s.Sex) + 1
	c.str(x, 3, inkWhite, s.Race)
	x += len(s.Race) + 1
	c.str(x, 3, inkWhite, "Age "+strconv.Itoa(s.Age))
	c.str(1, 4, inkWhite, s.Alignment)
	c.str(1, 5, inkWhite, s.Class)
	for i, l := range []string{"STR ", "INT ", "WIS ", "DEX ", "CON ", "CHA "} { // and showStat
		y := i + 7
		c.str(1, y, inkGreen, l)
		c.clear(5, y, 11, y)
		x := 5
		if s.Stats[i] < 10 {
			x++
		}
		c.str(x, y, inkGreen, strconv.Itoa(s.Stats[i]))
		if i == 0 {
			c.str(7, 7, inkGreen, s.Str00)
		}
	}
	if !s.Creating {
		c.str(1, 15, inkWhite, "Level")
		c.str(7, 15, inkWhite, s.Levels)
		c.str(17, 15, inkWhite, "Exp "+strconv.Itoa(s.Exp))
		if s.Next > 0 {
			col := inkWhite
			if s.Exp >= s.Next {
				col = inkGreen
			}
			c.str(17, 16, col, "Next "+strconv.Itoa(s.Next))
		}
	}
	y := 0x11 // showCombatStats
	c.str(1, y, inkWhite, "AC    ")
	c.str(4, y, inkGreen, strconv.Itoa(s.AC))
	c.str(1, y+1, inkWhite, "HP    ")
	hp := inkGreen
	if s.HP < s.MaxHP {
		hp = inkYellow
	}
	c.str(4, y+1, hp, strconv.Itoa(s.HP))
	c.str(9, y, inkWhite, "THAC0   ")
	c.str(15, y, inkGreen, strconv.Itoa(s.THAC0))
	c.str(8, y+1, inkWhite, "Damage        ")
	c.str(15, y+1, inkGreen, s.Damage)
	c.str(0x16+3, y+1, inkWhite, "Movement   ")
	c.str(0x16+0xC, y+1, inkGreen, strconv.Itoa(s.Move))
	if !s.Creating {
		y = 0x14
		c.str(1, y, inkWhite, "Status")
		c.str(8, y, inkGreen, s.Status)
		if s.Effects != "" {
			c.str(1, 0x13, inkWhite, "Effects")
			c.str(9, 0x13, inkGreen, s.Effects)
		}
	}
	if b := s.PortraitAt; b != (engine.CellRect{}) {
		c.border(b.X1, b.Y1, b.X2, b.Y2)
		if s.Portrait != nil {
			c.overs = append(c.overs, overlay{Img: s.Portrait, At: fit(s.Portrait.Bounds(), b.Pixels())}) // the whole box: the rope runs outside its cells
		}
	}
	for p, im := range s.Icons {
		if im != nil {
			x, y := v.IconBox.X1+5*p, v.IconBox.Y1
			c.overs = append(c.overs, overlay{Img: im, At: image.Rect(x*8, y*8, x*8+24, y*8+24)})
		}
	}
	if s.Spells != "" { // SpellList's title over the name row
		c.box(1, 1, right, 1, inkBlack, "")
		c.str(1, 1, s.NameColour, s.Name+"'s")
		c.str(len(s.Name)+4, 1, inkGreen, s.Spells)
	}
}

// picker lays a grid picker's page out (showPicker): the box, the pager,
// the saved characters' names, classes and PNG portraits.
func (c *cellPage) picker(pk *engine.Picker) {
	c.border(1, 1, c.cols-2, pk.Bottom)
	if pk.Pages > 1 {
		c.str(pk.Prev.X1, pk.Prev.Y1, inkWhite, "< page "+strconv.Itoa(pk.Page+1)+"/"+strconv.Itoa(pk.Pages)+" >")
	}
	for k, r := range pk.Cells {
		i := pk.Page*pk.PerPage + k
		if i < len(pk.Portraits) && pk.Portraits[i] != nil {
			c.overs = append(c.overs, overlay{Img: pk.Portraits[i], At: fit(pk.Portraits[i].Bounds(), engine.CellRect{X1: r.X1, Y1: r.Y1, X2: r.X1 + 10, Y2: r.Y1 + 4}.Pixels())})
		}
		if i < len(pk.Labels) {
			c.str(r.X1+12, r.Y1+1, inkWhite, pk.Labels[i])
		}
		if i < len(pk.Classes) && pk.Classes[i] != "" {
			c.str(r.X1+12, r.Y1+3, inkGreen, pk.Classes[i])
		}
	}
}

// iconEditor lays the combat icon editor's page out: the frame, the poses,
// the words, the swatches and the palette (their colours the engine's).
func (c *cellPage) iconEditor(ed *engine.IconEditor) {
	c.border(1, 1, c.cols-2, c.rows-3) // the bar under it
	for p, r := range ed.PoseAt {
		c.pics = append(c.pics, engine.Picture{At: r.Pixels(), Fill: true, Colour: c.ink[8]}) // the pose on grey: its black parts show
		if ed.Poses[p] != nil {
			c.overs = append(c.overs, overlay{Img: ed.Poses[p], At: r.Pixels(), Crisp: true})
		}
	}
	c.str(ed.PoseAt[0].X1+2, ed.PoseAt[0].Y1-1, inkWhite, "ready")
	c.str(ed.PoseAt[1].X1+2, ed.PoseAt[1].Y1-1, inkWhite, "action")
	if ed.Custom { // a dropped PNG: it can only go
		c.str(ed.Remove.X1, ed.Remove.Y1, inkGreen, "Remove")
		return
	}
	c.str(ed.Head.X1, ed.Head.Y1, inkGreen, "Head")
	c.str(ed.Weapon.X1, ed.Weapon.Y1, inkGreen, "Weapon")
	size := "small"
	if ed.Large {
		size = "large"
	}
	c.str(ed.Size.X1, ed.Size.Y1, inkGreen, "Size "+size)
	c.str(ed.Names.X1, ed.Names.Y1-1, inkWhite, "Colours")
	c.str(ed.Parts.X1, ed.Parts.Y1-1, inkWhite, "1  2")
	for i, name := range engine.IconParts {
		c.str(ed.Names.X1, ed.Names.Y1+i, inkGreen, name)
	}
	swatch := func(x, y int, colour, frame color.RGBA) { // 2 cells wide, framed
		r := image.Rect(x*8, y*8, x*8+16, y*8+8)
		c.pics = append(c.pics, engine.Picture{At: r, Fill: true, Colour: frame}, engine.Picture{At: r.Inset(1), Fill: true, Colour: colour})
	}
	for i := range engine.IconParts {
		for sh := range 2 {
			frame := c.ink[8]
			if i == ed.Part && sh == ed.Shade {
				frame = c.ink[15]
			}
			swatch(ed.Parts.X1+3*sh, ed.Parts.Y1+i, ed.Swatch[i][sh], frame)
		}
	}
	for i, n := range ed.Colours {
		if n >= 0 {
			swatch(ed.Palette.X1+2*(i%8), ed.Palette.Y1+i/8, ed.PaletteInk[i], c.ink[8])
		}
	}
}

// iconClick is what a click on cell (cx, cy) of the icon editor does.
func iconClick(ed *engine.IconEditor, cx, cy int) (engine.Action, bool) {
	in := func(r engine.CellRect) bool { return cx >= r.X1 && cx <= r.X2 && cy >= r.Y1 && cy <= r.Y2 }
	if ed.Custom {
		if in(ed.Remove) {
			return engine.Action{Kind: engine.ActIconRemove}, true
		}
		return engine.Action{}, false
	}
	head, weapon := engine.Action{Kind: engine.ActPicker, N: engine.PickIconHead}, engine.Action{Kind: engine.ActPicker, N: engine.PickWeapon}
	switch {
	case in(ed.Head):
		return head, true
	case in(ed.Weapon):
		return weapon, true
	case in(ed.Size):
		return engine.Action{Kind: engine.ActIconSize}, true
	case in(ed.Parts):
		return engine.Action{Kind: engine.ActIconColour, N: cy - ed.Parts.Y1, X: (cx - ed.Parts.X1) / 3, Y: -1}, true
	case in(ed.Palette):
		if v := ed.Colours[(cy-ed.Palette.Y1)*8+(cx-ed.Palette.X1)/2]; v >= 0 {
			return engine.Action{Kind: engine.ActIconColour, N: ed.Part, X: ed.Shade, Y: v}, true
		}
		return engine.Action{}, false
	}
	for _, r := range ed.PoseAt { // the head on the poses' upper part, the weapon below
		if in(engine.CellRect{X1: r.X1, Y1: r.Y1, X2: r.X2, Y2: r.Y1 + 3}) {
			return head, true
		}
		if in(engine.CellRect{X1: r.X1, Y1: r.Y1 + 4, X2: r.X2, Y2: r.Y2}) {
			return weapon, true
		}
	}
	return engine.Action{}, false
}
