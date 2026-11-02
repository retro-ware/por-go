package engine

import (
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Notes (extra): the journal's second tab, published as View.Lore, newest
// first. The game adds the printed journal's texts the text window names
// (journal entries, proclamations, tavern tales: journalPrinted); the
// player adds what the text window shows (ActNote, the text box's "> Notes"),
// kept with the map and square it was read on, and may remove those again
// (ActNoteRemove). Kept with the save (NOTES.GOB). A save from before the
// notes lists the journal texts it has heard (VOICED.GOB).

// noteRec is a note: a journal text by Title, or the player's (User) Text
// read at Place.
type noteRec struct {
	Title, Place string
	Text         []string
	User         bool
}

// Lore is a note in the journal: ID is its number for ActNoteRemove (-1: not
// removable), Place where the player read it ("" a journal text).
type Lore struct {
	Title, Place string
	Text         []string
	ID           int
}

// journalTexts are JOURNAL.TXT's texts by title, paragraphs joined and
// pictures left out.
var journalTexts = sync.OnceValue(func() map[string][]string {
	out := map[string][]string{}
	for t, ps := range loadJournal("JOURNAL.TXT") {
		var paras []string
		for _, p := range ps {
			switch {
			case strings.HasPrefix(p, "<") && strings.HasSuffix(p, ">"):
			case p == "" || len(paras) == 0:
				paras = append(paras, p)
			default:
				paras[len(paras)-1] = strings.TrimSpace(paras[len(paras)-1] + " " + p)
			}
		}
		out[t] = slices.DeleteFunc(paras, func(p string) bool { return p == "" })
	}
	return out
})

// noteJournal adds the journal text titled t, once, and says so.
func (e *Engine) noteJournal(t string) {
	if _, ok := journalTexts()[t]; !ok || slices.ContainsFunc(e.ext.notes, func(n noteRec) bool { return !n.User && n.Title == t }) {
		return
	}
	e.ext.notes = append(e.ext.notes, noteRec{Title: t})
	e.say("Note added to journal")
}

// paneNote is the text pane's text as one paragraph ("" none).
func (e *Engine) paneNote() string {
	var w []string
	for _, l := range e.pane().Lines {
		for _, r := range l {
			w = append(w, strings.Fields(r.Text)...)
		}
	}
	return strings.Join(w, " ")
}

// noteUser adds what the text pane shows, with the map and square, once.
func (e *Engine) noteUser() {
	s := e.paneNote()
	if s == "" {
		return
	}
	if slices.ContainsFunc(e.ext.notes, func(n noteRec) bool { return n.User && slices.Equal(n.Text, []string{s}) }) {
		e.say("Already in the journal")
		return
	}
	m := e.mapName()
	e.ext.notes = append(e.ext.notes, noteRec{Title: m, Place: m + ", " + strconv.Itoa(int(e.posX)) + "," + strconv.Itoa(int(e.posY)), Text: []string{s}, User: true})
	e.say("Note added to journal")
}

// noteRemove removes the player's note id.
func (e *Engine) noteRemove(id int) {
	if id >= 0 && id < len(e.ext.notes) && e.ext.notes[id].User {
		e.ext.notes = slices.Delete(e.ext.notes, id, id+1)
	}
}

// loreView is the Notes, newest first, then the journal texts an older save
// has heard.
func (e *Engine) loreView() []Lore {
	var out []Lore
	for i, n := range slices.Backward(e.ext.notes) {
		if n.User {
			out = append(out, Lore{n.Title, n.Place, n.Text, i})
		} else {
			out = append(out, Lore{n.Title, "", journalTexts()[n.Title], -1})
		}
	}
	var old []string
	for t := range journalTexts() {
		if e.ext.voiced[strings.ReplaceAll(t, " ", "_")] && !slices.ContainsFunc(e.ext.notes, func(n noteRec) bool { return !n.User && n.Title == t }) {
			old = append(old, t)
		}
	}
	slices.Sort(old)
	for _, t := range old {
		out = append(out, Lore{t, "", journalTexts()[t], -1})
	}
	return out
}
