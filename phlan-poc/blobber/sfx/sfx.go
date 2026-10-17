// Package sfx synthesizes the game's sound effects and music: no audio files, just tones and noise.
// Nothing plays until Init is called, so tests and headless runs stay silent.
package sfx

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

const rate = 44100

// tone is one segment of a sound: a frequency sweep from f0 to f1 Hz over dur seconds.
// wave is "sine", "square" or "noise"; volume decays linearly to 0 if decay is set.
type tone struct {
	f0, f1, dur float64
	wave        string
	decay       bool
}

// Sounds are the named effects for the `sound` opcode. Unknown names play "click".
var Sounds = map[string][]tone{
	"click":   {{900, 900, 0.03, "square", true}},
	"coins":   {{1300, 1300, 0.06, "square", true}, {1750, 1750, 0.12, "square", true}},
	"door":    {{120, 60, 0.45, "noise", true}},
	"trap":    {{400, 40, 0.6, "noise", true}},
	"hit":     {{200, 80, 0.12, "noise", true}},
	"magic":   {{300, 1200, 0.5, "sine", true}},
	"bell":    {{880, 880, 0.9, "sine", true}},
	"lever":   {{150, 150, 0.05, "square", true}, {0, 0, 0.08, "", false}, {110, 110, 0.12, "noise", true}},
	"fanfare": {{523, 523, 0.15, "square", false}, {659, 659, 0.15, "square", false}, {784, 784, 0.15, "square", false}, {1047, 1047, 0.5, "square", true}},
	"splash":  {{2000, 300, 0.4, "noise", true}},
}

// Tracks are looping tunes for the `music` opcode: note names with octave ("C4", "A#3") or "-" for a rest,
// one per beat. "stop" (or "") ends the music.
var Tracks = map[string]struct {
	bpm   float64
	notes string
}{
	"dungeon": {140, "A3 C4 E4 C4 A3 C4 E4 C4 G3 B3 D4 B3 G3 B3 D4 B3 F3 A3 C4 A3 E3 G#3 B3 G#3"},
	"cave":    {90, "D3 - F3 - A3 - G3 F3 E3 - C3 - D3 - - -"},
	"town":    {160, "C4 E4 G4 E4 F4 A4 C5 A4 G4 B4 D5 B4 C5 G4 E4 C4"},
	"lab":     {200, "C4 D#4 G4 A#4 G4 D#4 C4 - F4 G#4 C5 D#5 C5 G#4 F4 -"},
	"victory": {180, "C4 C4 C4 C4 G#3 A#3 C4 - A#3 C4 - - - -"},
}

var (
	ctx   *audio.Context
	music *audio.Player
	cur   string
)

// Init opens the audio device. Call it once from the game; without it Play and Music do nothing.
func Init() { ctx = audio.NewContext(rate) }

// Play plays a named sound effect.
func Play(name string) {
	if ctx == nil {
		return
	}
	ctx.NewPlayerF32FromBytes(pcm(Render(name))).Play()
}

// Music switches to a named looping track; "stop", "" or an unknown name stops the music.
// Asking for the track already playing keeps it going.
func Music(name string) {
	if ctx == nil || name == cur {
		return
	}
	if music != nil {
		music.Close()
		music, cur = nil, ""
	}
	s := Song(name)
	if s == nil {
		if name != "stop" && name != "" {
			fmt.Println("sfx: unknown track", name)
		}
		return
	}
	b := pcm(s)
	p, err := ctx.NewPlayerF32(audio.NewInfiniteLoopF32(bytes.NewReader(b), int64(len(b))))
	if err != nil {
		fmt.Println("sfx:", err)
		return
	}
	p.SetVolume(0.35)
	p.Play()
	music, cur = p, name
}

// Render synthesizes a sound effect as mono samples in -1..1.
func Render(name string) []float32 {
	ts, ok := Sounds[name]
	if !ok {
		ts = Sounds["click"]
	}
	var out []float32
	for _, t := range ts {
		out = append(out, synth(t)...)
	}
	return out
}

// Song synthesizes one loop of a track, or nil if there is no such track.
func Song(name string) []float32 {
	tr, ok := Tracks[name]
	if !ok {
		return nil
	}
	beat := 60 / tr.bpm
	var out []float32
	for _, n := range strings.Fields(tr.notes) {
		f := freq(n)
		t := tone{f, f, beat, "square", true}
		if f == 0 {
			t.wave = ""
		}
		out = append(out, synth(t)...)
	}
	return out
}

// freq converts "A4", "C#3" to Hz; anything else is a rest (0).
func freq(n string) float64 {
	semis := map[byte]int{'C': -9, 'D': -7, 'E': -5, 'F': -4, 'G': -2, 'A': 0, 'B': 2}
	if len(n) < 2 {
		return 0
	}
	s, ok := semis[n[0]]
	if !ok {
		return 0
	}
	if n[1] == '#' {
		s, n = s+1, n[1:]
	}
	oct := int(n[1] - '0')
	return 440 * math.Pow(2, float64(s+12*(oct-4))/12)
}

func synth(t tone) []float32 {
	n := int(t.dur * rate)
	out := make([]float32, n)
	phase := 0.0
	for i := range out {
		k := float64(i) / float64(n)
		phase += (t.f0 + (t.f1-t.f0)*k) / rate
		var v float64
		switch t.wave {
		case "sine":
			v = math.Sin(2 * math.Pi * phase)
		case "square":
			v = 0.5
			if math.Mod(phase, 1) < 0.5 {
				v = -0.5
			}
		case "noise":
			v = rand.Float64()*2 - 1
		}
		amp := 0.4
		if t.decay {
			amp *= 1 - k
		}
		if i < 64 { // tiny attack to avoid clicks
			amp *= float64(i) / 64
		}
		out[i] = float32(v * amp)
	}
	return out
}

// pcm turns mono samples into the stereo float32 little-endian stream ebiten's audio wants.
func pcm(s []float32) []byte {
	b := make([]byte, 0, len(s)*8)
	for _, v := range s {
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
	}
	return b
}
