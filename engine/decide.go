package engine

// Screens taking decisions (docs/engine-redesign.md): e.mode names the
// screen up and its handler (applyTemple, applyShop, applyTraining)
// applies one decision and returns, done or not; it never waits for
// input or time. Until M6 removes the engine goroutine, await feeds it.

type screenMode int

const (
	modeNone     screenMode = iota
	modeTemple              // View.Temple
	modeShop                // View.Shop
	modeTraining            // View.Training
)

// await puts screen m up and applies the decisions with apply until it is
// done (the blocking adapter of the legacy callers, until M6).
func (e *Engine) await(m screenMode, apply func(Action) (done bool)) {
	saved := e.mode
	e.mode = m
	defer func() { e.mode = saved }()
	for !apply(e.decision()) {
		e.DisplayPartyList()
	}
}

// actor is member n, made the selected one (n < 0: the selected one).
func (e *Engine) actor(n int) *Character {
	if n >= 0 && n < len(e.party) {
		e.selectMember(n)
	}
	return e.selected()
}
