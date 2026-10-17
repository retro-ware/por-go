package engine_test

import (
	"os"
	"testing"
	"time"

	"por/ega"
	"por/engine"
)

// TestView loads saved game A headless (needs the game files in the repo
// root) and checks the published View as the party turns and opens a sheet.
func TestView(t *testing.T) {
	if _, err := os.Stat("../TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	scr := ega.New()
	e, err := engine.New("..", scr)
	if err != nil {
		t.Fatal(err)
	}
	e.Init()
	go e.Play()
	i := e.SlotIndex("GAME A")
	if i < 0 {
		t.Skip("no GAME A save")
	}
	keys := func(ks ...uint16) engine.View {
		for _, k := range ks {
			for !scr.Idle() {
				time.Sleep(10 * time.Millisecond)
			}
			scr.PushKey(k)
			time.Sleep(50 * time.Millisecond)
		}
		for !scr.Idle() {
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(100 * time.Millisecond)
		return e.View()
	}
	load := []uint16{'L'}
	for ; i > 0; i-- {
		load = append(load, 0x5000)
	}
	v := keys(append(load, 13)...)
	if !v.MainFrame || v.Window != engine.WinThreeD {
		t.Fatalf("after loading: MainFrame %v Window %v, want the 3D view in the main frame", v.MainFrame, v.Window)
	}
	if len(v.Party) == 0 || v.Party[0].Name == "" || v.Party[0].Head == nil || v.Party[0].MaxHP <= 0 {
		t.Fatalf("party %+v: want named members with heads", v.Party)
	}
	if v.Area.Disk == 0 && v.Area.Block == 0 {
		t.Errorf("area %+v unset", v.Area)
	}
	f := v.Facing
	v = keys(0x4D00) // turn right
	if v.Facing != (f+1)%4 || !v.MainFrame || v.Window != engine.WinThreeD {
		t.Errorf("after turning right from %v: %+v", f, v)
	}
	v = keys('V') // the character sheet covers the frame
	if v.MainFrame {
		t.Errorf("character sheet: MainFrame still set")
	}
	v = keys(27)
	if !v.MainFrame {
		t.Errorf("back from the sheet: MainFrame not set")
	}
}
