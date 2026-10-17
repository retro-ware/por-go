// Command por runs Pool of Radiance on top of the original data files.
//
//	por [-data DIR] [-scale N]
//
// Gamepad (standard layout): D-pad or left stick move and turn, A picks the
// highlighted menu item, B is Escape, LB/RB move the menu highlight, LT/RT
// select the previous/next party member, X shows the party overview, Y
// toggles the minimap, Start is Enter.
//
// Mouse: pointing at a menu item highlights it, a click picks it (and list
// lines, party members, combat squares); the wheel moves the highlight and
// the right button is Escape. "?" lists the keys; F1-F8 open the inventory
// of party member 1-8. F10 switches between the original screen and the
// remastered one (present/remaster; POR_REMASTER=1 starts in it).
//
// DIR must hold the DOS game files (TITLE.DAX, 8X8D1.DAX, GAME data ...);
// it defaults to $GOPLAY_DATA, else the parent of the executable's module
// directory, i.e. the folder this source tree sits in. With -import SRC
// (used by the goplay web page) the install found under SRC is copied into
// DIR with a COVER.PNG, and the program exits.
package main

import (
	"flag"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"por/ega"
	"por/engine"
	"por/present/classic"
	"por/present/remaster"
)

type game struct {
	scr        *ega.Screen
	eng        *engine.Engine
	classic    classic.Presenter
	remaster   remaster.Presenter // draws through classic where it has no remaster
	remastered bool               // F10: the remastered screen instead of the original's

	pads    []ebiten.GamepadID // input buffers, reused each tick
	pressed []ebiten.Key
	chars   []rune

	stickDir  uint16 // left stick direction being held
	stickHeld int    // ticks it has been held

	dsf   float64      // device scale factor of the last Layout
	pic   classic.Rect // where the picture is drawn, in device pixels
	wheel float64      // wheel movement not yet sent as a step
	sent  bool         // input went to the engine this tick
	ticks int          // Update calls, counted only for POR_SHOT
}

// tps: Update runs 120 times a second, so a key waits at most ~8 ms before
// the engine sees it.
const tps = 120

func (g *game) Update() error {
	g.sent = false
	if shot != "" { // the user's input never reaches a capture, only the scripted keys
		g.ticks++
		if k := g.ticks / 30; g.ticks%30 == 0 && k <= len(shotKeys) {
			if c := shotKeys[k-1]; c == shotF10 {
				g.remastered = !g.remastered
				g.remaster.Switched()
			} else {
				g.push(c)
				g.settle()
			}
		}
		return nil
	}
	if g.remastered && g.remaster.Options() { // the renderer's options page has the keyboard
		g.remaster.UpdateOptions()
		return nil
	}
	g.keys()
	for _, id := range ebiten.AppendGamepadIDs(g.pads[:0]) {
		g.gamepad(id)
	}
	g.mouse()
	if g.sent {
		g.settle()
	}
	return nil
}

// push sends a key to the engine.
func (g *game) push(k uint16) {
	if g.remastered {
		ks, taken := g.remaster.Key(k, g.eng.View())
		for _, k := range ks {
			g.scr.PushKey(k)
			g.sent = true
		}
		if taken {
			return
		}
	}
	g.scr.PushKey(k)
	g.sent = true
}

// settle gives the engine up to 8 ms to finish with the keys just sent, so
// the frame drawn next already shows their result (instead of one frame
// later). Gosched, not Sleep: in the browser the engine shares the thread.
func (g *game) settle() {
	t0 := time.Now()
	for n := 0; !g.scr.Settled() && time.Since(t0) < 8*time.Millisecond; n++ {
		if n < 50 {
			runtime.Gosched()
		} else {
			time.Sleep(100 * time.Microsecond) // a long job: stop burning the CPU
		}
	}
}

// Held arrow keys repeat like the PC keyboard did (typematic), but only
// while the engine waits for input, so walking stops when the key is let go.
var repeatKeys = []ebiten.Key{
	ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyArrowLeft, ebiten.KeyArrowRight,
	ebiten.KeyKP8, ebiten.KeyKP2, ebiten.KeyKP4, ebiten.KeyKP6,
	ebiten.KeyPageUp, ebiten.KeyPageDown,
}

