package gpu

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gogpu/gogpu"
	_ "github.com/gogpu/wgpu/hal/allbackends" // the desktop GPU backends
)

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
	app      *gogpu.App
	ww, wh   int // requested size
	title    string
	tps      int
	resize   bool
	aw, ah   int  // the window's aspect while not full screen (SetWindowAspect; 0: free)
	lw, lh   int  // its size last frame
	unfocus  bool // keep updating while unfocused (always on here)
	full     bool
	toFull   atomic.Int32  // a fullscreen change for the main thread: 1 off, 2 on
	toSize   atomic.Uint64 // a window size for the main thread: w<<32 | h (0: none)
	fps      float64
	frames   int
	fpsStart time.Time
	drawT    time.Duration // in Draw and in present since fpsStart (POR_FPS)
	presentT time.Duration
	updT     time.Duration // in Update
	outT     time.Duration // outside Draw: the window's own work
	lastOut  time.Time

	// the screen's place in the framebuffer (for the cursor)
	ox, oy, k float64
}

func init() { win.ww, win.wh, win.tps, win.title = 640, 480, 60, "" }

func SetWindowSize(w, h int) {
	win.ww, win.wh = w, h
	if win.app != nil { // AppKit wants it on the main thread: OnUpdate applies it
		win.toSize.Store(uint64(w)<<32 | uint64(h))
	}
}

// SetWindowAspect keeps the window at w:h while it is not full screen (0:
// any shape): a resize by the player is corrected along the side that
// changed.
func SetWindowAspect(w, h int) { win.aw, win.ah = w, h }

