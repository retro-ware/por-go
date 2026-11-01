package engine

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Named save slots (extra), F10 at any prompt once a game is under way, and
// the party menu's Load (loading only). They replace the original's lettered
// saves A-J: Save is gone from camp and the party menu, and lettered saves
// in the save directory (an imported game's) become slots "GAME A".."GAME J".
// A slot is a folder <SaveDir>/saves/<name> holding the game's own save A
// (savgamA.dat and its CHRDATA* files), so the format is the original's.
// Loading unwinds whatever is running (a menu, a script, a battle) back to
// adventure, which resumes from the loaded game like the party menu's Load.
// Saving during combat is refused: the battle is not part of the format.

type slot struct {
	name string
	at   time.Time
}

// endGame: the game in play is over (a script quit, the party lost, Back
// to Main Menu). extra: the original exits to DOS when the party is lost
// (start:013D-0142); the port goes back to the party menu, and the next
// party starts afresh, not where this one died.
func (e *Engine) endGame() {
	e.unwindReset()
	e.resetAdventure()
	e.loadedGame = false
}

// unwindReset drops every screen up (without their leaves) and their
// input state: a load or Back to Main Menu; the handler that did it finds
// the stack gone (feed).
func (e *Engine) unwindReset() {
	e.resets++
	x := &e.ext
	x.held, x.pager = Action{}, nil
	x.fullMap, x.walk, x.back, x.aoe, x.enemiesOnly = nil, nil, false, 0, false
	x.panel = nil
	e.screens = nil // the screens pushed are gone with their callers
	e.picker, e.creation, e.slotsUp, x.creating = nil, nil, nil, false
	e.snap.invOn, e.snap.shop, e.snap.loot, e.snap.sheetOn = false, false, false, false
	e.partySel = 0                  // a defer on the way out may have put back the old party's index
	e.loadedGame = e.beginFlag != 0 // DS:49F8, as the party menu's Load
}

func (e *Engine) slotRoot() string { return filepath.Join(e.Cfg.SaveDir, "saves") }

// slots lists the saved games, newest first.
func (e *Engine) slots() []slot {
	ents, _ := os.ReadDir(e.slotRoot())
	var out []slot
	for _, en := range ents {
		st, err := os.Stat(filepath.Join(e.slotRoot(), en.Name(), "SAVGAMA.DAT"))
		if en.IsDir() && err == nil && !strings.HasPrefix(en.Name(), ".") { // .NAME.saving: unfinished
			out = append(out, slot{en.Name(), st.ModTime()})
		}
	}
	slices.SortFunc(out, func(a, b slot) int { return b.at.Compare(a.at) })
	return out
}

// slotName keeps what a folder name may hold.
func slotName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune(" -_!'", r) {
			return r
		}
		return -1
	}, s)
	return strings.TrimSpace(s)
}

// inSlot runs f with the save directory pointed at the slot's folder.
func (e *Engine) inSlot(name string, f func()) {
	dir := e.Cfg.SaveDir
	e.Cfg.SaveDir = filepath.Join(e.slotRoot(), name)
	defer func() { e.Cfg.SaveDir = dir }()
	f()
}

// SlotIndex is the slot's line on the slot screen, or -1 (tests).
func (e *Engine) SlotIndex(name string) int {
	return slices.IndexFunc(e.slots(), func(s slot) bool { return s.name == name })
}

// seenFile holds a slot's explored squares by map (extra: the original
// keeps none); oldSeenFile is what slots saved before 30 Oct 2026 hold
// them in, by script area (areaKey was script area and GEO block then).
const seenFile, oldSeenFile = "MAPS.GOB", "SEEN.GOB"

// voicedFile holds the voice-over recordings a slot has heard (voice.go).
const voicedFile = "VOICED.GOB"

// metFile holds the monster kinds a slot has met (bestiary.go).
const metFile = "BESTIARY.GOB"

// loadFrom loads the slot, its explored squares and its map labels (none
// for a slot saved before they were kept).
func (e *Engine) loadFrom(name string) (ok bool) {
	e.inSlot(name, func() { ok = e.loadSlot() })
	if !ok {
		return false
	}
	e.ext.slot = name
	e.ext.seen, e.ext.voiced, e.ext.met, e.ext.beasts = nil, nil, nil, nil
	if f, err := os.Open(filepath.Join(e.slotRoot(), name, seenFile)); err == nil {
		gob.NewDecoder(f).Decode(&e.ext.seen)
		f.Close()
	} else if f, err := os.Open(filepath.Join(e.slotRoot(), name, oldSeenFile)); err == nil {
		var old map[int]*[256]uint8
		gob.NewDecoder(f).Decode(&old)
		f.Close()
		e.ext.seen = e.seenByMap(old)
	}
	if f, err := os.Open(filepath.Join(e.slotRoot(), name, voicedFile)); err == nil {
		gob.NewDecoder(f).Decode(&e.ext.voiced)
		f.Close()
	}
	if f, err := os.Open(filepath.Join(e.slotRoot(), name, metFile)); err == nil {
		gob.NewDecoder(f).Decode(&e.ext.met)
		f.Close()
	}
	e.readLabels(filepath.Join(e.slotRoot(), name, labelsFile))
	return true
}

