package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// writeGame makes the smallest folder importGame accepts (no START.EXE).
func writeGame(t *testing.T, files map[string][]byte, zipName string) string {
	src := t.TempDir()
	if zipName == "" {
		for n, b := range files {
			os.WriteFile(filepath.Join(src, n), b, 0o644)
		}
		return src
	}
	f, _ := os.Create(filepath.Join(src, zipName))
	w := zip.NewWriter(f)
	for n, b := range files {
		fw, _ := w.Create(n)
		fw.Write(b)
	}
	w.Close()
	f.Close()
	return src
}

func TestImportZipAndLoose(t *testing.T) {
	files := map[string][]byte{"Pool/title.dax": {0, 0}, "Pool/8x8d1.dax": {2}, "Pool/items": {3}}
	loose := map[string][]byte{"title.dax": {0, 0}, "8x8d1.dax": {2}, "items": {3}}
	for name, src := range map[string]string{"zip": writeGame(t, files, "pool.zip"), "loose": writeGame(t, loose, "")} {
		dst := t.TempDir()
		// the cover needs a real TITLE.DAX, so only the file copy is checked
		importGame(src, dst)
		for _, n := range []string{"TITLE.DAX", "8X8D1.DAX", "ITEMS"} {
			if _, err := os.Stat(filepath.Join(dst, "dos", n)); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	}
	if err := importGame(writeGame(t, map[string][]byte{"a.txt": {1}}, ""), t.TempDir()); err == nil {
		t.Error("no game: want an error")
	}
}
