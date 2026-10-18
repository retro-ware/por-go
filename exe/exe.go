// Package exe unpacks the EXEPACK-compressed START.EXE so the engine can read
// the program's data tables (code wheel, race/class names, item words ...)
// straight from the original executable at run time.
package exe

import (
	"encoding/binary"
	"errors"
	"os"
)

// Image is the unpacked load image plus the data segment offset.
type Image struct {
	Mem []byte
	DS  int // byte offset of the Turbo Pascal data segment inside Mem
}

// Load reads and unpacks path.
func Load(path string) (*Image, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) < 0x20 || string(b[:2]) != "MZ" {
		return nil, errors.New("not an MZ executable")
	}
	hdr := int(binary.LittleEndian.Uint16(b[8:])) * 16
	cs := int(int16(binary.LittleEndian.Uint16(b[0x16:])))
	ip := int(binary.LittleEndian.Uint16(b[0x14:]))
	exepackOfs := hdr + cs*16 // start of the EXEPACK variables block
	if exepackOfs+18 > len(b) || string(b[exepackOfs+16:exepackOfs+18]) != "RB" {
		return nil, errors.New("not EXEPACK packed")
	}
	v := b[exepackOfs:]
	destLen := int(binary.LittleEndian.Uint16(v[12:])) * 16 // dest_len paragraphs
	_ = ip
	// packed data lies between the load module start and the EXEPACK block,
	// and is processed backwards (commands follow their data).
	packed := b[hdr:exepackOfs]
	// skip trailing 0xFF padding
	end := len(packed)
	for end > 0 && packed[end-1] == 0xFF {
		end--
	}
	out := make([]byte, destLen)
	dst := destLen
	i := end
	for i > 0 {
		cmd := packed[i-1]
		n := int(binary.LittleEndian.Uint16(packed[i-3:]))
		i -= 3
		switch cmd & 0xFE {
		case 0xB0: // fill
			fill := packed[i-1]
			i--
			for k := 0; k < n; k++ {
				dst--
				out[dst] = fill
			}
		case 0xB2: // copy
			for k := 0; k < n; k++ {
				dst--
				i--
				out[dst] = packed[i]
			}
		default:
			return nil, errors.New("bad EXEPACK command")
		}
		if cmd&1 == 1 {
			break
		}
	}
	// unconsumed leading bytes are stored as-is
	copy(out[dst-i:dst], packed[:i])
	return &Image{Mem: out, DS: 0xC7C0}, nil // DS = 0x0C7C, see decomp notes
}
