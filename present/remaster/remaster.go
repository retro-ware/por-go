// Package remaster shows the game with the Blobber renderer: the 3D view
// fills the window, and the original's windows are laid over it. It only
// reads the game, through engine.View, so drawing never touches the
// game's state.
package remaster

import (
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"slices"
	"strings"
	"time"

	"gpu"
	"gpu/vector"

	"blobber"
	"blobber/maps"

	"por/engine"
	"por/pal"
	"por/present/battle"
)

// Presenter draws the remastered screen. Screens it has no remaster for
// (no remastered map, sheets, shops, the title) it draws as the original
// laid them out, from the View's page (page.go).
type Presenter struct {
	pg page // the original's screen, drawn from the View

	view        *blobber.View
	failed      bool                        // the renderer could not start: classic from then on
	scenes      map[string]*blobber.Scene   // parsed maps by name
	shown       string                      // the scene the view shows
	last        time.Time                   // when the animation last advanced
	ticks       float64                     // animation time not yet run, in 1/60 s ticks
	border      borders                     // the game's window border
	bars        *gpu.Image                  // bars.png
	cardImgs    []*gpu.Image                // the cards, drawn at scale 1
	faceImgs    map[image.Image]*gpu.Image  // the faces on the GPU
	labels      map[string]*gpu.Image       // card labels in the game's font
	glyph       func(byte) [8]byte          // the game's font
	main        bool                        // the main menu is up (mainmenu.go)
	ask         bool                        // ... with a question over it
	logo        *gpu.Image                  // View.Logo on the GPU ...
	logoSrc     image.Image                 // ... uploaded from this
	backdrop    *gpu.Image                  // View.Backdrop on the GPU ...
	backdropSrc image.Image                 // ... uploaded from this
	mainAt      time.Time                   // when the main menu first showed the backdrop
	layer       *gpu.Image                  // the main menu drawn for its fade in
	tallImg     *gpu.Image                  // the credits or saved games, spread (credits.go)
	cards       []image.Rectangle           // the party's cards on screen in the 3D view, as last drawn (none elsewhere)
	faces       map[image.Image]image.Image // card art cropped from each paper-doll head
	inside      bool                        // the view has the inside ceiling
	active      bool                        // the last frame was drawn remastered
	menu        bool                        // the vertical menu is open
	sel         int                         // its highlighted item
	asked       string                      // the question menu last opened by itself (menuID); send clears it on a pick, so the same menu back after one (a refusal, back from Browse) opens again
	answered    [][8]byte                   // the text pane under the last question: once it is answered, that text is done
	mapImg      *gpu.Image                  // View.Map on the GPU
	mapSrc      image.Image                 // the View.Map it holds
	menuRects   []image.Rectangle           // the vertical menu's items on screen, as last drawn
	winRects    []image.Rectangle           // the Esc menu's lines on screen, as last drawn
	menuSel     int                         // the game's own choice in it
	noMenu      bool                        // nothing to pick: any click answers
	rows        []shownRows                 // the original's rows on screen, as last drawn
	pointer     image.Point                 // the mouse, as Hover last saw it (keys move the highlight while it rests)
	menuImg     *gpu.Image                  // the vertical menu, drawn at scale 1
	menuBuf     []byte                      // its pixels
	battle      battle.Presenter            // the combat map in 3D
	mapBottom   int                         // screen rows above this are the 3D map
	fight       bool                        // the last frame was the 3D combat map
	inv         inventory                   // the inventory screen (inventory.go)
	fightView   *engine.Battle              // the combat map last drawn
	fightLog    bool                        // the battle Log is up (fight.go)
	rowsImg     *gpu.Image                  // the combat rows, drawn at scale 1
	typing      bool                        // a line is being typed (input.go) ...
	typed       string                      // ... and reads this
	hovered     [3]int                      // the spot and cell the pointer was last on (Mouse)
	cardHover   int                         // 1 + the card the pointer is on (0 none): its name and class show beside it
	tipImg      *gpu.Image                  // the card tip's box, drawn at scale 1 (drawMenu's image, swapped in)
	tipBuf      []byte                      // its pixels
}

