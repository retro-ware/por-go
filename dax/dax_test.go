package dax

import "testing"

func TestSiblings(t *testing.T) {
	got := siblings("MON3CHA")
	if len(got) != 7 || got[0] != "MON1CHA" || got[2] != "MON4CHA" {
		t.Errorf("MON3CHA: %v", got)
	}
	if got := siblings("8X8D3.DAX"); got[0] != "8X8D1.DAX" {
		t.Errorf("8X8D3.DAX: %v", got)
	}
	if siblings("TITLE") != nil {
		t.Error("TITLE has no disk")
	}
}
