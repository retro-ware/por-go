package engine

import (
	"strings"
	"testing"
)

// TestLoreView: the recordings heard are the Notes, a journal entry by its
// title (pictures left out), a script text by its first words.
func TestLoreView(t *testing.T) {
	line := "'DON'T GO THERE! DON'T GO ON THE HILL!'"
	e := &Engine{}
	e.ext.voiced = map[string]bool{voiceKey(line): true, "Proclamation_LIX": true, "The_Moonsea_Reaches": true, "nothing": true}
	l := e.loreView()
	if len(l) != 3 || l[0].Title != "Proclamation LIX" || !strings.HasPrefix(l[0].Text[0], "Be it known") ||
		l[1].Title != "The Moonsea Reaches" || l[2].Title != "DON'T GO THERE!" || l[2].Text[0] != line {
		t.Fatalf("got %+v", l)
	}
	for _, n := range loreTexts() {
		for _, p := range n.Text {
			if strings.HasPrefix(p, "<") {
				t.Fatalf("%s: picture %q", n.Title, p)
			}
		}
	}
}
