package remaster

import (
	"image"
	"image/color"
	"image/draw"
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
	for _, p := range v.Pictures { // the borders round the boxes: View.Border's cells, the title on the top row
		if !p.Border {
			c.pics = append(c.pics, p)
			continue
		}
		f := p.At
		r := engine.CellRect{X1: f.Min.X/8 + 1, Y1: f.Min.Y/8 + 1, X2: f.Max.X/8 - 2, Y2: f.Max.Y/8 - 2}
		c.border(r.X1, r.Y1, r.X2, r.Y2)
		if t := p.Title; t != "" {
			c.run(engine.PaneRun{X: (r.X1 + r.X2 - len(t)) / 2, Text: t, Fg: p.Colour, Bg: c.ink[inkBlack], Upcase: true}, r.Y1)
		}
	}
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
func screenPage(v engine.View) *cellPage {
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
	if sl := v.Slots; sl != nil { // over the page under (DrawBoxTitled)
		c.box(1, 1, c.cols-2, c.rows-2, inkWhite, "Saved Games")
		if !sl.CanSave && !sl.LoadOnly {
			c.str(2, c.rows-4, inkYellow, "No saving now") // promptRow-3
		}
	}
	return c
}

// texts lays out the game's text: the text pane, the prompt row, the view
// window's text, the party panel (roster, status line), after-combat and
// the credits.
func (c *cellPage) texts(v engine.View) {
	if pn := v.Pane; pn != nil {
		for i, l := range pn.Lines {
			for _, r := range l {
				r.X += engine.TextPane.X1
				c.run(r, engine.TextPane.Y1+i)
			}
		}
	}
	for _, r := range v.Prompt {
		c.run(r, c.rows-1)
	}
	for y, l := range v.ViewText {
		for _, r := range l {
			c.run(r, y+1)
		}
	}
	if v.Status != "" {
		c.str(engine.StatusAt.X, engine.StatusAt.Y, inkGreen, v.Status)
	}
	if v.Roster {
		c.roster(v)
	}
	if v.Minimap && v.Map != nil {
		n := 16 * engine.MinimapCell
		c.pics = append(c.pics, engine.Picture{Img: mapImage(v.Map, engine.MinimapCell, c.ink, nil), At: image.Rectangle{Min: engine.MinimapAt, Max: engine.MinimapAt.Add(image.Pt(n, n))}})
	}
	if r := v.Results; r != nil {
		c.results(r)
	}
	if v.Credits {
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

// roster = 00BA:0767 drawPartyList while adventuring: the names (the one
// highlighted white, on blue in a member pick), AC and HP right-aligned.
func (c *cellPage) roster(v engine.View) {
	x, y := engine.RosterAt.X, engine.RosterAt.Y
	c.str(x, y, inkWhite, "Name")
	c.str(33, y, inkWhite, "AC  HP")
	for i, m := range v.Party {
		y := y + 2 + i
		switch {
		case m.Selected && choosing(v):
			c.run(engine.PaneRun{X: x, Text: m.Name, Fg: c.ink[inkWhite], Bg: c.ink[1], Upcase: true}, y)
		case m.Selected:
			c.str(x, y, inkWhite, m.Name)
		default:
			c.str(x, y, m.Colour, m.Name)
		}
		ac := strconv.Itoa(m.AC) // 00BA:08EF
		c.str(0x23-len(ac), y, inkGreen, ac)
		hp, hpc := strconv.Itoa(m.HP), inkGreen // 00BA:0973
		if m.HP < m.MaxHP {
			hpc = inkYellow
		}
		c.str(0x27-len(hp), y, hpc, hp)
	}
}

// rosterRow is the member whose roster row is at cell (cx, cy) (-1: none).
func rosterRow(v engine.View, cx, cy int) int {
	i := cy - engine.RosterAt.Y - 2
	if !v.Roster || cx < engine.RosterAt.X || cx > 0x26 || i < 0 || i >= len(v.Party) {
		return -1
	}
	return i
}

// results lays the after-combat page out (ovr06: hiredShare, showResult,
// the party destroyed) in the window's box.
func (c *cellPage) results(r *engine.Results) {
	right, bottom := c.cols-2, c.rows-3
	wrap := func(s string, x1, y1, x2 int) {
		c.clear(x1, y1, x2, bottom)
		for i, l := range engine.Wrap(s, x1, x2) {
			if y1+i <= bottom {
				c.str(x1, y1+i, inkGreen, l)
			}
		}
	}
	for k, s := range r.Shares {
		wrap(s, 5, 5+2*k, 34)
	}
	if r.Outcome != "" {
		c.str(1, 3, inkGreen, r.Outcome)
		c.str(1, 5, inkGreen, r.Exp)
		c.str(1, 7, inkGreen, "experience points.")
	}
	if r.Taken != "" {
		wrap(r.Taken, 1, 9, right)
	}
	for i, h := range r.Harmed {
		if y := 0xC + i; y <= 0x15 {
			c.str(1, y, inkCyan, h.Name[:min(len(h.Name), 15)])
			c.str(18, y, 12, h.What[:min(len(h.What), max(0, right-18))])
		}
	}
	if r.End != "" {
		wrap(r.End, r.EndAt.X1, r.EndAt.Y1, r.EndAt.X2)
	}
}

// draw draws the page into m (its pictures, the borders' cells, the text
// in the game's font).
func (c *cellPage) draw(m *image.RGBA, v engine.View, note Note) {
	drawPictures(m, c.pics, image.Point{})
	for _, r := range c.boxes {
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
	c.clear(12, 7, 26, 14) // showMoney
	for i, m := range s.Money {
		c.str(0x14-len(m.Name), 7+i, inkGreen, m.Name)
		c.str(21, 7+i, inkGreen, strconv.Itoa(m.N))
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
	if !s.Creating {
		c.str(0x16, y, inkWhite, "Party items      ")
		c.str(0x16+0xC, y, inkGreen, s.PartyItems)
	}
	c.str(0x16+3, y+1, inkWhite, "Movement   ")
	c.str(0x16+0xC, y+1, inkGreen, strconv.Itoa(s.Move))
	if !s.Creating {
		y = 0x14
		if s.Weapon != "" {
			c.str(1, y, inkWhite, "Weapon")
			c.str(8, y, inkGreen, s.Weapon)
		}
		y++
		if s.Armor != "" {
			c.str(2, y, inkWhite, "Armor")
			c.str(8, y, inkGreen, s.Armor)
		}
		y++
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

// iconEditor lays the combat icon editor's page out (the engine's
// icondesign.go draws the swatches): the frame, the poses, the words.
func (c *cellPage) iconEditor(ed *engine.IconEditor) {
	c.border(1, 1, c.cols-2, c.rows-3)
	for p, r := range ed.PoseAt {
		if ed.Poses[p] != nil {
			c.overs = append(c.overs, overlay{Img: ed.Poses[p], At: r.Pixels()})
		}
	}
	c.str(ed.PoseAt[0].X1+2, ed.PoseAt[0].Y1-1, inkWhite, "ready")
	c.str(ed.PoseAt[1].X1+2, ed.PoseAt[1].Y1-1, inkWhite, "action")
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
}

// iconClick is what a click on cell (cx, cy) of the icon editor does.
func iconClick(ed *engine.IconEditor, cx, cy int) (engine.Action, bool) {
	in := func(r engine.CellRect) bool { return cx >= r.X1 && cx <= r.X2 && cy >= r.Y1 && cy <= r.Y2 }
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
