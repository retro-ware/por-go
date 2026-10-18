package engine

import (
	"encoding/binary"
	"fmt"
	"reflect"
)

// The script variable records (DS:49D2 area, DS:49D6 game, DS:49DA aux).
// The ECL scripts address them only as words, by number (area word i is
// script address 0x4900+i, game 0x6B00+i, aux 0x9700+i), and SAVGAM*.DAT
// stores them word for word, so every field is a uint16 and the structs
// have no padding. The named fields are the words the engine itself uses,
// each with its byte offset in the record (the "area+0x186" of the
// listings); the Vxxxx arrays are the script-only words, named by the
// script address of their first word.

// areaVars = the area record (es:[DS:49D2]), reloaded with each area.
type areaVars struct {
	V4900       [0xC3]uint16
	MapX, MapY  uint16    // +0x186/+0x188 party square on the area map (ovr15:0818)
	Geo         uint16    // +0x18A GEO block of the 3D map (ovr30:10C7)
	Clock       [7]uint16 // +0x18C rounds, minutes, tens, hours, days, months, years (ovr21:03D5)
	V49CD       [0x18]uint16
	NoMagic     uint16 // +0x1CA nonzero: no spellcasting here (ovr16)
	In3D        uint16 // +0x1CC nonzero: a 3D area; zero: the overland map (ovr27:016D)
	WallSplitA  uint16 // +0x1CE with WallSplitB set, opLoadWalls loads into slots 1 and 3
	WallSplitB  uint16 // +0x1D0
	V49E9       [7]uint16
	PrevX       uint16 // +0x1E0 square before the step (ovr04:3395: the step sound)
	PrevY       uint16 // +0x1E2
	AreaKind    uint16 // +0x1E4 area number (ECL block), DS:5D7D copy
	V49F3       [8]uint16
	NoCoords    uint16 // +0x1F6 nonzero: no x,y on the status line, no Area map (00BA:290B)
	MsgDelay    uint16 // +0x1F8 text delay, DS:49E2 copy for the save file
	Sky         uint16 // +0x1FA sky colour (ovr27:016D)
	SkyInside   uint16 // +0x1FC sky colour on map cells > 0x7F
	ViewFlags   uint16 // +0x1FE save-file copy of DS:49F6 (bit 1) and DS:49F5 (bit 0)
	V4A00       [0x7C]uint16
	MarkFlags4  uint16 // +0x2F8 bit 3: area map mark 4 shown (ovr15 area map)
	V4A7D       [0xF]uint16
	Mark3       uint16 // +0x318 != 0xFF: area map mark 3 shown
	V4A8D       [0x12]uint16
	Mark4Off    uint16 // +0x33E zero: area map mark 4 shown
	MarkFlags57 uint16 // +0x340 bit 1 clear: mark 5, bit 0 clear: mark 7
	V4AA1       uint16
	RiverOpen   uint16 // +0x344 nonzero: the river can be crossed; set in a loaded game: EndgameHook(0)
	V4AA3       [0x10]uint16
	Boss        uint16 // +0x366 > 0xFD: marks 8/9; 0xFE: EndgameHook(1); 0xFF: combat terrain kind
	V4AB4       [0x1E]uint16
	Mark4       uint16 // +0x3A4 nonzero: area map mark 4 shown
	V4AD3       [0x27]uint16
	// +0x3F4 the wall slots' walldef numbers and +0x3FA their WALLDEF
	// blocks in the save file only: saveSlot overlays e.mapPairA/B here
	// and loadSlot reads them back and zeroes them. In the original the
	// last word was also the view animation flag (Engine.viewAnim).
	WallsA, WallsB [3]uint16
	V4B00          [0x200]uint16
}

// gameVars = the game record (es:[DS:49D6]), kept for the whole game.
type gameVars struct {
	V6B00        [0x2A8]uint16
	PartyExp     uint16 // +0x550 the classes a training hall teaches (bits)
	V6DA9        [0x17]uint16
	MaxDist      uint16 // +0x580 opSetupMonster's distance (ovr04:03B4)
	MonsterDist  uint16 // +0x582 the encounter's distance, counted down by opApproach
	V6DC2        [4]uint16
	Morale       uint16 // +0x58C 0..100, the monsters' flee threshold (ovr09)
	CombatEnd    uint16 // +0x58E 0x80 the party died, 0x81 it fled (ovr06:1527)
	Beaten       uint16 // +0x590 monsters beaten (ovr06:11AC)
	HoldMove     uint16 // +0x592 >= 0xFF: the next move is skipped (ovr15:0D6D)
	Search       uint16 // +0x594 bit 0 sweeping, bit 1 search this square
	Surprise     uint16 // +0x596 bit side+1: that side is surprised (ovr14)
	V6DCC        [6]uint16
	RestEvery    uint16 // +0x5A4 rest encounter check every n turns (ovr21)
	RestChance   uint16 // +0x5A6 its percent chance
	V6DD4        uint16
	LeftMap      uint16 // +0x5AA the step left the map (ovr15:068D)
	V6DD6        [0xB]uint16
	Portrait     uint16 // +0x5C2 HEAD/BODY pair shown instead of a picture, 0xFF none
	Temple       uint16 // +0x5C4 1: opCombat opens the temple
	NoLoot       uint16 // +0x5C6 1: the monsters drop no items
	V6DE4        [2]uint16
	Duel         uint16 // +0x5CC DuelSetup's n (ovr08:1A6F)
	V6DE7        [0x2B]uint16
	SymbolSet    uint16 // +0x624 the 8X8D<n> symbol set
	V6E13        [0x20]uint16
	AttackedAlly uint16 // +0x666 the party attacked an ally this fight
	V6E34        [0xA]uint16
	PartyCount   uint16 // +0x67C
	V6E3F        [0x2D]uint16
	Shop         uint16 // +0x6D8 1: opCombat opens the shop
	V6E6D        [3]uint16
	MonsterHit   uint16 // +0x6E0 low byte: signed to-hit bonus of the monsters
	PartyHit     uint16 // +0x6E2 low byte: signed to-hit bonus of the party
	PartyMove    uint16 // +0x6E4 movement bonus of the party
	V6E73        [0x8D]uint16
}

// auxVars = the aux record (es:[DS:49DA]), script words only.
type auxVars [0x200]uint16

// word is script word i of the record: the fields are walked in order,
// a uint16 counting one word and a [n]uint16 n (ecl.go). The record sizes
// (0x800 bytes each) are checked by TestVarOffsets.
func (a *areaVars) word(i int) *uint16 { return recordWord(a, i) }
func (g *gameVars) word(i int) *uint16 { return recordWord(g, i) }

func recordWord(rec any, i int) *uint16 {
	v := reflect.ValueOf(rec).Elem()
	for f := range v.NumField() {
		fv := v.Field(f)
		n := 1
		if fv.Kind() == reflect.Array {
			n = fv.Len()
		}
		if i < n {
			if fv.Kind() == reflect.Array {
				fv = fv.Index(i)
			}
			return fv.Addr().Interface().(*uint16)
		}
		i -= n
	}
	panic(fmt.Sprintf("%T has no script word %d", rec, i))
}

// recordBytes is a record (a pointer to areaVars, gameVars or auxVars) in
// its file layout, little-endian word for word (saves use saveFile).
func recordBytes(rec any) []byte {
	b, err := binary.Append(nil, binary.LittleEndian, rec)
	if err != nil {
		panic(err)
	}
	return b
}
