package remaster

import (
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"

	"gpu"
	"gpu/vector"

	"por/phlan-poc/blobber"

	"por/engine"
)

// The party's portrait cards: each member's face in the game's window
// border (in colour for the selected member: whose turn, whose screen; grey for the others), the hit
// point bar under it, the experience bar (the way to the next level, gold) under that while Ctrl is held, and the sheet's status over the face.

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
		p.faces = map[image.Image]image.Image{}
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
		key := cardKey{m.Selected, m.HP, m.MaxHP, m.Status, p.portrait(m), m.Portrait != nil, 0, 0, p.border.tiles[0], m.Memo}
		if m.NextExp > 0 && (p.expShown || gpu.IsKeyPressed(gpu.KeyControl)) {
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
	m := v.Party[i]
	lines, sel := []string{m.Name, m.Class}, -1
	if m.NextExp > 0 { // the experience and what the next level takes
		lines = append(lines, "Exp "+strconv.Itoa(m.Exp)+" / "+strconv.Itoa(m.NextExp))
	} else {
		lines = append(lines, "Exp "+strconv.Itoa(m.Exp))
	}
	var keys []byte
	if p.ctx > 0 {
		lines, p.ctxActs, keys = powerLines(m.Powers)
		sel, p.ctxKeys = p.ctxSel, keys
		if len(lines) == 0 { // nothing to do (a fight began)
			p.ctx = 0
			return
		}
	}
	rs, box := p.cardMenu(dst, i, lines, keys, sel)
	if p.ctx > 0 {
		p.ctxRects, p.ctxBox = append(p.ctxRects[:0], rs...), box
	}
}

// powerLines are a member's powers as a context menu's lines, decisions
// and hotkeys: each line's first letter not taken by a line above, else
// its next free one.
func powerLines(ps []engine.Power) (lines []string, acts []engine.ActKind, keys []byte) {
	for _, pw := range ps {
		var key byte
		for _, c := range []byte(strings.ToUpper(pw.Name)) {
			if c >= 'A' && c <= 'Z' && !slices.Contains(keys, c) {
				key = c
				break
			}
		}
		lines, acts, keys = append(lines, pw.Name), append(acts, pw.Kind), append(keys, key)
	}
	return lines, acts, keys
}

// cardMenu draws lines (with hotkeys keys, line sel highlighted) in a
// box beside member i's card, on the side away from the edge; it returns
// the lines' rectangles and the box's top edge.
func (p *Presenter) cardMenu(dst *gpu.Image, i int, lines []string, keys []byte, sel int) ([]image.Rectangle, image.Rectangle) {
	r, k := p.cards[i], cardScale(dst)
	w := 0
	for _, l := range lines {
		w = max(w, int(k*8*float64(len(l)+4))) // the box: a border and a blank column each side
	}
	gap := int(cardGap * k)
	cx := r.Max.X + gap + w/2
	if r.Min.X > dst.Bounds().Dx()/2 { // the right column: to the left of it
		cx = r.Min.X - gap - w/2
	}
	top := r.Min.Y
	if blobber.Settings.PartyRow { // the row: above the card, beside it
		top = max(0, r.Max.Y-int(k*8*float64(len(lines)+2)))
	}
	return p.tipMenu(dst, "", lines, keys, sel, k, cx, top), image.Rect(cx-w/2, top, cx+w/2, top)
}

