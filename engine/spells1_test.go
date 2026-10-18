package engine

import "testing"

func TestSpellTableInit(t *testing.T) {
	e := &Engine{}
	e.spellTableInit()
	for i := 1; i < len(e.spellFx); i++ {
		if e.spellFx[i] == nil {
			t.Fatalf("spell %d has no effect", i)
		}
	}
}
