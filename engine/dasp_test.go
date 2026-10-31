package engine

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// TestDasp: sound.cfg's events, a mono WAV decoded and mixed onto both
// channels, a clip dropped once played.
func TestDasp(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sound.cfg"), []byte("# hit = no.wav\n[Sound]\nhit = a.wav b.wav\nwalk =\n"), 0o644)
	wav := []byte("RIFF\x00\x00\x00\x00WAVEfmt \x10\x00\x00\x00")
	wav = binary.LittleEndian.AppendUint16(wav, 1) // PCM
	wav = binary.LittleEndian.AppendUint16(wav, 1) // mono
	wav = binary.LittleEndian.AppendUint32(wav, sndRate)
	wav = binary.LittleEndian.AppendUint32(wav, sndRate*2)
	wav = binary.LittleEndian.AppendUint16(wav, 2)
	wav = binary.LittleEndian.AppendUint16(wav, 16)
	wav = append(wav, "data\x04\x00\x00\x00"...)
	wav = binary.LittleEndian.AppendUint16(wav, 1000)
	wav = binary.LittleEndian.AppendUint16(wav, uint16(0xFFFF-499)) // -500
	os.WriteFile(filepath.Join(dir, "a.wav"), wav, 0o644)

	d := loadDasp(dir)
	if got := d.events["hit"]; len(got) != 2 || got[0] != "a.wav" || len(d.events["walk"]) != 0 || len(d.events) != 2 {
		t.Fatalf("events %v", d.events)
	}
	w := readWav(filepath.Join(dir, "a.wav"))
	if w.ch != 1 || len(w.pcm) != 2 || w.pcm[1] != -500 {
		t.Fatalf("wav %+v", w)
	}
	if readWav(filepath.Join(dir, "b.wav")).pcm != nil {
		t.Fatal("a missing file decoded")
	}
	d.playing = []daspClip{{daspWav: w}}
	p := make([]byte, 12) // three stereo frames, the third past the clip
	binary.LittleEndian.PutUint16(p, 32000)
	d.mix(p)
	want := []int16{32767, 1000, -500, -500, 0, 0} // clamped, both channels
	for i, v := range want {
		if got := int16(binary.LittleEndian.Uint16(p[2*i:])); got != v {
			t.Errorf("sample %d = %d, want %d", i, got, v)
		}
	}
	if len(d.playing) != 0 {
		t.Error("the clip did not end")
	}
}
