// Command eclscript translates ECL byte code into readable scripts:
// `eclscript DIR` (DIR is data/assets/scripts) writes every area script of the ECL<n>.DAX files (in data/dos;
// run it from the repo root, as cmd/ecl) to
// DIR/area<BLOCK>.txt, one command and its operands per line, jump
// targets as labels. A block found on several disks is written once.
//
// Variables are named from the engine's own names (engine.ScriptVarNames)
// and the names files DIR/names/global.txt (every area) and
// DIR/names/area<BLOCK>.txt (that area only, over global), and the jump
// labels' names DIR/names/labels/area<BLOCK>.txt, lines of
// "XXXX Name" (script address in hex, then the name; # starts a comment).
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"por/dax"
	"por/engine"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: eclscript DIR")
	}
	dir := os.Args[1]
	l, err := dax.NewLoader("data/dos")
	if err != nil {
		log.Fatal(err)
	}
	global := engine.ScriptVarNames()
	readNames(filepath.Join(dir, "names", "global.txt"), global)
	done := map[int][]byte{}
	for disk := 1; disk <= 8; disk++ {
		name := fmt.Sprintf("ECL%d", disk)
		if !l.Exists(name + ".DAX") {
			continue
		}
		for block := 0; block < 256; block++ {
			b := l.Block(name, block)
			if len(b) < 2 {
				continue
			}
			if old, ok := done[block]; ok {
				if !bytes.Equal(old, b) {
					log.Printf("%s block %d differs from an earlier disk's; kept the first", name, block)
				}
				continue
			}
			done[block] = b
			area := fmt.Sprintf("area%d", block)
			names := maps.Clone(global)
			readNames(filepath.Join(dir, "names", area+".txt"), names)
			readNames(filepath.Join(dir, "names", "labels", area+".txt"), names)
			seen := map[string]int{}
			for a, n := range names {
				if b, ok := seen[n]; ok {
					log.Fatalf("%s: %04X and %04X are both named %s", area, a, b, n)
				}
				seen[n] = a
			}
			var out bytes.Buffer
			engine.ScriptECL(b[2:], &out, names)
			if err := os.WriteFile(filepath.Join(dir, area+".txt"), out.Bytes(), 0o644); err != nil {
				log.Fatal(err)
			}
		}
	}
}

// readNames adds the "XXXX Name" lines of a names file, if there is one.
func readNames(path string, names map[int]string) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return
	} else if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line, _, _ := strings.Cut(sc.Text(), "#")
		fs := strings.Fields(line)
		if len(fs) == 0 {
			continue
		}
		a, err := strconv.ParseUint(fs[0], 16, 16)
		if len(fs) != 2 || err != nil {
			log.Fatalf("%s:%d: want \"XXXX Name\"", path, n)
		}
		names[int(a)] = fs[1]
	}
	if err := sc.Err(); err != nil {
		log.Fatal(err)
	}
}
