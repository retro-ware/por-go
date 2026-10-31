package engine

import "strings"

// Rolf's history of Phlan (extra): after the tour, Rolf waits in New Phlan
// at 2,2 (area 0, RolfEvent) and tells the Adventurer's Journal's "A History
// of Phlan and the Moonsea Reaches" by Jeff Grubb, a chapter at a time
// (journal/HISTORY.TXT), in the pager, voiced a paragraph at a time.

// phlanChapters are HISTORY.TXT's titles in the order of Rolf's menu.
var phlanChapters = []string{
	"The Moonsea Reaches",
	"The Ancient City of Phlan",
	"Greathammer & the First Rebirth",
	"The Fall of Phlan",
	"The Reemergence of Phlan",
	"Phlan Today",
}

// phlanHistory = area 0 PhlanHistory1..6: shows chapter n, voiced every
// time it is asked for, then goes back to Rolf's menu.
func (e *Engine) phlanHistory(n int) {
	lines := journalLayout(loadJournal("HISTORY.TXT"), phlanChapters[n:n+1], true)
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "\x00"):
			delete(e.ext.voiced, strings.ReplaceAll(l[1:], " ", "_"))
		case strings.HasPrefix(l, "\x03"):
			key, _, _ := strings.Cut(l[1:], "\x03")
			delete(e.ext.voiced, key)
		}
	}
	e.scriptText(true, "")
	e.showJournal("A History of Phlan", lines)
	e.eclPC = e.scriptLabel("RolfHistory")
}
