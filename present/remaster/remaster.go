// Package remaster shows the game with the Blobber renderer: the 3D view
// fills the window, and the original's windows are laid over it. It only
// reads the game, through engine.View and the classic frame, so switching
// to and from it never touches the game's state.
package remaster

import (
	"image"
	"image/draw"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"blobber"
	"blobber/maps"
	"blobber/party"
	"blobber/ui"

	"por/ega"
	"por/engine"
	"por/present/classic"
)

// Presenter draws the remastered screen. Screens it has no remaster for
// (no remastered map, combat, sheets, shops, the title) it leaves to
// Classic, which it shares with the classic mode so its frame is current.
type Presenter struct {
	Classic *classic.Presenter

	view   *blobber.View
	failed bool                        // the renderer could not start: classic from then on
	scenes map[string]*blobber.Scene   // parsed maps by name
	shown  string                      // the scene the view shows
	last   time.Time                   // when the animation last advanced
	ticks  float64                     // animation time not yet run, in 1/60 s ticks
	party  party.Party                 // View.Party as Blobber's cards show it
	faces  map[image.Image]image.Image // card art cropped from each paper-doll head
	inside bool                        // the view has the inside ceiling
	active bool                        // the last frame was drawn remastered
	menu   bool                        // the vertical menu is open
	sel    int                         // its highlighted item
	asked  string                      // the question menu last opened by itself (menuID)
}

// Draw shows the game on dst and returns where the classic picture went
// (zero when it is not on screen: the mouse then points at nothing).
func (p *Presenter) Draw(dst *ebiten.Image, scr *ega.Screen, note classic.Note, v engine.View) classic.Rect {
	sc := p.scene(v)
	if sc == nil || !v.MainFrame || (v.Window != engine.WinThreeD && v.Window != engine.WinPicture) {
		p.shown, p.active, p.menu = "", false, false // back from elsewhere: no tween from where the view was
		return p.Classic.Draw(dst, scr, note)
	}
	p.active = true
	frame := p.Classic.Frame(scr, note)
	cam := blobber.Camera{X: v.X, Y: v.Y, Dir: int(v.Facing)}
	if name := areaMaps[v.Area]; name != p.shown {
		p.view.Show(sc, cam)
		p.shown, p.inside = name, false
	} else {
		p.view.Follow(cam)
	}
	if v.Inside != p.inside { // indoors the original shows the inside sky: here, a ceiling
		p.inside = v.Inside
		if p.inside {
			blobber.SetFlats(sc.Floor, "")
		} else {
			blobber.SetFlats(sc.Floor, sc.Ceiling)
		}
	}
	blobber.Night = !v.Day
	p.animate()
	ui.CardAt = cardAt(dst)
	p.view.Draw(dst)
	p.drawParty(dst, v.Party)
	k := scale(dst)
	b := dst.Bounds()
	if n := len(v.Party); n > 0 { // pop-ups go between the card columns
		b.Min.X, b.Max.X = cardAt(dst)(0, n).Max.X, cardAt(dst)(n-1, n).Min.X
	}
	// the picture at the top, the text in the middle, an answer's menu right under it
	pic, txt := engine.InnerWindow.Framed(), engine.TextPane.Framed()
	ph, th := float64(pic.Pixels().Dy()), float64(txt.Pixels().Dy())
	pk := min(k, max(1, float64(min(int(float64(b.Dy())/2/(ph+th/2)), b.Dx()/ega.W))))
	cx, cy := (b.Min.X+b.Max.X)/2, b.Dy()/2
	if v.Window == engine.WinPicture {
		drawBox(dst, frame, pic, pk, cx, int(float64(cy)-pk*th/2)/2)
	}
	if v.TextShown {
		drawBox(dst, frame, txt, pk, cx, cy)
	}
	if v.Menu != nil && !v.Command { // a question: its choices pop up at once
		if id := menuID(v.Menu); id != p.asked {
			p.asked, p.menu, p.sel = id, true, v.Menu.Sel
		}
	} else {
		p.asked = ""
	}
	if v.Menu == nil { // typed input and "press a key" prompts: the original's line
		p.menu = false
		p.drawPrompt(dst, frame, k)
	} else if p.menu {
		lines := make([]string, len(v.Menu.Items))
		for i, it := range v.Menu.Items {
			lines[i] = it.Text
		}
		p.sel = min(p.sel, len(lines)-1)
		if v.Command {
			ui.Window(dst, v.Menu.Prompt, lines, p.sel, false)
		} else {
			ui.WindowAt(dst, v.Menu.Prompt, lines, p.sel, cx, cy+int(pk*th/2)+8)
		}
	}
	return classic.Rect{}
}

// Key takes key k before the game when the remastered view is up: Enter
// opens the vertical menu in place of the menu bar, Esc on the command bar
// opens the renderer's options. It returns the keys to send instead and
// whether k was taken.
func (p *Presenter) Key(k uint16, v engine.View) ([]uint16, bool) {
	if !p.active || p.view == nil {
		p.menu = false
		return nil, false
	}
	if p.view.OptionsOpen() {
		return nil, true
	}
	m := v.Menu
	if !p.menu {
		switch {
		case k == 13 && m != nil:
			p.menu, p.sel = true, m.Sel
		case k == 27 && v.Command:
			p.view.OpenOptions()
		default:
			return nil, false
		}
		return nil, true
	}
	if m == nil {
		p.menu = false
		return nil, false
	}
	n := len(m.Items)
	switch {
	case k == 0x4800:
		p.sel = (p.sel + n - 1) % n
	case k == 0x5000:
		p.sel = (p.sel + 1) % n
	case k == 13:
		p.menu = false
		if p.sel == m.Sel { // the game's own choice: Enter picks it (some prompts take nothing else)
			return nil, false
		}
		return []uint16{uint16(m.Items[p.sel].Key)}, true
	case k == 27:
		p.menu = false
	default:
		for _, it := range m.Items { // a hotkey still picks its item
			if c := byte(k); k < 128 && (c == it.Key || c-'a'+'A' == it.Key) {
				p.menu = false
				return nil, false
			}
		}
	}
	return nil, true
}

