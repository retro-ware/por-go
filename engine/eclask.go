package engine

import (
	"strconv"
	"strings"
)

// Questions (docs/engine-redesign.md, M4): an instruction or rule that
// needs the player does not wait. It records what it asks (View.Ask)
// and how the answer completes it (eclWait), and returns; RunECL then puts
// modeScript up, whose handler (applyScript) completes the instruction on
// the answering decision, and the VM goes on with the next one. A
// completion may ask again (the encounter's WAIT, a native script's next
// question): the screen stays up until nothing is pending.

// AskKind is what a question waits for.
type AskKind int

const (
	AskMenu    AskKind = iota + 1 // options (a menu bar, the scripts' HMENU, Parlay, a yes/no): ActChoose N; ActOK where Enter takes the first; no options: a message to acknowledge (ActOK)
	AskList                       // a list and its options (ListMenu, VMENU): ActChoose N with X the line (N -1: the line itself); ActClose takes a last LEAVE
	AskText                       // a typed line (Text, at most Max): ActText, then ActOK; ActClose gives up
	AskNumber                     // AskText for a number 0..65536
	AskMember                     // a party member (SelectCharacter, WHO): Party's Selected is the candidate; ActMember N takes one, ActLead N makes it the candidate, ActOK takes it
	AskPause                      // a pause (DELAY, MsgDelay, the title's waits): the presenter waits Ms, then sends ActOK (Enter cuts it short)
	AskKey                        // "press any key" (the pagers, PromptRow24): any key answers (ActOK, ActPage ...)
	AskSpell                      // a memorized spell (Spells): ActCast N, ActClose backs out
	AskConfirm                    // a yes/no question (Menu.Prompt, no Items): ActYes, or ActNo / ActClose; the presenter asks it, starting on No
)

// Ask is the question waiting for the player (View.Ask): every one is
// published the same way, whoever asks (the engine's prompts, a script
// instruction).
type Ask struct {
	Kind   AskKind
	Menu              // the prompt and the options (ActChoose N)
	List   []ListLine // AskList: the lines
	Text   string     // AskText, AskNumber: the line as typed so far
	Max    int        // AskText, AskNumber: the longest line
	Ms     int        // AskPause
	Spells []Spell    // AskSpell: the spells to pick from (ActCast ID)
}

// eclWait is the pending question: what it asks and how the answer
// completes it.
type eclWait struct {
	ask     Ask
	anim    bool              // the picture animates while it waits
	members bool              // F1-F8 and the portraits pick the member the question is for (the loot)
	apply   func(Action) bool // completes the question (true) or keeps it waiting
	done    func()            // nil, or what ending the ask puts back
}

// asked: the question on top is of kind k.
func (e *Engine) asked(k ...AskKind) bool {
	if e.eclWait == nil { // (a pending ask under another screen still counts, as it waits)
		return false
	}
	for _, x := range k {
		if e.eclWait.ask.Kind == x {
			return true
		}
	}
	return false
}

// choosing: a member pick waits (the roster highlights its candidate).
func (e *Engine) choosing() bool { return e.asked(AskMember) }

// typing: a line is being typed.
func (e *Engine) typing() bool { return e.asked(AskText, AskNumber) }

// pushWait puts the pending ask up as a screen (modeScript); then runs
// once it is answered and gone.
func (e *Engine) pushWait(then func()) {
	anim, last := e.animInput, e.pic.last
	member := e.eclWait.ask.Kind == AskMember
	e.pic.last = e.Ticks() // the animation timer starts over, as in DisplayInput
	e.push(modeScript, e.applyScript, func() {
		e.animInput, e.pic.last = anim, last
		e.viewAnim = 0
		if member { // the roster without the candidate's highlight
		}
		then()
	})
	e.showAsk()
}

