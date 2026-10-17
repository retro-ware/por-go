// Package events implements Xeen-style map event scripts in a text format.
//
// One file per map, one event per (x, y, facing), one opcode per line:
//
//	# comment
//	event 16,8 any            # facing: n e s w any; append "use" to fire only on VM.Use
//	  if mapflag searched goto done
//	  whowill search "Who will search the tree?"
//	  give gold 10
//	  set mapflag searched
//	  exit
//	done:
//	  text "Nothing here."
//
// Keywords (labels may be a name or a 0-based line number):
//
//	nop | exit | return | remove | nothing | selrndchar
//	text [style] "line" ["line2"]        style: main door doorlarge sign signlarge stat bottom large
//	npc ID "text" [confirm L]            ID from maps/npcs.txt; yes jumps to L, no continues
//	sound name | music name | cutscene name | town kind
//	teleport map x y [continue]          exits the event unless continue
//	fall map x y damage
//	if PROP [arg] [OP n] goto L          OP: < <= == != >= >   (no OP: value != 0)
//	give|take|set PROP [arg] [n]         take failing exits the event
//	setchar n                            0 = whole party
//	whowill what "text"
//	damage n kind | rnddamage kind max
//	spawn monster x y
//	alterwall x y side kind | altercell x y kind
//	moveobj name x y | movewall name x y | exchobj a b
//	giveitem kind [enchant [extra]] | itemtype kind
//	goto L | gotorandom L... | jumprnd max n goto L | choose L...
//	confirm "prompt" "word" ["word2"] goto L
//	alter L op                           rewrite line L's opcode (args kept)
//	call x,y facing
//
// PROP is any name the Host understands (hp, gold, gems, award, item, flag,
// mapflag, ...). `flag name` is global, `mapflag name` is scoped to "map:name".
package events

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
)

type Facing int

const (
	N Facing = iota
	E
	S
	W
	Any
)

var facings = map[string]Facing{"n": N, "e": E, "s": S, "w": W, "any": Any}

// Host is everything the scripts can touch; the VM owns no game state.
type Host interface {
	Get(who int, prop, arg string) int
	Give(who int, prop, arg string, n int)
	Take(who int, prop, arg string, n int) bool
	Set(who int, prop, arg string, n int)
	PartySize() int
	PlaySound(name string)
	PlayMusic(name string)
	Cutscene(name string)
	Town(kind string)
	Teleport(m string, x, y int)
	Fall(m string, x, y, damage int)
	Damage(who, n int, kind string)
	Spawn(monster string, x, y int)
	SetWall(x, y int, side, kind string)
	SetCell(x, y int, kind string)
	MoveObject(name string, x, y int)
	ExchangeObjects(a, b string)
	GiveItem(who int, kind string, enchant, extra int)
	End(won bool) // the game is over: victory or defeat
}

type WaitKind int

const (
	Message WaitKind = iota // dismiss: Resume(nil)
	Who                     // Resume(slot int); 0 = escape, exits the event
	Word                    // Resume(string)
	Choose                  // Resume(option int, 1-based)
	YesNo                   // Resume(bool)
)

// Wait is a pending prompt; show it, then call VM.Resume with the answer.
type Wait struct {
	Kind    WaitKind
	Style   string // text style, or whowill "what"
	Name    string // npc id
	Lines   []string
	Options int // choose
}

// Patch is a persisted script mutation (remove / nothing / alter).
type Patch struct {
	Map    string
	X, Y   int
	Facing Facing
	Use    bool
	Line   int    // -1 for whole-event patches
	Op     string // "remove", "nothing", or the new opcode
}

type instr struct {
	op   string
	args []string
}

type key struct {
	x, y int
	f    Facing
	use  bool
}

type event struct {
	m      string
	k      key
	lines  []instr
	labels map[string]int
}

type frame struct {
	ev *event
	pc int
}

type VM struct {
	Host Host
	Rand *rand.Rand
	maps map[string]map[key]*event

	stack   []frame
	who     int
	answer  any
	hasAns  bool
	patches []Patch
}

func New(h Host) *VM {
	return &VM{Host: h, Rand: rand.New(rand.NewSource(1)), maps: map[string]map[key]*event{}, who: 1}
}

// ---------- parsing ----------

