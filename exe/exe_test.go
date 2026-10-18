package exe

import "testing"

func TestLoad(t *testing.T) {
	im, err := Load("../START.EXE")
	if err != nil {
		t.Fatal(err)
	}
	if len(im.Mem) != 68592 {
		t.Fatalf("len %d", len(im.Mem))
	}
	if s := string(im.Mem[im.DS+0x5F1:][:23]); s != "C D M T V A R L S B E J" {
		t.Fatalf("DS:5F0 = %q", s)
	}
}
