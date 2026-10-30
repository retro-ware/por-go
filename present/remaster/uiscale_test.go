package remaster

import (
	"testing"

	"blobber"
	"gpu"
)

// TestUIScaleFits: whatever the Portraits and Minimap sizes, the eight cards
// and the map stay on screen and apart.
func TestUIScaleFits(t *testing.T) {
	defer func(o blobber.Options) { blobber.Settings = o }(blobber.Settings)
	for _, sz := range [][2]int{{640, 400}, {1904, 1204}, {3840, 2160}} {
		dst := gpu.NewImage(sz[0], sz[1])
		for _, c := range blobber.UISteps {
			for _, m := range blobber.UISteps {
				blobber.Settings.Cards, blobber.Settings.Map = c, m
				mr, at := mapRect(dst, 8), cardAt(dst)
				for i := range 8 {
					r := at(i, 8)
					if !r.In(dst.Bounds()) || r.Overlaps(mr) || i > 0 && i != 4 && r.Overlaps(at(i-1, 8)) {
						t.Fatalf("%v cards %v map %v: card %d %v, map %v", sz, c, m, i, r, mr)
					}
				}
			}
		}
		dst.Deallocate()
	}
}
