// Package engine is the Pool of Radiance game engine, ported routine by
// routine from the DOS START.EXE/GAME.OVR disassembly. It reads every asset
// from the original data directory at run time, exactly like the DOS build.
package engine

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"por/dax"
	"por/ega"
)

// Config mirrors POOL.CFG: graphics letter, sound letter, game dir, save dir,
// and the 5th line ('F' = full intro with second title picture + credits).
type Config struct {
	Graphics, Sound  byte
	GameDir, SaveDir string
	FullIntro        bool   // DS:49D1
	Param            string // ParamStr(1): "STING" unlocks the 'J' debug key
}

// Engine holds the global state the DOS program kept in its data segment.
type Engine struct {
	L         *dax.Loader
	S         *ega.Screen
	Cfg       Config
	tempSel   *Character             // selectTemp: selected but not in the party
	page      [pageStride * 168]byte // DS:4A20 drawing page contents (pageSet)
	pageDraw  bool                   // drawing the 3D view: the original draws into page DS:4A20, one cell up and left of the screen
	flips     int                    // traced Flips
	RandSeed  uint32                 // DS:4428, Turbo Pascal RandSeed
	FixedSeed *uint32                // if set, Randomize uses it instead of the clock (trace runs)
	Trace     TraceFunc              // nil in normal play; see trace.go
	tr        *traceState            // trace mode clock and key script

	font  []byte        // DS:5E20 -> 8X8D1 block 201, 8 bytes per glyph
	sym   [5]*dax.Image // DS:5E3A symbol sets
	set   Set           // DS:5376: the data set the current area reads
	start time.Time

	pendingScan byte // ReadKey: scan code waiting after a 0 byte
	draining    bool // ReadKeyByte is dropping typed-ahead keys
	msgDelay    byte // DS:49E2, message delay factor (ovr12 sets 1)
	menuSel     int  // DS:6B2B, DisplayInput selection
	menuExtra   bool // DS:6B2C

	party      []*Character // DS:5D96 list head ...
	partySel   int          // DS:5D92 selected character
	rosterHi   *Character   // SelectCharacter: 00BA:0767 highlights this one (nil: partySel)
	rosterNone bool         // ... or no one (who = nil)
	beginFlag  byte         // DS:49F3
	loadedGame bool         // DS:49F8
	saved      bool         // DS:557D
	debug      bool         // DS:470C

	game               gameVars // DS:49D6 record (vars.go)
	area               areaVars // DS:49D2
	aux                auxVars  // DS:49DA
	ecl                []byte   // DS:49DE
	posX, posY, facing int      // DS:6AAD, 6AAE, 6AAF
	eclPC              int      // DS:49ED
	viewMode           int      // DS:49FA (1 = 3D view)
	prevViewMode       int      // DS:49F9
	sqrpaci            [2]*dax.Image
	items              []byte // DS:5582: 16-byte base item records
	pos3, pos4         byte   // DS:6AB0, 6AB1
	viewBit0, viewBit1 byte   // DS:49F5/49F6
	mapPairA, mapPairB [3]int // DS:6AAE+4i / 6AB0+4i
	// viewAnim = area+0x3FE: nonzero cycles the view picture on every poll
	// (ovr26:03BD), cleared by displayInput (ovr26:06D8). No script sets it;
	// in the original only save/load wrote it, packing the last wall slot
	// there (the lettered LoadGame/SaveGame of traced runs still do: oldWalls).
	viewAnim uint16
	// fileWallsA/B = area+0x3F4..0x3FD as last saved or loaded: the original
	// keeps them in the area record, so the trace's area dump shows them.
	fileWallsA, fileWallsB [3]uint16
	loadingMonster         bool // DS:557C

	es                   eclState            // ovr08/ovr04 interpreter state
	msgRow, msgCol       int                 // DS:5E25 / 5E24 text-block cursor
	slowText             bool                // DS:4A00: DisplayTextBlock types out
	listTop              int                 // DS:6B29 ovr26 list menu: first visible row
	gs                   geoState            // ovr30 map, wall sets and view
	run                  eclRun              // ovr04 globals
	pic                  animPic             // DS:6ABE view-window picture
	sprCursor            bool                // e.spr is the aim cursor's icon (iconToSprite slot 0x19)
	port                 portrait            // DS:6B00.. HEAD/BODY portrait
	eclFile              string              // DS:547C
	ext                  extras              // convenience features (extras.go)
	snap                 snapState           // the View published for other presenters (snapshot.go)
	ac                   afterCombat         // ovr06 globals
	camp                 campState           // ovr16/ovr21 globals
	purse                purse               // the party's money
	loot                 purse               // DS:67F4 money found, taken after the fight
	animInput            bool                // animate the view picture while waiting for a key
	resting              byte                // DS:6DDA (ovr21 rest): messages skip the box border
	view                 viewState           // ovr20 View screen
	itemsRedraw          bool                // DS:6821
	usingItem            bool                // DS:6DBF
	usedScroll           *Item               // DS:6B80
	spellMenuKey         byte                // (extra) the key that closed the last spell menu
	rv                   ruleVars            // DS:6814..682A rule globals
	near                 []nearEntry         // DS:6716 combatants near a square
	nearList             []nearEntry         // DS:671B copy kept by checkAffect
	combatScreenX        [0x48]int8          // DS:604A
	combatScreenY        [0x48]int8          // DS:6092
	combatants           []*Character        // DS:65BD, combatant i (1-based) at [i-1]; filled by combat setup
	targets              [0x48]byte          // DS:6DE4, near list indexes kept by nearEnemies
	cmap                 *combatMap          // DS:6716
	cpos                 [0x49]combatPos     // DS:5F27, combatant i at [i]
	ncomb                int                 // DS:5F2A combatants on the map list
	occ                  [25][50]byte        // DS:60DB combatant index per square
	bodies               [10]body            // DS:66D6, 1-based, count DS:6715
	nbodies              int                 // DS:6715
	bodyIndex            byte                // DS:70C7
	icons                [0x48][2]*dax.Image // DS:5E5A, ready and action pose per slot
	spr                  *dax.Image          // DS:6E36, 4 frame missile/hit sprite
	sprUnder             []byte              // DS:4A24, background saved by a sprite draw
	place                placement           // DS:4440..4658 combat start formations
	terrain              byte                // DS:465A wilderness square of the battle
	clouds               []*cloud            // DS:6DBB lingering spell areas
	magicOn, combatRound byte                // DS:6E30/6DE3
	thrownCopy           *Item               // DS:5D9A
	noContinue           byte                // DS:49F4
	debugShow            byte                // DS:49F7 debug display (never set)

	// combat overlays (ovr10/13/14/23)
	spellTarget                     *Character       // DS:6C95 target slot 1
	spellList                       []*ListNode      // DS:4766 spell menu lines (ovr23)
	spellIDs                        []byte           // DS:476A spell of each non-header line
	spellItems                      [0x31]*Item      // DS:477F scroll of each line
	spellItemCount                  int              // DS:4843
	spellTargets                    [0x49]*Character // DS:6C91+4i targets 2..; slot 1 is spellTarget
	nSpellTargets                   int              // DS:6C94
	spellCaster                     *Character       // DS:6DB5
	curSpell                        byte             // DS:681B spell being cast
	spellOwnTargets                 byte             // DS:6820
	tgtX, tgtY                      int              // DS:6DB9/6DBA target square
	combatTargets                   bool             // DS:6B84 points at ovr14:20C3 (set by Combat), else ovr23:0A91
	whoVar                          bool             // SelectCharacter's who is DS:5D92 itself (opWho)
	spellFx                         [0x44]func()     // DS:6B84 spell effect table (slot 0 unused)
	spellValue, spellValue2         byte             // DS:4844/4845 spell scratch; DS:4845
	turnValue, modMoves, affectVeto byte             // DS:681A speed value, 681D, 681E affect scratch; DS:681D/681E
	fleeRoll                        byte             // DS:6825 flee/surrender roll
	slotHits, slotShots             [3]byte          // DS:6822+i / 6E2C+i, i = 1..2 attack slots; DS:6E2C
	aiLastDir, aiBlocked, aiState3  byte             // DS:443C..443E computer-turn movement state; DS:443D/443E
	attackedAlly                    byte             // DS:6DE2
	tiedTarget                      *Character       // DS:6826 scratch target
	foesHealth                      byte             // DS:6E2F

	// H
	areaIconTick byte  // DS:6DDC area map party icon pose counter
	snd          sound // PC speaker driver (08AE)

	DevAction func() // run once at the next adventuring command prompt (cmd/snap)
}

