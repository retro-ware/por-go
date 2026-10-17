package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"blobber"
	"blobber/events"
	"blobber/party"
	"blobber/ui"
)

var cdRoot = sync.OnceFunc(func() { os.Chdir("../..") })

// newTestHost starts a headless host on map m at (x, y).
func newTestHost(t *testing.T, m string, x, y int) *console {
	t.Helper()
	cdRoot()
	host := newConsole(party.New(party.NewCharacter("A", "knight", 30, 0), party.NewCharacter("B", "sorcerer", 12, 20)))
	w, err := host.enter(m)
	if err != nil {
		t.Fatal(err)
	}
	blobber.SetMap(w.lvl, x, y)
	return host
}

func use(h *console, x, y int) { pump(h.vm, h.vm.Use(h.m, x, y, events.N)) }

func TestTeleport(t *testing.T) {
	h := newTestHost(t, "demo", 26, 11)
	use(h, 26, 11)
	if x, y, _ := blobber.Pos(); x != 1 || y != 1 || h.m != "cave" {
		t.Fatalf("pos %d,%d map %s", x, y, h.m)
	}
	use(h, 1, 1)
	if x, y, _ := blobber.Pos(); x != 26 || y != 11 || h.m != "demo" {
		t.Fatalf("pos %d,%d map %s", x, y, h.m)
	}
}

// TestIceCave: the shaft from the cave leads down, the chest pays out once, the stairs lead back up.
func TestIceCave(t *testing.T) {
	h := newTestHost(t, "cave", 5, 5)
	use(h, 5, 5)
	if x, y, _ := blobber.Pos(); x != 1 || y != 1 || h.m != "icecave" {
		t.Fatalf("pos %d,%d map %s", x, y, h.m)
	}
	if h.worlds["icecave"].flats != [2]string{"ice_cave", "ice_cave"} {
		t.Fatal("flats", h.worlds["icecave"].flats)
	}
	gold := h.Gold
	use(h, 6, 5)
	use(h, 6, 5)
	if h.Gold != gold+150 {
		t.Fatal("chest gold", h.Gold-gold)
	}
	use(h, 1, 1)
	if x, y, _ := blobber.Pos(); x != 5 || y != 5 || h.m != "cave" {
		t.Fatalf("pos %d,%d map %s", x, y, h.m)
	}
}

func TestLever(t *testing.T) {
	h := newTestHost(t, "cave", 1, 5)
	lever, pulled := h.worlds["cave"].objs["lever"], h.worlds["cave"].objs["lever_pulled"]
	use(h, 1, 5)
	if it := h.FindItem("iron_dagger"); it == nil || it.Bonus != 2 {
		t.Fatal("no iron dagger +2")
	}
	if pulled.X != 1 || pulled.Y != 5 || lever.X != -1 {
		t.Fatal("lever sprites not swapped")
	}
	if !h.Flags["cave:lever"] {
		t.Fatal("flag")
	}
}

func TestFloorItems(t *testing.T) {
	h := newTestHost(t, "cave", 7, 3)
	if got := h.TakeFloor(7, 3); len(got) != 1 || got[0] != "potion" || h.FindItem("potion") == nil {
		t.Fatalf("took %v", got)
	}
	if len(h.TakeFloor(7, 3)) != 0 {
		t.Fatal("item taken twice")
	}
}

// Monsters and decorations belong to their map and survive a round trip.
func TestPopulationPerMap(t *testing.T) {
	h := newTestHost(t, "demo", 26, 11)
	demo := blobber.SwapPopulation(nil) // peek: count demo actors by swapping out and back
	blobber.SwapPopulation(demo)
	h.Teleport("cave", 1, 1)
	cave := blobber.SwapPopulation(nil)
	blobber.SwapPopulation(cave)
	if cave == nil || demo == nil {
		t.Fatal("nil population")
	}
	h.Teleport("demo", 26, 11)
	back := blobber.SwapPopulation(nil)
	blobber.SwapPopulation(back)
	if back.Len() != demo.Len() || cave.Len() == demo.Len() {
		t.Fatalf("demo %d cave %d back %d", demo.Len(), cave.Len(), back.Len())
	}
}

