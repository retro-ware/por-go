package engine

import (
	"bufio"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"por/pal"
	"sort"
	"strings"
)

// The full-screen map (extra): M on the command bar
// or a click on the minimap. It shows the minimap's area at 10 pixels a
// square with a letter on every labelled square and a legend beside it. A
// cursor (arrows, click) picks a square; Walk goes there (walk.go), Label
// names it, Remove clears it.
// New Phlan starts with its buildings labelled (phlanLabels); the labels of
// every area the player changed are kept in data/save/MAPNOTES.TXT.

// Label kinds, each with its colour on the maps.
const (
	poiUser   = 0
	poiShop   = 1
	poiTemple = 2
	poiInn    = 3
	poiCivic  = 4
)

var poiColour = [5]uint8{15, 10, 13, 14, 11}

type mapLabel struct {
	text string
	kind byte
}

// phlanLabels are the buildings of New Phlan (ECL3 block 0, GEO3 block 0):
// the squares whose event id (cell byte & 0x7F, the script's [6E82]) leads
// to a shop, temple, inn, tavern or office in the block's step event (ECL
// 9B4C ONGOTO).
var phlanLabels = func() map[int]mapLabel {
	m := map[int]mapLabel{}
	add := func(text string, kind byte, sq ...[2]int) {
		for _, s := range sq {
			m[s[1]<<4|s[0]] = mapLabel{text, kind}
		}
	}
	add("Boats", poiCivic, [2]int{15, 1})                                          // 1
	add("Harbor master", poiCivic, [2]int{11, 1})                                  // 2
	add("Bishop Braccio", poiTemple, [2]int{10, 5})                                // 5
	add("Temple of Tyr", poiTemple, [2]int{11, 3}, [2]int{8, 4}, [2]int{8, 6})     // 6
	add("Temple of Sune", poiTemple, [2]int{1, 1}, [2]int{3, 1}, [2]int{1, 3})     // 7
	add("Inn", poiInn, [2]int{4, 12}, [2]int{6, 12}, [2]int{4, 13}, [2]int{6, 13}, // 9
		[2]int{0, 14}, [2]int{1, 14}, [2]int{2, 14})
	add("Training Hall", poiCivic, [2]int{6, 1}, [2]int{6, 2}, [2]int{9, 0})          // 10, 17
	add("Shop", poiShop, [2]int{15, 8}, [2]int{9, 10}, [2]int{12, 10}, [2]int{9, 11}, // 19
		[2]int{11, 11})
	add("Tavern", poiInn, [2]int{10, 8}, [2]int{8, 9}, [2]int{14, 10}, [2]int{15, 10}, // 20
		[2]int{15, 12}, [2]int{15, 13}, [2]int{15, 14})
	add("Jeweler", poiShop, [2]int{8, 10})                                       // 21
	add("Arms and Armor", poiShop, [2]int{13, 8}, [2]int{8, 11}, [2]int{11, 12}, // 22
		[2]int{8, 13}, [2]int{9, 13})
	add("Silver Shop", poiShop, [2]int{11, 10}, [2]int{10, 13})                        // 23
	add("Temple of Tempus", poiTemple, [2]int{0, 12}, [2]int{2, 12})                   // 24
	add("Gate to slums", poiCivic, [2]int{0, 4})                                       // 25
	add("City Hall", poiCivic, [2]int{4, 4}, [2]int{4, 3}, [2]int{3, 4}, [2]int{5, 4}) // 27, 26
	return m
}()

const phlanKey = 0 // areaKey of New Phlan: ECL block 0, GEO block 0

const notesFile = "MAPNOTES.TXT"

// labels are the current area's labels (square y<<4|x -> label).
func (e *Engine) labels() map[int]mapLabel {
	k := e.areaKey()
	if m, ok := e.ext.labels[k]; ok {
		return m
	}
	if e.ext.labels == nil {
		e.ext.labels = map[int]map[int]mapLabel{}
	}
	m := map[int]mapLabel{}
	if k == phlanKey && e.set == 3 {
		for s, l := range phlanLabels {
			m[s] = l
		}
	}
	e.ext.labels[k] = m
	return m
}

