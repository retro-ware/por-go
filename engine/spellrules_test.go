package engine

import (
	"strings"
	"testing"
)

// TestSpellRulesAligned: every spell's rules start with its own name.
func TestSpellRulesAligned(t *testing.T) {
	for id := 1; id < len(spellNames); id++ {
		if !strings.HasPrefix(strings.ToLower(spellRules[id]), strings.ToLower(spellNames[id])+":") {
			t.Errorf("spell %d %q: rules %q", id, spellNames[id], spellRules[id])
		}
	}
}
