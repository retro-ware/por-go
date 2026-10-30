package engine

import "sync"

// vol (extra) is the Esc menu's volume sliders, 0 to 1.
var vol = struct {
	sync.Mutex
	music, sfx, voice float64
}{music: 1, sfx: 1, voice: 1}

// SetVolumes sets the sliders of the music, the sound effects and the voice-over.
func SetVolumes(music, sfx, voice float64) {
	vol.Lock()
	vol.music, vol.sfx, vol.voice = music, sfx, voice
	vol.Unlock()
}

// volume is how loud kind ("music", "sfx" or "voice") plays: the goplay
// desk's fader (the voice-over goes on the effects' one) times its slider.
func volume(kind string) float64 {
	vol.Lock()
	defer vol.Unlock()
	switch kind {
	case "music":
		return desk("music") * vol.music
	case "voice":
		return desk("sfx") * vol.voice
	}
	return desk("sfx") * vol.sfx
}
