package engine

// Screens taking decisions (docs/engine-redesign.md): e.mode names the
// screen up and its handler (applyTemple, applyShop, applyItems ...)
// applies one decision and returns, done or not; it never waits for
// input or time. A handler opens another screen by pushing it: the
// pushed screen takes the decisions until it is done, then the one under
// it gets actRedraw. The engine's top level (top) only dispatches each
// decision to the top screen; until M6c/M6d flatten combat and the script
// VM, inline, await and until feed screens from inside a blocking caller
// in the engine's coroutine (Apply).

type screenMode int

const (
	modeNone     screenMode = iota
	modeTemple              // View.Temple
	modeShop                // View.Shop
	modeTraining            // View.Training
	modeCamp                // View.Camp
	modeItems               // View.Inventory
	modeSheet               // View.Sheet
	modeParty               // View.MainMenu
	modeCreate              // View.Creation
	modePicker              // View.Picker
	modeIcon                // the icon editor (View.IconDrop)
	modeSlots               // View.Slots
	modeScript              // View.Script: a script instruction waits for its answer (eclask.go)
	modeTurn                // View.Battle.Turn: a member's combat turn (applyTurn)
	modeAim                 // View.Battle.Turn.Aim: aiming (playerAim's handler)
	modeConfirm             // View.Battle.Ask: a combat yes/no (Flee?, Attack Ally?)
	modeExplore             // View.Exploring: the adventuring commands (applyExplore)
	modeTitle               // the title pictures and credits (timed)
	modeRest                // camp's rest, five minutes a frame (timed; View.Camp)
	modeStory               // the finale's pictures (timed)
)

// screen is a screen taking decisions: its handler, what leaving it
// does, and the mode under it.
type screen struct {
	mode, under screenMode
	apply       func(Action) (done bool)
	leave       func()
	timed       bool // it takes ActTick every frame no decision comes
}

// push puts screen m up; it takes the decisions until apply is done,
// then leave runs (nil: nothing to do).
func (e *Engine) push(m screenMode, apply func(Action) bool, leave func()) {
	e.screens = append(e.screens, screen{m, e.mode, apply, leave, false})
	e.mode = m
}

// pushTimed puts up a timed screen: a frame going by with no decision
// gives its handler ActTick (decision), so a timed loop keeps its state
// in the handler instead of waiting in place.
func (e *Engine) pushTimed(m screenMode, apply func(Action) bool, leave func()) {
	e.push(m, apply, leave)
	e.screens[len(e.screens)-1].timed = true
}

// timed: the top screen takes the frames.
func (e *Engine) timed() bool { return len(e.screens) > 0 && e.screens[len(e.screens)-1].timed }

// inline runs push, which puts screens up, and feeds them until they are
// all done: the blocking adapter of the callers still inside the script
// VM or combat (opShop, opProgram, the combat sheet: M6c, M6d).
func (e *Engine) inline(push func()) {
	base := len(e.screens)
	push()
	e.until(base)
}

// await puts screen m up and feeds it decisions until it is done (the
// blocking adapter of combat's turn and the script asks, until M6c/M6d).
func (e *Engine) await(m screenMode, apply func(Action) (done bool)) {
	base := len(e.screens)
	e.push(m, apply, nil)
	e.until(base)
}

// until feeds decisions to the screens pushed over the first base until
// they are all done.
func (e *Engine) until(base int) {
	for len(e.screens) > base {
		e.feed(e.decision(), base)
	}
}

// feed gives a to the top screen; when that is done, it goes and the
// one under it (down to base) gets actRedraw.
func (e *Engine) feed(a Action, base int) {
	for n := len(e.screens); n > base; n = len(e.screens) {
		s := e.screens[n-1]
		if !s.apply(a) {
			return
		}
		e.screens, e.mode = e.screens[:n-1], s.under
		if s.leave != nil {
			s.leave()
		}
		a = Action{Kind: actRedraw}
	}
}

// actor is member n, made the selected one (n < 0: the selected one).
func (e *Engine) actor(n int) *Character {
	if n >= 0 && n < len(e.party) {
		e.selectMember(n)
	}
	return e.selected()
}
