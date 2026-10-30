package engine

import (
	"image"
	"strconv"
	"strings"
)

// Grid pickers (extra): the creation's heads, bodies and combat icon parts
// are picked from a grid of all of them, as many columns and rows as the
// whole page holds (a larger window shows more; no bar: the box takes the
// prompt row too), centred in the box; a click takes one, the arrows move
// a frame and Enter takes it, Esc or the right button leave. What does not
// fit is on further pages: a pager line "< page 1/2 >" at the bottom of
// the box says so, its arrows (clicked, or PgUp, PgDn and the wheel) turn
// the page.

// pickGrid shows n pictures, each cw by ch cells, drawn by draw(i, x, y)
// with its top left at cell (x, y), starting with cur framed. The index
// picked, or -1 (Esc).
func (e *Engine) pickGrid(n, cw, ch, cur int, draw func(i, x, y int)) int {
	if cur < 0 || cur >= n {
		cur = 0
	}
	var cols, rows, gap, x0, y0, bot, pager int // the layout, from the page's size (layout)
	layout := func() {
		bot = promptRow - 1 // the box's inner bottom: the page's last row holds its border
		gap, cols = 1, winRight/(cw+1)
		if winRight/cw > cols { // a blank between the columns only when it costs none
			gap, cols = 0, winRight/cw
		}
		cols = max(1, min(cols, n))
		rows, pager = max(1, bot/ch), 0
		if (n+cols-1)/cols > rows { // paged: the pager line takes the box's last row
			rows, pager = max(1, (bot-1)/ch), bot
		}
		rows = min(rows, (n+cols-1)/cols)
		x0 = 1 + (winRight-(cols*(cw+gap)-gap))/2
		y0 = 1 + (bot-pager/bot-rows*ch)/2
	}
	page := func() int { return cur / (cols * rows) }
	pages := func() int { return (n + cols*rows - 1) / (cols * rows) }
	cell := func(i int) (x, y int) { return x0 + i%cols*(cw+gap), y0 + (i/cols-page()*rows)*ch }
	at := func(cx, cy int) int { // the picture at a cell, or -1
		cx, cy = cx-x0, cy-y0
		col, row := cx/(cw+gap), cy/ch
		if i := (page()*rows+row)*cols + col; cx >= 0 && cy >= 0 && col < cols && row < rows && cx%(cw+gap) < cw && i < n {
			return i
		}
		return -1
	}
	pagerText := func() string { return "< page " + strconv.Itoa(page()+1) + "/" + strconv.Itoa(pages()) + " >" }
	pagerAt := func(cx, cy int) bool { // the pager's arrow at a cell turns the page
		if pager == 0 || cy != pager {
			return false
		}
		w := len(pagerText())
		switch x := (textCols - w) / 2; {
		case cx == x && page() > 0:
			cur = max(cur-cols*rows, 0)
		case cx == x+w-1 && page() < pages()-1:
			cur = min(cur+cols*rows, n-1)
		default:
			return false
		}
		return true
	}
	show := func() {
		layout()
		e.ClearScreen()
		e.DrawBorder(1, 1, winRight, bot, 0)
		first := page() * cols * rows
		for i := first; i < n && i < first+cols*rows; i++ {
			x, y := cell(i)
			draw(i, x, y)
		}
		x, y := cell(cur)
		x0, y0, x1, y1 := x*8, y*8, (x+cw)*8, (y+ch)*8
		for _, r := range [4]image.Rectangle{{image.Pt(x0, y0), image.Pt(x1, y0+1)}, {image.Pt(x0, y1-1), image.Pt(x1, y1)}, {image.Pt(x0, y0), image.Pt(x0+1, y1)}, {image.Pt(x1-1, y0), image.Pt(x1, y1)}} {
			e.fill(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, colLightGreen)
		}
		if pager > 0 {
			s := pagerText()
			x := (textCols - len(s)) / 2
			e.DisplayString(x, pager, colWhite, s)
			prev, next := colDarkGrey, colDarkGrey
			if page() > 0 {
				prev = colLightGreen
			}
			if page() < pages()-1 {
				next = colLightGreen
			}
			e.displayCharBg(x, pager, prev, colBlack, '<', true)
			e.displayCharBg(x+len(s)-1, pager, next, colBlack, '>', true)
		}
	}
	defer e.rawArrows()()
	defer e.redrawWith(e.relist)()
	defer e.fillCells(0, promptRow, lastCol, promptRow) // the border's bottom row, where the caller's bar goes
	for {
		show()
		picked := false
		mark := e.regionMark()
		e.addRegion(region{x0: 0, y0: 0, x1: 999, y1: 999, // the whole page: the layout may change under it (a resize)
			click: func(cx, cy int) Action {
				if pagerAt(cx, cy) {
					return Action{Kind: actRedraw}
				}
				if i := at(cx, cy); i >= 0 {
					cur, picked = i, true
					return Action{Kind: actRedraw}
				}
				return Action{}
			},
			hover: func(cx, cy int) {
				if i := at(cx, cy); i >= 0 && i != cur {
					cur = i
					show()
				}
			}})
		p := e.DisplayInput("", nil, colLightMagenta, colLightGreen, colWhite, true, false) // no bar: the wait alone
		e.regionCut(mark)
		switch p.ID {
		case PickEnter:
			return cur
		case PickRedraw:
			if picked {
				return cur
			}
		case NoPick:
			return -1
		case PickMove:
			switch p.Dir {
			case DirUp:
				cur = max(cur-cols, cur%cols)
			case DirDown:
				if cur+cols < n {
					cur += cols
				}
			case DirLeft:
				cur = max(cur-1, 0)
			case DirRight:
				cur = min(cur+1, n-1)
			case DirHome:
				cur = 0
			case DirEnd:
				cur = n - 1
			case DirPgUp, DirWheelUp:
				cur = max(cur-cols*rows, 0)
			case DirPgDn, DirWheelDown:
				cur = min(cur+cols*rows, n-1)
			}
		}
	}
}

