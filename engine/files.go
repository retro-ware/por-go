package engine

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Character file I/O, ported from ovr18. Record layouts are kept raw so the
// original SAVE files load and save byte for byte.

const (
	itemRecSize   = 0x3F
	affectRecSize = 9
)

// Item and Affect are the raw list nodes (next pointers live in the slices).
// Item is the 0x3F-byte DOS item record, in its file layout (encoding/binary).
type Item struct {
	PName   [0x29]byte // 0x00 shown name, Pascal string (built by ItemName)
	Pad29   byte       // 0x29
	_       [4]byte    // 0x2A next pointer
	Type    byte       // 0x2E base item type (index into the DS:5582 records)
	Words   [3]byte    // 0x2F name word ids into DS:10A7
	Plus    byte       // 0x32 signed
	Plus2   byte       // 0x33
	Readied byte       // 0x34
	Hidden  byte       // 0x35 bit 3-i: word i not known yet
	Magic   byte       // 0x36
	Weight  uint16     // 0x37
	Count   byte       // 0x39
	Price   uint16     // 0x3A
	Spells  [3]byte    // 0x3C a scroll's spells, +0x80 = being scribed
}

// Affect is a spell effect or condition on a character.
type Affect struct {
	affectRecord
	Item *Item // set when an item stands in for the affect (ovr13 item handlers)
}

// affectRecord is the 9-byte DOS affect node in its file layout.
type affectRecord struct {
	Type byte   // affect number, index into the ovr13 handler table
	Time uint16 // remaining duration, 0 = permanent
	Data byte
	Call byte    // non-zero: the handler runs when the affect is removed
	_    [4]byte // next pointer
}

// readRecords reads the n-byte records in b (a partial last one is dropped).
func readRecords[T any](b []byte, n int) []*T {
	var out []*T
	for ; len(b) >= n; b = b[n:] {
		v := new(T)
		binary.Read(bytes.NewReader(b[:n]), binary.LittleEndian, v)
		out = append(out, v)
	}
	return out
}

func readAffects(b []byte) []*Affect {
	var out []*Affect
	for _, r := range readRecords[affectRecord](b, affectRecSize) {
		out = append(out, &Affect{affectRecord: *r})
	}
	return out
}

