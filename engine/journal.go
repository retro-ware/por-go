package engine

import (
	"embed"
	"image"
	_ "image/png"
	"regexp"
	"strconv"
	"strings"
)

// Journal texts (extra): the game often only names an
// entry of the printed Adventurer's Journal ("SEE JOURNAL ENTRY 26", "YOU
// OVERHEAR TAVERN TALE 5", the proclamations at City Hall). When the text
// window names one, the entry is shown right there, with its pictures. The
// texts and pictures (pre-scaled to the screen, one shared 16-colour
// palette) are built in from journal/.

//go:embed journal
var journalFS embed.FS

// journalRef finds the references in the text window: "ENTRY 7", "ENTRIES
// 23 AND 14", "TAVERN TALE 5", "PROCLAMATION LIX.", "PROCLAMATIONS LXIV,
// LXXVIII, CIX, AND LIX.".
var journalRef = regexp.MustCompile(`\bENTRY\s*(\d+)|\bENTRIES\s*(\d+)\s+AND\s+(\d+)|\bTAVERN TALE\s*(\d+)|\bPROCLAMATIONS?\s*([IVXLCDM]+(?:(?:,\s*|\s+)(?:AND\s+)?[IVXLCDM]+)*)\b`)

var romanList = regexp.MustCompile(`[IVXLCDM]+`)

// journalRefs lists the titles of the texts s names, in order.
func journalRefs(s string) []string {
	var t []string
	for _, m := range journalRef.FindAllStringSubmatch(s, -1) {
		switch {
		case m[1] != "":
			t = append(t, "Journal Entry "+m[1])
		case m[2] != "":
			t = append(t, "Journal Entry "+m[2], "Journal Entry "+m[3])
		case m[4] != "":
			t = append(t, "Tavern Tale "+m[4])
		case m[5] != "":
			for _, r := range romanList.FindAllString(strings.ReplaceAll(m[5], "AND", ""), -1) {
				t = append(t, "Proclamation "+r)
			}
		}
	}
	return t
}

// loadJournal reads journal/name (JOURNAL.TXT, HISTORY.TXT): "[Title]"
// starts a text, a blank line a paragraph, "<FILE.PNG>" a picture, "#"
// lines are comments.
func loadJournal(name string) map[string][]string {
	b, _ := journalFS.ReadFile("journal/" + name)
	j := map[string][]string{}
	var title string
	for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "#"):
		case strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]"):
			title = l[1 : len(l)-1]
			j[title] = nil
		case title != "":
			j[title] = append(j[title], l)
		}
	}
	return j
}

// journalPrinted follows what opPrint put into the text window (clear: it
// started over) and shows the journal texts it names for the first time.
func (e *Engine) journalPrinted(s string, clear bool) {
	if clear {
		e.ext.printed, e.ext.refsShown = "", 0
	}
	e.ext.printed += s
	refs := journalRefs(strings.ToUpper(e.ext.printed))
	if len(refs) <= e.ext.refsShown {
		return
	}
	newRefs := refs[e.ext.refsShown:]
	e.ext.refsShown = len(refs)
	if lines := journalLines(newRefs); len(lines) > 0 {
		e.showJournal(journalBox, lines)
	}
}

// journalLines lays out the texts titled titles for the pager: "\x00" marks
// a title (a blank line under it), "\x01" a picture and "\x02" the rows it
// covers.
func journalLines(titles []string) []string {
	return journalLayout(loadJournal("JOURNAL.TXT"), titles, false)
}

// journalLayout is journalLines for the texts of j; voiced marks each
// paragraph's first line "\x03key\x03" with its own recording's key (the
// pager speaks it on the page where it starts, and keeps it on one page).
func journalLayout(j map[string][]string, titles []string, voiced bool) []string {
	var lines []string
	for _, t := range titles {
		paras, ok := j[t]
		if !ok {
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "\x00"+t, "")
		for _, p := range paras {
			switch {
			case p == "":
				lines = append(lines, "")
			case strings.HasPrefix(p, "<") && strings.HasSuffix(p, ">"):
				if im := journalPicture(p[1 : len(p)-1]); im != nil {
					lines = append(lines, "\x01"+p[1:len(p)-1])
					_, h := pictureSize(im)
					for range min((h+7)/8, jrRows()) - 1 {
						lines = append(lines, "\x02") // rows the picture covers
					}
				}
			case len(p) <= jrWidth: // as written: keeps the spacing of tables (entry 53)
				lines = append(lines, p)
			default:
				lines = append(lines, WrapWords(p, jrWidth)...)
			}
			if voiced && p != "" {
				for i := len(lines) - 1; i >= 0 && lines[i] != ""; i-- {
					if i == 0 || lines[i-1] == "" {
						lines[i] = "\x03" + voiceKey(p) + "\x03" + lines[i]
					}
				}
			}
		}
	}
	return lines
}