// Dice = ovr25:0DE3 Dice(n, sides): sum of n rolls of 1..sides, kept in a
// byte like the original (wraps at 256).
func (e *Engine) Dice(n, sides int) int {
	t := 0
	for i := 0; i < n; i++ {
		t = (t + e.Random(sides) + 1) & 0xFF
	}
	return t
}

// New opens the data directory and reads POOL.CFG.
func New(dir string, scr *ega.Screen) (*Engine, error) {
	l, err := dax.NewLoader(dir)
	if err != nil {
		return nil, err
	}
	e := &Engine{L: l, S: scr, set: 1, start: time.Now()}
	e.Cfg = Config{Graphics: 'E', Sound: 'P', GameDir: dir, SaveDir: filepath.Join(dir, "SAVE"), FullIntro: true}
	if p := l.Path("POOL.CFG"); p != "" {
		f, err := os.Open(p)
		if err == nil {
			defer f.Close()
			var lines []string
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				lines = append(lines, strings.TrimSpace(sc.Text()))
			}
			if len(lines) >= 5 {
				e.Cfg.Graphics, e.Cfg.Sound = upper(lines[0][0]), upper(lines[1][0])
				e.Cfg.FullIntro = upper(lines[4][0]) == 'F' // main 0x461F: cmp [484C],'F'
			}
		}
	}
	if !l.Exists("TITLE.DAX") || !l.Exists("8X8D1.DAX") {
		return nil, fmt.Errorf("%s: not a Pool of Radiance directory (TITLE.DAX / 8X8D1.DAX missing)", dir)
	}
	e.pic.block = -1
	e.port = portrait{headID: 0xFF, bodyID: 0xFF}
	e.loadFont()
	e.loadNotes()
	return e, nil
}

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

