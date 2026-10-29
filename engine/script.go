package engine

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The area event scripts as text (data/assets/scripts/area<N>.txt, written
// once from the ECL byte code by cmd/eclscript, edited by hand since):
//
//	Label:                one or more labels name the next line
//		Op arg arg ...    one command per line (eclOpName's names)
//		if A == B: Op ... a test (scriptfold.go)
//		Data 1 2 3 ...    the bytes of a table (GetTable's)
//	// comment
//
// Operands are a byte (12), a word (#0807, hex), a variable, label or
// table by name or script address ([4A07]), a variable read through the
// other operand kind (&Name), a string ("TEXT", Go quoting) or a string
// variable ($Name). Variables are named by the names files beside the
// scripts (names/global.txt, names/area<N>.txt) over ScriptVarNames.
//
// The engine runs a Script directly: eclPC is an instruction number and
// FetchArgs hands an instruction's operands to the opcode handlers in the
// form they had in the byte code. The tables are laid out one after the
// other from eclBase in e.ecl, where GetTable reads them.

// Script is one area's parsed event script.
type Script struct {
	Area   int
	Events [5]int         // the entry points' instruction numbers
	Code   []Instr        // the commands in file order
	Labels map[string]int // a code label's instruction number
	At     map[int]string // an instruction's first label
	Data   []byte         // the tables, from eclBase
}

// Instr is one command.
type Instr struct {
	Op   byte
	Args []Arg
	Line int // in the script file
}

// Arg is one operand in its byte-code form.
type Arg struct {
	Type byte   // 0 byte, 1 and 3 variable, 2 word, 0x80 string, 0x81 string variable
	N    int    // the value or address (a label's instruction number)
	S    string // a string's text
}

// scriptDir is where the area scripts are, in the data directory.
func (e *Engine) scriptDir() string { return filepath.Join(e.data, AssetsDir, "scripts") }

// loadScript parses area's script into e.script and lays its tables out
// in e.ecl. A script that is missing or wrong is reported and runs as an
// empty one (every event exits).
func (e *Engine) loadScript(area int) {
	clear(e.ecl)
	s, err := e.parseScriptFile(area)
	if err != nil {
		fmt.Fprintln(os.Stderr, "script:", err)
		s = &Script{Area: area, Code: []Instr{{Op: 0}}, Labels: map[string]int{}, At: map[int]string{}}
	}
	e.script = s
	copy(e.ecl, s.Data)
	e.native = map[int]func(*Engine){}
	for at, f := range nativeScripts {
		if at.area == area {
			if i, ok := s.Labels[at.label]; ok {
				e.native[i] = f
			}
		}
	}
}

func (e *Engine) parseScriptFile(area int) (*Script, error) {
	dir := e.scriptDir()
	if e.scriptNames == nil {
		e.scriptNames = ScriptVarNames()
		if err := ReadScriptNames(filepath.Join(dir, "names", "global.txt"), e.scriptNames); err != nil {
			return nil, err
		}
	}
	names := map[int]string{}
	for a, n := range e.scriptNames {
		names[a] = n
	}
	if err := ReadScriptNames(filepath.Join(dir, "names", fmt.Sprintf("area%d.txt", area)), names); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, fmt.Sprintf("area%d.txt", area))
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	vars := map[string]int{}
	for a, n := range names {
		vars[n] = a
	}
	areas, err := ReadScriptAreas(filepath.Join(dir, "names", "areas.txt"))
	if err != nil {
		return nil, err
	}
	s, err := ParseScript(string(b), vars, areas)
	if err != nil {
		return nil, fmt.Errorf("%s:%v", path, err)
	}
	s.Area = area
	return s, nil
}

// ParseScript parses a script; vars are the variables' addresses by name.
func ParseScript(text string, vars map[string]int, areas []ScriptArea) (*Script, error) {
	s := &Script{Labels: map[string]int{}, At: map[int]string{}}
	tables := map[string]int{}
	ops := map[string]byte{}
	for op := range 0x40 {
		ops[eclOpName(byte(op))] = byte(op)
	}
	type pending struct {
		line int
		args []string
	}
	var todo []pending
	var waiting []string // labels for the next line
	for _, a := range areas {
		if _, ok := vars[a.Name]; ok {
			return nil, fmt.Errorf("%s is an area and a variable", a.Name)
		}
	}
	src, err := unfoldScript(text, areas)
	if err != nil {
		return nil, err
	}
	for _, sl := range src {
		n, toks := sl.n, sl.toks
		if sl.label {
			l := strings.TrimSuffix(toks[0], ":")
			if _, dup := s.Labels[l]; dup || tables[l] != 0 {
				return nil, fmt.Errorf("%d: label %s twice", n, l)
			}
			waiting = append(waiting, l)
			continue
		}
		if toks[0] == "Data" {
			for _, l := range waiting {
				tables[l] = eclBase + len(s.Data)
			}
			waiting = nil
			for _, t := range toks[1:] {
				v, err := strconv.ParseUint(t, 10, 8)
				if err != nil {
					return nil, fmt.Errorf("%d: data byte %q", n, t)
				}
				s.Data = append(s.Data, byte(v))
			}
			continue
		}
		op, ok := ops[toks[0]]
		if !ok {
			return nil, fmt.Errorf("%d: no command %s", n, toks[0])
		}
		for _, l := range waiting {
			s.Labels[l] = len(s.Code)
			if _, ok := s.At[len(s.Code)]; !ok {
				s.At[len(s.Code)] = l
			}
		}
		waiting = nil
		s.Code = append(s.Code, Instr{Op: op, Line: n})
		todo = append(todo, pending{n, toks[1:]})
	}
	if len(waiting) > 0 {
		return nil, fmt.Errorf("label %s at the end", waiting[0])
	}
	for i := range s.Events {
		at, ok := s.Labels[fmt.Sprintf("event%d", i)]
		if !ok {
			return nil, fmt.Errorf("no event%d", i)
		}
		s.Events[i] = at
	}
	for _, a := range areas {
		if _, ok := s.Labels[a.Name]; ok || tables[a.Name] != 0 {
			return nil, fmt.Errorf("%s is an area and a label", a.Name)
		}
	}
	for i, p := range todo {
		for _, t := range p.args {
			a, err := scriptArg(t, s.Labels, tables, vars, areas)
			if err != nil {
				return nil, fmt.Errorf("%d: %v", p.line, err)
			}
			s.Code[i].Args = append(s.Code[i].Args, a)
		}
	}
	return s, nil
}

