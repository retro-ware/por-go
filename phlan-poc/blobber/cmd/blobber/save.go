package main

import (
	"encoding/json"
	"fmt"

	"blobber"
	"blobber/events"
	"blobber/party"
)

// saveGame is everything a game needs to resume: party, position, script patches and each visited map.
// ponytail: one slot, whole maps saved verbatim; slots and diffs when saves get big.
type saveGame struct {
	Party     *party.Party
	Map       string
	X, Y, Dir int
	Patches   []events.Patch
	Worlds    map[string]*savedWorld
}

type savedWorld struct {
	Grid     []string
	Objects  map[string][2]int
	Floor    []floorItem
	Monsters []blobber.Monster
	Seen     [][2]int
}

func (c *console) saveGame() error {
	x, y, dir := blobber.Pos()
	c.Prepare()
	s := saveGame{Party: c.Party, Map: c.m, X: x, Y: y, Dir: dir, Patches: c.vm.Patches(), Worlds: map[string]*savedWorld{}}
	for name, w := range c.worlds {
		pop := w.pop
		if name == c.m {
			pop = blobber.Live()
		}
		sw := &savedWorld{Grid: w.lvl, Objects: map[string][2]int{}, Floor: w.floor, Monsters: pop.Monsters(), Seen: pop.Seen()}
		for n, o := range w.objs {
			sw.Objects[n] = [2]int{o.X, o.Y}
		}
		s.Worlds[name] = sw
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return writeSave(b)
}

// loadSettings starts the game with the saved options (defaults when there are none) and saves
// them whenever they change.
func loadSettings() {
	if b, err := readSettings(); err == nil {
		json.Unmarshal(b, &blobber.Settings) // a broken file keeps the defaults for what it can't fill
	}
	blobber.OnSettings = func(o blobber.Options) {
		if b, err := json.MarshalIndent(o, "", "  "); err == nil {
			if err := writeSettings(b); err != nil {
				blobber.Message("Settings not saved: " + err.Error())
			}
		}
	}
}

// load replaces the running game with the saved one. The party pointer stays, so UI closures keep working.
func (c *console) loadGame() error {
	b, err := readSave()
	if err != nil {
		return err
	}
	var s saveGame
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s.Party == nil || s.Worlds[s.Map] == nil {
		return fmt.Errorf("save: no party or map")
	}
	onHit, onCast := c.OnHit, c.OnCast
	*c.Party = *s.Party
	c.OnHit, c.OnCast = onHit, onCast
	c.Relink()
	c.vm, c.worlds, c.m = events.New(c), map[string]*world{}, ""
	blobber.SwapPopulation(nil)
	for name, sw := range s.Worlds {
		w, err := c.enter(name) // fresh from disk: decorations, scripts
		if err != nil {
			return err
		}
		copy(w.lvl, sw.Grid)
		for n, xy := range sw.Objects {
			if o := w.objs[n]; o != nil {
				o.X, o.Y = xy[0], xy[1]
			}
		}
		w.floor = sw.Floor
		blobber.ClearMonsters()
		for _, m := range sw.Monsters {
			c.spawn(m.Name, m.X, m.Y, m.HP)
		}
		blobber.MarkSeen(sw.Seen)
		w.pop, c.m = blobber.SwapPopulation(nil), "" // park it; enter(s.Map) below brings the right one back
	}
	c.vm.ApplyPatches(s.Patches)
	w, err := c.enter(s.Map)
	if err != nil {
		return err
	}
	blobber.SetMap(w.lvl, s.X, s.Y)
	blobber.Face(s.Dir)
	blobber.Modal = nil
	return nil
}
