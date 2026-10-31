package engine

import (
	"os"
	"strings"
	"testing"
)

// TestPhlanHistory: Rolf's labels parse, every chapter is there, its title
// fits the page, and every paragraph fits one page under the title (the
// pager keeps a voiced paragraph whole).
func TestPhlanHistory(t *testing.T) {
	names, err := ReadAreaNames("../data/assets/scripts", 0, true)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile("../data/assets/scripts/area0.txt")
	s, err := ParseScript(string(b), names)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range []string{"RolfEvent", "RolfHistory", "PhlanHistory1", "PhlanHistory6"} {
		if _, ok := s.Labels[l]; !ok {
			t.Error("area 0 has no label", l)
		}
	}
	const rows = 25 - 2 - 1 - jrTop - 2 // jrRows at 40x25, less the title and its blank line
	for _, c := range phlanChapters {
		lines := journalLayout(loadJournal("HISTORY.TXT"), []string{c}, true)
		if len(lines) < 3 || len(c) > jrWidth {
			t.Fatalf("%q: missing or its title is too long", c)
		}
		run := 0
		for _, l := range lines[2:] {
			if run++; l == "" {
				run = 0
			} else if run > rows {
				t.Errorf("%q: paragraph longer than a page at %q", c, strings.TrimLeft(l, "\x03"))
			}
		}
	}
}
