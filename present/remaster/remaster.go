// Package remaster shows the game with the Blobber renderer: the 3D view
// fills the window, and the original's windows are laid over it. It only
// reads the game, through engine.View, so drawing never touches the
// game's state.
package remaster

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"gpu"
	"gpu/vector"

	xdraw "golang.org/x/image/draw"

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

	pauseFrom time.Time // a script's pause (AskPause) began; zero: none up

	view        *blobber.View
	scenes      map[string]*blobber.Scene   // parsed maps by name
	shown       string                      // the scene the view shows
	last        time.Time                   // when the animation last advanced
	ticks       float64                     // animation time not yet run, in 1/60 s ticks
	border      borders                     // the game's window border
	bars        *gpu.Image                  // bars.png
	cardImgs    []*gpu.Image                // the cards, drawn at scale 1...
	cardKeys    []cardKey                   // ...and what each shows
	faceImgs    map[image.Image]*gpu.Image  // the faces on the GPU
	hiFaces     map[image.Image]*gpu.Image  // the PNG portraits at the cards' window resolution
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
	grid        [2]int                      // the text cells the window holds, as last told to the engine (ActResize)
	gridAge     int                         // frames since
	cards       []image.Rectangle           // the party's cards on screen in the 3D view, as last drawn (none elsewhere)
	faces       map[image.Image]image.Image // card art cropped from each paper-doll head
	inside      bool                        // the view has the inside ceiling
	active      bool                        // the last frame was drawn remastered
	menu        bool                        // the vertical menu is open
	menuFor     string                      // the menu it opened for (menuID): another menu arriving closes it, so one key never answers two menus
	sel         int                         // its highlighted item
	asked       string                      // the question menu last opened by itself (menuID); send clears it on a pick, so the same menu back after one (a refusal, back from Browse) opens again
	answered    string                      // the text pane under the last question: once it is answered, that text is done
	mapImg      *gpu.Image                  // View.Map on the GPU
	mapSrc      *engine.MapSquares          // the View.Map it holds
	menuRects   []image.Rectangle           // the vertical menu's items on screen, as last drawn
	scrollBars  []barHit                    // the scroll bars drawn this frame (scrollbar.go)
	barDrag     *barHit                     // the bar whose thumb the pointer holds
	logTop      int                         // the battle Log's first line shown
	winRects    []image.Rectangle           // the Esc menu's lines on screen, as last drawn
	mainSel     int                         // the main menu's highlight
	mainSub     bool                        // the main menu shows the Party submenu (the character functions)
	mainAsk     int                         // the main menu's question up (askQuit ..)
	noMenu      bool                        // nothing to pick: any click answers
	pointer     image.Point                 // the mouse, as Hover last saw it (keys move the highlight while it rests)
	menuImg     *gpu.Image                  // the vertical menu, drawn at scale 1
	menuBuf     []byte                      // its pixels
	battle      battle.Presenter            // the combat map in 3D
	mapBottom   int                         // screen rows above this are the 3D map
	fight       bool                        // the last frame was the 3D combat map
	inv         inventory                   // the inventory screen (inventory.go)
	fightView   *engine.Battle              // the combat map last drawn
	replay      replay                      // the fight's events still to play
	fightLog    bool                        // the battle Log is up (fight.go)
	fightCard   image.Rectangle             // the active member's card (fight.go): left click the inventory, right click the sheet
	rowsImg     *gpu.Image                  // the combat rows, drawn at scale 1
	rowsKey     string                      // what they show
	typing      bool                        // a line is being typed (input.go) ...
	typed       string                      // ... and reads this ...
	typedCur    int                         // ... its cursor here
	cardHover   int                         // 1 + the card the pointer is on (0 none): its name and class show beside it
	dragFrom    int                         // 1 + the card pressed while the party is being ordered (View.Ordering), it follows the pointer to the card it is dropped on (0 none)
	dragAt      image.Point                 // where the pointer is while a card is dragged
	ordering    bool                        // View.Ordering
	ctx         int                         // 1 + the card whose context menu a right click opened (0 none): drawCardTip
	ctxSel      int                         // its highlighted line
	ctxActs     []engine.ActKind            // its lines' actions, for the member
	ctxKeys     []byte                      // its lines' hotkeys (the first letters)
	ctxRects    []image.Rectangle           // its lines on screen, as last drawn
	ctxBox      image.Rectangle             // where its box went: the spell pick opens there
	pick        []engine.Spell              // View.Ask (AskSpell) as last drawn: a spell to cast is being picked on this screen
	pickAt      image.Rectangle             // the context menu's box the pick list replaces (zero: the top middle, the bar's scale)
	pickSel     int                         // the pick list's highlight ...
	picking     bool                        // ... reset when it opens
	command     bool                        // the party explores (View.Exploring), as last drawn
	cmds        []command                   // the command menu's commands (explore.go), as last drawn
	camp        campUI                      // the camp screens (camp.go)
	shownView   engine.View                 // the View last drawn
	list        listUI                      // View.Ask (AskList)'s highlight (list.go)
	pageMenu    bool                        // a page's question is up (list.go)
	tipImg      *gpu.Image                  // the card tip's box, drawn at scale 1 (drawMenu's image, swapped in)
	tipBuf      []byte                      // its pixels
	scr         screenUI                    // the page screens' questions (screens.go)
	screen      image.Rectangle             // the window, as last drawn
	confirmFor  string                      // the yes/no question (View.Ask, AskConfirm) up, as last drawn ...
	confirmSel  int                         // ... its highlight (Yes 0, No 1) ...
	confirmAt   []image.Rectangle           // ... and its rows on screen
}