// keys passes the keyboard to the engine.
func (g *game) keys() {
	paced := false
	if g.remastered { // fluid movement: the view asks for the held move as the last one ends
		var held uint16
		for _, k := range repeatKeys[:8] {
			if inpututil.KeyPressDuration(k) > 0 {
				held, _ = biosKey(k)
			}
		}
		var send bool
		send, paced = g.remaster.Flow(held, g.scr.Idle())
		if send {
			g.push(held)
		}
	}
	for i, k := range repeatKeys {
		if i < 8 && paced {
			continue
		}
		if d := inpututil.KeyPressDuration(k); d > 1 && repeats(d) && g.scr.Idle() {
			code, _ := biosKey(k)
			g.push(code)
		}
	}
	for _, k := range inpututil.AppendJustPressedKeys(g.pressed[:0]) {
		if k == ebiten.KeyF10 { // never the game's: only how it is shown changes
			g.remastered = !g.remastered
			g.remaster.Switched()
		} else if code, ok := biosKey(k); ok {
			g.push(code)
		} else if k == ebiten.KeyF && ebiten.IsKeyPressed(ebiten.KeyControl) && ebiten.IsKeyPressed(ebiten.KeyShift) {
			g.push(engine.KeyFontPrev) // Ctrl-F next font, Ctrl-Shift-F the one before
		} else if k >= ebiten.KeyA && k <= ebiten.KeyZ && ebiten.IsKeyPressed(ebiten.KeyControl) {
			g.push(uint16(k-ebiten.KeyA) + 1) // Ctrl-letter: ASCII 1..26 (Ctrl-S sound toggle, Ctrl-C quit)
		}
	}
	// a Ctrl combination types no letter of its own (Ctrl+Alt is AltGr: it does)
	if ebiten.IsKeyPressed(ebiten.KeyControl) && !ebiten.IsKeyPressed(ebiten.KeyAlt) {
		return
	}
	for _, r := range ebiten.AppendInputChars(g.chars[:0]) {
		if r < 128 {
			g.push(uint16(r))
		}
	}
}

// mouse passes the pointer's text cell, clicks and wheel steps to the engine.
func (g *game) mouse() {
	mx, my := ebiten.CursorPosition()
	cx, cy, in := g.cell(float64(mx), float64(my))
	g.scr.SetMouse(cx, cy, in)
	if in && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.push(engine.MouseKey(engine.MouseLeft, cx, cy))
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.push(engine.MouseKey(engine.MouseRight, 0, 0))
	}
	_, dy := ebiten.Wheel()
	g.wheel += dy
	for ; g.wheel >= 1; g.wheel-- {
		g.push(engine.MouseKey(engine.MouseWheelUp, 0, 0))
	}
	for ; g.wheel <= -1; g.wheel++ {
		g.push(engine.MouseKey(engine.MouseWheelDown, 0, 0))
	}
}

// cell is the text cell (40x25) of the picture under device pixel (x, y).
func (g *game) cell(x, y float64) (int, int, bool) {
	p := g.pic
	if p.W <= 0 || x < p.X || y < p.Y || x >= p.X+p.W || y >= p.Y+p.H {
		return 0, 0, false
	}
	px := int((x - p.X) * ega.W / p.W)
	py := int((y - p.Y) * ega.H / p.H)
	return px / 8, py / 8, true
}

