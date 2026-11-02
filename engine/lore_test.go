package engine

import (
	"strings"
	"testing"
)

// TestNotes: a journal text the window names is a note once, newest first;
// an older save's heard journal texts follow; only the player's note can
// be removed.
func TestNotes(t *testing.T) {
	e := &Engine{}
	e.ext.voiced = map[string]bool{"Proclamation_LIX": true, "Journal_Entry_26": true, "1a83648acff21ee4": true}
	e.journalPrinted("SEE JOURNAL ENTRY 26.", true)
	e.journalPrinted("SEE JOURNAL ENTRY 26.", true)
	e.ext.notes = append(e.ext.notes, noteRec{Title: "Slums", Place: "Slums, 3,4", Text: []string{"A NOTE"}, User: true})
	l := e.loreView()
	if len(l) != 3 || l[0].ID != 1 || l[0].Place != "Slums, 3,4" || l[1].Title != "Journal Entry 26" || l[1].ID != -1 ||
		l[2].Title != "Proclamation LIX" || !strings.HasPrefix(l[2].Text[0], "Be it known") {
		t.Fatalf("got %+v", l)
	}
	e.noteRemove(0)
	e.noteRemove(1)
	if len(e.ext.notes) != 1 || e.ext.notes[0].User {
		t.Fatalf("after remove %+v", e.ext.notes)
	}
	for _, ps := range journalTexts() {
		for _, p := range ps {
			if strings.HasPrefix(p, "<") {
				t.Fatalf("picture %q", p)
			}
		}
	}
}