// scriptArg is one operand token.
func scriptArg(t string, labels, tables, vars map[string]int, areas []ScriptArea) (Arg, error) {
	if strings.HasPrefix(t, `"`) {
		str, err := strconv.Unquote(t)
		if err != nil {
			return Arg{}, fmt.Errorf("string %s", t)
		}
		n := (len(str)*6 + 7) / 8 // its length in the byte code, 6 bits a letter
		return Arg{Type: 0x80, N: n, S: str}, nil
	}
	if strings.HasPrefix(t, "#") {
		v, err := strconv.ParseUint(t[1:], 16, 16)
		return Arg{Type: 2, N: int(v)}, err
	}
	if t[0] >= '0' && t[0] <= '9' {
		v, err := strconv.ParseUint(t, 10, 8)
		return Arg{Type: 0, N: int(v)}, err
	}
	typ := byte(1)
	switch t[0] {
	case '&':
		typ, t = 3, t[1:]
	case '$':
		typ, t = 0x81, t[1:]
	}
	if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
		v, err := strconv.ParseUint(t[1:len(t)-1], 16, 16)
		return Arg{Type: typ, N: int(v)}, err
	}
	if i, ok := labels[t]; ok {
		return Arg{Type: typ, N: i}, nil
	}
	if a, ok := tables[t]; ok {
		return Arg{Type: typ, N: a}, nil
	}
	if a, ok := vars[t]; ok {
		return Arg{Type: typ, N: a}, nil
	}
	for _, a := range areas {
		if a.Name == t && typ == 1 {
			return Arg{Type: 0, N: a.ID}, nil
		}
	}
	return Arg{}, fmt.Errorf("no label, table or variable %s", t)
}

// scriptTokens splits a line at spaces outside strings, without its
// // comment.
func scriptTokens(line string) ([]string, error) {
	var toks []string
	cur, quoted, esc := "", false, false
	flush := func() {
		if cur != "" {
			toks = append(toks, cur)
		}
		cur = ""
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quoted:
			cur += string(c)
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				quoted = false
			}
		case c == '"':
			quoted = true
			cur += string(c)
		case c == '/' && strings.HasPrefix(line[i:], "//"):
			flush()
			return toks, nil
		case c == ' ' || c == '\t' || c == '\r':
			flush()
		case c == '{' || c == '}' || c == ';' || c == ':':
			flush()
			toks = append(toks, string(c))
		default:
			cur += string(c)
		}
	}
	if quoted {
		return nil, fmt.Errorf("unterminated string")
	}
	flush()
	return toks, nil
}

// ReadScriptNames adds the "XXXX Name" lines of a names file, if there is
// one (# starts a comment).
func ReadScriptNames(path string, names map[int]string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line, _, _ := strings.Cut(sc.Text(), "#")
		fs := strings.Fields(line)
		if len(fs) == 0 {
			continue
		}
		a, err := strconv.ParseUint(fs[0], 16, 16)
		if len(fs) != 2 || err != nil {
			return fmt.Errorf("%s:%d: want \"XXXX Name\"", path, n)
		}
		names[int(a)] = fs[1]
	}
	return sc.Err()
}

// ScriptArea is one area of names/areas.txt.
type ScriptArea struct {
	ID, Set int // NewECL's number, the data set holding it
	Name    string
}

// ReadScriptAreas reads the "ID SET Name" lines of an areas file, if
// there is one (# starts a comment).
func ReadScriptAreas(path string) ([]ScriptArea, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var areas []ScriptArea
	for n, line := range strings.Split(string(b), "\n") {
		line, _, _ = strings.Cut(line, "#")
		fs := strings.Fields(line)
		if len(fs) == 0 {
			continue
		}
		id, err1 := strconv.Atoi(fs[0])
		set, err2 := strconv.Atoi(fs[1%len(fs)])
		if len(fs) != 3 || err1 != nil || err2 != nil {
			return nil, fmt.Errorf("%s:%d: want \"ID SET Name\"", path, n+1)
		}
		areas = append(areas, ScriptArea{id, set, fs[2]})
	}
	return areas, nil
}
