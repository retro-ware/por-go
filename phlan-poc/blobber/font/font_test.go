package font

import "testing"

func TestMetrics(t *testing.T) {
	for _, r := range "AZaz09?!.,-:" {
		if glyphs[r] == nil || Advance(r) < 2 || Advance(r) > 12 {
			t.Errorf("%q: advance %d", r, Advance(r))
		}
	}
	if Width("") != 0 || Width("A") >= Width("AA") || Width("a b") != Advance('a')+space+Advance('b')-track {
		t.Fatal("width")
	}
	if get('一') != glyphs['?'] {
		t.Fatal("fallback")
	}
}
