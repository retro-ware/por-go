package remaster

import (
	"image"
	"testing"
)

func TestCell(t *testing.T) {
	p := &Presenter{rows: []shownRows{{image.Rect(100, 500, 100+320*3, 500+8*3), image.Rect(0, 0xC0, 320, 0xC8)}}}
	if x, y, ok := p.Cell(100+3*8*5+1, 501); !ok || x != 5 || y != 0x18 {
		t.Errorf("got %d %d %v, want 5 24 true", x, y, ok)
	}
	if _, _, ok := p.Cell(0, 0); ok {
		t.Error("off the rows with a menu up: no cell")
	}
	p.noMenu = true
	if _, _, ok := p.Cell(0, 0); !ok {
		t.Error("press a key: any click")
	}
}
