package engine

import (
	"fmt"
	"io"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
)

// ListECL writes a listing of one ECL block (the block bytes after its
// 2-byte header) to w. It decodes operands the way FetchArgs does, but
// never reads variables: a menu or ON GOTO count held in a variable is
// listed as 0 operands. Code is found by following the control flow from
// the five event entry points, so data between scripts is skipped. A
// development aid (cmd/ecl), not part of the game.
func ListECL(code []byte, w io.Writer) {
	events, lines, _ := walkECL(code)
	for i, pc := range events {
		fmt.Fprintf(w, "event %d -> %04X\n", i, pc)
	}
	for _, pc := range sortedPCs(lines) {
		fmt.Fprintf(w, "%04X  %s\n", pc, lines[pc].text)
	}
}

// ScriptECL writes one ECL block as a script (the format ParseScript
// reads): one command and its operands per line, jump targets as labels
// (event0..event4 for the entry points; the others by their names in
// names, L_XXXX unnamed), variables by their names in names (script
// address -> name; an unnamed one stays [XXXX]), and the tables inside
// the code image as Data lines under their labels. For cmd/eclscript.
func ScriptECL(code []byte, w io.Writer, names map[int]string) error {
	events, lines, jumps := walkECL(code)
	end := eclBase + len(code)
	labels := map[int][]string{}
	name := func(pc int, def string) string {
		if n := names[pc]; n != "" {
			return n
		}
		return def
	}
	for pc := range jumps {
		if _, ok := lines[pc]; !ok {
			return fmt.Errorf("a jump to %04X, which is not code", pc)
		}
		labels[pc] = []string{name(pc, fmt.Sprintf("L_%04X", pc))}
	}
	for pc := range lines {
		if _, ok := labels[pc]; !ok && names[pc] != "" {
			labels[pc] = []string{names[pc]} // a named place no jump goes to (a native script's)
		}
	}
	for i, pc := range events {
		if _, ok := labels[pc]; ok && strings.HasPrefix(labels[pc][0], "event") {
			labels[pc] = append(labels[pc], fmt.Sprintf("event%d", i))
		} else {
			labels[pc] = []string{fmt.Sprintf("event%d", i)} // an event entry wins over a jump label
		}
	}
	// the tables run from their address to the next code or table
	tables := eclTables(lines, end)
	code2 := map[int]bool{}
	for pc, l := range lines {
		for a := pc; a < l.next; a++ {
			if code2[a] {
				return fmt.Errorf("instructions overlap at %04X", a)
			}
			code2[a] = true
		}
	}
	var pcs []int
	for pc := range lines {
		pcs = append(pcs, pc)
	}
	for pc := range tables {
		if code2[pc] {
			return fmt.Errorf("the table at %04X is code", pc)
		}
		pcs = append(pcs, pc)
		labels[pc] = []string{name(pc, fmt.Sprintf("T_%04X", pc))}
	}
	sort.Ints(pcs)
	first := true
	for _, pc := range pcs {
		if ls, ok := labels[pc]; ok {
			if !first {
				fmt.Fprintln(w)
			}
			for _, n := range ls {
				fmt.Fprintf(w, "%s:\n", n)
			}
		}
		first = false
		if tables[pc] {
			a := pc
			for a < end && !code2[a] && (a == pc || !tables[a]) {
				a++
			}
			for i := pc; i < a; i += 16 {
				var bs []string
				for j := i; j < a && j < i+16; j++ {
					bs = append(bs, fmt.Sprint(code[j-eclBase]))
				}
				fmt.Fprintf(w, "\tData %s\n", strings.Join(bs, " "))
			}
			continue
		}
		l := lines[pc]
		args := append([]string(nil), l.args...)
		for j, a := range args {
			pre, rest := "", a
			if strings.HasPrefix(a, "&") || strings.HasPrefix(a, "$") {
				pre, rest = a[:1], a[1:]
			}
			var v int
			if _, err := fmt.Sscanf(rest, "[%04X]", &v); err != nil || len(rest) != 6 {
				continue
			}
			if slices.Contains(l.targets, v) {
				args[j] = pre + labels[v][0]
			} else if tables[v] {
				args[j] = pre + labels[v][0]
			} else if n := names[v]; n != "" {
				args[j] = pre + n
			}
		}
		fmt.Fprintf(w, "\t%s\n", strings.TrimSpace(eclOpName(l.op)+" "+strings.Join(args, " ")))
		switch l.name {
		case "opGoto", "opExit", "opReturn", "opNewECL":
		default:
			if _, ok := lines[l.next]; !ok {
				fmt.Fprintf(w, "\tExit // extra: the original runs on into data at %04X\n", l.next)
			}
		}
	}
	return nil
}

