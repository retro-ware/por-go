package engine

import (
	"bufio"
	"encoding/binary"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// DASP (extra): the Dubtrain Angband Sound Pack, in data/assets/dasp (its
// WAVs and sound.cfg, which names the files of each Angband sound event),
// plays in place of the original's effects (daspFx) and on events the
// original kept silent (shops, training, doors, treasure, affects ...),
// while the Esc menu's "DASP Sfx" is on (Engine.Dasp). Each event plays one
// of its files at random, mixed over the speaker, Tandy or Amiga output on
// the effects' volume; the music plays on under it. Ctrl-S silences it too.

// daspDir is the pack's folder under the assets.
const daspDir = "dasp"

// daspFx names the pack's event that replaces each PlaySound number; the
// rest (a step) keep the original's sound.
var daspFx = map[int]string{
	sndSpell:      "cast_spell",
	sndHit:        "hit",
	sndMagicHit:   "hit_good",
	sndFall:       "kill",
	sndMissile:    "shoot",
	sndBump:       "hitwall",
	sndSpell51:    "res_elec", // Lightning Bolt
	sndMiss:       "miss",
	sndSpell47:    "breathe_fire", // Fireball
	sndMissileEnd: "shoot_hit",
}

// daspAffects names the event of an affect a character gets.
var daspAffects = map[affectType]string{
	affBless:           "blessed",
	affPrayer:          "blessed",
	affPrayerBonus:     "blessed",
	affCurse:           "cursed",
	affBestowCurse:     "cursed",
	affProtEvil:        "prot_evil",
	affProtGood:        "prot_evil",
	affProtEvilRadius:  "prot_evil",
	affProtGoodRadius:  "prot_evil",
	affResistCold:      "res_cold",
	affResistFire:      "res_fire",
	affCharmed:         "confused",
	affEnlarged:        "hero",
	affStrength:        "hero",
	affShield:          "shield",
	affProtMissiles:    "shield",
	affSilenced:        "stun",
	affDetectInvisible: "see_invis",
	affInvisible:       "teleport",
	affBlink:           "teleport",
	affEnfeebled:       "drain_stat",
	affWeakened:        "drain_stat",
	affCauseDisease:    "drain_stat",
	affCoughing:        "drugged",
	affHelpless:        "paralyzed",
	affHeld:            "paralyzed",
	affHeldFast:        "paralyzed",
	affParalyse2d8:     "paralyzed",
	affParalyseNotElf:  "paralyzed",
	affParalyse1d9:     "paralyzed",
	affAsleep:          "slow",
	affSlow:            "slow",
	affBlind:           "blind",
	affHaste:           "speed",
	affRepulsed:        "flee",
	affPoisoned:        "poisoned",
	affPoisonDamage:    "poisoned",
}

// daspWav is a decoded WAV: s16 samples, ch (1 or 2) per frame; no pcm when
// the file is missing or not 16-bit 44100 Hz.
type daspWav struct {
	pcm []int16
	ch  int
}

type daspClip struct {
	daspWav
	pos int // frames played
}

// dasp is the pack: events from sound.cfg, the WAVs decoded so far and the
// clips playing. Guarded by sound.mu.
type dasp struct {
	dir     string
	events  map[string][]string
	wavs    map[string]daspWav
	playing []daspClip
}

// loadDasp reads dir/sound.cfg; nil without it.
func loadDasp(dir string) *dasp {
	f, err := os.Open(filepath.Join(dir, "sound.cfg"))
	if err != nil {
		return nil
	}
	defer f.Close()
	d := &dasp{dir: dir, events: map[string][]string{}, wavs: map[string]daspWav{}}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if k = strings.TrimSpace(k); ok && !strings.HasPrefix(k, "#") {
			d.events[k] = strings.Fields(v)
		}
	}
	return d
}

