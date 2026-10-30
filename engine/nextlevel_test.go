package engine

import "testing"

func TestLevelSpan(t *testing.T) {
	var c Character
	c.Levels[clsFighter], c.Levels[clsMage] = 1, 1 // a fighter/magic-user: the fighter's 2001 comes first
	if from, to := levelSpan(&c); from != 0 || to != 2001 {
		t.Errorf("level 1: %d..%d, want 0..2001", from, to)
	}
	c.Levels[clsFighter], c.Levels[clsMage] = 3, 0
	if from, to := levelSpan(&c); from != 4001 || to != 8001 {
		t.Errorf("level 3: %d..%d, want 4001..8001", from, to)
	}
	c.Levels[clsFighter] = 13 // past the table
	if _, to := levelSpan(&c); to != 0 {
		t.Errorf("at the top: %d, want 0", to)
	}
}