// Load parses one map's event file.
func (vm *VM) Load(m string, r io.Reader) error {
	evs := map[key]*event{}
	var cur *event
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		toks, err := tokenize(sc.Text())
		if err != nil {
			return fmt.Errorf("%s:%d: %v", m, n, err)
		}
		if len(toks) == 0 {
			continue
		}
		switch {
		case toks[0] == "event":
			use := len(toks) == 4 && toks[3] == "use"
			if len(toks) != 3 && !use {
				return fmt.Errorf("%s:%d: event X,Y FACING [use]", m, n)
			}
			x, y, ok := xy(toks[1])
			f, fok := facings[toks[2]]
			if !ok || !fok {
				return fmt.Errorf("%s:%d: bad event header", m, n)
			}
			cur = &event{m: m, k: key{x, y, f, use}, labels: map[string]int{}}
			evs[cur.k] = cur
		case cur == nil:
			return fmt.Errorf("%s:%d: line outside event", m, n)
		case len(toks) == 1 && strings.HasSuffix(toks[0], ":"):
			cur.labels[strings.TrimSuffix(toks[0], ":")] = len(cur.lines)
		default:
			cur.lines = append(cur.lines, instr{toks[0], toks[1:]})
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	for _, ev := range evs {
		for i, in := range ev.lines {
			for _, l := range labelArgs(in) {
				if _, err := ev.target(l); err != nil {
					return fmt.Errorf("%s: event %d,%d line %d: %v", m, ev.k.x, ev.k.y, i, err)
				}
			}
		}
	}
	vm.maps[m] = evs
	return nil
}

// labelArgs lists the args of an instruction that must be jump targets.
func labelArgs(in instr) []string {
	switch in.op {
	case "choose", "gotorandom":
		return in.args
	case "goto", "alter":
		if len(in.args) > 0 {
			return in.args[:1]
		}
	default:
		for i, a := range in.args {
			if (a == "goto" || a == "confirm") && i+1 < len(in.args) {
				return in.args[i+1 : i+2]
			}
		}
	}
	return nil
}

func (ev *event) target(l string) (int, error) {
	if n, ok := ev.labels[l]; ok {
		return n, nil
	}
	n, err := strconv.Atoi(l)
	if err != nil || n < 0 || n > len(ev.lines) {
		return 0, fmt.Errorf("unknown label %q", l)
	}
	return n, nil
}

func xy(s string) (int, int, bool) {
	a, b, ok := strings.Cut(s, ",")
	x, e1 := strconv.Atoi(a)
	y, e2 := strconv.Atoi(b)
	return x, y, ok && e1 == nil && e2 == nil
}

// tokenize splits on whitespace, keeps "quoted strings" as one token, strips # comments.
// Keywords are lowercased; quoted strings are not.
func tokenize(s string) ([]string, error) {
	var out []string
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == ' ' || c == '\t':
			i++
		case c == '#':
			return out, nil
		case c == '"':
			j := strings.IndexByte(s[i+1:], '"')
			if j < 0 {
				return nil, fmt.Errorf("unterminated string")
			}
			out = append(out, s[i+1:i+1+j])
			i += j + 2
		default:
			j := strings.IndexAny(s[i:], " \t#\"")
			if j < 0 {
				j = len(s) - i
			}
			out = append(out, strings.ToLower(s[i:i+j]))
			i += j
		}
	}
	return out, nil
}

// Loaded reports whether a map's events have been loaded.
func (vm *VM) Loaded(m string) bool { _, ok := vm.maps[m]; return ok }

// Marks reports which tiles of a map have events: bit 1 = automatic, bit 2 = use. For minimaps.
func (vm *VM) Marks(m string) map[[2]int]int {
	out := map[[2]int]int{}
	for k := range vm.maps[m] {
		bit := 1
		if k.use {
			bit = 2
		}
		out[[2]int{k.x, k.y}] |= bit
	}
	return out
}

// ---------- execution ----------

// Trigger runs the automatic event at (x,y) for the given facing: an exact-facing
// event wins over an "any" one. Returns a Wait if the script needs input, nil when
// done or when there is no event.
func (vm *VM) Trigger(m string, x, y int, f Facing) *Wait { return vm.start(m, x, y, f, false) }

// Use runs the "use" event at (x,y), i.e. the player searched or interacted with the tile.
func (vm *VM) Use(m string, x, y int, f Facing) *Wait { return vm.start(m, x, y, f, true) }

func (vm *VM) start(m string, x, y int, f Facing, use bool) *Wait {
	ev := vm.maps[m][key{x, y, f, use}]
	if ev == nil {
		ev = vm.maps[m][key{x, y, Any, use}]
	}
	if ev == nil {
		return nil
	}
	vm.stack = []frame{{ev, 0}}
	vm.who, vm.hasAns = 1, false
	return vm.run()
}

