package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/jfreymuth/oggvorbis"
)

// Voice-over (extra): story texts, journal texts and first visits are read
// aloud from data/assets/speech/<key>.ogg (Ogg Vorbis, rendered by voice/render.py,
// 44100 Hz). On unless switched off in the remastered view's Esc options; each text speaks once per save
// (VOICED.GOB beside SEEN.GOB); Enter or Esc silences it. The Amiga music is
// muted while it speaks.

// voiceOffFile in the save folder marks the voice as switched off.
const voiceOffFile = "VOICE.OFF"

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

func (e *Engine) voiceOn() bool { return !e.marked(voiceOffFile) }

// marked reports whether the marker file name exists in the save folder: a
// switch the player set (VOICE.OFF, SOUND.OFF, MINIMAP.ON) kept across runs.
func (e *Engine) marked(name string) bool {
	_, err := os.Stat(filepath.Join(e.Cfg.SaveDir, name))
	return err == nil
}

// mark creates or removes the marker file name in the save folder.
func (e *Engine) mark(name string, on bool) {
	p := filepath.Join(e.Cfg.SaveDir, name)
	if !on {
		os.Remove(p)
		return
	}
	os.MkdirAll(e.Cfg.SaveDir, 0o755)
	os.WriteFile(p, nil, 0o644)
}

// ToggleVoice switches the voice when toggle and reports whether it is on.
func (e *Engine) ToggleVoice(toggle bool) bool {
	if toggle {
		e.setVoiceOn(!e.voiceOn())
	}
	return e.voiceOn()
}

func (e *Engine) setVoiceOn(on bool) {
	e.mark(voiceOffFile, !on)
	if !on {
		e.stopVoice()
	}
}

// speak queues the text's recording, unless the voice is off, there is no
// recording or this save has heard it.
func (e *Engine) speak(key string) {
	if e.ext.voiced[key] || !e.voiceOn() {
		return
	}
	b, err := os.ReadFile(filepath.Join(e.data, AssetsDir, "speech", key+".ogg"))
	if err != nil {
		return
	}
	pcm, err := vorbisPCM(b)
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

// speaking reports whether a recording is queued or playing.
func (v *voice) speaking() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.queue) > 0
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
	scale(p[:n], volume("voice"))
	return n, nil
}

// vorbisPCM decodes an Ogg Vorbis file (mono or stereo, 44100 Hz) to s16
// stereo PCM.
func vorbisPCM(b []byte) ([]byte, error) {
	f, format, err := oggvorbis.ReadAll(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if format.SampleRate != 44100 || format.Channels > 2 {
		return nil, fmt.Errorf("voice: %d Hz, %d channels, want 44100 Hz mono or stereo", format.SampleRate, format.Channels)
	}
	pcm := make([]byte, 0, len(f)/format.Channels*4)
	for i := 0; i < len(f); i += format.Channels {
		l, r := f[i], f[i+format.Channels-1]
		pcm = binary.LittleEndian.AppendUint16(pcm, uint16(int16(max(-1, min(1, l))*32767)))
		pcm = binary.LittleEndian.AppendUint16(pcm, uint16(int16(max(-1, min(1, r))*32767)))
	}
	return pcm, nil
}
