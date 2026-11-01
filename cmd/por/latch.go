package main

import (
	"fmt"

	"por/engine"
)

// A hard UI rule: the input that changes the screen never acts on the
// screen it opens. When the screen changes, every key, mouse button, pad
// button and stick direction still held is stale: ignored until it is let
// go and pressed again (a held click does not drag on the new screen, a
// held arrow does not repeat into a new menu).

// latch tracks the held inputs that are stale; an input is any comparable
// key (gpu.Key, a mouse button, a pad button, a stick direction).
type latch struct {
	screen string
	stale  map[any]bool
}

// see starts a frame on screen (screenOf) with the inputs held now: a
// stale input no longer held is fresh again; when the screen differs from
// the last one, every input held turns stale.
func (l *latch) see(screen string, held []any) {
	now := map[any]bool{}
	for _, h := range held {
		now[h] = true
	}
	if screen != l.screen {
		l.screen, l.stale = screen, now
		return
	}
	for k := range l.stale {
		if !now[k] {
			delete(l.stale, k)
		}
	}
}

// down is whether input in, pressed or not, counts as held: a stale one
// never does.
func (l *latch) down(in any, pressed bool) bool { return pressed && !l.stale[in] }

// screenOf names the screen v shows (and whether the presenter's own
// inventory or the options page is up): it changes whenever a screen opens or closes, or a
// question pops up.
func screenOf(v engine.View, inventory, options bool) string {
	ask := -1
	if v.Ask != nil {
		ask = int(v.Ask.Kind)
	}
	return fmt.Sprint(inventory, options, v.Inventory != nil, v.Sheet != nil, v.SheetPage != nil, v.Camp != nil, v.Temple != nil,
		v.Shop != nil, v.Training != nil, v.Creation != nil, v.Picker != nil, v.Slots != nil, v.IconEditor != nil,
		v.Pages != nil, v.FullMap != nil, v.Overview, v.MainMenu != nil, v.Results != nil, v.Battle != nil, v.Credits, ask)
}
