package remaster

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"

	"gpu"
	"gpu/vector"

	"blobber/font"
	"blobber/ui"

	"por/engine"
)

// The inventory screen (F1-F8, Items): a book in Dragon Age's layout, the
// member's Eye of the Beholder paper doll on the left page and the party
// inventory on the right. It draws engine.View.Inventory in both modes and
// only sends keys: KeyItem picks a line, the menu's own keys act on it.

var (
	//go:embed doll/doll.png
	dollPNG []byte
	//go:embed doll/arrow_prev.png
	prevPNG []byte
	//go:embed doll/arrow_next.png
	nextPNG []byte
	//go:embed items/atlas.png
	atlasPNG []byte
	//go:embed items/items.rec
	itemsRec string
)

// Canvas and layout, in canvas pixels (shown at a whole scale).
const (
	invW, invH   = 480, 420
	dollX, dollY = 20, 100 // the doll, drawn twice its size
	rowX, rowY   = 254, 56 // the first party inventory line
	rowW, rowH   = 204, 18
	rows         = 14
	atlasCell    = 32
	atlasCols    = 24
)

var (
	ink      = color.RGBA{0x15, 0x20, 0x2f, 255}
	faded    = color.RGBA{0x6a, 0x60, 0x58, 255}
	selFill  = color.RGBA{0x40, 0x6c, 0x91, 120}
	gold     = color.RGBA{0xdc, 0xc4, 0x7c, 255}
	barFill  = color.RGBA{0x8a, 0x2a, 0x1c, 255}
	barEmpty = color.RGBA{0x3a, 0x30, 0x28, 255}
)

// dollBoxes are the doll's slot boxes (EOB's frames, 18x18 with the frame;
// the rings' 10x10) in doll pixels, by engine doll slot.
var dollBoxes = [engine.DollSlots]image.Rectangle{
	engine.DollWeapon:    image.Rect(13, 63, 31, 81),
	engine.DollShield:    image.Rect(61, 63, 79, 81),
	engine.DollArmor:     image.Rect(7, 23, 25, 41),
	engine.DollGauntlets: image.Rect(8, 43, 26, 61),
	engine.DollHelm:      image.Rect(81, 2, 99, 20),
	engine.DollBelt:      image.Rect(83, 41, 101, 59),
	engine.DollRobe:      image.Rect(83, 59, 101, 77),
	engine.DollCloak:     image.Rect(70, 22, 88, 40),
	engine.DollBoots:     image.Rect(60, 84, 78, 102),
	engine.DollRing1:     image.Rect(11, 83, 21, 93),
	engine.DollRing2:     image.Rect(23, 83, 33, 93),
	engine.DollArrows:    image.Rect(8, 3, 26, 21),
	engine.DollBolts:     image.Rect(83, 77, 101, 95),
	engine.DollBracers:   image.Rect(4, 95, 22, 113),
}

var slotNames = [engine.DollSlots]string{"Weapon", "Shield", "Armour", "Gauntlets", "Helm", "Belt", "Robe", "Cloak",
	"Boots", "Ring", "Ring", "Arrows", "Bolts", "Bracers"}

// inventory is the screen's state between frames.
type inventory struct {
	canvas, doll, prev, next, atlas *gpu.Image
	icons                           map[string]int // items.rec: icon key → atlas cell
	faces                           map[image.Image]*gpu.Image

	shown   bool        // the last frame was the inventory
	k       float64     // canvas scale
	at      image.Point // canvas top left on screen
	top     int         // first party inventory line shown
	lines   []hit       // clickable item rectangles (canvas)
	buttons []button    // the action buttons (canvas)
	arrows  [2]image.Rectangle
	hover   image.Point // the pointer (canvas)
	member  int         // View.Inventory.Member
	count   int         // party members
	sel     int         // View.Inventory.Sel
	seen    int         // the Sel the list last scrolled to
}

type hit struct {
	r     image.Rectangle
	index int
}

type button struct {
	r   image.Rectangle
	key byte
}

func decodePNG(b []byte) *gpu.Image {
	im, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return gpu.NewImageFromImage(im)
}

// readIcons parses items.rec: records of "field: value" lines between
// blank lines, comments starting with #.
func readIcons(rec string) map[string]int {
	m := map[string]int{}
	name := ""
	s := bufio.NewScanner(strings.NewReader(rec))
	for s.Scan() {
		k, v, ok := strings.Cut(s.Text(), ":")
		v = strings.TrimSpace(v)
		switch {
		case !ok || strings.HasPrefix(k, "#"):
		case k == "internal_name":
			name = v
		case k == "icon":
			if n, err := strconv.Atoi(v); err == nil && name != "" {
				m[name] = n
			}
		}
	}
	return m
}

func (inv *inventory) load() {
	if inv.canvas != nil {
		return
	}
	inv.canvas = gpu.NewImage(invW, invH)
	inv.doll, inv.prev, inv.next, inv.atlas = decodePNG(dollPNG), decodePNG(prevPNG), decodePNG(nextPNG), decodePNG(atlasPNG)
	inv.icons = readIcons(itemsRec)
	inv.faces = map[image.Image]*gpu.Image{}
}

