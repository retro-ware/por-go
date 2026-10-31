package engine

import (
	"image"
	"strconv"
	"strings"
)

// Grid pickers (extra): the creation's heads, bodies and combat icon parts
// and the saved characters are picked from a grid of all of them, as many
// columns and rows as the whole page holds (a larger window shows more; no
// bar: the box takes the prompt row too), centred in the box. What does not
// fit is on further pages: a pager line "< page 1/2 >" at the bottom of the
// box says so. The engine draws the page the presenter asks for (ActPage)
// and publishes where each picture is (View.Picker); the frame, the cursor,
// the clicks and the pager's arrows are the presenter's. ActPicked takes a
// picture, ActClose leaves.

// PickHead .. PickWeapon are the pickers ActPicker opens.
const (
	PickHead     = iota // the portrait's head (creation sheet)
	PickBody            // its body
	PickIconHead        // the combat icon's head (icon editor)
	PickWeapon          // its weapon
)

// picker is a grid picker up.
type picker struct {
	n, cw, ch, cur, page int
	draw                 func(i, x, y int) // draws picture i, its top left at cell (x, y)
	pick                 func(i int) bool  // takes picture i; done when the picker goes
	labels               []string          // the pictures' names (saved characters)
	ask                  string            // asked before a pick (%s: the label)

	cols, rows, gap, x0, y0, bot, pager int // the layout, from the page's size
}

func (p *picker) layout() {
	p.bot = promptRow - 1 // the box's inner bottom: the page's last row holds its border
	p.gap, p.cols = 1, winRight/(p.cw+1)
	if winRight/p.cw > p.cols { // a blank between the columns only when it costs none
		p.gap, p.cols = 0, winRight/p.cw
	}
	p.cols = max(1, min(p.cols, p.n))
	p.rows, p.pager = max(1, p.bot/p.ch), 0
	if (p.n+p.cols-1)/p.cols > p.rows { // paged: the pager line takes the box's last row
		p.rows, p.pager = max(1, (p.bot-1)/p.ch), p.bot
	}
	p.rows = min(p.rows, (p.n+p.cols-1)/p.cols)
	p.x0 = 1 + (winRight-(p.cols*(p.cw+p.gap)-p.gap))/2
	p.y0 = 1 + (p.bot-p.pager/p.bot-p.rows*p.ch)/2
	p.page = min(max(p.page, 0), p.pages()-1)
}

func (p *picker) perPage() int { return p.cols * p.rows }
func (p *picker) pages() int   { return max(1, (p.n+p.perPage()-1)/p.perPage()) }
func (p *picker) cell(i int) (x, y int) {
	return p.x0 + i%p.cols*(p.cw+p.gap), p.y0 + (i/p.cols-p.page*p.rows)*p.ch
}
func (p *picker) pagerText() string {
	return "< page " + strconv.Itoa(p.page+1) + "/" + strconv.Itoa(p.pages()) + " >"
}

// pushPicker puts a picker of n pictures, each cw by ch cells, up on the
// page holding cur.
func (e *Engine) pushPicker(n, cw, ch, cur int, draw func(i, x, y int), pick func(i int) bool) *picker {
	p := &picker{n: n, cw: cw, ch: ch, cur: min(max(cur, 0), max(n-1, 0)), draw: draw, pick: pick}
	p.layout()
	p.page = p.cur / p.perPage()
	under := e.picker
	e.picker = p
	show := func() { e.showPicker(p) }
	off := e.redrawWith(show)
	e.push(modePicker, func(a Action) bool {
		switch a.Kind {
		case ActClose:
			return true
		case ActPage:
			p.page = a.N
		case ActPicked:
			if a.N < 0 || a.N >= p.n {
				return false
			}
			p.cur = a.N
			if p.pick(a.N) || p.n == 0 {
				return true
			}
		case actRedraw:
		default:
			return false
		}
		show()
		return false
	}, func() {
		off()
		e.picker = under
		e.fillCells(0, promptRow, lastCol, promptRow) // the border's bottom row, where the caller's bar goes
	})
	show()
	return p
}

