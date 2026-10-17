// Package ui has the full-screen game windows. The inventory works like a JRPG's equip screen:
// the party's shared items on the left, the selected character's slots on the right; an item is
// assigned to a slot by picking both. The party bar stays visible below; clicking a portrait switches character.
package ui

import (
	"blobber"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"

	"embed"

	"blobber/font"
	"blobber/party"
	"blobber/sprites"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Layout in canvas pixels.
const (
	rowH         = 18
	rows         = 11
	listX, listY = 24, 50 // first item row
	listW        = 220
	slotX, slotY = 266, 50 // first slot row
	slotW        = 196
	btnY, btnW   = 314, 72
)

var (
	listBox  = rc(12, 12, 244, 248)
	slotBox  = rc(254, 12, 214, 248)
	closeBox = rc(6, 6, 32, 32) // town windows
	selFill  = color.RGBA{0x40, 0x6c, 0x91, 200}
	dim      = color.RGBA{0x6a, 0x60, 0x58, 255}
)

var slotLabels = map[party.Slot]string{
	party.RightHand: "Weapon", party.LeftHand: "Off hand", party.Ranged: "Ranged", party.Helmet: "Head",
	party.Body: "Body", party.Shoes: "Feet", party.RingL: "Ring", party.RingR: "Ring",
	party.Amulet1: "Neck", party.Amulet2: "Neck",
}

var buttons = []struct {
	label string
	key   ebiten.Key
}{{"Use", ebiten.KeyU}, {"Remove", ebiten.KeyR}, {"Discard", ebiten.KeyD}, {"Exit", ebiten.KeyEscape}}

func btnBox(i int) image.Rectangle {
	x0 := (cw - len(buttons)*(btnW+4) + 4) / 2
	return rc(x0+i*(btnW+4), btnY, btnW, 19)
}

// Inventory is the equip window.
type Inventory struct {
	P *party.Party

	sel, scroll, char int
	slot              int  // selected slot, -1 for none
	inSlots           bool // keyboard focus on the slot column
	msg               string
	hoverRow, hoverBt int
	hoverSlot         int
	canvas
}

func NewInventory(p *party.Party) *Inventory {
	return &Inventory{P: p, slot: -1, hoverRow: -1, hoverBt: -1, hoverSlot: -1}
}

// Select shows character i (0-based).
func (v *Inventory) Select(i int) { v.char = max(0, min(i, len(v.P.Chars)-1)) }

func (v *Inventory) cur() *party.Character { return v.P.Chars[v.char] }

// stacks is every item nobody is wearing.
func (v *Inventory) stacks() [][]*party.Item {
	var out [][]*party.Item
	for _, st := range v.P.Inventory {
		if st[0].Wearer == nil {
			out = append(out, st)
		}
	}
	return out
}

func (v *Inventory) selected() *party.Item {
	if st := v.stacks(); v.sel < len(st) {
		return st[v.sel][0]
	}
	return nil
}

// worn is the item in the selected slot.
func (v *Inventory) worn() *party.Item {
	if v.slot < 0 {
		return nil
	}
	return v.cur().Equipped[party.Slots[v.slot]]
}

// fits reports whether it is a candidate for the selected slot (any equippable item when none is selected).
func (v *Inventory) fits(it *party.Item) bool {
	if v.slot < 0 {
		return it.Equippable()
	}
	return party.Fits(it, party.Slots[v.slot])
}

func (v *Inventory) enabled(b string) bool {
	it := v.selected()
	switch b {
	case "Use":
		return it != nil && (it.Kind == party.Consumable || it.Kind == party.Scroll)
	case "Remove":
		return v.worn() != nil
	case "Discard":
		return it != nil
	}
	return true
}

// press runs button b; false closes the window.
func (v *Inventory) press(b string) bool {
	if !v.enabled(b) {
		return true
	}
	c, it := v.cur(), v.selected()
	switch b {
	case "Exit":
		return false
	case "Use":
		m, err := v.P.Use(c, it)
		if err != nil {
			m = err.Error()
		}
		v.msg = m
	case "Remove":
		w := v.worn()
		v.P.Unequip(w)
		v.msg = fmt.Sprintf("%s removes the %s", c.Name, w.Name())
	case "Discard": // items are never put down in the world: discarding destroys them
		v.P.RemoveItem(it)
		v.msg = "Discarded the " + it.Name()
	}
	v.clampSel()
	return true
}

// pickItem puts it in its slot on the character (the selected one of a ring/neck pair if that fits);
// potions and scrolls are used.
func (v *Inventory) pickItem(it *party.Item) {
	c := v.cur()
	var err error
	switch {
	case it.Kind == party.Consumable || it.Kind == party.Scroll:
		v.press("Use")
		return
	case !it.Equippable():
		v.msg = "Nothing to do with the " + it.Name()
		return
	case v.slot >= 0 && v.fits(it):
		err = v.P.EquipAt(c, it, party.Slots[v.slot])
	default:
		err = v.P.Equip(c, it)
	}
	if err != nil {
		v.msg = err.Error()
	} else {
		v.msg = fmt.Sprintf("%s equips the %s", c.Name, it.Name())
		v.slot, v.inSlots = -1, false
	}
	v.clampSel()
}

// pickSlot selects slot i; picking the selected slot again takes its item off.
func (v *Inventory) pickSlot(i int) {
	if v.slot == i && v.worn() != nil {
		v.press("Remove")
		return
	}
	v.slot = i
}

func (v *Inventory) clampSel() {
	n := len(v.stacks())
	v.sel = max(0, min(v.sel, n-1))
	v.scroll = max(0, min(v.scroll, n-rows, v.sel))
	v.scroll = max(v.scroll, v.sel-rows+1)
}

// Update handles input; false closes the window.
func (v *Inventory) Update() bool {
	pressed := blobber.KeyJustPressed
	if pressed(ebiten.KeyI) {
		return false
	}
	if pressed(ebiten.KeyEscape) && v.slot >= 0 { // Esc first lets go of the slot
		v.slot, v.inSlots = -1, false
	} else {
		for _, b := range buttons {
			if pressed(b.key) && !v.press(b.label) {
				return false
			}
		}
	}
	step := 0
	switch {
	case blobber.KeyRepeated(ebiten.KeyUp):
		step = -1
	case blobber.KeyRepeated(ebiten.KeyDown):
		step = 1
	case pressed(ebiten.KeyLeft):
		v.inSlots = false
	case pressed(ebiten.KeyRight):
		v.inSlots = true
		v.slot = max(v.slot, 0)
	case pressed(ebiten.KeyTab) || pressed(ebiten.KeyPageDown): // next character (RB)
		v.char = (v.char + 1) % len(v.P.Chars)
	case pressed(ebiten.KeyPageUp): // previous character (LB)
		v.char = (v.char + len(v.P.Chars) - 1) % len(v.P.Chars)
	case pressed(ebiten.KeyEnter) || pressed(ebiten.KeySpace):
		if v.inSlots {
			v.inSlots = false // on to the list to choose what goes there
		} else if it := v.selected(); it != nil {
			v.pickItem(it)
		}
	}
	if v.inSlots {
		v.slot = (v.slot + step + len(party.Slots)) % len(party.Slots)
	} else {
		v.sel += step
	}
	for i := range min(6, len(v.P.Chars)) {
		if pressed(ebiten.KeyF1 + ebiten.Key(i)) {
			v.char = i
		}
	}
	v.clampSel()
	return v.mouse()
}

func slotRow(i int) image.Rectangle { return rc(slotX-4, slotY+i*rowH-1, slotW+8, rowH) }

// mouse handles hover, clicks and the wheel; false closes the window.
func (v *Inventory) mouse() bool {
	x, y := v.canvas.mouse()
	sx, sy := ebiten.CursorPosition()
	left := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	right := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if _, wy := ebiten.Wheel(); wy != 0 {
		v.scroll = max(0, min(v.scroll-int(wy), len(v.stacks())-rows))
	}
	v.hoverRow, v.hoverBt, v.hoverSlot = -1, -1, -1
	if i := PartyAt(v.P, sx, sy); i >= 0 && left {
		v.char = i
	}
	if x >= listX && x < listX+listW && y >= listY && y < listY+rows*rowH {
		if i := v.scroll + (y-listY)/rowH; i < len(v.stacks()) {
			v.hoverRow = i
			if left {
				v.sel, v.inSlots = i, false
				v.pickItem(v.stacks()[i][0])
			}
		}
	}
	for i := range party.Slots {
		if in(x, y, slotRow(i)) {
			v.hoverSlot = i
			if left {
				v.inSlots = true
				v.pickSlot(i)
			} else if right && v.cur().Equipped[party.Slots[i]] != nil {
				v.slot = i
				v.press("Remove")
			}
		}
	}
	for i, b := range buttons {
		if in(x, y, btnBox(i)) {
			v.hoverBt = i
			if left && !v.press(b.label) {
				return false
			}
		}
	}
	v.clampSel()
	return true
}

func (v *Inventory) Draw(screen *ebiten.Image) {
	d := v.begin()
	Frame(d, rc(0, 0, cw, ch))
	c := v.cur()

	// party items
	DarkFrame(d, listBox)
	Print(d, "Party items", listX, 24, gold)
	st := v.stacks()
	if len(st) == 0 {
		Print(d, "(nothing)", listX+22, listY+2, tan)
	}
	for r := 0; r < rows && v.scroll+r < len(st); r++ {
		i, it := v.scroll+r, st[v.scroll+r][0]
		y := listY + r*rowH
		if i == v.sel && !v.inSlots || i == v.hoverRow {
			fillRect(d, rc(listX-4, y-1, listW+8, rowH), selFill)
		}
		icon(d, it.Icon, listX, y, 1)
		label, col := it.Name(), color.Color(light)
		if n := len(st[i]); n > 1 {
			label += fmt.Sprintf(" x%d", n)
		}
		if !it.Equippable() && it.Kind != party.Consumable && it.Kind != party.Scroll {
			col = dim
		}
		Print(d, label, listX+22, y+2, col)
	}
	if v.scroll > 0 {
		Print(d, "^", listX+listW-8, listY-14, gold)
	}
	if v.scroll+rows < len(st) {
		Print(d, "v", listX+listW-8, listY+rows*rowH, gold)
	}

	// the character's slots
	DarkFrame(d, slotBox)
	Print(d, c.Name+" the "+strings.Title(c.Class), slotX, 24, gold)
	for i, s := range party.Slots {
		r := slotRow(i)
		if i == v.slot || i == v.hoverSlot {
			fillRect(d, r, selFill)
		}
		y := slotY + i*rowH
		Print(d, slotLabels[s], slotX, y+2, tan)
		if it := c.Equipped[s]; it != nil {
			icon(d, it.Icon, slotX+64, y, 1)
			Print(d, it.Name(), slotX+86, y+2, light)
		} else {
			Print(d, "-", slotX+86, y+2, dim)
		}
	}

	// what the cursor points at, the last message, money
	var info *party.Item
	switch {
	case v.hoverRow >= 0 && v.hoverRow < len(st): // the click may just have taken the row away
		info = st[v.hoverRow][0]
	case v.hoverSlot >= 0:
		info = c.Equipped[party.Slots[v.hoverSlot]]
	case v.inSlots:
		info = v.worn()
	default:
		info = v.selected()
	}
	if info != nil {
		Print(d, info.Info(), 22, 266, gold)
	} else if v.slot >= 0 {
		Print(d, "Pick an item for the "+strings.ToLower(slotLabels[party.Slots[v.slot]])+" slot", 22, 266, tan)
	}
	Print(d, v.msg, 22, 284, light)
	m := fmt.Sprintf("AC %d   Gold %d  Gems %d", c.AC, v.P.Gold, v.P.Gems)
	Print(d, m, cw-22-font.Width(m), 284, tan)

	for i, b := range buttons {
		s := Normal
		if !v.enabled(b.label) {
			s = Off
		} else if i == v.hoverBt {
			s = Hover
		}
		r := btnBox(i)
		Button(d, r.Min.X, r.Min.Y, r.Dx(), b.label, s)
	}
	v.present(screen)
}

// Item icons: Kyrise's 16x16 RPG Icon Pack (CC BY 4.0), one file per Icon name.
//
//go:embed assets/items/*.png
var itemFS embed.FS

var itemIcons = map[string]*ebiten.Image{}

// itemIcon returns the icon for an item's Icon name, or nil.
func itemIcon(name string) *ebiten.Image {
	if img, ok := itemIcons[name]; ok {
		return img
	}
	var img *ebiten.Image
	if b, err := itemFS.ReadFile("assets/items/" + name + ".png"); err == nil {
		if m, err := png.Decode(bytes.NewReader(b)); err == nil {
			img = ebiten.NewImageFromImage(m)
		}
	}
	itemIcons[name] = img
	return img
}

// icon draws an item's 16x16 icon, or else the 16x16 sprite of that name.
func icon(dst *ebiten.Image, name string, x, y int, scale float64) {
	img := sprites.Get(name)
	if it := itemIcon(name); it != nil {
		img = it
	}
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(img, op)
}

func fillRect(dst *ebiten.Image, r image.Rectangle, c color.Color) {
	vector.DrawFilledRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), c, false)
}
