// Command geodump writes block BLOCK of GEO<DISK>.DAX (without its 2-byte
// header) to OUT: `geodump DISK BLOCK OUT`, run from the repo root (reads data/dos).
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"por/dax"
)

func main() {
	if len(os.Args) != 4 {
		log.Fatal("usage: geodump DISK BLOCK OUT")
	}
	disk, _ := strconv.Atoi(os.Args[1])
	block, _ := strconv.Atoi(os.Args[2])
	l, err := dax.NewLoader("data/dos")
	if err != nil {
		log.Fatal(err)
	}
	b := l.Block(fmt.Sprintf("GEO%d", disk), block)
	if len(b) < 2+0x400 {
		log.Fatalf("GEO%d has no block %d", disk, block)
	}
	if err := os.WriteFile(os.Args[3], b[2:2+0x400], 0o644); err != nil {
		log.Fatal(err)
	}
}
