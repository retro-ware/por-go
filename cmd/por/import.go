package main

import (
	"archive/zip"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/retro-ware/gameimport"
	"por/adf"
	"por/amiga"
	"por/dax"
	"por/ega"
)

// importGame (-import, for goplay) finds a Pool of Radiance install
// anywhere under src (the folder holding START.EXE and TITLE.DAX, but not
// SKY.DAX or BIGPIC1.DAX: those are Curse of the Azure Bonds'), copies
// the files the engine reads into dst, its SAVE and FONTS folders, and
// writes dst/COVER.PNG from the title picture for the floppy label.
func importGame(src, dst string) error {
	files, err := gameimport.Find(src, func(name string) bool {
		base, dir := path.Base(name), strings.ToUpper(path.Base(path.Dir(name)))
		ext := strings.ToUpper(path.Ext(base))
		return ext == ".DAX" || strings.EqualFold(base, "START.EXE") || strings.EqualFold(base, "POOL.CFG") ||
			strings.EqualFold(base, "ITEMS") || dir == "SAVE" || dir == "FONTS" && ext == ".F8"
	})
	if err != nil {
		return err
	}
	// The game's folder: TITLE.DAX and 8X8D1.DAX, not SKY.DAX or BIGPIC1.DAX
	// (Curse of the Azure Bonds'). START.EXE is not needed: the engine never reads it.
	// ponytail: a loose SAVE or FONTS folder is found by its name beside the game files.
	has := func(dir, n string) bool {
		for k := range files {
			if path.Dir(k) == dir && strings.EqualFold(path.Base(k), n) {
				return true
			}
		}
		return false
	}
	var dir string
	found := false
	for k := range files {
		d := path.Dir(k)
		if strings.EqualFold(path.Base(k), "TITLE.DAX") && has(d, "8X8D1.DAX") && !has(d, "SKY.DAX") && !has(d, "BIGPIC1.DAX") {
			dir, found = d, true
			break
		}
	}
	if !found {
		return errors.New("no Pool of Radiance found")
	}
	for k, b := range files {
		out, base, d := dst, path.Base(k), path.Dir(k)
		switch {
		case d == dir:
		case strings.EqualFold(path.Base(d), "SAVE") && path.Dir(d) == dir:
			out = filepath.Join(dst, "SAVE")
		case strings.EqualFold(path.Base(d), "FONTS") && path.Dir(d) == dir:
			out = filepath.Join(dst, "FONTS")
		default:
			continue
		}
		if err := os.MkdirAll(out, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, strings.ToUpper(base)), b, 0o644); err != nil {
			return err
		}
	}
	return writeCover(dst)
}

