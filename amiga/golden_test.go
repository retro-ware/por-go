package amiga

import (
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/golden.txt")

// TestGolden plays every song and effect of the game's sound image
// (POR_SOUND = a DISK2/SOUND.BIN made by por -import-disk2) and compares a
// hash of the chip writes and the rendered audio with testdata/golden.txt.
func TestGolden(t *testing.T) {
	path := os.Getenv("POR_SOUND")
	if path == "" {
		t.Skip("POR_SOUND not set")
	}
	img, err := os.ReadFile(path)
	if err != nil || len(img) != End-Base {
		t.Fatalf("%s: %v (%d bytes)", path, err, len(img))
	}
	var got strings.Builder
	play := func(name string, start func(p *Player), secs int) {
		p := New(append([]byte(nil), img...))
		h := sha256.New()
		p.Trace = func(off, size int, v uint32) {
			binary.Write(h, binary.LittleEndian, [3]uint32{uint32(off), uint32(size), v})
		}
		start(p)
		out := make([]byte, 4*22050)
		for range secs {
			p.Render(out, 22050)
			h.Write(out)
		}
		fmt.Fprintf(&got, "%s %x\n", name, h.Sum(nil)[:8])
	}
	for n := 1; n <= 8; n++ {
		play(fmt.Sprintf("song%d", n), func(p *Player) { p.Song(n) }, 40)
	}
	for n := 0; n <= 14; n++ {
		play(fmt.Sprintf("effect%d", n), func(p *Player) { p.Effect(n) }, 4)
	}
	play("song-then-effect", func(p *Player) {
		p.Song(2)
		p.Render(make([]byte, 4*22050*3), 22050)
		p.Effect(5)
	}, 4)

	if *update {
		if err := os.WriteFile("testdata/golden.txt", []byte(got.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile("testdata/golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Errorf("output changed:\n%s\nwant:\n%s", got.String(), want)
	}
}
