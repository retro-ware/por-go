package events

import (
	"fmt"
	"strings"
	"testing"
)

// fake records calls and holds a tiny party: one character, gold, flags.
type fake struct {
	gold  int
	flags map[string]bool
	log   []string
}

func (f *fake) Get(who int, prop, arg string) int {
	switch prop {
	case "gold":
		return f.gold
	case "flag":
		if f.flags[arg] {
			return 1
		}
	}
	return 0
}
func (f *fake) Give(who int, prop, arg string, n int) {
	if prop == "gold" {
		f.gold += n
	}
}
func (f *fake) Take(who int, prop, arg string, n int) bool {
	if prop == "gold" && f.gold >= n {
		f.gold -= n
		return true
	}
	return false
}
func (f *fake) Set(who int, prop, arg string, n int) {
	if prop == "flag" {
		f.flags[arg] = n != 0
	}
}
func (f *fake) PartySize() int                         { return 1 }
func (f *fake) PlaySound(s string)                     { f.log = append(f.log, "sound "+s) }
func (f *fake) PlayMusic(s string)                     { f.log = append(f.log, "music "+s) }
func (f *fake) Cutscene(s string)                      { f.log = append(f.log, "cutscene "+s) }
func (f *fake) Town(s string)                          { f.log = append(f.log, "town "+s) }
func (f *fake) Teleport(m string, x, y int)            { f.log = append(f.log, "teleport "+m) }
func (f *fake) Fall(m string, x, y, d int)             { f.log = append(f.log, "fall "+m) }
func (f *fake) Damage(who, n int, k string)            { f.log = append(f.log, "damage "+k) }
func (f *fake) Spawn(mon string, x, y int)             { f.log = append(f.log, "spawn "+mon) }
func (f *fake) SetWall(x, y int, side, kind string)    { f.log = append(f.log, "wall "+kind) }
func (f *fake) SetCell(x, y int, kind string)          { f.log = append(f.log, "cell "+kind) }
func (f *fake) MoveObject(n string, x, y int)          { f.log = append(f.log, "move "+n) }
func (f *fake) ExchangeObjects(a, b string)            { f.log = append(f.log, "exch "+a+b) }
func (f *fake) GiveItem(who int, k string, en, ex int) { f.log = append(f.log, "item "+k) }
func (f *fake) End(won bool)                           { f.log = append(f.log, fmt.Sprint("end ", won)) }

const src = `
# a searchable tree: only when the player searches
event 16,8 any use
  if mapflag searched goto done
  whowill search "Who will search the tree?"
  give gold 10
  set mapflag searched
  sound rustle
  exit
done:
  text "Nothing here."

event 1,1 n
  confirm "Password?" "open" "sesame" goto ok
  text sign "Wrong."
  exit
ok:
  call 2,2 any
  choose a b
  text "third"
  exit
a:
  text "first"
  exit
b:
  text "second"
  remove

event 2,2 any
  music theme
  return
`

func TestScript(t *testing.T) {
	h := &fake{flags: map[string]bool{}}
	vm := New(h)
	if err := vm.Load("demo", strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}
	// first search: party of one skips whowill, gets gold, flag set
	if vm.Trigger("demo", 16, 8, E) != nil || h.gold != 0 {
		t.Fatal("use events must not fire on arrival")
	}
	if w := vm.Use("demo", 16, 8, E); w != nil {
		t.Fatalf("unexpected wait %+v", w)
	}
	if h.gold != 10 || !h.flags["demo:searched"] || h.log[0] != "sound rustle" {
		t.Fatalf("gold=%d flags=%v log=%v", h.gold, h.flags, h.log)
	}
	// second search: flag set -> message
	w := vm.Use("demo", 16, 8, Any)
	if w == nil || w.Kind != Message || w.Lines[0] != "Nothing here." {
		t.Fatalf("got %+v", w)
	}
	if vm.Resume(nil) != nil || h.gold != 10 {
		t.Fatal("should finish without giving gold")
	}
	// facing-specific event: wrong facing does nothing
	if vm.Trigger("demo", 1, 1, S) != nil {
		t.Fatal("south should not trigger a north event")
	}
	w = vm.Trigger("demo", 1, 1, N)
	if w.Kind != Word {
		t.Fatalf("want word prompt, got %+v", w)
	}
	w = vm.Resume("SESAME") // case-insensitive, second word -> ok: call, then choose
	if h.log[len(h.log)-1] != "music theme" || w.Kind != Choose || w.Options != 3 {
		t.Fatalf("log=%v wait=%+v", h.log, w)
	}
	w = vm.Resume(2)
	if w.Lines[0] != "second" {
		t.Fatalf("got %+v", w)
	}
	vm.Resume(nil) // remove
	if vm.Trigger("demo", 1, 1, N) != nil {
		t.Fatal("event should be removed")
	}
	// patches survive a reload
	ps := vm.Patches()
	vm2 := New(h)
	vm2.Load("demo", strings.NewReader(src))
	vm2.ApplyPatches(ps)
	if vm2.Trigger("demo", 1, 1, N) != nil {
		t.Fatal("patch not applied")
	}
	// bad label fails at load
	if err := New(h).Load("x", strings.NewReader("event 0,0 any\n goto nowhere\n")); err == nil {
		t.Fatal("want label error")
	}
}
