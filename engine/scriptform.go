package engine

import (
	"fmt"
	"slices"
	"strings"
)

// The script text (data/assets/scripts/area<N>.txt, README.md there) is
// written from a Script by WriteScript and read back by ParseScript; the
// two agree on every form so that a script round-trips to the same byte
// code. This file holds what both use: the opcodes, the comparison flags
// a command leaves and the names of the numbers in a command.

// The opcodes the writer and parser handle by name.
const (
	opExit        byte = 0x00
	opGoto        byte = 0x01
	opGosub       byte = 0x02
	opCompare     byte = 0x03
	opAdd         byte = 0x04
	opSub         byte = 0x05
	opDiv         byte = 0x06
	opMul         byte = 0x07
	opRandom      byte = 0x08
	opSave        byte = 0x09
	opPicture     byte = 0x0E
	opPrint       byte = 0x11
	opPrintClear  byte = 0x12
	opReturn      byte = 0x13
	opCompareAnd  byte = 0x14
	opVMenu       byte = 0x15
	opIfEq        byte = 0x16
	opIfNe        byte = 0x17
	opIfLt        byte = 0x18
	opIfGt        byte = 0x19
	opIfLe        byte = 0x1A
	opIfGe        byte = 0x1B
	opNewECL      byte = 0x20
	opLoadMap     byte = 0x21
	opCombat      byte = 0x24
	opOnGoto      byte = 0x25
	opOnGosub     byte = 0x26
	opTreasure    byte = 0x27
	opGetTable    byte = 0x2A
	opHMenu       byte = 0x2B
	opCall        byte = 0x2D
	opAnd         byte = 0x2F
	opOr          byte = 0x30
	opFindItem    byte = 0x32
	opSaveTable   byte = 0x35
	opNewLine     byte = 0x33
	opInputNumber byte = 0x0F
	opInputString byte = 0x10
	opPartyStr    byte = 0x1D
	opParlay      byte = 0x2C
)

func isIf(op byte) bool { return op >= opIfEq && op <= opIfGe }

// ifOps are the tests' operators, by If opcode.
var ifOps = map[byte]string{opIfEq: "==", opIfNe: "!=", opIfLt: "<", opIfGt: ">", opIfLe: "<=", opIfGe: ">="}

// ifOf is the If opcode of a test's operator.
func ifOf(op string) (byte, bool) {
	for k, v := range ifOps {
		if v == op {
			return k, true
		}
	}
	return 0, false
}

// flipIf is the If that tests B op A where op tests A op B.
func flipIf(op byte) byte {
	switch op {
	case opIfLt:
		return opIfGt
	case opIfGt:
		return opIfLt
	case opIfLe:
		return opIfGe
	case opIfGe:
		return opIfLe
	}
	return op
}

// elseIf is the If that holds when op does not.
func elseIf(op byte) byte {
	switch op {
	case opIfEq:
		return opIfNe
	case opIfNe:
		return opIfEq
	case opIfLt:
		return opIfGe
	case opIfGe:
		return opIfLt
	case opIfGt:
		return opIfLe
	}
	return opIfGt
}

// isTerminator: the command after it is not reached by falling through.
func isTerminator(op byte) bool {
	return op == opGoto || op == opExit || op == opReturn || op == opNewECL
}

// A test (if A == B) reads the comparison flags, which the last comparing
// command set: Compare, CompareAnd, FindItem, or And/Or (which compare 0
// with the result). Every if makes its comparison; the forms that test
// flags set elsewhere say so: else if (the if's comparison), if Foo()
// (the section's return A op B), if Foo() op N (its return X), if X has
// Bits and if D = A & B; D op 0 (the And), and (op) for the flags as they
// are. The flag state is still tracked by the text, for the comments on
// the last form (flagsComment).

// returnsFrom walks the code a call of the section at label l runs (from
// l on, following its gotos; an unguarded return, exit or NewECL ends a
// path) and calls f with each return reached and the command before it
// (before its If when guarded), until f says to stop.
func (s *Script) returnsFrom(l int, f func(ret, prev int) bool) {
	seen := map[int]bool{}
	var walk func(i int) bool
	walk = func(i int) bool {
		for ; i < len(s.Code) && !seen[i]; i++ {
			seen[i] = true
			in := s.Code[i]
			guarded := i > 0 && isIf(s.Code[i-1].Op)
			switch {
			case in.Op == opReturn:
				c := i - 1
				if guarded {
					c--
				}
				if f(i, c) {
					return true
				}
				if !guarded {
					return false
				}
			case in.Op == opGoto && len(in.Args) == 1 && in.Args[0].Label:
				if walk(in.Args[0].N) {
					return true
				}
				if !guarded {
					return false
				}
			case (in.Op == opExit || in.Op == opNewECL) && !guarded:
				return false
			}
		}
		return false
	}
	walk(l)
}

