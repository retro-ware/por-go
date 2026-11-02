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

// ClueBook puts up the chapter list; a chapter chosen is read in the
// pager, then the list comes back; Exit goes back to adventuring.
func (e *Engine) ClueBook() {
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
		e.after(func() {
			e.showJournal("Clue Book", journalLayout(loadJournal("CLUEBOOK.TXT"), titles[index:], false))
		}, e.ClueBook)
	})
}