// Draw shows the game on dst and returns where the whole page went
// (zero when it is not on screen: the mouse then points at nothing).
func (p *Presenter) Draw(dst *gpu.Image, note Note, v engine.View) Rect {
	p.setup(note, v)
	p.fight = v.Battle != nil && !p.failed
	p.rows, p.noMenu = p.rows[:0], v.Menu == nil
	if p.fight {
		p.shown, p.active, p.menu = "", false, false
		p.fightView = v.Battle
		p.battle.Draw(dst, v.Battle)
		p.drawFight(dst, note.Glyph, v)
		return Rect{}
	}
	if p.main = v.MainMenu != nil; p.main {
		p.shown, p.active, p.menu = "", false, false
		p.drawMain(dst, v)
		return Rect{}
	}
	if v.Credits {
		p.shown, p.active, p.menu, p.cards = "", false, false, nil
		return p.drawCredits(dst, v, note)
	}
	if v.Slots {
		p.shown, p.active, p.menu, p.cards = "", false, false, nil
		return p.drawSlots(dst, v, note)
	}
	sc := p.scene(v)
	if sc == nil || !v.Choosing && (!v.MainFrame || (v.Window != engine.WinThreeD && v.Window != engine.WinPicture && v.Window != engine.WinText)) { // a member pick (Choosing) shows the view and the cards whatever page asked
		p.shown, p.active, p.menu, p.cards = "", false, false, nil // back from elsewhere: no tween from where the view was
		p.asked = ""                                               // and a question still waiting on return (View from the loot menu) pops up again
		return p.drawPage(dst, v, note)
	}
	p.active = true
	frame := p.pageImage(v, note)
	cam := blobber.Camera{X: v.X, Y: v.Y, Dir: int(v.Facing)}
	if name := areaMaps[v.Area]; name != p.shown {
		p.view.Show(sc, cam)
		p.shown, p.inside = name, false
	} else {
		p.view.Follow(cam)
	}
	if v.Inside != p.inside { // indoors the original shows the inside sky: here, a ceiling
		p.inside = v.Inside
		if p.inside {
			blobber.SetFlats(sc.Floor, "")
		} else {
			blobber.SetFlats(sc.Floor, sc.Ceiling)
		}
	}
	blobber.Night = !v.Day
	p.animate()
	p.view.Draw(dst)
	p.drawMap(dst, v.Map, len(v.Party))
	p.drawParty(dst, v.Party, cardAt(dst))
	p.view.DrawMenu(dst) // over the map and cards
	p.cards = p.cards[:0]
	for i := range v.Party {
		p.cards = append(p.cards, cardAt(dst)(i, len(v.Party)))
	}
	p.drawCardTip(dst, v)
	k := scale(dst)
	b := dst.Bounds()
	if n := len(v.Party); n > 0 { // pop-ups go between the card columns
		b.Min.X, b.Max.X = cardAt(dst)(0, n).Max.X, cardAt(dst)(n-1, n).Min.X
	}
	// the picture at the top, the text in the middle, an answer's menu right under it
	pic, txt := engine.InnerWindow.Framed(), engine.TextPane.Framed()
	ph, th := float64(pic.Pixels().Dy()), float64(txt.Pixels().Dy())
	pk := min(k, max(1, float64(min(int(float64(b.Dy())/2/(ph+th/2)), b.Dx()/pal.W))))
	cx, cy := (b.Min.X+b.Max.X)/2, b.Dy()/2
	if v.Window == engine.WinText { // text all over the view window, not just the picture frame
		pic = engine.ViewWindow.Framed()
	}
	if v.MainFrame && (v.Window == engine.WinPicture || v.Window == engine.WinText) { // not the page a member pick came from
		pp, py := pk, int(float64(cy)-pk*th/2)/2
		if v.Window == engine.WinPicture && blobber.Settings.PicDouble { // the pixel art at twice the pop-ups' scale (Esc, Graphics), as far as the text box and a menu still fit under it; the text box moves down under it
			pp = max(pk, min(2*pk, float64(int((float64(b.Dy())-pk*(th+7*8))/ph))))
			py = int(pp*ph/2) + int(pk*4)
			cy = py + int(pp*ph/2) + int(pk*(4+th/2))
		}
		drawBox(dst, frame, p.border.tiles, pic, pp, cx, py)
	}
	pane, asking := textPane(v), v.Menu != nil && !v.Command && !noChoice(v)
	if asking {
		p.answered = pane
	}
	shown := v.TextShown && v.MainFrame && (asking || !slices.Equal(pane, p.answered))     // back from a question with its text unchanged: the encounter is over
	if shown && v.Command && v.Window != engine.WinPicture && v.Window != engine.WinText { // the text under the command bar: along the bottom edge
		drawBox(dst, frame, p.border.tiles, txt, pk, cx, b.Dy()-int(pk*th/2))
	} else if shown { // a question, or a message to confirm (Enter, a click): a box in the middle, its choices under it
		drawBox(dst, frame, p.border.tiles, txt, pk, cx, cy)
	}
	if v.Menu != nil && !v.Command && !noChoice(v) && !v.Choosing { // a question: its choices pop up at once
		if id := menuID(v.Menu); id != p.asked {
			p.asked, p.menu, p.sel = id, true, v.Menu.Sel
		}
	} else {
		p.asked = ""
	}
	if v.Choosing && v.Menu != nil { // a member pick: the question alone in the middle, the cards round it take the pick (a click, F1-F8, the arrows and Enter; Esc or a right click give up)
		p.menu = false
		p.drawMenu(dst, note.Glyph, "", []string{strings.TrimSpace(v.Menu.Prompt)}, nil, -1, pk, cx, b.Dy()/2-int(pk*12))
		p.menuRects = p.menuRects[:0]
	} else if v.Menu == nil { // typed input and "press a key" prompts: nothing to show (the original's prompt line stays off the remastered screens)
		p.menu = false
	} else if p.menu {
		lines, keys := make([]string, len(v.Menu.Items)), make([]byte, len(v.Menu.Items))
		for i, it := range v.Menu.Items {
			lines[i], keys[i] = it.Text, it.Key
		}
		p.sel = min(p.sel, len(lines)-1)
		top := cy + int(pk*th/2) + int(pk*8)
		if v.Command || !shown { // no text box above: in the middle
			top = b.Dy()/2 - int(pk*float64(len(lines)+4)*4)
		}
		p.menuSel = v.Menu.Sel
		p.drawMenu(dst, note.Glyph, v.Menu.Prompt, lines, keys, p.sel, pk, cx, top)
	}
	if note.Show { // the toast: pageImage's bottom line (yellow on black) on a black band along the very bottom of the screen, no border
		h := int(pk * 8)
		dst.SubImage(image.Rect(b.Min.X, b.Max.Y-h, b.Max.X, b.Max.Y)).(*gpu.Image).Fill(color.Black)
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(pk, pk)
		op.GeoM.Translate(float64(cx)-pk*float64(pal.W)/2, float64(b.Max.Y-h))
		dst.DrawImage(frame.SubImage(image.Rect(0, pal.H-8, pal.W, pal.H)).(*gpu.Image), op)
	}
	return Rect{}
}

