package engine

import "testing"

// TestBeastOf: MON*CHA names find their bestiary kind; others none.
func TestBeastOf(t *testing.T) {
	for name, want := range map[string]string{"KOBOLD LEADER": "Kobold", "EFREETI": "Efreeti", "DISPLACER BEAST": "Displacer Beast", "HUGE SCORPION": "Giant Scorpion", "BANDIT": ""} {
		got := ""
		if b := beastOf(name); b != nil {
			got = b.Name
		}
		if got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}
