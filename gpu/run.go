package gpu

import (
	"errors"
	"runtime"
	"time"

	"gpu/internal/glfw"
)

func init() { runtime.LockOSThread() } // GLFW wants the main thread

// Game is what RunGame runs, like ebiten.Game: Update runs TPS times a
// second, Draw once a frame into a screen of the size Layout returns for the
// window's size.
type Game interface {
	Update() error
	Draw(screen *Image)
	Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int)
}

// ErrTermination returned by Update ends RunGame without an error.
var ErrTermination = errors.New("gpu: termination")

var win struct {
	w        *glfw.Window
	ww, wh   int // requested size
	title    string
	tps      int
	resize   bool
	unfocus  bool // keep updating while unfocused (always on here)
	full     bool
	saved    [4]int // windowed position and size before fullscreen
	fps      float64
	frames   int
	fpsStart time.Time

	// the screen's place in the framebuffer (for the cursor)
	ox, oy, k float64
}

func init() { win.ww, win.wh, win.tps, win.title = 640, 480, 60, "" }

func SetWindowSize(w, h int) {
	win.ww, win.wh = w, h
	if win.w != nil {
		win.w.SetSize(w, h)
	}
}

func SetWindowTitle(t string) {
	win.title = t
	if win.w != nil {
		win.w.SetTitle(t)
	}
}

// WindowResizingMode says whether the player may resize the window.
type WindowResizingMode int

const (
	WindowResizingModeDisabled WindowResizingMode = iota
	WindowResizingModeEnabled
)

func SetWindowResizingMode(m WindowResizingMode) { win.resize = m == WindowResizingModeEnabled }
func SetTPS(n int)                               { win.tps = n }
func SetRunnableOnUnfocused(b bool)              { win.unfocus = b }
func ActualFPS() float64                         { return win.fps }
func IsFullscreen() bool                         { return win.full }

func SetFullscreen(on bool) {
	if win.w == nil || on == win.full {
		win.full = on && win.w == nil
		return
	}
	win.full = on
	if on {
		x, y := win.w.GetPos()
		w, h := win.w.GetSize()
		win.saved = [4]int{x, y, w, h}
		m := glfw.GetPrimaryMonitor()
		v := m.GetVideoMode()
		win.w.SetMonitor(m, 0, 0, v.Width, v.Height, v.RefreshRate)
	} else {
		s := win.saved
		win.w.SetMonitor(nil, s[0], s[1], s[2], s[3], 0)
	}
}

// MonitorType is the window's monitor.
type MonitorType struct{}

// Monitor returns the window's monitor.
func Monitor() *MonitorType { return &MonitorType{} }

// DeviceScaleFactor is framebuffer pixels per window unit.
func (*MonitorType) DeviceScaleFactor() float64 {
	if win.w == nil {
		return 1
	}
	fw, _ := win.w.GetFramebufferSize()
	ww, _ := win.w.GetSize()
	if ww == 0 {
		return 1
	}
	return float64(fw) / float64(ww)
}

// RunGameOptions are RunGameWithOptions' options.
type RunGameOptions struct {
	InitUnfocused bool
}

func RunGame(g Game) error { return RunGameWithOptions(g, nil) }

// RunGameWithOptions opens the window and runs g until the window closes or
// Update returns an error.
func RunGameWithOptions(g Game, o *RunGameOptions) error {
	if o == nil {
		o = &RunGameOptions{}
	}
	if err := glfw.Init(); err != nil {
		return err
	}
	defer glfw.Terminate()
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, b2i(win.resize))
	if o.InitUnfocused {
		glfw.WindowHint(glfw.FocusOnShow, glfw.False)
		glfw.WindowHint(glfw.Focused, glfw.False)
	}
	w, err := glfw.CreateWindow(win.ww, win.wh, win.title, nil, nil)
	if err != nil {
		return err
	}
	win.w = w
	attach(w)
	initInput(w)
	if err := attachSurface(surfaceTarget(w)); err != nil {
		return err
	}
	if win.full {
		win.full = false
		SetFullscreen(true)
	}
	var screen *Image
	last, acc := time.Now(), 0.0
	win.fpsStart = last
	for !w.ShouldClose() {
		glfw.PollEvents()
		now := time.Now()
		acc += now.Sub(last).Seconds() * float64(win.tps)
		last = now
		n := int(acc)
		acc -= float64(n)
		if n > 8 { // ponytail: a long stall drops ticks instead of catching up
			n = 8
		}
		if screen == nil {
			n = max(n, 1) // Update comes before the first Draw
		}
		for range n {
			tickInput()
			if err := g.Update(); err != nil {
				if err == ErrTermination {
					return nil
				}
				return err
			}
		}
		ww, wh := w.GetSize()
		fw, fh := w.GetFramebufferSize()
		if ww == 0 || wh == 0 { // minimised
			yield(w)
			continue
		}
		sw, sh := g.Layout(ww, wh)
		sw, sh = max(sw, 1), max(sh, 1)
		if screen == nil || screen.r.Dx() != sw || screen.r.Dy() != sh {
			if screen != nil {
				screen.Deallocate()
			}
			screen = NewImage(sw, sh)
		}
		screen.Clear()
		g.Draw(screen)
		win.k = min(float64(fw)/float64(sw), float64(fh)/float64(sh))
		win.ox, win.oy = (float64(fw)-float64(sw)*win.k)/2, (float64(fh)-float64(sh)*win.k)/2
		present(screen, fw, fh)
		yield(w)
		win.frames++
		if d := now.Sub(win.fpsStart).Seconds(); d >= 1 {
			win.fps, win.frames, win.fpsStart = float64(win.frames)/d, 0, now
		}
	}
	return nil
}

func b2i(b bool) int {
	if b {
		return glfw.True
	}
	return glfw.False
}