// padButtons maps standard-layout gamepad buttons to BIOS key codes (and
// the engine's virtual gamepad keys).
var padButtons = []struct {
	b      ebiten.StandardGamepadButton
	key    uint16
	repeat bool
}{
	{ebiten.StandardGamepadButtonLeftTop, engine.KeyPadUp, true},
	{ebiten.StandardGamepadButtonLeftBottom, engine.KeyPadDown, true},
	{ebiten.StandardGamepadButtonLeftLeft, engine.KeyPadLeft, true},
	{ebiten.StandardGamepadButtonLeftRight, engine.KeyPadRight, true},
	{ebiten.StandardGamepadButtonRightBottom, engine.KeyPadSelect, false},
	{ebiten.StandardGamepadButtonRightRight, 27, false},
	{ebiten.StandardGamepadButtonRightLeft, engine.KeyPadOverview, false},
	{ebiten.StandardGamepadButtonRightTop, engine.KeyPadMinimap, false},
	{ebiten.StandardGamepadButtonFrontTopLeft, ',', true},
	{ebiten.StandardGamepadButtonFrontTopRight, '.', true},
	{ebiten.StandardGamepadButtonFrontBottomLeft, 0x4700, true},
	{ebiten.StandardGamepadButtonFrontBottomRight, 0x4F00, true},
	{ebiten.StandardGamepadButtonCenterRight, 13, false},
	{ebiten.StandardGamepadButtonCenterLeft, 27, false},
}

// Held buttons repeat after padDelay ticks (1/tps s), every padRate ticks.
const padDelay, padRate = tps / 3, tps * 3 / 20

func repeats(d int) bool { return d == 1 || d >= padDelay && (d-padDelay)%padRate == 0 }

func (g *game) gamepad(id ebiten.GamepadID) {
	if !ebiten.IsStandardGamepadLayoutAvailable(id) {
		return
	}
	for _, p := range padButtons {
		d := inpututil.StandardGamepadButtonPressDuration(id, p.b)
		if d == 1 || p.repeat && d > 1 && repeats(d) {
			g.push(p.key)
		}
	}
	// Left stick as a D-pad.
	x := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
	y := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
	dir := uint16(0)
	switch {
	case y < -0.6:
		dir = engine.KeyPadUp
	case y > 0.6:
		dir = engine.KeyPadDown
	case x < -0.6:
		dir = engine.KeyPadLeft
	case x > 0.6:
		dir = engine.KeyPadRight
	}
	if dir != g.stickDir {
		g.stickDir, g.stickHeld = dir, 0
	}
	if dir != 0 {
		g.stickHeld++
		if repeats(g.stickHeld) {
			g.push(dir)
		}
	}
}

func (g *game) Draw(dst *ebiten.Image) {
	text, at := g.eng.Note()
	note := classic.Note{Text: text, Show: time.Since(at) < 2*time.Second, Glyph: g.eng.Glyph}
	if g.remastered {
		g.pic = g.remaster.Draw(dst, g.scr, note, g.eng.View())
	} else {
		g.pic = g.classic.Draw(dst, g.scr, note)
	}
	if shot != "" && g.ticks > 30*(len(shotKeys)+4) {
		saveShot(dst)
	}
}

// POR_SHOT=FILE.png writes the window to FILE after the keys in POR_KEYS,
// one every quarter second, and exits; for full size checks of the
// presenters. The window opens unfocused and ignores the user's input
// meanwhile. POR_KEYS escapes: \r Enter, \e Escape, \U \D \L \R the
// arrows, \T F10 (switch the presenter).
var shot, shotKeys = os.Getenv("POR_SHOT"), scriptKeys(os.Getenv("POR_KEYS"))

const shotF10 = 0xFFFF // in shotKeys: not a key for the engine

func scriptKeys(s string) []uint16 {
	esc := map[byte]uint16{'r': 13, 'e': 27, 'U': 0x4800, 'D': 0x5000, 'L': 0x4B00, 'R': 0x4D00, 'T': shotF10}
	var out []uint16
	for i := 0; i < len(s); i++ {
		if c, ok := esc[s[min(i+1, len(s)-1)]]; s[i] == '\\' && i+1 < len(s) && ok {
			out = append(out, c)
			i++
		} else {
			out = append(out, uint16(s[i]))
		}
	}
	return out
}

func saveShot(dst *ebiten.Image) {
	b := dst.Bounds()
	img := image.NewRGBA(b)
	dst.ReadPixels(img.Pix)
	f, err := os.Create(shot)
	if err == nil {
		err = png.Encode(f, img)
		f.Close()
	}
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}

