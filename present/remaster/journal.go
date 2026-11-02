package remaster

import (
	"image"
	"image/color"
	"image/draw"
	"slices"

	"gpu"

	"por/engine"
	"por/present/input"
)

// The quest journal (View.Quests): a page of its own over the whole
// window while exploring, opened by J or the command menu's Journal. On
// the left a leather-bound list, the open quests first and then the done
// ones (struck through), Close last; on the right, on parchment, the quest
// under the highlight: who gave it where, and what they said. Keys: the
// arrows pick, PgUp/PgDn (the wheel over the page, the pad's shoulders)
// scroll the page, Esc, J or Close shut it; the pointer picks as it moves,
// a click on Close or the right button shut it.

type journalUI struct {
	open    bool
	sel     int               // the highlighted row of rows
	top     int               // the first row shown
	y, maxY int               // the page's first pixel row shown, its last
	at      int               // the quest y scrolls (-1 none)
	rows    []int             // the list: a quest of v.Quests, -1 a heading, -2 Close
	rects   []image.Rectangle // rows on the window, from top
	pane    image.Rectangle   // the page on the window
	pointer image.Point
	page    sheetCanvas
}

// The journal's inks: a leather list and parchment pages.
var (
	jLeather   = color.RGBA{0x3a, 0x22, 0x14, 0xff}
	jLeatherHi = color.RGBA{0x8a, 0x5a, 0x1c, 0xff}
	jParch     = color.RGBA{0xe8, 0xd8, 0xb0, 0xff}
	jParchDark = color.RGBA{0xd2, 0xbc, 0x8a, 0xff}
	jInk       = color.RGBA{0x3a, 0x26, 0x18, 0xff}
	jRed       = color.RGBA{0x8a, 0x1c, 0x14, 0xff}
	jGold      = color.RGBA{0xf0, 0xc8, 0x50, 0xff}
	jCream     = color.RGBA{0xf4, 0xe8, 0xcc, 0xff}
	jFaded     = color.RGBA{0x9a, 0x84, 0x6a, 0xff}
)

// journalRows lists the quests: a heading and the open ones, a heading
// and the done ones, then Close.
func journalRows(qs []engine.Quest) []int {
	var rows []int
	for _, done := range []bool{false, true} {
		head := len(rows)
		for i, q := range qs {
			if q.Done == done {
				rows = append(rows, i)
			}
		}
		if len(rows) > head {
			rows = slices.Insert(rows, head, -1)
		}
	}
	return append(rows, -2)
}

// pick moves the highlight d selectable rows on (headings skipped).
func (j *journalUI) pick(d int) {
	for i := j.sel + d; i >= 0 && i < len(j.rows); i += d {
		if j.rows[i] != -1 {
			j.sel = i
			return
		}
	}
}

func (p *Presenter) openJournal(v engine.View) {
	j := &p.journal
	j.open, j.sel, j.top, j.at = true, 0, 0, -1
	j.rows = journalRows(v.Quests)
	j.pick(1)
}

// journalKey takes key k while the journal is up.
func (p *Presenter) journalKey(k uint16) {
	j := &p.journal
	switch a := actOf(k); {
	case a == input.Up:
		j.pick(-1)
	case a == input.Down:
		j.pick(1)
	case a == input.Home:
		j.sel = 0
		j.pick(1)
	case a == input.End:
		j.sel = len(j.rows) - 1
	case a == input.PgUp:
		j.y -= 80
	case a == input.PgDn:
		j.y += 80
	case a == input.Esc || k == 'j' || k == 'J':
		j.open = false
	case a == input.Enter || a == input.Space || a == input.PadA:
		if j.sel < len(j.rows) && j.rows[j.sel] == -2 {
			j.open = false
		}
	}
}

// journalRow is the row under window pixel pt (-1 none).
func (j *journalUI) row(pt image.Point) int {
	for i, r := range j.rects {
		if pt.In(r) && j.top+i < len(j.rows) && j.rows[j.top+i] != -1 {
			return j.top + i
		}
	}
	return -1
}

// journalMouse takes the pointer while the journal is up.
func (p *Presenter) journalMouse(pt image.Point, click, right bool, wheel int) {
	j := &p.journal
	if pt != j.pointer {
		j.pointer = pt
		if i := j.row(pt); i >= 0 {
			j.sel = i
		}
	}
	if right {
		j.open = false
		return
	}
	if click {
		if i := j.row(pt); i >= 0 && j.rows[i] == -2 {
			j.open = false
		}
	}
	for ; wheel != 0; wheel -= unit(wheel) {
		if pt.In(j.pane) {
			j.y -= 10 * unit(wheel)
		} else {
			j.pick(-unit(wheel))
		}
	}
}

