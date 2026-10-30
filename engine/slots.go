package engine

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Named save slots (extra), F9 at any prompt once a game is under way, and
// the party menu's Load (loading only). They replace the original's lettered
// saves A-J: Save is gone from camp and the party menu, and lettered saves
// in the save directory (an imported game's) become slots "GAME A".."GAME J".
// A slot is a folder <SaveDir>/saves/<name> holding the game's own save A
// (savgamA.dat and its CHRDATA* files), so the format is the original's.
// Loading unwinds whatever is running (a menu, a script, a battle) back to
// adventure, which resumes from the loaded game like the party menu's Load.
// Saving during combat is refused: the battle is not part of the format.

// slotLoaded is the panic that carries a load out to adventure.
type slotLoaded struct{}

type slot struct {
	name string
	at   time.Time
}

// adventure is the party menu, then the game loop; a slot load starts
// the game loop again on the loaded game.
func (e *Engine) adventure() {
	start := func() {
		e.PartyMenu() // ovr17:0161
	}
	party := start
	for {
		if !e.guardedRun(start) {
			// extra: the original exits to DOS when the party is lost
			// (start:013D-0142); the port goes back to the party menu
			e.unwindReset()
			e.resetAdventure() // the next party starts afresh, not where this one died
			e.loadedGame = false
			start = party
			continue
		}
		start = func() {}
		if e.beginFlag == 0 { // a save from the party menu: no area yet, so back there
			start = party
		}
	}
}

func (e *Engine) guardedRun(start func()) (reload bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(toMainMenu); ok {
				return // reload false: the party menu, as when the party is lost
			}
			if _, ok := r.(slotLoaded); !ok {
				panic(r)
			}
			e.unwindReset()
			reload = true
		}
	}()
	e.ext.guarded = true
	defer func() { e.ext.guarded = false }()
	start()
	e.GameLoop() // ovr04:32F8
	return false
}