// loadNotes reads MAPNOTES.TXT: "area KEY" starts an area, then
// "X Y KIND TEXT" per label.
func (e *Engine) loadNotes() {
	f, err := os.Open(filepath.Join(e.Cfg.SaveDir, notesFile))
	if err != nil {
		return
	}
	defer f.Close()
	e.ext.labels = map[int]map[int]mapLabel{}
	var cur map[int]mapLabel
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var k, x, y, kind int
		line := sc.Text()
		if _, err := fmt.Sscanf(line, "area %d", &k); err == nil {
			cur = map[int]mapLabel{}
			e.ext.labels[k] = cur
			continue
		}
		var text string
		if n, _ := fmt.Sscanf(line, "%d %d %d", &x, &y, &kind); n == 3 && cur != nil && onMap(x, y) && kind >= 0 && kind < len(poiColour) {
			if f := strings.SplitN(line, " ", 4); len(f) == 4 {
				text = f[3]
			}
			cur[y<<4|x] = mapLabel{text, byte(kind)}
		}
	}
}

// saveNotes writes the labels of every area that was looked at.
func (e *Engine) saveNotes() {
	var b strings.Builder
	keys := make([]int, 0, len(e.ext.labels))
	for k := range e.ext.labels {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "area %d\n", k)
		m := e.ext.labels[k]
		sq := make([]int, 0, len(m))
		for s := range m {
			sq = append(sq, s)
		}
		sort.Ints(sq)
		for _, s := range sq {
			fmt.Fprintf(&b, "%d %d %d %s\n", s&15, s>>4, m[s].kind, m[s].text)
		}
	}
	os.WriteFile(filepath.Join(e.Cfg.SaveDir, notesFile), []byte(b.String()), 0o644)
}

// legend gives every label text a letter (in order of the texts).
func legend(m map[int]mapLabel) (texts []string, letter map[string]byte, kind map[string]byte) {
	letter, kind = map[string]byte{}, map[string]byte{}
	for _, l := range m {
		if _, ok := letter[l.text]; !ok {
			letter[l.text] = 0
			kind[l.text] = l.kind
			texts = append(texts, l.text)
		}
	}
	sort.Slice(texts, func(i, j int) bool { return strings.ToLower(texts[i]) < strings.ToLower(texts[j]) })
	for i, t := range texts {
		c := byte('A' + i)
		if i >= 26 {
			c = byte('a' + i - 26)
		}
		letter[t] = c
	}
	return texts, letter, kind
}

// Full map geometry: 16x16 squares of fmCell pixels, the page's height
// under the title row and above the message row; the legend is to the
// right, from text column fmLegend.
const (
	fmCell   = 11
	fmX, fmY = 0, 8
	fmLegend = 0x16
	fmRows   = 0x15 // legend lines: rows 2..0x16
)

// saveScreen keeps the picture and its text cells; the func puts them back.
func (e *Engine) saveScreen() func() {
	cells, pg := e.ext.cells, e.pg
	pg.layers = append([]layer(nil), pg.layers...)
	return func() { e.ext.cells, e.pg = cells, pg }
}

// FullMap shows the full-screen map until Exit. It runs from the command
// bar's key handling and puts the screen back as it was.
func (e *Engine) FullMap() {
	if !e.minimapAllowed() {
		e.DisplayMessageWait(colYellow, "Not Here")
		return
	}
	restore := e.saveScreen()
	off := e.modalOn() // the command bar's regions under it take no hover or click
	inCommand, sel := e.ext.inCommand, e.menuSel
	e.ext.inCommand, e.ext.fullMap = false, true
	defer func() {
		off()
		restore()
		e.ext.inCommand, e.menuSel, e.ext.fullMap = inCommand, sel, false
	}()
	cx, cy := e.posX, e.posY
	if !onMap(cx, cy) {
		cx, cy = 0, 0
	}
	m := e.labels()
	mark := e.regionMark()
	defer e.regionCut(mark)
	e.addRegion(region{x0: fmX / 8, y0: fmY / 8, x1: (fmX + 16*fmCell - 1) / 8, y1: (fmY + 16*fmCell - 1) / 8, click: func(tx, ty int) Action {
		x, y := (tx*8+4-fmX)/fmCell, (ty*8+4-fmY)/fmCell
		if !onMap(x, y) {
			return Action{}
		}
		if x == cx && y == cy {
			return Action{Kind: actHotkey, N: 'L'} // a second click names the square
		}
		cx, cy = x, y
		return Action{Kind: actRedraw}
	}})
	texts := []string(nil)
	var firstSq map[string]int
	e.addRegion(region{x0: fmLegend, y0: 2, x1: 0x27, y1: 2 + fmRows - 1, click: func(_, y int) Action {
		if i := y - 2; i < len(texts) {
			s := firstSq[texts[i]]
			cx, cy = s&15, s>>4
			return Action{Kind: actRedraw}
		}
		return Action{}
	}})
	defer e.rawArrows()()
	for {
		texts, firstSq = e.drawFullMap(m, cx, cy)
		const (
			walk Choice = iota
			label
			remove
			exit
		)
		p := e.DisplayInput("", Bar{{"Walk", walk}, {"Label", label}, {"Remove", remove}, {"Exit", exit}}, colLightMagenta, colLightGreen, colWhite, true, false)
		switch p.ID {
		case PickMove:
			switch p.Dir {
			case DirUp:
				cy = max(cy-1, 0)
			case DirDown:
				cy = min(cy+1, 15)
			case DirLeft:
				cx = max(cx-1, 0)
			case DirRight:
				cx = min(cx+1, 15)
			}
		case walk:
			if e.startWalk(cx, cy) {
				return
			}
			e.DisplayMessageWait(colYellow, "No known way there")
		case label:
			old := m[cy<<4|cx]
			if s, ok := e.labelInput(old.text); ok {
				if s == "" {
					delete(m, cy<<4|cx)
				} else {
					kind := byte(poiUser)
					if s == old.text {
						kind = old.kind
					}
					m[cy<<4|cx] = mapLabel{s, kind}
				}
				e.saveNotes()
			}
		case remove:
			if _, ok := m[cy<<4|cx]; ok {
				delete(m, cy<<4|cx)
				e.saveNotes()
			}
		case exit, NoPick:
			return
		}
	}
}