// textPane is the text pane's glyphs.
func textPane(v engine.View) [][8]byte {
	r := engine.TextPane
	var g [][8]byte
	for y := r.Y1; y <= r.Y2; y++ {
		for x := r.X1; x <= r.X2; x++ {
			g = append(g, v.Text[y][x].Glyph)
		}
	}
	return g
}

// Key turns window key k into actions (Actions hands them over). When the
// remastered view is up, Enter opens the vertical menu in place of the
// menu bar and Esc on the command bar the renderer's options.
func (p *Presenter) Key(k uint16, v engine.View) {
	if !p.key(k, v) {
		p.keyAct(k, v)
	}
}

// key takes k on the remastered screens; false leaves it to keyAct.
func (p *Presenter) key(k uint16, v engine.View) bool {
	if v.Inventory != nil && p.inv.view != nil && !v.Choosing {
		return p.inventoryKey(k)
	}
	if p.fight {
		return p.fightKey(k)
	}
	if !p.active || p.view == nil {
		p.menu = false
		return false
	}
	if p.view.OptionsOpen() {
		return true
	}
	m := v.Menu
	if !p.menu {
		switch {
		case v.Choosing: // a member pick has no menu: every key goes to the portraits (keyAct)
			return false
		case k == 13 && noChoice(v): // "press enter to continue": nothing to pick
			return false
		case k == 13 && m != nil:
			p.menu, p.sel = true, m.Sel
		case k == 27 && v.Command:
			p.view.OpenOptions()
		default:
			return false
		}
		return true
	}
	if m == nil {
		p.menu = false
		return false
	}
	n := len(m.Items)
	switch {
	case k == 0x4800:
		p.sel = (p.sel + n - 1) % n
	case k == 0x5000:
		p.sel = (p.sel + 1) % n
	case k == 13:
		p.menu = false
		if p.sel == m.Sel { // the game's own choice: Enter picks it (some prompts take nothing else)
			return false
		}
		p.send(engine.ActPick, p.sel)
	case k == 27:
		p.menu = false
	default:
		for _, it := range m.Items { // a hotkey still picks its item
			if c := byte(k); k < 128 && (c == it.Key || c-'a'+'A' == it.Key) {
				p.menu = false
				return false
			}
		}
	}
	return true
}