// unwindReset drops the input state of the screens the load left.
func (e *Engine) unwindReset() {
	x := &e.ext
	x.inCommand, x.padHot, x.onSelect, x.menu, x.list, x.choosing = false, 0, nil, 0, false, false
	x.quick, x.held, x.regions, x.raw = nil, Action{}, nil, false
	x.typing, x.modal, x.floor, x.viewing, x.toItems = false, 0, 0, false, false
	x.campBar, x.selSet, x.paging = false, false, false
	x.fullMap, x.walk, x.back, x.aoe, x.picker, x.enemiesOnly = false, nil, false, 0, false, false
	x.menuFloor, x.panel, e.snap.typing = 0, nil, nil
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

// LoadSlotNamed loads the slot without the screen (tests).
func (e *Engine) LoadSlotNamed(name string) bool {
	return e.loadFrom(name)
}

// seenFile holds a slot's explored squares by map (extra: the original
// keeps none); oldSeenFile is what slots saved before 30 Oct 2026 hold
// them in, by script area (areaKey was script area and GEO block then).
const seenFile, oldSeenFile = "MAPS.GOB", "SEEN.GOB"

// voicedFile holds the voice-over recordings a slot has heard (voice.go).
const voicedFile = "VOICED.GOB"

// loadFrom loads the slot and its explored squares (none for a slot saved
// before they were kept).
func (e *Engine) loadFrom(name string) (ok bool) {
	e.inSlot(name, func() { ok = e.loadSlot() })
	if !ok {
		return false
	}
	e.ext.seen, e.ext.voiced = nil, nil
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
	return true
}

// seenByMap turns explored squares keyed by script area and GEO block
// (oldSeenFile) into ones keyed by data set and GEO block: the scripts of
// one map (names/areas.txt) pool what they saw.
func (e *Engine) seenByMap(old map[int]*[256]uint8) map[int]*[256]uint8 {
	areas, _ := ReadScriptAreas(filepath.Join(e.scriptDir(), "names", "areas.txt"))
	seen := map[int]*[256]uint8{}
	for k, m := range old {
		set := k >> 16 // unknown: the area number stands (nothing pools)
		for _, a := range areas {
			if a.ID == k>>16 {
				set = a.Set
			}
		}
		nm := seen[set<<16|k&0xFFFF]
		if nm == nil {
			nm = &[256]uint8{}
			seen[set<<16|k&0xFFFF] = nm
		}
		for i, v := range m {
			nm[i] = max(nm[i], v)
		}
	}
	return seen
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
	if _, err := os.Stat(filepath.Join(e.slotRoot(), tmp, "SAVGAMA.DAT")); err != nil {
		e.refuse("Save failed")
		return
	}
	os.RemoveAll(filepath.Join(e.slotRoot(), name))
	if err := os.Rename(filepath.Join(e.slotRoot(), tmp), filepath.Join(e.slotRoot(), name)); err != nil {
		e.refuse("Save failed")
		return
	}
	e.say("Game saved successfully") // extra: the note, on the slot page and the page under it
}

// slotRows is how many lines the list shows: rows 3 up to two above the footer.
func slotRows() int { return promptRow - 6 }

// SlotMenu is the F9 screen: "New Slot", then the saves, newest first. The
// cursor starts on the newest save, the one saved last. Enter saves (on New
// Slot under a new name, on a save as an overwrite after a Yes); L loads.
// loadOnly (the party menu's Load) offers no saving, Enter loads there, and
// it returns true after a load instead of unwinding to adventure.
func (e *Engine) SlotMenu(loadOnly bool) bool {
	defer e.modalOn()()
	restore := e.saveScreen()
	defer restore()
	canSave := !loadOnly && e.ext.inGame && len(e.party) > 0 && e.beginFlag != 5
	first := 0 // the row of the first save
	if canSave {
		first = 1
	}
	sel, top := first, 0
	defer e.redrawWith(e.relist)() // the loop draws the list again at the new size
	type word struct {
		label string
		key   byte   // the letter that picks it (shown; 0: not a menu item)
		a     Action // what a click on it does
	}
	const (
		loadIt Choice = iota
		deleteIt
	)
	load, del := Action{Kind: actChoice, N: int(loadIt)}, Action{Kind: actChoice, N: int(deleteIt)}
	footer := []word{{"L=load", 'L', load}, {"D=delete", 'D', del}, {"Esc=back", 0, Action{Kind: ActClose}}}
	switch {
	case canSave:
		footer = append([]word{{"Enter=save", 0, Action{Kind: ActContinue}}}, footer...)
	case loadOnly:
		footer = append([]word{{"Enter=load", 0, Action{Kind: ActContinue}}}, footer[1:]...)
	}
	for {
		list := e.slots()
		rows := len(list) + first
		sel = min(max(sel, 0), max(rows-1, 0))
		top = min(max(top, sel-slotRows()+1), sel)
		e.DrawBoxTitled(1, 1, winRight, promptRow-1, 0, colWhite, "Saved Games")
		if rows == 0 {
			e.DisplayString(3, 3, colLightGreen, "No saved games yet")
		}
		for i := top; i < rows && i < top+slotRows(); i++ {
			col := colLightGreen
			if i == sel {
				col = colWhite
				e.DisplayString(2, 3+i-top, colYellow, ">")
			}
			if i < first {
				e.DisplayString(3, 3+i-top, col, "New Slot")
				continue
			}
			e.DisplayString(3, 3+i-top, col, cut(list[i-first].name, 0x15))
			e.DisplayString(25, 3+i-top, colLightGrey, list[i-first].at.Format("Jan 02 15:04"))
		}
		if !canSave && !loadOnly {
			e.DisplayString(2, promptRow-3, colYellow, "No saving now")
		}
		// extra (nav.go): a click selects a line, a second one is Enter; the
		// keys in the footer are clicked by their words
		mark := e.regionMark()
		e.addRegion(region{x0: 2, y0: 3, x1: 0x25, y1: 3 + slotRows() - 1, click: func(_, cy int) Action {
			i := top + cy - 3
			if i >= rows {
				return Action{}
			}
			if i == sel {
				return Action{Kind: ActContinue}
			}
			sel = i
			return Action{Kind: actRedraw}
		}})
		x, gap := 2, 0x24
		for _, f := range footer {
			gap -= len(f.label)
		}
		gap /= len(footer) - 1 // spread over the row inside the frame
		for _, f := range footer {
			e.DisplayString(x, promptRow-2, colYellow, f.label)
			e.addRegion(region{x0: x, y0: promptRow - 2, x1: x + len(f.label) - 1, y1: promptRow - 2, click: func(int, int) Action { return f.a }})
			x += len(f.label) + gap
		}
		outer := e.snap.menu // extra: the footer's letters as a menu (ActPick)
		e.snap.menu, e.snap.own = &Menu{}, true
		var picks []Action
		for _, f := range footer {
			if f.key != 0 {
				e.snap.menu.Items = append(e.snap.menu.Items, MenuItem{Text: f.label, Key: f.key})
				picks = append(picks, f.a)
			}
		}
		a := e.input()
		if a.Kind == ActPick && a.N >= 0 && a.N < len(picks) {
			a = picks[a.N]
		}
		e.snap.menu, e.snap.own = outer, false
		e.regionCut(mark)
		cur := ""
		if sel >= first && sel < rows {
			cur = list[sel-first].name
		}
		enter := a.Kind == ActContinue
		switch {
		case a.Kind == ActClose || a.Kind == ActSlots:
			return false
		case a.Kind == ActMove && a.N == DirUp:
			sel--
		case a.Kind == ActMove && a.N == DirDown:
			sel++
		case enter && canSave && cur == "": // New Slot
			name := slotName(e.inputLine("Save as: ", 0xA, 0x14)) // extra: the case as typed
			if name == "" {
				continue
			}
			// one slot per name in any case (the folders may not tell case apart)
			if i := slices.IndexFunc(list, func(s slot) bool { return strings.EqualFold(s.name, name) }); i >= 0 {
				if !e.YesNo(colYellow, colLightGreen, colWhite, cut("Overwrite "+list[i].name+"? ", textCols)) {
					continue
				}
				os.RemoveAll(filepath.Join(e.slotRoot(), list[i].name))
			}
			e.saveTo(name)
			sel = first
		case enter && canSave:
			if e.YesNo(colYellow, colLightGreen, colWhite, cut("Overwrite "+cur+"? ", textCols)) {
				e.saveTo(cur)
				sel = first
			}
		case a == del && cur != "":
			if e.YesNo(colYellow, colLightGreen, colWhite, cut("Delete "+cur+"? ", textCols)) {
				os.RemoveAll(filepath.Join(e.slotRoot(), cur))
			}
		case (a == load || enter && loadOnly) && cur != "":
			if e.beginFlag == 5 {
				e.combatViewLeave()
			}
			ok := false
			ok = e.loadFrom(cur)
			if ok && loadOnly {
				return true
			}
			if ok {
				panic(slotLoaded{})
			}
		}
	}
}