func SetWindowTitle(t string) {
	win.title = t
	if win.app != nil {
		win.app.SetTitle(t)
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
	win.full = on
	if win.app != nil { // AppKit wants it on the main thread: OnUpdate applies it
		win.toFull.Store(map[bool]int32{false: 1, true: 2}[on])
	}
}

// MonitorType is the window's monitor.
type MonitorType struct{}

// Monitor returns the window's monitor.
func Monitor() *MonitorType { return &MonitorType{} }

// DeviceScaleFactor is framebuffer pixels per window unit.
func (*MonitorType) DeviceScaleFactor() float64 {
	if win.app == nil || win.app.ScaleFactor() == 0 {
		return 1
	}
	return win.app.ScaleFactor()
}

// RunGameOptions are RunGameWithOptions' options.
type RunGameOptions struct {
	InitUnfocused bool // ponytail: gogpu has no unfocused start; the window opens focused
}

func RunGame(g Game) error { return RunGameWithOptions(g, nil) }

// RunGameWithOptions opens the window and runs g until the window closes or
// Update returns an error.
func RunGameWithOptions(g Game, o *RunGameOptions) error {
	cfg := gogpu.DefaultConfig().WithTitle(win.title).WithSize(win.ww, win.wh).
		WithResizable(win.resize).WithContinuousRender(true).WithVSync(os.Getenv("POR_NOVSYNC") == "") // POR_NOVSYNC=1: frames as fast as they come, for measuring
	if win.full {
		cfg = cfg.WithFullscreen()
		setComposition(false)
	}
	app := gogpu.NewApp(cfg)
	win.app = app
	if win.full { // the Windows platform ignores the config's fullscreen: apply it as the Esc menu would (a no-op where the config did it)
		win.toFull.Store(2)
	}
	initInput(app)
	app.OnUpdate(func(float64) { // the main thread
		if f := win.toFull.Swap(0); f != 0 {
			app.SetFullscreen(f == 2)
			setComposition(f != 2)
		}
		if s := win.toSize.Swap(0); s != 0 {
			app.RequestSize(int(s>>32), int(s&0xFFFFFFFF))
		}
	})
	var (
		screen *Image
		last   time.Time
		acc    float64
		runErr error
	)
	app.OnDraw(func(dc *gogpu.Context) {
		p := app.DeviceProvider()
		view := dc.SurfaceView()
		if p == nil || view == nil || runErr != nil {
			return
		}
		if screen == nil && last.IsZero() {
			if err := adoptDevice(p.Device(), p.Queue(), p.SurfaceFormat()); err != nil {
				runErr = err
				app.Quit()
				return
			}
			last = time.Now()
			win.fpsStart = last
		}
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
		tu := time.Now()
		for range n {
			tickInput()
			if err := g.Update(); err != nil {
				if err != ErrTermination {
					runErr = err
				}
				app.Quit()
				return
			}
		}
		win.updT += time.Since(tu)
		ww, wh := app.Size()
		fw, fh := dc.FramebufferSize()
		if ww == 0 || wh == 0 || fw == 0 || fh == 0 { // minimised
			return
		}
		if win.aw > 0 && !win.full && ww*win.ah != wh*win.aw { // off the aspect: the side the player left alone follows the other
			nw, nh := ww, ww*win.ah/win.aw
			if ww == win.lw && wh != win.lh {
				nw, nh = wh*win.aw/win.ah, wh
			}
			win.toSize.Store(uint64(nw)<<32 | uint64(nh)) // from the main thread: here is the render thread, and setContentSize there traps during a live resize
		}
		win.lw, win.lh = ww, wh
		sw, sh := g.Layout(ww, wh)
		sw, sh = max(sw, 1), max(sh, 1)
		direct := sw == fw && sh == fh && !offscreen // the game draws straight into the frame: no copy of the whole screen per frame
		if screen == nil || screen.r.Dx() != sw || screen.r.Dy() != sh || (screen.t == surf) != direct {
			if screen != nil && screen.t != surf {
				screen.Deallocate()
			}
			if direct {
				if surf.depth != nil {
					surf.dview.Release()
					surf.depth.Release()
					surf.depth, surf.dview = nil, nil
				}
				surf.w, surf.h = sw, sh
				screen = &Image{surf, image.Rect(0, 0, sw, sh)}
			} else {
				screen = NewImage(sw, sh)
			}
			fmt.Fprintf(os.Stderr, "screen %dx%d for a %dx%d window, %dx%d framebuffer, direct=%v\n", sw, sh, ww, wh, fw, fh, direct) // rare: a flash of one frame could be a stray size
		}
		surf.view = view
		screen.Clear()
		var passes []string
		if win.frames == 0 && os.Getenv("POR_PASSES") != "" { // POR_PASSES=1: one frame's render passes with each fps line
			passLog = &passes
		}
		t0 := time.Now()
		g.Draw(screen)
		win.drawT += time.Since(t0)
		win.k = min(float64(fw)/float64(sw), float64(fh)/float64(sh))
		win.ox, win.oy = (float64(fw)-float64(sw)*win.k)/2, (float64(fh)-float64(sh)*win.k)/2
		t1 := time.Now()
		if direct { // the frame's passes end here (a cleared frame when nothing was drawn, as before)
			flushBatch()
			if surf.clear {
				passOn(surf, false)
			}
			submit()
		} else if !Skip("present") {
			present(screen, view, fw, fh)
		}
		win.presentT += time.Since(t1)
		if glitchDir != "" {
			glitchCheck(screen)
		}
		if passLog != nil {
			passLog = nil
			fmt.Fprintln(os.Stderr, strings.Join(passes, "\n"))
		}
		win.frames++
		win.outT += now.Sub(win.lastOut) // since the last Draw returned: the window's own work (events, the swap chain)
		win.lastOut = time.Now()
		if d := now.Sub(win.fpsStart).Seconds(); d >= 1 {
			if os.Getenv("POR_FPS") != "" { // POR_FPS=1: a line a second with the frame rate and where a frame's time goes
				n := time.Duration(max(win.frames, 1))
				fr := max(win.frames, 1)
				fmt.Fprintf(os.Stderr, "fps %.1f  frame %v  update %v  draw %v  present %v  outside %v  per frame: draws %d (3D %d) passes %d clears %d writes %d uploads %d\n", float64(win.frames)/d, time.Duration(d*float64(time.Second))/n, win.updT/n, win.drawT/n, win.presentT/n, win.outT/n, Stats.Draws/fr, Stats.Draws3D/fr, Stats.Passes/fr, Stats.Clears/fr, Stats.Writes/fr, Stats.Uploads/fr)
				Stats = struct{ Draws, Draws3D, Passes, Clears, Writes, Uploads int }{}
			}
			win.fps, win.frames, win.fpsStart = float64(win.frames)/d, 0, now
			win.drawT, win.presentT, win.updT, win.outT = 0, 0, 0, 0
		}
	})
	if err := app.Run(); err != nil {
		return err
	}
	return runErr
}

// Stats counts the GPU work issued since the last POR_FPS line: draw calls,
// render passes, buffer writes and texture uploads.
var Stats struct{ Draws, Draws3D, Passes, Clears, Writes, Uploads int }

// POR_GLITCH=DIR: after each frame is presented the screen is read back and
// compared to the frame before; when more than half of its pixels changed,
// both frames go to DIR as PNGs (a glitch of one frame that a capture of
// every frame would be too slow to catch, and that reading the screen before
// the present could hide).
var glitchDir = os.Getenv("POR_GLITCH")

var glitch struct {
	prev, cur []byte
	n, saved  int
}

func glitchCheck(screen *Image) {
	w, h := screen.Size()
	if len(glitch.cur) != w*h*4 {
		glitch.prev, glitch.cur, glitch.n = make([]byte, w*h*4), make([]byte, w*h*4), 0
	}
	screen.ReadPixels(glitch.cur)
	d := 0
	for i := 0; i < len(glitch.cur); i += 16 {
		if glitch.cur[i] != glitch.prev[i] || glitch.cur[i+1] != glitch.prev[i+1] || glitch.cur[i+2] != glitch.prev[i+2] {
			d++
		}
	}
	if glitch.n > 0 && d*32 > len(glitch.cur) && glitch.saved < 40 {
		for k, px := range [][]byte{glitch.prev, glitch.cur} {
			f, err := os.Create(filepath.Join(glitchDir, fmt.Sprintf("glitch%d_%c.png", glitch.saved, 'a'+k)))
			if err == nil {
				err = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(f, &image.RGBA{Pix: px, Stride: w * 4, Rect: image.Rect(0, 0, w, h)})
				f.Close()
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "glitch:", err)
			}
		}
		fmt.Fprintf(os.Stderr, "glitch %d at frame %d: %.0f%% of the pixels changed\n", glitch.saved, glitch.n, 100*float64(d*16)/float64(len(glitch.cur)))
		glitch.saved++
	}
	glitch.prev, glitch.cur = glitch.cur, glitch.prev
	glitch.n++
}

// Skip reports whether name is in POR_SKIP, a comma list of the frame's parts left out
// (fill, sky, planes, floors, walls, bills, dither, hud; in a battle table, board, shadows, frames,
// figures, sparks) to time the rest, or of its savings (clear: a whole-image Fill drawn, not cleared;
// back: the planes write depth) to check the picture is the same without them.
func Skip(name string) bool { return slices.Contains(strings.Split(os.Getenv("POR_SKIP"), ","), name) }

// surf stands for the window's frame, drawn into directly when the screen is the frame's
// size; its view is the one gogpu lends each frame. offscreen keeps the screen an image of
// its own (and a present pass copying it) when something reads it back.
var surf = &texture{external: true}
var offscreen = os.Getenv("POR_SHOT") != "" || os.Getenv("BLOBBER_SHOT") != "" || os.Getenv("POR_GLITCH") != "" || os.Getenv("POR_OFFSCREEN") != ""
