// Command eclscript translates ECL byte code into readable scripts:
// `eclscript DIR` writes every block of ECL1..ECL8.DAX (the data files
// beside the working directory's parent, as cmd/ecl) to
// DIR/ECL<DISK>_<BLOCK>.txt, one command and its operands per line,
// jump targets as labels.
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
	l, err := dax.NewLoader("..")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
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
			var out bytes.Buffer
			engine.ScriptECL(b[2:], &out)
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s_%d.txt", name, block)), out.Bytes(), 0o644); err != nil {
				log.Fatal(err)
			}
		}
	}
}
