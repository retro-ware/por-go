package engine

import (
	"sync/atomic"
	"time"

	"por/ega"
)

// Actions (extra): the remastered screens act on the game through typed
// actions instead of keys. A screen ported to them (ItemsMenu ...) publishes
// what it shows in the View and waits in act; the presenter sends Act.

// ActKind is what an Action does.
type ActKind int

const (
	ActNone   ActKind = iota
	ActClose          // leave the screen
	ActMember         // show party member N
	ActReady          // inventory: ready or unready item N
	ActUse            // inventory: use item N
	ActThrow          // inventory (combat): throw item N
	ActDrop           // inventory: drop item N (the player already confirmed)
	ActSell           // inventory: sell item N (confirmed)
)

// Action is one thing the player did on a remastered screen.
type Action struct {
	Kind ActKind
	N    int
}

// acts carries the presenter's actions to the engine goroutine.
type acts struct {
	ch      chan Action
	pending atomic.Int32 // sent and not yet done
	taken   int32        // received by the engine goroutine, done once it waits again
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

// act publishes the View and waits for the next action. Keys typed
// meanwhile still reach the game's own handling (F9, Ctrl-F, ?), but no
// screen sees them.
func (e *Engine) act() Action {
	e.actsDone()
	for {
		e.publish()
		e.S.SetState(ega.Waiting)
		select {
		case a := <-e.acts.ch:
			e.S.SetState(ega.Working)
			e.acts.taken++
			return a
		case k := <-e.S.Keys:
			e.S.SetState(ega.Working)
			e.translate(k) // the extras act; the key itself is no answer
		}
	}
}

// say shows s as the window's note: what a direct action reports instead
// of the original's message line.
func (e *Engine) say(s string) { e.ext.note, e.ext.noteAt = s, time.Now() }