// eclOpName is the script name of opcode op: its handler's name without
// "op", or the opcode's own where several share a handler.
func eclOpName(op byte) string {
	if n, ok := eclOpNames[op]; ok {
		return n
	}
	if int(op) >= len(eclOps) || eclOps[op] == nil {
		return fmt.Sprintf("Op%02X", op)
	}
	n := runtime.FuncForPC(reflect.ValueOf(eclOps[op]).Pointer()).Name()
	n = strings.TrimSuffix(strings.TrimPrefix(n, "por/engine.(*Engine)."), "-fm")
	return strings.TrimPrefix(n, "op")
}

var eclOpNames = map[byte]string{
	0x04: "Add", 0x05: "Sub", 0x06: "Div", 0x07: "Mul", 0x11: "Print", 0x12: "PrintClear",
	0x16: "IfEq", 0x17: "IfNe", 0x18: "IfLt", 0x19: "IfGt", 0x1A: "IfLe", 0x1B: "IfGe",
	0x21: "LoadFiles", 0x37: "LoadPieces", 0x2F: "And", 0x30: "Or",
}

// ScriptVarNames are the names of the script words the engine itself
// uses: the named fields of the area and game records (an array word as
// Name_i) and the specials (the party's x, y and facing are the same
// words for reading and writing: eclSpecialLow+0xE3 = eclSpecialHigh). cmd/eclscript starts from these.
func ScriptVarNames() map[int]string {
	names := map[int]string{}
	add := func(base int, rec any) {
		t := reflect.TypeOf(rec)
		i := 0
		for f := range t.NumField() {
			sf := t.Field(f)
			n := 1
			if sf.Type.Kind() == reflect.Array {
				n = sf.Type.Len()
			}
			if !strings.HasPrefix(sf.Name, "V") || sf.Name == "ViewFlags" {
				for k := range n {
					name := sf.Name
					if n > 1 {
						name = fmt.Sprintf("%s_%d", sf.Name, k)
					}
					names[base+i+k] = name
				}
			}
			i += n
		}
	}
	add(areaVarBase, areaVars{})
	add(gameVarBase, gameVars{})
	for k, n := range map[int]string{
		eclSpecialLow + 0xF1: "SetGroundShade", eclSpecialLow + 0xF7: "SetSkyShade",
		eclSpecialHigh: "PartyX", eclSpecialHigh + 1: "PartyY",
		eclSpecialHigh + 2: "Facing", eclSpecialHigh + 3: "WallAhead", eclSpecialHigh + 4: "Cell",
		0x3DE: "Sound", 0x33D: "FacingRaw",
	} {
		names[k] = n
	}
	return names
}

// eclLine is one decoded instruction of walkECL.
type eclLine struct {
	op         byte
	name, text string
	args       []string
	targets    []int
	next       int // the address after it
}

// walkECL decodes the code reachable from the five event entry points:
// the entry addresses, the instructions by address and the jump targets.
// Flow never enters the tables (eclTables), which a plain walk would
// decode as code where an instruction runs on into one.
func walkECL(code []byte) ([]int, map[int]eclLine, map[int]bool) {
	tables := map[int]bool{}
	for {
		events, lines, jumps := walkFrom(code, tables)
		more := eclTables(lines, eclBase+len(code))
		if len(more) == len(tables) {
			return events, lines, jumps
		}
		tables = more
	}
}

// eclTables are the table addresses inside the code image (GetTable's
// and Protection's first operand) of the decoded lines.
func eclTables(lines map[int]eclLine, end int) map[int]bool {
	t := map[int]bool{}
	for _, l := range lines {
		if l.name != "opGetTable" && l.name != "opProtection" || len(l.args) == 0 {
			continue
		}
		var a int
		if _, err := fmt.Sscanf(l.args[0], "[%04X]", &a); err == nil && a >= eclBase && a < end {
			t[a] = true
		}
	}
	return t
}

func walkFrom(code []byte, tables map[int]bool) ([]int, map[int]eclLine, map[int]bool) {
	e := &Engine{ecl: make([]byte, 0x1E00)}
	copy(e.ecl, code)
	end := eclBase + len(code)
	e.eclPC = eclBase
	var events []int
	for i := 0; i < 5; i++ {
		e.FetchArgs(1)
		events = append(events, e.ArgAddr(1))
	}
	lines := map[int]eclLine{}
	jumps := map[int]bool{}
	work := append([]int(nil), events...)
	for len(work) > 0 {
		pc := work[len(work)-1]
		work = work[:len(work)-1]
		if _, seen := lines[pc]; seen || pc < eclBase || pc >= end || tables[pc] {
			continue
		}
		name, text, next, targets, args := e.eclDecode(pc)
		lines[pc] = eclLine{e.eclByte(pc), name, text, args, targets, next}
		for _, t := range targets {
			jumps[t] = true
		}
		work = append(work, targets...)
		switch name {
		case "opGoto", "opExit", "opReturn", "opNewECL", "?":
		case "opIf":
			work = append(work, next)
			if n2, _, after, _, _ := e.eclDecode(next); n2 != "?" && !tables[next] {
				work = append(work, after)
			}
		default:
			work = append(work, next)
		}
	}
	return events, lines, jumps
}

