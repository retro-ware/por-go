package engine

import "testing"

func TestNextLevelExp(t *testing.T) {
	var c Character
	c.Levels[clsFighter], c.Levels[clsMage] = 1, 1 // a fighter/magic-user: the fighter's 2001 comes first
	if got := nextLevelExp(&c); got != 2001 {
		t.Errorf("next level: %d, want 2001", got)
	}
	c.Levels[clsFighter], c.Levels[clsMage] = 13, 0 // past the table
	if got := nextLevelExp(&c); got != 0 {
		t.Errorf("at the top: %d, want 0", got)
	}
}
