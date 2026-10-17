package sfx

import (
	"math"
	"testing"
)

func TestSynth(t *testing.T) {
	if f := freq("A4"); f != 440 {
		t.Fatal(f)
	}
	if f := freq("C5"); math.Abs(f-523.25) > 0.1 {
		t.Fatal(f)
	}
	if freq("-") != 0 || Song("nope") != nil {
		t.Fatal("rest / unknown track")
	}
	for name := range Sounds {
		s := Render(name)
		peak := float32(0)
		for _, v := range s {
			peak = max(peak, v, -v)
		}
		if len(s) == 0 || peak == 0 || peak > 1 {
			t.Errorf("%s: %d samples, peak %v", name, len(s), peak)
		}
	}
	for name := range Tracks {
		if len(Song(name)) == 0 {
			t.Errorf("track %s empty", name)
		}
	}
	if len(pcm(make([]float32, 10))) != 80 {
		t.Fatal("pcm size")
	}
}