// Draw shows the game on dst and returns where the whole page went
// (zero when it is not on screen: the mouse then points at nothing).
func (p *Presenter) Draw(dst *gpu.Image, note Note, v engine.View) Rect {
	if ev := p.playing(); ev != nil && ev.Scene != nil { // the page as it stood at the pause played
		v = *ev.Scene
	} else if ev != nil { // the fight as it stood at the event played
		v.Battle, v.Party = ev.Battle, ev.Party
		v.Ask = nil
	}
	p.scrollBars = p.scrollBars[:0]
	p.battle.Asking = confirm(v) != ""
	r := p.draw(dst, note, v)
	if q := confirm(v); q != "" { // the engine's yes/no question: over everything, starting on No
		if q != p.confirmFor {
			p.confirmFor, p.confirmSel = q, 1
		}
		p.keyWindow(dst, q, []string{"Yes", "No"}, yesNoKeys, nil, p.confirmSel, false)
		p.confirmAt = slices.Clone(p.winRects)
	} else {
		p.confirmFor, p.confirmAt = "", nil
	}
	p.pick = spellPick(v)
	if up := p.pickList() != nil; up != p.picking { // a pick list opens on its first spell
		p.picking, p.pickSel = up, 0
	}
	if p.pick == nil {
		p.pickAt = image.Rectangle{}
	} else { // the spell pick (the combat's Cast list) over whatever screen is up
		k := scale(dst)
		cx, top := dst.Bounds().Dx()/2, int(k)*16
		if at := p.pickAt; at != (image.Rectangle{}) { // from a card's context menu: in its place at its scale, the edge next to the card kept
			k = cardScale(dst)
			w := 0
			for _, s := range p.pick {
				w = max(w, int(k)*8*(len(s.Name)+4)) // as drawMenu's box: a border and a blank column each side
			}
			cx, top = at.Min.X+w/2, at.Min.Y
			if at.Min.X > dst.Bounds().Dx()/2 { // the right column: the box grows to the left
				cx = at.Max.X - w/2
			}
			cx = min(max(cx, w/2), dst.Bounds().Dx()-w/2)
		}
		p.drawPickList(dst, note.Glyph, k, cx, top)
	}
	return r
}

// pickList is the spell list up for a choice: View.Ask (AskSpell) anywhere, else
// the bar's Cast in combat (Picking); nil when none.
func (p *Presenter) pickList() []engine.Spell {
	switch {
	case p.pick != nil:
		return p.pick
	case p.fight && p.battle.Picking && p.fightView != nil && p.fightView.Turn != nil:
		return p.fightView.Turn.Spells
	}
	return nil
}

// drawPickList draws the spell list (pickList) as a menu with hotkeys, its
// box centred on cx with its top at top.
func (p *Presenter) drawPickList(dst *gpu.Image, glyph func(byte) [8]byte, k float64, cx, top int) {
	lines := make([]string, len(p.pickList()))
	for i, s := range p.pickList() {
		lines[i] = s.Name
	}
	p.pickSel = min(max(p.pickSel, 0), len(lines)-1)
	p.drawMenu(dst, glyph, "Cast:", lines, spellKeys(len(lines)), nil, p.pickSel, k, cx, top)
}

// spellKeys are a spell list's hotkeys, as every spell list's (camp,
// training): the digits 1-9 (digitKeys).
func spellKeys(n int) []byte { return digitKeys(make([]byte, n), func(int) bool { return true }) }

// pickKey takes key k while a spell is picked: the arrows, Enter, a hotkey, Esc.
func (p *Presenter) pickKey(k uint16) bool {
	spells := p.pickList()
	if spells == nil || globalKey(k) {
		return false
	}
	switch i, back, _ := menuNav(k, &p.pickSel, spellKeys(len(spells))); {
	case back:
		p.cast(-1)
	case i >= 0:
		p.cast(i)
	}
	return true
}

// pickClick takes a click while a spell is picked: a line casts, elsewhere gives up.
func (p *Presenter) pickClick(x, y int) bool {
	if p.pickList() == nil {
		return false
	}
	p.cast(slices.IndexFunc(p.menuRects, func(r image.Rectangle) bool { return image.Pt(x, y).In(r) }))
	return true
}

// cast casts spell i of the list (-1: none, giving up).
func (p *Presenter) cast(i int) {
	spells, fromBar := p.pickList(), p.pick == nil
	p.battle.Picking = false
	switch {
	case i >= 0 && i < len(spells):
		p.send(engine.ActCast, spells[i].ID)
	case !fromBar: // the game waits for the pick: give it up
		p.send(engine.ActClose, 0)
	}
}

