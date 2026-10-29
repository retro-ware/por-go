// Package dax reads SSI Gold Box .DAX archives (DOS Pool of Radiance layout).
//
// File layout: u16 index size, then index entries of 9 bytes
// (u8 id, i32 offset from data start, u16 raw size, u16 stored size),
// then block data. Blocks are run-length coded:
//
//	n = int8(next)
//	n >= 0: copy the following n+1 bytes
//	n <  0: repeat the following byte -n times
package dax

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Entry is one index record.
type Entry struct {
	ID      int
	Offset  int32
	RawSize uint16
	Stored  uint16
}

// File is a loaded archive. Blocks are decoded on first access and cached.
type File struct {
	Entries []Entry
	data    []byte
	cache   map[int][]byte
	mu      sync.Mutex
}

// Open reads and indexes a DAX file.
func Open(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	n := int(binary.LittleEndian.Uint16(b))
	f := &File{data: b[2+n:], cache: map[int][]byte{}}
	for i := 2; i+9 <= 2+n; i += 9 {
		f.Entries = append(f.Entries, Entry{
			ID:      int(b[i]),
			Offset:  int32(binary.LittleEndian.Uint32(b[i+1:])),
			RawSize: binary.LittleEndian.Uint16(b[i+5:]),
			Stored:  binary.LittleEndian.Uint16(b[i+7:]),
		})
	}
	return f, nil
}

// Block returns the decoded block with the given id, or nil. The slice is
// the cached one: callers must not modify it.
func (f *File) Block(id int) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	if b, ok := f.cache[id]; ok {
		return b
	}
	for _, e := range f.Entries {
		if e.ID != id {
			continue
		}
		src := f.data[e.Offset : int(e.Offset)+int(e.Stored)]
		var out []byte
		if e.RawSize == 0 {
			out = append([]byte(nil), src...)
		} else {
			out = Decode(src, int(e.RawSize))
		}
		f.cache[id] = out
		return out
	}
	return nil
}

// Decode expands the RLE stream. Output is capped at rawSize; the original
// decoded into a fixed buffer, so over-long blocks are truncated, not errors.
func Decode(src []byte, rawSize int) []byte {
	out := make([]byte, 0, rawSize)
	for i := 0; i < len(src) && len(out) < rawSize; {
		n := int(int8(src[i]))
		if n >= 0 {
			end := i + 1 + n + 1
			if end > len(src) {
				end = len(src)
			}
			out = append(out, src[i+1:end]...)
			i = end
		} else {
			if i+1 < len(src) {
				for k := 0; k < -n; k++ {
					out = append(out, src[i+1])
				}
			}
			i += 2
		}
	}
	if len(out) > rawSize {
		out = out[:rawSize]
	}
	return out
}

// Loader resolves archive names case-insensitively inside a data directory
// and keeps opened files cached, like the original's single-file cache but
// without the reload cost.
type Loader struct {
	Dir   string
	mu    sync.Mutex
	files map[string]*File
	names map[string]string   // lower-case name -> real file name
	amiga *Loader             // UseAmiga's: Amiga graphics by base name (PIC.DAX for PIC1..8)
	memo  map[amigaKey][]byte // finished Amiga blocks (name munging and centre done once)
}

type amigaKey struct {
	name string
	id   int
}

// NewLoader scans dir for files.
func NewLoader(dir string) (*Loader, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	l := &Loader{Dir: dir, files: map[string]*File{}, names: map[string]string{}}
	for _, e := range ents {
		l.names[strings.ToLower(e.Name())] = e.Name()
	}
	return l, nil
}

// UseAmiga makes the Amiga archives in dir (cmd/por -import-disk2) stand in
// for the DOS ones of the same base name; a missing dir changes nothing.
func (l *Loader) UseAmiga(dir string) {
	l.amiga, _ = NewLoader(dir)
}

// Path returns the real path for a case-insensitive file name, or "".
func (l *Loader) Path(name string) string {
	if real, ok := l.names[strings.ToLower(name)]; ok {
		return filepath.Join(l.Dir, real)
	}
	return ""
}

// Exists reports whether a data file is present.
func (l *Loader) Exists(name string) bool { return l.Path(name) != "" }

// File opens (or returns cached) archive "name" (e.g. "TITLE.DAX").
func (l *Loader) File(name string) *File {
	key := strings.ToLower(name)
	if !strings.HasSuffix(key, ".dax") {
		key += ".dax"
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if f, ok := l.files[key]; ok {
		return f
	}
	p := l.Path(key)
	if p == "" {
		l.files[key] = nil
		return nil
	}
	f, err := Open(p)
	if err != nil {
		f = nil
	}
	l.files[key] = f
	return f
}

// Block loads block id from archive name; nil if either is missing.
func (l *Loader) Block(name string, id int) []byte {
	if l.amiga != nil {
		k := amigaKey{name, id}
		l.mu.Lock()
		b, ok := l.memo[k]
		l.mu.Unlock()
		if ok {
			return b
		}
		base := strings.TrimRight(strings.TrimSuffix(strings.ToUpper(name), ".DAX"), "0123456789")
		b = l.amiga.Block(base, id)
		if b == nil && base == "CHEAD" && id >= 0x80 {
			b = l.amiga.Block(base, id-0x80) // one Amiga head for both poses, not the DOS action head
		}
		if b != nil {
			if base == "HEAD" { // here, not at import, so older imports get it too
				b = centre(b, 11)
			}
			l.mu.Lock()
			if l.memo == nil {
				l.memo = map[amigaKey][]byte{}
			}
			l.memo[k] = b
			l.mu.Unlock()
			return b
		}
	}
	if f := l.File(name); f != nil {
		if b := f.Block(id); b != nil {
			return b
		}
	}
	for _, n := range siblings(name) { // all data is always there: the block on another disk
		if f := l.File(n); f != nil {
			if b := f.Block(id); b != nil {
				return b
			}
		}
	}
	return nil
}

// siblings are the same archive on the other disks: name with its disk
// digit (the last digit, "GEO3", "MON3CHA", "8X8D3") as 1..8, in order.
// The game split one set of blocks over its disks; only WALLDEF 23 and
// CPIC 24 and 152 differ between disks, and for those the disk asked for
// has its own.
func siblings(name string) []string {
	i := strings.LastIndexAny(name, "12345678")
	if i < 0 {
		return nil
	}
	var out []string
	for d := byte('1'); d <= '8'; d++ {
		if d != name[i] {
			out = append(out, name[:i]+string(d)+name[i+1:])
		}
	}
	return out
}

// centre widens a one-frame DOS-layout picture to w cells, centred. The
// Amiga heads are 7 cells wide; the engine places them where the 11-cell
// DOS heads go, above the 11-cell bodies.
func centre(b []byte, w int) []byte {
	h, old := int(b[0])|int(b[1])<<8, int(b[2])|int(b[3])<<8
	if old >= w || b[8] != 1 {
		return b
	}
	pad := (w - old) / 2 * 4 // bytes of blank pixels on the left
	out := append(append([]byte(nil), b[:17]...), make([]byte, h*w*4)...)
	out[2], out[3] = byte(w), byte(w>>8)
	for y := 0; y < h; y++ {
		copy(out[17+y*w*4+pad:], b[17+y*old*4:17+(y+1)*old*4])
	}
	return out
}
