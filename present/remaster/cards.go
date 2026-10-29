package remaster

import (
	_ "embed"
	"image"
	"image/color"
	"slices"

	"gpu"

	"blobber"

	"por/engine"
)

// The party's portrait cards: each member's face in the game's window
// border (in colour for the selected member: whose turn, whose screen; grey for the others), the hit
// point bar under it, and the sheet's status over the face.

// Card layout at scale 1.
const (
	cardW        = 64          // the border round a 6 cell inside
	portraitSize = cardW - 16  // a card's face fills the inside of the border, a paper-doll head's lit part
	frameH       = cardW       // the border holds only the face
	barY         = frameH + 1  // the hit point bar
	cardH        = barY + 2*11 // and room for a second bar's row
	cardFoot     = 11          // the empty strip under the bar
	cardFace     = (cardW - portraitSize) / 2
)

//go:embed bars.png
var barsPNG []byte

// The bar art (bars.png): the groove's caps and middle, then the fills.
var (
	barLeft, barMid, barRight = image.Rect(0, 0, 10, 11), image.Rect(10, 0, 22, 11), image.Rect(22, 0, 32, 11)
	greenFill, redFill        = image.Rect(0, 11, 82, 15), image.Rect(0, 15, 82, 19)
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
		p.faceImgs, p.labels = map[image.Image]*gpu.Image{}, map[string]*gpu.Image{}
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
	}
	for i, m := range ms {
		d := p.cardImgs[i]
		d.Clear()
		d.SubImage(image.Rect(8, 8, cardW-8, frameH-8)).(*gpu.Image).Fill(color.Black)
		t := p.border.tiles
		if !m.Selected {
			t = p.border.grey
		}
		drawBorder(d, t, gpu.GeoM{}, cardW/8, frameH/8)
		if f := p.portrait(m); f != nil {
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
		if s := cardLabels[m.Status]; s != "" {
			p.label(d, s, cardFace+1, cardFace+portraitSize-9)
		}
		fill := greenFill
		if m.HP*4 < m.MaxHP {
			fill = redFill
		}
		p.bar(d, -5, barY, cardW+10, m.HP, m.MaxHP, fill) // the caps' art is inset 5 px: this lines it up with the border
		r := at(i, len(ms))
		op := &gpu.DrawImageOptions{}
		k := float64(r.Dx()) / cardW
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
		dst.DrawImage(d, op)
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
		w = max(w, k*8*(len(l)+4)) // the box: a border and a blank column each side
	}
	cx := r.Max.X + cardGap*k + w/2
	if r.Min.X > dst.Bounds().Dx()/2 { // the right column: to the left of it
		cx = r.Min.X - cardGap*k - w/2
	}
	rects := slices.Clone(p.menuRects)                                                  // not a menu: the question's clicks stay
	p.menuImg, p.tipImg, p.menuBuf, p.tipBuf = p.tipImg, p.menuImg, p.tipBuf, p.menuBuf // its own image: the menu's keeps its size
	p.drawMenu(dst, p.glyph, "", lines, keys, sel, float64(k), cx, r.Min.Y)
	p.menuImg, p.tipImg, p.menuBuf, p.tipBuf = p.tipImg, p.menuImg, p.tipBuf, p.menuBuf
	if p.ctx > 0 {
		p.ctxRects, p.ctxBox = append(p.ctxRects[:0], p.menuRects...), image.Rect(cx-w/2, r.Min.Y, cx+w/2, r.Min.Y)
	}
	p.menuRects = rects
}

// bar draws a hit point bar w wide: the groove, filled v of maxV.
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

// label writes s in the game's font, white on black, at (x, y).
func (p *Presenter) label(dst *gpu.Image, s string, x, y int) {
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
	k := max(1, min(int(scale(screen)), b.Dx()/((w+4)*8)))
	h := (len(lines) + 2) * 8 * k
	if title != "" {
		h += 16 * k
	}
	top := (b.Dy() - h) / 2
	if bottom {
		top = b.Dy() - h - 8*k
	}
	rects := slices.Clone(p.menuRects) // not the game's menu: its clicks stay
	defer func() { p.menuRects = rects }()
	p.drawMenu(screen, p.glyph, title, lines, keys, sel, float64(k), b.Dx()/2, top)
	p.winRects = slices.Clone(p.menuRects) // its lines, for the Esc menu's mouse
}
