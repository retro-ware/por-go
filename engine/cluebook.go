package engine

import "strings"

// The clue book (extra): TSR's Pool of Radiance clue book (journal/
// CLUEBOOK.TXT, its maps CLUE*.PNG), opened with B while exploring: a list
// of its chapters, the chosen one read on to the end of the book in the
// pager, Esc back to the list.

// clueChapters are CLUEBOOK.TXT's titles in the book's order.
func clueChapters() []string {
	b, _ := journalFS.ReadFile("journal/CLUEBOOK.TXT")
	var t []string
	for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]") {
			t = append(t, l[1:len(l)-1])
		}
	}
	return t
}

// ClueBook opens the book again on the page last read, if it was left
// before its end; else (and on Esc there) the chapter list.
func (e *Engine) ClueBook() {
	if pg := e.ext.clue; pg != nil && pg.p < len(journalPages(pg.lines)) {
		e.after(func() { e.pushPager(modeJournal, pg) }, e.clueList)
		return
	}
	e.clueList()
}

// clueList puts up the chapter list; a chapter chosen is read in the
// pager, then the list comes back; Exit goes back to adventuring.
func (e *Engine) clueList() {
	titles := clueChapters()
	var list []*ListNode
	for _, t := range titles {
		list = append(list, &ListNode{Text: cut(t, textCols)})
	}
	redraw, index := true, 0
	var sel *ListNode
	e.newPage()
	e.blank(1, 1, winRight, winBottom)
	e.ListMenu("Clue Book", nil, 0, 0, 0, 1, 4, 38, 22, list, true, &redraw, &index, &sel, func(Pick) {
		if sel == nil {
			e.DrawMainScreen()
			return
		}
		e.ext.clue = &pager{title: "Clue Book", lines: journalLayout(loadJournal("CLUEBOOK.TXT"), titles[index:], false), journal: true}
		e.after(func() { e.pushPager(modeJournal, e.ext.clue) }, e.clueList)
	})
}
