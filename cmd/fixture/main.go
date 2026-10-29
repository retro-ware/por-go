// Command fixture builds a save fixture for a trace scenario
// (decomp/emu/saves/NAME): save A of SAVE/ with its party, moved into
// another area. The area's ECL block replaces the saved script, as when a
// script enters it (ovr04:0BD1 opNewECL), and the party stands at x,y.
//
//	fixture -out DIR [-data DIR] [-block N -walls A,B,C [-geo G]] [-x X -y Y -dir D] [-set VAR=VAL ...] [-strong]
//
// -walls are the WALLDEF blocks of wall slots 1..3 as SaveGame records them
// (255 none; a block of two chunks fills its slot and the next, which is
// then 255), as the area's LOAD PIECES leaves them. LoadGame reloads them.
// -set pokes an ECL variable (0x4900.. area record, 0x6B00.. game record)
// as a word. -strong makes every member hit always and take a long time to
// die, so a scripted Quick fight ends in a win.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"por/dax"
	"por/engine"
)

type sets []string

func (s *sets) String() string     { return strings.Join(*s, ",") }
func (s *sets) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	data := flag.String("data", "", "game directory (default: data/ in the nearest folder above holding data/dos/TITLE.DAX, as cmd/trace)")
	out := flag.String("out", "", "fixture directory to write")
	block := flag.Int("block", -1, "ECL block to enter (-1 keeps the saved area)")
	x := flag.Int("x", -1, "party x")
	y := flag.Int("y", -1, "party y")
	dir := flag.Int("dir", -1, "facing 0 N, 2 E, 4 S, 6 W")
	geo := flag.Int("geo", -1, "GEO block to load (default: the ECL block)")
	walls := flag.String("walls", "", "WALLDEF blocks of wall slots 1,2,3 (255 none)")
	strong := flag.Bool("strong", false, "make the party win fights")
	var vars sets
	flag.Var(&vars, "set", "VAR=VAL, hex, repeatable")
	flag.Parse()
	if *out == "" {
		log.Fatal("-out is required")
	}
	root := *data
	if root == "" {
		wd, _ := os.Getwd()
		for d := wd; d != filepath.Dir(d); d = filepath.Dir(d) {
			if _, err := os.Stat(filepath.Join(d, "data", "dos", "TITLE.DAX")); err == nil {
				root = filepath.Join(d, "data")
				break
			}
		}
	}
	sv, err := os.ReadFile(filepath.Join(root, engine.SaveDir, "SAVGAMA.DAT"))
	if err != nil {
		log.Fatal(err)
	}
	s, err := engine.OpenSave(sv)
	if err != nil {
		log.Fatalf("%s/SAVGAMA.DAT: %v", engine.SaveDir, err)
	}

	if *block >= 0 {
		l, err := dax.NewLoader(filepath.Join(root, engine.DOSDir))
		if err != nil {
			log.Fatal(err)
		}
		disk := 0
		var b []byte
		for d := 1; d <= 8 && disk == 0; d++ {
			if b = l.Block(fmt.Sprintf("ECL%d", d), *block); len(b) > 2 {
				disk = d
			}
		}
		if disk == 0 {
			log.Fatalf("no ECL file has block %d", *block)
		}
		g := *geo
		if g < 0 {
			g = *block // most areas load the GEO block of their own number
		}
		ws := strings.Split(*walls, ",")
		if len(ws) != 3 {
			log.Fatal("-block needs -walls A,B,C")
		}
		var ids [3]int
		for i, w := range ws {
			if ids[i], err = strconv.Atoi(w); err != nil {
				log.Fatalf("bad -walls %q", *walls)
			}
		}
		s.EnterArea(disk, *block, g, b[2:], ids)
	}
	s.SetPos(*x, *y, *dir)
	for _, kv := range vars {
		k, v, ok := strings.Cut(kv, "=")
		a, err1 := strconv.ParseUint(k, 16, 16)
		n, err2 := strconv.ParseUint(v, 16, 16)
		if !ok || err1 != nil || err2 != nil {
			log.Fatalf("bad -set %q", kv)
		}
		if err := s.SetVar(int(a), uint16(n)); err != nil {
			log.Fatalf("-set %s: %v", k, err)
		}
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "SAVGAMA.DAT"), s.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	for _, name := range s.Party() {
		for _, ext := range []string{".SAV", ".SPC", ".ITM"} {
			b, err := os.ReadFile(filepath.Join(root, engine.SaveDir, name+ext))
			if err != nil {
				continue
			}
			if ext == ".SAV" && *strong {
				b = engine.MakeStrong(b)
			}
			if err := os.WriteFile(filepath.Join(*out, name+ext), b, 0o644); err != nil {
				log.Fatal(err)
			}
		}
	}
}
