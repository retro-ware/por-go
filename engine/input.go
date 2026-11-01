package engine

// typed (extra) asks for the line the presenter edits, starting as s:
// ActText sets it (only the characters ok takes, at most maxlen), show
// draws it; Enter ends it, Esc too (false); then gets the line.
func (e *Engine) typed(s string, maxlen int, ok func(byte) bool, show func(string), then func(string, bool)) {
	w := &eclWait{ask: Ask{Kind: AskText, Text: s, Max: maxlen}}
	t := &w.ask
	var done bool
	show(s)
	e.asking(func() { // an ask (AskText): the presenter edits the line
		e.eclWait = w
		w.apply = func(a Action) bool {
			switch a.Kind {
			case ActText:
				b := make([]byte, 0, maxlen)
				for i := 0; i < len(a.Text) && len(b) < maxlen; i++ {
					if ok(a.Text[i]) {
						b = append(b, a.Text[i])
					}
				}
				t.Text = string(b)
				show(t.Text)
			case ActOK:
				done = true
				return true
			case ActClose:
				return true
			}
			return false
		}
	}, func() { then(t.Text, done) })
}

// DisplayMessageWait = 0709:0D68 (str, colour): message on row 24 for
// msgDelay*225 ms, then cleared.
func (e *Engine) DisplayMessageWait(colour int, s string) {
	e.say(s)                       // extra: a flash at the bottom (the original's row 24 is never drawn on the remastered 3D view); nothing to confirm
	e.Delay(int(e.msgDelay) * 225) // 0709:0DBE
}
