package sprites

import "testing"

// Every sprite must decode to something visible and only use palette characters.
func TestArt(t *testing.T) {
	for name, src := range art {
		img := Decode(src)
		n := 0
		for _, ch := range []byte(src) {
			if _, ok := palette[ch]; !ok && ch != '.' && ch != '\n' {
				t.Errorf("%s: unknown pixel %q", name, ch)
			}
		}
		b := img.Bounds()
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				if img.RGBAAt(x, y).A > 0 {
					n++
				}
			}
		}
		if n < 20 || b.Dx() > 24 || b.Dy() > 24 {
			t.Errorf("%s: %dx%d with %d pixels", name, b.Dx(), b.Dy(), n)
		}
	}
}