// TestLab: the lab demonstrates every wired opcode, and every station runs headlessly.
func TestLab(t *testing.T) {
	h := newTestHost(t, "lab", 1, 2)
	src, _ := os.ReadFile("maps/lab.evt")
	words := map[string]bool{}
	for _, l := range strings.Split(string(src), "\n") {
		if f := strings.Fields(l); len(f) > 0 {
			words[f[0]] = true
		}
	}
	for _, op := range strings.Fields(`sound music cutscene town alterwall nop exit return remove nothing text npc teleport fall if give take set setchar
		selrndchar whowill damage rnddamage spawn altercell moveobj movewall exchobj giveitem itemtype goto
		gotorandom jumprnd choose confirm call alter`) {
		if !words[op] {
			t.Errorf("lab does not demonstrate %s", op)
		}
	}
	for x := 1; x <= 39; x += 2 {
		for _, y := range []int{1, 3} {
			h.enter("lab")
			blobber.SetMap(h.worlds["lab"].lvl, x, y)
			use(h, x, y)
		}
	}
	if h.m != "demo" && h.m != "lab" {
		t.Fatal(h.m)
	}
}

func TestAlterWallAndTown(t *testing.T) {
	h := newTestHost(t, "lab", 29, 3)
	use(h, 29, 3)
	if blobber.Cell(29, 4) != '.' {
		t.Fatal("wall not opened")
	}
	use(h, 29, 3)
	if blobber.Cell(29, 4) != 'I' {
		t.Fatalf("wall not closed: %q flags %v", blobber.Cell(29, 4), h.Flags)
	}
	blobber.Modal = nil
	use(h, 35, 1)
	shop, ok := blobber.Modal.(*ui.Shop)
	if !ok {
		t.Fatal("no smith")
	}
	h.Gold = 100
	shop.Choose(0) // cheapest item
	if h.Gold >= 100 || len(h.Inventory) == 0 {
		t.Fatal("bought nothing", h.Gold)
	}
	blobber.Modal = nil
	use(h, 33, 1)
	if c, ok := blobber.Modal.(*ui.Cutscene); !ok || len(c.Pages) != 4 {
		t.Fatalf("cutscene %#v", blobber.Modal)
	}
	blobber.Modal = nil
}

// Every npc a script names is in the registry, with a portrait from the pool.
func TestNPCRegistry(t *testing.T) {
	cdRoot()
	f, err := os.Open("maps/npcs.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ui.LoadNPCs(f)
	files, _ := filepath.Glob("maps/*.evt")
	re := regexp.MustCompile(`(?m)^\s*npc\s+(\S+)`)
	for _, fn := range files {
		b, _ := os.ReadFile(fn)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			if n, ok := ui.NPCs[m[1]]; !ok || ui.PortraitID(n.Portrait) == nil {
				t.Errorf("%s: npc %q missing from maps/npcs.txt or its portrait", fn, m[1])
			}
		}
	}
}

// A swing kills a wounded rat with a sure roll; the monster's turn wounds the party; a save round-trips all of it.
func TestCombatAndSave(t *testing.T) {
	h := newTestHost(t, "demo", 2, 1)
	h.installCombat()
	defer func() { blobber.OnMelee, blobber.OnKill, blobber.OnTurn = nil, nil, nil }()
	roll = func(n int) int { return n - 1 } // always the top roll
	defer func() { roll = rand.Intn }()
	kinds["rat"] = kind{"zombie_rat", 6, 2, 3, 5, 10, 0}
	blobber.ClearMonsters()
	h.spawn("rat", 2, 1, 2) // in the party's cell: melee
	h.AddItem(party.NewItem("steel_sword", 0))
	h.Equip(h.Chars[0], h.FindItem("steel_sword"))
	h.swing(h.Chars[0], 0, blobber.Monster{Name: "rat", X: 2, Y: 1, HP: 2})
	if blobber.MonsterAt(2, 1) != "" || h.Chars[0].Exp != 5 {
		t.Fatalf("rat alive=%q exp=%d", blobber.MonsterAt(2, 1), h.Chars[0].Exp)
	}
	h.spawn("rat", 3, 1, -1)
	h.defend(blobber.Monster{Name: "rat", X: 3, Y: 1})
	if h.Chars[0].HP == 30 && h.Chars[1].HP == 12 {
		t.Fatal("nobody hurt")
	}
	h.tick()
	saveFile = filepath.Join(t.TempDir(), "s.sav")
	h.Flags["quest"] = true
	if err := h.saveGame(); err != nil {
		t.Fatal(err)
	}
	hp0, hp1, min := h.Chars[0].HP, h.Chars[1].HP, h.Minutes
	h.Chars[0].HP, h.Chars[1].HP = 1, 1
	blobber.ClearMonsters()
	if err := h.loadGame(); err != nil {
		t.Fatal(err)
	}
	if h.Chars[0].HP != hp0 || h.Chars[1].HP != hp1 || h.Minutes != min || !h.Flags["quest"] || h.m != "demo" {
		t.Fatalf("party not restored: %v", h.Summary())
	}
	if blobber.MonsterAt(3, 1) != "rat" || h.Chars[0].Equipped[party.RightHand] == nil {
		t.Fatal("world or equipment not restored")
	}
	if x, y, _ := blobber.Pos(); x != 2 || y != 1 {
		t.Fatalf("pos %d,%d", x, y)
	}
}