// labelInput reads a label on row 24 in upper and lower case; Esc gives up.
func (e *Engine) labelInput(s string) (string, bool) {
	const prompt, max = "Label: ", 0x20
	t, ok := e.typed(s, max, func(c byte) bool { return c >= 0x20 && c < 0x7F }, func(t string) {
		e.ClearRow24()
		e.DisplayString(0, promptRow, colLightMagenta, prompt)
		e.DisplayString(len(prompt), promptRow, colWhite, t+"_")
	})
	e.ClearRow24()
	if !ok {
		return s, false
	}
	return strings.TrimSpace(t), true
}

// drawFullMap draws the map, the cursor at (cx, cy) and the legend; it
// returns the legend's texts and the first square of each.
func (e *Engine) drawFullMap(m map[int]mapLabel, cx, cy int) ([]string, map[string]int) {
	e.ClearScreen()
	e.DisplayString(1, 0, colWhite, "Map")
	e.DisplayString(fmLegend, 0, colWhite, "Labels")
	texts, letter, kind := legend(m)
	first := map[string]int{}
	for s := 0; s < 256; s++ {
		if l, ok := m[s]; ok {
			if _, ok := first[l.text]; !ok {
				first[l.text] = s
			}
		}
	}
	for i, t := range texts {
		if i >= fmRows {
			break
		}
		e.DisplayString(fmLegend, 2+i, int(poiColour[kind[t]]), string(letter[t]))
		e.DisplayString(fmLegend+2, 2+i, colLightGreen, cut(t, 0x28-fmLegend-2))
	}
	info := fmt.Sprintf("%d,%d", cx, cy)
	if l, ok := m[cy<<4|cx]; ok {
		info += " " + l.text
	}
	e.DisplayString(1, messageRow, colLightCyan, cut(info, 0x26))
	e.mapPage(fmX, fmY, fmCell, m, letter)
	px, py := fmX+cx*fmCell, fmY+cy*fmCell
	e.fill(px, py, px+fmCell, py+1, 14)
	e.fill(px, py+fmCell-1, px+fmCell, py+fmCell, 14)
	e.fill(px, py, px+1, py+fmCell, 14)
	e.fill(px+fmCell-1, py, px+fmCell, py+fmCell, 14)
	return texts, first
}

// doorColour is a light brown (Amiga palette 0xA84): it stands out
// against the white walls and the grey, black and blue floors.
const doorColour = pal.AmigaBase + 16*pal.AmigaPortrait + 6

// canvas is what drawMap draws on.
type canvas interface {
	Fill(x0, y0, x1, y1 int, c uint8)
}