// Layout works in device pixels, so the picture fills the window exactly.
func (g *game) Layout(w, h int) (int, int) {
	g.dsf = ebiten.Monitor().DeviceScaleFactor()
	return int(float64(w) * g.dsf), int(float64(h) * g.dsf)
}

// biosKey maps keys the DOS game read as non-ASCII (scan code in the high
// byte, zero low byte) exactly like INT 16h returned them.
func biosKey(k ebiten.Key) (uint16, bool) {
	switch k {
	case ebiten.KeyEnter, ebiten.KeyKPEnter:
		return 13, true
	case ebiten.KeyEscape:
		return 27, true
	case ebiten.KeyBackspace:
		return 8, true
	case ebiten.KeyTab:
		return 9, true
	case ebiten.KeyF1, ebiten.KeyF2, ebiten.KeyF3, ebiten.KeyF4, ebiten.KeyF5, ebiten.KeyF6, ebiten.KeyF7, ebiten.KeyF8:
		return engine.KeyF1 + uint16(k-ebiten.KeyF1)<<8, true
	case ebiten.KeyF9:
		return engine.KeyF9, true
	case ebiten.KeyArrowUp, ebiten.KeyKP8:
		return 0x4800, true
	case ebiten.KeyArrowDown, ebiten.KeyKP2:
		return 0x5000, true
	case ebiten.KeyArrowLeft, ebiten.KeyKP4:
		return 0x4B00, true
	case ebiten.KeyArrowRight, ebiten.KeyKP6:
		return 0x4D00, true
	case ebiten.KeyHome, ebiten.KeyKP7:
		return 0x4700, true
	case ebiten.KeyEnd, ebiten.KeyKP1:
		return 0x4F00, true
	case ebiten.KeyPageUp, ebiten.KeyKP9:
		return 0x4900, true
	case ebiten.KeyPageDown, ebiten.KeyKP3:
		return 0x5100, true
	case ebiten.KeyKP5:
		return 0x4C00, true
	}
	return 0, false
}

func main() {
	dir := flag.String("data", os.Getenv("GOPLAY_DATA"), "directory with the original game files")
	scale := flag.Int("scale", 3, "window scale")
	imp := flag.String("import", "", "copy the game found under this directory into -data, then exit (goplay)")
	imp2 := flag.String("import-disk2", "", "convert the Amiga graphics from the .adf disks under this directory into -data/DISK2, then exit (goplay)")
	flag.Parse()
	if *imp2 != "" {
		if *dir == "" {
			log.Fatal("-import-disk2 needs -data")
		}
		if err := importDisk2(*imp2, *dir); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *imp != "" {
		if *dir == "" {
			log.Fatal("-import needs -data")
		}
		if err := importGame(*imp, *dir); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *dir == "" {
		exe, _ := os.Executable()
		*dir = filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(*dir, "TITLE.DAX")); err != nil {
			*dir = filepath.Join("..")
		}
	}
	scr := ega.New()
	eng, err := engine.New(*dir, scr)
	if err != nil {
		log.Fatal(err)
	}
	actx := audio.NewContext(44100)
	if pl, err := actx.NewPlayer(eng.SoundReader()); err == nil {
		// at least the 2048 frames (46 ms) a browser pulls at once, or the
		// rest of each pull plays as silence (goplay)
		pl.SetBufferSize(50 * time.Millisecond)
		pl.Play()
	}
	eng.Cfg.Param = flag.Arg(0)
	go eng.Run()

	g := &game{scr: scr, eng: eng, dsf: 1, remastered: os.Getenv("POR_REMASTER") != ""}
	g.remaster.Classic = &g.classic
	ebiten.SetWindowSize(ega.W*(*scale), ega.H*(*scale)*6/5) // 4:3 like a CRT
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("Pool of Radiance")
	ebiten.SetTPS(tps)
	if shot != "" {
		ebiten.SetRunnableOnUnfocused(true)
	}
	if err := ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{InitUnfocused: shot != ""}); err != nil {
		log.Fatal(err)
	}
}
