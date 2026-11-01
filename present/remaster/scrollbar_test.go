package remaster

import (
	"image"
	"testing"
)

func TestScrollBar(t *testing.T) {
	b := scrollBar{image.Rect(0, 100, 6, 200), 40, 10, 0}
	if th := b.thumb(); th.Min.Y != 100 || th.Max.Y != 125 {
		t.Errorf("thumb at the top = %v", th)
	}
	b.top = 30
	if th := b.thumb(); th.Max.Y != 200 {
		t.Errorf("thumb at the end = %v", th)
	}
	for _, c := range []struct{ y, top int }{{100, 0}, {50, 0}, {150, 15}, {200, 30}, {500, 30}} {
		if got := b.to(c.y); got != c.top {
			t.Errorf("to(%d) = %d, want %d", c.y, got, c.top)
		}
	}
}
