package main

import (
	"archive/zip"
	"bytes"
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

	"por"
	"por/adf"
	"por/amiga"
	"por/dax"
	"por/engine"
	"por/pal"

	"github.com/retro-ware/gameimport"
)

// importGame (-import, for goplay) finds a Pool of Radiance install
// anywhere under src (the folder holding START.EXE and TITLE.DAX, but not
// SKY.DAX or BIGPIC1.DAX: those are Curse of the Azure Bonds'), copies
// the files the engine reads into dst/dos, its SAVE folder into dst/save
// and its FONTS into dst/assets/fonts, and writes dst/COVER.PNG from the
// title picture for the floppy label.
func importGame(src, dst string) error {
	files, err := gameimport.Find(src, func(name string) bool {
		base, dir := path.Base(name), strings.ToUpper(path.Base(path.Dir(name)))
		ext := strings.ToUpper(path.Ext(base))
		return ext == ".DAX" || strings.EqualFold(base, "POOL.CFG") ||
			strings.EqualFold(base, "ITEMS") || dir == "SAVE" || dir == "FONTS" && ext == ".F8"
	})
	if err != nil {
		return err
	}
	// The game's folder: TITLE.DAX and 8X8D1.DAX, not SKY.DAX or BIGPIC1.DAX
	// (Curse of the Azure Bonds'). START.EXE is not copied: the engine never reads it.
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
		var out string
		base, d := path.Base(k), path.Dir(k)
		switch {
		case d == dir:
			out = filepath.Join(dst, engine.DOSDir)
		case strings.EqualFold(path.Base(d), "SAVE") && path.Dir(d) == dir:
			out = filepath.Join(dst, engine.SaveDir)
		case strings.EqualFold(path.Base(d), "FONTS") && path.Dir(d) == dir:
			out = filepath.Join(dst, engine.AssetsDir, "fonts")
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

// setupData makes an installation whole once the game's files are in dst:
// the port's own assets (the text scripts) and the cover picture.
func setupData(dst string) error {
	const scripts = "data/assets/scripts"
	err := fs.WalkDir(por.Assets, scripts, func(p string, d fs.DirEntry, err error) error { // not os.CopyFS: a rerun overwrites
		if err != nil || d.IsDir() {
			return err
		}
		b, err := por.Assets.ReadFile(p)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, engine.AssetsDir, "scripts", filepath.FromSlash(strings.TrimPrefix(p, scripts+"/")))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
	if err != nil {
		return err
	}
	return writeCover(dst)
}

// writeCover renders TITLE.DAX block 2, the title picture.
func writeCover(dst string) error {
	l, err := dax.NewLoader(filepath.Join(dst, engine.DOSDir))
	if err != nil {
		return err
	}
	b := l.Block("TITLE", 2)
	if b == nil {
		return errors.New("TITLE.DAX has no title picture")
	}
	im := dax.ParseImage(b, false, 0)
	cp := make(color.Palette, 16)
	for i := range cp {
		cp[i] = pal.EGA(i)
	}
	img := image.NewPaletted(image.Rect(0, 0, pal.W, pal.H), cp)
	d, w := im.Frame(0), im.Width*8
	for y := 0; y < im.Height && y < pal.H; y++ {
		for x := 0; x < w && x < pal.W; x++ {
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
	slot   byte // pal.Amiga palette
	split  bool
	sprite bool // animation block (PIC/FINAL frames XORed against the first)
}{
	"icon": {pal.AmigaTerrain, false, false}, "comspr": {pal.AmigaTerrain, false, false}, "cpic": {pal.AmigaTerrain, false, false},
	"bacpac": {pal.AmigaTerrain, true, false}, "sqrpaci": {pal.AmigaTerrain, true, false}, "wildcom": {pal.AmigaTerrain, true, false},
	"dungcom": {pal.AmigaTerrain, true, false}, "randcom": {pal.AmigaTerrain, true, false},
	"head": {pal.AmigaPortrait, false, false}, "body": {pal.AmigaPortrait, false, false}, "chead": {pal.AmigaTerrain, false, false}, "cbody": {pal.AmigaTerrain, false, false},
	"pic": {pal.AmigaView, false, true}, "final": {pal.AmigaView, false, true}, "sprit": {pal.AmigaPortrait, false, true},
}

// importDisk2 (-import-disk2, goplay drive B) finds the Amiga Pool of
// Radiance data disk among the .adf images under src (loose or zipped) and
// writes its graphics, converted to DOS-layout archives, to dst/amiga.
func importDisk2(src, dst string) error {
	var disk []adf.File
	var snd []byte        // disk 1's program: the Amiga sound players and samples
	var title image.Image // disk 1's intro: the painted title
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
			if strings.EqualFold(f.Path, "intro") && title == nil {
				title = amiga.Title(f.Data)
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
	out := filepath.Join(dst, engine.AmigaDir)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if snd != nil {
		if err := os.WriteFile(filepath.Join(out, "SOUND.BIN"), snd, 0o644); err != nil {
			return err
		}
	}
	if title != nil {
		var b bytes.Buffer
		png.Encode(&b, title)
		if err := os.WriteFile(filepath.Join(out, "TITLE.PNG"), b.Bytes(), 0o644); err != nil {
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