// pickHead picks a portrait head from the 14: the number picked (1..14),
// or 0.
func (e *Engine) pickHead(cur int) int {
	return 1 + e.pickGrid(len(portraitHead)-1, 11, 5, cur-1, func(i, x, y int) {
		e.DrawPicture(x, y, e.LoadPic(charArt.file("HEAD"), int(portraitHead[i+1])), 0)
	})
}

// pickBody picks a portrait body from the 12 (1..12), or 0.
func (e *Engine) pickBody(cur int) int {
	return 1 + e.pickGrid(12, 11, 6, cur-1, func(i, x, y int) {
		e.DrawPicture(x, y, e.LoadPic(charArt.file("BODY"), int(portraitBody[i+1])), 0)
	})
}

// pickSaved (extra): the Add and Forget lists are cards, the head of the
// character's portrait (a dropped PNG fitted into its place) with the name
// and class beside it, laid out and paged by pickGrid; a character whose
// files are gone (nil) is its name alone. The index picked, or -1.
func (e *Engine) pickSaved(chars []*Character, names []string, cur int) int {
	w := 0
	for i, c := range chars {
		w = max(w, len(names[i]))
		if c != nil {
			w = max(w, len(className(c.Class)))
		}
	}
	return e.pickGrid(len(chars), 12+w, 5, cur, func(i, x, y int) {
		c := chars[i]
		switch {
		case c == nil:
		case c.Portrait != nil:
			e.pg.overlays = append(e.pg.overlays, Overlay{Img: c.Portrait, At: fit(c.Portrait.Bounds(), CellRect{x, y, x + 10, y + 4}.Pixels())})
		case c.PortHead > 0 && int(c.PortHead) < len(portraitHead):
			e.DrawPicture(x, y, e.LoadPic(charArt.file("HEAD"), int(portraitHead[c.PortHead])), 0)
		}
		e.DisplayString(x+12, y+1, colWhite, names[i])
		if c != nil {
			e.DisplayString(x+12, y+3, colLightGreen, className(c.Class))
		}
	})
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
