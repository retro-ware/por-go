// Command por runs Pool of Radiance on top of the original data files.
//
//	por [-scale N]
//
// Gamepad (standard layout): D-pad or left stick move and turn, A picks the
// highlighted menu item, B is Escape, LB/RB move the menu highlight, LT/RT
// select the previous/next party member, X shows the party overview, Y
// toggles the minimap, Start is Enter.
//
// Mouse: pointing at a menu item highlights it, a click picks it (and list
// lines, party members, combat squares); the wheel moves the highlight and
// the right button is Escape. "?" lists the keys; F1-F8 and a card's left
// click select party member 1-8 and open their context menu, a right click
// on a card opens their inventory.
//
// The DOS game files (TITLE.DAX, 8X8D1.DAX, GAME data ...) are read from
// data/ in the working directory; gameimport (shipped beside por, driven by
// import.toml) puts them there. With -import SRC -data DIR (goplay setup) the
// install found under SRC is copied into DIR with a COVER.PNG, and the
// program exits.
// The Windows exe's icon: icon.ico (the Amiga title picture's hero and
// dragon, columns 70..270 of data/amiga/TITLE.PNG scaled to 256, 48, 32 and
// 16) linked in through rsrc_windows_amd64.syso, which tools/winicon writes:
//
//go:generate go run ../../tools/winicon -ico icon.ico -o rsrc_windows_amd64.syso
package main

import (
	"blobber"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"gpu"

	"github.com/ebitengine/oto/v3"

	"por/engine"
	"por/pal"
	"por/present/remaster"
)

type game struct {
	eng      *engine.Engine
	remaster remaster.Presenter

	pads    []gpu.GamepadID // input buffers, reused each tick
	pressed []gpu.Key
	chars   []rune

	stickDir  uint16 // left stick direction being held
	stickHeld int    // ticks it has been held

	dsf       float64       // device scale factor of the last Layout
	pic       remaster.Rect // where the picture is drawn, in device pixels
	wheel     float64       // wheel movement not yet sent as a step
	sent      bool          // input went to the engine this tick
	ticks     int           // Update calls, counted only for POR_SHOT
	frames    int           // frames saved, POR_SHOT with %d
	rightOnce bool          // POR_KEYS \M: the right button pressed this tick
	leftOnce  bool          // POR_KEYS \C: the left button
}

// tps: Update runs 120 times a second, so a key waits at most ~8 ms before
// the engine sees it.
const tps = 120

func (g *game) Update() error {
	g.sent = false
	if scripted { // the user's input never reaches a capture or a measurement, only the scripted keys (script, from Draw)
		if os.Getenv("POR_POINTER") != "" {
			g.mouse()
		}
		return nil
	}
	if g.remaster.Options() { // the renderer's options page has the keyboard
		g.remaster.UpdateOptions()
		if g.remaster.Options() {
			mx, my := gpu.CursorPosition()
			_, dy := gpu.Wheel()
			g.remaster.OptionsMouse(mx, my, gpu.IsMouseButtonJustPressed(gpu.MouseButtonLeft), gpu.IsMouseButtonJustPressed(gpu.MouseButtonRight), dy)
		}
		return nil
	}
	g.keys()
	for _, id := range gpu.AppendGamepadIDs(g.pads[:0]) {
		g.gamepad(id)
	}
	g.mouse()
	for _, f := range gpu.AppendDroppedFiles(nil) {
		cx, cy, in := g.cell(float64(f.X), float64(f.Y))
		g.remaster.Drop(f.Path, cx, cy, in, g.eng.View())
	}
	g.act()
	if g.sent {
		g.settle()
	}
	return nil
}