// Flow paces a held move key (code, 0 for none) with fluid movement: it
// reports whether to send code now (ready: the game waits for a key), and
// whether the view paces held keys at all (else they repeat as typed).
func (p *Presenter) Flow(code uint16, ready bool) (send, paced bool) {
	if !p.active || p.view == nil || p.menu || p.view.OptionsOpen() {
		if p.view != nil {
			p.view.Flow(false, false)
		}
		return false, false
	}
	return p.view.Flow(code != 0, ready), p.view.Fluid()
}

// Options reports whether the renderer's options page is up; while it is,
// UpdateOptions reads the keyboard for it and the game gets no keys.
func (p *Presenter) Options() bool { return p.view != nil && p.view.OptionsOpen() }

// UpdateOptions runs the options page for one frame.
func (p *Presenter) UpdateOptions() { p.view.UpdateOptions() }

// menuID tells menus apart: prompt and items.
func menuID(m *engine.Menu) string {
	id := m.Prompt
	for _, it := range m.Items {
		id += "|" + it.Text
	}
	return id
}

// cardAt lays the party's cards along the window's edges: the first half
// of the party down the left, the rest down the right, at the largest
// whole scale where a column of four fits (eight with two NPCs).
func cardAt(dst *ebiten.Image) func(i, n int) image.Rectangle {
	const gap, rows = 8, 4
	b := dst.Bounds()
	k := max(1, min(ui.K(480, 360), (b.Dy()-blobber.MinimapRect().Dy())/(rows*(ui.CardH+gap))))
	return func(i, n int) image.Rectangle {
		left := (n + 1) / 2
		col, row, per := 0, i, left
		if i >= left {
			col, row, per = 1, i-left, n-left
		}
		y := b.Dy() - (per-row)*(ui.CardH+gap)*k + ui.CardFoot*k // the bars end a gap above the bottom
		x := gap * k
		if col == 1 {
			x = b.Dx() - gap*k - ui.CardW*k
		}
		return image.Rect(x, y, x+ui.CardW*k, y+ui.CardH*k)
	}
}

// scene is the remastered map of the area v shows, or nil.
func (p *Presenter) scene(v engine.View) *blobber.Scene {
	name, ok := areaMaps[v.Area]
	if !ok || p.failed {
		return nil
	}
	if p.view == nil {
		var err error
		if p.view, err = blobber.NewView(); err != nil {
			log.Println("remaster:", err)
			p.failed = true
			return nil
		}
		ui.Install() // the skin for the menu and options windows
	}
	if sc, ok := p.scenes[name]; ok {
		return sc
	}
	var sc *blobber.Scene
	if f, err := maps.Levels.Open(name + ".map"); err != nil {
		log.Println("remaster:", err)
	} else {
		defer f.Close()
		if sc, _, err = blobber.ParseScene(f); err != nil {
			log.Printf("remaster: %s.map: %v", name, err)
		}
	}
	if p.scenes == nil {
		p.scenes = map[string]*blobber.Scene{}
	}
	p.scenes[name] = sc // nil too: a broken map stays classic
	return sc
}

// animate runs the renderer's 60 ticks a second, whatever the frame rate.
func (p *Presenter) animate() {
	now := time.Now()
	if !p.last.IsZero() {
		p.ticks = min(p.ticks+now.Sub(p.last).Seconds()*60, 6) // after a pause, no long catch-up
	}
	p.last = now
	for ; p.ticks >= 1; p.ticks-- {
		p.view.Update()
	}
}

// scale is the largest whole factor the classic screen fits dst at (at
// least 1): pixels stay square.
func scale(dst *ebiten.Image) float64 {
	b := dst.Bounds()
	return float64(max(1, min(b.Dx()/ega.W, b.Dy()/ega.H)))
}

// drawBox copies box r of the classic frame, border included, onto dst at
// scale k, centred on cx, cy.
func drawBox(dst, frame *ebiten.Image, r engine.CellRect, k float64, cx, cy int) {
	px := r.Pixels()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(cx)-k*float64(px.Dx())/2, float64(cy)-k*float64(px.Dy())/2)
	dst.DrawImage(frame.SubImage(px).(*ebiten.Image), op)
}

// drawPrompt lays the original's menu and prompt rows along the bottom.
func (p *Presenter) drawPrompt(dst, frame *ebiten.Image, k float64) {
	r := engine.PromptRows.Pixels()
	b := dst.Bounds()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate((float64(b.Dx())-k*float64(r.Dx()))/2, float64(b.Dy())-k*float64(r.Dy()))
	dst.DrawImage(frame.SubImage(r).(*ebiten.Image), op)
}

// drawParty shows the party as Blobber's portrait cards, each with the
// character's paper-doll head.
func (p *Presenter) drawParty(dst *ebiten.Image, ms []engine.Member) {
	p.party.Chars = p.party.Chars[:0]
	for _, m := range ms {
		p.party.Chars = append(p.party.Chars, &party.Character{Name: m.Name, HP: m.HP, MaxHP: m.MaxHP, Face: p.face(m.Head)})
	}
	ui.DrawParty(dst, &p.party)
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
	const n = ui.PortraitSize
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
	if p.faces == nil {
		p.faces = map[image.Image]image.Image{}
	}
	p.faces[head] = f
	return f
}
