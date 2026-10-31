package engine

// Screens taking decisions (docs/engine-redesign.md): e.mode names the
// screen up and its handler (applyTemple, applyShop, applyItems ...)
// applies one decision and returns, done or not; it never waits for
// input or time. A handler opens another screen by pushing it: the
// pushed screen takes the decisions until it is done, then the one under
// it gets actRedraw. Until M6 removes the engine goroutine, await and
// until feed them.

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
)

// screen is a screen taking decisions: its handler, what leaving it
// does, and the mode under it.
type screen struct {
	mode, under screenMode
	apply       func(Action) (done bool)
	leave       func()
}

// push puts screen m up; it takes the decisions until apply is done,
// then leave runs (nil: nothing to do).
func (e *Engine) push(m screenMode, apply func(Action) bool, leave func()) {
	e.screens = append(e.screens, screen{m, e.mode, apply, leave})
	e.mode = m
}

// await puts screen m up and feeds it decisions until it is done (the
// blocking adapter of the legacy callers, until M6).
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
