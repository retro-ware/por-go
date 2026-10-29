package engine

import (
	"image"
	"image/color"
	"testing"
)

// A square PNG is both poses; one twice as wide is cut in the middle, the
// left half ready, the right action, mirrored to face left; other shapes
// are refused.
func TestIconPNG(t *testing.T) {
	wide := image.NewRGBA(image.Rect(0, 0, 4, 2))
	wide.Set(0, 0, color.White) // ready: top left
	wide.Set(3, 1, color.White) // action: bottom right
	var c Character
	if c.setIconPNG(image.NewRGBA(image.Rect(0, 0, 3, 1))) || c.Icon != nil {
		t.Fatal("3:1 taken")
	}
	if !c.setIconPNG(wide) {
		t.Fatal("2:1 refused")
	}
	at := func(im image.Image, x, y int) bool { _, _, _, a := im.At(x, y).RGBA(); return a != 0 }
	if r := c.iconPNG(0, false); r.Bounds().Dx() != 2 || !at(r, 0, 0) || at(r, 1, 1) {
		t.Fatal("ready is not the left half")
	}
	if a := c.iconPNG(1, false); !at(a, 1, 1) || at(a, 0, 0) {
		t.Fatal("action is not the right half")
	}
	if a := c.iconPNG(1, true); !at(a, 0, 1) {
		t.Fatal("the flip did not mirror")
	}
	if !c.setIconPNG(image.NewRGBA(image.Rect(0, 0, 2, 2))) || c.iconPNG(0, false) != c.iconPNG(1, false) {
		t.Fatal("a square is not both poses, cached")
	}
}
