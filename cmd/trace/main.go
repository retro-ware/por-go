// Command trace runs the Go engine headless with a key script and a fixed
// RandSeed and writes the same trace JSON as the emulator harness
// (decomp/emu/run.py game), for a mechanical comparison.
//
//	trace [-data DIR] [-seed N] [-keys "enter l a"] [-script FILE] [-out trace.json] [-png out.png] [-png-at N:out.png]
//
// Key tokens are those of decomp/emu/keys.py. Saves go to a scratch copy
// of DIR/SAVE, never to the real one; the files of $POR_SAVES (a scenario
// fixture) replace those of DIR/SAVE in that copy.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"por/ega"
	"por/engine"
	"por/pal"
)

func main() {
	data := flag.String("data", "", "game directory (default: the repo root above por-go)")
	seed := flag.String("seed", "0x12345678", "RandSeed set by Randomize")
	keys := flag.String("keys", "", "key tokens, space separated")
	script := flag.String("script", "", "key script file ('#' comments)")
	out := flag.String("out", "", "trace JSON (default stdout)")
	pngOut := flag.String("png", "", "screen PNG at the end")
	pngAt := flag.String("png-at", "", "N:PATH, screen PNG after the Nth trace record (as run.py --png-at)")
	flag.Parse()

	if *data == "" {
		wd, _ := os.Getwd()
		for d := wd; d != filepath.Dir(d); d = filepath.Dir(d) {
			if _, err := os.Stat(filepath.Join(d, "START.EXE")); err == nil {
				*data = d
				break
			}
		}
	}
	sd, err := strconv.ParseUint(*seed, 0, 32)
	if err != nil {
		log.Fatal(err)
	}
	toks := strings.Fields(*keys)
	if *script != "" {
		b, err := os.ReadFile(*script)
		if err != nil {
			log.Fatal(err)
		}
		var st []string
		for _, ln := range strings.Split(string(b), "\n") {
			st = append(st, strings.Fields(strings.SplitN(ln, "#", 2)[0])...)
		}
		toks = append(st, toks...)
	}
	ks, err := ParseKeys(toks)
	if err != nil {
		log.Fatal(err)
	}

	scr := ega.New()
	e, err := engine.New(*data, scr)
	if err != nil {
		log.Fatal(err)
	}
	tmp, err := os.MkdirTemp("", "por-trace-save")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	copyDir(filepath.Join(*data, "SAVE"), tmp)
	if fix := os.Getenv("POR_SAVES"); fix != "" { // scenario fixture, as in emu.py
		copyDir(fix, tmp)
	}
	e.Cfg.SaveDir = tmp

	atN, atPath := -1, ""
	if n, p, ok := strings.Cut(*pngAt, ":"); ok {
		atN, _ = strconv.Atoi(n)
		atPath = p
	}
	var recs [][]any
	e.StartTrace(func(addr, name string, args ...any) {
		r := append([]any{addr, name}, args...)
		recs = append(recs, r)
		if len(recs) == atN {
			writePNG(scr, atPath)
		}
	}, uint32(sd), ks)

	stop := run(e)
	fmt.Fprintln(os.Stderr, stop)
	res := map[string]any{
		"mode": "game", "side": "go", "seed": sd, "stop": stop, "keys": ks,
		"virtual_ms": e.TraceMS(), "keys_left": e.TraceKeysLeft(), "trace": recs, "state": e.TraceState(),
	}
	w := io.Writer(os.Stdout)
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		w = f
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Fatal(err)
	}
	if *pngOut != "" {
		writePNG(scr, *pngOut)
	}
	fmt.Fprintf(os.Stderr, "%d trace records, %d keys left, virtual %d ms\n", len(recs), e.TraceKeysLeft(), e.TraceMS())
}

func writePNG(scr *ega.Screen, path string) {
	img := image.NewRGBA(image.Rect(0, 0, pal.W, pal.H))
	for i, c := range scr.Pix {
		img.Set(i%pal.W, i/pal.W, scr.RGBA(c))
	}
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	png.Encode(f, img)
	f.Close()
}

// run plays the program body until the key script ends the run.
func run(e *engine.Engine) (stop string) {
	defer func() {
		if r := recover(); r != nil {
			if d, ok := r.(engine.TraceDone); ok {
				stop = d.Reason
				return
			}
			stop = fmt.Sprintf("panic: %v", r)
		}
	}()
	e.Run()
	return "returned"
}

func copyDir(src, dst string) {
	ents, _ := os.ReadDir(src)
	for _, de := range ents {
		if de.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, de.Name()))
		if err == nil {
			os.WriteFile(filepath.Join(dst, de.Name()), b, 0o644)
		}
	}
}
