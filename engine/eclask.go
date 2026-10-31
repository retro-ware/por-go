package engine

import (
	"strconv"
	"strings"
)

// Script asks (docs/engine-redesign.md, M4): a script instruction that
// needs the player does not wait. It records what it asks (View.Script)
// and how the answer completes it (eclWait), and returns; RunECL then puts
// modeScript up, whose handler (applyScript) completes the instruction on
// the answering decision, and the VM goes on with the next one. A
// completion may ask again (the encounter's WAIT, a native script's next
// question): the screen stays up until nothing is pending.

// AskKind is what a pending script instruction waits for.
type AskKind int

const (
	AskMenu   AskKind = iota + 1 // a one-line menu (HMENU, the encounter's, Parlay's, "press Enter"): View.Menu; ActChoose N, ActContinue where Enter takes the first
	AskList                      // a vertical menu (VMENU): View.List; ActChoose with X the line; ActClose takes a last LEAVE
	AskText                      // a typed line (INPUT string or number): View.Typing; ActText, then ActContinue
	AskMember                    // a member pick (WHO): View.Choosing; ActMember N, the arrows step, ActContinue takes the candidate
	AskPause                     // a pause (DELAY): the presenter waits Ms, then sends ActContinue (Enter cuts it short)
)

// ScriptAsk is the pending script instruction's question.
type ScriptAsk struct {
	Kind    AskKind
	Prompt  string
	Options []string // the menu's options or the list's lines
	Max     int      // AskText: the longest line
	Number  bool     // AskText: a number 0..65536
	Ms      int      // AskPause
}

// eclWait is the pending instruction: what it asks, how it is shown
// (the menu, list or typed line the presenter draws) and its completion.
type eclWait struct {
	ask    ScriptAsk
	menu   *Menu
	list   []*ListNode
	typing *Typing
	anim   bool              // the picture animates while it waits
	apply  func(Action) bool // completes the instruction (true) or keeps it waiting
	done   func()            // nil, or what ending the ask puts back
}

// waitScript puts modeScript up while an instruction is pending (the
// blocking adapter until M6, as await).
func (e *Engine) waitScript() {
	if e.eclWait == nil {
		return
	}
	menu, list, typing, anim, last := e.snap.menu, e.snap.list, e.snap.typing, e.animInput, e.pic.last
	e.ext.menu++
	e.pic.last = e.Ticks() // the animation timer starts over, as in DisplayInput
	e.showAsk()
	e.await(modeScript, e.applyScript)
	e.snap.menu, e.snap.list, e.snap.typing, e.animInput, e.pic.last = menu, list, typing, anim, last
	e.ext.menu--
	e.ext.typing, e.ext.choosing = false, false
	e.viewAnim = 0
}

// applyScript is modeScript's handler: a completes the pending
// instruction or not; done when nothing is pending any more.
func (e *Engine) applyScript(a Action) bool {
	w := e.eclWait
	if w == nil {
		return true
	}
	if !w.apply(a) {
		e.showAsk()
		return false
	}
	if w.done != nil {
		w.done()
	}
	if e.eclWait == w {
		e.eclWait = nil
	}
	if e.eclWait == nil {
		return true
	}
	e.showAsk()
	return false
}

// showAsk publishes the pending ask the way the presenter draws it.
func (e *Engine) showAsk() {
	w := e.eclWait
	e.snap.menu, e.snap.list, e.snap.typing, e.animInput = w.menu, w.list, w.typing, w.anim
	e.ext.typing, e.ext.choosing = w.typing != nil, w.ask.Kind == AskMember
}

func menuTexts(m *Menu) []string {
	var t []string
	for _, it := range m.Items {
		t = append(t, it.Text)
	}
	return t
}

// askChoice asks menu (MenuInput's hotkey text) after prompt: then gets
// the choice's place in the menu (0 for Enter with allowEnter).
func (e *Engine) askChoice(prompt, menu string, allowEnter, anim bool, then func(int)) {
	m, q := e.newQuestion(prompt, hotBar(menu), true, true)
	e.eclWait = &eclWait{ask: ScriptAsk{Kind: AskMenu, Prompt: m.Prompt, Options: menuTexts(m)}, menu: m, anim: anim,
		apply: func(a Action) bool {
			p, ok := q.decide(a)
			switch {
			case !ok:
			case p.extra():
				e.partyKeypad(p)
				e.DisplayPartyList()
			case p.ID >= 0:
				then(int(p.ID))
				return true
			case p.ID == PickEnter && allowEnter:
				then(0)
				return true
			}
			return false
		}}
}

// askContinue is the scripts' one-item "PRESS <RETURN>" menu.
func (e *Engine) askContinue(then func()) {
	e.askChoice("", "~PRESS <ENTER>/<RETURN> TO CONTINUE", true, false, func(int) {
		e.ClearBox(0, promptRow, lastCol, promptRow)
		then()
	})
}