// script is a scripted run's clock, run by Draw once a frame, so a capture is
// the same frame for frame run to run (ticks and frames interleave differently
// each run): the renderer ticks once a frame, a key of POR_KEYS goes in every
// scriptTicks frames, and with POR_FREEZE time stands still after the keys (an
// outside capture of the frame).
func (g *game) script() {
	if freeze != "" && g.ticks > scriptTicks*(len(shotKeys)+4) {
		return
	}
	g.ticks++
	g.remaster.Tick()
	if k := g.ticks / scriptTicks; g.ticks%scriptTicks == 0 && k <= len(shotKeys) {
		switch c := shotKeys[k-1]; c {
		case shotRight, shotLeft: // \M, \C: a click at the pointer
			g.rightOnce, g.leftOnce = c == shotRight, c == shotLeft
		case shotNext: // \P: the pointer moves on to the next of POR_POINTER's x,y;x,y...
			shotPointer++
		case shotWait: // \w
		default:
			g.push(shotKeys[k-1])
			g.act()
			g.settle()
		}
	}
}

const scriptTicks = 15 // frames between a script's keys: a quarter second at 60 Hz

// push hands a window key to the presenter, which makes the actions.
func (g *game) push(k uint16) { g.remaster.Key(k, g.eng.View()) }

// act sends the screens' actions to the game.
func (g *game) act() {
	for _, a := range g.remaster.Actions() {
		g.eng.Act(a)
		g.sent = true
	}
}

