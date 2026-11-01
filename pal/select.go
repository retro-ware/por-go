package pal

import (
	"image"
	"image/color"
)

// The two selection markers, for the selected and the hovered alike: the
// text selection is a blue (EGA 1) ground under a text line, menu or list
// row; the outline selection a green (EGA 10) frame round anything with a
// picture (an item box, a picker cell, a battle-bar button).

// SelectOutline marks picture r: fill paints a green frame t pixels thick
// along the inside of r.
func SelectOutline(r image.Rectangle, t int, fill func(image.Rectangle, color.RGBA)) {
	c := EGA(10)
	fill(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+t), c)
	fill(image.Rect(r.Min.X, r.Max.Y-t, r.Max.X, r.Max.Y), c)
	fill(image.Rect(r.Min.X, r.Min.Y, r.Min.X+t, r.Max.Y), c)
	fill(image.Rect(r.Max.X-t, r.Min.Y, r.Max.X, r.Max.Y), c)
}
