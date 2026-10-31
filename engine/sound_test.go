package engine

import (
	"testing"

	"por/dax"
)

// A song started on the Tandy set reaches the SN76489, Ctrl-S mutes it.
func TestTandySound(t *testing.T) {
	l, err := dax.NewLoader(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{L: l, Cfg: Config{Sound: 'T'}}
	r := e.SoundReader()
	e.soundWait(0)
	e.PlaySound(sndUnmute)
	e.PlaySound(sndSpell)
	buf := make([]byte, 4*sndRate/2)
	r.Read(buf)
	loud := false
	for i := 0; i < len(buf); i += 4 {
		loud = loud || buf[i] != 0 || buf[i+1] != 0
	}
	if !loud {
		t.Fatal("Tandy song is silent")
	}
	e.soundToggle()
	if e.snd.typ != sndNone || e.snd.song != 0 {
		t.Fatalf("Ctrl-S: typ %d song %d", e.snd.typ, e.snd.song)
	}
	e.soundToggle()
	if e.snd.typ != sndTandy {
		t.Fatalf("Ctrl-S again: typ %d", e.snd.typ)
	}
}

// Every sound the game plays is audible on the PC speaker and on the Tandy.
func TestEverySoundAudible(t *testing.T) {
	for _, typ := range []byte{'P', 'T'} {
		for n := sndSpell; n <= sndMissileEnd; n++ {
			l, err := dax.NewLoader(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			e := &Engine{L: l, Cfg: Config{Sound: typ}}
			r := e.SoundReader()
			e.soundWait(0)
			e.PlaySound(n)
			buf := make([]byte, 4*sndRate)
			r.Read(buf)
			loud := false
			for i := 0; i < len(buf); i += 4 {
				loud = loud || buf[i] != 0 || buf[i+1] != 0
			}
			if !loud {
				t.Errorf("sound %d on %c is silent", n, typ)
			}
		}
	}
}

// With "Step sounds" off a step or a turn is silent.
func TestNoSteps(t *testing.T) {
	l, err := dax.NewLoader(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{L: l, Cfg: Config{Sound: 'P'}}
	e.NoSteps.Store(true)
	r := e.SoundReader()
	e.soundWait(0)
	e.PlaySound(sndStep)
	buf := make([]byte, 4*sndRate)
	r.Read(buf)
	for i := 0; i < len(buf); i++ {
		if buf[i] != 0 {
			t.Fatal("a step sounds with Step sounds off")
		}
	}
}

// The program's jumps stay inside it.
func TestSoundProgram(t *testing.T) {
	for i, op := range sndProgram {
		if (op.kind == sndCall || op.kind == sndLoop) && (op.arg <= 0 || op.arg >= len(sndProgram)) {
			t.Errorf("op %d goes to %d", i, op.arg)
		}
		if op.kind == sndSwitch && (op.arg < 0 || op.arg >= sndChannels) {
			t.Errorf("op %d switches to channel %d", i, op.arg)
		}
		if op.kind == sndSet && op.field == sndVibrato && op.arg >= len(sndVibratos) {
			t.Errorf("op %d sets vibrato %d", i, op.arg)
		}
	}
}
