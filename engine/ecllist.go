package engine

import (
	"fmt"
	"io"
	"reflect"
	"runtime"
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

// ScriptECL writes one ECL block as a script: one command and its
// operands per line, jump targets as labels (event0..event4 for the
// entry points, L_XXXX for the others), a blank line after each
// unconditional end of flow. A development aid (cmd/ecl), like ListECL.
func ScriptECL(code []byte, w io.Writer) {
	events, lines, jumps := walkECL(code)
	label := map[int]string{}
	for pc := range jumps {
		label[pc] = fmt.Sprintf("L_%04X", pc)
	}
	for i, pc := range events {
		label[pc] = fmt.Sprintf("event%d", i) // an event entry wins over a jump label
	}
	for i, pc := range sortedPCs(lines) {
		l := lines[pc]
		if n, ok := label[pc]; ok {
			if i > 0 {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "%s:\n", n)
		}
		args := l.args
		for _, t := range l.targets {
			for j, a := range args {
				if a == fmt.Sprintf("[%04X]", t) {
					args[j] = label[t]
				}
			}
		}
		name := strings.TrimPrefix(l.name, "op")
		if l.name == "opIf" {
			name = []string{"IfEq", "IfNe", "IfLt", "IfGt", "IfLe", "IfGe"}[l.op-0x16]
		}
		fmt.Fprintf(w, "\t%s\n", strings.TrimSpace(name+" "+strings.Join(args, " ")))
	}
}

// eclLine is one decoded instruction of walkECL.
type eclLine struct {
	op         byte
	name, text string
	args       []string
	targets    []int
}

// walkECL decodes the code reachable from the five event entry points:
// the entry addresses, the instructions by address and the jump targets.
func walkECL(code []byte) ([]int, map[int]eclLine, map[int]bool) {
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
		if _, seen := lines[pc]; seen || pc < eclBase || pc >= end {
			continue
		}
		name, text, next, targets, args := e.eclDecode(pc)
		lines[pc] = eclLine{e.eclByte(pc), name, text, args, targets}
		for _, t := range targets {
			jumps[t] = true
		}
		work = append(work, targets...)
		switch name {
		case "opGoto", "opExit", "opReturn", "opNewECL", "?":
		case "opIf":
			work = append(work, next)
			if n2, _, after, _, _ := e.eclDecode(next); n2 != "?" {
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
				if t == 2 {
					args = append(args, fmt.Sprintf("#%04X", e.ArgAddr(i)))
				} else {
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
				args = append(args, fmt.Sprintf("str[%04X]", e.ArgAddr(i)))
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