// Run is the program body (START.EXE main). It runs on its own goroutine and
// blocks on keyboard reads like the DOS code did.
func (e *Engine) Run() {
	e.Init()  // ovr12:002D
	e.Title() // ovr02: title pictures, lightning, credits
	// Copy protection (ovr03:00FD code wheel) deliberately skipped.
	if e.Trace != nil {
		e.Trace("ovr03:00FD", "CopyProtection")
	}
	e.adventure()
	e.Exit()
}

// Play is the rest of main after the title: party menu, then adventure.
func (e *Engine) Play() {
	if !slices.ContainsFunc(e.items, func(b byte) bool { return b != 0 }) {
		e.MessageRow24b(0xE, "ITEMS missing: import the game again") // extra (readItems): waits for a key
	}
	e.adventure()
}

// Exit = 0899:0000: back to DOS.
func (e *Engine) Exit() {
	if e.tr != nil {
		panic(TraceDone{"exit"})
	}
	os.Exit(0)
}

// Randomize = 0AF8:1247: RandSeed = DOS time (CX = hour:min low word,
// DX = sec:hundredths high word).
func (e *Engine) Randomize() {
	if e.Trace != nil {
		e.Trace("0AF8:1247", "Randomize")
	}
	if e.FixedSeed != nil {
		e.RandSeed = *e.FixedSeed
		return
	}
	t := time.Now()
	e.RandSeed = uint32(t.Hour())<<8 | uint32(t.Minute()) | (uint32(t.Second())<<8|uint32(t.Nanosecond()/1e7))<<16
}