// firstReturn is the first return a call of the section at label l comes
// back by with a comparing command (Compare, And, Or) before it, and that
// command; -1 when an unguarded plain return comes first, or none does.
func (s *Script) firstReturn(l int) (ret, c int) {
	ret, c = -1, -1
	s.returnsFrom(l, func(r, p int) bool {
		if p >= 0 && (s.Code[p].Op == opCompare || (s.Code[p].Op == opAnd || s.Code[p].Op == opOr) && len(s.Code[p].Args) == 3) {
			ret, c = r, p
			return true
		}
		return r == 0 || !isIf(s.Code[r-1].Op)
	})
	return ret, c
}

// flagState is what the comparison flags hold at a point of the script.
type flagState struct {
	kind byte  // opCompare, opCompareAnd, opFindItem, opAnd (0 against the result); 0 unknown
	args []Arg // the compared operands
	at   int   // the command that set them, -1 for none yet
}

// same: the same comparison (ignoring where it was made).
func (f flagState) same(g flagState) bool {
	return f.kind == g.kind && f.kind != 0 && slices.Equal(f.args, g.args)
}

// holds: the flags hold the comparison of kind on args; flipped when they
// hold it the other way round (0 op D for D op 0).
func (f flagState) holds(kind byte, args []Arg) (ok, flipped bool) {
	if f.kind == opAnd && kind == opCompare && len(args) == 2 {
		if args[0] == f.args[1] && args[1] == f.args[0] {
			return true, true
		}
	}
	return f.same(flagState{kind, args, 0}), false
}

// scriptFlags is the flag state before each command of s, by the text.
func scriptFlags(s *Script) []flagState {
	before := make([]flagState, len(s.Code))
	st := flagState{at: -1}
	for i := range s.Code {
		before[i] = st
		st = s.flagsAfter(st, i)
	}
	return before
}

// flagsAfter is the state after command i runs with st before it.
func (s *Script) flagsAfter(st flagState, i int) flagState {
	in := s.Code[i]
	guarded := i > 0 && isIf(s.Code[i-1].Op)
	switch in.Op {
	case opCompare, opCompareAnd, opFindItem:
		return flagState{in.Op, in.Args, i}
	case opAnd, opOr:
		if len(in.Args) == 3 {
			return flagState{opAnd, []Arg{{}, in.Args[2]}, i}
		}
	case opGosub:
		if len(in.Args) == 1 && in.Args[0].Label {
			if t, changed := s.returnFlags(in.Args[0].N, map[int]bool{}); changed {
				return flagState{t.kind, t.args, i}
			}
		}
	case opGoto, opExit, opReturn, opNewECL:
		if !guarded {
			return flagState{0, nil, i}
		}
	}
	return st
}

// returnFlags is the flag state a section called at label l returns
// with, by the text from l to its returns: changed is false when nothing
// in it compares. Unknown (kind 0) when its returns disagree, it leaves by
// a goto, or it calls itself.
func (s *Script) returnFlags(l int, busy map[int]bool) (flagState, bool) {
	if busy[l] {
		return flagState{0, nil, l}, true
	}
	busy[l] = true
	defer delete(busy, l)
	type result struct {
		st      flagState
		changed bool
	}
	var results []result
	st, changed := flagState{at: -1}, false
	for i := l; i < len(s.Code); i++ {
		in := s.Code[i]
		guarded := i > 0 && isIf(s.Code[i-1].Op)
		switch {
		case in.Op == opReturn:
			results = append(results, result{st, changed})
			if !guarded {
				goto done
			}
		case in.Op == opGoto && !guarded:
			return flagState{0, nil, l}, true
		case (in.Op == opExit || in.Op == opNewECL) && !guarded:
			goto done
		case in.Op == opGosub && len(in.Args) == 1 && in.Args[0].Label:
			if t, c := s.returnFlags(in.Args[0].N, busy); c {
				st, changed = flagState{t.kind, t.args, i}, true
			}
		default:
			if n := s.flagsAfter(st, i); n.at != st.at {
				st, changed = n, true
			}
		}
	}
done:
	if len(results) == 0 {
		return flagState{0, nil, l}, true
	}
	for _, r := range results[1:] {
		if r.changed != results[0].changed || r.changed && !r.st.same(results[0].st) {
			return flagState{0, nil, l}, true
		}
	}
	return results[0].st, results[0].changed
}

// menuOf is the two-part menu section at label l: a section that asks
// a one-line menu into D and returns with the flags comparing D to answer
// k (if Ask() == "YES"). ok is false when l is no such section.
func (s *Script) menuOf(l int) (d Arg, items []string, k int, ok bool) {
	for i := l; i < len(s.Code); i++ {
		in := s.Code[i]
		if in.Op == opHMenu && len(in.Args) >= 2 {
			d = in.Args[0]
			for _, a := range in.Args[2:] {
				items = append(items, a.S)
			}
			break
		}
		if isTerminator(in.Op) {
			return d, nil, 0, false
		}
	}
	t, changed := s.returnFlags(l, map[int]bool{})
	if items == nil || !changed || t.kind != opCompare || t.args[0] != d || t.args[1].Type != 0 || t.args[1].N >= len(items) {
		return d, nil, 0, false
	}
	return d, items, t.args[1].N, true
}

