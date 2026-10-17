package engine_test

import (
	"image"
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
	e, scr, v := battleView(t)
	b := v.Battle
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
	if v := keyWait(e, scr, 'V'); v.Battle != nil {
		t.Errorf("character sheet: Battle still set")
	}
	if v := keyWait(e, scr, 27); v.Battle == nil {
		t.Errorf("back from the sheet: no Battle")
	}
}

func keyWait(e *engine.Engine, scr *ega.Screen, k uint16) engine.View {
	scr.PushKey(k)
	time.Sleep(50 * time.Millisecond)
	for !scr.Idle() {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	return e.View()
}

// battleView loads GAME A and starts the dev combat scene.
func battleView(t *testing.T) (*engine.Engine, *ega.Screen, engine.View) {
	if _, err := os.Stat("../TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	scr := ega.New()
	e, err := engine.New("..", scr)
	if err != nil {
		t.Fatal(err)
	}
	e.DevAction = e.DevScene("combat")
	seed := uint32(7) // the same fight every run
	e.FixedSeed = &seed
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
	return e, scr, v
}

// TestSquareKey: a square clicked on the remastered map moves the active
// unit as a click on the original's page does, and does nothing where
// the game takes no clicks on the map.
func TestSquareKey(t *testing.T) {
	e, scr, v := battleView(t)
	if v.Battle == nil {
		t.Fatal("no Battle in combat")
	}
	active := func(v engine.View) engine.Unit {
		for _, u := range v.Battle.Units {
			if u.Active {
				return u
			}
		}
		t.Fatal("no active unit")
		return engine.Unit{}
	}
	u := active(v)
	free := func(x, y int) bool {
		if x < 0 || y < 0 || x >= 50 || y >= 25 || v.Battle.Ground[y][x].Wall {
			return false
		}
		for _, o := range v.Battle.Units {
			if o.X == x && o.Y == y {
				return false
			}
		}
		return true
	}
	to := image.Point{-1, -1}
	for _, d := range []image.Point{{0, 1}, {1, 0}, {-1, 0}, {0, -1}} {
		if free(u.X+d.X, u.Y+d.Y) {
			to = image.Pt(u.X+d.X, u.Y+d.Y)
			break
		}
	}
	if to.X < 0 {
		t.Skip("active unit boxed in")
	}
	if v := keyWait(e, scr, engine.SquareKey(to.X, to.Y)); active(v).X != u.X || active(v).Y != u.Y {
		t.Fatalf("a square clicked at the menu moved the unit")
	}
	keyWait(e, scr, 'M')
	if got := active(keyWait(e, scr, engine.SquareKey(to.X, to.Y))); got.X != to.X || got.Y != to.Y {
		t.Errorf("clicked square (%d,%d): unit at (%d,%d)", to.X, to.Y, got.X, got.Y)
	}
}

// TestBattleFight plays the dev fight out on Quick and watches the view:
// aiming shows the cursor, blows show effects, the fallen leave the units.
func TestBattleFight(t *testing.T) {
	e, scr, v := battleView(t)
	if v.Battle == nil {
		t.Fatal("no Battle in combat")
	}
	if v := keyWait(e, scr, 'A'); v.Battle == nil || !v.Battle.CursorOn || len(v.Battle.CursorAt) == 0 {
		t.Errorf("aiming: no cursor in the view")
	}
	keyWait(e, scr, 27)
	units := len(v.Battle.Units)
	effects, fewer := false, false
	done := make(chan bool)
	go func() { // sample the view as the fight runs (Delay publishes)
		for {
			select {
			case <-done:
				return
			default:
			}
			if b := e.View().Battle; b != nil {
				effects = effects || len(b.Effects) > 0
				fewer = fewer || len(b.Units) < units
			}
			time.Sleep(time.Millisecond)
		}
	}()
	for range 400 {
		v := e.View()
		if v.Battle == nil && v.MainFrame {
			break // back to the main screen: the fight is over
		}
		k := uint16(13)
		if v.Menu != nil {
			for _, it := range v.Menu.Items {
				if it.Key == 'Q' {
					k = 'Q'
				}
			}
		}
		keyWait(e, scr, k)
	}
	close(done)
	if !effects {
		t.Errorf("no effects seen in the whole fight")
	}
	if !fewer {
		t.Errorf("nobody fell in the whole fight")
	}
}
