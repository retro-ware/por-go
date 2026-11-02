// Package remaster shows the game with the Blobber renderer: the 3D view
// fills the window, and the original's windows are laid over it. It only
// reads the game, through engine.View, so drawing never touches the
// game's state.
package remaster

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"strings"
	"time"

	"gpu"
	"gpu/vector"
	"por/present/input"

	"por/phlan-poc/blobber"

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

	view                 *blobber.View
	scenes               map[string]*blobber.Scene   // parsed maps by name
	shown                string                      // the scene the view shows
	last                 time.Time                   // when the animation last advanced
	ticks                float64                     // animation time not yet run, in 1/60 s ticks
	border               borders                     // the game's window border
	bars                 *gpu.Image                  // bars.png
	idSeen               int                         // the shop's last Id reveal shown and closed (identify.go)
	idImg                *gpu.Image                  // its pop-up
	expShown             bool                        // drawParty: every card's experience bar, Ctrl or not (the training hall)
	cardImgs             []*gpu.Image                // the cards, drawn at scale 1...
	cardKeys             []cardKey                   // ...and what each shows
	faceImgs             map[image.Image]*gpu.Image  // the faces on the GPU
	hiFaces              map[image.Image]*gpu.Image  // the PNG portraits at the cards' window resolution
	labels               map[string]*gpu.Image       // card labels in the game's font
	glyph                func(byte) [8]byte          // the game's font
	main                 bool                        // the main menu is up (mainmenu.go)
	ask                  bool                        // ... with a question over it
	logo                 srcImage                    // View.Logo on the GPU
	backdrop             srcImage                    // View.Backdrop on the GPU
	mainAt               time.Time                   // when the main menu first showed the backdrop
	layer                *gpu.Image                  // the main menu drawn for its fade in
	grid                 [2]int                      // the text cells the window holds, as last told to the engine (ActResize)
	gridAge              int                         // frames since
	cards                []image.Rectangle           // the party's cards on screen in the 3D view, as last drawn (none elsewhere)
	faces                map[image.Image]image.Image // card art cropped from each paper-doll head
	inside               bool                        // the view has the inside ceiling
	active               bool                        // the last frame was drawn remastered
	menu                 bool                        // the vertical menu is open
	menuFor              string                      // the menu it opened for (menuID): another menu arriving closes it, so one key never answers two menus
	sel                  int                         // its highlighted item
	asked                string                      // the question menu last opened by itself (menuID); send clears it on a pick, so the same menu back after one (a refusal, back from Browse) opens again
	answered             string                      // the text pane under the last question: once it is answered, that text is done
	mapImg               *gpu.Image                  // View.Map on the GPU
	mapAt                image.Rectangle             // where drawMap last put the minimap (a click there opens the full map)
	mapSrc               *engine.MapSquares          // the View.Map it holds
	menuRects            []image.Rectangle           // the vertical menu's items on screen, as last drawn
	scrollBars           []barHit                    // the scroll bars drawn this frame (scrollbar.go)
	barDrag              *barHit                     // the bar whose thumb the pointer holds
	logTop               int                         // the battle Log's first line shown
	sheet                sheetCanvas                 // the character sheet
	beastPage            sheetCanvas                 // camp's bestiary (drawBestiary)
	journal              journalUI                   // the quest journal (journal.go)
	resultsPage          sheetCanvas                 // the after-combat page (drawResults)
	areaSrc, areaMarkSrc image.Image                 // the overland's map and marker last uploaded (drawAreaMap)
	areaImg, areaMark    *gpu.Image
	winRects             []image.Rectangle        // the Esc menu's lines on screen, as last drawn
	mainSel              int                      // the main menu's highlight
	mainBack             int                      // the main menu's row that opened the Party submenu: Exit goes back to it
	mainSub              bool                     // the main menu shows the Party submenu (the character functions)
	mainAsk              int                      // the main menu's question up (askQuit ..)
	noMenu               bool                     // nothing to pick: any click answers
	pointer              image.Point              // the mouse, as Hover last saw it (keys move the highlight while it rests)
	menuImg              *gpu.Image               // the vertical menu, drawn at scale 1
	menuBuf              []byte                   // its pixels
	footImg              *gpu.Image               // the menu's footer on its bottom border
	footBuf              []byte                   // its pixels
	battle               battle.Presenter         // the combat map in 3D
	mapBottom            int                      // screen rows above this are the 3D map
	fight                bool                     // the last frame was the 3D combat map
	inv                  inventory                // the inventory screen (inventory.go)
	fightView            *engine.Battle           // the combat map last drawn
	replay               replay                   // the fight's events still to play
	logGrey              bool                     // drawMenu's plain text light grey, not green (the battle Log)
	fightLog             bool                     // the battle Log is up (fight.go)
	fightCard            image.Rectangle          // the active member's card (fight.go): left click the inventory, right click the sheet
	rowsImg              *gpu.Image               // the combat rows, drawn at scale 1
	rowsKey              string                   // what they show
	typing               bool                     // a line is being typed (input.go) ...
	typed                string                   // ... and reads this ...
	typedCur             int                      // ... its cursor here
	cardHover            int                      // 1 + the card the pointer is on (0 none): its name and class show beside it
	dragFrom             int                      // 1 + the card pressed while the party is being ordered (View.Ordering), it follows the pointer to the card it is dropped on (0 none)
	dragAt               image.Point              // where the pointer is while a card is dragged
	Sound                func(engine.CombatEvent) // plays a fight's EvSound when its turn comes (Engine.PlayEventSound)
	dragStart            image.Point              // where it was pressed
	dragged              bool                     // it has moved far enough from dragStart to be a drag
	ordering             bool                     // View.Ordering
	ctx                  int                      // 1 + the card whose context menu a right click opened (0 none): drawCardTip
	ctxSel               int                      // its highlighted line
	ctxActs              []engine.ActKind         // its lines' actions, for the member
	ctxKeys              []byte                   // its lines' hotkeys (the first letters)
	campCtx              [][]image.Rectangle      // in camp every member's context menu lines (drawCampMenus)
	campImg              []*gpu.Image             // drawCampMenus: each member's menu image
	campBuf              [][]byte                 // and its pixels
	menuGrey             bool                     // drawMenu's border grey: a menu without the focus
	menuTight            bool                     // drawMenu without the blank column inside the border
	ctxRects             []image.Rectangle        // its lines on screen, as last drawn
	ctxBox               image.Rectangle          // where its box went: the spell pick opens there
	pick                 []engine.Spell           // View.Ask (AskSpell) as last drawn: a spell to cast is being picked on this screen
	picBottom            int                      // the bottom of the picture box drawn this frame (0: none): menus go under it
	pickAt               image.Rectangle          // the context menu's box the pick list replaces (zero: the top middle, the bar's scale)
	pickSel              int                      // the pick list's highlight ...
	picking              bool                     // ... reset when it opens
	command              bool                     // the party explores (View.Exploring), as last drawn
	cmds                 []command                // the command menu's commands (explore.go), as last drawn
	camp                 campUI                   // the camp screens (camp.go)
	shownView            engine.View              // the View last drawn
	list                 listUI                   // View.Ask (AskList)'s highlight (list.go)
	pageMenu             bool                     // a page's question is up (list.go)
	mapPointer           image.Point              // the full map: the pointer's last cell (a move there moves the cursor)
	mapMenu              bool                     // the full map's cursor cell has its context menu open (drawMapMenu)
	tipImg               *gpu.Image               // the card tip's box, drawn at scale 1 (drawMenu's image, swapped in)
	tipBuf               []byte                   // its pixels
	spellTipImg          *gpu.Image               // the camp spell lists' rules tooltip (drawSpellTip), its own image
	spellTipBuf          []byte                   // its pixels
	scr                  screenUI                 // the page screens' questions (screens.go)
	screen               image.Rectangle          // the window, as last drawn
	confirmFor           string                   // the yes/no question (View.Ask, AskConfirm) up, as last drawn ...
	confirmSel           int                      // ... its highlight (Yes 0, No 1) ...
	confirmAt            []image.Rectangle        // ... and its rows on screen
	noteBtn              image.Rectangle          // the text box's "> Notes" on the window (empty: no text box)
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
	p.noteBtn = image.Rectangle{}
	p.battle.Asking = confirm(v) != ""
	if !pageScreen(v) { // none up: the next one opens fresh (F10 on the last save)
		p.scr.up = ""
	}
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
	if it := p.identified(v); it != nil { // the shop's Id: what it revealed, over everything
		p.drawIdentified(dst, note.Glyph, it)
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
		if p.picBottom > 0 { // a picture up: under it, not over it, as far as the list fits
			h := int(k) * 8 * (len(p.pick) + 4) // drawMenu's box: the border, the title and a blank line
			top = max(top, min(p.picBottom+int(k)*4, dst.Bounds().Dy()-h))
		}
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

// draw shows the screen the View is: the page, the main menu, a tall page or the 3D view.
func (p *Presenter) draw(dst *gpu.Image, note Note, v engine.View) Rect {
	p.screen = dst.Bounds()
	p.picBottom = 0
	p.setup(note, v)
	p.command = v.Exploring
	if screenUp(v) && !screenUp(p.shownView) { // camp, the temple, the shop, training start on their main screen
		p.camp.screen, p.camp.sel, p.camp.rows = campMain, 0, nil
		if t := v.Training; t != nil && len(t.Learn) == 0 && t.Why == "" { // the training question starts on No
			p.camp.sel = 1
		}
	}
	if askList(v) != nil && askList(p.shownView) == nil { // a list opens at its first line
		p.list = listUI{sel: max(0, slices.IndexFunc(askList(v), func(l engine.ListLine) bool { return !l.Off }))}
	}
	p.shownView = v
	p.pace(v)
	p.pageMenu = false
	p.mapMenu = p.mapMenu && v.FullMap != nil
	p.cmds = commands(v)
	if !p.active {
		p.ctx = 0
	}
	p.fight = v.Battle != nil                                  // the battle view is the only one (the engine has no combat page); under a modal (help, F10 ...) the engine leaves Battle out
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
	if p.main = v.MainMenu != nil; p.main && v.Hall { // the training hall: its menu over the 3D view
		p.active, p.menu = false, false
		if sc := p.scene(v); sc != nil {
			p.draw3D(dst, v, sc, note)
		} else {
			dst.Fill(color.Black)
		}
		p.drawMainUI(dst, v)
		return Rect{}
	}
	if p.main {
		p.shown, p.active, p.menu = "", false, false
		p.drawMain(dst, v)
		return Rect{}
	}
	p.journal.open = p.journal.open && v.Exploring
	if p.journal.open { // the quest journal: a page of its own over the whole window
		p.shown, p.active, p.menu = "", true, false
		dst.Fill(color.Black)
		p.drawJournal(dst, note, v)
		if j := &p.journal; j.ask {
			p.keyWindow(dst, "Remove this note?", []string{"Yes", "No"}, yesNoKeys, nil, j.askSel, false)
			j.askAt = slices.Clone(p.winRects)
		}
		return Rect{}
	}
	if screenUp(v) && v.Camp != nil && p.camp.screen == campBestiary { // the bestiary: a page of its own over the whole window
		p.shown, p.active, p.menu = "", true, false
		dst.Fill(color.Black)
		p.drawBestiary(dst, note, v)
		return Rect{}
	}
	sc := p.scene(v)
	area := v.Window == engine.WinAreaMap && v.AreaMap != nil                   // the overland: its map over the whole window, the 3D view's cards and boxes over it
	if sc == nil && !area || v.Results != nil || !choosing(v) && !v.Adventure { // a member pick (Choosing) shows the view and the cards whatever page asked
		if screenUp(v) { // training from the hall's party menu: over the 3D view
			p.active, p.cards = true, nil // its lines take clicks (Click), no card under them
			if sc != nil {
				p.draw3D(dst, v, sc, note)
			} else {
				dst.Fill(color.Black)
			}
			b := dst.Bounds()
			p.drawCamp(dst, note.Glyph, v, scale(dst), b.Dx()/2, b.Dy()/4, b.Max.Y)
			return Rect{}
		}
		p.shown, p.active, p.menu, p.cards = "", false, false, nil        // back from elsewhere: no tween from where the view was
		p.asked = ""                                                      // and a question still waiting on return (View from the loot menu) pops up again
		if sheetUp(v) && v.SheetPage != nil && v.SheetPage.Spells == "" { // the sheet: the presenter's, at the inventory's scale
			dst.Fill(color.Black)
			p.drawSheet(dst, note, v)
			return Rect{}
		}
		if v.Results != nil {
			return p.drawResults(dst, v, note)
		}
		if !ownLayout(v) { // nothing of the engine's page: the page's pictures and text in the remaster's layout
			return p.drawGeneric(dst, v, note)
		}
		r := p.drawPage(dst, v, note)
		p.scr.at = r
		p.scr.pw, p.scr.ph = pageSize(v)
		p.drawPageMenu(dst, note, v)
		return r
	}
	p.active = true
	if area {
		p.shown = ""
		p.drawAreaMap(dst, v.AreaMap)
	} else {
		p.draw3D(dst, v, sc, note)
	}
	if v.Sweeping { // the reminder: under the minimap, else in its corner
		k := cardScale(dst)
		const s = "Sweeping"
		w, gap := int(k*float64(8*len(s)+2)), int(k*cardGap)
		y := gap
		if p.mapAt.Max.Y > 0 {
			y = p.mapAt.Max.Y + gap/2
		}
		p.label(dst, s, dst.Bounds().Dx()-gap-w, y, k)
	}
	p.ordering = v.Ordering
	if !p.ordering {
		p.dragFrom = 0
	}
	at := cardAt(dst)
	if p.dragFrom > 0 && p.dragged { // the dragged card hangs under the pointer
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
	if p.ctx == 0 && !p.campHome(v) { // the tooltip (camp shows the members' menus); a context menu goes over everything (below)
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
	tk := fitScale(k, 1, func(k float64) bool { return k*float64(txt.Pixels().Dx()) <= bw && k*th <= bh/2 })
	mk := fitScale(k, 1, func(k float64) bool { return k*28*8 <= bw && k*12*8 <= bh/2 })
	cx, cy := (b.Min.X+b.Max.X)/2, b.Dy()/2
	if v.Window == engine.WinText { // text all over the view window, not just the picture frame
		pic = engine.ViewWindow.Framed()
	}
	picBottom := 0                                                                                       // the bottom of the picture box, when one is up
	if v.Adventure && (v.Window == engine.WinPicture || v.Window == engine.WinText) && !p.spellList(v) { // the spell lists' rules go there; not the page a member pick came from
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
		inside := viewTextImage(&p.pg.pic, v.ViewText, pic.Pixels().Inset(8), note) // text over the whole view window (WinText)
		if v.Window == engine.WinPicture {
			inside = pictureBox(&p.pg.pic, v.Pictures, pic.Pixels().Inset(8))
		}
		drawBox(dst, inside, p.border.tiles, pic, pp, cx, py)
		picBottom = py + int(pp*ph/2)
		p.picBottom = picBottom
	}
	pane, asking := textPane(v), askMenu(v) != nil && !v.Exploring && !noChoice(v)
	if asking {
		p.answered = pane
	}
	shown := v.Adventure && v.Pane != nil && v.Pane.Shown() && (asking || pane != p.answered) // back from a question with its text unchanged: the encounter is over
	ask := typing(v) != nil && typing(v).Prompt != ""                                         // a script's input: its question and the line in one pop-up, no text box
	shown = shown && !ask && !(askList(v) != nil && askMenu(v).Prompt != "")                  // a vertical menu's prompt is the list's own title
	shown = shown && !(v.Shop != nil && p.camp.screen != campMain)                            // the shop's submenus (identify, appraise) stand alone, the greeting gone
	if shown && v.Exploring && v.Window != engine.WinPicture && v.Window != engine.WinText {  // the text under the command bar: along the bottom edge
		p.paneBox(dst, note, v, tk, cx, b.Dy()-int(tk*th/2))
	} else if shown { // a question, or a message to confirm (Enter, a click): a box in the middle, its choices under it
		p.paneBox(dst, note, v, tk, cx, cy)
		picBottom = max(picBottom, cy+int(tk*th/2)) // the menus go under the text too
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
		top := b.Dy()/2 - int(mk*float64(len(engine.WrapWords(typing(v).Prompt, 24))+5)*4)
		if picBottom > 0 { // never over the picture: right under it
			top = max(top, picBottom+int(pk*4))
		}
		p.drawMenu(dst, note.Glyph, typing(v).Prompt, []string{caretLine(t, c, w, p.blink())}, nil, nil, 0, mk, cx, top)
		p.menuRects = p.menuRects[:0]
	} else if askList(v) != nil && askMenu(v) != nil { // a list and its question (list.go)
		p.menu = false
		rows := len(titleLines(askMenu(v).Prompt)) + min(len(askList(v)), listRows) + len(askMenu(v).Items) + 4 // drawMenu's: the title, a blank, the lines, a blank, the options, the border
		top := max(b.Dy()/2-int(mk*float64(rows)*4), picBottom+int(pk*4))
		p.drawList(dst, note.Glyph, v, mk, cx, max(0, min(top, b.Max.Y-int(mk*float64(rows)*8)))) // all of it on screen, over the picture's edge if it must
	} else if screenUp(v) && spellPick(v) == nil { // camp (camp.go), the temple, the shop, training
		p.menu = false
		p.drawCamp(dst, note.Glyph, v, mk, cx, max(b.Dy()/4, picBottom+int(pk*4)), b.Max.Y)
		if p.campHome(v) {
			p.drawCampMenus(dst, v)
		}
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
		h := int(k * 8) // at K, as every glyph
		dst.SubImage(image.Rect(b.Min.X, b.Max.Y-h, b.Max.X, b.Max.Y)).(*gpu.Image).Fill(color.Black)
		pw, _ := pageSize(v)
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(float64(cx)-k*float64(pw)/2, float64(b.Max.Y-h))
		m := image.NewRGBA(image.Rect(0, 0, pw, 8))
		noteLine(m, note, 0)
		dst.DrawImage(upload(&p.pg.toast, m), op)
	}
	if p.ctx > 0 && !p.campHome(v) { // the context menu: on top, nothing dimmed
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

// optionsKeys: the pad's acts as the options page reads them.
var optionsKeys = map[input.Act]gpu.Key{input.Up: gpu.KeyUp, input.Down: gpu.KeyDown, input.Left: gpu.KeyLeft, input.Right: gpu.KeyRight,
	input.PgUp: gpu.KeyPageUp, input.PgDn: gpu.KeyPageDown, input.Home: gpu.KeyHome, input.End: gpu.KeyEnd,
	input.PadA: gpu.KeyEnter, input.Enter: gpu.KeyEnter, input.Esc: gpu.KeyEscape}

// OptionsPad hands a gamepad key (as PadKeys sends it) to the options page,
// for the next UpdateOptions.
func (p *Presenter) OptionsPad(k uint16) {
	if gk, ok := optionsKeys[actOf(k)]; ok && p.view != nil {
		p.view.PadKey(gk)
	}
}

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

// scale is K, the UI's one glyph cell size over 8 (CLAUDE.md): the largest
// whole factor the inventory's invW x invH canvas fits dst at (at least
// 1). Every glyph and ribbon tile is 8*K window pixels; a larger window
// gives more cells, not bigger ones.
func scale(dst *gpu.Image) float64 {
	b := dst.Bounds()
	return float64(max(1, min(b.Dx()/invW, b.Dy()/invH)))
}

// window is every window's frame: a cw by ch cell box at g on dst, bg
// inside (nil: the inside stays as drawn), then the ribbon border in tiles
// t (the game's corner, side and top cells: borders), as DrawBorder does.
// It returns the inside, in dst's pixels.
func window(dst *gpu.Image, t [3]*gpu.Image, g gpu.GeoM, cw, ch int, bg color.Color) image.Rectangle {
	x0, y0 := g.Apply(8, 8)
	x1, y1 := g.Apply(float64(cw*8-8), float64(ch*8-8))
	if bg != nil {
		vector.DrawFilledRect(dst, float32(x0), float32(y0), float32(x1-x0), float32(y1-y0), bg)
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

// srcImage is a picture on the GPU, uploaded again only when its source changes.
type srcImage struct {
	img *gpu.Image
	src image.Image
}

// of is src on the GPU.
func (s *srcImage) of(src image.Image) *gpu.Image {
	if src != s.src {
		if s.img != nil {
			s.img.Deallocate()
		}
		s.img, s.src = gpu.NewImageFromImage(src), src
	}
	return s.img
}

// draw3D draws the 3D view of the area and its minimap.
func (p *Presenter) draw3D(dst *gpu.Image, v engine.View, sc *blobber.Scene, note Note) {
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
}

// ownLayout: v is a page whose screen lays itself out on the page canvas
// (the special screens, the pagers and maps, the after-combat page, the
// credits); every other page is drawGeneric's.
func ownLayout(v engine.View) bool {
	return pageScreen(v) || modal(v) || v.SheetPage != nil || v.Creation != nil || v.Credits
}
