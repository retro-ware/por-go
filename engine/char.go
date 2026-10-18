package engine

import (
	"bytes"
	"encoding/binary"
	"strconv"
)

// Character is a party member or a monster: its record (the DOS file
// layout), its item and affect lists, its combat block and readied items.
type Character struct {
	charRecord
	Items   []*Item     // list at +0xC8 (0x3F-byte nodes)
	Affects []*Affect   // list at +0x7F (9-byte nodes)
	Combat  *CombatInfo // block at +0x108 while fighting
	Slots   [13]*Item   // readied items, +0xCC (set by ApplyItems)
}

// charRecord is the 0x11D-byte DOS character record, field for field, so
// the .SAV files and the MON*CHA blocks load with encoding/binary. Bytes
// stay bytes: the rules rely on their wrap-around and signed readings.
type charRecord struct {
	PName      [16]byte  // 0x00 Pascal string, max 15
	Stats      [6]byte   // 0x10 str int wis dex con cha
	Str00      byte      // 0x16 exceptional strength
	Memorized  [21]byte  // 0x17 spell ids, +0x80 = still to be memorised
	Study      byte      // 0x2C hours of study the pending spells need (0/4/6)
	HitDice    byte      // 0x2D THAC0 base, from the DS:3C7C table
	Race       byte      // 0x2E
	Class      byte      // 0x2F
	Age        uint16    // 0x30
	MaxHP      byte      // 0x32
	SpellBook  [56]byte  // 0x33 [n-1]: spell n known
	FightLevel byte      // 0x6B fighter level (1 for non-fighters)
	Size       byte      // 0x6C low bits: icon size; 0x80 flag
	Saves      [5]byte   // 0x6D saving throw targets
	BaseMove   byte      // 0x72
	MaxLevel   byte      // 0x73 highest class level
	Drained    byte      // 0x74 levels lost to energy drain
	DrainedHP  byte      // 0x75 hit points lost with them
	Undead     byte      // 0x76 turn undead row
	Thief      [8]byte   // 0x77 thief skills
	_          [4]byte   // 0x7F affect list pointer
	Pad83      byte      // 0x83
	Control    byte      // 0x84 0 party, 0xB3 charmed, > 0x7F hired NPC
	Share      byte      // 0x85 hired NPC's treasure shares (& 7)
	Pad86      [2]byte   // 0x86
	Coins      [7]uint16 // 0x88 copper .. jewelry: only read when joining the party
	Levels     [8]byte   // 0x96 cleric druid fighter paladin ranger mage thief monk
	Sex        byte      // 0x9E
	Kind       byte      // 0x9F monster kind (bonuses against it)
	Alignment  byte      // 0xA0
	BaseAttack [2]byte   // 0xA1 attacks per hand
	BaseDice   [2]byte   // 0xA3 damage dice count per hand
	BaseSides  [2]byte   // 0xA5 damage dice sides per hand
	BaseBonus  [2]byte   // 0xA7 damage bonus per hand
	BaseAC     byte      // 0xA9
	StrBonus   byte      // 0xAA non-zero: strength modifies hit and damage
	RandomID   byte      // 0xAB
	Exp        uint32    // 0xAC
	ClassMask  byte      // 0xB0 items the classes may use
	HPRoll     byte      // 0xB1 rolled hit points
	SpellSlots [6]byte   // 0xB2 cleric levels 1..3, magic-user levels 1..3
	ExpValue   uint16    // 0xB8 a monster's worth
	ExpPerHP   byte      // 0xBA
	PortHead   byte      // 0xBB
	PortBody   byte      // 0xBC
	IconHead   byte      // 0xBD
	IconBody   byte      // 0xBE
	Order      byte      // 0xBF
	IconSize   byte      // 0xC0
	IconColour [6]byte   // 0xC1 icon parts, low nibble and high nibble
	ItemCount  byte      // 0xC7
	_          [4]byte   // 0xC8 item list pointer
	_          [52]byte  // 0xCC readied item pointers
	Hands      byte      // 0x100
	SaveBonus  byte      // 0x101
	Weight     uint16    // 0x102
	_          [8]byte   // 0x104 party link and combat block pointers
	Status     byte      // 0x10C index into DS:1047 names
	Okay       byte      // 0x10D non-zero = active
	Side       byte      // 0x10E 0 party, 1 monsters (1 also draws the name yellow)
	Auto       byte      // 0x10F the computer fights for it
	THAC0      byte      // 0x110
	AC         [2]byte   // 0x111 60 - armour class: front, rear
	Attacks    [2]byte   // 0x113 attacks per hand this round
	Dice       [2]byte   // 0x115 damage dice count per hand
	Sides      [2]byte   // 0x117 damage dice sides per hand
	Bonus      [2]byte   // 0x119 damage bonus per hand
	HP         byte      // 0x11B
	Move       byte      // 0x11C
}

