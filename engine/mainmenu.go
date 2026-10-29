package engine

// toMainMenu unwinds the game in play to adventure, which goes back to the
// party menu (guardedRun).
type toMainMenu struct{}

// MainMenu (extra) drops the game in play and goes back to the party menu,
// the game's main menu; safe from any goroutine. Ignored where a slot load
// could not unwind either (no game yet, a modal screen).
func (e *Engine) MainMenu() { e.Act(Action{Kind: ActMainMenu}) }

// mainMenu runs ActMainMenu on the engine goroutine.
func (e *Engine) mainMenu() {
	if !e.ext.guarded || e.ext.modal > 0 {
		return
	}
	e.stopVoice()
	if e.beginFlag == 5 {
		e.combatViewLeave()
	}
	panic(toMainMenu{})
}