// Random = Turbo Pascal Random(n) (0AF8:11C0): RandSeed = RandSeed*0x08088405+1,
// then the high word mod n. The seed advances even for n = 0, which returns 0.
func (e *Engine) Random(n int) int {
	e.RandSeed = e.RandSeed*0x08088405 + 1
	if uint16(n) == 0 {
		return 0
	}
	return int(uint16(e.RandSeed>>16) % uint16(n))
}

// Ticks = 0709:0C3F, time in hundredths of a second.
func (e *Engine) Ticks() int64 {
	if e.tr != nil { // virtual clock: 10 ms per call (emu prims.py _ticks)
		e.Trace("0709:0C3F", "Ticks")
		t := e.tr.ms / 10
		e.tr.ms += 10
		return t
	}
	return int64(time.Since(e.start) / (10 * time.Millisecond))
}

// Delay = 0A40:029C (Turbo Pascal Delay, milliseconds).
func (e *Engine) Delay(ms int) {
	if e.tr != nil {
		e.Trace("0A40:029C", "Delay", ms)
		e.tr.ms += int64(ms)
		return
	}
	if e.beginFlag == 5 { // the 3D combat view animates with the page: moves, missiles, hits
		e.publish()
	}
	e.S.SetState(ega.Pausing)
	ega.Delay(ms)
	e.S.SetState(ega.Working)
}

// KeyPressed / ReadKey / ClearKeyboard = 0A40:02FD, BIOS read, 0899:0104.
func (e *Engine) KeyPressed() bool {
	if e.tr != nil {
		return e.tr.keyPressed(e)
	}
	e.walkFeed()
	e.publish()
	for len(e.ext.ahead) == 0 && e.S.KeyPressed() { // nav.go: translate as the key is seen
		e.feed(e.S.ReadKey())
	}
	e.release()
	return len(e.ext.ahead) > 0
}

func (e *Engine) ReadKey() uint16 {
	if e.tr != nil {
		return e.tr.readKey(e)
	}
	e.ext.skip = false
	for e.release(); len(e.ext.ahead) == 0; e.release() {
		if e.ext.menu == 0 {
			e.showHint()
		}
		e.publish()
		e.feed(e.S.ReadKey())
	}
	k := e.ext.ahead[0]
	e.ext.ahead = e.ext.ahead[1:]
	return k
}

// feed translates a key from the window (nav.go) into the keys the game
// will read.
func (e *Engine) feed(raw uint16) {
	if k := e.translate(raw); k != 0 {
		e.ext.ahead = append(e.ext.ahead, k)
	}
}

// release hands the game the next of the keys a translated key left to
// follow it, once the keys before are read. Not while ReadKeyByte drains
// typed-ahead keys, which would keep only the last.
func (e *Engine) release() {
	if len(e.ext.ahead) == 0 && len(e.ext.later) > 0 && !e.draining {
		e.ext.ahead = append(e.ext.ahead, e.ext.later[0])
		e.ext.later = e.ext.later[1:]
	}
}

// The original's drain loop is dead code: 0899:0104 jumps straight to its exit.
func (e *Engine) ClearKeyboard() {}

// WaitSeconds = ovr02:0000 — wait n seconds or until a key is pressed, then
// flush the keyboard.
func (e *Engine) WaitSeconds(n int) {
	end := e.Ticks() + int64(n)*100
	for e.Ticks() < end && !e.keyWaiting() {
		e.idle()
	}
	e.ClearKeyboard()
}

// idle yields the CPU inside the original's busy-wait loops; it is not a
// call of the original, so it is untraced and costs no virtual time.
func (e *Engine) idle() {
	e.ext.skip = false
	if e.tr == nil {
		e.S.SetState(ega.Waiting)
		ega.Delay(1)
	}
}
