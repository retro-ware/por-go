package engine

import (
	"os"
	"testing"
)

// The 3D screen's text pane keeps the original's place on a page larger
// than 40x25: its clear must not read as the main frame torn down (the
// remastered view stays up, its pop-up the pane's size).
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
	e.ClearBox(1, msgTop, paneRight, paneBottom)
	e.publish()
	if v := e.View(); !v.MainFrame || TextPane != (CellRect{1, msgTop, 0x26, 0x16}) {
		t.Fatalf("frame %v pane %v", v.MainFrame, TextPane)
	}
}
