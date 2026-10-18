package engine

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// saveFile is the fixed part of SAVGAMx.DAT (ovr18 LoadGame/SaveGame),
// read and written with encoding/binary; the party's purse follows it.
type saveFile struct {
	Set                      byte
	Area                     areaVars
	Game                     gameVars
	Aux                      auxVars
	ECL                      [0x1E00]byte
	X, Y, Facing, Pos3, Pos4 byte
	ViewMode, BeginFlag      byte
	Count                    byte          // party members
	Names                    [8]pascalName // their CHRDAT<key><n> files
}

// pascalName is a Turbo Pascal string[40]: length byte, then the text.
type pascalName [0x29]byte

func (p *pascalName) String() string { return string(p[1 : 1+min(int(p[0]), 40)]) }

func (p *pascalName) Set(s string) {
	*p = pascalName{byte(min(len(s), 40))}
	copy(p[1:], s)
}

// SaveEditor edits a SAVGAMx.DAT image outside the game: cmd/fixture
// builds the trace scenarios' save fixtures with it.
type SaveEditor struct {
	f    saveFile
	tail []byte // the purse
}

// OpenSave parses a SAVGAMx.DAT image.
func OpenSave(b []byte) (*SaveEditor, error) {
	s := &SaveEditor{}
	n, err := binary.Decode(b, binary.LittleEndian, &s.f)
	if err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}
	s.tail = append([]byte(nil), b[n:]...)
	return s, nil
}

// Bytes is the edited image.
func (s *SaveEditor) Bytes() []byte {
	b, err := binary.Append(nil, binary.LittleEndian, &s.f)
	if err != nil {
		panic(err)
	}
	return append(b, s.tail...)
}

// SetVar sets an area (0x4900..) or game (0x6B00..) script word.
func (s *SaveEditor) SetVar(a int, v uint16) error {
	switch addrKind(a) {
	case spaceArea:
		*s.f.Area.word(a - areaVarBase) = v
	case spaceGame:
		*s.f.Game.word(a - gameVarBase) = v
	default:
		return errors.New("not an area or game variable")
	}
	return nil
}

// EnterArea moves the save into ECL block of disk (its script image ecl),
// loading 3D map geo and wall slots 1..3 from WALLDEF blocks walls (0xFF
// none), as a script's opNewECL would (ovr04:0BD1) and SaveGame records it.
func (s *SaveEditor) EnterArea(disk, block, geo int, ecl []byte, walls [3]int) {
	s.f.Set = byte(disk)
	s.f.Game.SymbolSet = uint16(disk)
	s.f.Area.AreaKind = uint16(block)
	s.f.Area.Geo = uint16(geo)
	s.f.ECL = [0x1E00]byte{}
	copy(s.f.ECL[:], ecl)
	for i, id := range walls {
		blk, slot := id, i+1
		if id == 0xFF {
			blk, slot = -1, -1
		}
		s.f.Area.WallsA[i], s.f.Area.WallsB[i] = uint16(blk), uint16(slot)
	}
	for a := eclAreaScratch; a < eclAreaScratch+0x20; a++ { // zeroed on entry (InitAreaScript)
		s.SetVar(a, 0)
	}
}

// SetPos places the party; a negative value keeps the saved one.
func (s *SaveEditor) SetPos(x, y, facing int) {
	for _, p := range []struct {
		v   int
		dst *byte
	}{{x, &s.f.X}, {y, &s.f.Y}, {facing, &s.f.Facing}} {
		if p.v >= 0 {
			*p.dst = byte(p.v)
		}
	}
}

// Party is the base names of the members' .SAV/.SPC/.ITM files.
func (s *SaveEditor) Party() []string {
	var out []string
	for i := range min(int(s.f.Count), len(s.f.Names)) {
		out = append(out, s.f.Names[i].String())
	}
	return out
}

// MakeStrong rewrites a .SAV character record so the character always hits
// and takes long to die (cmd/fixture -strong: a scripted fight is won).
func MakeStrong(rec []byte) []byte {
	var c Character
	c.setRecord(rec)
	c.HitDice = 40 // THAC0 is recomputed from it
	c.MaxHP, c.HP = 250, 250
	c.BaseAC, c.AC = 80, [2]byte{80, 80}  // AC -20 (60 - AC)
	c.BaseDice[0], c.BaseSides[0] = 6, 12 // unarmed damage 6d12
	return c.record()
}
