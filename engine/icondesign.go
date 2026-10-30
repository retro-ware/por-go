package engine

import "image"

// The combat icon designer (extra), in place of the original's icon
// editor states (ovr17:382D: Parts/color-1/color-2/Size, then Next/Prev
// stepping): one page with both poses large and live, Head and Weapon
// opening grids of all the parts, Size toggling, and the six parts' two
// colours each as swatches that a palette of the 16 colours sets. Keep
// takes it, Exit (Esc) puts the old icon back.

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

// iconEditor is the designer, on the selected character (the creation,
// the camp's Alter).
func (e *Engine) iconEditor() {
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
	show := func() {
		build()
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
				swatch(idSwatchCol+3*s, idPartRow+i, c.IconColour[i]>>(4*s)&0xF, frame)
			}
		}
		for i := 0; i < 16; i++ {
			swatch(idPaletteCol+2*(i%8), idPaletteRow+i/8, byte(i), colDarkGrey)
		}
	}
	e.snap.iconDrop = func(im image.Image) {
		if im == nil || !c.setIconPNG(im) {
			e.say("Not a PNG, square or twice as wide as high")
			return
		}
		show()
	}
	defer func() { e.snap.iconDrop, e.pg.overlays = nil, nil }()
	const (
		headIt Choice = iota
		weaponIt
		sizeIt
		keep
		exit
	)
	// pickPart: a grid of every head (or weapon) on the icon as it is.
	pickPart := func(pic *byte, name string, n int) {
		was := *pic
		if i := e.pickGrid(name+": ", n, 12, 3, 3, int(was), func(i, x, y int) {
			*pic = byte(i)
			build()
			if im := e.icons[0xC][0]; im != nil {
				e.picture(x*8, y*8, im.Width*8, im.Height, im.Frame(0), im.Mask)
			}
		}); i >= 0 {
			*pic = byte(i)
		} else {
			*pic = was
		}
		e.DrawBoxTitled(1, 1, winRight, winBottom, 0, colBlack, "")
	}
	for {
		show()
		mark := e.regionMark()
		hot := func(x0, y0, x1, y1 int, h byte) {
			e.addRegion(region{x0: x0, y0: y0, x1: x1, y1: y1, click: func(int, int) Action { return Action{Kind: actHotkey, N: int(h)} }})
		}
		hot(idWordsCol, idPreviewRow, idWordsCol+5, idPreviewRow, 'H')
		hot(idWordsCol, idPreviewRow+1, idWordsCol+5, idPreviewRow+1, 'W')
		hot(idWordsCol, idPreviewRow+2, idWordsCol+9, idPreviewRow+2, 'S')
		for p := 0; p < 2; p++ { // the poses: the head on their upper part, the weapon below
			x := idPreviewCols + 11*p
			hot(x, idPreviewRow, x+8, idPreviewRow+3, 'H')
			hot(x, idPreviewRow+4, x+8, idPreviewRow+8, 'W')
		}
		e.addRegion(region{x0: idSwatchCol, y0: idPartRow, x1: idSwatchCol + 4, y1: idPartRow + 5, click: func(cx, cy int) Action {
			part, shade = cy-idPartRow, (cx-idSwatchCol)/3
			return Action{Kind: actRedraw}
		}})
		e.addRegion(region{x0: idPaletteCol, y0: idPaletteRow, x1: idPaletteCol + 15, y1: idPaletteRow + 1, click: func(cx, cy int) Action {
			v := byte((cy-idPaletteRow)*8 + (cx-idPaletteCol)/2)
			c.IconColour[part] = c.IconColour[part]&^(0xF<<(4*shade)) | v<<(4*shade)
			return Action{Kind: actRedraw}
		}})
		p := e.DisplayInput("", Bar{{"Head", headIt}, {"Weapon", weaponIt}, {"Size", sizeIt}, {"Keep", keep}, {"Exit", exit}}, colLightMagenta, colLightGreen, colWhite, true, false)
		e.regionCut(mark)
		switch p.ID {
		case headIt:
			pickPart(&c.IconHead, "Head", 14)
		case weaponIt:
			pickPart(&c.IconBody, "Weapon", 32)
		case sizeIt:
			if c.IconSize == 2 {
				c.IconSize = 1
			} else {
				c.IconSize = 2
			}
		case exit, NoPick:
			c.IconHead, c.IconBody, c.IconSize, c.Icon = head, weapon, size, icon
			copy(c.IconColour[:], colours[:])
		case keep:
			e.Music(3) // Amiga 0x2F33E
		}
		if p.ID == keep || p.ID == exit || p.ID == NoPick {
			break
		}
	}
	build()
	e.iconCopy(0xC, order, false)
	e.ClearRow24()
	e.freeIcon(0xC) // ovr33:014E
	if e.viewMode > 1 {
		e.LoadIconPic("ICON", 0, 0xB)
		e.LoadIconPic("ICON", 1, 0xC)
	}
	e.combatViewEnter() // ovr32:0105
}
