package engine

// toMainMenu unwinds the game in play to Apply (catch), which goes back to
// the party menu.
type toMainMenu struct{}

// MainMenu (extra) drops the game in play and goes back to the party menu,
// the game's main menu. Ignored where a slot load
// could not unwind either (no game yet, a modal screen).
func (e *Engine) MainMenu() { e.Apply(Action{Kind: ActMainMenu}) }

// mainMenu runs ActMainMenu (Apply).
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
