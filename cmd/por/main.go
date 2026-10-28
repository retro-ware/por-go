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
	"blobber"
	"flag"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ebitengine/oto/v3"
	"gpu"

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
	queue      []uint16           // keys still to send, one each time the engine waits (sendKeys)

	pads    []gpu.GamepadID // input buffers, reused each tick
	pressed []gpu.Key
	chars   []rune

	stickDir  uint16 // left stick direction being held
	stickHeld int    // ticks it has been held

	dsf   float64      // device scale factor of the last Layout
	pic   classic.Rect // where the picture is drawn, in device pixels
	wheel float64      // wheel movement not yet sent as a step
	sent  bool         // input went to the engine this tick
	over  image.Point  // the 3D map square under the pointer +1 last sent (HoverKey)
	ticks int          // Update calls, counted only for POR_SHOT
}

// tps: Update runs 120 times a second, so a key waits at most ~8 ms before
// the engine sees it.
const tps = 120

func (g *game) Update() error {
	g.sent = false
	if len(g.queue) > 0 && g.scr.Idle() {
		g.scr.PushKey(g.queue[0])
		g.queue, g.sent = g.queue[1:], true
	}
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
	for _, id := range gpu.AppendGamepadIDs(g.pads[:0]) {
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
		g.sendKeys(ks)
		if taken {
			return
		}
	}
	ks := []uint16{k}
	if !g.remastered {
		ks = classicCombatKey(k, g.eng.View())
	}
	g.sendKeys(ks)
}

// sendKeys sends ks to the engine: the first at once, the others one at a
// time, each once the engine waits for a key again. It keeps only the last
// of keys typed ahead (ReadKeyByte), so a button's Done then Delay sent
// together would lose the Done.
func (g *game) sendKeys(ks []uint16) {
	if len(ks) == 0 {
		return
	}
	if len(g.queue) > 0 { // after the keys still waiting
		g.queue = append(g.queue, ks...)
		return
	}
	g.scr.PushKey(ks[0])
	g.queue, g.sent = append(g.queue, ks[1:]...), true
}