// draw shows the screen the View is: the page, the main menu, a tall page or the 3D view.
func (p *Presenter) draw(dst *gpu.Image, note Note, v engine.View) Rect {
	p.screen = dst.Bounds()
	p.setup(note, v)
	p.command = v.Exploring
	if screenUp(v) && !screenUp(p.shownView) { // camp, the temple, the shop, training start on their main screen
		p.camp.screen, p.camp.sel = campMain, 0
	}
	if askList(v) != nil && askList(p.shownView) == nil { // a list opens at its first line
		p.list = listUI{sel: max(0, slices.IndexFunc(askList(v), func(l engine.ListLine) bool { return !l.Off }))}
	}
	p.shownView = v
	p.pace(v)
	p.pageMenu = false
	p.cmds = commands(v)
	if !p.active {
		p.ctx = 0
	}
	p.fight = v.Battle != nil                                  // the battle view is the only one: the engine has no combat page
	p.noMenu = askMenu(v) == nil || len(askMenu(v).Items) == 1 // a one-item bar (PRESS <ENTER> TO CONTINUE) has nothing to pick either
	p.resize(dst, v)
	if p.fight {
		p.shown, p.active, p.menu = "", false, false
		p.fightView = v.Battle
		p.battle.Draw(dst, v.Battle)
		p.drawFight(dst, note, v)
		if p.view != nil {
			p.view.DrawMenu(dst) // the Esc menu over the battle
		}
		return Rect{}
	}
	if openAt != "" {
		p.drawOpen(dst)
		return Rect{}
	}
	if p.main = v.MainMenu != nil; p.main {
		p.shown, p.active, p.menu = "", false, false
		p.drawMain(dst, v)
		return Rect{}
	}
	sc := p.scene(v)
	if sc == nil || !choosing(v) && (!v.Frame || (v.Window != engine.WinThreeD && v.Window != engine.WinPicture && v.Window != engine.WinText)) { // a member pick (Choosing) shows the view and the cards whatever page asked
		p.shown, p.active, p.menu, p.cards = "", false, false, nil // back from elsewhere: no tween from where the view was
		p.asked = ""                                               // and a question still waiting on return (View from the loot menu) pops up again
		r := p.drawPage(dst, v, note)
		p.scr.at = r
		p.scr.pw, p.scr.ph = pageSize(v)
		p.drawPageMenu(dst, note, v)
		return r
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
	p.drawMap(dst, v, note.Font)
	p.ordering = v.Ordering
	if !p.ordering {
		p.dragFrom = 0
	}
	at := cardAt(dst)
	if p.dragFrom > 0 { // the dragged card hangs under the pointer
		at = func(i, n int) image.Rectangle {
			r := cardAt(dst)(i, n)
			if i == p.dragFrom-1 {
				r = r.Sub(r.Min).Add(p.dragAt).Sub(image.Pt(r.Dx()/2, r.Dy()/2))
			}
			return r
		}
	}
	p.drawParty(dst, v.Party, at)
	p.view.DrawMenu(dst) // over the map and cards
	p.cards = p.cards[:0]
	for i := range v.Party {
		p.cards = append(p.cards, cardAt(dst)(i, len(v.Party)))
	}
	if p.ctx == 0 { // the tooltip; a context menu goes over everything (below)
		p.drawCardTip(dst, v)
	}
	k := scale(dst)
	b := dst.Bounds()
	if n := len(v.Party); n > 0 && blobber.Settings.PartyRow { // pop-ups go over the card row
		b.Max.Y = cardAt(dst)(0, n).Min.Y
	} else if n > 0 { // pop-ups go between the card columns
		b.Min.X, b.Max.X = cardAt(dst)(0, n).Max.X, cardAt(dst)(n-1, n).Min.X
	}
	// the picture at the top, the text in the middle, an answer's menu right under it
	pic, txt := engine.InnerWindow.Framed(), engine.TextPane.Framed()
	ph, th := float64(pic.Pixels().Dy()), float64(txt.Pixels().Dy())
	pk := min(k, max(1, float64(min(int(float64(b.Dy())/2/(ph+th/2)), b.Dx()/pal.W))))
	// the text box and the menus at their sizes (Esc, Graphics), as far as they fit between the cards
	bw, bh := float64(b.Dx()), float64(b.Dy())
	tk := fitScale(pk, blobber.Settings.Text, func(k float64) bool { return k*float64(txt.Pixels().Dx()) <= bw && k*th <= bh/2 })
	mk := fitScale(pk, blobber.Settings.Menu, func(k float64) bool { return k*28*8 <= bw && k*12*8 <= bh/2 })
	cx, cy := (b.Min.X+b.Max.X)/2, b.Dy()/2
	if v.Window == engine.WinText { // text all over the view window, not just the picture frame
		pic = engine.ViewWindow.Framed()
	}
	picBottom := 0                                                                // the bottom of the picture box, when one is up
	if v.Frame && (v.Window == engine.WinPicture || v.Window == engine.WinText) { // not the page a member pick came from
		// the picture at its size as far as it, the text box and a menu fit one under the other: in the middle of the space over the text, else from the top with the text moved down under it
		pw, gap := float64(pic.Pixels().Dx()), pk*4
		pp := fitScale(pk, blobber.Settings.Pic, func(k float64) bool {
			return k*pw <= bw && 3*gap+k*ph+tk*th+mk*7*8 <= bh
		})
		py := int(float64(cy)-tk*th/2) / 2
		if float64(py)-pp*ph/2 < gap || float64(py)+pp*ph/2+gap > float64(cy)-tk*th/2 {
			py = int(gap + pp*ph/2)
			cy = py + int(pp*ph/2+gap+tk*th/2)
		}
		inside := frame.SubImage(pic.Pixels().Inset(8)).(*gpu.Image) // text over the whole view window (WinText): still the page's (step 3)
		if v.Window == engine.WinPicture {
			inside = pictureBox(&p.pg.pic, v.Pictures, pic.Pixels().Inset(8))
		}
		drawBox(dst, inside, p.border.tiles, pic, pp, cx, py)
		picBottom = py + int(pp*ph/2)
	}
	pane, asking := textPane(v), askMenu(v) != nil && !v.Exploring && !noChoice(v)
	if asking {
		p.answered = pane
	}
	shown := v.Frame && v.Pane != nil && v.Pane.Shown() && (asking || pane != p.answered) // back from a question with its text unchanged: the encounter is over
	ask := typing(v) != nil && typing(v).Prompt != ""                                     // a script's input: its question and the line in one pop-up, no text box
	shown = shown && !ask
	if shown && v.Exploring && v.Window != engine.WinPicture && v.Window != engine.WinText { // the text under the command bar: along the bottom edge
		drawBox(dst, paneImage(&p.pg.pane, v.Pane, note), p.border.tiles, txt, tk, cx, b.Dy()-int(tk*th/2))
	} else if shown { // a question, or a message to confirm (Enter, a click): a box in the middle, its choices under it
		drawBox(dst, paneImage(&p.pg.pane, v.Pane, note), p.border.tiles, txt, tk, cx, cy)
	}
	if askMenu(v) != nil && !v.Exploring && !noChoice(v) && !choosing(v) { // a question: its choices pop up at once
		if id := menuID(askMenu(v)); id != p.asked { // not the menu just answered
			p.asked, p.menu, p.menuFor, p.sel = id, true, id, 0
		}
	} else {
		p.asked = ""
	}
	if choosing(v) && askMenu(v) != nil { // a member pick: the question alone in the middle, the cards round it take the pick (a click, F1-F8, the arrows and Enter; Esc or a right click give up)
		p.menu = false
		p.drawMenu(dst, note.Glyph, "", []string{strings.TrimSpace(askMenu(v).Prompt)}, nil, nil, -1, mk, cx, b.Dy()/2-int(mk*12))
		p.menuRects = p.menuRects[:0]
	} else if ask { // the question over the line being typed, its cursor blinking
		p.menu = false
		t, w := typing(v).Text, min(typing(v).Max, 24)
		c := len(t)
		if p.typing {
			t, c = p.typed, p.typedCur
		}
		top := b.Dy()/2 - int(mk*float64(len(wrap(typing(v).Prompt, 24))+5)*4)
		if picBottom > 0 { // never over the picture: right under it
			top = max(top, picBottom+int(pk*4))
		}
		p.drawMenu(dst, note.Glyph, typing(v).Prompt, []string{caretLine(t, c, w, p.blink())}, nil, nil, 0, mk, cx, top)
		p.menuRects = p.menuRects[:0]
	} else if askList(v) != nil && askMenu(v) != nil { // a list and its question (list.go)
		p.menu = false
		p.drawList(dst, note.Glyph, v, mk, cx, max(b.Dy()/2-int(mk*float64(min(len(askList(v)), listRows)+len(askMenu(v).Items)+6)*4), picBottom+int(pk*4)))
	} else if screenUp(v) && spellPick(v) == nil { // camp (camp.go), the temple, the shop, training
		p.menu = false
		p.drawCamp(dst, note.Glyph, v, mk, cx, max(b.Dy()/4, picBottom+int(pk*4)), b.Max.Y)
	} else if v.Exploring && p.menu { // the command menu (explore.go)
		cmds := commands(v)
		lines, keys := make([]string, len(cmds)), make([]byte, len(cmds))
		for i, c := range cmds {
			lines[i], keys[i] = c.text, c.key
		}
		p.sel = min(p.sel, len(lines)-1)
		top := b.Dy()/2 - int(mk*float64(len(lines)+4)*4)
		if picBottom > 0 {
			top = max(top, picBottom+int(pk*4))
		}
		p.drawMenu(dst, note.Glyph, "", lines, keys, nil, p.sel, mk, cx, top)
	} else if askMenu(v) == nil { // "press a key" prompts: nothing to show (the original's prompt line stays off the remastered screens)
		p.menu = false
	} else if p.menu && menuID(askMenu(v)) != p.menuFor { // the menu the popup was opened for is gone (answered): the next one opens on its own key
		p.menu = false
	} else if p.menu {
		lines, keys := make([]string, len(askMenu(v).Items)), make([]byte, len(askMenu(v).Items))
		for i, it := range askMenu(v).Items {
			lines[i], keys[i] = it.Text, it.Key
		}
		p.sel = min(p.sel, len(lines)-1)
		top := cy + int(tk*th/2) + int(pk*8)
		if v.Exploring || !shown { // no text box above: in the middle
			top = b.Dy()/2 - int(mk*float64(len(lines)+4)*4)
			if picBottom > 0 { // never over the picture: right under it
				top = max(top, picBottom+int(pk*4))
			}
		}
		p.drawMenu(dst, note.Glyph, askMenu(v).Prompt, lines, keys, nil, p.sel, mk, cx, top)
	}
	if note.Show { // the toast: pageImage's bottom line (noteLine) on a black band along the very bottom of the screen, no border
		h := int(pk * 8)
		dst.SubImage(image.Rect(b.Min.X, b.Max.Y-h, b.Max.X, b.Max.Y)).(*gpu.Image).Fill(color.Black)
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(pk, pk)
		op.GeoM.Translate(float64(cx)-pk*float64(pal.W)/2, float64(b.Max.Y-h))
		pw, _ := pageSize(v)
		m := image.NewRGBA(image.Rect(0, 0, pw, 8))
		noteLine(m, note, 0)
		dst.DrawImage(upload(&p.pg.toast, m), op)
	}
	if p.ctx > 0 { // the context menu: on top, what is under it dimmed
		b := dst.Bounds()
		vector.DrawFilledRect(dst, 0, 0, float32(b.Dx()), float32(b.Dy()), color.NRGBA{A: 0xA0}, false)
		p.drawCardTip(dst, v)
	}
	return Rect{}
}

// textPane is the text pane's text, to tell a pane apart from the one
// answered.
func textPane(v engine.View) string {
	if v.Pane == nil {
		return ""
	}
	var s strings.Builder
	for _, l := range v.Pane.Lines {
		for _, r := range l {
			fmt.Fprintf(&s, "%d:%s|", r.X, r.Text)
		}
		s.WriteByte('\n')
	}
	return s.String()
}

// Key turns window key k into actions (Actions hands them over). When the
// remastered view is up, Enter opens the vertical menu in place of the
// menu bar and Esc on the command bar the renderer's options.
func (p *Presenter) Key(k uint16, v engine.View) {
	if p.Playing() { // a key cuts the fight's moment short
		p.replay.skip = true
		return
	}
	if !p.key(k, v) {
		p.keyAct(k, v)
	}
}

// key takes k on the remastered screens; false leaves it to keyAct.
func (p *Presenter) key(k uint16, v engine.View) bool {
	if confirm(v) != "" {
		return p.confirmKey(k)
	}
	if p.main && (p.mainSkip() || p.mainKey(k, v)) {
		return true
	}
	if p.pickKey(k) {
		return true
	}
	if askList(v) != nil && askMenu(v) != nil && !choosing(v) {
		return p.listKey(k, v)
	}
	if p.pageMenu && p.pageMenuKey(k, v) {
		return true
	}
	if v.Inventory != nil && p.inv.view != nil && !choosing(v) {
		return p.inventoryKey(k)
	}
	if p.fight {
		return p.fightKey(k)
	}
	if p.ctx > 0 { // the card's context menu: the arrows, Enter, a hotkey or Esc, as the game's menus
		if _, ok := memberKey(k, 0, len(v.Party), false); ok { // another member: selected (keyAct), the menu closes
			p.ctx = 0
			return false
		}
		if globalKey(k) { // help, the minimap, save/load ...: keyAct; other keys stay here (they would act on the view under it)
			return false
		}
		switch i, back, _ := menuNav(k, &p.ctxSel, p.ctxKeys); {
		case back:
			p.ctx = 0
		case i >= 0:
			p.ctxSend(i)
		}
		return true
	}
	if !p.active || p.view == nil {
		p.menu = false
		return false
	}
	if p.view.OptionsOpen() {
		return true
	}
	if screenUp(v) {
		return p.campKey(k, v)
	}
	if v.Exploring {
		if !p.menu {
			switch k {
			case 13, KeyPadSelect:
				p.menu, p.sel = true, 0
			case 27:
				p.view.OpenOptions()
			default:
				return p.exploreKey(k, v)
			}
			return true
		}
		cmds := commands(v)
		keys := make([]byte, len(cmds))
		for i, c := range cmds {
			keys[i] = c.key
		}
		switch i, back, ok := menuNav(k, &p.sel, keys); {
		case back:
			p.menu = false
		case i >= 0:
			p.menu = false
			p.runCommand(i)
		case !ok:
			p.menu = false
			return p.exploreKey(k, v)
		}
		return true
	}
	m := askMenu(v)
	if !p.menu {
		switch {
		case choosing(v): // a member pick has no menu: every key goes to the portraits (keyAct)
			return false
		case k == 13 && noChoice(v): // "press enter to continue": nothing to pick
			return false
		case k == 13 && m != nil:
			p.menu, p.menuFor, p.sel = true, menuID(m), 0
		case k == 27 && v.Exploring:
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
	if n, ok := memberKey(k, selectedIndex(v), len(v.Party), true); ok { // the member the bar is for (camp, the temple)
		p.send(engine.ActLead, n)
		return true
	}
	keys := make([]byte, len(m.Items))
	for i, it := range m.Items {
		keys[i] = it.Key
	}
	switch i, back, ok := menuNav(k, &p.sel, keys); {
	case back && v.Exploring: // the command bar has no Exit item: the popup alone closes
		p.menu = false
	case back: // the game leaves the menu (its Exit item): hiding the popup alone left the game waiting in it
		p.send(engine.ActClose, 0)
	case i >= 0:
		p.menu = false
		p.send(engine.ActChoose, i)
	case !ok:
		return !globalKey(k) // help, the minimap, save/load ... go on (keyAct); other keys would act on the screen under the popup
	}
	return true
}

// Click takes a click at screen pixel (x, y) on the remastered screens: a
// portrait, an action bar button, or an item of the vertical menu. It
// reports whether it was taken.
func (p *Presenter) Click(x, y int) bool {
	if p.Playing() { // the fight plays on: the click only cuts its moment short
		p.replay.skip = true
		return true
	}
	if p.barClick(x, y) {
		return true
	}
	if confirm(p.shownView) != "" { // the yes/no question: its rows (or the battle bar's Yes, No); a click outside is No
		if p.fight && p.battle.Click(x, y) {
			return true
		}
		p.confirmPick(slices.IndexFunc(p.confirmAt, image.Pt(x, y).In))
		return true
	}
	if v := p.shownView; askList(v) != nil && askMenu(v) != nil && !choosing(v) && cardUnder(p.cards, x, y) < 0 { // a card is the member the list is for (below)
		p.listClick(x, y, v)
		return true
	}
	if v := p.shownView; p.pageMenu && pageScreen(v) {
		for i, r := range p.winRects {
			if image.Pt(x, y).In(r) {
				p.screenPick(v, i)
				return true
			}
		}
		if s := p.scr.sub; screenName(v) == "slots" || s == scrAsk || s == scrSlot || s == scrField || s == scrChange { // outside a popup: back (the creation sheet, the pickers and the icon editor take their page's clicks: Mouse)
			p.screenKey(27, v)
			return true
		}
	}
	if p.pageMenu {
		for i, r := range p.winRects {
			if image.Pt(x, y).In(r) && sheetUp(p.shownView) {
				p.sheetItems(p.shownView)[i].do()
				return true
			}
			if image.Pt(x, y).In(r) {
				p.send(engine.ActChoose, i)
				return true
			}
		}
		if v := p.shownView; !pageScreen(v) && !sheetUp(v) && v.FullMap == nil && v.Pages == nil { // outside a page's question: back (the sheet's portrait, the full map and the pagers take their page's clicks: Mouse)
			p.send(engine.ActClose, 0)
			return true
		}
	}
	if p.main {
		if p.mainSkip() {
			return true
		}
		return p.mainClick(x, y)
	}
	if p.pickClick(x, y) {
		return true
	}
	if !p.fight {
		if p.ctx > 0 { // the context menu: a line acts for its member; a click elsewhere closes it (on another card: that one's)
			for j, r := range p.ctxRects {
				if image.Pt(x, y).In(r) && p.active {
					p.ctxSend(j)
					return true
				}
			}
			p.ctx = 0 // a click outside: back
			return true
		}
		for i, r := range p.cards { // a portrait: its context menu while exploring, else its inventory (a right click: the sheet, Mouse)
			if image.Pt(x, y).In(r) && p.active {
				p.cardClick(i)
				if p.ordering { // while the party is ordered the card is picked up instead (Hover drops it)
					p.dragFrom, p.dragAt = 1+i, image.Pt(x, y)
				}
				return true
			}
		}
		if v := p.shownView; screenUp(v) && p.active && !p.view.OptionsOpen() {
			for i, r := range p.menuRects {
				if image.Pt(x, y).In(r) {
					p.campPick(v, p.camp.top+i)
					return true
				}
			}
			p.campBack(v) // a click outside: back
			return true
		}
		if v := p.shownView; v.Exploring && !p.menu && p.active && p.view != nil && !p.view.OptionsOpen() { // the view: walk, turn, the command menu
			p.exploreClick(x, y, v)
			return true
		}
		if !p.menu || !p.active || p.view == nil || p.view.OptionsOpen() {
			return false
		}
		for i, r := range p.menuRects {
			if image.Pt(x, y).In(r) {
				p.menu = false
				if p.command {
					p.runCommand(i)
					return true
				}
				p.send(engine.ActChoose, i)
				return true
			}
		}
		p.key(27, p.shownView) // a click outside: back, as Esc
		return true
	}
	return p.fightClick(x, y)
}

// Hover moves the vertical menu's highlight to the item under screen pixel (x, y).
// down: the left button is held; a card pressed while the party is being
// ordered (Click took it) follows the pointer and goes where it is let go.
func (p *Presenter) Hover(x, y int, down bool) {
	if p.barHover(y, down) {
		return
	}
	if confirm(p.shownView) != "" { // the yes/no question takes a moved pointer (in combat the bar's hover goes on below)
		if pt := image.Pt(x, y); pt != p.pointer {
			p.pointer = pt
			if i := slices.IndexFunc(p.confirmAt, pt.In); i >= 0 {
				p.confirmSel = i
			}
		}
		if !p.fight {
			return
		}
	}
	if v := p.shownView; askList(v) != nil && askMenu(v) != nil && !choosing(v) {
		p.listHover(x, y, v)
	}
	if p.main {
		p.mainHover(x, y)
		return
	}
	if p.pickList() != nil {
		if image.Pt(x, y) != p.pointer { // only a moved pointer takes the highlight
			p.pointer = image.Pt(x, y)
			if i := slices.IndexFunc(p.menuRects, p.pointer.In); i >= 0 {
				p.pickSel = i
			}
		}
		return
	}
	if p.pageMenu && image.Pt(x, y) != p.pointer { // the page's menus (keyWindow): the sheet's, a question's, the page screens'
		p.pointer = image.Pt(x, y)
		if i := slices.IndexFunc(p.winRects, p.pointer.In); i >= 0 {
			switch {
			case !pageScreen(p.shownView):
				p.sel = i
			case p.scr.sub != scrType:
				p.scr.sel = p.scr.top + i
			}
		}
	}
	if p.fight {
		p.pointer = image.Pt(x, y)
		if !p.fightLog {
			p.battle.Point(x, y)
		}
		return
	}
	p.cardHover = 1 + cardUnder(p.cards, x, y)
	if p.dragFrom > 0 {
		p.dragAt = image.Pt(x, y)
		if !down {
			if to := p.cardHover; to > 0 && to != p.dragFrom {
				p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActOrder, N: p.dragFrom - 1, X: to - 1})
			}
			p.dragFrom = 0
		}
	}
	if screenUp(p.shownView) && p.ctx == 0 && image.Pt(x, y) != p.pointer { // camp-style screens highlight under the pointer too
		p.pointer = image.Pt(x, y)
		if i := slices.IndexFunc(p.menuRects, p.pointer.In); i >= 0 {
			p.camp.sel = p.camp.top + i
		}
		return
	}
	if !(p.menu || p.ctx > 0) || image.Pt(x, y) == p.pointer {
		return
	}
	p.pointer = image.Pt(x, y)
	if p.ctx > 0 { // a moved pointer takes the context menu's highlight (keys keep it while it rests)
		if i := slices.IndexFunc(p.ctxRects, p.pointer.In); i >= 0 { // off the lines the highlight stays
			p.ctxSel = i
		}
		return
	}
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
	return askMenu(v) != nil && !v.Exploring && len(askMenu(v).Items) == 1 && !choosing(v) // a member pick's lone Select is still a choice: whom
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
// of the party down the left, the rest down the right, at cardScale.
func cardAt(dst *gpu.Image) func(i, n int) image.Rectangle {
	b := dst.Bounds()
	k := cardScale(dst)
	px := func(n int) int { return int(math.Round(float64(n) * k)) }
	return func(i, n int) image.Rectangle {
		if blobber.Settings.PartyRow { // one row along the bottom, centred
			step := px(cardW + cardGap)
			x := (b.Dx()-n*step+px(cardGap))/2 + i*step
			y := b.Dy() - px(cardH+cardGap) + px(cardFoot)
			return image.Rect(x, y, x+px(cardW), y+px(cardH))
		}
		left := (n + 1) / 2
		col, row, per := 0, i, left
		if i >= left {
			col, row, per = 1, i-left, n-left
		}
		y := b.Dy() - px((per-row)*(cardH+cardGap)) + px(cardFoot) // the bars end a gap above the bottom
		x := px(cardGap)
		if col == 1 {
			x = b.Dx() - px(cardGap) - px(cardW)
		}
		return image.Rect(x, y, x+px(cardW), y+px(cardH))
	}
}

// mapRect is where the map goes: a square in the top right corner, a card
// gap from the edges, mapPx on a side.
func mapRect(dst *gpu.Image, _ int) image.Rectangle {
	b := dst.Bounds()
	gap, side := cardGap*cardBase(dst), mapPx(dst)
	return image.Rect(b.Dx()-gap-side, gap, b.Dx()-gap, gap+side)
}

const cardGap, cardRows = 8, 4

// cardBase is the cards' size 1 (Esc, Graphics, Portraits): the largest
// whole scale where a column of four fits under the smallest map.
func cardBase(dst *gpu.Image) int {
	return max(1, min(dst.Bounds().Dx()/480, dst.Bounds().Dy()/360, (dst.Bounds().Dy()-mapSide-2-cardGap)/(cardRows*(cardH+cardGap))))
}

// cardScale is the cards' scale: cardBase times the Portraits size, as far
// as a column of four (eight members with NPCs) still fits under the map at
// its size and a card takes at most a sixth of the width (fitScale).
func cardScale(dst *gpu.Image) float64 {
	b := dst.Bounds()
	room := float64(b.Dy() - 2*cardGap*cardBase(dst))
	if !blobber.Settings.MapOff {
		room -= float64(mapPx(dst))
	}
	if blobber.Settings.PartyRow { // eight across the bottom, under the map, at most a third of the height
		return fitScale(float64(cardBase(dst)), blobber.Settings.Cards, func(k float64) bool {
			return k*8*(cardW+cardGap)+k*cardGap <= float64(b.Dx()) && k*(cardH+cardGap) <= min(room, float64(b.Dy())/3)
		})
	}
	return fitScale(float64(cardBase(dst)), blobber.Settings.Cards, func(k float64) bool {
		return k*cardRows*(cardH+cardGap) <= room && k*cardW <= float64(b.Dx())/6
	})
}

// mapPx is the map's side: one map pixel to a card pixel at size 1 (Esc,
// Graphics, Minimap size), at most 60 percent of the height and a third of
// the width; the cards make way for it.
func mapPx(dst *gpu.Image) int {
	b := dst.Bounds()
	return int(mapSide * fitScale(float64(cardBase(dst)), blobber.Settings.Map, func(k float64) bool {
		return k*mapSide <= float64(b.Dy())*0.6 && k*mapSide <= float64(b.Dx())/3
	}))
}

// fitScale is unit times size s (one of blobber.UISteps) when ok takes it,
// else the largest whole scale under it that ok takes (so a size too big
// for the screen still grows as far as it fits, its pixels square), else
// the largest smaller step, else the smallest.
func fitScale(unit, s float64, ok func(k float64) bool) float64 {
	if s <= 0 {
		s = 1
	}
	if k := unit * s; ok(k) {
		return k
	}
	for k := math.Ceil(unit*s) - 1; k >= 1; k-- {
		if ok(k) {
			return k
		}
	}
	for i := len(blobber.UISteps) - 1; i >= 0; i-- {
		if t := blobber.UISteps[i]; t < s && ok(unit*t) {
			return unit * t
		}
	}
	return unit * blobber.UISteps[0]
}

// mapSide is View.Map's side in pixels.
const mapSide = 16 * engine.MapCell

// drawMap shows the map as the full map draws it (View.Map) in mapRect.
func (p *Presenter) drawMap(dst *gpu.Image, v engine.View, font func(byte, bool) [8]byte) {
	if v.Map == nil || blobber.Settings.MapOff {
		return
	}
	if p.mapImg == nil {
		p.mapImg = gpu.NewImage(mapSide, mapSide)
	}
	if v.Map != p.mapSrc { // a new View: upload it once
		p.mapImg.WritePixels(mapImage(v.Map, engine.MapCell, v.Ink, font).Pix)
		p.mapSrc = v.Map
	}
	r := mapRect(dst, len(v.Party))
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

// scene is the remastered map of the area v shows, or nil for an area
// without one (logged once: there is no other view of it).
func (p *Presenter) scene(v engine.View) *blobber.Scene {
	name, ok := areaMaps[v.Area]
	if !ok {
		name = fmt.Sprintf("area%d", v.Area)
	}
	return p.sceneNamed(name)
}

// openAt (POR_OPEN=map:NAME:X:Y:D, D 0..3 for N E S W) shows the remastered
// map NAME from square X, Y alone, whatever the engine shows: the
// comparison sheets (phlan-poc/compare/compare.py) shoot it with POR_SHOT.
var openAt = os.Getenv("POR_OPEN")

// drawOpen draws the view openAt names.
func (p *Presenter) drawOpen(dst *gpu.Image) {
	f := strings.Split(openAt, ":")
	if len(f) != 5 || f[0] != "map" {
		log.Fatalf("POR_OPEN=%q: want map:NAME:X:Y:D", openAt)
	}
	x, _ := strconv.Atoi(f[2])
	y, _ := strconv.Atoi(f[3])
	d, _ := strconv.Atoi(f[4])
	sc := p.sceneNamed(f[1])
	if sc == nil {
		log.Fatalf("POR_OPEN=%q: no map %s", openAt, f[1])
	}
	if p.shown != f[1] {
		p.view.Show(sc, blobber.Camera{X: x, Y: y, Dir: d})
		p.shown = f[1]
		blobber.SetFlats(sc.Floor, sc.Ceiling)
	}
	p.animate()
	p.view.Draw(dst)
}

// sceneNamed is the remastered map NAME, or nil (logged once).
func (p *Presenter) sceneNamed(name string) *blobber.Scene {
	if p.view == nil {
		var err error
		if p.view, err = blobber.NewView(); err != nil {
			log.Fatalln("remaster:", err)
		}
		blobber.Window = func(d *gpu.Image, t string, l []string, k []byte, sel int, b bool) { // its menu and options in the game's box
			p.keyWindow(d, t, l, k, nil, sel, b)
		}
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
	p.scenes[name] = sc // nil too: a missing or broken map is logged once
	return sc
}

// Scripted stops animate's clock: a capture script ticks the renderer itself (Tick), so
// every frame of a scripted run is the same run to run.
var Scripted bool

// Tick advances the renderer's animation one tick (1/60 s) under a script.
func (p *Presenter) Tick() {
	if p.view != nil {
		p.view.Update()
	}
}

// animate runs the renderer's 60 ticks a second, whatever the frame rate.
func (p *Presenter) animate() {
	if Scripted {
		return
	}
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
// on cx, cy: its inside, in a plain window of tiles' cells.
func drawBox(dst, inside *gpu.Image, tiles [3]*gpu.Image, r engine.CellRect, k float64, cx, cy int) {
	px := r.Pixels()
	var g gpu.GeoM
	g.Scale(k, k)
	g.Translate(float64(cx)-k*float64(px.Dx())/2, float64(cy)-k*float64(px.Dy())/2)
	in := window(dst, tiles, g, r.X2-r.X1+1, r.Y2-r.Y1+1, nil)
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(in.Min.X), float64(in.Min.Y))
	dst.DrawImage(inside, op)
}

// window is every window's frame: a cw by ch cell box at g on dst, bg
// inside (nil: the inside stays as drawn), then the ribbon border in tiles
// t (the game's corner, side and top cells: borders), as DrawBorder does.
// It returns the inside, in dst's pixels.
func window(dst *gpu.Image, t [3]*gpu.Image, g gpu.GeoM, cw, ch int, bg color.Color) image.Rectangle {
	x0, y0 := g.Apply(8, 8)
	x1, y1 := g.Apply(float64(cw*8-8), float64(ch*8-8))
	if bg != nil {
		vector.DrawFilledRect(dst, float32(x0), float32(y0), float32(x1-x0), float32(y1-y0), bg, false)
	}
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
	return image.Rect(int(x0), int(y0), int(x1), int(y1))
}

// resize tells the engine the text cells the window holds (grid) when
// they change, and again now and then while the page is not yet of that
// size (the action was dropped): the engine lays its screens out to them.
func (p *Presenter) resize(dst *gpu.Image, v engine.View) {
	cols, rows := grid(dst)
	pw, ph := pageSize(v)
	p.gridAge++
	if g := [2]int{cols, rows}; g != p.grid || p.gridAge > 30 && (pw != cols*8 || ph != rows*8) {
		p.grid, p.gridAge = g, 0
		p.inv.acts = append(p.inv.acts, engine.Action{Kind: engine.ActResize, X: cols, Y: rows})
	}
}

// Cell is the original's text cell under screen pixel (x, y) on a
// remastered screen, which shows none of its rows: "press a key" takes a
// click anywhere, as on the original's screen.
func (p *Presenter) Cell(x, y int) (int, int, bool) { return 0, 0, p.noMenu }

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

// portrait is m's card art: the PNG the player dropped on the sheet, fitted
// into the PortraitSize square, on its bottom edge (cached), else the paper-doll head's face.
func (p *Presenter) portrait(m engine.Member) image.Image {
	if m.Portrait == nil {
		return p.face(m.Head)
	}
	if f, ok := p.faces[m.Portrait]; ok {
		return f
	}
	f := fitSquare(m.Portrait, portraitSize)
	if p.faces == nil {
		p.faces = map[image.Image]image.Image{}
	}
	p.faces[m.Portrait] = f
	return f
}

// hiFace is a PNG portrait at n pixels square for a card at the window's
// resolution (cached per size): more detail than the card's own face.
func (p *Presenter) hiFace(src image.Image, n int) *gpu.Image {
	if g, ok := p.hiFaces[src]; ok && g.Bounds().Dx() == n {
		return g
	}
	g := gpu.NewImageFromImage(fitSquare(src, n))
	if old, ok := p.hiFaces[src]; ok {
		old.Deallocate()
	}
	p.hiFaces[src] = g
	return g
}

// fitSquare is src fitted into an n pixel square, its shape kept, on the
// square's foot: by whole pixels when those nearly fill it (crisp pixel
// art), else by Catmull-Rom.
func fitSquare(src image.Image, n int) *image.RGBA {
	b := src.Bounds()
	f := image.NewRGBA(image.Rect(0, 0, n, n))
	w, h := n, b.Dy()*n/max(b.Dx(), 1)
	if h > n {
		w, h = b.Dx()*n/max(b.Dy(), 1), n
	}
	var sc xdraw.Scaler = xdraw.CatmullRom
	if s := wholeScale(b.Dx(), b.Dy(), n, n); s > 0 {
		w, h, sc = s*b.Dx(), s*b.Dy(), xdraw.NearestNeighbor
	}
	sc.Scale(f, image.Rect((n-w)/2, n-h, (n-w)/2+w, n), src, b, draw.Src, nil)
	return f
}

// wholeScale is the whole number of pixels per source pixel that scales a
// sw by sh picture up to nearly fill tw by th (at least 90%), 0 when none
// does (a smaller picture is drawn smoothly to the exact size instead).
func wholeScale(sw, sh, tw, th int) int {
	s := min(tw/max(sw, 1), th/max(sh, 1))
	if s >= 1 && s*sw >= tw*9/10 && s*sh >= th*9/10 {
		return s
	}
	return 0
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

// ctxSend acts line j of the context menu for its member and closes it; a
// spell pick opens where the menu was.
func (p *Presenter) ctxSend(j int) {
	if p.ctxActs[j] == engine.ActSpells {
		p.pickAt = p.ctxBox
	}
	p.send(p.ctxActs[j], p.ctx-1)
	p.ctx = 0
}

// cardClick is a left click on member i's card: while exploring (the
// command bar) it selects the member and opens its context menu, elsewhere
// it is the member's inventory; while the party is ordered it only selects.
func (p *Presenter) cardClick(i int) {
	switch {
	case p.ordering:
		p.send(engine.ActLead, i)
	case p.command:
		p.send(engine.ActLead, i)
		p.ctx, p.ctxSel = 1+i, 0 // the first line highlighted: Enter acts at once
	default:
		p.send(engine.ActInventory, i)
	}
}

// pace ends a script's pause (AskPause) once its time is up: the engine
// never sleeps, the presenter keeps the time.
func (p *Presenter) pace(v engine.View) {
	if v.Ask == nil || v.Ask.Kind != engine.AskPause {
		p.pauseFrom = time.Time{}
		return
	}
	switch {
	case p.pauseFrom.IsZero():
		p.pauseFrom = time.Now()
	case time.Since(p.pauseFrom) >= time.Duration(v.Ask.Ms)*time.Millisecond:
		p.send(engine.ActOK, 0)
		p.pauseFrom = time.Time{}
	}
}