// writeRecords is the inverse of readRecords.
func writeRecords[T any](list []*T) []byte {
	var buf bytes.Buffer
	for _, v := range list {
		binary.Write(&buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

// savePath resolves a file name inside the save directory case-insensitively
// (DOS was case-insensitive; the shipped files are upper case).
func (e *Engine) savePath(name string) string {
	dir := e.Cfg.SaveDir
	if ents, err := os.ReadDir(dir); err == nil {
		for _, en := range ents {
			if strings.EqualFold(en.Name(), name) {
				return filepath.Join(dir, en.Name())
			}
		}
	}
	return filepath.Join(dir, strings.ToUpper(name))
}

func (e *Engine) fileExists(path string) bool { // 0802:02EB
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// removeSubstr = 0802:0356; makeFileName = 0802:0425.
// The scan runs over the original length and does not recheck the position
// after a deletion, so adjacent copies leave every second one (original quirk).
func removeSubstr(s, sub string) string {
	if sub == "" {
		return s
	}
	for i, n := 1, len(s)-len(sub)+1; i <= n; i++ {
		if i-1+len(sub) <= len(s) && s[i-1:i-1+len(sub)] == sub {
			s = s[:i-1] + s[i-1+len(sub):]
		}
	}
	return s
}
func makeFileName(s string) string {
	for _, c := range " .*,?/\\:;|" { // DS:0D1A..0D23
		s = removeSubstr(s, string(c))
	}
	if len(s) > 8 {
		s = s[:8]
	}
	return strings.ToUpper(s)
}

// charBase = savePath + MakeFileName(RemoveSubstr(name,'.')) for roster files.
func (e *Engine) charBase(c *Character) string {
	return makeFileName(removeSubstr(c.Name(), "."))
}

// fileError reports a save file that could not be read or written, once.
// (DOS asked for the save disk here and retried until it worked.)
func (e *Engine) fileError(verb, path string) {
	s := "Cannot " + verb + " " + filepath.Base(path)
	if len(s) > 40 {
		s = s[:40]
	}
	e.MessageRow24b(0xE, s)
}

// MessageRow24b = 0709:0BF5: clear row 24, show, wait for a key.
func (e *Engine) MessageRow24b(colour int, s string) byte {
	if e.Trace != nil {
		e.Trace("0709:0BF5", "MessageRow24b", colour, s)
	}
	e.ClearRow24()
	e.DisplayString(0, 24, colour, s)
	return e.ReadKeyByte()
}

// BuildCharList = ovr18:0010: names from charlist.txt, "* " for party members.
func (e *Engine) BuildCharList() []string {
	p := e.savePath("charlist.txt")
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer f.Close()
	var list []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name := strings.TrimRight(sc.Text(), "\r")
		if len(name) > 40 {
			name = name[:40]
		}
		for _, c := range e.party {
			if c.Name() == name {
				name = "* " + name
				if len(name) > 40 {
					name = name[:40]
				}
				break
			}
		}
		list = append(list, name)
	}
	return list
}

// WriteCharList = ovr18:01B8: rewrite charlist.txt without "*" entries.
func (e *Engine) WriteCharList(list []string) {
	os.MkdirAll(e.Cfg.SaveDir, 0o755)
	f, err := os.Create(e.savePath("charlist.txt"))
	if err != nil {
		e.fileError("write", "CHARLIST.TXT")
		return
	}
	defer f.Close()
	for _, n := range list {
		if len(n) > 0 && n[0] == '*' {
			continue
		}
		f.WriteString(n + "\r\n")
	}
}

// SaveCharacter = ovr18:069E (c, name): name "" = roster save (.cha, listed
// in charlist.txt), else <name>.sav as used by SaveGame.
func (e *Engine) SaveCharacter(c *Character, name string) {
	var base, ext string
	if name == "" {
		list := append(e.BuildCharList(), c.Name())
		e.WriteCharList(list)
		base, ext = e.charBase(c), ".cha"
	} else {
		if len(name) > 8 {
			name = name[:8]
		}
		base, ext = name, ".sav"
	}
	os.MkdirAll(e.Cfg.SaveDir, 0o755)
	path := e.savePath(base + ext)
	if err := os.WriteFile(path, c.record(), 0o644); err != nil {
		e.fileError("write", path)
	}
	os.Remove(e.savePath(base + ".itm"))
	if len(c.Items) > 0 {
		os.WriteFile(e.savePath(base+".itm"), writeRecords(c.Items), 0o644)
	}
	os.Remove(e.savePath(base + ".spc"))
	if len(c.Affects) > 0 {
		var recs []*affectRecord
		for _, a := range c.Affects {
			recs = append(recs, &a.affectRecord)
		}
		os.WriteFile(e.savePath(base+".spc"), writeRecords(recs), 0o644)
	}
}

// LoadCharacter = ovr18:0AEC (name, c, useSav). Returns false when the file
// is missing.
func (e *Engine) LoadCharacter(name string, c *Character, useSav bool) bool {
	nm := makeFileName(name)
	ext := ".cha"
	if useSav {
		ext = ".sav"
	}
	path := e.savePath(nm + ext)
	b, err := os.ReadFile(path)
	if err != nil {
		e.fileError("read", path)
		return false
	}
	c.setRecord(b)
	c.Items, c.Affects = nil, nil
	if b, err := os.ReadFile(e.savePath(nm + ".itm")); err == nil {
		c.Items = readRecords[Item](b, itemRecSize)
	}
	if b, err := os.ReadFile(e.savePath(nm + ".spc")); err == nil {
		c.Affects = readAffects(b)
	}
	e.ClearKeyboard()
	return true
}

// LoadMonster = ovr18:0EC4 (index, c): MON<n>CHA/SPC/ITM.DAX block index.
func (e *Engine) LoadMonster(index int, c *Character) bool {
	b := e.L.Block(e.set.file("MON")+"CHA", index)
	if b == nil {
		e.MessageRow24b(0xF, "Unable to load monster")
		e.Exit()
		return false
	}
	c.setRecord(b)
	c.Items, c.Affects = nil, nil
	switch c.Race {
	case 1, 3, 5:
		c.IconSize = 1
	case 2, 4, 6, 7:
		c.IconSize = 2
	}
	for i := 1; i <= 3; i++ {
		r := byte(e.Dice(1, 6))
		c.IconColour[i-1] = (r+8)<<4 | r
	}
	c.IconColour[3] = 0xC4
	for i := 5; i <= 6; i++ {
		r := byte(e.Dice(1, 6))
		c.IconColour[i-1] = (r+8)<<4 | r
	}
	e.loadingMonster = true // DS:557C
	if b := e.L.Block(e.set.file("MON")+"SPC", index); b != nil {
		c.Affects = readAffects(b)
	}
	if b := e.L.Block(e.set.file("MON")+"ITM", index); b != nil {
		c.Items = readRecords[Item](b, itemRecSize)
	}
	e.loadingMonster = false
	e.ClearKeyboard()
	return true
}

// Portrait tables DS:8C7.. (gender-major).
var (
	portHeads     = [2][8]byte{{1, 2, 3, 4, 5, 8, 0xB, 0xC}, {6, 7, 9, 0xA, 0xD, 0xE, 6, 7}}
	portBodyCls0  = [2][5]byte{{2, 4, 2, 4, 2}, {2, 4, 8, 2, 4}}
	portBodyOther = [2][5]byte{{1, 3, 4, 5, 6}, {7, 9, 0xB, 7, 9}}
	portBodyCls5  = [2][5]byte{{2, 2, 2, 2, 2}, {8, 0xA, 0xC, 8, 0xA}}
	portBodyCls6  = [2][5]byte{{6, 6, 6, 6, 6}, {8, 0xA, 8, 0xA, 8}}
)

// AddMonsterToParty = ovr18:1262 (index): NPC joins the party.
func (e *Engine) AddMonsterToParty(index int) {
	if e.partyCount() > 7 {
		return
	}
	c := &Character{}
	e.LoadMonster(index, c)
	c.RandomID = byte(e.Random(0x100))
	var classes []int
	for cls := 0; cls < 7; cls++ {
		if int8(c.Levels[cls]) > 0 { // jle on a signed byte (ovr18:12CA)
			classes = append(classes, cls)
		}
	}
	if len(classes) == 0 {
		classes = []int{2}
	}
	cls := classes[e.Dice(1, len(classes))-1]
	r := e.Dice(1, 5) - 1
	g := int(c.Sex) & 1
	switch cls {
	case 0:
		c.PortBody = portBodyCls0[g][r]
	case 5:
		c.PortBody = portBodyCls5[g][r]
	case 6:
		c.PortBody = portBodyCls6[g][r]
	default:
		c.PortBody = portBodyOther[g][r]
	}
	c.PortHead = portHeads[g][e.Dice(1, 8)-1]
	e.AddToParty(c)
	e.BuildCharIcon(c, true)
}

func (e *Engine) partyCount() int { return int(e.game.PartyCount) }
func (e *Engine) setPartyCount(n int) {
	e.game.PartyCount = uint16(n)
}

// AddToParty = ovr18:1446 (c): append, give the lowest free order 0..7;
// the newcomer's coins go to the party's purse.
func (e *Engine) AddToParty(c *Character) {
	c.Order = 0xFF
	e.party = append(e.party, c)
	var used [8]bool
	for _, m := range e.party {
		if m.Order < 8 {
			used[m.Order] = true
		}
	}
	e.partySel = len(e.party) - 1
	o := 0
	for o < 8 && used[o] {
		o++
	}
	c.Order = byte(o)
	e.setPartyCount(e.partyCount() + 1)
	e.takeRecordCoins(c)
	e.RecomputeCharacter(c) // ovr24:0000
}

// eraseRosterCopies deletes <name>.cha/.itm/.spc (nested check as in LoadGame
// when nested is true, independent checks as in SaveGame otherwise).
func (e *Engine) eraseRosterCopies(c *Character, nested bool) {
	base := e.charBase(c)
	cha := e.savePath(base + ".cha")
	if e.fileExists(cha) {
		os.Remove(cha)
		os.Remove(e.savePath(base + ".itm"))
		os.Remove(e.savePath(base + ".spc"))
	} else if !nested {
		os.Remove(e.savePath(base + ".itm"))
		os.Remove(e.savePath(base + ".spc"))
	}
}

// LoadGame = ovr18:1602.
func (e *Engine) LoadGame() {
	menu := ""
	for L := byte('A'); L <= 'J'; L++ {
		if e.fileExists(e.savePath("savgam" + string(L) + ".dat")) {
			menu += string(L) + " "
		}
	}
	if menu == "" {
		return
	}
	var key byte
	for {
		key, _ = e.DisplayInput("Load Which Game: ", menu, 0xD, 0xA, 0xF, false, false)
		if key >= 'A' && key <= 'J' && e.fileExists(e.savePath("savgam"+string(key)+".dat")) {
			break
		}
		if key == 0 {
			return
		}
	}
	e.loadSlot(key)
	e.oldWalls()
}

// oldWalls is the original's overlap, kept for the lettered saves of traced
// runs: the last wall slot's word at area+0x3FE is also the view animation
// flag, so after a save or load the view picture cycles on every poll until
// a menu returns (an empty slot packs 0xFFFF).
func (e *Engine) oldWalls() { e.viewAnim = e.fileWallsB[2] }

// loadSlot is LoadGame after the prompt: savgam<key>.dat and its party.
func (e *Engine) loadSlot(key byte) bool {
	path := e.savePath("savgam" + string(key) + ".dat")
	b, err := os.ReadFile(path)
	if err != nil || len(b) < 0x3351 {
		e.fileError("read", path)
		return false
	}
	e.ClearRow24()
	e.DisplayString(0, 24, 0xA, "Loading...Please Wait")
	p := 0
	rd := func(n int) []byte { s := b[p : p+n]; p += n; return s }
	e.set = Set(rd(1)[0])
	setWords(e.area.words()[:], rd(0x800))
	wallsA, wallsB := e.area.WallsA, e.area.WallsB
	e.area.WallsA, e.area.WallsB = [3]uint16{}, [3]uint16{}
	e.fileWallsA, e.fileWallsB = wallsA, wallsB
	setWords(e.game.words()[:], rd(0x800))
	setWords(e.aux[:], rd(0x400))
	copy(e.ecl, rd(0x1E00))
	pos := rd(5)
	e.posX, e.posY, e.facing = int(pos[0]), int(pos[1]), int(pos[2])
	e.pos3, e.pos4 = pos[3], pos[4]
	e.viewMode = int(rd(1)[0])
	e.beginFlag = rd(1)[0]
	count := int(rd(1)[0])
	names := rd(0x148)
	w := e.area.ViewFlags
	e.viewBit1, e.viewBit0 = byte(w>>1), byte(w&1)
	e.msgDelay = byte(e.area.MsgDelay)
	e.setPartyCount(0)
	e.party, e.partySel = nil, 0
	e.purse = readPurse(b[p:])
	for i := 0; i < count && i < 8; i++ {
		ps := names[i*0x29:]
		n := int(ps[0])
		if n > 40 {
			n = 40
		}
		nm := makeFileName(removeSubstr(string(ps[1:1+n]), "."))
		if e.fileExists(e.savePath(nm + ".sav")) {
			c := &Character{}
			if e.LoadCharacter(nm, c, true) {
				e.AddToParty(c)
			}
		}
	}
	for _, c := range e.party {
		e.eraseRosterCopies(c, true)
	}
	for _, c := range e.party {
		e.BuildCharIcon(c, true)
	}
	e.partySel = 0
	e.set = Set(e.game.SymbolSet)
	if e.viewMode < 2 {
		e.LoadGeo(int(byte(e.area.Geo))) // ovr30:10C7
	}
	if e.viewMode > 1 {
		e.LoadIconPic("ICON", 0, 0xB) // ovr33:01C8
	} else {
		for i := 1; i <= 3; i++ {
			e.mapPairA[i-1] = int(int16(wallsA[i-1]))
			e.mapPairB[i-1] = int(int16(wallsB[i-1]))
			if e.mapPairA[i-1] >= 0 {
				e.LoadWalldef(e.mapPairA[i-1]&0xFF, e.mapPairB[i-1]&0xFF) // ovr30:0E08
			}
		}
	}
	if e.area.RiverOpen != 0 {
		e.EndgameHook(0) // ovr19:06B3
	}
	e.ClearKeyboard()
	e.ClearRow24()
	return true
}

// SaveGame = ovr18:1D58.
func (e *Engine) SaveGame() {
	var key byte
	for {
		key, _ = e.DisplayInput("Save Which Game: ", "A B C D E F G H I J", 0xD, 0xA, 0xF, false, e.beginFlag == 2) // ovr18:1D8B
		if key == 0 || key >= 'A' && key <= 'J' {
			break
		}
	}
	if key == 0 {
		return
	}
	e.saveSlot(key)
	e.oldWalls()
}

// saveSlot is SaveGame after the prompt.
func (e *Engine) saveSlot(key byte) {
	e.ClearRow24()
	e.DisplayString(0, 24, 0xA, "Saving...Please Wait")
	e.area.MsgDelay = uint16(e.msgDelay)
	e.area.ViewFlags = uint16(e.viewBit1)*2 + uint16(e.viewBit0)
	e.game.SymbolSet = uint16(e.set)
	b := []byte{byte(e.set)}
	area := e.area
	for i := range 3 {
		area.WallsA[i] = uint16(e.mapPairA[i])
		area.WallsB[i] = uint16(e.mapPairB[i])
	}
	e.fileWallsA, e.fileWallsB = area.WallsA, area.WallsB
	b = append(b, wordsBytes(area.words()[:])...)
	b = append(b, wordsBytes(e.game.words()[:])...)
	b = append(b, wordsBytes(e.aux[:])...)
	b = append(b, e.ecl...)
	b = append(b, byte(e.posX), byte(e.posY), byte(e.facing), e.pos3, e.pos4)
	b = append(b, byte(e.viewMode), e.beginFlag, byte(len(e.party)))
	names := make([]byte, 0x148)
	for i := range e.party {
		s := "CHRDAT" + string(key) + strconv.Itoa(i+1)
		names[i*0x29] = byte(len(s))
		copy(names[i*0x29+1:], s)
	}
	b = append(b, names...)
	b = appendPurse(b, e.purse)
	os.MkdirAll(e.Cfg.SaveDir, 0o755)
	path := e.savePath("savgam" + string(key) + ".dat")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		e.fileError("write", path)
	}
	for i, c := range e.party {
		e.SaveCharacter(c, "CHRDAT"+string(key)+strconv.Itoa(i+1))
		e.eraseRosterCopies(c, false)
	}
	e.saved = true
	e.ClearRow24()
}