// Click takes a click at screen pixel (x, y) on the remastered screens: a
// portrait, an action bar button, or an item of the vertical menu. It
// reports whether it was taken.
func (p *Presenter) Click(x, y int) bool {
	if p.main {
		return p.mainClick(x, y)
	}
	if !p.fight {
		for i, r := range p.cards { // a portrait: that member's View page on the command bar, else the member a bar is for
			if image.Pt(x, y).In(r) && p.active {
				if gpu.IsKeyPressed(gpu.KeyShift) { // Shift-click: the next head
					p.send(engine.ActHead, i)
				} else {
					p.send(engine.ActMember, i)
				}
				return true
			}
		}
		if !p.menu || !p.active || p.view == nil || p.view.OptionsOpen() {
			return false
		}
		for i, r := range p.menuRects {
			if image.Pt(x, y).In(r) {
				p.menu = false
				if i == p.menuSel { // the game's own choice: Enter picks it (some prompts take nothing else)
					p.send(engine.ActContinue, 0)
				} else {
					p.send(engine.ActPick, i)
				}
				return true
			}
		}
		return false
	}
	return p.fightClick(x, y)
}

// Hover moves the vertical menu's highlight to the item under screen pixel (x, y).
func (p *Presenter) Hover(x, y int) {
	if p.main {
		p.mainHover(x, y)
		return
	}
	if p.fight {
		for i, r := range p.menuRects {
			if p.battle.Picking && image.Pt(x, y).In(r) {
				p.sel = i
			}
		}
		if !p.battle.Picking && !p.fightLog {
			p.battle.Point(x, y)
		}
		return
	}
	p.cardHover = 1 + cardUnder(p.cards, x, y)
	if !p.menu || image.Pt(x, y) == p.pointer {
		return
	}
	p.pointer = image.Pt(x, y)
	for i, r := range p.menuRects {
		if p.pointer.In(r) {
			p.sel = i
		}
	}
}

// Flow paces a held move key (code, 0 for none) with fluid movement: it
// reports whether to send code now (ready: the game waits for a key), and
// whether the view paces held keys at all (else they repeat as typed).
func (p *Presenter) Flow(code uint16, ready bool) (send, paced bool) {
	if !p.active || p.view == nil || p.menu || p.view.OptionsOpen() {
		if p.view != nil {
			p.view.Flow(false, false)
		}
		return false, false
	}
	return p.view.Flow(code != 0, ready), p.view.Fluid()
}

// Options reports whether the renderer's options page is up; while it is,
// UpdateOptions reads the keyboard for it and the game gets no keys.
func (p *Presenter) Options() bool { return p.view != nil && p.view.OptionsOpen() }

// UpdateOptions runs the options page for one frame.
func (p *Presenter) UpdateOptions() { p.view.UpdateOptions() }

// OptionsMouse hands the pointer to the Esc menu: the line under (x, y)
// highlights, a left click picks or steps it, a right click goes back,
// the wheel steps a setting.
func (p *Presenter) OptionsMouse(x, y int, left, right bool, wheel float64) {
	line := -1
	for i, r := range p.winRects {
		if image.Pt(x, y).In(r) {
			line = i
		}
	}
	p.view.PointOptions(line, left, right, int(math.Copysign(math.Ceil(math.Abs(wheel)), wheel)))
}

// noChoice reports a question with one answer ("press enter to continue"):
// a message, not a choice.
func noChoice(v engine.View) bool {
	return v.Menu != nil && !v.Command && len(v.Menu.Items) == 1 && !v.Choosing // a member pick's lone Select is still a choice: whom
}

// cardUnder is the card at screen pixel (x, y), -1 for none.
func cardUnder(cards []image.Rectangle, x, y int) int {
	for i, r := range cards {
		if image.Pt(x, y).In(r) {
			return i
		}
	}
	return -1
}

// menuID tells menus apart: prompt and items.
func menuID(m *engine.Menu) string {
	id := m.Prompt
	for _, it := range m.Items {
		id += "|" + it.Text
	}
	return id
}

