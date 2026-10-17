package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"blobber"
	"blobber/events"
	"blobber/maps"
	"blobber/party"
	"blobber/sfx"
	"blobber/sprites"
	"blobber/ui"
)

// console is the demo Host: the party supplies stats, worlds hold per-map state,
// and the few opcodes without engine support (sound, music, cutscene, town, alterwall) print.
type console struct {
	*party.Party
	vm     *events.VM
	m      string // current map name
	worlds map[string]*world
	start  start // from maps/world.txt
}

// world is one loaded map: its grid, named props, floor items and (while not current) population.
type world struct {
	lvl   []string
	objs  map[string]*blobber.Object
	floor []floorItem
	pop   *blobber.Population
	flats [2]string // painted floor and ceiling names ("" = default)
	// phlan-poc: this map's thin walls and water, swapped into blobber.Faces/Edges/Water on enter
	faces map[[3]int][]blobber.FacePart
	edges map[[3]int]bool
	water map[[2]int]bool
	nomap map[[3]int]bool
}

// floorItem is loot hidden on a tile; as in World of Xeen items are never drawn in the world, F finds them.
type floorItem struct {
	It   *party.Item
	X, Y int
}

func newConsole(p *party.Party) *console {
	for name, load := range map[string]func(io.Reader) error{"monsters.txt": loadMonsters, "classes.txt": party.LoadClasses, "races.txt": party.LoadRaces, "spells.txt": party.LoadSpells} {
		if f, err := maps.Open(name); err == nil {
			if err := load(f); err != nil {
				fmt.Println(" ", err)
			}
			f.Close()
		}
	}
	c := &console{Party: p, worlds: map[string]*world{}, start: loadWorld()}
	c.vm = events.New(c)
	return c
}

// enter loads a map on first visit and makes it current: population, objects, events.
// It does not move the party; Teleport and main do that.
func (c *console) enter(name string) (*world, error) {
	w := c.worlds[name]
	if w == nil {
		var err error
		if w, err = c.load(name); err != nil {
			return nil, err
		}
	} else {
		prev := blobber.SwapPopulation(w.pop)
		if old := c.worlds[c.m]; old != nil && c.m != name {
			old.pop = prev
		}
	}
	c.m = name
	blobber.Faces, blobber.Edges, blobber.Water, blobber.NoMap = w.faces, w.edges, w.water, w.nomap
	c.refresh()
	return w, nil
}

