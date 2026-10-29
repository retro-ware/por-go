package engine

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The area event scripts are text (data/assets/scripts/area<N>.txt,
// README.md there), written from the ECL byte code by cmd/eclscript
// (DecodeScript, WriteScript) and parsed at area load (ParseScript).
// Variables are named by the names files beside the scripts
// (names/global.txt, names/area<N>.txt) over ScriptVarNames.
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
	Tables map[string]int // a table's address in Data (eclBase + its offset)
}

// Instr is one command.
type Instr struct {
	Op      byte
	Args    []Arg
	Line    int    // in the script file
	Comment string // a note from the decoder, written after the command
}

// Arg is one operand in its byte-code form.
type Arg struct {
	Type  byte   // 0 byte, 1 and 3 variable, 2 word, 0x80 string, 0x81 string variable
	N     int    // the value or address (a label's instruction number)
	S     string // a string's text
	Label bool   // N is a label's instruction number (a jump's operand)
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
	names, err := ReadAreaNames(dir, area, false)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, fmt.Sprintf("area%d.txt", area))
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s, err := ParseScript(string(b), names)
	if err != nil {
		return nil, fmt.Errorf("%s:%v", path, err)
	}
	s.Area = area
	return s, nil
}

// ReadAreaNames reads area's names from the names files in the scripts
// folder dir: the variables (with labels, the labels and tables too), the
// areas and the values.
func ReadAreaNames(dir string, area int, labels bool) (*ScriptNames, error) {
	n := &ScriptNames{Vars: ScriptVarNames()}
	files := []string{"global.txt", fmt.Sprintf("area%d.txt", area)}
	if labels {
		files = append(files, fmt.Sprintf("labels/area%d.txt", area))
	}
	for _, f := range files {
		if err := ReadScriptNames(filepath.Join(dir, "names", f), n.Vars); err != nil {
			return nil, err
		}
	}
	var err error
	if n.Areas, err = ReadScriptAreas(filepath.Join(dir, "names", "areas.txt")); err != nil {
		return nil, err
	}
	for _, a := range n.Areas {
		if a.ID == area {
			n.Set = a.Set
		}
	}
	for _, f := range []string{"values.txt", "pictures.txt", "variables.txt"} {
		if n.Values, err = ReadScriptValues(filepath.Join(dir, "names", f), n.Values); err != nil {
			return nil, err
		}
	}
	return n, nil
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

// ScriptNames are the names a script's words stand for.
type ScriptNames struct {
	Vars     map[int]string // variables (and, for cmd/eclscript, labels and tables) by address
	Areas    []ScriptArea
	Values   []ScriptValue
	Set      int                    // the area's data set
	Treasure func(block int) string // an ITEM block's items, for cmd/eclscript's comments
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