// icon is the atlas cell of an item: the key "TT:word" for one of its
// name words (or a word of one), else "TT" for its type; -1 for none.
func (inv *inventory) icon(it *engine.InvItem) int {
	t := fmt.Sprintf("%02X", it.Type)
	for _, w := range it.Words {
		if n, ok := inv.icons[t+":"+w]; ok {
			return n
		}
		for _, f := range strings.Fields(w) {
			if n, ok := inv.icons[t+":"+f]; ok {
				return n
			}
		}
	}
	if n, ok := inv.icons[t]; ok {
		return n
	}
	return -1
}

// drawIcon draws the item's icon in square r (32 or 16 pixels: whole or
// half size, the aspect kept).
func (inv *inventory) drawIcon(it *engine.InvItem, r image.Rectangle) {
	n := inv.icon(it)
	if n < 0 {
		font.Draw(inv.canvas, "?", r.Min.X+r.Dx()/2-4, r.Min.Y+r.Dy()/2-8, ink)
		return
	}
	at := image.Pt(n%atlasCols*atlasCell, n/atlasCols*atlasCell)
	src := image.Rectangle{at, at.Add(image.Pt(atlasCell, atlasCell))}
	op := &gpu.DrawImageOptions{}
	k := float64(r.Dx()) / atlasCell
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
	inv.canvas.DrawImage(inv.atlas.SubImage(src).(*gpu.Image), op)
}

func (inv *inventory) blit(src *gpu.Image, x, y int, k float64) {
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(x), float64(y))
	inv.canvas.DrawImage(src, op)
}

// fit cuts s to w pixels of the font.
func fit(s string, w int) string {
	for s != "" && font.Width(s) > w {
		s = s[:len(s)-1]
	}
	return s
}

// slotRect is a doll slot's inside on the canvas (its frame's 1 px off).
func slotRect(s int) image.Rectangle {
	b := dollBoxes[s].Inset(1)
	return image.Rect(dollX+2*b.Min.X, dollY+2*b.Min.Y, dollX+2*b.Max.X, dollY+2*b.Max.Y)
}

