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
	ActNone    ActKind = iota
	ActClose           // leave the screen
	ActMember          // show party member N
	ActReady           // inventory: ready or unready item N
	ActUse             // inventory: use item N
	ActThrow           // inventory (combat): throw item N
	ActDrop            // inventory: drop item N (the player already confirmed)
	ActSell            // inventory: sell item N (confirmed)
	ActWalk            // combat: walk to square X, Y (one of Battle.Reach)
	ActAttack          // combat: attack unit N (one of Battle.Targets; 0: aim first)
	ActGuard           // combat: end the turn on guard
	ActBandage         // combat: bandage the first dying member
	ActWait            // combat: Done's Delay
	ActEnd             // combat: Done's Quit: end the turn
	ActQuick           // combat: the computer fights for the member
	ActTurn            // combat: turn undead
	ActCast            // combat: cast spell N (one of Turn.Spells)
	ActItems           // combat: the inventory
	ActAim             // combat, aiming: take square X, Y
	ActPoint           // combat, aiming: the pointer is on square X, Y (the preview follows)
	ActYes             // combat: yes to Battle.Ask (ActClose: no)
)

// Action is one thing the player did on a remastered screen.
type Action struct {
	Kind ActKind
	N    int
	X, Y int // a combat map square
}

// acts carries the presenter's actions to the engine goroutine.
type acts struct {
	ch      chan Action
	pending atomic.Int32 // sent and not yet done
	taken   int32        // received by the engine goroutine, done once it waits again
	waiting atomic.Bool  // a screen waits in act: actions sent meanwhile are its
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
	return (st == pausing || st == waiting && !e.S.KeyPressed()) && !e.Busy()
}

// Idle reports whether the engine waits for input and none is queued (held
// keys repeat only then, so they never pile up).
func (e *Engine) Idle() bool { return e.acts.state.Load() == waiting && !e.S.KeyPressed() }

// readKey blocks for the next key from the window.
func (e *Engine) readKey() uint16 {
	e.pace(waiting)
	defer e.pace(working)
	return <-e.S.Keys
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
	if !e.acts.waiting.Load() { // made on a View the game has moved on from: dropped like a typed-ahead key
		return
	}
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

// act publishes the View and waits for the next action. Keys typed
// meanwhile still reach the game's own handling (F9, Ctrl-F, ?), but no
// screen sees them.
func (e *Engine) act() Action {
	e.actsDone()
	e.acts.waiting.Store(true)
	defer e.acts.waiting.Store(false)
	for {
		e.publish()
		e.pace(waiting)
		select {
		case a := <-e.acts.ch:
			e.pace(working)
			e.acts.taken++
			return a
		case k := <-e.S.Keys:
			e.pace(working)
			e.translate(k) // the extras act; the key itself is no answer
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
