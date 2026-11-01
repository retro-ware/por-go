package engine

import "testing"

// A script's HMENU does not wait: it leaves the VM pending on its menu
// (View.Script), and the answering decision completes it; an encounter's
// WAIT asks again, a typed number not 0..65536 is asked again.
func TestScriptAsks(t *testing.T) {
	e := quickStarted(t)
	e.es.str[1], e.es.str[2] = "YES", "NO"
	var got int
	e.askChoice("", "~YES ~NO", false, false, func(c int) { got = c })
	if w := e.eclWait; w == nil || w.ask.Kind != AskMenu || len(w.ask.Options) != 2 {
		t.Fatalf("ask %+v", e.eclWait)
	}
	if e.applyScript(Action{Kind: ActOK}) {
		t.Fatal("Enter answered a menu without it")
	}
	if !e.applyScript(Action{Kind: ActChoose, N: 1}) || got != 1 || e.eclWait != nil {
		t.Fatalf("NO gave %d, pending %v", got, e.eclWait != nil)
	}

	rounds := 0
	var round func()
	round = func() {
		e.askChoice("", "~COMBAT ~WAIT", false, false, func(c int) {
			if rounds++; c == 1 {
				round()
			}
		})
	}
	round()
	if e.applyScript(Action{Kind: ActChoose, N: 1}) || e.eclWait == nil {
		t.Fatal("WAIT ended the menu")
	}
	if !e.applyScript(Action{Kind: ActChoose, N: 0}) || rounds != 2 {
		t.Fatalf("%d rounds", rounds)
	}

	var n uint16
	e.askNumber(func(v uint16) { n = v })
	e.applyScript(Action{Kind: ActText, Text: "99999"})
	if e.applyScript(Action{Kind: ActOK}) {
		t.Fatal("99999 taken")
	}
	e.applyScript(Action{Kind: ActText, Text: "42"})
	if !e.applyScript(Action{Kind: ActOK}) || n != 42 {
		t.Fatalf("number %d", n)
	}

	var who *Character
	e.askMember("WHO?", nil, func(c *Character) { who = c })
	if !e.applyScript(Action{Kind: ActMember, N: 1}) || who != e.party[1] {
		t.Fatal("the member pick did not take member 2")
	}
}
