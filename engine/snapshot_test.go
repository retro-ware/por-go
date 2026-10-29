package engine_test

import (
	"image"
	"os"
	"slices"
	"testing"
	"time"

	"por/engine"
)

// TestView loads saved game A headless (needs the game files in the repo
// root) and checks the published View as the party turns and opens a sheet.
func TestView(t *testing.T) {
	if _, err := os.Stat("../TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	e, err := engine.New("..")
	if err != nil {
		t.Fatal(err)
	}
	e.Init()
	go e.Play()
	i := e.SlotIndex("GAME A")
	if i < 0 {
		t.Skip("no GAME A save")
	}
	keys := func(as ...engine.Action) engine.View {
		for _, a := range as {
			for !e.Idle() {
				time.Sleep(10 * time.Millisecond)
			}
			e.Act(resolve(e, a))
			time.Sleep(50 * time.Millisecond)
		}
		for !e.Idle() {
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(100 * time.Millisecond)
		return e.View()
	}
	load := []engine.Action{hot('L')}
	for ; i > 0; i-- {
		load = append(load, down)
	}
	v := keys(append(load, cont)...)
	if !v.MainFrame || v.Window != engine.WinThreeD {
		t.Fatalf("after loading: MainFrame %v Window %v, want the 3D view in the main frame", v.MainFrame, v.Window)
	}
	if len(v.Party) == 0 || v.Party[0].Name == "" || v.Party[0].Head == nil || v.Party[0].MaxHP <= 0 {
		t.Fatalf("party %+v: want named members with heads", v.Party)
	}
	f := v.Facing
	v = keys(engine.Action{Kind: engine.ActMove, N: engine.DirRight}) // turn right
	if v.Facing != (f+1)%4 || !v.MainFrame || v.Window != engine.WinThreeD {
		t.Errorf("after turning right from %v: %+v", f, v)
	}
	v = keys(hot('V')) // the character sheet covers the frame
	if v.MainFrame {
		t.Errorf("character sheet: MainFrame still set")
	}
	v = keys(engine.Action{Kind: engine.ActClose})
	if !v.MainFrame {
		t.Errorf("back from the sheet: MainFrame not set")
	}
}

func TestBattleView(t *testing.T) {
	e, v := battleView(t)
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
	_ = e
}

// hot stands for the menu item with hotkey c (resolve finds its index).
func hot(c byte) engine.Action { return engine.Action{Kind: engine.ActPick, N: -int(c)} }

var down, cont = engine.Action{Kind: engine.ActMove, N: engine.DirDown}, engine.Action{Kind: engine.ActContinue}

// resolve turns hot(c) into the pick of that item on the menu shown.
func resolve(e *engine.Engine, a engine.Action) engine.Action {
	if a.Kind != engine.ActPick || a.N >= 0 {
		return a
	}
	if m := e.View().Menu; m != nil {
		for i, it := range m.Items {
			if it.Key == byte(-a.N) {
				a.N = i
				return a
			}
		}
	}
	return a
}

func keyWait(e *engine.Engine, a engine.Action) engine.View {
	for !e.Idle() {
		time.Sleep(10 * time.Millisecond)
	}
	e.Act(resolve(e, a))
	time.Sleep(50 * time.Millisecond)
	for !e.Idle() {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	return e.View()
}

// battleView loads GAME A and starts the dev combat scene.
func battleView(t *testing.T) (*engine.Engine, engine.View) {
	if _, err := os.Stat("../TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	e, err := engine.New("..")
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
	ks := []engine.Action{hot('L')}
	for ; i > 0; i-- {
		ks = append(ks, down)
	}
	var v engine.View
	for _, k := range append(ks, cont, cont, cont, cont, cont, cont, cont) {
		for !e.Idle() {
			time.Sleep(10 * time.Millisecond)
		}
		if v = e.View(); v.Battle != nil {
			break
		}
		e.Act(resolve(e, k))
		time.Sleep(50 * time.Millisecond)
	}
	return e, v
}

// TestWalk: ActWalk to a square in Reach moves the active unit there and
// the turn goes on.
func TestWalk(t *testing.T) {
	e, v := battleView(t)
	if v.Battle == nil || v.Battle.Turn == nil {
		t.Fatal("no turn in combat")
	}
	u := active(t, v)
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
	v = actWait(e, engine.Action{Kind: engine.ActWalk, X: to.X, Y: to.Y})
	if got := active(t, v); got.X != to.X || got.Y != to.Y {
		t.Errorf("walked to (%d,%d): unit at (%d,%d)", to.X, to.Y, got.X, got.Y)
	}
	if v.Battle.Turn == nil {
		t.Errorf("after the walk: no turn")
	}
}

func active(t *testing.T, v engine.View) engine.Unit {
	for _, u := range v.Battle.Units {
		if u.Active {
			return u
		}
	}
	t.Fatal("no active unit")
	return engine.Unit{}
}

// TestBattleFight plays the dev fight out on Quick and watches the view:
// blows show effects, the fallen leave the units.
func TestBattleFight(t *testing.T) {
	e, v := battleView(t)
	if v.Battle == nil {
		t.Fatal("no Battle in combat")
	}
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
		switch {
		case v.Battle != nil && v.Battle.Ask != "":
			actWait(e, engine.Action{Kind: engine.ActClose})
		case v.Battle != nil && v.Battle.Turn != nil:
			actWait(e, engine.Action{Kind: engine.ActQuick})
		default:
			keyWait(e, cont)
		}
	}
	close(done)
	if !effects {
		t.Errorf("no effects seen in the whole fight")
	}
	if !fewer {
		t.Errorf("nobody fell in the whole fight")
	}
}

// TestAttackFoe: ActAttack on a target foe attacks it at once (walking
// up to it first when it stands out of reach): the turn moves on.
func TestAttackFoe(t *testing.T) {
	e, v := battleView(t)
	me := active(t, v)
	if len(v.Battle.Targets) == 0 {
		t.Skip("no target")
	}
	v = actWait(e, engine.Action{Kind: engine.ActAttack, N: v.Battle.Targets[0]})
	if v.Battle == nil {
		return // the fight ended
	}
	for _, u := range v.Battle.Units {
		if u.Active && u.ID == me.ID && v.Battle.Turn != nil && v.Battle.Turn.Unit == me.ID {
			t.Fatalf("attacked: still %s's turn", me.Name)
		}
	}
}

// TestCombatActions: the turn offers Wait and the foes ActAttack takes,
// with their chance to be hit; Wait ends the turn.
func TestCombatActions(t *testing.T) {
	e, v := battleView(t)
	me := active(t, v)
	if !slices.Contains(v.Battle.Turn.Can, engine.ActWait) {
		t.Errorf("Can %+v: no Wait", v.Battle.Turn.Can)
	}
	if len(v.Battle.Targets) == 0 {
		t.Errorf("no Targets with foes about")
	}
	for _, u := range v.Battle.Units {
		if slices.Contains(v.Battle.Targets, u.ID) && (u.Hit < 5 || u.Hit > 100) {
			t.Errorf("target %s: hit chance %d%%, want 5..100", u.Name, u.Hit)
		}
	}
	v = actWait(e, engine.Action{Kind: engine.ActWait})
	if v.Battle != nil && v.Battle.Turn != nil && v.Battle.Turn.Unit == me.ID {
		t.Errorf("Wait: still %s's turn", me.Name)
	}
}

// TestAimCycle: aiming a weapon cycles foes only, never the aimer.
func TestAimCycle(t *testing.T) {
	e, v := battleView(t)
	v = nextToFoe(t, e, v)
	v = actWait(e, engine.Action{Kind: engine.ActAttack})
	if v.Battle == nil || v.Battle.Turn == nil || !v.Battle.Turn.Aim {
		t.Fatal("not aiming")
	}
	for _, u := range v.Battle.Units {
		if slices.Contains(v.Battle.Targets, u.ID) && u.Party {
			t.Errorf("party member %s among the aim's targets", u.Name)
		}
	}
	if c := v.Battle.Cursor; v.Battle.CursorOn && c == image.Pt(active(t, v).X, active(t, v).Y) {
		t.Errorf("the cursor starts on the aimer")
	}
}

// TestAimHover: pointing at another square while aiming moves the aim there.
func TestAimHover(t *testing.T) {
	e, v := battleView(t)
	v = nextToFoe(t, e, v)
	v = actWait(e, engine.Action{Kind: engine.ActAttack})
	if v.Battle == nil || !v.Battle.CursorOn {
		t.Fatal("aiming: no cursor")
	}
	for _, u := range v.Battle.Units {
		if u.Party || image.Pt(u.X, u.Y) == v.Battle.Cursor {
			continue
		}
		if v = actWait(e, engine.Action{Kind: engine.ActPoint, X: u.X, Y: u.Y}); v.Battle == nil || v.Battle.Cursor != image.Pt(u.X, u.Y) {
			t.Errorf("pointing at %s: the aim stays at %v", u.Name, v.Battle.Cursor)
		}
		return
	}
	t.Skip("one foe")
}

// nextToFoe walks the active unit to a square next to a foe (Aim takes
// only a foe in reach of the weapon): the dev fight starts out of reach.
func nextToFoe(t *testing.T, e *engine.Engine, v engine.View) engine.View {
	for _, u := range v.Battle.Units {
		if u.Party {
			continue
		}
		for d := range 9 {
			sq := image.Pt(u.X+d%3-1, u.Y+d/3-1)
			if slices.Contains(v.Battle.Reach, sq) {
				return actWait(e, engine.Action{Kind: engine.ActWalk, X: sq.X, Y: sq.Y})
			}
		}
	}
	t.Skip("no square next to a foe in reach")
	return v
}

// TestAimOutOfReach: with no foe in reach of the weapon Attack does not aim.
func TestAimOutOfReach(t *testing.T) {
	e, _ := battleView(t)
	v := actWait(e, engine.Action{Kind: engine.ActAttack})
	if v.Battle == nil || v.Battle.Turn == nil || v.Battle.Turn.Aim {
		t.Errorf("aiming with every foe two squares off a melee fighter")
	}
}

// TestViewMember: a portrait click (KeyViewMember) on the command bar opens
// that member's View page.
func TestViewMember(t *testing.T) {
	if _, err := os.Stat("../TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	e, err := engine.New("..")
	if err != nil {
		t.Fatal(err)
	}
	e.Init()
	go e.Play()
	i := e.SlotIndex("GAME A")
	if i < 0 {
		t.Skip("no GAME A save")
	}
	keyWait(e, hot('L'))
	for ; i > 0; i-- {
		keyWait(e, down)
	}
	v := keyWait(e, cont)
	if len(v.Party) < 2 {
		t.Skip("party of one")
	}
	if v = keyWait(e, engine.Action{Kind: engine.ActMember, N: 1}); v.MainFrame {
		t.Fatal("portrait click: still on the main screen")
	}
	if !v.Party[1].Selected {
		t.Errorf("portrait click: member 2 not the one viewed")
	}
}

// actWait sends a and waits for the engine to take it.
func actWait(e *engine.Engine, a engine.Action) engine.View {
	e.Act(a)
	time.Sleep(50 * time.Millisecond)
	for e.Busy() {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	return e.View()
}
