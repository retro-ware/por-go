// Command eclscript translates ECL byte code into readable scripts:
// `eclscript DIR` (DIR is data/assets/scripts) writes every area script
// of the ECL<n>.DAX files (in data/dos; run it from the repo root, as
// cmd/ecl) to DIR/area<BLOCK>.txt in the form DIR/README.md describes
// (engine.DecodeScript, engine.WriteScript). A block found on several
// disks is written once.
//
// Variables are named from the engine's own names (engine.ScriptVarNames)
// and the names files DIR/names/global.txt (every area) and
// DIR/names/area<BLOCK>.txt (that area only, over global), and the jump
// labels' names DIR/names/labels/area<BLOCK>.txt, lines of
// "XXXX Name" (script address in hex, then the name; # starts a comment).
// It writes DIR/names/values.txt from the game data (monster and item
// names) and names the operands of a kind by it, names/pictures.txt and
// names/areas.txt (names/README.md); the conditions and some command
// pairs are folded (engine/scriptfold.go).
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
	var vals bytes.Buffer
	vals.WriteString("# Made by cmd/eclscript from the game data: the monsters (MON<set>CHA) and\n# item types (ITEM<set>) by set. \"Kind Set Number Name\"; a name two\n# numbers share has its number on.\n")
	for set := 1; set <= 8; set++ {
		for _, v := range engine.DataValues(l, set) {
			fmt.Fprintf(&vals, "%s %d %d %s\n", v.Kind, v.Set, v.N, v.Name)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "names", "values.txt"), vals.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
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
			names, err := engine.ReadAreaNames(dir, block, true)
			if err != nil {
				log.Fatal(err)
			}
			names.Treasure = func(v int) string { return engine.TreasureItems(l, names.Set, v) }
			seen := map[string]int{}
			for a, n := range names.Vars {
				if b, ok := seen[n]; ok {
					log.Fatalf("%s: %04X and %04X are both named %s", area, a, b, n)
				}
				seen[n] = a
			}
			var out bytes.Buffer
			if err := engine.ScriptECL(b[2:], &out, names); err != nil {
				log.Fatalf("%s: %v", area, err)
			}
			if err := os.WriteFile(filepath.Join(dir, area+".txt"), out.Bytes(), 0o644); err != nil {
				log.Fatal(err)
			}
		}
	}
}
