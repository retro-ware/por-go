package engine

// Screens taking decisions (docs/engine-redesign.md): e.mode names the
// screen up and its handler (applyTemple, applyShop, applyItems ...)
// applies one decision and returns, done or not; it never waits for
// input or time. A handler opens another screen by pushing it: the
// pushed screen takes the decisions until it is done, then the one under
// it gets actRedraw. The engine's top level (top) only dispatches each
// decision to the top screen (Apply); work that follows a screen goes in
// its leave or a continuation (after, whenDown).

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
	modeCombat              // the fight's rounds, a turn a frame (timed; View.Battle)
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

// after runs push, which may put screens up, and f once they are all
// gone again (at once when it put none up).
func (e *Engine) after(push func(), f func()) {
	base := len(e.screens)
	push()
	e.whenDown(base, f)
}

// whenDown runs f once the stack is down to base screens: in the leave
// of the screen at base, or of the one that leave put up in its place.
func (e *Engine) whenDown(base int, f func()) {
	if len(e.screens) <= base {
		f()
		return
	}
	s := &e.screens[base]
	leave := s.leave
	s.leave = func() {
		if leave != nil {
			leave()
		}
		e.whenDown(base, f)
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
		if len(e.screens) >= n { // the leave put a screen up: it has drawn itself
			return
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