// writeCover renders TITLE.DAX block 2, the title picture.
func writeCover(dst string) error {
	l, err := dax.NewLoader(dst)
	if err != nil {
		return err
	}
	b := l.Block("TITLE", 2)
	if b == nil {
		return errors.New("TITLE.DAX has no title picture")
	}
	im := dax.ParseImage(b, false, 0)
	pal := make(color.Palette, 16)
	for i, c := range ega.Palette {
		pal[i] = color.RGBA{c[0], c[1], c[2], 255}
	}
	img := image.NewPaletted(image.Rect(0, 0, ega.W, ega.H), pal)
	d, w := im.Frame(0), im.Width*8
	for y := 0; y < im.Height && y < ega.H; y++ {
		for x := 0; x < w && x < ega.W; x++ {
			img.SetColorIndex(x, y, d[y*w+x]&15)
		}
	}
	f, err := os.Create(filepath.Join(dst, "COVER.PNG"))
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// amigaGfx: the Amiga Pool of Radiance graphics archives, with the palette
// (0x54080 for PIC/FINAL, which the Amiga sets before them; 0x54020 for
// the ground tiles and the combat icons, which the combat screen (0x39F0E)
// and the icon editor (0x2EB02) set; 0x54040 for the portraits) each is
// drawn in and whether its frames pack two tiles side by side.
// ponytail: 8X8D (the 3D-view symbols) stays DOS; the Amiga set is laid out
// for its own wall renderer and does not match WALLDEF's symbol numbers.
var amigaGfx = map[string]struct {
	slot   byte // ega.Amiga palette
	split  bool
	sprite bool // animation block (PIC/FINAL frames XORed against the first)
}{
	"icon": {ega.AmigaTerrain, false, false}, "comspr": {ega.AmigaTerrain, false, false}, "cpic": {ega.AmigaTerrain, false, false},
	"bacpac": {ega.AmigaTerrain, true, false}, "sqrpaci": {ega.AmigaTerrain, true, false}, "wildcom": {ega.AmigaTerrain, true, false},
	"dungcom": {ega.AmigaTerrain, true, false}, "randcom": {ega.AmigaTerrain, true, false},
	"head": {ega.AmigaPortrait, false, false}, "body": {ega.AmigaPortrait, false, false}, "chead": {ega.AmigaTerrain, false, false}, "cbody": {ega.AmigaTerrain, false, false},
	"pic": {ega.AmigaView, false, true}, "final": {ega.AmigaView, false, true}, "sprit": {ega.AmigaPortrait, false, true},
}

// importDisk2 (-import-disk2, goplay drive B) finds the Amiga Pool of
// Radiance data disk among the .adf images under src (loose or zipped) and
// writes its graphics, converted to DOS-layout archives, to dst/DISK2.
func importDisk2(src, dst string) error {
	var disk []adf.File
	var snd []byte // disk 1's program: the Amiga sound players and samples
	look := func(img []byte) bool {
		_, files, err := adf.Read(img)
		if err != nil {
			return false
		}
		n := 0
		for _, f := range files {
			if strings.EqualFold(f.Path, "program") && snd == nil {
				snd = amiga.Image(f.Data)
			}
			if _, ok := amigaGfx[strings.TrimSuffix(strings.ToLower(f.Path), ".dax")]; ok {
				n++
			}
		}
		if n >= 8 { // the POOLDATA disk; the program disk has none
			disk = files
		}
		return disk != nil && snd != nil
	}
	filepath.WalkDir(src, func(p string, d fs.DirEntry, _ error) error {
		if d == nil || d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".adf":
			if b, err := os.ReadFile(p); err == nil && look(b) {
				return fs.SkipAll
			}
		case ".zip":
			z, err := zip.OpenReader(p)
			if err != nil {
				return nil
			}
			defer z.Close()
			for _, f := range z.File {
				if !strings.EqualFold(filepath.Ext(f.Name), ".adf") || strings.Contains(f.Name, "__MACOSX") {
					continue
				}
				r, err := f.Open()
				if err != nil {
					continue
				}
				b, _ := io.ReadAll(r)
				r.Close()
				if look(b) {
					return fs.SkipAll
				}
			}
		}
		return nil
	})
	if disk == nil && snd == nil {
		return errors.New("no Amiga Pool of Radiance disk found")
	}
	out := filepath.Join(dst, "DISK2")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if snd != nil {
		if err := os.WriteFile(filepath.Join(out, "SOUND.BIN"), snd, 0o644); err != nil {
			return err
		}
	}
	for _, f := range disk {
		name := strings.TrimSuffix(strings.ToLower(f.Path), ".dax")
		g, ok := amigaGfx[name]
		if !ok {
			continue
		}
		a := dax.OpenAmiga(f.Data)
		blocks := map[int][]byte{}
		for _, e := range a.Entries {
			b := a.Block(e.ID)
			if len(b) < 16 || e.ID > 255 {
				continue
			}
			if g.sprite {
				blocks[e.ID] = dax.AmigaSprite(b, name != "sprit", g.slot)
			} else {
				blocks[e.ID] = dax.AmigaImage(b, g.split, g.slot)
			}
		}
		if err := os.WriteFile(filepath.Join(out, strings.ToUpper(name)+".DAX"), dax.WriteDOS(blocks), 0o644); err != nil {
			return err
		}
	}
	return nil
}