// drawJournal draws the journal over the whole window at the inventory's
// scale and cells.
func (p *Presenter) drawJournal(dst *gpu.Image, note Note, v engine.View) {
	j := &p.journal
	sc := &j.page
	b := dst.Bounds()
	k := max(1, min(b.Dx()/invW, b.Dy()/invH))
	cw, ch := max(invW, b.Dx()/k)/8, max(invH, b.Dy()/k)/8
	if w, h := cw*8, ch*8; gpu.Sized(&sc.img, w, h) {
		sc.w, sc.h, sc.px = w, h, image.NewRGBA(image.Rect(0, 0, w, h))
	}
	fill := func(m *image.RGBA, r image.Rectangle, c color.RGBA) {
		draw.Draw(m, r, image.NewUniform(c), image.Point{}, draw.Src)
	}
	text := func(m *image.RGBA, s string, x, y int, c color.RGBA) { drawText(m, note.Glyph, s, x, y, c) }
	const listCells = 24
	lw := listCells * 8
	fill(sc.px, sc.px.Rect, jLeather)
	for y := 0; y < sc.h; y += 4 { // the leather's grain
		fill(sc.px, image.Rect(0, y, lw, y+1), color.RGBA{0x34, 0x1e, 0x12, 0xff})
	}

	// the list: a row every 12 pixels, scrolling with the highlight
	j.rows = journalRows(v.Quests)
	j.sel = min(max(j.sel, 0), len(j.rows)-1)
	if j.rows[j.sel] == -1 {
		j.pick(1)
	}
	const rowH = 12
	fit := max(1, (ch*8-24)/rowH)
	j.top = min(max(j.top, j.sel-fit+1), j.sel, max(0, len(j.rows)-fit))
	var rows []image.Rectangle
	for i := j.top; i < min(len(j.rows), j.top+fit); i++ {
		y := 12 + (i-j.top)*rowH
		r := image.Rect(10, y-2, lw-6, y+10)
		rows = append(rows, r)
		switch q := j.rows[i]; {
		case q == -1:
			h := "Open quests"
			if v.Quests[j.rows[i+1]].Done {
				h = "Completed"
			}
			text(sc.px, h, 12, y, jGold)
			fill(sc.px, image.Rect(12, y+9, 12+8*len(h), y+10), jLeatherHi)
		case q == -2:
			if i == j.sel {
				fill(sc.px, r, jLeatherHi)
			}
			text(sc.px, "Close", 20, y, jCream)
		default:
			if i == j.sel {
				fill(sc.px, r, jLeatherHi)
			}
			c, mark := jCream, "\x04" // a diamond in the game's font (CP437)
			if v.Quests[q].Done {
				c, mark = jFaded, "+"
			}
			name := upTo(v.Quests[q].Name, listCells-5)
			text(sc.px, mark, 12, y, jGold)
			text(sc.px, name, 24, y, c)
			if v.Quests[q].Done {
				fill(sc.px, image.Rect(24, y+4, 24+8*len(name), y+5), c)
			}
		}
	}
	if len(v.Quests) == 0 {
		text(sc.px, "No quests yet.", 12, 24, jFaded)
	}

	// the page: parchment, the quest under the highlight
	rx, rw := lw, (cw-listCells-1)*8
	view := (ch - 2) * 8
	j.pane = image.Rect(rx, 8, rx+rw, 8+view)
	pg := image.NewRGBA(image.Rect(0, 0, rw, view))
	q := -1
	if j.sel < len(j.rows) {
		q = j.rows[j.sel]
	}
	if q >= 0 {
		qu := v.Quests[q]
		cols := rw/8 - 3
		var lines [][2]any // text, ink
		add := func(s string, c color.RGBA) { lines = append(lines, [2]any{s, c}) }
		for _, l := range engine.WrapWords("Given by "+qu.Giver, cols) {
			add(l, jInk)
		}
		for _, l := range engine.WrapWords(qu.Place, cols) {
			add(l, jFaded)
		}
		add("", jInk)
		for _, t := range qu.Text {
			for _, l := range engine.WrapWords(t, cols) {
				add(l, jInk)
			}
			add("", jInk)
		}
		tall := 44 + 10*len(lines)
		ent := image.NewRGBA(image.Rect(0, 0, rw, max(tall, view)))
		fill(ent, ent.Rect, jParch)
		for y := 0; y < ent.Rect.Dy(); y += 10 { // faint ruled lines
			fill(ent, image.Rect(8, y+9, rw-8, y+10), jParchDark)
		}
		status, stc := "Open", jRed
		if qu.Done {
			status, stc = "Completed", jFaded
		}
		text(ent, qu.Name, (rw-8*len(qu.Name))/2, 8, jRed)
		fill(ent, image.Rect(16, 20, rw-16, 21), jInk)
		text(ent, status, (rw-8*len(status))/2, 26, stc)
		y := 44
		for _, l := range lines {
			text(ent, l[0].(string), 12, y, l[1].(color.RGBA))
			y += 10
		}
		if j.at != q {
			j.at, j.y = q, 0
		}
		j.maxY = max(0, tall-view)
		j.y = min(max(j.y, 0), j.maxY)
		draw.Draw(pg, pg.Rect, ent, image.Pt(0, j.y), draw.Src)
	} else {
		fill(pg, pg.Rect, jParch)
		s := "Adventurer's Journal"
		text(pg, s, (rw-8*len(s))/2, view/2-8, jRed)
	}
	for x := 0; x < rw; x++ { // the page's edges darken
		for _, y := range []int{0, 1, view - 2, view - 1} {
			pg.SetRGBA(x, y, jParchDark)
		}
	}
	draw.Draw(sc.px, j.pane, pg, image.Point{}, draw.Src)
	if note.Show && note.Text != "" {
		noteLine(sc.px, note, (ch-1)*8)
	}

	sc.img.WritePixels(sc.px.Pix)
	t := p.border.tiles
	window(sc.img, t, gpu.GeoM{}, listCells+1, ch, nil)
	var g gpu.GeoM
	g.Translate(float64(listCells)*8, 0)
	window(sc.img, t, g, cw-listCells, ch, nil)
	onBorder(sc.img, note.Glyph, (lw-8*9)/2, 0, true, []string{"Journal"}, []color.RGBA{white})
	kx, ky := p.fit(dst, sc.img, sc.w, sc.h)
	on := func(r image.Rectangle) image.Rectangle {
		return image.Rect(int(float64(r.Min.X)*kx), int(float64(r.Min.Y)*ky), int(float64(r.Max.X)*kx), int(float64(r.Max.Y)*ky))
	}
	j.pane = on(j.pane)
	j.rects = j.rects[:0]
	for _, r := range rows {
		j.rects = append(j.rects, on(r))
	}
}