// Resume continues a script after a Wait with the player's answer.
func (vm *VM) Resume(answer any) *Wait {
	vm.answer, vm.hasAns = answer, true
	return vm.run()
}

func (vm *VM) exit() { vm.stack = nil }

func (vm *VM) run() *Wait {
	for len(vm.stack) > 0 {
		fr := &vm.stack[len(vm.stack)-1]
		if fr.pc >= len(fr.ev.lines) {
			vm.stack = vm.stack[:len(vm.stack)-1] // falling off the end acts like return
			continue
		}
		in := fr.ev.lines[fr.pc]
		fr.pc++
		if w := vm.exec(fr, in); w != nil {
			fr.pc-- // re-run the waiting instruction with the answer
			return w
		}
		vm.hasAns = false
	}
	return nil
}

// take consumes a pending answer, or returns the Wait to ask for one.
func (vm *VM) ask(w *Wait) (any, *Wait) {
	if vm.hasAns {
		return vm.answer, nil
	}
	return nil, w
}

func (vm *VM) exec(fr *frame, in instr) *Wait {
	ev, a, h := fr.ev, in.args, vm.Host
	jump := func(l string) {
		n, _ := ev.target(l)
		fr.pc = n
	}
	num := func(i int) int { n, _ := strconv.Atoi(a[i]); return n }
	switch in.op {
	case "nop":
	case "exit":
		vm.exit()
	case "return":
		vm.stack = vm.stack[:len(vm.stack)-1]
	case "remove":
		vm.patch(Patch{ev.m, ev.k.x, ev.k.y, ev.k.f, ev.k.use, -1, "remove"})
		vm.exit()
	case "nothing":
		vm.patch(Patch{ev.m, ev.k.x, ev.k.y, ev.k.f, ev.k.use, -1, "nothing"})
		vm.exit()
	case "alter":
		n, _ := ev.target(a[0])
		vm.patch(Patch{ev.m, ev.k.x, ev.k.y, ev.k.f, ev.k.use, n, a[1]})
	case "text":
		style, lines := "main", a
		if len(a) > 1 && isStyle(a[0]) {
			style, lines = a[0], a[1:]
		}
		_, w := vm.ask(&Wait{Kind: Message, Style: style, Lines: lines})
		return w
	case "npc":
		if len(a) >= 4 && a[2] == "confirm" {
			ans, w := vm.ask(&Wait{Kind: YesNo, Name: a[0], Lines: a[1:2]})
			if w != nil {
				return w
			}
			if ans.(bool) {
				jump(a[3])
			}
		} else {
			_, w := vm.ask(&Wait{Kind: Message, Style: "npc", Name: a[0], Lines: a[1:2]})
			return w
		}
	case "sound":
		h.PlaySound(a[0])
	case "music":
		h.PlayMusic(a[0])
	case "cutscene":
		h.Cutscene(a[0])
	case "town":
		h.Town(a[0])
	case "teleport":
		h.Teleport(a[0], num(1), num(2))
		if len(a) < 4 || a[3] != "continue" {
			vm.exit()
		}
	case "fall":
		h.Fall(a[0], num(1), num(2), num(3))
		vm.exit()
	case "if":
		prop, arg, op, n, l := cond(a)
		prop, arg = scope(ev.m, prop, arg)
		if cmp(h.Get(vm.who, prop, arg), op, n) {
			jump(l)
		}
	case "give", "take", "set":
		prop, arg, n := propArgs(a)
		prop, arg = scope(ev.m, prop, arg)
		switch in.op {
		case "give":
			h.Give(vm.who, prop, arg, n)
		case "set":
			h.Set(vm.who, prop, arg, n)
		case "take":
			if !h.Take(vm.who, prop, arg, n) {
				vm.exit()
			}
		}
	case "setchar":
		vm.who = num(0)
	case "selrndchar":
		vm.who = 1 + vm.Rand.Intn(max(1, h.PartySize()))
	case "whowill":
		if h.PartySize() <= 1 {
			vm.who = 1
			break
		}
		ans, w := vm.ask(&Wait{Kind: Who, Style: a[0], Lines: a[1:]})
		if w != nil {
			return w
		}
		if vm.who = ans.(int); vm.who == 0 {
			vm.exit()
		}
	case "damage":
		h.Damage(vm.who, num(0), a[1])
	case "rnddamage":
		h.Damage(vm.who, 1+vm.Rand.Intn(max(1, num(1))), a[0])
	case "spawn":
		h.Spawn(a[0], num(1), num(2))
	case "alterwall":
		h.SetWall(num(0), num(1), a[2], a[3])
	case "altercell":
		h.SetCell(num(0), num(1), a[2])
	case "moveobj", "movewall":
		h.MoveObject(a[0], num(1), num(2))
	case "exchobj":
		h.ExchangeObjects(a[0], a[1])
	case "giveitem":
		en, ex := 0, 0
		if len(a) > 1 {
			en = num(1)
		}
		if len(a) > 2 {
			ex = num(2)
		}
		h.GiveItem(vm.who, a[0], en, ex)
	case "itemtype":
		h.GiveItem(vm.who, a[0], 0, 0)
	case "goto":
		jump(a[0])
	case "gotorandom":
		jump(a[vm.Rand.Intn(len(a))])
	case "jumprnd":
		if 1+vm.Rand.Intn(max(1, num(0))) == num(1) {
			jump(a[3])
		}
	case "choose":
		ans, w := vm.ask(&Wait{Kind: Choose, Options: len(a) + 1})
		if w != nil {
			return w
		}
		if i := ans.(int) - 1; i >= 0 && i < len(a) {
			jump(a[i])
		}
	case "confirm":
		ans, w := vm.ask(&Wait{Kind: Word, Lines: a[:1]})
		if w != nil {
			return w
		}
		got := strings.ToLower(ans.(string))
		for _, word := range a[1 : len(a)-2] {
			if got == strings.ToLower(word) {
				jump(a[len(a)-1])
			}
		}
	case "win", "gameover":
		h.End(in.op == "win")
		vm.exit()
	case "call":
		x, y, _ := xy(a[0])
		if t := vm.maps[ev.m][key{x, y, facings[a[1]], false}]; t != nil {
			vm.stack = append(vm.stack, frame{t, 0})
		}
	default:
		panic(fmt.Sprintf("events: %s: unknown opcode %q", ev.m, in.op))
	}
	return nil
}

