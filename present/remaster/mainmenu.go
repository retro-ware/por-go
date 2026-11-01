package remaster

import (
	"image"
	"image/color"
	"slices"
	"time"

	"gpu"
	"gpu/vector"

	"por/phlan-poc/blobber"

	"por/engine"
)

// The main menu (the party menu, View.MainMenu): the title's lettering on
// black, the functions under it in a box wrapped round them, the party's
// cards where the 3D view has them.

// The Amiga painted title (View.Backdrop) shows alone for titleHold when
// the main menu first comes up, then the menu fades in over it.
const titleHold, titleFade = 2 * time.Second, 1500 * time.Millisecond

// titleDim is how far the picture darkens under the menu (0 not, 1 black).
const titleDim = 0.6

// drawMain draws the main menu.
func (p *Presenter) drawMain(dst *gpu.Image, v engine.View) {
	dst.Fill(color.Black)
	b := dst.Bounds()
	if v.Backdrop == nil {
		p.drawMainUI(dst, v)
		return
	}
	bb := v.Backdrop.Bounds()
	k := min(float64(b.Dx())/float64(bb.Dx()), float64(b.Dy())/float64(bb.Dy())) // fit, keeping the aspect: the window has its shape (SetWindowAspect); the full screen gets black bars rather than a cropped or stretched picture
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate((float64(b.Dx())-float64(bb.Dx())*k)/2, (float64(b.Dy())-float64(bb.Dy())*k)/2)
	dst.DrawImage(p.backdrop.of(v.Backdrop), op)
	a := p.mainFade()
	dim := uint8(titleDim * 255 * min(max(a, 0), 1)) // the picture darkens as the menu comes, so the lettering reads
	vector.DrawFilledRect(dst, 0, 0, float32(b.Dx()), float32(b.Dy()), color.NRGBA{A: dim})
	if a >= 1 {
		p.drawMainUI(dst, v)
		return
	}
	gpu.Sized(&p.layer, b.Dx(), b.Dy())
	p.layer.Clear()
	p.drawMainUI(p.layer, v)
	lo := &gpu.DrawImageOptions{}
	lo.ColorScale.ScaleAlpha(max(a, 0))
	dst.DrawImage(p.layer, lo)
}

// mainFade is how far the menu has faded in over the title (0 not yet, 1
// fully); the first call starts the clock.
func (p *Presenter) mainFade() float32 {
	if p.mainAt.IsZero() {
		p.mainAt = time.Now()
	}
	return float32(time.Since(p.mainAt)-titleHold) / float32(titleFade)
}

// mainSkip ends the title's hold and fade on a key or click while they
// run, so the menu shows at once; it reports whether it did, and that
// input goes no further: the menu it would have acted on was not yet seen.
func (p *Presenter) mainSkip() bool {
	if p.mainFade() >= 1 {
		return false
	}
	p.mainAt = time.Now().Add(-titleHold - titleFade)
	return true
}

// mainNames are the party menu's functions as the menu names them.
var mainNames = map[engine.ActKind]string{
	engine.ActBegin: "BEGIN Adventuring", engine.ActLoad: "Load Game", engine.ActQuickStart: "Quick Start", engine.ActQuit: blobber.QuitRow,
	engine.ActAdd: "Add", engine.ActCreate: "Create", engine.ActForget: "Forget", engine.ActDropMember: "Drop",
	engine.ActModify: "Modify", engine.ActTraining: "Train", engine.ActMember: "View", engine.ActRemove: "Remove",
}

// Main menu questions (mainUI.ask).
const (
	askNone = iota
	askQuit
	askDrop
	askSure // the drop's second question
)

// mainItems are the main menu's lines (the Party submenu with the
// character functions) or, with a question up, its answers and the
// question.
func (p *Presenter) mainItems(v engine.View) (string, []menuRow) {
	ask := func(q int) func() { return func() { p.mainAsk, p.sel = q, 1 } }
	no := menuRow{"No", 'N', ask(askNone)}
	switch p.mainAsk {
	case askQuit:
		return blobber.QuitQuestion, []menuRow{{"Yes", 'Y', func() { p.mainAsk = askNone; p.send(engine.ActQuit, 0) }}, no}
	case askDrop:
		return "Drop " + selectedName(v) + " forever?", []menuRow{{"Yes", 'Y', ask(askSure)}, no}
	case askSure:
		return "Are you sure?", []menuRow{{"Yes", 'Y', func() { p.mainAsk = askNone; p.send(engine.ActDropMember, -1) }}, no}
	}
	open := func(k engine.ActKind) bool { return slices.Contains(v.MainMenu, k) }
	var it []menuRow
	add := func(k engine.ActKind) {
		if !open(k) {
			return
		}
		do := func() { p.send(k, -1) }
		switch k {
		case engine.ActQuit:
			do = ask(askQuit)
		case engine.ActDropMember:
			do = ask(askDrop)
		}
		key := mainNames[k][0]
		if k == engine.ActQuit { // Q is Quick Start's
			key = 'D'
		}
		it = append(it, menuRow{mainNames[k], key, do})
	}
	if p.mainSub {
		for _, k := range []engine.ActKind{engine.ActAdd, engine.ActCreate, engine.ActForget, engine.ActDropMember, engine.ActModify, engine.ActTraining, engine.ActMember, engine.ActRemove} {
			add(k)
		}
		return "", append(it, menuRow{"Exit", 'E', func() { p.mainSub, p.mainSel = false, p.mainBack }})
	}
	add(engine.ActBegin)
	add(engine.ActLoad) // Load Game leads when there are saved games, Quick Start without them
	if !open(engine.ActLoad) {
		add(engine.ActQuickStart)
	}
	it = append(it, menuRow{"Party", 'P', func() { p.mainBack, p.mainSub, p.mainSel = p.mainSel, true, 0 }})
	if open(engine.ActLoad) {
		add(engine.ActQuickStart)
	}
	add(engine.ActQuit)
	return "", it
}

