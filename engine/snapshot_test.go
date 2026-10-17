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

func TestBattleView(t *testing.T) {
	b := battleView(t).Battle
	if b == nil {
		t.Fatal("no Battle in combat")
	}
	party, foes, walls := 0, 0, 0
	for _, u := range b.Units {
		if u.Icon == nil {
			t.Errorf("unit %+v has no icon", u)
		}
		if u.Party {
			party++
		} else {
			foes++
		}
	}
	for _, row := range b.Ground {
		for _, s := range row {
			if s.Wall {
				walls++
			}
			if b.Tiles[s.Tile] == nil {
				t.Fatalf("square tile %d missing from Tiles", s.Tile)
			}
		}
	}
	if party == 0 || foes == 0 || walls == 0 {
		t.Errorf("party %d foes %d walls %d: want some of each", party, foes, walls)
	}
}

// battleView loads GAME A and starts the dev combat scene.
func battleView(t *testing.T) engine.View {
	if _, err := os.Stat("../TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	scr := ega.New()
	e, err := engine.New("..", scr)
	if err != nil {
		t.Fatal(err)
	}
	e.DevAction = e.DevScene("combat")
	e.Init()
	go e.Play()
	i := e.SlotIndex("GAME A")
	if i < 0 {
		t.Skip("no GAME A save")
	}
	ks := []uint16{'L'}
	for ; i > 0; i-- {
		ks = append(ks, 0x5000)
	}
	var v engine.View
	for _, k := range append(ks, 13, ' ', ' ', ' ', ' ', ' ', ' ') {
		for !scr.Idle() {
			time.Sleep(10 * time.Millisecond)
		}
		if v = e.View(); v.Battle != nil {
			break
		}
		scr.PushKey(k)
		time.Sleep(50 * time.Millisecond)
	}
	return v
}