// askList asks for a line of list (ListMenu without options): then gets
// its index, -1 when the player backs out.
func (e *Engine) askList(list []*ListNode, then func(int)) {
	m, q := e.newQuestion("", nil, true, true)
	ask := ScriptAsk{Kind: AskList}
	for _, l := range list {
		ask.Options = append(ask.Options, l.Text)
	}
	e.eclWait = &eclWait{ask: ask, menu: m, list: list,
		apply: func(a Action) bool {
			e.ext.chosen = -1
			p, ok := q.decide(a)
			i := e.ext.chosen
			switch {
			case !ok:
			case p.ID == PickExit || p.ID == NoPick:
				then(-1)
				return true
			case i >= 0 && i < len(list) && !list[i].skipped():
				then(i)
				return true
			}
			return false
		}}
}

// askLine asks for a typed line of at most maxlen (InputString's, upcased;
// with number, InputNumber's: asked again until it is 0..65536).
func (e *Engine) askLine(maxlen int, number bool, then func(string)) {
	e.ClearRow24()
	t := &Typing{Max: maxlen, Prompt: strings.TrimSpace(e.ext.printed)} // the question is the text the script printed
	e.eclWait = &eclWait{ask: ScriptAsk{Kind: AskText, Prompt: t.Prompt, Max: maxlen, Number: number}, typing: t,
		apply: func(a Action) bool {
			switch a.Kind {
			case ActText:
				b := make([]byte, 0, maxlen)
				for i := 0; i < len(a.Text) && len(b) < maxlen; i++ {
					if c := a.Text[i]; c >= 0x20 && c <= 0x7A {
						b = append(b, c)
					}
				}
				t.Text = string(b)
				e.DisplaySpaces(1, promptRow, colBlack, 39)
				e.DisplayString(1, promptRow, colWhite, t.Text)
			case ActContinue, ActClose:
				s := strings.ToUpper(t.Text)
				if n, err := strconv.ParseInt(strings.TrimLeft(s, " "), 10, 32); number && (err != nil || n < 0 || n > 0x10000) {
					t.Text = ""
					return false
				}
				e.ClearRow24()
				then(s)
				return true
			}
			return false
		}}
}

// askNumber is askLine for a number (InputNumber): then gets its low word.
func (e *Engine) askNumber(then func(uint16)) {
	e.askLine(6, true, func(s string) {
		n, _ := strconv.ParseInt(strings.TrimLeft(s, " "), 10, 32)
		then(uint16(n))
	})
}

// askMember asks for a party member (SelectCharacter without Exit),
// starting at who; the candidate is the selected member as it steps (the
// var of opWho is the selection itself).
func (e *Engine) askMember(prompt string, who *Character, then func(*Character)) {
	if len(e.party) == 0 {
		then(nil)
		return
	}
	if who == nil {
		who = e.party[0]
	}
	e.rosterHi, e.rosterNone, e.ext.choosing = who, false, true
	e.DisplayPartyList()
	mark := e.regionMark()
	e.rosterRegion() // a click on a member picks it
	m, q := e.newQuestion(cut(prompt, textCols)+" ", Bar{{"Select", 0}}, true, true)
	e.eclWait = &eclWait{ask: ScriptAsk{Kind: AskMember, Prompt: m.Prompt}, menu: m, anim: e.beginFlag == 2 || e.beginFlag == 6,
		done: func() {
			e.regionCut(mark)
			e.rosterHi, e.rosterNone = nil, false
			e.ext.choosing = false
			e.DisplayPartyList()
		},
		apply: func(a Action) bool {
			p, ok := q.decide(a)
			who := e.rosterHi
			switch {
			case !ok:
			case p.ID == PickEnter || p.ID == 0:
				then(who)
				return true
			case p.extra():
				i := e.partyIndex(who)
				switch p.Dir {
				case DirEnd, DirDown, DirRight: // next, wrapping to the first
					who = e.party[(i+1)%len(e.party)]
				case DirHome, DirUp, DirLeft: // previous, wrapping to the last
					who = e.party[(i+len(e.party)-1)%len(e.party)]
				}
				e.selectChar(who)
				e.rosterHi, e.rosterNone = who, false
				e.DisplayPartyList()
			}
			return false
		}}
}

// askPause is a pause of ms the presenter paces (the engine never sleeps).
func (e *Engine) askPause(ms int, then func()) {
	if ms <= 0 {
		then()
		return
	}
	e.eclWait = &eclWait{ask: ScriptAsk{Kind: AskPause, Ms: ms},
		apply: func(a Action) bool {
			if a.Kind != ActContinue && a.Kind != ActClose {
				return false
			}
			then()
			return true
		}}
}
