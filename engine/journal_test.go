package engine

import (
	"reflect"
	"testing"
)

func TestJournalRefs(t *testing.T) {
	for s, want := range map[string][]string{
		"READ JOURNAL ENTRY 26.":                     {"Journal Entry 26"},
		"SEE ENTRIES 23 AND 14.":                     {"Journal Entry 23", "Journal Entry 14"},
		"YOU OVERHEAR TAVERN TALE 5":                 {"Tavern Tale 5"},
		"IN YOUR JOURNAL YOU NOTE PROCLAMATION LIX.": {"Proclamation LIX"},
		"YOU NOTE PROCLAMATIONCXXVI AND CX.":         {"Proclamation CXXVI", "Proclamation CX"},
		"PROCLAMATIONS LXIV, LXXVIII, CIX, AND LIX.": {"Proclamation LXIV", "Proclamation LXXVIII", "Proclamation CIX", "Proclamation LIX"},
		"PROCLAMATIONS ARE POSTED ON THE WALLS":      nil,
		"THE SENTRY 5 LOOKS":                         nil,
		"UNDER ENTRY ":                               nil,
	} {
		if got := journalRefs(s); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: %q, want %q", s, got, want)
		}
	}
}

func TestJournalPagesShrinkPicture(t *testing.T) {
	pic := func(rows int) []string {
		l := []string{"\x01P"}
		for range rows - 1 {
			l = append(l, "\x02")
		}
		return l
	}
	fill := func(n int) []string {
		l := []string{"\x00T"}
		for range n - 1 {
			l = append(l, "x")
		}
		return l
	}
	// 9 rows of text leave 10: over half of 19, so the picture shrinks in.
	p := journalPages(append(fill(9), pic(19)...))
	if len(p) != 1 || pictureRows(p[0][9:]) != 10 {
		t.Fatalf("shrink: %d pages, %q", len(p), p)
	}
	// 12 rows leave 7: too little, the picture starts the next page whole.
	p = journalPages(append(fill(12), pic(19)...))
	if len(p) != 2 || pictureRows(p[1]) != 19 {
		t.Fatalf("next page: %d pages, %q", len(p), p)
	}
}
