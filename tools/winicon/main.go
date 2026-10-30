// winicon writes a Windows resource object (.syso) holding one icon, so the
// Go linker embeds it into the exe as its icon:
//
//	go run ./tools/winicon -ico cmd/por/icon.ico -o cmd/por/rsrc_windows_amd64.syso
//
// It is a COFF object with a single .rsrc section: a resource directory
// (RT_ICON per image, one RT_GROUP_ICON listing them) with a relocation for
// every data entry's address, which the linker resolves against the section's
// final place. No manifest, no version info.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
)

const (
	rtIcon      = 3
	rtGroupIcon = 14
	lang        = 0x0409 // US English, as resource compilers default to
)

func main() {
	ico := flag.String("ico", "icon.ico", "the icon file")
	out := flag.String("o", "rsrc_windows_amd64.syso", "the object to write")
	flag.Parse()
	b, err := os.ReadFile(*ico)
	if err != nil {
		log.Fatal(err)
	}
	images, group, err := splitICO(b)
	if err != nil {
		log.Fatalf("%s: %v", *ico, err)
	}
	if err := os.WriteFile(*out, coff(images, group), 0o644); err != nil {
		log.Fatal(err)
	}
}

// splitICO takes an .ico apart: each image's bytes (RT_ICON 1..n) and the
// GRPICONDIR that lists them by that ID instead of by file offset.
func splitICO(b []byte) (images [][]byte, group []byte, err error) {
	if len(b) < 6 || binary.LittleEndian.Uint16(b[2:]) != 1 {
		return nil, nil, fmt.Errorf("not an icon file")
	}
	n := int(binary.LittleEndian.Uint16(b[4:]))
	var g bytes.Buffer
	g.Write(b[:6])
	for i := range n {
		e := b[6+16*i:]
		if len(e) < 16 {
			return nil, nil, fmt.Errorf("short directory")
		}
		size, off := binary.LittleEndian.Uint32(e[8:]), binary.LittleEndian.Uint32(e[12:])
		if uint64(off)+uint64(size) > uint64(len(b)) {
			return nil, nil, fmt.Errorf("image %d beyond the file", i+1)
		}
		images = append(images, b[off:off+size])
		g.Write(e[:12])
		binary.Write(&g, binary.LittleEndian, uint16(i+1))
	}
	return images, g.Bytes(), nil
}

// coff lays out the object: file header, one section header, the .rsrc
// section, its relocations, one symbol for the section, an empty string table.
func coff(images [][]byte, group []byte) []byte {
	// The resource tree: root → types → IDs → language → data entry.
	type res struct {
		typ, id uint16
		data    []byte
	}
	var all []res
	for i, im := range images {
		all = append(all, res{rtIcon, uint16(i + 1), im})
	}
	all = append(all, res{rtGroupIcon, 1, group})
	byType := [][]res{nil, nil} // RT_ICON entries, then the RT_GROUP_ICON entry (ascending type IDs)
	for _, r := range all {
		if r.typ == rtIcon {
			byType[0] = append(byType[0], r)
		} else {
			byType[1] = append(byType[1], r)
		}
	}
	const dirSize, entrySize, dataEntrySize = 16, 8, 16
	// Offsets within the section: the root directory (types), one directory
	// per type (IDs), one per resource (its language), the data entries, the
	// data.
	typeDirs := uint32(dirSize + entrySize*len(byType))
	nameDirs := typeDirs
	for _, rs := range byType {
		nameDirs += dirSize + entrySize*uint32(len(rs))
	}
	dataEntries := nameDirs + uint32(len(all))*(dirSize+entrySize)
	data := (dataEntries + uint32(len(all))*dataEntrySize + 7) &^ 7

	var sec bytes.Buffer
	put := func(vs ...uint32) {
		for _, v := range vs {
			binary.Write(&sec, binary.LittleEndian, v)
		}
	}
	dir := func(n int) { put(0, 0, 0, uint32(n)<<16) } // Characteristics, TimeDateStamp, Major+Minor version, named entries 0 + ID entries n
	dir(len(byType))
	off := typeDirs
	for t, rs := range byType {
		put(uint32([]uint16{rtIcon, rtGroupIcon}[t]), 0x80000000|off)
		off += dirSize + entrySize*uint32(len(rs))
	}
	k := 0
	for _, rs := range byType {
		dir(len(rs))
		for _, r := range rs {
			put(uint32(r.id), 0x80000000|(nameDirs+uint32(k)*(dirSize+entrySize)))
			k++
		}
	}
	for i := range all {
		dir(1)
		put(lang, dataEntries+uint32(i)*dataEntrySize)
	}
	var relocs []uint32 // section offsets of the data entries' address fields
	pos := data
	for i, r := range all {
		relocs = append(relocs, dataEntries+uint32(i)*dataEntrySize)
		put(pos, uint32(len(r.data)), 0, 0) // OffsetToData (relocated), Size, CodePage, Reserved
		pos += (uint32(len(r.data)) + 7) &^ 7
	}
	for sec.Len() < int(data) {
		sec.WriteByte(0)
	}
	for _, r := range all {
		sec.Write(r.data)
		for sec.Len()%8 != 0 {
			sec.WriteByte(0)
		}
	}

	const fileHeader, sectionHeader, relocSize, symbolSize = 20, 40, 10, 18
	var o bytes.Buffer
	w := func(vs ...any) {
		for _, v := range vs {
			binary.Write(&o, binary.LittleEndian, v)
		}
	}
	rawData := uint32(fileHeader + sectionHeader)
	relocPos := rawData + uint32(sec.Len())
	symPos := relocPos + relocSize*uint32(len(relocs))
	// IMAGE_FILE_HEADER
	w(uint16(0x8664), uint16(1), uint32(0), symPos, uint32(1), uint16(0), uint16(0x0004)) // amd64, one section, no timestamp, symbols, no optional header, line numbers stripped
	// IMAGE_SECTION_HEADER
	w([8]byte{'.', 'r', 's', 'r', 'c'}, uint32(0), uint32(0), uint32(sec.Len()), rawData, relocPos, uint32(0), uint16(len(relocs)), uint16(0), uint32(0x40000040)) // initialized data, readable
	o.Write(sec.Bytes())
	for _, r := range relocs {
		w(r, uint32(0), uint16(3)) // IMAGE_REL_AMD64_ADDR32NB against symbol 0, the section
	}
	// The symbol table: the section's own symbol, then an empty string table.
	w([8]byte{'.', 'r', 's', 'r', 'c'}, uint32(0), uint16(1), uint16(0), uint8(3), uint8(0)) // static, section 1
	w(uint32(4))
	return o.Bytes()
}
