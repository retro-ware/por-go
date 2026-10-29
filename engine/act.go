package engine

import (
	"sync/atomic"
	"time"
)

// Actions (extra): the remastered screens act on the game through typed
// actions instead of keys. A screen ported to them (ItemsMenu ...) publishes
// what it shows in the View and waits in act; the presenter sends Act.

// ActKind is what an Action does.
type ActKind int

const (
	ActNone      ActKind = iota
	ActClose             // leave the screen
	ActMember            // show party member N
	ActReady             // inventory: ready or unready item N
	ActUse               // inventory: use item N
	ActThrow             // inventory (combat): throw item N
	ActDrop              // inventory: drop item N (the player already confirmed)
	ActSell              // inventory: sell item N (confirmed)
	ActWalk              // combat: walk to square X, Y (one of Battle.Reach)
	ActAttack            // combat: attack unit N (one of Battle.Targets; 0: aim first)
	ActGuard             // combat: end the turn on guard
	ActBandage           // combat: bandage the first dying member
	ActWait              // combat: Done's Delay
	ActEnd               // combat: Done's Quit: end the turn
	ActQuick             // combat: the computer fights for the member
	ActTurn              // combat: turn undead
	ActCast              // combat: cast spell N (one of Turn.Spells)
	ActItems             // combat: the inventory
	ActAim               // combat, aiming: take square X, Y
	ActPoint             // combat, aiming: the pointer is on square X, Y (the preview follows)
	ActYes               // combat: yes to Battle.Ask (ActClose: no)
	ActFlee              // combat: step off the map's edge and try to run away (Turn offers it at the edge)
	ActPick              // pick item N of View.Menu
	ActSelect            // highlight item N of View.Menu
	ActContinue          // Enter: the highlighted choice, or "press a key"
	ActText              // the line being typed (View.Typing) now reads Text
	ActMove              // a step or cursor move in direction N (Dir...)
	ActInventory         // member N's items (F1-F8)
	ActMinimap           // the minimap over the roster (Tab)
	ActMap               // the full map (M)
	ActOverview          // the party overview, on the View page the items (P)
	ActHelp              // the help pages (?)
	ActSlots             // the save slots (F9)
	ActFont              // the next text font (N -1: the one before)
	ActSound             // sound on or off (Ctrl-S)
	ActSpot              // a click on View.Spots[N] at cell X, Y
	ActHover             // the pointer came onto View.Spots[N] at cell X, Y
	ActSpells            // inventory: the member's memorised spells
	ActMagic             // inventory: the spell effects on the member
	ActHeal              // inventory: a paladin lays on hands
	ActCure              // inventory: a paladin cures disease
)

// Directions of ActMove.
const (
	DirUp      = iota // walk on, the line or item before
	DirDown           // a step back, the next line or item
	DirLeft           // turn left, the item before
	DirRight          // turn right, the next item
	DirHome           // the member or line before
	DirEnd            // the next member or line
	DirPgUp           // the page before
	DirPgDn           // the next page
	DirCentre         // keypad 5
	DirBack           // the item before (,)
	DirOn             // the next item (.)
	DirWheelUp        // the wheel: the highlight, line or page before
	DirWheelDown
)

// Action is one thing the player did on a remastered screen.
type Action struct {
	Kind ActKind
	N    int
	X, Y int    // a combat map square, a text cell (ActSpot)
	Text string // ActText
}

// acts carries the presenter's actions to the engine goroutine.
type acts struct {
	ch      chan Action
	pending atomic.Int32 // sent and not yet done
	taken   int32        // received by the engine goroutine, done once it waits again
	state   atomic.Int32 // what the engine goroutine does (pace)
}

// Engine states (pace): the window draws a key's result once the engine
// is done with it (Settled).
const (
	working = iota // handling input
	waiting        // waiting for input
	pausing        // a deliberate delay (Delay)
)

// pace records what the engine goroutine does now.
func (e *Engine) pace(st int32) { e.acts.state.Store(st) }

// Settled reports whether the engine has taken the keys and actions sent
// and finished with them, or pauses on purpose; the picture is worth drawing.
func (e *Engine) Settled() bool {
	st := e.acts.state.Load()
	return (st == pausing || st == waiting && len(e.acts.ch) == 0) && !e.Busy()
}

// Idle reports whether the engine waits for input and none is queued (held
// keys repeat only then, so they never pile up).
func (e *Engine) Idle() bool { return e.acts.state.Load() == waiting && len(e.acts.ch) == 0 }

// nextAct publishes the View and blocks for the next action.
func (e *Engine) nextAct() Action {
	e.publish()
	e.pace(waiting)
	a := <-e.acts.ch
	e.pace(working)
	e.acts.taken++
	return a
}

// sleep pauses the engine goroutine for ms milliseconds.
func sleep(ms int) {
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

// done counts the actions taken as finished: the engine waits again.
func (e *Engine) actsDone() {
	e.acts.pending.Add(-e.acts.taken)
	e.acts.taken = 0
}

// Act sends a to the screen waiting for it; safe from any goroutine.
func (e *Engine) Act(a Action) {
	e.acts.pending.Add(1)
	select {
	case e.acts.ch <- a:
	default:
		e.acts.pending.Add(-1) // a burst nobody reads: dropped like a typed-ahead key
	}
}

// Busy reports whether an action is not done yet (the window waits for its
// result before drawing, as for a key).
func (e *Engine) Busy() bool { return e.acts.pending.Load() > 0 }

// act publishes the View and waits for the next action. The extras (F9,
// font, help) still act; the screen sees every other action.
func (e *Engine) act() Action {
	e.actsDone()
	for {
		a := e.nextAct()
		if !e.extraAct(a) {
			return a
		}
	}
}

// say shows s as the window's note: what a direct action reports instead
// of the original's message line.
func (e *Engine) say(s string) { e.ext.note, e.ext.noteAt = s, time.Now() }

// ask (extra) puts the yes/no question q to the player as the View's
// Battle.Ask and waits for ActYes or ActClose.
func (e *Engine) ask(q string) bool {
	defer func() { e.snap.ask = "" }()
	for {
		e.snap.ask = q
		switch e.act().Kind {
		case ActYes:
			return true
		case ActClose:
			return false
		}
	}
}