// seenByMap turns explored squares keyed by script area and GEO block
// (oldSeenFile) into ones keyed by data set and GEO block: the scripts of
// one map (names/areas.txt) pool what they saw.
func (e *Engine) seenByMap(old map[int]*[256]uint8) map[int]*[256]uint8 {
	areas := e.scriptAreas()
	seen := map[int]*[256]uint8{}
	for k, m := range old {
		k = mapOfArea(areas, k)
		nm := seen[k]
		if nm == nil {
			nm = &[256]uint8{}
			seen[k] = nm
		}
		for i, v := range m {
			nm[i] = max(nm[i], v)
		}
	}
	return seen
}

func (e *Engine) scriptAreas() []ScriptArea {
	areas, _ := ReadScriptAreas(filepath.Join(e.scriptDir(), "names", "areas.txt"))
	return areas
}

// mapOfArea turns an old key (script area and GEO block) into areaKey's
// (data set and GEO block) by the scripts' areas (names/areas.txt).
func mapOfArea(areas []ScriptArea, k int) int {
	set := k >> 16 // unknown: the area number stands (nothing pools)
	for _, a := range areas {
		if a.ID == k>>16 {
			set = a.Set
		}
	}
	return set<<16 | k&0xFFFF
}

func (e *Engine) saveTo(name string) {
	// written beside the old save, which is replaced only once the new one is
	// complete (a fresh folder: no stale CHRDAT files of a bigger party)
	tmp := "." + name + ".saving"
	os.RemoveAll(filepath.Join(e.slotRoot(), tmp))
	e.inSlot(tmp, func() { e.saveSlot() })
	if f, err := os.Create(filepath.Join(e.slotRoot(), tmp, seenFile)); err == nil {
		gob.NewEncoder(f).Encode(e.ext.seen)
		f.Close()
	}
	if f, err := os.Create(filepath.Join(e.slotRoot(), tmp, voicedFile)); err == nil {
		gob.NewEncoder(f).Encode(e.ext.voiced)
		f.Close()
	}
	if f, err := os.Create(filepath.Join(e.slotRoot(), tmp, metFile)); err == nil {
		gob.NewEncoder(f).Encode(e.ext.met)
		f.Close()
	}
	e.writeLabels(filepath.Join(e.slotRoot(), tmp, labelsFile))
	if _, err := os.Stat(filepath.Join(e.slotRoot(), tmp, "SAVGAMA.DAT")); err != nil {
		e.refuse("Save failed")
		return
	}
	os.RemoveAll(filepath.Join(e.slotRoot(), name))
	if err := os.Rename(filepath.Join(e.slotRoot(), tmp), filepath.Join(e.slotRoot(), name)); err != nil {
		e.refuse("Save failed")
		return
	}
	e.ext.slot = name
	e.say("Game saved successfully") // extra: the note, on the slot page and the page under it
}

// slotScreen is the slot screen up.
type slotScreen struct{ loadOnly bool }

func (e *Engine) canSave(ss *slotScreen) bool {
	return !ss.loadOnly && e.ext.inGame && len(e.party) > 0 && e.beginFlag != 5
}

// slotsView is the slot screen for the View.
func (e *Engine) slotsView() *Slots {
	ss := e.slotsUp
	if ss == nil {
		return nil
	}
	v := &Slots{CanSave: e.canSave(ss), LoadOnly: ss.loadOnly, Current: e.ext.slot}
	for _, sl := range e.slots() {
		v.Names, v.Times = append(v.Names, sl.name), append(v.Times, sl.at.Format("Jan 02 15:04"))
	}
	return v
}

// pushSlots puts the F10 screen up (View.Slots): the saves, newest first;
// the list, its cursor and the questions (the name, Overwrite, Delete)
// are the presenter's. loadOnly (the party menu's Load) offers no saving
// and leaves after a load; elsewhere a load unwinds to adventure.
func (e *Engine) pushSlots(loadOnly bool) {
	ss := &slotScreen{loadOnly}
	under := e.slotsUp
	e.slotsUp = ss
	e.push(modeSlots, func(a Action) bool { // the page is the presenter's (View.Slots), over the one under
		return e.applySlots(ss, a)
	}, func() {
		e.slotsUp = under
	})
}

// applySlots applies a decision on the slot screen; done when it is left
// or a load from the party menu is made.
func (e *Engine) applySlots(ss *slotScreen, a Action) (done bool) {
	switch a.Kind {
	case ActClose, ActSlots:
		return true
	case ActSave:
		name := slotName(a.Text)
		if !e.canSave(ss) || name == "" {
			return false
		}
		// one slot per name in any case (the folders may not tell case apart)
		if i := slices.IndexFunc(e.slots(), func(s slot) bool { return strings.EqualFold(s.name, name) }); i >= 0 {
			os.RemoveAll(filepath.Join(e.slotRoot(), e.slots()[i].name))
		}
		e.saveTo(name)
	case ActDelete:
		if e.SlotIndex(a.Text) >= 0 {
			os.RemoveAll(filepath.Join(e.slotRoot(), a.Text))
		}
	case ActLoad:
		if e.SlotIndex(a.Text) < 0 {
			return false
		}
		if e.beginFlag == 5 {
			e.combatViewLeave()
		}
		if !e.loadFrom(a.Text) {
			return false
		}
		if ss.loadOnly {
			return true
		}
		e.unwindReset()       // the screens up are dropped: adventure resumes from the loaded game
		if e.beginFlag == 0 { // a save from the party menu: no area yet, so back there
			e.PartyMenu(e.GameLoop)
		} else {
			e.GameLoop()
		}
	}
	return false
}
