package engine

import "testing"

// TestRuneLetter decodes the Sokal Keep scroll (ECL1 block 21) to the words
// the keep's script asks for.
func TestRuneLetter(t *testing.T) {
	for want, l := range map[string][]byte{
		"LUX":     {0x56, 0x6b, 0x71},
		"SAMOSUD": {0x67, 0x40, 0x58, 0x5c, 0x67, 0x6b, 0x46},
		"SHESTNI": {0x67, 0x4e, 0x48, 0x67, 0x69, 0x5a, 0x50},
	} {
		var s []byte
		for _, c := range l {
			s = append(s, runeLetter(c+1))
		}
		if string(s) != want {
			t.Errorf("got %s, want %s", s, want)
		}
	}
}
