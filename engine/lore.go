package engine

import (
	"slices"
	"strings"
	"sync"
)

// Lore found (extra): the journal's Notes. Every voiced text is lore: a
// recording this save has played (e.ext.voiced, VOICED.GOB) is one the party
// has found, whether the voice is on or not. Published as View.Lore in the
// order of the texts: the journal's entries, Rolf's history, then the
// script texts (journal/LINES.TXT).

// Lore is a note: its title and paragraphs.
type Lore struct {
	Title string
	Text  []string
}

type loreText struct {
	key string
	Lore
}

// loreTexts are the voiced texts by key.
var loreTexts = sync.OnceValue(func() []loreText {
	var out []loreText
	for _, name := range []string{"JOURNAL.TXT", "HISTORY.TXT"} {
		j := loadJournal(name)
		titles := slices.Sorted(func(yield func(string) bool) {
			for t := range j {
				if !yield(t) {
					return
				}
			}
		})
		if name == "HISTORY.TXT" {
			titles = phlanChapters
		}
		for _, t := range titles {
			var paras []string
			for _, p := range j[t] {
				switch {
				case strings.HasPrefix(p, "<") && strings.HasSuffix(p, ">"):
				case p == "" || len(paras) == 0:
					paras = append(paras, p)
				default:
					paras[len(paras)-1] = strings.TrimSpace(paras[len(paras)-1] + " " + p)
				}
			}
			paras = slices.DeleteFunc(paras, func(p string) bool { return p == "" })
			out = append(out, loreText{strings.ReplaceAll(t, " ", "_"), Lore{t, paras}})
		}
	}
	b, _ := journalFS.ReadFile("journal/LINES.TXT")
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, loreText{voiceKey(l), Lore{loreTitle(l), []string{l}}})
		}
	}
	return out
})

// loreTitle names a script text by its first words (at most 19 letters: the
// journal list's width).
func loreTitle(s string) string {
	t := ""
	for _, w := range strings.Fields(strings.Trim(s, "'\" ")) {
		if len(t)+len(w)+1 > 19 {
			break
		}
		t = strings.TrimSpace(t + " " + w)
	}
	if t == "" {
		t = s[:min(len(s), 19)]
	}
	return t
}

// loreView is the Notes: the lore this save has found.
func (e *Engine) loreView() []Lore {
	var out []Lore
	for _, t := range loreTexts() {
		if e.ext.voiced[t.key] {
			out = append(out, t.Lore)
		}
	}
	return out
}