// The numbers in a command read by name where the names know them: a
// command's operand of a kind (LoadMonster's monster, Picture's picture;
// operandKinds), an area, and a number beside a variable by the names of
// that variable's values and flag bits (names/variables.txt): SelSide =
// PartyControlled, if Facing == South, Flags |= Seen|Done, Flags &=
// ~Seen, if Flags has Seen|Done, Scratch10 = ~Inside & Cell.

// varKinds are the variables whose values are of a kind named elsewhere.
var varKinds = map[string]string{
	"Geo": "Map", "WallSlot1": "Walls", "WallSlot2": "Walls", "WallSlot3": "Walls",
	"SelIcon": "Icon", "AreaKind": "Area",
}

// bitOps are the operators between a variable and a mask of its bits.
var bitOps = []string{"&", "|", "&=", "|=", "has", "lacks"}

// kindName is the name of kind's value v (an Area by names.Areas), or "".
func (n *ScriptNames) kindName(kind string, v int) string {
	if kind == "Area" {
		for _, a := range n.Areas {
			if a.ID == v {
				return a.Name
			}
		}
		return ""
	}
	return valueName(n.Values, kind, n.Set, v)
}

// kindValue is the number kind's name stands for.
func (n *ScriptNames) kindValue(kind, name string) (int, bool) {
	if kind == "Area" {
		for _, a := range n.Areas {
			if a.Name == name {
				return a.ID, true
			}
		}
		return 0, false
	}
	if kind == "" {
		return 0, false
	}
	return valueOf(n.Values, kind, n.Set, name)
}

// varValue is the name of number v beside variable name under operator
// op (its value, or with a bit operator its bits: A|B, ~A, ~(A|B)), or
// "".
func (n *ScriptNames) varValue(name, op string, v int) string {
	if k := varKinds[name]; k != "" {
		if x := n.kindName(k, v); x != "" {
			return x
		}
	}
	if x := valueName(n.Values, name, n.Set, v); x != "" || !slices.Contains(bitOps, op) || v <= 0 || v > 0xFF {
		return x
	}
	bits := func(v int) string {
		var s []string
		for b := 1; b <= 0x80; b <<= 1 {
			if v&b == 0 {
				continue
			}
			x := valueName(n.Values, name, n.Set, b)
			if x == "" {
				return ""
			}
			s = append(s, x)
		}
		return strings.Join(s, "|")
	}
	if x := bits(v); x != "" {
		return x
	}
	if op == "&" || op == "&=" {
		if x := bits(0xFF ^ v); x != "" {
			if strings.Contains(x, "|") {
				return "~(" + x + ")"
			}
			return "~" + x
		}
	}
	return ""
}

// varNumber is the number a value or bit name beside variable name
// stands for (varValue's forms).
func (n *ScriptNames) varNumber(name, text string) (int, bool) {
	if k := varKinds[name]; k != "" {
		if v, ok := n.kindValue(k, text); ok {
			return v, true
		}
	}
	if v, ok := valueOf(n.Values, name, n.Set, text); ok {
		return v, true
	}
	not := strings.HasPrefix(text, "~")
	text = strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(text, "~"), "("), ")"), "~")
	v := 0
	for _, p := range strings.Split(text, "|") {
		b, ok := valueOf(n.Values, name, n.Set, p)
		if !ok {
			return 0, false
		}
		v |= b
	}
	if not {
		v ^= 0xFF
	}
	return v, true
}

// eventNames are the entry points' names in the text (on load {), by
// event number.
var eventNames = [5]string{"move", "step", "camp", "rest", "load"}

// eventLabel is event i's label (event4).
func eventLabel(i int) string { return fmt.Sprintf("event%d", i) }

// commandNames are the commands written as calls, Name(operands), by
// opcode: the opcode names but the printing ones.
func commandName(op byte) string {
	switch op {
	case opPrintClear:
		return "Text"
	}
	return eclOpName(op)
}

// commandOp is the opcode of a command name, or false.
func commandOp(name string) (byte, bool) {
	if name == "Text" {
		return opPrintClear, true
	}
	if name == "PrintClear" || name == "" {
		return 0, false
	}
	op := opcode(name)
	return op, op != 0 || name == "Exit"
}

// treasureLabels name Treasure's operands.
var treasureLabels = []string{"copper", "silver", "electrum", "gold", "platinum", "gems", "jewelry", "items"}

// tableKind is the kind of a table's entries by its name: a table of
// Monsters or Icons names them.
func tableKind(label string) string {
	for _, k := range []string{"Monster", "Icon"} {
		if strings.HasSuffix(label, k+"s") {
			return k
		}
	}
	return ""
}

// The variables several forms name.
const (
	hasScratch   = "Scratch10" // where a bit test (if X has Bits) puts the masked bits
	varPortrait  = "Portrait"
	varSymbolSet = "SymbolSet"
	varShop      = "Shop"
	varShopFlag  = "ShopFlag"
	varShopKind  = "ShopKind"
	varTemple    = "Temple"
)
