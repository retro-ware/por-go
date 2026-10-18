package engine

import (
	"encoding/binary"
	"log"
	"os"
)

// Init = ovr12:002D: allocate the global records, set the start state and
// load the always-resident assets. Everything the DOS code zeroed with
// FillChar is zero-valued here.
func (e *Engine) Init() {
	e.game, e.area, e.aux = gameVars{}, areaVars{}, auxVars{} // DS:49D6, 49D2, 49DA
	e.ecl = make([]byte, 0x1E00)                              // DS:49DE, ECL code at pc-0x9900
	e.posX, e.posY, e.facing = 0xF, 1, 6                      // DS:6AAD/6AAE/6AAF
	e.eclPC = 0x9900                                          // DS:49ED
	e.msgDelay = 1                                            // DS:49E2
	e.set = 1                                                 // DS:5376 = 1
	e.viewMode, e.prevViewMode = 1, 1
	e.saved = false
	e.viewBit0, e.viewBit1 = 1, 1                // DS:49F5/49F6
	e.slowText = true                            // DS:4A00
	e.listTop = 1                                // DS:6B29
	e.mapPairA[1], e.mapPairA[2] = -1, -1        // DS:6AAE+4i, i = 2..3
	e.mapPairB[1], e.mapPairB[2] = -1, -1        // DS:6AB0+4i
	e.es.skyShade, e.es.groundShade = 0xE8, 0xDB // DS:6F69/6F68

	e.font = e.L.Block(e.set.file("8X8D"), 201) // 0709:017C
	e.ClearRow24()
	e.DisplayString(0, promptRow, colLightGreen, "Loading...Please Wait")
	e.LoadSymbolSet(4, 202) // ovr35:003B
	e.LoadSymbolSet(0, 203)
	e.sqrpaci[0] = e.LoadPic("SQRPACI", 1) // 0464:0111 → DS:5E52
	e.sqrpaci[1] = e.LoadPic("SQRPACI", 2) // DS:5E56
	for i := 0; i <= 0xB; i++ {
		e.LoadIconPic("COMSPR", i, i+0xD) // ovr33:01C8
	}
	e.LoadIconPic("COMSPR", 0x19, 0x19)
	e.items = e.readItems() // ITEMS file, 2-byte header then the records → DS:5582
	e.ClearRow24()
	e.Randomize()           // 0AF8:1247
	e.spellTableInit()      // ovr23:3413
	e.setupAffectHandlers() // fills the affect handler table
}

// readItems reads the base item table: skips the 2-byte count and takes up
// to itemTypes records (a short or missing file leaves blank ones).
func (e *Engine) readItems() []itemBase {
	items := make([]itemBase, itemTypes)
	b, err := os.ReadFile(e.L.Path("ITEMS"))
	if err != nil || len(b) < 2 {
		// extra: without the table no class may use any item and nothing has stats
		log.Println("ITEMS (the base item table) is missing from the game data: import the game again")
		return items
	}
	b = append(b[2:], make([]byte, itemTypes*16)...)
	binary.Decode(b, binary.LittleEndian, items)
	return items
}

// InitGraphics loads the font and symbol sets 4 and 0 the way Init does
// (for tools that skip Init).
func (e *Engine) InitGraphics() {
	e.font = e.L.Block(e.set.file("8X8D"), 201)
	e.LoadSymbolSet(4, 202)
	e.LoadSymbolSet(0, 203)
}