const charRecSize = 0x11D

// Stats indexes.
const (
	statStr = iota
	statInt
	statWis
	statDex
	statCon
	statCha
)

// fighterKind: fighter, paladin or ranger levels; CotAB gives all three
// exceptional strength and the fighter's constitution hit points.
func fighterKind(c *Character) bool {
	return int8(c.Levels[clsFighter]) > 0 || int8(c.Levels[clsPaladin]) > 0 || int8(c.Levels[clsRanger]) > 0
}

// Class values (the order of the DS:0D38 names); the first eight are also
// the Levels indexes.
const (
	clsCleric = iota
	clsDruid
	clsFighter
	clsPaladin
	clsRanger
	clsMage
	clsThief
	clsMonk
	clsClericFighter
	clsClericFighterMage
	clsClericRanger
	clsClericMage
	clsClericThief
	clsFighterMage
	clsFighterThief
	clsFighterMageThief
	clsMageThief
)

// Race values (the order of the DS:0F03 names).
const (
	raceMonster = iota
	raceDwarf
	raceElf
	raceGnome
	raceHalfElf
	raceHalfling
	raceHalfOrc
	raceHuman
)

// Status values (the order of the DS:1047 names).
const (
	statusOkay = iota
	statusAnimated
	statusTempGone
	statusRunning
	statusUnconscious
	statusDying
	statusDead
	statusStoned
	statusGone
)

// Side values.
const (
	sideParty    = 0
	sideMonsters = 1
)

// setRecord reads a record in the DOS layout (short data leaves the rest zero).
func (c *Character) setRecord(b []byte) {
	buf := make([]byte, charRecSize)
	copy(buf, b)
	binary.Read(bytes.NewReader(buf), binary.LittleEndian, &c.charRecord)
}

// record is the record in the DOS layout (pointers zero).
func (c *Character) record() []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, &c.charRecord)
	return buf.Bytes()
}

// spellSlot is the number of spells of a level (1..3) a class (0 cleric,
// 1 magic-user) can memorise; 0 for anything else.
func (c *Character) spellSlot(class, level int) byte {
	if class < 0 || class > 1 || level < 1 || level > 3 {
		return 0
	}
	return c.SpellSlots[3*class+level-1]
}

func (c *Character) Name() string {
	n := min(int(c.PName[0]), 15)
	return string(c.PName[1 : 1+n])
}

// SetName stores a pascal string (max 15).
func (c *Character) SetName(s string) {
	if len(s) > 15 {
		s = s[:15]
	}
	c.PName = [16]byte{byte(len(s))}
	copy(c.PName[1:], s)
}

// displayAC = 00BA:08EF (char, x, y): |ac60-60|, with "-" when ac60 > 60, colour 0xA.
func (e *Engine) displayAC(c *Character, x, y int) {
	v := int(c.AC[0])
	s := strconv.Itoa(abs(v - 60))
	if v > 60 {
		s = "-" + s
	}
	e.DisplayString(x, y, colLightGreen, s)
}

// displayHP = 00BA:0973 (char, x, y, flag): yellow when below max, white
// otherwise, magenta when flag set.
func (e *Engine) displayHP(c *Character, x, y int, flag bool) {
	col := colLightGreen
	if c.HP < c.MaxHP {
		col = colYellow
	}
	if flag {
		col = colLightMagenta
	}
	e.DisplayString(x, y, col, strconv.Itoa(int(c.HP)))
}

// displayName = 00BA:1816 (char, x, y, possessive): red when not okay,
// yellow for NPCs, cyan otherwise; optional "'s" suffix.
func (e *Engine) displayName(c *Character, x, y int, possessive bool) {
	col := colLightRed
	if c.Okay != 0 {
		col = colLightCyan
		if c.Side == sideMonsters {
			col = colYellow
		}
	}
	e.DisplayString(x, y, col, c.Name())
	if possessive {
		e.DisplayString(x+len(c.Name()), y, col, "'s")
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