// drawMap is the one map renderer: the current map's 16x16 squares of
// cell pixels at (ox, oy) on dst, seen squares grey, walked ones blue,
// walls, doors, labelled squares (their legend letter when letter is set
// and the cell fits it, else a dot) and the party.
func (e *Engine) drawMap(dst canvas, ox, oy, cell int, labels map[int]mapLabel, letter map[string]byte) {
	seen := e.ext.seen[e.areaKey()]
	if seen == nil {
		seen = &[256]uint8{}
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			s := seen[y<<4|x]
			px, py := ox+x*cell, oy+y*cell
			switch s {
			case seenSeen:
				dst.Fill(px, py, px+cell, py+cell, 8) // dark grey: seen
			case seenVisited:
				dst.Fill(px, py, px+cell, py+cell, 1) // blue: walked
			}
			e.mapWalls(dst, x, y, px, py, cell, s)
			l, ok := labels[y<<4|x]
			switch {
			case !ok || x == e.posX && y == e.posY:
			case letter != nil && cell >= 10:
				g, _ := e.glyph(letter[l.text], true)
				for r := 0; r < 8; r++ {
					for b := 0; b < 8; b++ {
						if g[r]&(0x80>>b) != 0 {
							dst.Fill(px+1+b, py+1+r, px+2+b, py+2+r, poiColour[l.kind])
						}
					}
				}
			default:
				a, b := cell*2/7, cell*5/7
				dst.Fill(px+a, py+a, px+b, py+b, poiColour[l.kind])
			}
		}
	}
	e.mapParty(dst, ox, oy, cell, max(2, cell/5)) // a dot with a tick towards the facing
}

// pixels is an off-screen picture in screen colour indices, for drawMap.
type pixels struct {
	w, h int
	pix  []uint8
}

func (p *pixels) Fill(x0, y0, x1, y1 int, c uint8) {
	for y := max(y0, 0); y < min(y1, p.h); y++ {
		for x := max(x0, 0); x < min(x1, p.w); x++ {
			p.pix[y*p.w+x] = c
		}
	}
}

// mapPage draws the map on the page at ox, oy (drawMap's arguments).
func (e *Engine) mapPage(ox, oy, cell int, labels map[int]mapLabel, letter map[string]byte) {
	n := 16 * cell
	p := &pixels{n, n, make([]uint8, n*n)}
	e.drawMap(p, 0, 0, cell, labels, letter)
	e.picture(ox, oy, n, n, p.pix, -1)
}

// mapImage is drawMap off screen, cell pixels a square, in the screen's
// colours (nil where there is no map).
func (e *Engine) mapImage(cell int) image.Image {
	if !e.minimapAllowed() {
		return nil
	}
	n := 16 * cell
	p := &pixels{n, n, make([]uint8, n*n)}
	m := e.labels()
	_, letter, _ := legend(m)
	e.drawMap(p, 0, 0, cell, m, letter)
	out := image.NewRGBA(image.Rect(0, 0, n, n))
	for i, c := range p.pix {
		out.SetRGBA(i%n, i/n, e.pal.RGBA(c))
	}
	return out
}

// mapWalls draws the walls of square (x, y) as lines at the edges of a
// size-pixel cell at (px, py): doors and locked doors in their colours,
// walls of squares not seen yet dim (s: the square's seen state).
func (e *Engine) mapWalls(dst canvas, x, y, px, py, size int, s uint8) {
	for _, d := range [4]int{0, 2, 4, 6} {
		if e.WallAt(x, y, d) == 0 {
			continue
		}
		c := uint8(15)
		switch e.DoorAt(x, y, d) {
		case 1:
			c = doorColour
		case 2, 3:
			c = 14
		}
		if s == 0 {
			c = 8 // not seen yet: dim, doors not told apart (as Area)
		}
		switch d {
		case 0:
			dst.Fill(px, py, px+size, py+1, c)
		case 2:
			dst.Fill(px+size-1, py, px+size, py+size, c)
		case 4:
			dst.Fill(px, py+size-1, px+size, py+size, c)
		case 6:
			dst.Fill(px, py, px+1, py+size, c)
		}
	}
}

// mapParty draws the party on a map at (ox, oy): a dot of 2r-1 pixels with
// a tick towards the facing.
func (e *Engine) mapParty(dst canvas, ox, oy, size, r int) {
	if !onMap(e.posX, e.posY) {
		return
	}
	cx, cy := ox+e.posX*size+size/2, oy+e.posY*size+size/2
	dst.Fill(cx-r+1, cy-r+1, cx+r, cy+r, 12)
	for i := r + 1; i <= r+2; i++ {
		tx, ty := cx+i*dirDX[e.viewFacing()], cy+i*dirDY[e.viewFacing()]
		dst.Fill(tx, ty, tx+1, ty+1, 14)
	}
}
