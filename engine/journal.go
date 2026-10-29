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

// loadJournal reads JOURNAL.TXT: "[Title]" starts a text, a blank line a
// paragraph, "<FILE.PNG>" a picture, "#" lines are comments.
func loadJournal() map[string][]string {
	b, _ := journalFS.ReadFile("journal/JOURNAL.TXT")
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
	j := loadJournal()
	var lines []string
	for _, t := range newRefs {
		paras, ok := j[t]
		if !ok {
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "\x00"+t)
		for _, p := range paras {
			switch {
			case p == "":
				lines = append(lines, "")
			case strings.HasPrefix(p, "<") && strings.HasSuffix(p, ">"):
				if im := journalPicture(p[1 : len(p)-1]); im != nil {
					lines = append(lines, "\x01"+p[1:len(p)-1])
					_, h := pictureSize(im)
					for range min((h+7)/8, jrRows) - 1 {
						lines = append(lines, "\x02") // rows the picture covers
					}
				}
			case len(p) <= jrWidth: // as written: keeps the spacing of tables (entry 53)
				lines = append(lines, p)
			default:
				lines = append(lines, wrapText(p, jrWidth)...)
			}
		}
	}
	if len(lines) > 0 {
		e.showJournal(lines)
	}
}

// The pager: the text inside a full-screen box, jrRows lines a page.
const (
	jrWidth = 0x24
	jrRows  = 0x15
)

// wrapText breaks s into lines of at most w characters at spaces.
func wrapText(s string, w int) []string {
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

// showJournal pages through lines ("\x00" marks a title) until the last
// page or Esc, then puts the screen back. On a page with a tall picture,
// Up and Down (and the wheel) scroll it.
func (e *Engine) showJournal(lines []string) {
	off := e.modalOn()
	e.ext.paging = true
	defer func() { off(); e.ext.paging = false }()
	restore := e.saveScreen()
	pages := journalPages(lines)
	draw := func(p, off int) (maxOff int) {
		e.setOverlays(nil)
		e.DrawBoxTitled(1, 1, winRight, 23, 0, colWhite, "Adventurer's Journal")
		e.ClearCells(2, 2, 37, 23)
		page := pages[p]
		for i, l := range page {
			switch {
			case strings.HasPrefix(l, "\x00"):
				e.DisplayString(2, 2+i, colYellow, l[1:])
				e.speak(strings.ReplaceAll(l[1:], " ", "_")) // per page: the Enter that turns it silences the last one
			case strings.HasPrefix(l, "\x01"):
				maxOff = max(maxOff, e.drawPicture(journalPicture(l[1:]), 2*8+jrWidth*4, (2+i)*8, pictureRows(page[i:])*8, off))
			case l != "\x02":
				e.DisplayString(2, 2+i, colWhite, l)
			}
		}
		more := "press a key"
		if p < len(pages)-1 {
			more = "more: " + strconv.Itoa(p+1) + "/" + strconv.Itoa(len(pages))
		}
		if maxOff > 0 {
			more = "up/down: scroll, " + more
		}
		e.DisplayString(winRight-1-len(more), messageRow, colLightGreen, more)
		return maxOff
	}
	for p := 0; p < len(pages); p++ {
		for off := 0; ; {
			maxOff := draw(p, off)
			k := e.readKey()
			if k == 27 {
				p = len(pages)
				break
			}
			if maxOff > 0 && k == 0x4800 {
				off = max(off-16, 0)
				continue
			}
			if maxOff > 0 && k == 0x5000 && off < maxOff { // at the bottom, Down pages on
				off = min(off+16, maxOff)
				continue
			}
			break
		}
	}
	e.setOverlays(nil)
	restore()
}

func (e *Engine) setOverlays(o []Overlay) { e.pg.overlays = o }

// journalPages cuts lines into pages of at most jrRows; a page does not
// start with a blank line or end with a title, and a picture is not split:
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
		for i := 0; i < min(jrRows, len(lines)); i++ {
			if !strings.HasPrefix(lines[i], "\x01") {
				continue
			}
			rows := pictureRows(lines[i:])
			if left := jrRows - i; rows > left && left*2 >= rows {
				lines = append(lines[:i+left], lines[i+rows:]...) // drop the rows it no longer covers
			}
		}
		n := min(jrRows, len(lines))
		for n < len(lines) && n > 1 && lines[n] == "\x02" { // back to the picture's first row
			n--
		}
		if n < len(lines) && n > 1 && strings.HasPrefix(lines[n-1], "\x00") {
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

var pictures = map[string]image.Image{} // the engine goroutine's alone

// pictureSize is the screen area a picture covers: the scans are 300 dpi,
// shown at about 90 screen pixels an inch, at most the text width wide
// and, unless taller than wide (those scroll), 19 rows high (with a title
// and a line it fills a page).
func pictureSize(im image.Image) (w, h int) {
	b := im.Bounds()
	s := min(0.3, float64(jrWidth*8)/float64(b.Dx()))
	if !tall(im) {
		s = min(s, 152/float64(b.Dy()))
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

// drawPicture puts a journal picture centred on x, top at y, at most maxH
// high: paper white on the screen, the scan itself over it at the window's
// resolution. One taller than that shrinks to fit, or, if taller than
// wide, shows maxH of it from off pixels down; it returns how far it
// scrolls (0: not at all).
func (e *Engine) drawPicture(im image.Image, x, y, maxH, off int) (maxOff int) {
	if im == nil {
		return 0
	}
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
	r := image.Rect(x-w/2, y, x-w/2+w, y+h)
	e.fill(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, 15)
	e.pg.overlays = append(e.pg.overlays, Overlay{Img: im, Src: src, At: r})
	return maxOff
}
