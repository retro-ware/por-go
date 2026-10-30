package engine

import "image"

// Grid pickers (extra): the creation's heads, bodies and combat icon parts
// are picked from a grid of all of them; a click takes one, the arrows
// move a frame and Keep or Enter take it; the grid scrolls by rows when
// it does not fit (Next, Prev, PgUp, PgDn, the wheel).

// pickGrid shows n pictures, cols across, each cw by ch cells, drawn by
// draw(i, x, y) with its top left at cell (x, y), starting with cur
// framed. The index picked, or -1 (Exit, Esc).
func (e *Engine) pickGrid(prompt string, n, cols, cw, ch, cur int, draw func(i, x, y int)) int {
	gap := 0
	if cols*(cw+1)-1 <= winRight {
		gap = 1
	}
	rows := winBottom / ch // rows shown at once
	all := (n + cols - 1) / cols
	top := 0
	if cur < 0 || cur >= n {
		cur = 0
	}
	cell := func(i int) (x, y int) { return 1 + i%cols*(cw+gap), 1 + (i/cols-top)*ch }
	at := func(cx, cy int) int { // the picture at a cell, or -1
		col, row := (cx-1)/(cw+gap), top+(cy-1)/ch
		if i := row*cols + col; cx >= 1 && cy >= 1 && col < cols && (cx-1)%(cw+gap) < cw && i < n {
			return i
		}
		return -1
	}
	show := func() {
		e.ClearScreen()
		e.DrawBorder(1, 1, winRight, winBottom, 0)
		for i := top * cols; i < n && i < (top+rows)*cols; i++ {
			x, y := cell(i)
			draw(i, x, y)
		}
		x, y := cell(cur)
		x0, y0, x1, y1 := x*8, y*8, (x+cw)*8, (y+ch)*8
		for _, r := range [4]image.Rectangle{{image.Pt(x0, y0), image.Pt(x1, y0+1)}, {image.Pt(x0, y1-1), image.Pt(x1, y1)}, {image.Pt(x0, y0), image.Pt(x0+1, y1)}, {image.Pt(x1-1, y0), image.Pt(x1, y1)}} {
			e.fill(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, colLightGreen)
		}
	}
	const (
		keep Choice = iota
		next
		prev
		exit
	)
	defer e.rawArrows()()
	for {
		top = min(max(top, cur/cols-rows+1), cur/cols)
		show()
		picked := false
		mark := e.regionMark()
		e.addRegion(region{x0: 1, y0: 1, x1: winRight, y1: winBottom,
			click: func(cx, cy int) Action {
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
		bar := Bar{{"Keep", keep}}
		if top+rows < all {
			bar = append(bar, Entry{"Next", next})
		}
		if top > 0 {
			bar = append(bar, Entry{"Prev", prev})
		}
		bar = append(bar, Entry{"Exit", exit})
		p := e.DisplayInput(prompt, bar, colLightMagenta, colLightGreen, colWhite, true, false)
		e.regionCut(mark)
		switch p.ID {
		case keep, PickEnter:
			return cur
		case PickRedraw:
			if picked {
				return cur
			}
		case exit, NoPick:
			return -1
		case next:
			top = min(top+rows, all-rows)
			cur = min(max(cur, top*cols), n-1)
		case prev:
			top = max(top-rows, 0)
			cur = min(cur, (top+rows)*cols-1)
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
				cur = max(cur-cols*rows, cur%cols)
			case DirPgDn, DirWheelDown:
				cur = min(cur+cols*rows, n-1)
			}
		}
	}
}

// pickHead picks a portrait head from the 14: the number picked (1..14),
// or 0.
func (e *Engine) pickHead(cur int) int {
	return 1 + e.pickGrid("Head: ", len(portraitHead)-1, 3, 11, 5, cur-1, func(i, x, y int) {
		e.DrawPicture(x, y, e.LoadPic(charArt.file("HEAD"), int(portraitHead[i+1])), 0)
	})
}

// pickBody picks a portrait body from the 12 (1..12), or 0.
func (e *Engine) pickBody(cur int) int {
	return 1 + e.pickGrid("Body: ", 12, 3, 11, 6, cur-1, func(i, x, y int) {
		e.DrawPicture(x, y, e.LoadPic(charArt.file("BODY"), int(portraitBody[i+1])), 0)
	})
}

// iconRect is the page rectangle of a combat icon pose drawn at cell (x, y).
func iconRect(x, y int) image.Rectangle { return image.Rect(x*8, y*8, x*8+24, y*8+24) }