// load reads maps/<name>.map (grid plus object/item/monster/deco lines) and maps/<name>.evt.
func (c *console) load(name string) (*world, error) {
	mf, err := maps.Open(name + ".map")
	if err != nil {
		return nil, err
	}
	defer mf.Close()
	lvl, extra, err := blobber.LoadMap(mf)
	if err != nil {
		return nil, err
	}
	if !c.vm.Loaded(name) {
		ef, err := maps.Open(name + ".evt")
		if err == nil {
			defer ef.Close()
			err = c.vm.Load(name, ef)
		}
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	// the new map starts with an empty population; park the current one with its world
	prev := blobber.SwapPopulation(nil)
	if old := c.worlds[c.m]; old != nil {
		old.pop = prev
	}
	w := &world{lvl: lvl, objs: map[string]*blobber.Object{},
		faces: map[[3]int][]blobber.FacePart{}, edges: map[[3]int]bool{}, water: map[[2]int]bool{}, nomap: map[[3]int]bool{}}
	c.worlds[name] = w
	for _, l := range extra {
		if err := c.place(w, l); err != nil {
			return nil, fmt.Errorf("maps/%s.map: %q: %v", name, l, err)
		}
	}
	return w, nil
}

// place interprets one extra map line:
//
//	object NAME SPRITE X Y      named prop, target of moveobj / movewall / exchobj
//	item BASE X Y [BONUS]       item lying on the floor, taken with F
//	monster SPRITE X Y          wandering monster
//	deco grass|lantern|firefly X Y
//	face X Y SIDE CHAR          phlan-poc: the face of floor cell X, Y towards SIDE (0 N .. 3 W) in material CHAR
//	floor NAME / ceiling NAME   painted assets/walls/floor_NAME.png / ceiling_NAME.png
func (c *console) place(w *world, line string) error {
	f := strings.Fields(line)
	num := func(i int) int {
		if i >= len(f) {
			return 0
		}
		n, _ := strconv.Atoi(f[i])
		return n
	}
	need := map[string]int{"object": 5, "item": 4, "monster": 4, "deco": 4, "floor": 2, "ceiling": 2, "face": 5, "edge": 4, "water": 3, "nomap": 4}[f[0]]
	if need == 0 || len(f) < need {
		return fmt.Errorf("want %d fields", need)
	}
	switch f[0] {
	case "object":
		img := sprites.Get(f[2])
		if img == nil {
			return fmt.Errorf("no sprite %q", f[2])
		}
		w.objs[f[1]] = &blobber.Object{X: num(3), Y: num(4), Img: img, Size: 0.6}
	case "item":
		w.drop(party.NewItem(f[1], num(4)), num(2), num(3))
	case "monster":
		return c.spawn(f[1], num(2), num(3), -1)
	case "water": // phlan-poc: water X Y, see blobber.Water
		w.water[[2]int{num(1), num(2)}] = true
	case "nomap": // phlan-poc: nomap X Y SIDE, a face left off the minimap, see blobber.NoMap
		w.nomap[[3]int{num(1), num(2), num(3)}] = true
	case "edge": // phlan-poc: edge X Y SIDE, a thin wall, see blobber.Edges
		w.edges[[3]int{num(1), num(2), num(3)}] = true
	case "face": // phlan-poc: face X Y SIDE CHAR [T0 T1 U0 U1], see blobber.Faces
		p := blobber.FacePart{Mat: f[4][0], T1: 1, U1: 1}
		if len(f[4]) > 1 { // a texture name instead of a map character
			p.Mat, p.Tex = 0, f[4]
		}
		if len(f) >= 9 {
			fmt.Sscan(strings.Join(f[5:9], " "), &p.T0, &p.T1, &p.U0, &p.U1)
		}
		k := [3]int{num(1), num(2), num(3)}
		w.faces[k] = append(w.faces[k], p)
	case "floor":
		w.flats[0] = f[1]
	case "ceiling":
		w.flats[1] = f[1]
	case "deco":
		k, ok := map[string]int{"grass": blobber.Grass, "lantern": blobber.Lantern, "firefly": blobber.Firefly}[f[1]]
		if !ok {
			return fmt.Errorf("unknown deco %q", f[1])
		}
		blobber.Spawn(k, num(2), num(3))
	}
	return nil
}

func (w *world) drop(it *party.Item, x, y int) {
	w.floor = append(w.floor, floorItem{it, x, y})
}

// refresh publishes the current world's props to the engine.
func (c *console) refresh() {
	w := c.worlds[c.m]
	blobber.SetFlats(w.flats[0], w.flats[1])
	blobber.Objects = blobber.Objects[:0]
	for _, o := range w.objs {
		blobber.Objects = append(blobber.Objects, o)
	}
}

// TakeFloor picks up every item lying at (x, y) and reports what was taken.
func (c *console) TakeFloor(x, y int) []string {
	w := c.worlds[c.m]
	var names []string
	kept := w.floor[:0]
	for _, f := range w.floor {
		if f.X == x && f.Y == y {
			c.AddItem(f.It)
			names = append(names, f.It.Name())
		} else {
			kept = append(kept, f)
		}
	}
	w.floor = kept
	return names
}

// ---------- events.Host ----------

func (c *console) PlaySound(s string) { sfx.Play(s) }
func (c *console) PlayMusic(s string) { sfx.Music(s) }

// Cutscene shows maps/cutscenes/<name>.txt full screen. The script carries on underneath.
func (c *console) Cutscene(s string) {
	f, err := maps.Open("cutscenes/" + s + ".txt")
	if err != nil {
		fmt.Println("  cutscene:", err)
		return
	}
	defer f.Close()
	blobber.Modal = &ui.Cutscene{Pages: ui.ParseCutscene(f)}
}

// Town opens a shop window (smith, guild, temple, tavern, trainer, bank).
func (c *console) Town(s string) {
	shop := ui.NewTown(s, c.Party)
	shop.Sound = sfx.Play
	blobber.Modal = shop
}

func (c *console) Teleport(m string, x, y int) {
	w, err := c.enter(m)
	if err != nil {
		fmt.Println("  teleport failed:", err)
		return
	}
	blobber.SetMap(w.lvl, x, y)
}

// teleportMenu lists every level (up to 9) and takes the party to the one picked: to the game's
// start cell on the start map, elsewhere to the first open cell. Esc cancels.
func (c *console) teleportMenu() {
	names := maps.List()
	names = names[:min(9, len(names))]
	lines := []string{"TELEPORT TO LEVEL", ""}
	for i, n := range names {
		lines = append(lines, fmt.Sprintf("  %d  %s", i+1, n))
	}
	blobber.ShowPrompt(append(lines, "", "Esc cancels"), false, func(a string) {
		if a == "" { // Esc
			return
		}
		i := int(a[0] - '1')
		if i < 0 || i >= len(names) {
			return
		}
		w, err := c.enter(names[i])
		if err != nil {
			blobber.Message(err.Error())
			return
		}
		x, y := c.start.x, c.start.y
		if names[i] != c.start.m {
			x, y = openCell(w.lvl)
		}
		blobber.SetMap(w.lvl, x, y)
		blobber.Message("Teleported to " + names[i] + ".")
	})
}

// openCell finds the first floor cell of a level, row by row.
func openCell(lvl []string) (int, int) {
	for y, row := range lvl {
		if x := strings.IndexByte(row, '.'); x >= 0 {
			return x, y
		}
	}
	return 1, 1
}

func (c *console) Fall(m string, x, y, d int) {
	c.Teleport(m, x, y)
	c.Party.Damage(0, d, "physical")
}

func (c *console) Spawn(mon string, x, y int) { c.spawn(mon, x, y, -1) }

// SetWall changes the wall on one side of cell x,y. Walls are whole cells here, so this sets the
// neighbouring cell on that side: KIND is a material letter (# B M W I, any case), a material name
// (stone brick moss wood iron, "wall" = stone) or "open" / "." (floor).
func (c *console) SetWall(x, y int, side, kind string) {
	d := strings.Index("nesw", side)
	if d < 0 || kind == "" {
		fmt.Println("  alterwall: bad side or kind", side, kind)
		return
	}
	ch := strings.ToUpper(kind)[0]
	if named, ok := map[string]byte{"wall": '#', "stone": '#', "brick": 'B', "moss": 'M', "wood": 'W', "iron": 'I', "open": '.'}[kind]; ok {
		ch = named
	}
	off := [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}[d]
	blobber.SetCell(x+off[0], y+off[1], ch)
}

func (c *console) SetCell(x, y int, kind string) {
	if kind != "" {
		blobber.SetCell(x, y, kind[0])
	}
}

func (c *console) MoveObject(n string, x, y int) {
	if o := c.worlds[c.m].objs[n]; o != nil {
		o.X, o.Y = x, y
	} else {
		fmt.Println("  moveobj: no object", n)
	}
}

func (c *console) ExchangeObjects(a, b string) {
	oa, ob := c.worlds[c.m].objs[a], c.worlds[c.m].objs[b]
	if oa == nil || ob == nil {
		fmt.Println("  exchobj: missing", a, b)
		return
	}
	oa.X, oa.Y, ob.X, ob.Y = ob.X, ob.Y, oa.X, oa.Y
}

func (c *console) GiveItem(who int, k string, en, ex int) {
	c.AddItem(party.NewItem(k, en))
}

// ---------- driving scripts ----------

// pump drives a script: messages become world text in front of the party, wherever it is now
// (a teleport ... continue moves it), and the script continues; other prompts open a modal Prompt
// and resume when the player answers.
func pump(vm *events.VM, w *events.Wait) {
	for ; w != nil; w = vm.Resume(answer(w)) {
		switch w.Kind {
		case events.Message:
			if w.Name != "" && blobber.OnPromptAnswer != nil { // NPC speech waits for its window
				ask(vm, w)
				return
			}
			x, y, dir := blobber.Pos()
			if w.Style == "shopsign" { // on the wall ahead, above a shop's stairs
				blobber.ShowSignAbove(x, y, dir, w.Lines)
			} else {
				blobber.ShowSign(x, y, dir, w.Lines)
			}
		default:
			if blobber.OnPromptAnswer == nil { // headless (tests): auto-answer
				continue
			}
			ask(vm, w)
			return
		}
	}
}

// ask shows w as a Prompt; the answer resumes the script via pump.
func ask(vm *events.VM, w *events.Wait) {
	resume := func(a any) { pump(vm, vm.Resume(a)) }
	switch w.Kind {
	case events.Who:
		lines := append([]string{"Who will " + w.Style + "? " + strings.Join(w.Lines, " ")}, blobber.OnPromptAnswer()...)
		blobber.ShowPrompt(lines, false, func(a string) { resume(digit(a)) })
	case events.Choose:
		lines := []string{"Choose:"}
		for i := 1; i <= w.Options; i++ {
			lines = append(lines, fmt.Sprintf("  %d  %d", i, i))
		}
		blobber.ShowPrompt(lines, false, func(a string) { resume(max(1, digit(a))) })
	case events.Message:
		blobber.Modal = &ui.Talk{Who: w.Name, Text: strings.Join(w.Lines, " "), Done: func(bool) { resume(nil) }}
	case events.YesNo:
		if w.Name != "" {
			blobber.Modal = &ui.Talk{Who: w.Name, Text: strings.Join(w.Lines, " "), Ask: true, Done: func(y bool) { resume(y) }}
			return
		}
		blobber.ShowPrompt(append([]string{w.Name + ":"}, append(w.Lines, "(y/n)")...), false, func(a string) { resume(a == "y") })
	case events.Word:
		blobber.ShowPrompt(w.Lines, true, func(a string) { resume(a) })
	}
}

func digit(a string) int {
	n, _ := strconv.Atoi(a)
	return n
}

// answer is the headless fallback used when no prompt UI is wired (tests).
func answer(w *events.Wait) any {
	switch w.Kind {
	case events.Who, events.Choose:
		return 1
	case events.Word:
		return ""
	case events.YesNo:
		return true
	}
	return nil
}
