package engine

import (
	"os"
	"testing"
)

// TestVorbisPCM: a 0.1 s mono 44100 Hz tone decodes to s16 stereo with
// both channels equal.
func TestVorbisPCM(t *testing.T) {
	b, err := os.ReadFile("testdata/tone.ogg")
	if err != nil {
		t.Fatal(err)
	}
	pcm, err := vorbisPCM(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(pcm) != 4410*4 {
		t.Fatalf("got %d bytes, want %d", len(pcm), 4410*4)
	}
	for i := 0; i < len(pcm); i += 4 {
		if pcm[i] != pcm[i+2] || pcm[i+1] != pcm[i+3] {
			t.Fatalf("frame %d: channels differ", i/4)
		}
	}
}

// voice/render.py names the files with the same hash.
func TestVoiceKey(t *testing.T) {
	if k := voiceKey("  YOU ARE IN THE MERCHANTS' CAMP AREA. "); k != "5913b4acb1eb5288" {
		t.Fatal(k)
	}
}
