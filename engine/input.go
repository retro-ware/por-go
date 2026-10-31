package engine

import "strings"

// DisplaySpaces = 0709:05EB (x, y, colour, count): count blank cells.
func (e *Engine) DisplaySpaces(x, y, colour, count int) {
	if x > lastCol || y > promptRow {
		return
	}
	e.displayCharN(x, y, colour, count, ' ', true)
}

// ClearRow24 = ovr26:06FB: blank the message/menu row.
func (e *Engine) ClearRow24() { e.DisplaySpaces(0, promptRow, colBlack, textCols) }

// InputString = 0709:09E3 (prompt, colour, maxlen): line editor on row 24.
// Characters 0x20..0x7A are accepted while len < maxlen and echoed in white
// one cell right of the last; backspace erases; Enter or Esc ends; the
// result is upcased.
func (e *Engine) InputString(prompt string, colour, maxlen int) string {
	return strings.ToUpper(e.inputLine(prompt, colour, maxlen))
}

// inputLine is InputString keeping the case as typed (extra: character names).
// extra: the presenter edits the line (View.Typing); it is echoed in white
// one cell right of the prompt.
func (e *Engine) inputLine(prompt string, colour, maxlen int) string {
	if len(prompt) > 40 {
		prompt = prompt[:40]
	}
	e.ClearRow24()
	e.DisplayString(0, promptRow, colour, prompt)
	if prompt == "" { // a script's input (opInputString, opInputNumber): the question is the text it printed
		e.ext.asked = strings.TrimSpace(e.ext.printed)
		defer func() { e.ext.asked = "" }()
	}
	s, _ := e.typed("", maxlen, func(c byte) bool { return c >= 0x20 && c <= 0x7A }, func(s string) {
		e.DisplaySpaces(len(prompt)+1, promptRow, colBlack, max(0, 40-len(prompt)-1))
		e.DisplayString(len(prompt)+1, promptRow, colWhite, s)
	})
	e.ClearRow24()
	return s
}

// inlineInput (extra) edits a field where it stands: the cells from x, y
// hold the text typed (maxlen at most, the characters ok takes) with the
// cursor, a block in the selection colour, after it; Enter ends it (true),
// Esc too (false). The caller redraws the field.
func (e *Engine) inlineInput(x, y, maxlen, colour int, ok func(byte) bool) (string, bool) {
	return e.typed("", maxlen, ok, func(t string) {
		e.DisplaySpaces(x, y, colBlack, maxlen+1)
		e.DisplayString(x, y, colour, t)
		e.displayCharBg(x+len(t), y, colWhite, selBg(colour, colWhite), ' ', true)
	})
}

// typed (extra) waits for the line the presenter edits, starting as s:
// ActText sets it (only the characters ok takes, at most maxlen), show
// draws it; Enter ends it, Esc too (false).
func (e *Engine) typed(s string, maxlen int, ok func(byte) bool, show func(string)) (string, bool) {
	e.ext.typing = true
	defer func() { e.ext.typing, e.snap.typing = false, nil }()
	for {
		e.snap.typing = &Typing{Text: s, Max: maxlen, Prompt: e.ext.asked}
		show(s)
		e.actsDone()
		a := e.nextAct()
		switch a.Kind {
		case ActText:
			b := make([]byte, 0, maxlen)
			for i := 0; i < len(a.Text) && len(b) < maxlen; i++ {
				if ok(a.Text[i]) {
					b = append(b, a.Text[i])
				}
			}
			s = string(b)
		case ActContinue:
			return s, true
		case ActClose:
			return s, false
		default:
			e.extraAct(a)
		}
	}
}

// DisplayMessageWait = 0709:0D68 (str, colour): message on row 24 for
// msgDelay*225 ms, then cleared.
func (e *Engine) DisplayMessageWait(colour int, s string) {
	e.say(s)                       // extra: a flash at the bottom (the original's row 24 is never drawn on the remastered 3D view); nothing to confirm
	e.Delay(int(e.msgDelay) * 225) // 0709:0DBE
}