// The pager: the text inside a full-screen box, jrRows lines a page from
// row jrTop (under the journal's title and a blank row) to two rows above
// the "press a key" line (messageRow); a taller page holds more lines.
const (
	journalBox = "Adventurer's Journal"
	jrWidth    = 0x24
	jrTop      = 3
)

func jrRows() int { return messageRow - 1 - jrTop }

// WrapWords breaks s into lines of at most w characters at spaces.
func WrapWords(s string, w int) []string {
	var out []string
	line := ""
	for _, word := range strings.Fields(s) {
		for len(word) > w { // (no word is that long, but never loop)
			out = append(out, word[:w])
			word = word[w:]
		}
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= w:
			line += " " + word
		default:
			out = append(out, line)
			line = word
		}
	}
	return append(out, line)
}

// pager is help's or the journal's state (modeHelp, modeJournal): the
// page shown and how far a tall picture on it is scrolled.
type pager struct {
	title   string
	lines   []string // the journal's (journalLayout); none: the help
	p, off  int
	journal bool
}

// showJournal pages through lines ("\x00" marks a title) until the last
// page or Esc (View.Pages). On a page with a tall picture, ActPage -1 and
// +1 scroll it. The box is the whole page; a resize pages the text again
// for the new height. box is the border's title.
func (e *Engine) showJournal(box string, lines []string) {
	e.pushPager(modeJournal, &pager{title: box, lines: lines, journal: true})
}

// pushPager puts the pager pg up as screen m.
func (e *Engine) pushPager(m screenMode, pg *pager) {
	under := e.ext.pager
	e.ext.pager = pg
	e.push(m, func(a Action) bool { return e.applyPager(pg, a) }, func() { e.ext.pager = under })
	e.speakPage(pg)
}

// applyPager turns the page (or scrolls its picture) on a key; done after
// the last page or on Esc.
func (e *Engine) applyPager(pg *pager, a Action) (done bool) {
	if !e.pressed(a) {
		return false
	}
	page, maxOff := e.pagerPage(pg)
	up, down := a.Kind == ActPage && a.N < 0, a.Kind == ActPage && a.N > 0
	switch {
	case a.Kind == ActClose:
		return true
	case !pg.journal && up:
		pg.p = max(pg.p-1, 0)
	case maxOff > 0 && up:
		pg.off = max(pg.off-16, 0)
	case maxOff > 0 && down && pg.off < maxOff: // at the bottom, Down pages on
		pg.off = min(pg.off+16, maxOff)
	default:
		pg.p, pg.off = pg.p+1, 0
	}
	if pg.p >= page.Pages {
		return true
	}
	e.speakPage(pg)
	return false
}

// pagerPage is pg's page as View.Pages shows it, and how far its tall
// picture scrolls (0: none).
func (e *Engine) pagerPage(pg *pager) (*Pages, int) {
	if !pg.journal {
		pages := helpPages(e.ext.help)
		pg.p = min(pg.p, len(pages)-1)
		return e.helpPage(pg.p), 0
	}
	pages := journalPages(pg.lines)
	pg.p = min(pg.p, len(pages)-1) // a resize may have left fewer
	page := pages[pg.p]
	v := &Pages{Title: e.boxTitle(pg.title), Top: jrTop, Page: pg.p, Pages: len(pages)}
	maxOff := 0
	for i, l := range page {
		var line []PaneRun
		switch {
		case strings.HasPrefix(l, "\x00"):
			line = []PaneRun{e.textRun(2, colYellow, l[1:])}
		case strings.HasPrefix(l, "\x03"):
			_, text, _ := strings.Cut(l[1:], "\x03")
			line = []PaneRun{e.textRun(2, colWhite, text)}
		case strings.HasPrefix(l, "\x01"):
			name := l[1:]
			if sc, off := e.journalScan(name, 2*8+jrWidth*4, (jrTop+i)*8, pictureRows(page[i:])*8, pg.off); sc != nil {
				v.Scans = append(v.Scans, *sc)
				maxOff = max(maxOff, off)
			}
		case l != "\x02":
			line = []PaneRun{e.textRun(2, colWhite, l)}
		}
		v.Lines = append(v.Lines, line)
	}
	more := "press a key"
	if pg.p < len(pages)-1 {
		more = "more: " + strconv.Itoa(pg.p+1) + "/" + strconv.Itoa(len(pages))
	}
	if maxOff > 0 {
		more = "up/down: scroll, " + more
	}
	v.Hint = e.textRun(winRight-1-len(more), colLightGreen, more)
	return v, maxOff
}

// speakPage speaks the journal page's title and the paragraphs that start
// on it, in order (once each: the Enter that turns the page silences the
// last).
func (e *Engine) speakPage(pg *pager) {
	if !pg.journal {
		return
	}
	pages := journalPages(pg.lines)
	for _, l := range pages[min(pg.p, len(pages)-1)] {
		switch {
		case strings.HasPrefix(l, "\x00"):
			e.speak(strings.ReplaceAll(l[1:], " ", "_"))
		case strings.HasPrefix(l, "\x03"):
			key, _, _ := strings.Cut(l[1:], "\x03")
			e.speak(key)
		}
	}
}