func isStyle(s string) bool {
	switch s {
	case "main", "door", "doorlarge", "sign", "signlarge", "shopsign", "stat", "bottom", "large":
		return true
	}
	return false
}

// cond parses: PROP [arg] [OP n] goto L
func cond(a []string) (prop, arg, op string, n int, label string) {
	g := len(a) - 2 // index of "goto"
	label = a[g+1]
	c := a[:g]
	prop, arg, op, n = c[0], "", "!=", 0
	if len(c) >= 3 && isOp(c[len(c)-2]) {
		op = c[len(c)-2]
		n, _ = strconv.Atoi(c[len(c)-1])
		c = c[:len(c)-2]
	}
	if len(c) > 1 {
		arg = c[1]
	}
	return prop, arg, op, n, label
}

// propArgs parses: PROP [arg] [n]
func propArgs(a []string) (prop, arg string, n int) {
	prop, n = a[0], 1
	for _, s := range a[1:] {
		if v, err := strconv.Atoi(s); err == nil {
			n = v
		} else {
			arg = s
		}
	}
	return prop, arg, n
}

// scope keeps map flags distinct per map: "mapflag x" -> prop "flag", arg "<map>:x".
func scope(m, prop, arg string) (string, string) {
	if prop == "mapflag" {
		return "flag", m + ":" + arg
	}
	return prop, arg
}

func isOp(s string) bool {
	switch s {
	case "<", "<=", "==", "!=", ">=", ">":
		return true
	}
	return false
}

func cmp(v int, op string, n int) bool {
	switch op {
	case "<":
		return v < n
	case "<=":
		return v <= n
	case "==":
		return v == n
	case "!=":
		return v != n
	case ">=":
		return v >= n
	}
	return v > n
}

// ---------- patches ----------

func (vm *VM) patch(p Patch) {
	vm.patches = append(vm.patches, p)
	vm.apply(p)
}

func (vm *VM) apply(p Patch) {
	ev := vm.maps[p.Map][key{p.X, p.Y, p.Facing, p.Use}]
	if ev == nil {
		return
	}
	switch {
	case p.Op == "remove":
		delete(vm.maps[p.Map], ev.k)
	case p.Op == "nothing":
		ev.lines = []instr{{"text", []string{"Nothing here."}}, {"exit", nil}}
		ev.labels = map[string]int{}
	case p.Line >= 0 && p.Line < len(ev.lines):
		ev.lines[p.Line].op = p.Op
	}
}

// Patches returns every script mutation so far, for the host to save.
func (vm *VM) Patches() []Patch { return vm.patches }

// ApplyPatches replays saved mutations after Load.
func (vm *VM) ApplyPatches(ps []Patch) {
	for _, p := range ps {
		vm.patch(p)
	}
}