// cardAt lays the party's cards along the window's edges: the first half
// of the party down the left, the rest down the right, at the largest
// whole scale where a column of four fits (eight with two NPCs).
func cardAt(dst *gpu.Image) func(i, n int) image.Rectangle {
	b := dst.Bounds()
	k := cardScale(dst)
	return func(i, n int) image.Rectangle {
		left := (n + 1) / 2
		col, row, per := 0, i, left
		if i >= left {
			col, row, per = 1, i-left, n-left
		}
		y := b.Dy() - (per-row)*(cardH+cardGap)*k + cardFoot*k // the bars end a gap above the bottom
		x := cardGap * k
		if col == 1 {
			x = b.Dx() - cardGap*k - cardW*k
		}
		return image.Rect(x, y, x+cardW*k, y+cardH*k)
	}
}

// mapRect is where the map goes: a square right-aligned with the cards'
// right edge, a card gap below the top and above the right column's top
// card (n cards in all).
func mapRect(dst *gpu.Image, n int) image.Rectangle {
	b := dst.Bounds()
	k := cardScale(dst)
	gap := cardGap * k
	top := cardAt(dst)(n-1, max(n, 2)) // the right column's last card...
	per := max(n, 2) - (max(n, 2)+1)/2
	y := top.Min.Y - (per-1)*(cardH+cardGap)*k // ...up to its first
	side := min(y-2*gap, b.Dx()/2)
	if blobber.Settings.MapSmall {
		side /= 2
	}
	return image.Rect(b.Dx()-gap-side, gap, b.Dx()-gap, gap+side)
}

const cardGap, cardRows = 8, 4

// cardScale is the cards' whole scale: the largest where a column of four
// fits under the map at its smallest.
func cardScale(dst *gpu.Image) int {
	return max(1, min(dst.Bounds().Dx()/480, dst.Bounds().Dy()/360, (dst.Bounds().Dy()-mapSide-2-cardGap)/(cardRows*(cardH+cardGap))))
}

// mapSide is View.Map's side in pixels.
const mapSide = 16 * engine.MapCell

// drawMap shows the map as the full map draws it (View.Map) in mapRect.
func (p *Presenter) drawMap(dst *gpu.Image, m image.Image, n int) {
	if m == nil || blobber.Settings.MapOff {
		return
	}
	if p.mapImg == nil {
		p.mapImg = gpu.NewImage(mapSide, mapSide)
	}
	if m != p.mapSrc { // a new View: upload it once
		p.mapImg.WritePixels(m.(*image.RGBA).Pix)
		p.mapSrc = m
	}
	r := mapRect(dst, n)
	a := float32(blobber.Settings.MapAlpha) // the opacity slider (Esc, Options)
	vector.StrokeRect(dst, float32(r.Min.X)+.5, float32(r.Min.Y)+.5, float32(r.Dx())-1, float32(r.Dy())-1, 1, color.NRGBA{0x80, 0x80, 0x80, uint8(255 * a)}, false)
	vector.DrawFilledRect(dst, float32(r.Min.X+1), float32(r.Min.Y+1), float32(r.Dx()-2), float32(r.Dy()-2), color.NRGBA{A: uint8(255 * a)}, false)
	k := float64(r.Dx()-2) / mapSide // any scale: the square keeps the aspect
	op := &gpu.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(a)
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(r.Min.X)+(float64(r.Dx())-mapSide*k)/2, float64(r.Min.Y)+(float64(r.Dy())-mapSide*k)/2)
	dst.DrawImage(p.mapImg, op)
}

// scene is the remastered map of the area v shows, or nil.
func (p *Presenter) scene(v engine.View) *blobber.Scene {
	name, ok := areaMaps[v.Area]
	if !ok || p.failed {
		return nil
	}
	if p.view == nil {
		var err error
		if p.view, err = blobber.NewView(); err != nil {
			log.Println("remaster:", err)
			p.failed = true
			return nil
		}
		blobber.Window = p.keyWindow // its menu and options in the game's box
	}
	if sc, ok := p.scenes[name]; ok {
		return sc
	}
	var sc *blobber.Scene
	if f, err := maps.Levels.Open(name + ".map"); err != nil {
		log.Println("remaster:", err)
	} else {
		defer f.Close()
		if sc, _, err = blobber.ParseScene(f); err != nil {
			log.Printf("remaster: %s.map: %v", name, err)
		}
	}
	if p.scenes == nil {
		p.scenes = map[string]*blobber.Scene{}
	}
	p.scenes[name] = sc // nil too: a broken map stays classic
	return sc
}

// animate runs the renderer's 60 ticks a second, whatever the frame rate.
func (p *Presenter) animate() {
	now := time.Now()
	if !p.last.IsZero() {
		p.ticks = min(p.ticks+now.Sub(p.last).Seconds()*60, 6) // after a pause, no long catch-up
	}
	p.last = now
	for ; p.ticks >= 1; p.ticks-- {
		p.view.Update()
	}
}

