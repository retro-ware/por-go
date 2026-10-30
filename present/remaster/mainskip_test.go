package remaster

import (
	"por/engine"
	"testing"
	"time"
)

// A key or click during the title's hold or fade ends it and is swallowed;
// once the menu is in, input goes through.
func TestMainSkip(t *testing.T) {
	p := &Presenter{main: true}
	if p.mainFade() >= 1 {
		t.Fatal("fresh menu should be fading")
	}
	if !p.key(13, engine.View{}) {
		t.Fatal("Enter during the fade should be taken")
	}
	if p.mainFade() < 1 {
		t.Fatal("the fade should be over after the skip")
	}
	if p.mainSkip() {
		t.Fatal("a second key must not be swallowed")
	}
	p.mainAt = time.Now()
	if !p.Click(0, 0) || p.mainFade() < 1 {
		t.Fatal("a click during the hold should end it and be taken")
	}
}
