package main

import (
	"testing"

	"por/engine"
)

// The input that opens a screen never acts on it: held through the change
// it is ignored until let go, then works again.
func TestLatch(t *testing.T) {
	var l latch
	l.see(screenOf(engine.View{}, false, false), nil)
	if !l.down("left", true) {
		t.Fatal("a press on the same screen should count")
	}
	l.see(screenOf(engine.View{}, true, false), []any{"left"}) // the click opened the inventory
	if l.down("left", true) {
		t.Fatal("the click that opened the screen acted on it")
	}
	l.see(screenOf(engine.View{}, true, false), nil) // let go
	if !l.down("left", true) {
		t.Fatal("a new press on the new screen should count")
	}
	l.see(screenOf(engine.View{}, true, false), []any{"left"}) // the same screen: nothing turns stale
	if !l.down("left", true) {
		t.Fatal("a held input on an unchanged screen was dropped")
	}
}
