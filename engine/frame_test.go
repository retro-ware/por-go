package engine

import (
	"os"
	"testing"
)

// The text pane reaches the page's bottom: a clear of it on a page taller
// than 25 rows must not read as the main frame torn down (the remastered
// view stays up).
func TestTextPaneFollowsPage(t *testing.T) {
	if _, err := os.Stat("../data/dos/TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	e, err := New("../data")
	if err != nil {
		t.Fatal(err)
	}
	defer setPageSize(minCols, minRows)
	setPageSize(43, 27)
	e.DrawMainFrame(true)
	e.ClearBox(1, msgTop, winRight, winBottom)
	e.publish()
	if !e.View().MainFrame {
		t.Fatal("the text pane's clear tore the main frame")
	}
}