// asking runs ask, which records an ask (the engine's own prompts: the
// pager, a yes/no, a typed line ...), and puts it up as a screen; a
// pending script ask is kept under it. then runs once it is answered and
// gone (at once when ask asked nothing).
func (e *Engine) asking(ask func(), then func()) {
	prev := e.eclWait
	e.eclWait = nil
	ask()
	if e.eclWait == nil {
		e.eclWait = prev
		then()
		return
	}
	e.pushWait(func() {
		e.eclWait = prev
		then()
	})
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

// showAsk shows the pending ask: the picture's animation, a member
// pick's candidate in the roster.
func (e *Engine) showAsk() {
	w := e.eclWait
	e.animInput = w.anim
}

// listLines are the lines of list as View.Ask shows them.
func listLines(list []*ListNode) []ListLine {
	l := make([]ListLine, len(list))
	for i, n := range list {
		l[i] = ListLine{Text: n.Text, Header: n.Flag != 0, Dim: n.Dim || n.Grey, Off: n.skipped(), Without: n.Without, Spell: n.Spell}
	}
	return l
}

// askChoice asks menu (MenuInput's hotkey text) after prompt: then gets
// the choice's place in the menu (0 for Enter with allowEnter).
func (e *Engine) askChoice(prompt, menu string, allowEnter, anim bool, then func(int)) {
	m, q := e.newQuestion(prompt, hotBar(menu), true, true)
	e.eclWait = &eclWait{ask: Ask{Kind: AskMenu, Menu: *m}, anim: anim,
		apply: func(a Action) bool {
			p, ok := q.decide(a)
			switch {
			case !ok:
			case p.extra():
				e.partyKeypad(p)
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
		e.clearPrompt()
		then()
	})
}

// askList asks for a line of list (ListMenu without options): then gets
// its index, -1 when the player backs out.
func (e *Engine) askList(list []*ListNode, then func(int)) {
	m, q := e.newQuestion("", nil, true, true)
	q.list = true
	e.eclWait = &eclWait{ask: Ask{Kind: AskList, Menu: *m, List: listLines(list)},
		apply: func(a Action) bool {
			p, ok := q.decide(a)
			i := p.Line
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
	e.clearPrompt()
	kind := AskText
	if number {
		kind = AskNumber
	}
	w := &eclWait{ask: Ask{Kind: kind, Menu: Menu{Prompt: strings.TrimSpace(e.ext.printed)}, Max: maxlen}} // the question is the text the script printed
	t := &w.ask
	e.eclWait = w
	w.apply = func(a Action) bool {
		switch a.Kind {
		case ActText:
			b := make([]byte, 0, maxlen)
			for i := 0; i < len(a.Text) && len(b) < maxlen; i++ {
				if c := a.Text[i]; c >= 0x20 && c <= 0x7A {
					b = append(b, c)
				}
			}
			t.Text = string(b)
			e.clearPrompt()
			e.text(1, promptRow, colWhite, t.Text)
		case ActOK, ActClose:
			s := strings.ToUpper(t.Text)
			if n, err := strconv.ParseInt(strings.TrimLeft(s, " "), 10, 32); number && (err != nil || n < 0 || n > 0x10000) {
				t.Text = ""
				return false
			}
			e.clearPrompt()
			then(s)
			return true
		}
		return false
	}
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
	e.rosterHi, e.rosterNone = who, false
	m, q := e.newQuestion(cut(prompt, textCols)+" ", Bar{{"Select", 0}}, true, true)
	e.eclWait = &eclWait{ask: Ask{Kind: AskMember, Menu: *m}, anim: e.beginFlag == 2 || e.beginFlag == 6,
		done: func() {
			e.rosterHi, e.rosterNone = nil, false
		},
		apply: func(a Action) bool {
			p, ok := q.decide(a)
			who := e.rosterHi
			switch {
			case !ok:
			case p.ID == PickEnter || p.ID == 0:
				then(who)
				return true
			case p.extra(): // ActLead stepped the candidate
				e.selectChar(who)
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
	e.eclWait = &eclWait{ask: Ask{Kind: AskPause, Ms: ms},
		apply: func(a Action) bool {
			if a.Kind != ActOK && a.Kind != ActClose {
				return false
			}
			then()
			return true
		}}
}
