package remaster

import (
	_ "embed"
	"image"
	"image/color"
	"slices"
	"strconv"

	"gpu"

	"blobber"

	"por/engine"
)

// The party's portrait cards: each member's face in the game's window
// border (in colour for the selected member: whose turn, whose screen; grey for the others), the hit
// point bar under it, the experience bar (the way to the next level, gold) under that while Shift is held, and the sheet's status over the face.

// Card layout at scale 1.
const (
	cardW        = 64         // the border round a 6 cell inside
	portraitSize = cardW - 16 // a card's face fills the inside of the border, a paper-doll head's lit part
	frameH       = cardW      // the border holds only the face
	barY         = frameH + 1 // the hit point bar
	expY         = barY + 9   // the experience bar, a pixel under the hit point bar (the groove art is rows 1..8 of 11)
	cardH        = expY + 9
	cardFoot     = 0 // no strip under the bars
	cardFace     = (cardW - portraitSize) / 2
)

//go:embed bars.png
var barsPNG []byte

// The bar art (bars.png): the groove's caps and middle, then the fills.
var (
	barLeft, barMid, barRight    = image.Rect(0, 0, 10, 11), image.Rect(10, 0, 22, 11), image.Rect(22, 0, 32, 11)
	greenFill, redFill, goldFill = image.Rect(0, 11, 82, 15), image.Rect(0, 15, 82, 19), image.Rect(0, 19, 82, 23) // gold: the experience bar
)

// borders are the window border's tiles (View.Border) on the GPU, in
// colour and in grey.
type borders struct {
	src         [3]image.Image
	tiles, grey [3]*gpu.Image
}

// update uploads the game's tiles when they change.
func (b *borders) update(t [3]image.Image) {
	if t == b.src || t[0] == nil {
		return
	}
	b.src = t
	for i, m := range t {
		b.tiles[i] = gpu.NewImageFromImage(m)
		g := image.NewRGBA(m.Bounds())
		for y := m.Bounds().Min.Y; y < m.Bounds().Max.Y; y++ {
			for x := m.Bounds().Min.X; x < m.Bounds().Max.X; x++ {
				c := color.RGBAModel.Convert(m.At(x, y)).(color.RGBA)
				l := uint8((299*int(c.R) + 587*int(c.G) + 114*int(c.B)) / 1000)
				g.SetRGBA(x, y, color.RGBA{l, l, l, c.A})
			}
		}
		b.grey[i] = gpu.NewImageFromImage(g)
	}
}

// setup takes this frame's font and border tiles from the game.
func (p *Presenter) setup(note Note, v engine.View) {
	p.glyph, blobber.Glyph = note.Glyph, note.Glyph
	p.border.update(v.Border)
	if p.faceImgs == nil {
		p.faceImgs, p.hiFaces, p.labels = map[image.Image]*gpu.Image{}, map[image.Image]*gpu.Image{}, map[string]*gpu.Image{}
	}
}

// drawParty draws the members' cards where at puts them.
func (p *Presenter) drawParty(dst *gpu.Image, ms []engine.Member, at func(i, n int) image.Rectangle) {
	if p.border.tiles[0] == nil {
		return
	}
	if p.bars == nil {
		p.bars = gpu.NewImageFromImage(decodePNG(barsPNG))
	}
	for len(p.cardImgs) < len(ms) {
		p.cardImgs = append(p.cardImgs, gpu.NewImage(cardW, cardH))
		p.cardKeys = append(p.cardKeys, cardKey{})
	}
	for i, m := range ms {
		d := p.cardImgs[i]
		key := cardKey{m.Selected, m.HP, m.MaxHP, m.Status, p.portrait(m), m.Portrait != nil, 0, 0, p.border.tiles[0]}
		if m.NextExp > 0 && gpu.IsKeyPressed(gpu.KeyShift) {
			key.exp, key.nextExp = m.Exp-m.LevelExp, m.NextExp-m.LevelExp
		}
		if key != p.cardKeys[i] { // the card's picture changed: redraw it (a card is a render target of its own, a pass each)
			p.cardKeys[i] = key
			p.drawCard(d, m, key)
		}
		r := at(i, len(ms))
		op := &gpu.DrawImageOptions{}
		k := float64(r.Dx()) / cardW
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
		dst.DrawImage(d, op)
		if m.Portrait != nil { // the PNG at the window's resolution (the card's 48 pixels lose its detail), the status over it
			op := &gpu.DrawImageOptions{}
			op.GeoM.Translate(float64(r.Min.X)+cardFace*k, float64(r.Min.Y)+cardFace*k)
			if m.HP <= 0 {
				op.ColorScale.Scale(0.35, 0.35, 0.35, 1)
			}
			dst.DrawImage(p.hiFace(m.Portrait, max(1, int(portraitSize*k))), op)
			if s := cardLabels[m.Status]; s != "" {
				p.label(dst, s, r.Min.X+int((cardFace+1)*k), r.Min.Y+int((cardFace+portraitSize-9)*k), k)
			}
		}
	}
}