// tipMenu draws a menu box that is not the game's menu (a tooltip, a
// context menu): on its own image, the menu's click rectangles kept; it
// returns the box's own line rectangles.
func (p *Presenter) tipMenu(dst *gpu.Image, title string, lines []string, keys []byte, sel int, k float64, cx, top int) []image.Rectangle {
	rects := slices.Clone(p.menuRects)                                                  // not a menu: the question's clicks stay
	p.menuImg, p.tipImg, p.menuBuf, p.tipBuf = p.tipImg, p.menuImg, p.tipBuf, p.menuBuf // its own image: the menu's keeps its size
	p.drawMenu(dst, p.glyph, title, lines, keys, nil, sel, k, cx, top)
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
func (p *Presenter) keyWindow(screen *gpu.Image, title string, lines []string, keys []byte, off []bool, sel int, bottom bool) float64 {
	if yesNo(keys) { // every Yes/No question: centred over the dimmed view
		b := screen.Bounds()
		vector.DrawFilledRect(screen, 0, 0, float32(b.Dx()), float32(b.Dy()), color.NRGBA{A: 0xA0})
		bottom = false
	}
	w := 0
	for _, l := range titleLines(title) {
		w = max(w, visLen(l))
	}
	for _, l := range lines {
		w = max(w, len(l))
	}
	b := screen.Bounds()
	k0 := float64(max(1, min(int(scale(screen)), b.Dx()/((w+4)*8))))
	rows := len(lines) + 2
	if title != "" {
		rows += len(titleLines(title)) + 1
	}
	k := fitScale(k0, 1, func(k float64) bool { // K, as far as the box fits
		return k*float64((w+4)*8) <= float64(b.Dx()) && k*float64((rows+1)*8) <= float64(b.Dy())
	})
	h := int(float64(rows*8) * k)
	top := (b.Dy() - h) / 2
	if bottom {
		top = b.Dy() - h - int(8*k)
	}
	rects := slices.Clone(p.menuRects) // not the game's menu: its clicks stay
	defer func() { p.menuRects = rects }()
	p.drawMenu(screen, p.glyph, title, lines, keys, off, sel, k, b.Dx()/2, top)
	p.winRects = slices.Clone(p.menuRects) // its lines, for the Esc menu's mouse
	return k
}

// yesNo: a menu's rows are a Yes/No question's.
func yesNo(keys []byte) bool { return slices.Equal(keys, yesNoKeys) }

// cardKey is what a card's picture (cardImgs) shows; drawParty redraws a card
// when it changes.
type cardKey struct {
	selected     bool
	hp, maxHP    int
	status       string
	face         image.Image // the face drawn on the card (nil: a PNG portrait goes over it at the window's resolution)
	png          bool
	exp, nextExp int // the experience bar while Ctrl is held (0: no bar)
	tiles        *gpu.Image
	memo         string // camp: the spells ready over the slots, on the bottom border
}

// drawCard draws m's card into d (scale 1): border, face, status, bars.
func (p *Presenter) drawCard(d *gpu.Image, m engine.Member, key cardKey) {
	d.Clear()
	t := p.border.tiles
	if !m.Selected {
		t = p.border.grey
	}
	window(d, t, gpu.GeoM{}, cardW/8, frameH/8, color.Black)
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
	if m.Memo != "" { // on the portrait's bottom border, centred
		onBorder(d, p.glyph, (cardW-8*(len(m.Memo)+2))/2, frameH-8, true, []string{m.Memo}, []color.RGBA{green})
	}
	fill := greenFill
	if m.HP*4 < m.MaxHP {
		fill = redFill
	}
	p.bar(d, -5, barY, cardW+10, m.HP, m.MaxHP, fill) // the caps' art is inset 5 px: this lines it up with the border
	if key.nextExp > 0 {                              // Ctrl: the way to the next level, full once the experience is there
		p.bar(d, -5, expY, cardW+10, key.exp, key.nextExp, goldFill)
	}
}

// cardAt lays the party's cards along the window's edges: the first half
// of the party down the left, the rest down the right, at cardScale.
func cardAt(dst *gpu.Image) func(i, n int) image.Rectangle {
	b := dst.Bounds()
	k := cardScale(dst)
	px := func(n int) int { return int(math.Round(float64(n) * k)) }
	return func(i, n int) image.Rectangle {
		if blobber.Settings.PartyRow { // one row along the bottom, centred
			step := px(cardW + cardGap)
			x := (b.Dx()-n*step+px(cardGap))/2 + i*step
			y := b.Dy() - px(cardH+cardGap) + px(cardFoot)
			return image.Rect(x, y, x+px(cardW), y+px(cardH))
		}
		left := (n + 1) / 2
		col, row, per := 0, i, left
		if i >= left {
			col, row, per = 1, i-left, n-left
		}
		y := b.Dy() - px((per-row)*(cardH+cardGap)) + px(cardFoot) // the bars end a gap above the bottom
		x := px(cardGap)
		if col == 1 {
			x = b.Dx() - px(cardGap) - px(cardW)
		}
		return image.Rect(x, y, x+px(cardW), y+px(cardH))
	}
}

const cardGap, cardRows = 8, 4

// cardBase is the cards' size 1 (Esc, Graphics, Portraits): the largest
// whole scale where a column of four fits under the smallest map.
func cardBase(dst *gpu.Image) int {
	return max(1, min(int(scale(dst)), (dst.Bounds().Dy()-mapSide-2-cardGap)/(cardRows*(cardH+cardGap)))) // K, as far as a column of cards fits
}

// cardScale is the cards' scale: cardBase times the Portraits size, as far
// as a column of four (eight members with NPCs) still fits under the map at
// its size and a card takes at most a sixth of the width (fitScale).
func cardScale(dst *gpu.Image) float64 {
	b := dst.Bounds()
	room := float64(b.Dy() - 2*cardGap*cardBase(dst))
	if !blobber.Settings.MapOff {
		room -= float64(mapPx(dst))
	}
	if blobber.Settings.PartyRow { // eight across the bottom, under the map, at most a third of the height
		return fitScale(float64(cardBase(dst)), blobber.Settings.Cards, func(k float64) bool {
			return k*8*(cardW+cardGap)+k*cardGap <= float64(b.Dx()) && k*(cardH+cardGap) <= min(room, float64(b.Dy())/3)
		})
	}
	return fitScale(float64(cardBase(dst)), blobber.Settings.Cards, func(k float64) bool {
		return k*cardRows*(cardH+cardGap) <= room && k*cardW <= float64(b.Dx())/6
	})
}

// cardLabels: the sheet's status as the card spells it over the portrait,
// short enough to fit there (a member at 0 hit points is not always dead).
var cardLabels = map[string]string{
	"Animated":    "undead",
	"Running":     "fled",
	"Unconscious": "KO",
	"Dying":       "dying",
	"Dead":        "dead",
	"Poisoned":    "poison", // dead of poison, as the sheet puts it
	"Stoned":      "stone",
	"Gone":        "gone",
}

// portrait is m's card art: the PNG the player dropped on the sheet, fitted
// into the PortraitSize square, on its bottom edge (cached), else the paper-doll head's face.
func (p *Presenter) portrait(m engine.Member) image.Image {
	if m.Portrait == nil {
		return p.face(m.Head)
	}
	if f, ok := p.faces[m.Portrait]; ok {
		return f
	}
	f := fitSquare(m.Portrait, portraitSize)
	p.faces[m.Portrait] = f
	return f
}

// hiFace is a PNG portrait at n pixels square for a card at the window's
// resolution (cached per size): more detail than the card's own face.
func (p *Presenter) hiFace(src image.Image, n int) *gpu.Image {
	if g, ok := p.hiFaces[src]; ok && g.Bounds().Dx() == n {
		return g
	}
	g := gpu.NewImageFromImage(fitSquare(src, n))
	if old, ok := p.hiFaces[src]; ok {
		old.Deallocate()
	}
	p.hiFaces[src] = g
	return g
}

// fitSquare is src fitted into an n pixel square, its shape kept, on the
// square's foot: by whole pixels when those nearly fill it (crisp pixel
// art), else by Catmull-Rom.
func fitSquare(src image.Image, n int) *image.RGBA {
	b := src.Bounds()
	f := image.NewRGBA(image.Rect(0, 0, n, n))
	at := fit(b, f.Rect)
	if s := wholeScale(b.Dx(), b.Dy(), n, n); s > 0 {
		w, h := s*b.Dx(), s*b.Dy()
		at = image.Rect((n-w)/2, n-h, (n-w)/2+w, n)
	}
	xdraw.NearestNeighbor.Scale(f, at, src, b, draw.Src, nil) // crisp pixels, no filtering, at any size
	return f
}

// wholeScale is the whole number of pixels per source pixel that scales a
// sw by sh picture up to nearly fill tw by th (at least 90%), 0 when none
// does (a smaller picture is drawn smoothly to the exact size instead).
func wholeScale(sw, sh, tw, th int) int {
	s := min(tw/max(sw, 1), th/max(sh, 1))
	if s >= 1 && s*sw >= tw*9/10 && s*sh >= th*9/10 {
		return s
	}
	return 0
}

// face is the card art for a head: its lit pixels (the face, hair and
// shoulders) centred in a PortraitSize square, unscaled (nil without a head).
func (p *Presenter) face(head image.Image) image.Image {
	if head == nil {
		return nil
	}
	if f, ok := p.faces[head]; ok {
		return f
	}
	const n = portraitSize
	lit := image.Rectangle{}
	b := head.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, g, bl, a := head.At(x, y).RGBA(); a > 0 && r|g|bl != 0 {
				lit = lit.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	f := image.NewRGBA(image.Rect(0, 0, n, n))
	draw.Draw(f, f.Bounds(), image.Black, image.Point{}, draw.Src)
	c := lit.Min.Add(lit.Max).Div(2) // centre on centre; a part wider than the square is cut evenly
	draw.Draw(f, f.Bounds(), head, c.Sub(image.Pt(n/2, n/2)), draw.Over)
	p.faces[head] = f
	return f
}
