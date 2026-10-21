package engine

import "testing"

// voice/render.py names the files with the same hash.
func TestVoiceKey(t *testing.T) {
	if k := voiceKey("  YOU ARE IN THE MERCHANTS' CAMP AREA. "); k != "5913b4acb1eb5288" {
		t.Fatal(k)
	}
}