// settle gives the engine a moment to take the actions sent, so the frame
// shows their result.
func (g *game) settle() {
	t0 := time.Now()
	for n := 0; !g.eng.Settled() && time.Since(t0) < 8*time.Millisecond; n++ {
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
	// fluid movement: the view asks for the held move as the last one ends.
	// A key counts as held only after holdTicks: a tap (even one a little
	// longer than half a turn) is one move, never two
	var held uint16
	for _, k := range repeatKeys[:8] {
		if gpu.KeyPressDuration(k) >= holdTicks {
			held, _ = biosKey(k)
		}
	}
	var send bool
	send, paced = g.remaster.Flow(held, g.eng.Idle())
	if send {
		g.push(held)
	}
	for i, k := range repeatKeys {
		if i < 8 && paced {
			continue
		}
		if d := gpu.KeyPressDuration(k); d > 1 && repeats(d) && g.eng.Idle() {
			code, _ := biosKey(k)
			g.push(code)
		}
	}
	for _, k := range gpu.AppendJustPressedKeys(g.pressed[:0]) {
		if code, ok := biosKey(k); ok {
			g.push(code)
		} else if k == gpu.KeyF && gpu.IsKeyPressed(gpu.KeyControl) && gpu.IsKeyPressed(gpu.KeyShift) {
			g.push(remaster.KeyFontPrev) // Ctrl-F next font, Ctrl-Shift-F the one before
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
	if pts := strings.Split(os.Getenv("POR_POINTER"), ";"); shot != "" && pts[0] != "" { // POR_SHOT: the pointer at x,y (screen pixels)
		fmt.Sscanf(pts[min(shotPointer, len(pts)-1)], "%d,%d", &mx, &my)
	}
	if g.remaster.InventoryShown() { // its own clicks and wheel; keys still go to the game (a capture's \C is down for one tick: a click)
		_, dy := gpu.Wheel()
		g.remaster.InventoryMouse(mx, my, gpu.IsMouseButtonPressed(gpu.MouseButtonLeft) || g.leftOnce, gpu.IsMouseButtonJustPressed(gpu.MouseButtonRight) || g.rightOnce, dy)
		g.leftOnce, g.rightOnce = false, false
		return
	}
	g.remaster.Hover(mx, my, gpu.IsMouseButtonPressed(gpu.MouseButtonLeft))
	left := gpu.IsMouseButtonJustPressed(gpu.MouseButtonLeft) || g.leftOnce
	g.leftOnce = false
	if left && g.remaster.Click(mx, my) { // an action bar button, a menu item, the combat map
		return
	}
	cx, cy, in := g.cell(float64(mx), float64(my))
	_, dy := gpu.Wheel()
	g.wheel += dy
	steps := int(g.wheel)
	g.wheel -= float64(steps)
	right := gpu.IsMouseButtonJustPressed(gpu.MouseButtonRight) || g.rightOnce
	g.rightOnce = false
	g.remaster.Mouse(cx, cy, in, left, right, steps, g.eng.View())
}

func (g *game) cell(x, y float64) (int, int, bool) {
	p := g.pic
	if p.W <= 0 {
		return g.remaster.Cell(int(x), int(y))
	}
	if p.W <= 0 || x < p.X || y < p.Y || x >= p.X+p.W || y >= p.Y+p.H {
		return 0, 0, false
	}
	v := g.eng.View()
	cols, rows := 40, 25
	if len(v.Text) > 0 {
		cols, rows = len(v.Text[0]), len(v.Text)
	}
	return int((x - p.X) * float64(cols) / p.W), int((y - p.Y) * float64(rows) / p.H), true
}

// padButtons maps standard-layout gamepad buttons to BIOS key codes (and
// the engine's virtual gamepad keys).
var padButtons = []struct {
	b      gpu.StandardGamepadButton
	key    uint16
	repeat bool
}{
	{gpu.StandardGamepadButtonLeftTop, 0x4800, true},
	{gpu.StandardGamepadButtonLeftBottom, 0x5000, true},
	{gpu.StandardGamepadButtonLeftLeft, 0x4B00, true},
	{gpu.StandardGamepadButtonLeftRight, 0x4D00, true},
	{gpu.StandardGamepadButtonRightBottom, remaster.KeyPadSelect, false},
	{gpu.StandardGamepadButtonRightRight, 27, false},
	{gpu.StandardGamepadButtonRightLeft, remaster.KeyPadOverview, false},
	{gpu.StandardGamepadButtonRightTop, remaster.KeyPadMinimap, false},
	{gpu.StandardGamepadButtonFrontTopLeft, ',', true},
	{gpu.StandardGamepadButtonFrontTopRight, '.', true},
	{gpu.StandardGamepadButtonFrontBottomLeft, 0x4700, true},
	{gpu.StandardGamepadButtonFrontBottomRight, 0x4F00, true},
	{gpu.StandardGamepadButtonCenterRight, 13, false},
	{gpu.StandardGamepadButtonCenterLeft, 27, false},
}

// Held buttons repeat after padDelay ticks (1/tps s), every padRate ticks;
// a move key held holdTicks (200 ms) flows on, a shorter press is a tap.
const padDelay, padRate, holdTicks = tps / 3, tps * 3 / 20, tps / 5

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
		dir = 0x4800
	case y > 0.6:
		dir = 0x5000
	case x < -0.6:
		dir = 0x4B00
	case x > 0.6:
		dir = 0x4D00
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
	if scripted {
		g.script()
	}
	text, at := g.eng.Note()
	note := remaster.Note{Text: text, Show: time.Since(at) < time.Second+time.Duration(len(text))*80*time.Millisecond, Glyph: g.eng.Glyph}
	if g.remaster.Inventory(dst, note, g.eng.View()) { // the inventory screen
		g.pic = remaster.Rect{}
	} else {
		g.pic = g.remaster.Draw(dst, note, g.eng.View())
	}
	if shot == "" {
		if quit != "" && g.ticks > scriptTicks*(len(shotKeys)+4) { // POR_QUIT=1: the run ends after the keys (measuring)
			os.Exit(0)
		}
		return
	}
	if strings.Contains(shot, "%d") { // every frame while the keys play, numbered (to catch a glitch of one frame)
		if g.ticks > scriptTicks && g.ticks <= scriptTicks*(len(shotKeys)+1) {
			saveShot(dst, fmt.Sprintf(shot, g.frames))
			g.frames++
		}
	} else if g.ticks > scriptTicks*(len(shotKeys)+4) {
		saveShot(dst, shot)
	}
	if g.ticks > scriptTicks*(len(shotKeys)+4) {
		os.Exit(0)
	}
}

// parseSize reads "WxH".
func parseSize(s string) (w, h int, err error) {
	_, err = fmt.Sscanf(s, "%dx%d", &w, &h)
	return
}

// POR_SHOT=FILE.png writes the window to FILE after the keys in POR_KEYS,
// one every quarter second, and exits; for full size checks of the
// presenters. A FILE with %d in it is written for every frame while the
// keys play, numbered from 0 (a glitch of one frame). The window opens unfocused and ignores the user's input
// meanwhile. POR_DEV names a dev scene (engine.DevScene) to start at the
// first adventuring prompt. POR_POINTER=x,y puts the pointer there. POR_KEYS escapes: \r Enter, \e Escape, \U \D \L \R the
// arrows, \1..\8 F1..F8, \M and \C a right and a left click at the
// pointer, \P the pointer on to the next of POR_POINTER's x,y;x,y... positions, \w a quarter second of nothing (a
// long capture: the FPS of a view).
var shot, shotKeys = os.Getenv("POR_SHOT"), scriptKeys(os.Getenv("POR_KEYS"))
var freeze = os.Getenv("POR_FREEZE")
var quit = os.Getenv("POR_QUIT")
var scripted = shot != "" || freeze != "" || quit != ""

const shotRight, shotLeft, shotNext, shotWait = 0xFF01, 0xFF02, 0xFF03, 0xFF04 // scriptKeys' codes for \M, \C, \P, \w

var shotPointer int // the position of POR_POINTER in use

func scriptKeys(s string) []uint16 {
	esc := map[byte]uint16{'r': 13, 'e': 27, 'U': 0x4800, 'D': 0x5000, 'L': 0x4B00, 'R': 0x4D00, 'M': shotRight, 'C': shotLeft, 'P': shotNext, 'w': shotWait}
	for n := range 8 { // \1..\8: F1..F8
		esc['1'+byte(n)] = remaster.KeyF1 + uint16(n)<<8
	}
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

func saveShot(dst *gpu.Image, name string) {
	b := dst.Bounds()
	img := image.NewRGBA(b)
	dst.ReadPixels(img.Pix)
	f, err := os.Create(name)
	if err == nil {
		err = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(f, img)
		f.Close()
	}
	if err != nil {
		log.Fatal(err)
	}
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
		return remaster.KeyF1 + uint16(k-gpu.KeyF1)<<8, true
	case gpu.KeyF9:
		return remaster.KeyF9, true
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
	dir := flag.String("data", "", "where -import and -import-disk2 write (goplay)")
	scale := flag.Int("scale", 3, "window scale (with -window)")
	window := flag.Bool("window", false, "a window instead of the full screen (the Esc menu's setting is left as it is)")
	imp := flag.String("import", "", "copy the game found under this directory into -data, then exit (goplay)")
	imp2 := flag.String("import-disk2", "", "convert the Amiga graphics from the .adf disks under this directory into -data/amiga, then exit (goplay)")
	explore := flag.Bool("explore", false, "debug: skip the intro, Quick Start a party if there is none and begin adventuring at once")
	battle := flag.Bool("battle", false, "debug: a fight the computer plays out for both sides at the first prompt (with -explore: at once), for measuring the battle view")
	setup := flag.Bool("setup", false, "one-time setup after the game was imported: write the area scripts into data/assets/scripts and data/COVER.PNG from data/dos/TITLE.DAX, then exit (gameimport)")
	flag.Parse()
	if p := os.Getenv("POR_CPUPROFILE"); p != "" { // slowness: a CPU profile of the first 20 seconds, for go tool pprof
		if f, err := os.Create(p); err == nil {
			pprof.StartCPUProfile(f)
			time.AfterFunc(20*time.Second, func() { pprof.StopCPUProfile(); f.Close() })
		}
	}
	if n, err := strconv.Atoi(os.Getenv("POR_STACKS")); err == nil { // a hang: every goroutine to stderr every n seconds
		go func() {
			for range time.Tick(time.Duration(n) * time.Second) {
				buf := make([]byte, 1<<20)
				fmt.Fprintf(os.Stderr, "--- stacks %s\n", time.Now().Format(time.TimeOnly))
				os.Stderr.Write(buf[:runtime.Stack(buf, true)])
			}
		}()
	}
	if *setup {
		if err := setupData("data"); err != nil {
			log.Fatal(err)
		}
		return
	}
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
	if _, err := os.Stat(filepath.Join("data", engine.DOSDir, "TITLE.DAX")); err != nil {
		wd, _ := os.Getwd()
		fmt.Fprintf(os.Stderr, "No Pool of Radiance data in %s.\nRun gameimport, or copy the DOS game files (TITLE.DAX, 8X8D1.DAX, ...) there yourself.\n", filepath.Join(wd, "data", engine.DOSDir))
		os.Exit(1)
	}
	eng, err := engine.New("data")
	if err != nil {
		log.Fatal(err)
	}
	blobber.Voice = eng.ToggleVoice                                 // the voice-over switch on the Esc menu's Audio page
	settingsFile := filepath.Join(eng.Cfg.SaveDir, "SETTINGS.JSON") // the Esc menu's options (minimap, picture box, speed, fullscreen ...) kept across runs
	if b, err := os.ReadFile(settingsFile); err == nil {
		json.Unmarshal(b, &blobber.Settings)
	}
	eng.Deskew.Store(blobber.Settings.Deskew)
	engine.SetVolumes(blobber.Settings.Music, blobber.Settings.Effects, blobber.Settings.Voice)
	blobber.OnSettings = func(o blobber.Options) {
		eng.Deskew.Store(o.Deskew)
		engine.SetVolumes(o.Music, o.Effects, o.Voice)
		b, _ := json.MarshalIndent(o, "", " ")
		os.MkdirAll(eng.Cfg.SaveDir, 0o755)
		os.WriteFile(settingsFile, b, 0o644)
	}
	blobber.OnSaveLoad = func() { eng.Act(engine.Action{Kind: engine.ActSlots}) } // as F9
	blobber.OnMainMenu = eng.MainMenu
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
	if *explore {
		eng.Explore, eng.Cfg.FullIntro = true, false
	}
	if d := os.Getenv("POR_DEV"); d != "" { // POR_DEV=combat: a dev scene at the first prompt, for captures
		eng.DevAction = eng.DevScene(d)
	}
	if *battle {
		eng.DevAction = eng.DevScene("autocombat")
	}
	if n, err := strconv.ParseUint(os.Getenv("POR_SEED"), 10, 32); err == nil { // POR_SEED=1: the same dice every run, for comparing captures
		seed := uint32(n)
		eng.FixedSeed = &seed
	}
	go eng.Run()

	g := &game{eng: eng, dsf: 1}
	remaster.Scripted = scripted
	gpu.SetWindowSize(pal.W*(*scale), pal.H*(*scale))                // the title pictures' shape (16:10): they fill the window whole, uncropped
	gpu.SetWindowAspect(pal.W, pal.H)                                // and a resize keeps it so; the full screen has its own shape, the UI pages lay out to it (ActResize) and the pictures get black bars
	if w, h, err := parseSize(os.Getenv("POR_WINDOW")); err == nil { // POR_WINDOW=WxH (points): a window of that size and shape, for captures of other shapes
		gpu.SetWindowSize(w, h)
		gpu.SetWindowAspect(0, 0)
	}
	gpu.SetWindowResizingMode(gpu.WindowResizingModeEnabled)
	gpu.SetWindowTitle("Pool of Radiance")
	gpu.SetTPS(tps)
	gpu.SetFullscreen((os.Getenv("GOPLAY_FULLSCREEN") == "1" || blobber.Settings.Fullscreen) && !*window) // the screen at its own resolution, unless the Esc menu or -window says a window
	if shot != "" {
		gpu.SetRunnableOnUnfocused(true)
	}
	if err := gpu.RunGameWithOptions(g, &gpu.RunGameOptions{InitUnfocused: shot != ""}); err != nil {
		log.Fatal(err)
	}
}