// classicCombatKey gives classic combat the remaster's controls (present/battle
// Key) where the original has none: W A S D scroll the map and Home brings
// whose turn it is back to the middle; aiming, Tab and Shift-Tab cycle the
// targets and Enter takes the aimed one. Other keys pass unchanged.
func classicCombatKey(k uint16, v engine.View) []uint16 {
	if v.Battle == nil {
		return []uint16{k}
	}
	if d := strings.IndexByte("wdsaWDSA", byte(k)); k < 128 && d >= 0 {
		return []uint16{engine.KeyPan + uint16(d%4)}
	}
	if v.Menu == nil { // Move's steps: Home is keypad 7
		return []uint16{k}
	}
	aiming, take := false, uint16(0)
	for _, it := range v.Menu.Items {
		aiming = aiming || it.Text == "Next" || it.Text == "Center"
		if it.Text == "Attack" {
			take = uint16(it.Key)
		}
	}
	switch {
	case k == 0x4700:
		return []uint16{engine.KeyPan + 4}
	case k == 9 && aiming && gpu.IsKeyPressed(gpu.KeyShift):
		return []uint16{'P'}
	case k == 9 && aiming:
		return []uint16{'N'}
	case k == 13 && aiming && take != 0:
		return []uint16{take}
	}
	return []uint16{k}
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
var repeatKeys = []gpu.Key{
	gpu.KeyArrowUp, gpu.KeyArrowDown, gpu.KeyArrowLeft, gpu.KeyArrowRight,
	gpu.KeyKP8, gpu.KeyKP2, gpu.KeyKP4, gpu.KeyKP6,
	gpu.KeyPageUp, gpu.KeyPageDown,
}

// keys passes the keyboard to the engine.
func (g *game) keys() {
	paced := false
	if g.remastered { // fluid movement: the view asks for the held move as the last one ends
		var held uint16
		for _, k := range repeatKeys[:8] {
			if gpu.KeyPressDuration(k) > 0 {
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
		if d := gpu.KeyPressDuration(k); d > 1 && repeats(d) && g.scr.Idle() {
			code, _ := biosKey(k)
			g.push(code)
		}
	}
	for _, k := range gpu.AppendJustPressedKeys(g.pressed[:0]) {
		if k == gpu.KeyF10 { // never the game's: only how it is shown changes
			g.remastered = !g.remastered
			g.remaster.Switched()
		} else if code, ok := biosKey(k); ok {
			g.push(code)
		} else if k == gpu.KeyF && gpu.IsKeyPressed(gpu.KeyControl) && gpu.IsKeyPressed(gpu.KeyShift) {
			g.push(engine.KeyFontPrev) // Ctrl-F next font, Ctrl-Shift-F the one before
		} else if k >= gpu.KeyA && k <= gpu.KeyZ && gpu.IsKeyPressed(gpu.KeyControl) {
			g.push(uint16(k-gpu.KeyA) + 1) // Ctrl-letter: ASCII 1..26 (Ctrl-S sound toggle, Ctrl-C quit)
		}
	}
	// a Ctrl combination types no letter of its own (Ctrl+Alt is AltGr: it does)
	if gpu.IsKeyPressed(gpu.KeyControl) && !gpu.IsKeyPressed(gpu.KeyAlt) {
		return
	}
	for _, r := range gpu.AppendInputChars(g.chars[:0]) {
		if r < 128 {
			g.push(uint16(r))
		}
	}
}

// mouse passes the pointer's text cell, clicks and wheel steps to the engine.
func (g *game) mouse() {
	mx, my := gpu.CursorPosition()
	if g.remaster.InventoryShown() { // its own clicks and wheel; keys still go to the game
		_, dy := gpu.Wheel()
		g.wheel += dy
		steps := int(g.wheel)
		g.wheel -= float64(steps)
		g.sendKeys(g.remaster.InventoryMouse(mx, my, gpu.IsMouseButtonJustPressed(gpu.MouseButtonLeft), steps))
		if gpu.IsMouseButtonJustPressed(gpu.MouseButtonRight) {
			g.push(engine.MouseKey(engine.MouseRight, 0, 0))
		}
		return
	}
	if g.remastered {
		g.remaster.Hover(mx, my)
	}
	if g.remastered && gpu.IsMouseButtonJustPressed(gpu.MouseButtonLeft) {
		if ks, ok := g.remaster.Click(mx, my); ok { // an action bar button, a menu item
			g.sendKeys(ks)
			return
		}
		if x, y, ok := g.remaster.Square(mx, my); ok { // a square on the 3D map
			g.scr.PushKey(engine.SquareKey(x, y))
			g.sent = true
			return
		}
	}
	if g.remastered && g.scr.Idle() { // pointing at a square: aiming previews it
		if x, y, ok := g.remaster.Square(mx, my); ok && image.Pt(x+1, y+1) != g.over {
			g.over = image.Pt(x+1, y+1)
			g.scr.PushKey(engine.HoverKey(x, y))
			g.sent = true
		}
	}
	cx, cy, in := g.cell(float64(mx), float64(my))
	g.scr.SetMouse(cx, cy, in)
	if in && gpu.IsMouseButtonJustPressed(gpu.MouseButtonLeft) {
		g.push(engine.MouseKey(engine.MouseLeft, cx, cy))
	}
	if gpu.IsMouseButtonJustPressed(gpu.MouseButtonRight) {
		g.push(engine.MouseKey(engine.MouseRight, 0, 0))
	}
	_, dy := gpu.Wheel()
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
	if p.W <= 0 && g.remastered {
		return g.remaster.Cell(int(x), int(y))
	}
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
	b      gpu.StandardGamepadButton
	key    uint16
	repeat bool
}{
	{gpu.StandardGamepadButtonLeftTop, engine.KeyPadUp, true},
	{gpu.StandardGamepadButtonLeftBottom, engine.KeyPadDown, true},
	{gpu.StandardGamepadButtonLeftLeft, engine.KeyPadLeft, true},
	{gpu.StandardGamepadButtonLeftRight, engine.KeyPadRight, true},
	{gpu.StandardGamepadButtonRightBottom, engine.KeyPadSelect, false},
	{gpu.StandardGamepadButtonRightRight, 27, false},
	{gpu.StandardGamepadButtonRightLeft, engine.KeyPadOverview, false},
	{gpu.StandardGamepadButtonRightTop, engine.KeyPadMinimap, false},
	{gpu.StandardGamepadButtonFrontTopLeft, ',', true},
	{gpu.StandardGamepadButtonFrontTopRight, '.', true},
	{gpu.StandardGamepadButtonFrontBottomLeft, 0x4700, true},
	{gpu.StandardGamepadButtonFrontBottomRight, 0x4F00, true},
	{gpu.StandardGamepadButtonCenterRight, 13, false},
	{gpu.StandardGamepadButtonCenterLeft, 27, false},
}

// Held buttons repeat after padDelay ticks (1/tps s), every padRate ticks.
const padDelay, padRate = tps / 3, tps * 3 / 20

func repeats(d int) bool { return d == 1 || d >= padDelay && (d-padDelay)%padRate == 0 }

func (g *game) gamepad(id gpu.GamepadID) {
	if !gpu.IsStandardGamepadLayoutAvailable(id) {
		return
	}
	for _, p := range padButtons {
		d := gpu.StandardGamepadButtonPressDuration(id, p.b)
		if d == 1 || p.repeat && d > 1 && repeats(d) {
			g.push(p.key)
		}
	}
	// Left stick as a D-pad.
	x := gpu.StandardGamepadAxisValue(id, gpu.StandardGamepadAxisLeftStickHorizontal)
	y := gpu.StandardGamepadAxisValue(id, gpu.StandardGamepadAxisLeftStickVertical)
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

func (g *game) Draw(dst *gpu.Image) {
	text, at := g.eng.Note()
	note := classic.Note{Text: text, Show: time.Since(at) < 2*time.Second, Glyph: g.eng.Glyph}
	if g.remaster.Inventory(dst, g.eng.View()) { // the inventory screen, in both modes
		g.pic = classic.Rect{}
	} else if g.remastered {
		g.pic = g.remaster.Draw(dst, g.scr, note, g.eng.View())
	} else {
		g.pic = g.classic.Draw(dst, g.scr, note)
		mx, my := gpu.CursorPosition()
		g.classic.DrawBattle(dst, g.pic, g.eng.View().Battle, mx, my) // the remaster's targeting previews over the map
	}
	if shot != "" && g.ticks > 30*(len(shotKeys)+4) {
		saveShot(dst)
	}
}

// POR_SHOT=FILE.png writes the window to FILE after the keys in POR_KEYS,
// one every quarter second, and exits; for full size checks of the
// presenters. The window opens unfocused and ignores the user's input
// meanwhile. POR_DEV names a dev scene (engine.DevScene) to start at the
// first adventuring prompt. POR_KEYS escapes: \r Enter, \e Escape, \U \D \L \R the
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

func saveShot(dst *gpu.Image) {
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
	g.dsf = gpu.Monitor().DeviceScaleFactor()
	return int(float64(w) * g.dsf), int(float64(h) * g.dsf)
}

// biosKey maps keys the DOS game read as non-ASCII (scan code in the high
// byte, zero low byte) exactly like INT 16h returned them.
func biosKey(k gpu.Key) (uint16, bool) {
	switch k {
	case gpu.KeyEnter, gpu.KeyKPEnter:
		return 13, true
	case gpu.KeyEscape:
		return 27, true
	case gpu.KeyBackspace:
		return 8, true
	case gpu.KeyTab:
		return 9, true
	case gpu.KeyF1, gpu.KeyF2, gpu.KeyF3, gpu.KeyF4, gpu.KeyF5, gpu.KeyF6, gpu.KeyF7, gpu.KeyF8:
		return engine.KeyF1 + uint16(k-gpu.KeyF1)<<8, true
	case gpu.KeyF9:
		return engine.KeyF9, true
	case gpu.KeyArrowUp, gpu.KeyKP8:
		return 0x4800, true
	case gpu.KeyArrowDown, gpu.KeyKP2:
		return 0x5000, true
	case gpu.KeyArrowLeft, gpu.KeyKP4:
		return 0x4B00, true
	case gpu.KeyArrowRight, gpu.KeyKP6:
		return 0x4D00, true
	case gpu.KeyHome, gpu.KeyKP7:
		return 0x4700, true
	case gpu.KeyEnd, gpu.KeyKP1:
		return 0x4F00, true
	case gpu.KeyPageUp, gpu.KeyKP9:
		return 0x4900, true
	case gpu.KeyPageDown, gpu.KeyKP3:
		return 0x5100, true
	case gpu.KeyKP5:
		return 0x4C00, true
	}
	return 0, false
}

var speaker, voice *oto.Player // kept from the garbage collector

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
	if _, err := os.Stat(filepath.Join("data", "TITLE.DAX")); *dir == "" && err == nil {
		*dir = "data" // goplay: started in games/por with no arguments
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
	blobber.Voice = eng.ToggleVoice // the voice-over switch on the Esc options page
	if actx, ready, err := oto.NewContext(&oto.NewContextOptions{SampleRate: 44100, ChannelCount: 2, Format: oto.FormatSignedInt16LE}); err == nil {
		go func() {
			<-ready // in the browser: after the first click or key
			speaker = actx.NewPlayer(eng.SoundReader())
			// at least the 2048 frames (46 ms) a browser pulls at once, or the
			// rest of each pull plays as silence (goplay)
			speaker.SetBufferSize(44100 * 4 / 20) // 50 ms of 16-bit stereo
			speaker.Play()
			voice = actx.NewPlayer(eng.VoiceReader())
			voice.SetBufferSize(44100 * 4 / 20)
			voice.Play()
		}()
	}
	eng.Cfg.Param = flag.Arg(0)
	if d := os.Getenv("POR_DEV"); d != "" { // POR_DEV=combat: a dev scene at the first prompt, for captures
		eng.DevAction = eng.DevScene(d)
	}
	if n, err := strconv.ParseUint(os.Getenv("POR_SEED"), 10, 32); err == nil { // POR_SEED=1: the same dice every run, for comparing captures
		seed := uint32(n)
		eng.FixedSeed = &seed
	}
	go eng.Run()

	g := &game{scr: scr, eng: eng, dsf: 1, remastered: os.Getenv("POR_REMASTER") != ""}
	g.remaster.Classic = &g.classic
	gpu.SetWindowSize(ega.W*(*scale), ega.H*(*scale)*6/5) // 4:3 like a CRT
	gpu.SetWindowResizingMode(gpu.WindowResizingModeEnabled)
	gpu.SetWindowTitle("Pool of Radiance")
	gpu.SetTPS(tps)
	gpu.SetFullscreen(os.Getenv("GOPLAY_FULLSCREEN") == "1") // goplay starts games fullscreen
	if shot != "" {
		gpu.SetRunnableOnUnfocused(true)
	}
	if err := gpu.RunGameWithOptions(g, &gpu.RunGameOptions{InitUnfocused: shot != ""}); err != nil {
		log.Fatal(err)
	}
}
