package engine

import (
	"image"
	"slices"

	"por/pal"
)

// The combat icon designer (extra), in place of the original's icon
// editor states (ovr17:382D: Parts/color-1/color-2/Size, then Next/Prev
// stepping): one page with both poses large and live, Head and Weapon
// opening grids of all the parts, Size toggling, and the six parts' two
// colours each as swatches that a palette sets: the 16 EGA colours the DOS
// editor stepped through, or on Amiga art the 15 terrain palette slots the
// Amiga's stepped through (1..15, in its order). It takes decisions: the
// page's parts click as ActPicker (PickIconHead, PickWeapon), ActIconSize
// and ActIconColour; ActKeep takes it, ActClose puts the old icon back. The
// bar is the presenter's.

// Layout, in cells.
const (
	idPreviewRow  = 2  // the poses, 9 cells square
	idPreviewCols = 2  // ready; action 11 to the right
	idWordsCol    = 24 // Head, Weapon, Size
	idPartRow     = 13 // the six parts, a row each
	idPartCol     = 2
	idSwatchCol   = 12 // colour 1; colour 2 three to the right
	idPaletteRow  = 20 // the 16 colours, 8 a row, 2 cells each
	idPaletteCol  = 2
)

// iconParts are the parts in IconColour order, with their colours' names
// where they are not shades (the hair and the face share a part).
var iconParts = [6]string{"Body", "Arm", "Leg", "Hair/Face", "Shield", "Weapon"}

