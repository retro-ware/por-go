package engine

import "strings"

// Questions to the player (ovr26's menu bars, remade): DisplayInput
// publishes the prompt and its options as View.Ask and waits for the
// player's decision (ActChoose N, ActClose, ActOK); how the options
// are shown and picked is the presenter's.

func isHotkey(c byte) bool { return c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' } // set [0-9A-Z]

// optionText (extra): how a word reads as an option. A hotkey inside a
// word reads lower case and the word's first letter upper case, so "sWeep"
// reads "Sweep", "aiM" "Aim".
func optionText(w string) string {
	b := []byte(w)
	lower := func(c byte) bool { return c >= 'a' && c <= 'z' }
	hasLower := strings.IndexFunc(w, func(r rune) bool { return r >= 'a' && r <= 'z' }) >= 0
	for i, c := range b {
		switch {
		case i == 0 && lower(c):
			b[i] = c - 32
		case i > 0 && hasLower && c >= 'A' && c <= 'Z':
			b[i] = c + 32
		}
	}
	return string(b)
}

// Choice identifies a menu item: each menu numbers its own items (from 0);
// the negative values are what a menu bar hands back besides an item.
type Choice int

const (
	NoPick     Choice = -1 - iota // the wait ended with nothing (Esc on a menu without Exit)
	PickEnter                     // Enter, where it does not pick the highlighted item
	PickRedraw                    // draw again: the cursor moved (ActPoint), a screen over it went (actRedraw)
	PickRelist                    // another member (F1-F8): the caller rebuilds the list
	PickSlots                     // F9 on a menu bar of the slot screen
	PickExit                      // an Exit item (ListMenu ends on it)
	pickNext                      // ListMenu's Next and Prev
	pickPrev
)

// Pick is what a menu bar hands back: the item's Choice or an outcome.
type Pick struct {
	ID   Choice
	Line int // the list's line ActChoose came with (ListMenu); -1 none
}

// extra: the pick is one of the original's extended keys (the keypad, or
// an outcome of the remaster's own), not an item, Enter or nothing.
func (p Pick) extra() bool {
	switch p.ID {
	case PickRedraw, PickRelist, PickSlots:
		return true
	}
	return false
}

// Entry is one word of a menu bar. Its [0-9A-Z] letter is the hotkey: shown
// lit and typed to pick it; the decision is ID.
type Entry struct {
	Word string
	ID   Choice
}

// Bar is a menu bar's items.
type Bar []Entry

func (b Bar) String() string {
	w := make([]string, len(b))
	for i, it := range b {
		w[i] = it.Word
	}
	return strings.Join(w, " ")
}

// ids: the Choice of each menu item, in menuItems order (a word with two
// hotkeys gives both its ID).
func (b Bar) ids() []Choice {
	var ids []Choice
	for _, it := range b {
		for i := 0; i < len(it.Word); i++ {
			if isHotkey(it.Word[i]) {
				ids = append(ids, it.ID)
			}
		}
	}
	return ids
}

// textBar makes a bar of a menu that comes as text (a script's): each word
// is a choice, numbered by its first hotkey's item.
// ponytail: a word with two hotkeys gives both the first one's number; split
// such words into entries if a script ever has one.
func textBar(menu string) Bar {
	var b Bar
	n := 0
	for _, w := range strings.Split(menu, " ") {
		b = append(b, Entry{w, Choice(n)})
		n += len(Bar{{Word: w}}.ids())
	}
	return b
}

// displayThen = DisplayInput (ovr26:03BD: prompt, menu, colours,
// allowExtra, anim) as a screen (modeScript): the question is an ask
// (View.Ask) the presenter answers; then gets the pick once the screen is
// gone. The colours are the original's and unused.
func (e *Engine) displayThen(prompt string, bar Bar, _, normal, highlight int, allowExtra, anim bool, then func(Pick)) {
	e.question(AskMenu, prompt, bar, allowExtra, normal != 0 || highlight != 0, anim, then)
}

// question is displayThen asking a question of kind k (AskMenu, AskList
// with the list ListMenu shows, AskMember).
func (e *Engine) question(k AskKind, prompt string, bar Bar, allowExtra, allowEnter, anim bool, then func(Pick)) {
	m, q := e.newQuestion(prompt, bar, allowExtra, allowEnter)
	ask := Ask{Kind: k, Menu: *m}
	switch {
	case k == AskList:
		ask.List, q.list = listLines(e.snap.list), true
	case k == AskMenu && m.Prompt == "" && len(m.Items) == 0: // nothing to ask: any key
		ask.Kind = AskKey
	}
	var p Pick
	prev := e.eclWait // a pending script ask is kept under it
	e.eclWait = &eclWait{ask: ask, anim: anim,
		apply: func(a Action) (ok bool) {
			if a.Kind == actRedraw { // a screen over it is gone: the question is still up, drawn again (applyScript's showAsk), not answered
				return false
			}
			p, ok = q.decide(a)
			return ok
		}}
	e.pushWait(func() {
		e.eclWait = prev
		then(p)
	})
}

// newQuestion is the menu DisplayInput (or a script's ask) publishes for
// prompt and bar and the question deciding its answers.
func (e *Engine) newQuestion(prompt string, bar Bar, allowExtra, allowEnter bool) (*Menu, *question) {
	m := &Menu{Prompt: strings.TrimSpace(prompt)}
	var ids []Choice
	for _, it := range bar {
		if i := strings.IndexFunc(it.Word, func(r rune) bool { return r < 0x80 && isHotkey(byte(r)) }); i >= 0 {
			m.Items = append(m.Items, MenuItem{Text: optionText(it.Word), Key: it.Word[i]})
			ids = append(ids, it.ID)
		}
	}
	return m, &question{e: e, items: m.Items, ids: ids, allowExtra: allowExtra, allowEnter: allowEnter}
}

// question is the question DisplayInput waits on.
type question struct {
	e                      *Engine
	items                  []MenuItem
	ids                    []Choice // each item's Choice
	allowExtra, allowEnter bool
	list                   bool // a list's question: ActChoose N -1 is the line itself
}

// decide: what the question hands back for a, if it ends the wait (ok);
// the pick's Line is ActChoose's X (-1 for any other decision).
func (q *question) decide(a Action) (p Pick, ok bool) {
	p, ok = q.pick(a)
	p.Line = -1
	if a.Kind == ActChoose {
		p.Line = a.X
	}
	return p, ok
}

func (q *question) pick(a Action) (p Pick, ok bool) {
	e := q.e
	switch a.Kind {
	case ActChoose:
		if a.N == -1 && q.list { // the line itself
			return Pick{ID: PickEnter}, true
		}
		if a.N >= 0 && a.N < len(q.ids) {
			return Pick{ID: q.ids[a.N]}, true
		}
	case actHotkey:
		return q.key(byte(a.N))
	case actRedraw:
		return q.outcome(Pick{ID: PickRedraw})
	case ActOK:
		return Pick{ID: PickEnter}, q.allowEnter || len(q.items) == 0
	case ActClose: // the Exit option, so it always backs out
		if !e.modal() && q.has('E', "Exit") {
			return q.key('E')
		}
		return Pick{ID: NoPick}, true
	case ActPoint: // the full map's cursor
		if ms := e.ext.fullMap; ms != nil && onMap(a.X, a.Y) {
			c := &ms.cursor
			c.X, c.Y = a.X, a.Y
			return q.outcome(Pick{ID: PickRedraw})
		}
	case ActLead: // the member keys: another member; in a member pick the candidate
		switch n := a.N; {
		case n < 0 || n >= len(e.party) || e.modal():
		case e.choosing():
			e.rosterHi, e.rosterNone = e.party[n], false
			return q.outcome(Pick{ID: PickRedraw})
		case q.allowExtra && e.beginFlag != 5:
			e.selectMember(n)
			return q.outcome(Pick{ID: PickRelist})
		}
	case ActMember: // a portrait: that member's View page, or the member a list or question is for
		switch n := a.N; {
		case e.modal() || n < 0 || n >= len(e.party):
		case e.choosing() || e.viewing() || e.eclWait.members:
			return q.member(n)
		case q.allowExtra && e.beginFlag != 5:
			e.selectMember(n)
			return q.outcome(Pick{ID: PickRelist})
		}
	case ActText:
	default:
		if e.modal() { // the slot screen's: any other action is Enter
			if a.Kind == ActSlots {
				return q.outcome(Pick{ID: PickSlots})
			}
			return Pick{ID: PickEnter}, true
		}
		switch a.Kind {
		case ActInventory:
			return q.member(a.N)
		case ActOverview: // P: on the View page the items
			if e.viewing() {
				if len(e.carried(e.selected())) > 0 {
					return q.key('I')
				}
			} else {
				return q.key('P')
			}
		case ActMap:
			if e.ext.fullMap != nil && !e.typing() { // M closes the map it opened
				return q.key('E')
			}
			return q.key('M')
		}
	}
	return Pick{}, false
}

// has: an option with hotkey k reading text.
func (q *question) has(k byte, text string) bool {
	for _, it := range q.items {
		if it.Key == k && it.Text == text {
			return true
		}
	}
	return false
}

// key picks the option whose hotkey is k (a click or a shortcut standing
// for it); with the keypad allowed a digit is the keypad's key.
func (q *question) key(k byte) (Pick, bool) {
	k = upper(k)
	for i, it := range q.items {
		if it.Key == k {
			return Pick{ID: q.ids[i]}, true
		}
	}
	return Pick{}, false
}

// outcome hands back an extended key's outcome where the question takes them.
func (q *question) outcome(p Pick) (Pick, bool) { return p, q.allowExtra }

// member: F1..F8 (n = 0..7) or a portrait. In a member pick it is the
// pick; on the View page or in the inventory it switches to member n
// (Exit, then View again); with a View option it selects member n and
// opens their items. Not in combat, where View is the fighter's.
func (q *question) member(n int) (Pick, bool) {
	e := q.e
	if n < 0 || n >= len(e.party) {
		return Pick{}, false
	}
	switch {
	case e.choosing(): // SelectCharacter: member n is the pick
		e.rosterHi = e.party[n]
		return q.key('S')
	case e.beginFlag == 5:
		return Pick{}, false
	case e.viewing():
		if n == e.partySel {
			return Pick{}, false
		}
		e.selectMember(n) // the View page for member n
		e.ext.held = Action{Kind: actHotkey, N: 'V'}
		return q.key('E')
	case e.eclWait.members: // F1-F8 pick the member the question is for
		if n == e.partySel {
			return Pick{}, false
		}
		e.selectMember(n)
		return q.outcome(Pick{ID: PickRelist})
	}
	return Pick{}, false
}

// YesNo = ovr26:10D1 (c1, c2, c3, prompt): "Yes No" on row 24, No
// preselected; then gets true for Yes.
func (e *Engine) YesNo(c1, c2, c3 int, prompt string, then func(bool)) {
	const (
		yes Choice = iota
		no
	)
	e.displayThen(prompt, Bar{{"Yes", yes}, {"No", no}}, c1, c2, c3, false, false, func(p Pick) {
		switch p.ID {
		case yes:
			then(true)
		case no:
			then(false)
		default:
			e.YesNo(c1, c2, c3, prompt, then)
		}
	})
}