// drawCardTip names the member whose card the pointer is on (cardHover):
// name and class in a box beside the card, on the side away from the edge.
// After a right click (ctx) the box is the member's context menu instead:
// the lines of Member.Powers, the pointer's highlighted (Click acts).
func (p *Presenter) drawCardTip(dst *gpu.Image, v engine.View) {
	i := p.cardHover - 1
	if p.ctx > 0 {
		i = p.ctx - 1
	}
	if i < 0 || i >= len(p.cards) || i >= len(v.Party) {
		return
	}
	m, r, k := v.Party[i], p.cards[i], cardScale(dst)
	lines, sel := []string{m.Name, m.Class}, -1
	if m.NextExp > 0 { // the experience and what the next level takes
		lines = append(lines, "Exp "+strconv.Itoa(m.Exp)+" / "+strconv.Itoa(m.NextExp))
	} else {
		lines = append(lines, "Exp "+strconv.Itoa(m.Exp))
	}
	var keys []byte
	if p.ctx > 0 {
		lines, sel, p.ctxActs, p.ctxKeys = nil, p.ctxSel, p.ctxActs[:0], p.ctxKeys[:0]
		for _, pw := range m.Powers {
			lines, p.ctxActs, p.ctxKeys = append(lines, pw.Name), append(p.ctxActs, pw.Kind), append(p.ctxKeys, pw.Name[0]) // the first letter: the hotkey (all differ)
		}
		keys = p.ctxKeys
		if len(lines) == 0 { // nothing to do (a fight began)
			p.ctx = 0
			return
		}
	}
	w := 0
	for _, l := range lines {
		w = max(w, int(k*8*float64(len(l)+4))) // the box: a border and a blank column each side
	}
	gap := int(cardGap * k)
	cx := r.Max.X + gap + w/2
	if r.Min.X > dst.Bounds().Dx()/2 { // the right column: to the left of it
		cx = r.Min.X - gap - w/2
	}
	rs := p.tipMenu(dst, "", lines, keys, sel, k, cx, r.Min.Y)
	if p.ctx > 0 {
		p.ctxRects, p.ctxBox = append(p.ctxRects[:0], rs...), image.Rect(cx-w/2, r.Min.Y, cx+w/2, r.Min.Y)
	}
}

// tipMenu draws a menu box that is not the game's menu (a tooltip, a
// context menu): on its own image, the menu's click rectangles kept; it
// returns the box's own line rectangles.
func (p *Presenter) tipMenu(dst *gpu.Image, title string, lines []string, keys []byte, sel int, k float64, cx, top int) []image.Rectangle {
	rects := slices.Clone(p.menuRects)                                                  // not a menu: the question's clicks stay
	p.menuImg, p.tipImg, p.menuBuf, p.tipBuf = p.tipImg, p.menuImg, p.tipBuf, p.menuBuf // its own image: the menu's keeps its size
	p.drawMenu(dst, p.glyph, title, lines, keys, sel, k, cx, top)
	p.menuImg, p.tipImg, p.menuBuf, p.tipBuf = p.tipImg, p.menuImg, p.tipBuf, p.menuBuf
	rs := slices.Clone(p.menuRects)
	p.menuRects = rects
	return rs
}

// bar draws a bar w wide: the groove, filled v of maxV with the fill's art.
func (p *Presenter) bar(dst *gpu.Image, x, y, w, v, maxV int, fill image.Rectangle) {
	blit := func(src, d image.Rectangle) {
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(float64(d.Dx())/float64(src.Dx()), float64(d.Dy())/float64(src.Dy()))
		op.GeoM.Translate(float64(d.Min.X), float64(d.Min.Y))
		dst.DrawImage(p.bars.SubImage(src).(*gpu.Image), op)
	}
	h := barLeft.Dy()
	blit(barLeft, image.Rect(x, y, x+10, y+h))
	blit(barRight, image.Rect(x+w-10, y, x+w, y+h))
	blit(barMid, image.Rect(x+10, y, x+w-10, y+h))
	if maxV > 0 && v > 0 {
		fw := max(1, (w-14)*min(v, maxV)/maxV)
		blit(fill, image.Rect(x+7, y+3, x+7+fw, y+3+fill.Dy()))
	}
}