// pushIcon puts the designer up on the selected character (the creation,
// camp's Icon).
func (e *Engine) pushIcon() {
	c := e.selected()
	e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colBlack, "")
	e.combatViewLeave() // ovr32:00E3: the combat palette, black shown dark grey so the icon's black shows
	head, weapon, size, icon := c.IconHead, c.IconBody, c.IconSize, c.Icon
	var colours [6]byte
	copy(colours[:], c.IconColour[:])
	part, shade := 0, 0 // the swatch selected: IconColour[part], its low (0) or high (1) nibble
	order := c.Order
	build := func() { // the working copy, recoloured, in slot 0xC
		e.BuildCharIcon(c, false) // ovr18:0482
		e.iconCopy(order, 0xC, true)
	}
	swatch := func(x, y int, colour byte, frame int) { // 2 cells wide, framed
		e.fill(x*8, y*8, x*8+16, y*8+8, byte(frame))
		e.fill(x*8+1, y*8+1, x*8+15, y*8+7, colour)
	}
	// The swatches show what a click gives: on Amiga art that is the
	// terrain palette's slot the part's pixels take for the colour number
	// (amigaIconColour: another hue and shade than the EGA colour of that
	// number, magenta is black there), not the EGA colour. The DOS palette's
	// columns put the darker of each pair on top; the Amiga's is its 15
	// slots in slot order, as its Next stepped them (slot 0 is left out).
	amiga := false
	look := func(i byte) byte {
		if amiga {
			return pal.AmigaBase + 16*pal.AmigaTerrain + amigaIconColour[i]
		}
		return i
	}
	const noCell = 0xFF
	var cells [16]byte // the colour number in each palette cell, row-major (noCell: empty)
	layout := func() {
		im := e.icons[0xC][0]
		amiga = im != nil && slices.ContainsFunc(im.Data, func(v byte) bool { return v >= pal.AmigaBase })
		if amiga {
			for n, s := range amigaIconColour {
				if s > 0 {
					cells[s-1] = byte(n)
				}
			}
			cells[15] = noCell
			return
		}
		lum := func(i byte) int {
			c := pal.Identity.RGBA(i)
			return int(c.R) + int(c.G) + int(c.B)
		}
		for k := 0; k < 8; k++ {
			a, b := byte(k), byte(k+8)
			if lum(a) > lum(b) {
				a, b = b, a
			}
			cells[k], cells[k+8] = a, b
		}
	}
	var show func()
	show = func() {
		build()
		layout()
		e.ClearCells(1, 1, winRight, winBottom)
		var o []Overlay
		for p := 0; p < 2; p++ {
			x := idPreviewCols + 11*p
			at := image.Rect(x*8, idPreviewRow*8, x*8+72, idPreviewRow*8+72)
			if c.Icon != nil {
				o = append(o, Overlay{Img: iconPose(c.Icon, p, false), At: at})
			} else if im := e.icons[0xC][p]; im != nil {
				o = append(o, Overlay{Img: e.layerRGBA(layer{d: im.Frame(0), w: im.Width * 8, mask: im.Mask}), At: at})
			}
		}
		e.pg.overlays = o
		e.DisplayString(idPreviewCols+2, idPreviewRow-1, colWhite, "ready")
		e.DisplayString(idPreviewCols+11+2, idPreviewRow-1, colWhite, "action")
		e.DisplayString(idWordsCol, idPreviewRow, colLightGreen, "Head")
		e.DisplayString(idWordsCol, idPreviewRow+1, colLightGreen, "Weapon")
		sizeName := "small"
		if c.IconSize == 2 {
			sizeName = "large"
		}
		e.DisplayString(idWordsCol, idPreviewRow+2, colLightGreen, "Size "+sizeName)
		e.DisplayString(idPartCol, idPartRow-1, colWhite, "Colours")
		e.DisplayString(idSwatchCol, idPartRow-1, colWhite, "1  2")
		for i, name := range iconParts {
			e.DisplayString(idPartCol, idPartRow+i, colLightGreen, name)
			for s := 0; s < 2; s++ {
				frame := colDarkGrey
				if i == part && s == shade {
					frame = colWhite
				}
				swatch(idSwatchCol+3*s, idPartRow+i, look(c.IconColour[i]>>(4*s)&0xF), frame)
			}
		}
		for i, v := range cells {
			if v != noCell {
				swatch(idPaletteCol+2*(i%8), idPaletteRow+i/8, look(v), colDarkGrey)
			}
		}
		e.snap.iconDrop = func(im image.Image) { // the whole screen takes the PNG (a new page ends it: set with each)
			if e.takeIconPNG(c, im) {
				show()
			}
		}
	}
	// pickPart: a grid of every head (or weapon) on the icon as it is.
	pickPart := func(pic *byte, n int) {
		was := *pic
		e.pushPicker(n, 3, 3, int(was), func(i, x, y int) {
			*pic = byte(i)
			build()
			if im := e.icons[0xC][0]; im != nil {
				e.picture(x*8, y*8, im.Width*8, im.Height, im.Frame(0), im.Mask, im, 0)
			}
			*pic = was
		}, func(i int) bool { *pic = byte(i); return true })
	}
	mark := e.regionMark()
	draw := func() {
		e.regionCut(mark)
		e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colBlack, "")
		show()
		hot := func(x0, y0, x1, y1 int, a Action) {
			e.addRegion(region{x0: x0, y0: y0, x1: x1, y1: y1, click: func(int, int) Action { return a }})
		}
		head, weapon := Action{Kind: ActPicker, N: PickIconHead}, Action{Kind: ActPicker, N: PickWeapon}
		hot(idWordsCol, idPreviewRow, idWordsCol+5, idPreviewRow, head)
		hot(idWordsCol, idPreviewRow+1, idWordsCol+5, idPreviewRow+1, weapon)
		hot(idWordsCol, idPreviewRow+2, idWordsCol+9, idPreviewRow+2, Action{Kind: ActIconSize})
		for p := 0; p < 2; p++ { // the poses: the head on their upper part, the weapon below
			x := idPreviewCols + 11*p
			hot(x, idPreviewRow, x+8, idPreviewRow+3, head)
			hot(x, idPreviewRow+4, x+8, idPreviewRow+8, weapon)
		}
		e.addRegion(region{x0: idSwatchCol, y0: idPartRow, x1: idSwatchCol + 4, y1: idPartRow + 5, click: func(cx, cy int) Action {
			return Action{Kind: ActIconColour, N: cy - idPartRow, X: (cx - idSwatchCol) / 3, Y: -1}
		}})
		e.addRegion(region{x0: idPaletteCol, y0: idPaletteRow, x1: idPaletteCol + 15, y1: idPaletteRow + 1, click: func(cx, cy int) Action {
			v := cells[(cy-idPaletteRow)*8+(cx-idPaletteCol)/2]
			if v == noCell {
				return Action{}
			}
			return Action{Kind: ActIconColour, N: part, X: shade, Y: int(v)}
		}})
	}
	off := e.redrawWith(draw)
	e.push(modeIcon, func(a Action) bool {
		switch a.Kind {
		case ActPicker:
			switch a.N {
			case PickIconHead:
				pickPart(&c.IconHead, 14)
			case PickWeapon:
				pickPart(&c.IconBody, 32)
			}
			return false
		case ActIconSize:
			if c.IconSize == 2 {
				c.IconSize = 1
			} else {
				c.IconSize = 2
			}
		case ActIconColour:
			if a.N < 0 || a.N > 5 || a.X < 0 || a.X > 1 {
				return false
			}
			part, shade = a.N, a.X // ponytail: the swatch the palette sets is the engine's (the page shows it framed)
			if a.Y >= 0 && a.Y < 16 {
				c.IconColour[part] = c.IconColour[part]&^(0xF<<(4*shade)) | byte(a.Y)<<(4*shade)
			}
		case ActClose:
			c.IconHead, c.IconBody, c.IconSize, c.Icon = head, weapon, size, icon
			copy(c.IconColour[:], colours[:])
			return true
		case ActKeep:
			e.Music(3) // Amiga 0x2F33E
			return true
		case actRedraw:
		default:
			return false
		}
		draw()
		return false
	}, func() {
		off()
		e.regionCut(mark)
		e.snap.iconDrop, e.pg.overlays = nil, nil
		build()
		e.iconCopy(0xC, order, false)
		e.ClearRow24()
		e.freeIcon(0xC) // ovr33:014E
		if e.viewMode > 1 {
			e.LoadIconPic("ICON", 0, 0xB)
			e.LoadIconPic("ICON", 1, 0xC)
		}
		e.combatViewEnter() // ovr32:0105
	})
	draw()
}
