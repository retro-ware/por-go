package engine

import (
	"testing"
)

// The 3D screen's text pane keeps the original's place on a page larger
// than 40x25: its clear must not read as the main frame torn down (the
// remastered view stays up, its pop-up the pane's size).
func TestTextPaneFollowsPage(t *testing.T) {
	e := testEngine(t)
	defer setPageSize(minCols, minRows)
	setPageSize(43, 27)
	e.beginFlag = 4
	e.clearPane()
	e.publish()
	if v := e.View(); v.Pane == nil || TextPane != (CellRect{1, msgTop, 0x26, 0x16}) {
		t.Fatalf("frame %v pane %v", v.Pane != nil, TextPane)
	}
}