// showPicker draws the page of the picker up.
func (e *Engine) showPicker(p *picker) {
	p.layout()
	e.ClearScreen()
	e.DrawBorder(1, 1, winRight, p.bot, 0)
	first := p.page * p.perPage()
	for i := first; i < p.n && i < first+p.perPage(); i++ {
		x, y := p.cell(i)
		p.draw(i, x, y)
	}
	if p.pager > 0 {
		s := p.pagerText()
		e.DisplayString((textCols-len(s))/2, p.pager, colWhite, s)
	}
}

// pickerView is the picker up for the View.
func (e *Engine) pickerView() *Picker {
	p := e.picker
	if p == nil {
		return nil
	}
	v := &Picker{N: p.n, Page: p.page, Pages: p.pages(), PerPage: p.perPage(), Cols: p.cols, Cur: p.cur, Labels: p.labels, Ask: p.ask}
	first := p.page * p.perPage()
	for i := first; i < p.n && i < first+p.perPage(); i++ {
		x, y := p.cell(i)
		v.Cells = append(v.Cells, CellRect{x, y, x + p.cw - 1, y + p.ch - 1})
	}
	if p.pager > 0 {
		x := (textCols - len(p.pagerText())) / 2
		v.Prev, v.Next = CellRect{x, p.pager, x, p.pager}, CellRect{x + len(p.pagerText()) - 1, p.pager, x + len(p.pagerText()) - 1, p.pager}
	}
	return v
}

// pickHead puts the picker of the 14 portrait heads up; set gets the
// number picked (1..14).
func (e *Engine) pickHead(cur int, set func(int)) {
	e.pushPicker(len(portraitHead)-1, 11, 5, cur-1, func(i, x, y int) {
		e.DrawPicture(x, y, e.LoadPic(charArt.file("HEAD"), int(portraitHead[i+1])), 0)
	}, func(i int) bool { set(i + 1); return true })
}

// pickBody: the 12 portrait bodies (1..12).
func (e *Engine) pickBody(cur int, set func(int)) {
	e.pushPicker(12, 11, 6, cur-1, func(i, x, y int) {
		e.DrawPicture(x, y, e.LoadPic(charArt.file("BODY"), int(portraitBody[i+1])), 0)
	}, func(i int) bool { set(i + 1); return true })
}

// pickSaved (extra): the Add and Forget lists are cards, the head of the
// character's portrait (a dropped PNG fitted into its place) with the name
// and class beside it, in a picker; a character whose files are gone (nil)
// is its name alone. pick takes one (done: the picker goes).
func (e *Engine) pickSaved(chars *[]*Character, names *[]string, pick func(i int) bool) *picker {
	w := 0
	for i, c := range *chars {
		w = max(w, len((*names)[i]))
		if c != nil {
			w = max(w, len(className(c.Class)))
		}
	}
	var p *picker
	p = e.pushPicker(len(*chars), 12+w, 5, 0, func(i, x, y int) {
		c := (*chars)[i]
		switch {
		case c == nil:
		case c.Portrait != nil:
			e.pg.overlays = append(e.pg.overlays, Overlay{Img: c.Portrait, At: fit(c.Portrait.Bounds(), CellRect{x, y, x + 10, y + 4}.Pixels())})
		case c.PortHead > 0 && int(c.PortHead) < len(portraitHead):
			e.DrawPicture(x, y, e.LoadPic(charArt.file("HEAD"), int(portraitHead[c.PortHead])), 0)
		}
		e.DisplayString(x+12, y+1, colWhite, (*names)[i])
		if c != nil {
			e.DisplayString(x+12, y+3, colLightGreen, className(c.Class))
		}
	}, func(i int) bool {
		done := pick(i)
		p.n, p.labels = len(*chars), *names
		return done
	})
	p.labels = *names
	return p
}

// loadSaved loads the saved characters named (nil where the files are gone).
func (e *Engine) loadSaved(names []string) []*Character {
	chars := make([]*Character, len(names))
	for i, n := range names {
		c := &Character{}
		if e.LoadCharacter(strings.TrimPrefix(n, "* "), c, false) { // ovr18:0AEC
			chars[i] = c
		}
	}
	return chars
}

// iconRect is the page rectangle of a combat icon pose drawn at cell (x, y).
func iconRect(x, y int) image.Rectangle { return image.Rect(x*8, y*8, x*8+24, y*8+24) }