func TestWorldRestAndEnd(t *testing.T) {
	h := newTestHost(t, "demo", 2, 1)
	if h.start.m != "town" || h.start.title != "Blobber Demo" || h.start.dir != 0 || len(h.start.heroes) != 3 || h.start.gold != 500 {
		t.Fatalf("world: %+v", h.start)
	}
	if pt := h.newParty(); len(pt.Chars) != 3 || pt.Chars[0].Name != "Ann" || pt.Chars[0].Class != "knight" || pt.Chars[0].MaxHP < 10 || pt.FindItem("chainmail") == nil {
		t.Fatalf("default party: %+v", pt.Chars[0])
	}
	blobber.ClearMonsters()
	h.Chars[0].HP, h.Food, h.Day = 3, 1, 1
	h.rest()
	if h.Chars[0].HP != 3 {
		t.Fatal("rest without food")
	}
	h.Food = 5
	h.rest()
	if h.Chars[0].HP != 30 || h.Food != 3 || h.Minutes != 16*60 {
		t.Fatalf("rest: hp %d food %d min %d", h.Chars[0].HP, h.Food, h.Minutes)
	}
	h.spawn("rat", 3, 1, -1)
	h.Chars[0].HP = 3
	h.rest()
	if h.Chars[0].HP != 3 {
		t.Fatal("rest next to a monster")
	}
	// End with a cutscene up: the cutscene stays, the last word follows when it closes
	cs := &ui.Cutscene{}
	blobber.Modal = cs
	h.End(true)
	if w, ok := blobber.Modal.(*then); !ok || w.Overlay != cs {
		t.Fatalf("End should wrap the cutscene, got %T", blobber.Modal)
	}
	blobber.Modal = nil
}

// The town loads; F in front of each shop's stairs opens it, just facing them only shows the sign.
func TestTown(t *testing.T) {
	h := newTestHost(t, "town", 12, 10)
	for _, d := range []struct {
		x, y int
		f    events.Facing
	}{{3, 4, events.N}, {9, 4, events.N}, {15, 4, events.N}, {21, 4, events.N}, {3, 7, events.S}, {9, 7, events.S}, {17, 7, events.S}} {
		blobber.Modal = nil
		pump(h.vm, h.vm.Trigger("town", d.x, d.y, d.f))
		if blobber.Modal != nil {
			t.Fatalf("stairs %v: facing them opened %T", d, blobber.Modal)
		}
		pump(h.vm, h.vm.Use("town", d.x, d.y, d.f))
		if _, ok := blobber.Modal.(*ui.Shop); !ok {
			t.Fatalf("stairs %v: no building opened, got %T", d, blobber.Modal)
		}
	}
	blobber.Modal = nil
	use(h, 23, 10)
	if x, y, _ := blobber.Pos(); h.m != "demo" || x != 2 || y != 1 {
		t.Fatalf("stairs: %s %d,%d", h.m, x, y)
	}
}

func TestAutoFight(t *testing.T) {
	h := newTestHost(t, "demo", 2, 1)
	h.installCombat()
	defer func() { blobber.OnMelee, blobber.OnKill, blobber.OnTurn = nil, nil, nil }()
	roll = func(n int) int { return n - 1 } // always the top roll
	defer func() { roll = rand.Intn }()
	kinds["rat"] = kind{"zombie_rat", 6, 2, 3, 5, 10, 0}
	blobber.ClearMonsters()
	h.spawn("rat", 2, 1, -1)
	h.spawn("rat", 2, 1, -1)
	god, autoFight, autoPace = true, true, 0 // god: monster hits do nothing, so the party can't fall first
	defer func() { god, autoPace = false, 0.7 }()
	for range 50 {
		if len(blobber.Melee()) == 0 {
			break
		}
		h.melee()
	}
	if n := len(blobber.Melee()); n != 0 || autoFight {
		t.Fatalf("%d monster(s) left, autoFight=%v", n, autoFight)
	}
}