// scale is the largest whole factor the classic screen fits dst at (at
// least 1): pixels stay square.
func scale(dst *gpu.Image) float64 {
	b := dst.Bounds()
	return float64(max(1, min(b.Dx()/pal.W, b.Dy()/pal.H)))
}

// drawBox copies box r of the classic frame onto dst at scale k, centred
// on cx, cy: its inside, in a plain border (drawBorder) of tiles' cells.
func drawBox(dst, frame *gpu.Image, tiles [3]*gpu.Image, r engine.CellRect, k float64, cx, cy int) {
	px := r.Pixels()
	var g gpu.GeoM
	g.Scale(k, k)
	g.Translate(float64(cx)-k*float64(px.Dx())/2, float64(cy)-k*float64(px.Dy())/2)
	op := &gpu.DrawImageOptions{GeoM: g}
	op.GeoM.Translate(8*k, 8*k)
	dst.DrawImage(frame.SubImage(px.Inset(8)).(*gpu.Image), op)
	drawBorder(dst, tiles, g, r.X2-r.X1+1, r.Y2-r.Y1+1)
}

// drawBorder draws a cw by ch cell box's border at g in tiles t (the
// game's corner, side and top cells: borders), as DrawBorder does.
func drawBorder(dst *gpu.Image, t [3]*gpu.Image, g gpu.GeoM, cw, ch int) {
	cell := func(i, x, y int) {
		op := &gpu.DrawImageOptions{}
		op.GeoM.Translate(float64(x*8), float64(y*8))
		op.GeoM.Concat(g)
		dst.DrawImage(t[i], op)
	}
	for x := 1; x < cw-1; x++ {
		cell(2, x, 0)
		cell(2, x, ch-1)
	}
	for y := 1; y < ch-1; y++ {
		cell(1, 0, y)
		cell(1, cw-1, y)
	}
	for _, c := range [][2]int{{0, 0}, {cw - 1, 0}, {0, ch - 1}, {cw - 1, ch - 1}} {
		cell(0, c[0], c[1])
	}
}

// shownRows is a strip of the original's screen (pix) where it shows (at).
type shownRows struct{ at, pix image.Rectangle }

// Cell is the original's text cell under screen pixel (x, y), on the rows
// of its screen shown as they are (menu lines, prompts, the combat panel),
// so its own mouse handling picks from them.
func (p *Presenter) Cell(x, y int) (int, int, bool) {
	for _, s := range p.rows {
		if image.Pt(x, y).In(s.at) {
			px := s.pix.Min.X + (x-s.at.Min.X)*s.pix.Dx()/s.at.Dx()
			py := s.pix.Min.Y + (y-s.at.Min.Y)*s.pix.Dy()/s.at.Dy()
			return px / 8, py / 8, true
		}
	}
	return 0, 0, p.noMenu // "press a key": a click anywhere, as on the original's screen
}

// cardLabels: the sheet's status as the card spells it over the portrait,
// short enough to fit there (a member at 0 hit points is not always dead).
var cardLabels = map[string]string{
	"Animated":    "undead",
	"Running":     "fled",
	"Unconscious": "KO",
	"Dying":       "dying",
	"Dead":        "dead",
	"Poisoned":    "poisoned", // dead of poison, as the sheet puts it
	"Stoned":      "stone",
	"Gone":        "gone",
}

// face is the card art for a head: its lit pixels (the face, hair and
// shoulders) centred in a PortraitSize square, unscaled (nil without a head).
func (p *Presenter) face(head image.Image) image.Image {
	if head == nil {
		return nil
	}
	if f, ok := p.faces[head]; ok {
		return f
	}
	const n = portraitSize
	lit := image.Rectangle{}
	b := head.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, g, bl, a := head.At(x, y).RGBA(); a > 0 && r|g|bl != 0 {
				lit = lit.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	f := image.NewRGBA(image.Rect(0, 0, n, n))
	draw.Draw(f, f.Bounds(), image.Black, image.Point{}, draw.Src)
	c := lit.Min.Add(lit.Max).Div(2) // centre on centre; a part wider than the square is cut evenly
	draw.Draw(f, f.Bounds(), head, c.Sub(image.Pt(n/2, n/2)), draw.Over)
	if p.faces == nil {
		p.faces = map[image.Image]image.Image{}
	}
	p.faces[head] = f
	return f
}
