// Command amigasongs renders the Amiga music player's songs 1-7 to WAV
// files for listening: `amigasongs SOUND.BIN OUTDIR [SECONDS]`, e.g.
//
//	go run ./cmd/amigasongs data/amiga/SOUND.BIN /tmp/songs 120
//
// Each song plays for SECONDS (default 120) from a fresh player; trailing
// silence is cut, so a song that plays once ends where it stops.
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"por/amiga"
)

const rate = 44100

func main() {
	if len(os.Args) < 3 {
		log.Fatal("usage: amigasongs SOUND.BIN OUTDIR [SECONDS]")
	}
	img, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	secs := 120
	if len(os.Args) > 3 {
		if secs, err = strconv.Atoi(os.Args[3]); err != nil {
			log.Fatal(err)
		}
	}
	if err := os.MkdirAll(os.Args[2], 0o755); err != nil {
		log.Fatal(err)
	}
	for song := 1; song <= 7; song++ {
		p := amiga.New(img)
		p.Song(song)
		pcm := make([]byte, secs*rate*4) // s16 stereo
		p.Render(pcm, rate)
		end := len(pcm)
		for end >= 4 && binary.LittleEndian.Uint32(pcm[end-4:]) == 0 {
			end -= 4
		}
		pcm = pcm[:end]
		name := filepath.Join(os.Args[2], fmt.Sprintf("song%d.wav", song))
		if err := os.WriteFile(name, wav(pcm), 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s  %.1f s\n", name, float64(len(pcm))/4/rate)
	}
}

// wav wraps s16 stereo PCM in a RIFF header.
func wav(pcm []byte) []byte {
	h := make([]byte, 44)
	copy(h, "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+len(pcm)))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1) // PCM
	binary.LittleEndian.PutUint16(h[22:], 2)
	binary.LittleEndian.PutUint32(h[24:], rate)
	binary.LittleEndian.PutUint32(h[28:], rate*4)
	binary.LittleEndian.PutUint16(h[32:], 4)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(len(pcm)))
	return append(h, pcm...)
}