func sortedPCs(lines map[int]eclLine) []int {
	pcs := make([]int, 0, len(lines))
	for pc := range lines {
		pcs = append(pcs, pc)
	}
	sort.Ints(pcs)
	return pcs
}

// eclArgCount is the FetchArgs count of each fixed-length opcode handler.
var eclArgCount = map[string]int{
	"opAddNPC": 2, "opAndOr": 3, "opArith": 3, "opCall": 1, "opCheckParty": 6,
	"opClock": 1, "opCompare": 2, "opCompareAnd": 4, "opDamage": 5,
	"opEncounterMenu": 14, "opFindItem": 1, "opGetTable": 3, "opGosub": 1,
	"opGoto": 1, "opInputNumber": 2, "opInputString": 2, "opLoadCharacter": 1,
	"opLoadFiles": 3, "opLoadMonster": 3, "opNewECL": 1, "opParlay": 6,
	"opPartyStrength": 1, "opPartySurprise": 2, "opPicture": 1, "opPrint": 1,
	"opProgram": 1, "opProtection": 1, "opRandom": 2, "opRob": 3, "opSave": 2,
	"opSaveTable": 3, "opSetupMonster": 3, "opSpell": 3, "opSurprise": 4,
	"opTreasure": 8, "opWho": 1,
}

// eclDecode decodes the instruction at pc: its name, its listing, the
// next instruction's address, the addresses it jumps to and its operands. It uses e's
// argument state, so it runs on a scratch engine (ListECL's, eclAhead's).
func (e *Engine) eclDecode(pc int) (string, string, int, []int, []string) {
	e.eclPC = pc
	var targets []int
	count := 0
	op := e.eclByte(pc)
	var args []string
	fetch := func(n int) {
		k := 0
		for i := 1; i <= n; i++ {
			t, lo := e.eclByte(e.eclPC+1), e.eclByte(e.eclPC+2)
			e.es.argType[i], e.es.argLo[i], e.es.argHi[i] = t, lo, 0
			e.eclPC += 2
			switch t {
			case 0:
				args = append(args, fmt.Sprintf("%d", lo))
			case 1, 2, 3:
				e.eclPC++
				e.es.argHi[i] = e.eclByte(e.eclPC)
				switch t {
				case 2:
					args = append(args, fmt.Sprintf("#%04X", e.ArgAddr(i)))
				case 3:
					args = append(args, fmt.Sprintf("&[%04X]", e.ArgAddr(i)))
				default:
					args = append(args, fmt.Sprintf("[%04X]", e.ArgAddr(i)))
				}
			case 0x80:
				k++
				s := ""
				if lo > 0 {
					s = e.decodePackedString(int(lo))
				}
				args = append(args, fmt.Sprintf("%q", s))
			case 0x81:
				k++
				e.eclPC++
				e.es.argHi[i] = e.eclByte(e.eclPC)
				args = append(args, fmt.Sprintf("$[%04X]", e.ArgAddr(i)))
			default:
				args = append(args, fmt.Sprintf("?%02X:%02X", t, lo))
			}
		}
	}
	imm := func(i int) int { // a count operand, only when immediate
		if e.es.argType[i] == 0 {
			return int(e.es.argLo[i])
		}
		return 0
	}
	name := "?"
	if int(op) < len(eclOps) && eclOps[op] != nil {
		name = strings.TrimPrefix(runtime.FuncForPC(reflect.ValueOf(eclOps[op]).Pointer()).Name(), "por/engine.(*Engine).")
		name = strings.TrimSuffix(name, "-fm")
	}
	switch name {
	case "opVMenu":
		fetch(3)
		fetch(imm(3))
	case "opHMenu":
		fetch(2)
		fetch(imm(2))
	case "opOnGoto":
		fetch(2)
		count = imm(2)
		fetch(count)
	default:
		fetch(eclArgCount[name])
	}
	switch name {
	case "opGoto", "opGosub":
		targets = append(targets, e.ArgAddr(1))
	case "opOnGoto":
		for i := 1; i <= count; i++ {
			targets = append(targets, e.ArgAddr(i))
		}
	}
	e.eclPC++
	return name, fmt.Sprintf("%02X %s %s", op, name, strings.Join(args, " ")), e.eclPC, targets, args
}
