package engine

import "testing"

func TestTavernSpin(t *testing.T) {
	for _, c := range []struct {
		gold, wager uint32
		r           int
		want        uint32
	}{
		{100000, 5000, 9, 105000}, // green, past what a 16-bit word held
		{100000, 5000, 5, 100000}, // blue
		{100000, 5000, 2, 95000},  // red
		{100000, 5001, 9, 100000}, // over the house limit
		{40, 50, 9, 40},           // more than the party has
		{40, 0, 9, 40},
	} {
		if got, _ := tavernSpin(c.gold, c.wager, c.r); got != c.want {
			t.Errorf("tavernSpin(%d, %d, %d) = %d, want %d", c.gold, c.wager, c.r, got, c.want)
		}
	}
	if (&Engine{set: 3}).nativeScript() != nil {
		t.Fatal("native script at pc 0")
	}
}