// drawMainUI draws the main menu's lettering, functions and cards.
func (p *Presenter) drawMainUI(dst *gpu.Image, v engine.View) {
	b := dst.Bounds()
	mk := scale(dst)
	asking := p.mainAsk
	p.mainAsk = askNone
	_, items := p.mainItems(v)
	p.mainAsk = asking
	lines, keys := make([]string, len(items)), make([]byte, len(items))
	for i, it := range items {
		lines[i], keys[i] = it.text, it.key
	}
	menuH := float64(len(lines)+2) * 8 * mk
	var lh, lk int
	if v.Logo != nil {
		lb := v.Logo.Bounds()
		lk = max(1, min(b.Dx()*3/4/lb.Dx(), (b.Dy()-int(menuH))*2/3/lb.Dy()))
		lh = lb.Dy() * lk
	}
	gap := int(16 * mk)
	top := (b.Dy() - lh - gap - int(menuH)) / 2
	if v.Logo != nil {
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(float64(lk), float64(lk))
		op.GeoM.Translate(float64((b.Dx()-v.Logo.Bounds().Dx()*lk)/2), float64(top))
		dst.DrawImage(p.logo.of(v.Logo), op)
	}
	p.mainSel = min(max(p.mainSel, 0), len(lines)-1)
	p.drawMenu(dst, p.glyph, "", lines, keys, nil, p.mainSel, mk, b.Dx()/2, top+lh+gap)
	p.drawParty(dst, v.Party, cardAt(dst))
	p.cards = p.cards[:0]
	for i := range v.Party {
		p.cards = append(p.cards, cardAt(dst)(i, len(v.Party)))
	}
	p.drawCardTip(dst, v)
	if p.mainAsk != askNone { // a question pops up over it
		q, items := p.mainItems(v)
		var lines []string
		var keys []byte
		for _, it := range items {
			lines, keys = append(lines, it.text), append(keys, it.key)
		}
		p.keyWindow(dst, q, lines, keys, nil, p.sel, false)
	}
	p.ask = p.mainAsk != askNone
}

// mainPick picks line i of the main menu or its question.
func (p *Presenter) mainPick(v engine.View, i int) {
	if _, items := p.mainItems(v); i >= 0 && i < len(items) {
		if !p.ask {
			p.mainSel = i
		}
		items[i].do()
	}
}

// mainClick takes a click on the main menu: a function or a portrait.
func (p *Presenter) mainClick(x, y int) bool {
	v := p.shownView
	if p.ask {
		i := slices.IndexFunc(p.winRects, image.Pt(x, y).In)
		if i < 0 { // a click outside: back (No)
			_, items := p.mainItems(v)
			i = len(items) - 1
		}
		p.mainPick(v, i)
		return true
	}
	for i, r := range p.cards { // the party menu has no inventory: a left click selects (a right one: the sheet, Mouse)
		if image.Pt(x, y).In(r) {
			p.send(engine.ActLead, i)
			return true
		}
	}
	for i, r := range p.menuRects {
		if image.Pt(x, y).In(r) {
			p.mainPick(v, i)
			return true
		}
	}
	return false
}

// mainHover highlights the function under the pointer when it moves.
func (p *Presenter) mainHover(x, y int) {
	p.cardHover = 1 + cardUnder(p.cards, x, y)
	if image.Pt(x, y) == p.pointer {
		return
	}
	p.pointer = image.Pt(x, y)
	rects := p.menuRects
	if p.ask {
		rects = p.winRects
	}
	for i, r := range rects {
		if p.pointer.In(r) {
			if p.ask {
				p.sel = i
			} else {
				p.mainSel = i
			}
		}
	}
}

// mainKey moves the main menu's highlight (Up, Down), picks it (Enter, a
// hotkey); Esc answers No or goes Back; the member keys select a member.
func (p *Presenter) mainKey(k uint16, v engine.View) bool {
	_, items := p.mainItems(v)
	sel, n := &p.mainSel, len(items)
	if p.ask {
		sel = &p.sel
	}
	if n == 0 {
		return false
	}
	if m, ok := memberKey(k, selectedIndex(v), len(v.Party), true); ok && !p.ask { // not under a question about the member
		p.send(engine.ActLead, m)
		return true
	}
	i, back, ok := menuNav(k, sel, rowKeys(items))
	switch {
	case back && (p.ask || p.mainSub):
		p.mainPick(v, n-1) // No, Back
	case back:
		return false
	case i >= 0:
		p.mainPick(v, i)
	}
	return ok
}
