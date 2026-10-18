package engine_test

import (
	"image"
	"os"
	"slices"
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
	if !slices.Contains(v.Battle.Reach, to) {
		t.Fatalf("(%d,%d) next to the active unit is not in Reach", to.X, to.Y)
	}
	if p := v.Battle.Paths[to]; len(p) != 2 || p[0].Point != image.Pt(u.X, u.Y) || p[1].Point != to {
		t.Fatalf("path to (%d,%d) next to the active unit: %v", to.X, to.Y, p)
	}
	v = keyWait(e, scr, engine.SquareKey(to.X, to.Y)) // at the main menu: walks there and hands back
	if got := active(v); got.X != to.X || got.Y != to.Y {
		t.Errorf("clicked square (%d,%d): unit at (%d,%d)", to.X, to.Y, got.X, got.Y)
	}
	if v.Menu == nil {
		t.Errorf("after the walk: no menu (still in Move?)")
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

// TestClickFoe: a foe clicked at the main menu is attacked at once: the
// turn moves on (walking up to it first when it stands out of reach).
func TestClickFoe(t *testing.T) {
	e, scr, v := battleView(t)
	var me, foe engine.Unit
	for _, u := range v.Battle.Units {
		if u.Active {
			me = u
		} else if !u.Party && foe.Name == "" {
			foe = u
		}
	}
	if foe.Name == "" {
		t.Skip("no foe")
	}
	v = keyWait(e, scr, engine.SquareKey(foe.X, foe.Y))
	if v.Battle == nil {
		return // the fight moved off the map (the end): the click did something
	}
	for _, u := range v.Battle.Units {
		if u.Active && u.ID == me.ID {
			t.Fatalf("clicked foe %s at (%d,%d) from (%d,%d): still %s's turn (cursor on: %v, menu %+v)", foe.Name, foe.X, foe.Y, me.X, me.Y, me.Name, v.Battle.CursorOn, v.Menu)
		}
	}
}

// TestCombatActions: the main menu publishes its turn enders and the foes
// a click attacks; Wait ends the turn; a click on one's own figure opens
// the sheet.
func TestCombatActions(t *testing.T) {
	e, scr, v := battleView(t)
	var me engine.Unit
	for _, u := range v.Battle.Units {
		if u.Active {
			me = u
		}
	}
	if !slices.ContainsFunc(v.Battle.Actions, func(m engine.MenuItem) bool { return m.Text == "Wait" }) {
		t.Errorf("Actions %+v: no Wait", v.Battle.Actions)
	}
	if len(v.Battle.Targets) == 0 {
		t.Errorf("no Targets with foes about")
	}
	for _, u := range v.Battle.Units {
		if slices.Contains(v.Battle.Targets, u.ID) && (u.Hit < 5 || u.Hit > 100) {
			t.Errorf("target %s: hit chance %d%%, want 5..100", u.Name, u.Hit)
		}
	}
	if v := keyWait(e, scr, engine.SquareKey(me.X, me.Y)); v.Battle != nil {
		t.Errorf("a click on one's own figure: still on the map, not the sheet")
	}
	keyWait(e, scr, 'E') // leave the sheet
	v = keyWait(e, scr, 'W')
	if v.Battle != nil {
		for _, u := range v.Battle.Units {
			if u.Active && u.ID == me.ID {
				t.Errorf("Wait: still %s's turn", me.Name)
			}
		}
	}
}

// TestAimCycle: Next while aiming visits foes only, never the aimer, and
// the cursor's flight between them is no effect in the view.
func TestAimCycle(t *testing.T) {
	e, scr, v := battleView(t)
	var me engine.Unit
	party := map[image.Point]bool{}
	for _, u := range v.Battle.Units {
		if u.Active {
			me = u
		}
		if u.Party {
			party[image.Pt(u.X, u.Y)] = true
		}
	}
	v = keyWait(e, scr, 'A')
	for i := range 6 {
		if v.Battle == nil {
			t.Fatalf("step %d: off the map", i)
		}
		if len(v.Battle.Effects) > 0 {
			t.Errorf("step %d: %d effects while aiming", i, len(v.Battle.Effects))
		}
		if c := v.Battle.Cursor; v.Battle.CursorOn && party[c] {
			t.Errorf("step %d: the cursor on a party member at %v (aimer %s at %d,%d)", i, c, me.Name, me.X, me.Y)
		}
		v = keyWait(e, scr, 'N')
	}
}

// TestAimHover: pointing at another foe while aiming moves the aim to it.
func TestAimHover(t *testing.T) {
	e, scr, v := battleView(t)
	v = keyWait(e, scr, 'A')
	if v.Battle == nil || !v.Battle.CursorOn {
		t.Fatal("aiming: no cursor")
	}
	from := v.Battle.Cursor
	for _, u := range v.Battle.Units {
		if u.Party || image.Pt(u.X, u.Y) == from {
			continue
		}
		if v = keyWait(e, scr, engine.HoverKey(u.X, u.Y)); v.Battle != nil && v.Battle.Cursor == image.Pt(u.X, u.Y) {
			return
		}
	}
	t.Errorf("pointing at the other foes never moved the aim from %v", from)
}

// TestViewMember: a portrait click (KeyViewMember) on the command bar opens
// that member's View page.
func TestViewMember(t *testing.T) {
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
	keyWait(e, scr, 'L')
	for ; i > 0; i-- {
		keyWait(e, scr, 0x5000)
	}
	v := keyWait(e, scr, 13)
	if len(v.Party) < 2 {
		t.Skip("party of one")
	}
	if v = keyWait(e, scr, engine.KeyViewMember+1); v.MainFrame {
		t.Fatal("portrait click: still on the main screen")
	}
	if !v.Party[1].Selected {
		t.Errorf("portrait click: member 2 not the one viewed")
	}
}
