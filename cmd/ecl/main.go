// Command ecl lists one ECL script block: `ecl DISK BLOCK` reads block
// BLOCK of ECL<DISK>.DAX from data/dos; run it from the repo root.
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"por/dax"
	"por/engine"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: ecl DISK BLOCK")
	}
	disk, _ := strconv.Atoi(os.Args[1])
	block, _ := strconv.Atoi(os.Args[2])
	l, err := dax.NewLoader("data/dos")
	if err != nil {
		log.Fatal(err)
	}
	b := l.Block(fmt.Sprintf("ECL%d", disk), block)
	if len(b) < 2 {
		log.Fatalf("ECL%d has no block %d", disk, block)
	}
	engine.ListECL(b[2:], os.Stdout)
}
