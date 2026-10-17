// Package remaster shows the game with the Blobber renderer: the 3D view
// fills the window, and the original's windows are laid over it. It only
// reads the game, through engine.View and the classic frame, so switching
// to and from it never touches the game's state.
package remaster

import (
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"blobber"
	"blobber/maps"

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
	failed bool                      // the renderer could not start: classic from then on
	scenes map[string]*blobber.Scene // parsed maps by name
	shown  string                    // the scene the view shows
	last   time.Time                 // when the animation last advanced
	ticks  float64                   // animation time not yet run, in 1/60 s ticks
}

// Draw shows the game on dst and returns where the classic picture went
// (zero when it is not on screen: the mouse then points at nothing).
func (p *Presenter) Draw(dst *ebiten.Image, scr *ega.Screen, note classic.Note, v engine.View) classic.Rect {
	sc := p.scene(v)
	if sc == nil || !v.MainFrame || (v.Window != engine.WinThreeD && v.Window != engine.WinPicture) {
		p.shown = "" // back from elsewhere: no tween from where the view was
		return p.Classic.Draw(dst, scr, note)
	}
	frame := p.Classic.Frame(scr, note)
	cam := blobber.Camera{X: v.X, Y: v.Y, Dir: int(v.Facing)}
	if name := areaMaps[v.Area]; name != p.shown {
		p.view.Show(sc, cam)
		p.shown = name
	} else {
		p.view.Follow(cam)
	}
	p.animate()
	p.view.Draw(dst)
	k := scale(dst)
	b := dst.Bounds()
	pk := min(k, max(1, float64(b.Dy()/2/engine.ViewWindow.Framed().Pixels().Dy()))) // the pop-ups fit their halves
	if v.Window == engine.WinPicture {
		drawBox(dst, frame, engine.ViewWindow.Framed(), pk, b.Dy()/4)
	}
	if v.TextShown {
		drawBox(dst, frame, engine.TextPane.Framed(), pk, b.Dy()*3/4)
	}
	p.drawPrompt(dst, frame, k)
	return classic.Rect{}
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
// scale k, centred across and on the line cy.
func drawBox(dst, frame *ebiten.Image, r engine.CellRect, k float64, cy int) {
	px := r.Pixels()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate((float64(dst.Bounds().Dx())-k*float64(px.Dx()))/2, float64(cy)-k*float64(px.Dy())/2)
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