// textRun is s as DisplayString draws it at column x in colour: on black,
// upcased, cut at the page's edge.
func (e *Engine) textRun(x, colour int, s string) PaneRun {
	s = s[:max(0, min(len(s), 255, textCols-x))]
	return PaneRun{X: x, Text: s, Fg: e.pal.RGBA(uint8(colour)), Bg: e.pal.RGBA(colBlack), Upcase: true}
}

// boxTitle is the title DrawBoxTitled centres on the top row of a box
// round the whole page.
func (e *Engine) boxTitle(t string) PaneRun {
	t = t[:min(len(t), 0x28)]
	return e.textRun((1+winRight-len(t))/2, colWhite, t)
}

// journalPages cuts lines into pages of at most jrRows; a page does not
// start or end with a blank line or end with a title, and a picture is not split:
// one that overruns the page shrinks into the rows left when they hold
// half of it (it stays sharp, at the window's resolution), else it starts
// the next page.
func journalPages(lines []string) [][]string {
	lines = append([]string(nil), lines...)
	var pages [][]string
	for len(lines) > 0 {
		for len(lines) > 0 && lines[0] == "" {
			lines = lines[1:]
		}
		for i := 0; i < min(jrRows(), len(lines)); i++ {
			if !strings.HasPrefix(lines[i], "\x01") {
				continue
			}
			rows := pictureRows(lines[i:])
			if left := jrRows() - i; rows > left && left*2 >= rows {
				lines = append(lines[:i+left], lines[i+rows:]...) // drop the rows it no longer covers
			}
		}
		n := min(jrRows(), len(lines))
		if n < len(lines) && lines[n] != "" { // a voiced paragraph is not split, unless it fills the page
			k := n - 1
			for k > 0 && lines[k] != "" && !strings.HasPrefix(lines[k], "\x03") {
				k--
			}
			if k > 0 && strings.HasPrefix(lines[k], "\x03") {
				n = k
			}
		}
		for n < len(lines) && n > 1 && lines[n] == "\x02" { // back to the picture's first row
			n--
		}
		for n < len(lines) && n > 1 && (lines[n-1] == "" || strings.HasPrefix(lines[n-1], "\x00")) {
			n--
		}
		if n > 0 {
			pages = append(pages, lines[:n])
		}
		lines = lines[n:]
	}
	return pages
}

// journalPicture decodes a picture from journal/ once; nil if missing.
func journalPicture(name string) image.Image {
	if im, ok := pictures[name]; ok {
		return im
	}
	var im image.Image
	if f, err := journalFS.Open("journal/" + name); err == nil {
		im, _, _ = image.Decode(f)
		f.Close()
	}
	pictures[name] = im
	return im
}

var pictures = map[string]image.Image{} // the engine's alone

// pictureSize is the screen area a picture covers: the scans are 300 dpi,
// shown at about 90 screen pixels an inch, at most the text width wide
// and, unless taller than wide (those scroll), 17 rows high (with a title,
// the blank line under it and a line it fills a page).
func pictureSize(im image.Image) (w, h int) {
	b := im.Bounds()
	s := min(0.3, float64(jrWidth*8)/float64(b.Dx()))
	if !tall(im) {
		s = min(s, 136/float64(b.Dy()))
	}
	return int(float64(b.Dx()) * s), int(float64(b.Dy()) * s)
}

func tall(im image.Image) bool { return im.Bounds().Dy() > im.Bounds().Dx() }

// pictureRows counts the rows of the picture starting lines.
func pictureRows(lines []string) int {
	n := 1
	for n < len(lines) && lines[n] == "\x02" {
		n++
	}
	return n
}

// journalScan places the journal picture name centred on x, top at y, at
// most maxH high (View.Pages' Scan): the scan on paper white, at the
// window's resolution. One taller than that shrinks to fit, or, if taller
// than wide, shows maxH of it from off pixels down; it returns how far it
// scrolls (0: not at all). nil: no such picture.
func (e *Engine) journalScan(name string, x, y, maxH, off int) (*Scan, int) {
	im := journalPicture(name)
	if im == nil {
		return nil, 0
	}
	maxOff := 0
	w, h := pictureSize(im)
	src := im.Bounds()
	switch {
	case h > maxH && tall(im):
		maxOff = h - maxH
		off = min(off, maxOff)
		src.Min.Y, src.Max.Y = src.Min.Y+off*src.Dy()/h, src.Min.Y+(off+maxH)*src.Dy()/h
		h = maxH
	case h > maxH:
		w, h = w*maxH/h, maxH
	}
	return &Scan{Name: name, Img: im, Src: src, At: image.Rect(x-w/2, y, x-w/2+w, y+h), Paper: e.pal.RGBA(15)}, maxOff
}
