package engine

import (
	"fmt"
	"sort"
	"strconv"
)

// DecodeScript decodes an area's ECL byte code (an ECL block without its
// two-byte header) into a Script: the commands reachable from the five
// events, their labels (named by names.Vars where it names an address,
// else L_XXXX) and the tables. cmd/eclscript writes the result with
// WriteScript.
func DecodeScript(code []byte, names *ScriptNames) (*Script, error) {
	events, lines, jumps := walkECL(code)
	end := eclBase + len(code)
	tables := eclTables(lines, end)
	isCode := map[int]bool{}
	for pc, l := range lines {
		for a := pc; a < l.next; a++ {
			if isCode[a] {
				return nil, fmt.Errorf("instructions overlap at %04X", a)
			}
			isCode[a] = true
		}
	}
	for pc := range jumps {
		if _, ok := lines[pc]; !ok {
			return nil, fmt.Errorf("a jump to %04X, which is not code", pc)
		}
	}
	for pc := range tables {
		if isCode[pc] {
			return nil, fmt.Errorf("the table at %04X is code", pc)
		}
	}
	s := &Script{Labels: map[string]int{}, At: map[int]string{}, Tables: map[string]int{}}
	exit := opcode("Exit")
	name := func(pc int, def string) string {
		if n := names.Vars[pc]; n != "" {
			return n
		}
		return def
	}
	// the code, in address order; a label where a jump lands, an event
	// starts or the names name the address
	pcs := sortedPCs(lines)
	index := map[int]int{} // instruction number by address
	for _, pc := range pcs {
		index[pc] = len(s.Code)
		l := lines[pc]
		s.Code = append(s.Code, Instr{Op: l.op})
		if jumps[pc] || names.Vars[pc] != "" {
			s.label(name(pc, fmt.Sprintf("L_%04X", pc)), index[pc])
		}
		switch l.name {
		case "opGoto", "opExit", "opReturn", "opNewECL":
		default:
			if _, ok := lines[l.next]; !ok {
				// the original runs on into what is not code
				s.Code = append(s.Code, Instr{Op: exit, Comment: fmt.Sprintf("the original runs on into data at %04X", l.next)})
			}
		}
	}
	for i, pc := range events {
		s.Events[i] = index[pc]
		s.label(fmt.Sprintf("event%d", i), index[pc])
	}
	// the tables, each to the next code or table, laid out one after the
	// other from eclBase as the parser lays them (loadScript copies Data
	// there): their references move with them
	var tpcs []int
	for pc := range tables {
		tpcs = append(tpcs, pc)
	}
	sort.Ints(tpcs)
	moved := map[int]int{}
	for _, pc := range tpcs {
		moved[pc] = eclBase + len(s.Data)
		s.Tables[name(pc, fmt.Sprintf("T_%04X", pc))] = moved[pc]
		a := pc
		for a < end && !isCode[a] && (a == pc || !tables[a]) {
			a++
		}
		s.Data = append(s.Data, code[pc-eclBase:a-eclBase]...)
	}
	// the operands
	for _, pc := range pcs {
		l := lines[pc]
		i := index[pc]
		for _, a := range l.args {
			arg, err := decodeArg(a, l, index)
			if err != nil {
				return nil, fmt.Errorf("%04X: %v", pc, err)
			}
			if at, ok := moved[arg.N]; ok && !arg.Label && (arg.Type == 1 || arg.Type == 3) {
				arg.N = at
			}
			s.Code[i].Args = append(s.Code[i].Args, arg)
		}
	}
	return s, nil
}

// label names instruction i.
func (s *Script) label(name string, i int) {
	s.Labels[name] = i
	if _, ok := s.At[i]; !ok {
		s.At[i] = name
	}
}

// decodeArg is an operand of eclDecode's listing as an Arg: a jump
// target as its instruction number, else as in the byte code.
func decodeArg(a string, l eclLine, index map[int]int) (Arg, error) {
	var v int
	switch {
	case a[0] == '"':
		str, err := strconv.Unquote(a)
		if err != nil {
			return Arg{}, err
		}
		return Arg{Type: 0x80, N: (len(str)*6 + 7) / 8, S: str}, nil
	case a[0] == '#':
		_, err := fmt.Sscanf(a, "#%04X", &v)
		return Arg{Type: 2, N: v}, err
	case a[0] == '[':
		_, err := fmt.Sscanf(a, "[%04X]", &v)
		if err != nil {
			return Arg{}, err
		}
		for _, t := range l.targets {
			if t == v {
				return Arg{Type: 1, N: index[v], Label: true}, nil
			}
		}
		return Arg{Type: 1, N: v}, nil
	case a[0] == '&':
		_, err := fmt.Sscanf(a, "&[%04X]", &v)
		return Arg{Type: 3, N: v}, err
	case a[0] == '$':
		_, err := fmt.Sscanf(a, "$[%04X]", &v)
		return Arg{Type: 0x81, N: v}, err
	case a[0] >= '0' && a[0] <= '9':
		v, err := strconv.Atoi(a)
		return Arg{Type: 0, N: v}, err
	}
	return Arg{}, fmt.Errorf("operand %s", a)
}