// Inventory draws the inventory screen when the game shows it, and
// reports whether it did. Prompts it asks over the list (Drop it? Look)
// are left to the caller's own drawing.
func (p *Presenter) Inventory(dst *gpu.Image, v engine.View) bool {
	inv := &p.inv
	inv.shown = false
	in := v.Inventory
	if in == nil || v.Menu == nil || !hasKey(v.Menu, 'E') {
		return false
	}
	inv.load()
	inv.shown, inv.member, inv.count, inv.sel = true, in.Member, len(v.Party), in.Sel
	c := inv.canvas
	c.Fill(color.Black)
	ui.Frame(c, image.Rect(0, 0, invW, invH))
	ui.Parchment(c, image.Rect(14, 14, 238, invH-14))
	ui.Parchment(c, image.Rect(242, 14, invW-14, invH-14))
	for _, t := range []struct {
		s  string
		cx int
	}{{"Equipment", 126}, {"Party Inventory", 354}} {
		ui.Title(c, "", t.cx, 18)
		font.Draw(c, t.s, t.cx-font.Width(t.s)/2, 28, ink)
	}
	inv.lines = inv.lines[:0]

	// the member: arrows, portrait and name
	inv.arrows = [2]image.Rectangle{image.Rect(22, 60, 62, 90), image.Rect(190, 60, 230, 90)}
	inv.blit(inv.prev, 22, 60, 2)
	inv.blit(inv.next, 190, 60, 2)
	if in.Member >= 0 && in.Member < len(v.Party) {
		m := v.Party[in.Member]
		if f := p.face(m.Head); f != nil {
			g, ok := inv.faces[m.Head]
			if !ok {
				g = gpu.NewImageFromImage(f)
				inv.faces[m.Head] = g
			}
			inv.blit(g, 68, 52, 1)
		}
		font.Draw(c, fit(m.Name, 72), 116, 58, ink)
		font.Draw(c, "HP "+strconv.Itoa(m.HP)+"/"+strconv.Itoa(m.MaxHP), 116, 76, faded)
	}

	// the doll and the readied items
	inv.blit(inv.doll, dollX, dollY, 2)
	for s, it := range in.Doll {
		r := slotRect(s)
		if it == nil {
			continue
		}
		inv.drawIcon(it, r)
		inv.lines = append(inv.lines, hit{r, it.Index})
		if it.Index == in.Sel {
			vector.StrokeRect(c, float32(r.Min.X-1), float32(r.Min.Y-1), float32(r.Dx()+2), float32(r.Dy()+2), 2, gold, false)
		}
	}

	// the party inventory
	n := len(in.Pool)
	for i, it := range in.Pool { // keep a newly highlighted line in sight (the wheel scrolls freely)
		if it.Index == in.Sel && in.Sel != inv.seen {
			inv.top = min(inv.top, i)
			inv.top = max(inv.top, i-rows+1)
		}
	}
	inv.seen = in.Sel
	inv.top = max(0, min(inv.top, n-rows))
	for i := inv.top; i < n && i < inv.top+rows; i++ {
		it := &in.Pool[i]
		y := rowY + (i-inv.top)*rowH
		r := image.Rect(rowX, y, rowX+rowW, y+rowH)
		if it.Index == in.Sel {
			vector.DrawFilledRect(c, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), selFill, false)
		}
		inv.drawIcon(it, image.Rect(rowX+1, y+1, rowX+17, y+17))
		font.Draw(c, fit(it.Name, rowW-22), rowX+20, y+1, ink)
		inv.lines = append(inv.lines, hit{r, it.Index})
	}
	if n > rows { // where the lines shown are in the list
		h := rows * rowH
		vector.DrawFilledRect(c, rowX+rowW+2, rowY, 3, float32(h), barEmpty, false)
		vector.DrawFilledRect(c, rowX+rowW+2, float32(rowY+h*inv.top/n), 3, float32(h*rows/n), gold, false)
	}
	if n == 0 {
		font.Draw(c, "Nothing", rowX+20, rowY+1, faded)
	}

	// capacity
	y := rowY + rows*rowH + 6
	font.Draw(c, "Capacity", rowX, y, ink)
	s := strconv.Itoa(n) + " / " + strconv.Itoa(in.Cap)
	font.Draw(c, s, rowX+rowW-font.Width(s), y, ink)
	vector.DrawFilledRect(c, rowX, float32(y+16), rowW, 6, barEmpty, false)
	vector.DrawFilledRect(c, rowX, float32(y+16), float32(rowW*min(n, in.Cap)/max(1, in.Cap)), 6, barFill, false)

	// what the pointer is on, the actions of the highlighted item
	info := ""
	for s := range in.Doll {
		if inv.hover.In(slotRect(s)) {
			info = slotNames[s]
			if it := in.Doll[s]; it != nil {
				info = it.Name
			}
		}
	}
	for _, l := range inv.lines {
		if inv.hover.In(l.r) && l.r.Min.X >= rowX {
			info = ""
		}
	}
	font.Draw(c, fit(info, 200), 24, dollY+234, ink)
	inv.buttons = inv.buttons[:0]
	equipped := false
	for _, it := range in.Doll {
		if it != nil && it.Index == in.Sel {
			equipped = true
		}
	}
	x := 24
	for _, m := range v.Menu.Items {
		label := m.Text
		switch m.Key {
		case 'N', 'P': // the list scrolls here itself
			continue
		case 'R':
			label = "Equip"
			if equipped {
				label = "Remove"
			}
		}
		w := font.Width(label) + 14
		st := ui.Normal
		r := image.Rect(x, invH-44, x+w, invH-25)
		if inv.hover.In(r) {
			st = ui.Hover
		}
		ui.Button(c, x, r.Min.Y, w, label, st)
		inv.buttons = append(inv.buttons, button{r, m.Key})
		x += w + 2
	}

	// onto the screen at the largest whole scale
	b := dst.Bounds()
	inv.k = float64(max(1, min(b.Dx()/invW, b.Dy()/invH)))
	inv.at = image.Pt((b.Dx()-int(inv.k)*invW)/2, (b.Dy()-int(inv.k)*invH)/2)
	dst.Fill(color.Black)
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(inv.k, inv.k)
	op.GeoM.Translate(float64(inv.at.X), float64(inv.at.Y))
	dst.DrawImage(c, op)
	return true
}

func hasKey(m *engine.Menu, k byte) bool {
	for _, it := range m.Items {
		if it.Key == k {
			return true
		}
	}
	return false
}

// toCanvas is screen pixel (x, y) on the inventory's canvas.
func (inv *inventory) toCanvas(x, y int) image.Point {
	if inv.k == 0 {
		return image.Point{-1, -1}
	}
	return image.Pt(int(float64(x-inv.at.X)/inv.k), int(float64(y-inv.at.Y)/inv.k))
}

// InventoryShown: the last frame was the inventory screen (its mouse
// input goes to InventoryMouse, not the game's text cells).
func (p *Presenter) InventoryShown() bool { return p.inv.shown }

// InventoryMouse is the keys for the pointer at screen pixel (x, y): a
// click picks an item (again: equips or removes it), an action button or
// an arrow (the previous or next member: F1-F8); the wheel scrolls.
func (p *Presenter) InventoryMouse(x, y int, click bool, wheel int) []uint16 {
	inv := &p.inv
	inv.hover = inv.toCanvas(x, y)
	inv.top = max(0, inv.top-wheel)
	if !click {
		return nil
	}
	for _, l := range inv.lines {
		if inv.hover.In(l.r) {
			if l.index == inv.sel {
				return []uint16{'R'}
			}
			return []uint16{engine.KeyItem + uint16(l.index)}
		}
	}
	for _, b := range inv.buttons {
		if inv.hover.In(b.r) {
			return []uint16{uint16(b.key)}
		}
	}
	for i, r := range inv.arrows {
		if inv.hover.In(r) && inv.count > 1 {
			n := (inv.member + inv.count + 2*i - 1) % inv.count
			return []uint16{engine.KeyF1 + uint16(n)<<8}
		}
	}
	return nil
}