// readWav decodes a 16-bit 44100 Hz mono or stereo WAV.
func readWav(path string) daspWav {
	b, err := os.ReadFile(path)
	if err != nil || len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return daspWav{}
	}
	le16 := binary.LittleEndian.Uint16
	ch := 0
	for i := 12; i+8 <= len(b); {
		n := int(binary.LittleEndian.Uint32(b[i+4:]))
		body := b[i+8 : min(i+8+n, len(b))]
		switch string(b[i : i+4]) {
		case "fmt ":
			if len(body) >= 16 && le16(body) == 1 && binary.LittleEndian.Uint32(body[4:]) == sndRate && le16(body[14:]) == 16 {
				ch = int(le16(body[2:]))
			}
		case "data":
			if ch != 1 && ch != 2 {
				return daspWav{}
			}
			pcm := make([]int16, len(body)/2)
			for j := range pcm {
				pcm[j] = int16(le16(body[2*j:]))
			}
			return daspWav{pcm, ch}
		}
		i += 8 + n + n&1
	}
	return daspWav{}
}

// dasp plays one of event's files and reports whether the pack took the
// sound: false when it is off or has nothing for event.
func (e *Engine) dasp(event string) bool {
	s := &e.snd
	if event == "" || !e.Dasp.Load() || !s.on || s.typ == sndNone {
		return false
	}
	s.mu.Lock()
	d := s.dasp
	var name string
	var w daspWav
	cached := false
	if d != nil && len(d.events[event]) > 0 {
		files := d.events[event]
		name = files[rand.IntN(len(files))] // not the game's dice: captures with POR_SEED stay the same
		w, cached = d.wavs[name]
	}
	s.mu.Unlock()
	if name == "" {
		return false
	}
	if !cached {
		w = readWav(filepath.Join(d.dir, name)) // outside the lock: the audio thread keeps going
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d.wavs[name] = w
	if len(w.pcm) == 0 {
		return false
	}
	// the same file started within 0.1 s (Bless on the whole party) plays once
	if !slices.ContainsFunc(d.playing, func(c daspClip) bool { return &c.pcm[0] == &w.pcm[0] && c.pos < sndRate/10 }) {
		d.playing = append(d.playing, daspClip{daspWav: w})
	}
	return true
}

// mix adds the clips playing to p (s16 stereo) on the effects' volume.
func (d *dasp) mix(p []byte) {
	if len(d.playing) == 0 {
		return
	}
	g := volume("sfx")
	for i := 0; i+3 < len(p); i += 4 {
		l := int(int16(binary.LittleEndian.Uint16(p[i:])))
		r := int(int16(binary.LittleEndian.Uint16(p[i+2:])))
		for k := range d.playing {
			c := &d.playing[k]
			if j := c.pos * c.ch; j+c.ch <= len(c.pcm) {
				l += int(float64(c.pcm[j]) * g)
				r += int(float64(c.pcm[j+c.ch-1]) * g)
				c.pos++
			}
		}
		binary.LittleEndian.PutUint16(p[i:], uint16(int16(max(-32768, min(32767, l)))))
		binary.LittleEndian.PutUint16(p[i+2:], uint16(int16(max(-32768, min(32767, r)))))
	}
	d.playing = slices.DeleteFunc(d.playing, func(c daspClip) bool { return (c.pos+1)*c.ch > len(c.pcm) })
}

// hitEvent is event, or a monster's hit when the target is one of the party.
func (e *Engine) hitEvent(target *Character, event string) string {
	if e.partyMember(target) {
		return "mon_hit"
	}
	return event
}

// fallEvent: a monster's death cry, or the party member's death.
func (e *Engine) fallEvent(c *Character) string {
	if e.partyMember(c) {
		return "death"
	}
	return "kill"
}

// moneySound plays the coins of picking up p, by its gold.
func (e *Engine) moneySound(p purse) {
	switch {
	case p.gold == 0 && p.copper == 0:
	case p.gold < 200:
		e.dasp("money1")
	case p.gold < 600:
		e.dasp("money2")
	default:
		e.dasp("money3")
	}
}
