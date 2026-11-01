package remaster

import "testing"

func TestLogInk(t *testing.T) {
	for in, want := range map[string]string{
		"Ronja attacks Gnoll and misses": "Ronja attacks Gnoll and misses",
		"for 12 points of damage":        "for \x0212\x02 points of damage",
		"Spell: Hold Person":             "\x01Spell: Hold Person\x01",
		"backstab for 7":                 "\x01backstab for \x01\x027\x02\x01\x01",
	} {
		if got := logInk(in); got != want {
			t.Errorf("logInk(%q) = %q, want %q", in, got, want)
		}
	}
}
