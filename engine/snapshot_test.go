package engine_test

import (
	"image"
	"os"
	"slices"
	"testing"
	"time"

	"por/engine"
)

// explore: a headless engine (needs the game files in the repo root) that
// begins adventuring with the Quick Start party once Init and Play run, as
// -explore does; its roster goes to a temp dir, never the game's saves.
func explore(t *testing.T) *engine.Engine {
	t.Helper()
	if _, err := os.Stat("../data/dos/TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	e, err := engine.New("../data")
	if err != nil {
		t.Fatal(err)
	}
	e.Cfg.SaveDir = t.TempDir()
	e.Explore = true
	return e
}

// TestView starts a game headless and checks the published View as the
// party turns and opens a sheet.
func TestView(t *testing.T) {
	e := explore(t)
	e.Init()
	e.Start(e.Play)
	keys := func(as ...engine.Action) engine.View {
		for _, a := range as {
			keyWait(e, a)
		}
		return settle(e)
	}
	v := keys()
	if v.Pane == nil || v.Window != engine.WinThreeD {
		t.Fatalf("after loading: Pane %v Window %v, want the 3D view in the main frame", v.Pane != nil, v.Window)
	}
	if len(v.Party) == 0 || v.Party[0].Name == "" || v.Party[0].Head == nil || v.Party[0].MaxHP <= 0 {
		t.Fatalf("party %+v: want named members with heads", v.Party)
	}
	f := v.Facing
	v = keys(engine.Action{Kind: engine.ActTurnRight})
	if v.Facing != (f+1)%4 || v.Pane == nil || v.Window != engine.WinThreeD {
		t.Errorf("after turning right from %v: %+v", f, v)
	}
	v = keys(engine.Action{Kind: engine.ActMember, N: -1}) // the character sheet covers the frame
	if v.Pane != nil {
		t.Errorf("character sheet: MainFrame still set")
	}
	v = keys(engine.Action{Kind: engine.ActClose})
	if v.Pane == nil {
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
func hot(c byte) engine.Action { return engine.Action{Kind: engine.ActChoose, N: -int(c)} }

var cont = engine.Action{Kind: engine.ActOK}

// resolve turns hot(c) into the pick of that item on the menu shown.
func resolve(e *engine.Engine, a engine.Action) engine.Action {
	if a.Kind != engine.ActChoose || a.N >= 0 {
		return a
	}
	if m := e.View().Ask; m != nil {
		for i, it := range m.Items {
			if it.Key == byte(-a.N) {
				a.N = i
				return a
			}
		}
	}
	return a
}

// keyWait applies a (hot resolved) and lets the engine settle.
func keyWait(e *engine.Engine, a engine.Action) engine.View {
	e.Apply(resolve(e, a))
	return settle(e)
}

// settle ticks the engine, as the window does once a frame, while it is in
// a timed loop (View.Ticks), for d at most (default 300 ms: a picture may
// animate for ever).
func settle(e *engine.Engine, d ...time.Duration) engine.View {
	v, max := e.View(), 300*time.Millisecond
	if len(d) > 0 {
		max = d[0]
	}
	for t0 := time.Now(); v.Ticks && time.Since(t0) < max; {
		time.Sleep(time.Millisecond)
		v = e.Apply(engine.Action{Kind: engine.ActTick})
	}
	return v
}

// battleView starts a game and the dev combat scene.
func battleView(t *testing.T) (*engine.Engine, engine.View) { return battleScene(t, "combat") }

// battleScene starts a game and the dev fight scene name.
func battleScene(t *testing.T, name string) (*engine.Engine, engine.View) {
	e := explore(t)
	e.DevAction = e.DevScene(name)
	seed := uint32(1) // the same fight every run (and one whose first unit can walk and reach a foe)
	e.FixedSeed = &seed
	e.Init()
	e.Start(e.Play)
	v := settle(e)
	for range 7 {
		if v.Battle != nil {
			break
		}
		v = keyWait(e, cont)
	}
	if v.Battle == nil {
		t.Fatal("no fight")
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

// TestBattleFight plays the dev fight out on Quick and watches its events
// (Apply hands them out): blows show effects, the fallen leave the units.
func TestBattleFight(t *testing.T) {
	e, v := battleView(t)
	if v.Battle == nil {
		t.Fatal("no Battle in combat")
	}
	units := len(v.Battle.Units)
	effects, fewer, died := false, false, false
	apply := func(a engine.Action) engine.View {
		v := e.Apply(a)
		for _, ev := range v.Events {
			died = died || ev.Kind == engine.EvDied
			if b := ev.Battle; b != nil {
				effects = effects || len(b.Effects) > 0
				fewer = fewer || len(b.Units) < units
			}
		}
		return v
	}
	for range 2000 { // the fight's turns, one a tick
		v = e.View()
		if v.Battle == nil && v.Pane != nil {
			break // back to the main screen: the fight is over
		}
		switch {
		case v.Ticks:
			apply(engine.Action{Kind: engine.ActTick})
		case v.Ask != nil && v.Ask.Kind == engine.AskConfirm:
			apply(engine.Action{Kind: engine.ActClose})
		case v.Battle != nil && v.Battle.Turn != nil:
			apply(engine.Action{Kind: engine.ActQuick})
		default:
			apply(cont)
		}
	}
	if !effects {
		t.Errorf("no effects seen in the whole fight")
	}
	if !fewer || !died {
		t.Errorf("nobody fell in the whole fight (units fewer %v, EvDied %v)", fewer, died)
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
	nextToFoe(t, e, v)
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
	nextToFoe(t, e, v)
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

// events applies a and settles, gathering the fight's events.
func events(e *engine.Engine, a engine.Action) (engine.View, map[engine.EventKind]int) {
	n := map[engine.EventKind]int{}
	v := e.Apply(a)
	for range 1000 { // a picture may animate for ever
		for _, ev := range v.Events {
			n[ev.Kind]++
		}
		if !v.Ticks {
			break
		}
		v = e.Apply(engine.Action{Kind: engine.ActTick})
	}
	return v, n
}

// TestAimStrike: an aim taken on a foe strikes it (EvAttack) and ends the turn.
func TestAimStrike(t *testing.T) {
	e, v := battleView(t)
	v = nextToFoe(t, e, v)
	me := active(t, v)
	v = actWait(e, engine.Action{Kind: engine.ActAttack})
	if v.Battle == nil || v.Battle.Turn == nil || !v.Battle.Turn.Aim {
		t.Fatal("not aiming")
	}
	c := v.Battle.Cursor
	v, n := events(e, engine.Action{Kind: engine.ActAim, X: c.X, Y: c.Y})
	if n[engine.EvAttack] == 0 {
		t.Errorf("aim taken at %v: no EvAttack (events %v)", c, n)
	}
	if v.Battle != nil && v.Battle.Turn != nil && (v.Battle.Turn.Aim || v.Battle.Turn.Unit == me.ID) {
		t.Errorf("after the strike: still aiming or %s's turn", me.Name)
	}
}

// TestCastAimed: Cure Light Wounds begins casting; on the caster's next
// turn the player aims it first (aimSpell), then it is cast (EvSpell) and
// the turn ends.
func TestCastAimed(t *testing.T) {
	e, v := battleScene(t, "castfight")
	if v.Battle == nil || v.Battle.Turn == nil || len(v.Battle.Turn.Spells) == 0 {
		t.Skip("no caster's turn")
	}
	me := active(t, v)
	v, _ = events(e, engine.Action{Kind: engine.ActCast, N: v.Battle.Turn.Spells[0].ID})
	for range 20 { // the others' turns go by until the caster's spell goes off
		if v.Battle == nil || v.Battle.Turn == nil || v.Battle.Turn.Aim {
			break
		}
		v, _ = events(e, engine.Action{Kind: engine.ActEnd})
	}
	if v.Battle == nil || v.Battle.Turn == nil || !v.Battle.Turn.Aim || v.Battle.Turn.Unit != me.ID {
		t.Fatal("the spell begun: no aim on the caster's next turn")
	}
	at := active(t, v)
	v, n := events(e, engine.Action{Kind: engine.ActAim, X: at.X, Y: at.Y})
	if n[engine.EvSpell] != 1 {
		t.Errorf("aim taken on the caster: events %v, want one EvSpell", n)
	}
	if v.Battle != nil && v.Battle.Turn != nil && v.Battle.Turn.Unit == me.ID {
		t.Errorf("after the cast: still %s's turn", me.Name)
	}
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
	e := explore(t)
	e.Init()
	e.Start(e.Play)
	v := settle(e)
	if len(v.Party) < 2 {
		t.Fatal("party of one")
	}
	if v = keyWait(e, engine.Action{Kind: engine.ActMember, N: 1}); v.Pane != nil {
		t.Fatal("portrait click: still on the main screen")
	}
	if !v.Party[1].Selected {
		t.Errorf("portrait click: member 2 not the one viewed")
	}
}

// actWait sends a and waits for the engine to take it.
func actWait(e *engine.Engine, a engine.Action) engine.View {
	v := settle(e)
	for v = keyWait(e, a); v.Ask != nil && v.Ask.Kind == engine.AskPause; {
		v = keyWait(e, cont) // the presenter's part: a pause is over at once
	}
	return v
}

// TestAttackRefusal: a foe that is no target (here: a party member) is
// refused with the reason on the note, the turn going on.
func TestAttackRefusal(t *testing.T) {
	e, v := battleView(t)
	if v.Battle == nil || v.Battle.Turn == nil {
		t.Fatal("no turn")
	}
	actWait(e, engine.Action{Kind: engine.ActAttack, N: v.Battle.Turn.Unit})
	if s, _ := e.Note(); s != "Not a target" {
		t.Errorf("note %q, want Not a target", s)
	}
	if e.View().Battle.Turn == nil {
		t.Error("the turn was used")
	}
}

// TestFightSheet: the card's right click (ActMember) opens the active
// member's sheet, not the first member's.
func TestFightSheet(t *testing.T) {
	e, v := battleView(t)
	var active string
	for _, u := range v.Battle.Units {
		if u.ID == v.Battle.Turn.Unit {
			active = u.Name
		}
	}
	v = actWait(e, engine.Action{Kind: engine.ActMember, N: 0})
	var selected string
	for _, m := range v.Party {
		if m.Selected {
			selected = m.Name
		}
	}
	if v.Sheet == nil || selected != active {
		t.Errorf("sheet %v, selected %q, want the sheet of %q", v.Sheet != nil, selected, active)
	}
}

// TestScriptInputPrompt: a script's InputString publishes the text it
// printed as the line's question (the remaster's pop-up); a typed line ends it.
func TestScriptInputPrompt(t *testing.T) {
	e := explore(t)
	e.DevAction = e.DevScene("password")
	e.Init()
	e.Start(e.Play)
	v := settle(e)
	for range 20 {
		if v.Ask != nil && v.Ask.Kind == engine.AskText {
			break
		}
		v = keyWait(e, cont)
	}
	if v.Ask == nil || v.Ask.Kind != engine.AskText || v.Ask.Prompt != "WHAT DO YOU SAY? (TYPE A SINGLE WORD)" {
		t.Fatalf("ask %+v, want the script's typed line", v.Ask)
	}
	e.Apply(engine.Action{Kind: engine.ActText, Text: "ohlo"}, engine.Action{Kind: engine.ActOK})
	if a := settle(e).Ask; a != nil && a.Kind == engine.AskText {
		t.Fatal("the line did not end")
	}
}
