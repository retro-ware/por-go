package engine

import (
	"bytes"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/hajimehoshi/go-mp3"
)

// Voice-over (extra): story texts, journal texts and first visits are read
// aloud from SPEECH/<key>.mp3 in the game folder (rendered by voice/render.py,
// 44100 Hz). Opt-in from the remastered view's Esc options; each text speaks once per save
// (VOICED.GOB beside SEEN.GOB); Enter or Esc silences it.

// voiceOnFile in the save folder marks the voice as switched on.
const voiceOnFile = "VOICE.ON"

type voice struct {
	mu    sync.Mutex
	queue [][]byte // s16 stereo PCM, the first one playing
}

// voiceKey names a script text's file: FNV-64a of the trimmed text, in hex
// (voice/render.py computes the same).
func voiceKey(s string) string {
	h := fnv.New64a()
	h.Write([]byte(strings.TrimSpace(s)))
	return strconv.FormatUint(h.Sum64(), 16)
}

func (e *Engine) voiceOn() bool {
	_, err := os.Stat(filepath.Join(e.Cfg.SaveDir, voiceOnFile))
	return err == nil
}

// ToggleVoice switches the voice when toggle and reports whether it is on.
func (e *Engine) ToggleVoice(toggle bool) bool {
	if toggle {
		e.setVoiceOn(!e.voiceOn())
	}
	return e.voiceOn()
}

func (e *Engine) setVoiceOn(on bool) {
	p := filepath.Join(e.Cfg.SaveDir, voiceOnFile)
	if on {
		os.MkdirAll(e.Cfg.SaveDir, 0o755)
		os.WriteFile(p, nil, 0o644)
		return
	}
	os.Remove(p)
	e.stopVoice()
}

// speak queues the text's recording, unless the voice is off, there is no
// recording or this save has heard it.
func (e *Engine) speak(key string) {
	if e.tr != nil || e.ext.voiced[key] || !e.voiceOn() {
		return
	}
	b, err := os.ReadFile(filepath.Join(e.L.Dir, "SPEECH", key+".mp3"))
	if err != nil {
		return
	}
	d, err := mp3.NewDecoder(bytes.NewReader(b))
	if err != nil {
		return
	}
	pcm, err := io.ReadAll(d)
	if err != nil {
		return
	}
	if e.ext.voiced == nil {
		e.ext.voiced = map[string]bool{}
	}
	e.ext.voiced[key] = true
	e.vox.mu.Lock()
	e.vox.queue = append(e.vox.queue, pcm)
	e.vox.mu.Unlock()
}

func (e *Engine) stopVoice() {
	e.vox.mu.Lock()
	e.vox.queue = nil
	e.vox.mu.Unlock()
}

// VoiceReader is the voice's audio source, in SoundReader's format.
func (e *Engine) VoiceReader() io.Reader { return &e.vox }

func (v *voice) Read(p []byte) (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	n := len(p) / 4 * 4
	o := 0
	for o < n && len(v.queue) > 0 {
		c := copy(p[o:n], v.queue[0])
		o += c
		if v.queue[0] = v.queue[0][c:]; len(v.queue[0]) == 0 {
			v.queue = v.queue[1:]
		}
	}
	clear(p[o:n])
	scale(p[:n], volume("sfx"))
	return n, nil
}