// label writes s in the game's font, white on black, at (x, y), k times
// its size.
func (p *Presenter) label(dst *gpu.Image, s string, x, y int, k float64) {
	img, ok := p.labels[s]
	if !ok {
		m := image.NewRGBA(image.Rect(0, 0, 8*len(s)+2, 10))
		for i := range m.Pix {
			if i%4 == 3 {
				m.Pix[i] = 255
			}
		}
		drawText(m, p.glyph, s, 1, 1, white)
		img = gpu.NewImageFromImage(m)
		p.labels[s] = img
	}
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(img, op)
}

// keyWindow is Blobber's text window (its Esc menu, the options, prompts) as
// the game's own menu box, the lines' hotkeys (keys[i] in lines[i]) in white.
func (p *Presenter) keyWindow(screen *gpu.Image, title string, lines []string, keys []byte, sel int, bottom bool) {
	w := len(title)
	for _, l := range lines {
		w = max(w, len(l))
	}
	b := screen.Bounds()
	k0 := float64(max(1, min(int(scale(screen)), b.Dx()/((w+4)*8))))
	rows := len(lines) + 2
	if title != "" {
		rows += 2
	}
	k := k0 * fitStep(blobber.Settings.Menu, func(s float64) bool { // the Menus size (Esc, Graphics), as far as the box fits
		return k0*s*float64((w+4)*8) <= float64(b.Dx()) && k0*s*float64((rows+1)*8) <= float64(b.Dy())
	})
	h := int(float64(rows*8) * k)
	top := (b.Dy() - h) / 2
	if bottom {
		top = b.Dy() - h - int(8*k)
	}
	rects := slices.Clone(p.menuRects) // not the game's menu: its clicks stay
	defer func() { p.menuRects = rects }()
	p.drawMenu(screen, p.glyph, title, lines, keys, sel, k, b.Dx()/2, top)
	p.winRects = slices.Clone(p.menuRects) // its lines, for the Esc menu's mouse
}

// cardKey is what a card's picture (cardImgs) shows; drawParty redraws a card
// when it changes.
type cardKey struct {
	selected     bool
	hp, maxHP    int
	status       string
	face         image.Image // the face drawn on the card (nil: a PNG portrait goes over it at the window's resolution)
	png          bool
	exp, nextExp int // the experience bar while Shift is held (0: no bar)
	tiles        *gpu.Image
}

// drawCard draws m's card into d (scale 1): border, face, status, bars.
func (p *Presenter) drawCard(d *gpu.Image, m engine.Member, key cardKey) {
	d.Clear()
	d.SubImage(image.Rect(8, 8, cardW-8, frameH-8)).(*gpu.Image).Fill(color.Black)
	t := p.border.tiles
	if !m.Selected {
		t = p.border.grey
	}
	drawBorder(d, t, gpu.GeoM{}, cardW/8, frameH/8)
	if f := p.portrait(m); f != nil && m.Portrait == nil { // a PNG is drawn at the window's resolution below
		g, ok := p.faceImgs[f]
		if !ok {
			g = gpu.NewImageFromImage(f)
			p.faceImgs[f] = g
		}
		op := &gpu.DrawImageOptions{}
		op.GeoM.Translate(cardFace, cardFace)
		if m.HP <= 0 {
			op.ColorScale.Scale(0.35, 0.35, 0.35, 1)
		}
		d.DrawImage(g, op)
	}
	if s := cardLabels[m.Status]; s != "" && m.Portrait == nil {
		p.label(d, s, cardFace+1, cardFace+portraitSize-9, 1)
	}
	fill := greenFill
	if m.HP*4 < m.MaxHP {
		fill = redFill
	}
	p.bar(d, -5, barY, cardW+10, m.HP, m.MaxHP, fill) // the caps' art is inset 5 px: this lines it up with the border
	if key.nextExp > 0 {                              // Shift: the way to the next level, full once the experience is there
		p.bar(d, -5, expY, cardW+10, key.exp, key.nextExp, goldFill)
	}
}
