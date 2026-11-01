package engine

import (
	"bufio"
	"fmt"
	"image"
	"maps"
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
// New Phlan's buildings are always labelled (phlanLabels, in every game);
// the player's labels go over them and are kept with the save (labelsFile).

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

const phlanKey = 3 << 16 // areaKey of New Phlan: data set 3, GEO block 0

// labelsFile holds a slot's map labels (the player's, by map).
const labelsFile = "LABELS.TXT"

// builtinLabels are map k's fixed labels: New Phlan's buildings.
func builtinLabels(k int) map[int]mapLabel {
	if k == phlanKey {
		return phlanLabels
	}
	return nil
}

// userLabels are the player's labels of the current map (ext.labels).
func (e *Engine) userLabels() map[int]mapLabel {
	k := e.areaKey()
	if e.ext.labels == nil {
		e.ext.labels = map[int]map[int]mapLabel{}
	}
	if e.ext.labels[k] == nil {
		e.ext.labels[k] = map[int]mapLabel{}
	}
	return e.ext.labels[k]
}

// labels are the current map's labels (square y<<4|x -> label): the
// built-in ones with the player's over them.
func (e *Engine) labels() map[int]mapLabel {
	m := map[int]mapLabel{}
	maps.Copy(m, builtinLabels(e.areaKey()))
	maps.Copy(m, e.ext.labels[e.areaKey()])
	return m
}

// readLabels reads a slot's labelsFile: "map KEY" (areaKey) starts a map,
// then "X Y KIND TEXT" per label.
func (e *Engine) readLabels(path string) {
	e.ext.labels = nil
	f, err := os.Open(path)
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
		if _, err := fmt.Sscanf(line, "map %d", &k); err == nil {
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

// writeLabels writes the player's labels to path, but those that are built in.
func (e *Engine) writeLabels(path string) {
	var b strings.Builder
	keys := make([]int, 0, len(e.ext.labels))
	for k := range e.ext.labels {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "map %d\n", k)
		m := e.ext.labels[k]
		sq := make([]int, 0, len(m))
		for s, l := range m {
			if b, ok := builtinLabels(k)[s]; !ok || b != l {
				sq = append(sq, s)
			}
		}
		sort.Ints(sq)
		for _, s := range sq {
			fmt.Fprintf(&b, "%d %d %d %s\n", s&15, s>>4, m[s].kind, m[s].text)
		}
	}
	os.WriteFile(path, []byte(b.String()), 0o644)
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

// mapScreen is the full map's state while it is up (modeMap).
type mapScreen struct {
	cursor image.Point      // View.FullMap.Cursor (ActPoint)
	labels map[int]mapLabel // the map's labels (labels)
	label  *string          // the label being typed (nil: none)
}

// FullMap puts up the full-screen map (View.FullMap) until Esc; the
// cursor's cell has a context menu (Walk, Mark, Remove).
func (e *Engine) FullMap() {
	if !e.minimapAllowed() {
		e.DisplayMessageWait(colYellow, "Not Here")
		return
	}
	ms := &mapScreen{cursor: image.Pt(e.posX, e.posY), labels: e.labels()}
	if !onMap(ms.cursor.X, ms.cursor.Y) {
		ms.cursor = image.Point{}
	}
	e.ext.fullMap = ms
	open := true
	e.push(modeMap, func(Action) bool { return !open }, func() { e.ext.fullMap = nil })
	c := &ms.cursor
	var ask func()
	ask = func() {
		const (
			walk Choice = iota
			mark
			remove
		)
		at := c.Y<<4 | c.X
		bar := Bar{{"Walk", walk}} // the cell's context menu: Walk, Mark on an unlabelled one, Remove on the player's marker
		if _, ok := ms.labels[at]; !ok {
			bar = append(bar, Entry{"Mark", mark})
		} else if _, ok := e.userLabels()[at]; ok {
			bar = append(bar, Entry{"Remove", remove})
		}
		e.displayThen("", bar, colLightMagenta, colLightGreen, colWhite, true, false, func(p Pick) {
			switch p.ID {
			case walk:
				if e.startWalk(c.X, c.Y) {
					open = false
					return
				}
				e.DisplayMessageWait(colYellow, "No known way there")
			case mark:
				e.labelInput(ms, "", func(s string, ok bool) {
					if ok && s != "" {
						e.userLabels()[at] = mapLabel{s, poiUser}
						ms.labels = e.labels()
					}
					ask()
				})
				return
			case remove:
				delete(e.userLabels(), at)
				ms.labels = e.labels()
			case NoPick:
				open = false
				return
			}
			ask()
		})
	}
	ask()
}

// labelInput reads a label (View.FullMap.Label) in upper and lower case; Esc gives
// up (false); then gets it.
func (e *Engine) labelInput(ms *mapScreen, s string, then func(string, bool)) {
	const max = 0x20
	e.typed(s, max, func(c byte) bool { return c >= 0x20 && c < 0x7F }, func(t string) {
		ms.label = &t
	}, func(t string, ok bool) {
		ms.label = nil
		if !ok {
			then(s, false)
			return
		}
		then(strings.TrimSpace(t), true)
	})
}

// mapName is the current map's name in words, from the scripts' names of
// the GEO blocks (names/pictures.txt), or "Map".
func (e *Engine) mapName() string {
	vals, _ := ReadScriptValues(filepath.Join(e.scriptDir(), "names", "pictures.txt"), nil)
	n := valueName(vals, "Map", int(e.set), int(e.area.Geo))
	if n == "" {
		return "Map"
	}
	var b strings.Builder
	for i, c := range n { // "ValjevoCastle3" -> "Valjevo Castle 3"
		if i > 0 && (c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' && !(n[i-1] >= '0' && n[i-1] <= '9')) {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// fullMapView is the full map as View.FullMap shows it: the map at
// fmCell pixels a square, the cursor, the legend's lines that fit.
func (e *Engine) fullMapView(ms *mapScreen) *FullMap {
	m := ms.labels
	texts, letter, kind := legend(m)
	first := map[string]int{}
	for s := 0; s < 256; s++ {
		if l, ok := m[s]; ok {
			if _, ok := first[l.text]; !ok {
				first[l.text] = s
			}
		}
	}
	cx, cy := ms.cursor.X, ms.cursor.Y
	info := fmt.Sprintf("%d,%d", cx, cy)
	if l, ok := m[cy<<4|cx]; ok {
		info += " " + l.text
	}
	v := &FullMap{Name: e.mapName(), Map: e.mapSquares(m), At: image.Pt(fmX, fmY), Cell: fmCell,
		Cursor: ms.cursor, LegendAt: image.Pt(fmLegend, 2), Info: info, Label: ms.label}
	for _, t := range texts[:min(len(texts), fmRows)] {
		s := first[t]
		v.Legend = append(v.Legend, MapLabel{Letter: letter[t], Text: t, Colour: int(poiColour[kind[t]]), First: image.Pt(s&15, s>>4)})
	}
	return v
}

// doorColour is a light brown (Amiga palette 0xA84): it stands out
// against the white walls and the grey, black and blue floors.
const doorColour = pal.AmigaBase + 16*pal.AmigaPortrait + 6

// mapSquares is the current map with labels m as the maps show it.
func (e *Engine) mapSquares(m map[int]mapLabel) *MapSquares {
	_, letter, _ := legend(m)
	seen := e.ext.seen[e.areaKey()]
	if seen == nil {
		seen = &[256]uint8{}
	}
	f := e.viewFacing()
	ms := &MapSquares{Party: image.Pt(e.posX, e.posY), Ahead: image.Pt(dirDX[f], dirDY[f])}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			q := &ms.Squares[y][x]
			q.Seen = Seen(seen[y<<4|x])
			for i, d := range [4]int{0, 2, 4, 6} {
				switch {
				case e.WallAt(x, y, d) == 0:
				case e.DoorAt(x, y, d) == 1:
					q.Walls[i] = Door
				case e.DoorAt(x, y, d) == 2, e.DoorAt(x, y, d) == 3:
					q.Walls[i] = LockedDoor
				default:
					q.Walls[i] = Wall
				}
			}
			if l, ok := m[y<<4|x]; ok {
				q.Ink, q.Letter = int(poiColour[l.kind]), letter[l.text]
			}
		}
	}
	return ms
}
