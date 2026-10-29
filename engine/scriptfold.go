package engine

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// The scripts' conditions. A test and the commands it guards read as
//
//	if Gold >= 100: Goto Rich           one command
//	if Facing == 2 { Goto A; Exit }     several, on one line
//	if Facing == 2 {                    or one a line
//		Save 4 SymbolSet
//		NewECL 21
//	}
//	also if Facing < 2: Goto B          the same test again, not redone
//
// with == != < > <= >= between two operands (Compare), or A == B && C == D
// and A != B || C != D (CompareAnd). It stands for the byte code's
// Compare A B, then IfEq (or the comparison's If) before each command:
// every command in a block is guarded by the one test, as a Gosub or Call
// in it may change what later ones see. "also" guards with the flags of
// the test before without a new Compare. The plain Compare and If lines
// still work.

var ifOps = map[string]string{"==": "IfEq", "!=": "IfNe", "<": "IfLt", ">": "IfGt", "<=": "IfLe", ">=": "IfGe"}

// flagSetters are the commands that change the test flags (Gosub and
// Call may).
var flagSetters = []string{"Compare", "CompareAnd", "And", "Or", "FindItem", "Gosub", "Call"}

// srcLine is one command or label of a script with its line number.
type srcLine struct {
	n     int
	toks  []string
	label bool
}

// unfoldScript reads a script into its labels and plain commands, the
// conditions written out as Compare and If commands.
func unfoldScript(text string, areas []ScriptArea) ([]srcLine, error) {
	var out []srcLine
	var last []string // the last test's operands and its kind ("Compare" or "CompareAnd")
	inBlock, guard := false, ""
	for n, line := range strings.Split(text, "\n") {
		n++
		toks, err := scriptTokens(line)
		if err != nil {
			return nil, fmt.Errorf("%d: %v", n, err)
		}
		if len(toks) == 0 {
			continue
		}
		if len(toks) == 2 && toks[1] == ":" && !unicode.IsSpace(rune(line[0])) {
			if inBlock {
				return nil, fmt.Errorf("%d: a label inside an if block", n)
			}
			out = append(out, srcLine{n: n, toks: toks[:1], label: true})
			continue
		}
		emit := func(t ...string) { out = append(out, srcLine{n: n, toks: t}) }
		// the command parts of the line, split at ; { }
		for len(toks) > 0 {
			switch t := toks[0]; {
			case t == ";":
				toks = toks[1:]
			case t == "}":
				if !inBlock {
					return nil, fmt.Errorf("%d: } without an if", n)
				}
				inBlock, toks = false, toks[1:]
			case t == "{":
				return nil, fmt.Errorf("%d: { without an if", n)
			case t == "if" || t == "also" && len(toks) > 1 && toks[1] == "if":
				if inBlock {
					return nil, fmt.Errorf("%d: an if inside an if block", n)
				}
				also := t == "also"
				if also {
					toks = toks[1:]
				}
				end := slices.IndexFunc(toks, func(s string) bool { return s == "{" || s == ":" })
				if end < 0 {
					return nil, fmt.Errorf("%d: an if without : or {", n)
				}
				cond := toks[1:end]
				block := toks[end] == "{"
				test, ifOp, err := parseCond(cond)
				if err != nil {
					return nil, fmt.Errorf("%d: %v", n, err)
				}
				if also {
					if !slices.Equal(test, last) {
						return nil, fmt.Errorf("%d: also if tests other operands than the if before", n)
					}
				} else {
					emit(append([]string{test[len(test)-1]}, test[:len(test)-1]...)...)
					last = test
				}
				guard, toks = ifOp, toks[end+1:]
				if block {
					inBlock = true
					continue
				}
				i := slices.IndexFunc(toks, func(s string) bool { return s == ";" || s == "{" || s == "}" })
				if i < 0 {
					i = len(toks)
				}
				if i == 0 {
					return nil, fmt.Errorf("%d: an if without a command", n)
				}
				for _, c := range expandMacro(toks[:i], areas) {
					emit(guard)
					emit(c...)
				}
				if slices.Contains(flagSetters, toks[0]) {
					last = nil
				}
				toks = toks[i:]
			default:
				i := slices.IndexFunc(toks, func(s string) bool { return s == ";" || s == "{" || s == "}" })
				if i < 0 {
					i = len(toks)
				}
				if slices.Contains(flagSetters, t) || strings.HasPrefix(t, "If") {
					last = nil // the flags are no longer the last if's
				}
				for _, c := range expandMacro(toks[:i], areas) {
					if inBlock {
						emit(guard)
					}
					emit(c...)
				}
				toks = toks[i:]
			}
		}
	}
	if inBlock {
		return nil, fmt.Errorf("an if block without }")
	}
	return out, nil
}

// The commands that stand for several (a guard guards each):
//
//	SetMapXY X Y      Save X MapX, Save Y MapY
//	EnterArea Name    Save <its set> SymbolSet, NewECL Name
func expandMacro(c []string, areas []ScriptArea) [][]string {
	switch {
	case c[0] == "SetMapXY" && len(c) == 3:
		return [][]string{{"Save", c[1], "MapX"}, {"Save", c[2], "MapY"}}
	case c[0] == "EnterArea" && len(c) == 2:
		for _, a := range areas {
			if a.Name == c[1] {
				return [][]string{{"Save", strconv.Itoa(a.Set), "SymbolSet"}, {"NewECL", c[1]}}
			}
		}
	}
	return [][]string{c}
}

// foldMacros writes the commands expandMacro makes as the one, and an
// area number as its name.
func foldMacros(lines []string, areas []ScriptArea) []string {
	byID := map[string]ScriptArea{}
	for _, a := range areas {
		byID[strconv.Itoa(a.ID)] = a
	}
	var out []string
	for i := 0; i < len(lines); i++ {
		ind, f := splitCmd(lines[i])
		var ind2 string
		var g []string
		if i+1 < len(lines) {
			ind2, g = splitCmd(lines[i+1])
		}
		pair := ind != "" && ind == ind2 && len(f) == 3 && len(g) >= 2 && f[0] == "Save"
		switch {
		case pair && f[2] == "MapX" && len(g) == 3 && g[0] == "Save" && g[2] == "MapY":
			out = append(out, ind+"SetMapXY "+f[1]+" "+g[1])
			i++
		case pair && f[2] == "SymbolSet" && len(g) == 2 && g[0] == "NewECL" && byID[g[1]].Name != "" && strconv.Itoa(byID[g[1]].Set) == f[1]:
			out = append(out, ind+"EnterArea "+byID[g[1]].Name)
			i++
		case len(f) == 2 && f[0] == "NewECL" && byID[f[1]].Name != "":
			out = append(out, ind+"NewECL "+byID[f[1]].Name)
		default:
			out = append(out, lines[i])
		}
	}
	return out
}

// splitCmd is a line's indent and words when it is a plain command line
// without a comment or string.
func splitCmd(l string) (string, []string) {
	t := strings.TrimLeft(l, "\t")
	if t == l || strings.ContainsAny(t, "\"/:{};") || strings.HasPrefix(t, "if ") || strings.HasPrefix(t, "also ") {
		return "", nil
	}
	return l[:len(l)-len(t)], strings.Fields(t)
}

// parseCond is a test's operands with its kind last, and its If.
func parseCond(c []string) ([]string, string, error) {
	switch {
	case len(c) == 3 && ifOps[c[1]] != "":
		return []string{c[0], c[2], "Compare"}, ifOps[c[1]], nil
	case len(c) == 7 && c[1] == "==" && c[3] == "&&" && c[5] == "==":
		return []string{c[0], c[2], c[4], c[6], "CompareAnd"}, "IfEq", nil
	case len(c) == 7 && c[1] == "!=" && c[3] == "||" && c[5] == "!=":
		return []string{c[0], c[2], c[4], c[6], "CompareAnd"}, "IfNe", nil
	}
	return nil, "", fmt.Errorf("a test is A op B, A == B && C == D or A != B || C != D: %s", strings.Join(c, " "))
}

// foldScript writes a plain script's Compare and If commands as if lines
// (cmd/eclscript); unfoldScript reads them back to the same commands.
func foldScript(text string, areas []ScriptArea) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	cmd := func(i int) (string, []string, bool) { // a command line's name and operands
		if i >= len(lines) || !strings.HasPrefix(lines[i], "\t") {
			return "", nil, false
		}
		toks, err := scriptTokens(lines[i])
		if err != nil || len(toks) == 0 {
			return "", nil, false
		}
		return toks[0], toks[1:], true
	}
	opOf := map[string]string{}
	for k, v := range ifOps {
		opOf[v] = k
	}
	var out []string
	for i := 0; i < len(lines); i++ {
		op, args, ok := cmd(i)
		test := op == "Compare" && len(args) == 2 || op == "CompareAnd" && len(args) == 4
		if !ok || !test {
			out = append(out, lines[i])
			continue
		}
		// the runs of one If before commands after the test
		type run struct {
			ifOp string
			cmds []string
		}
		var runs []run
		j := i + 1
		for {
			ifOp, _, ok1 := cmd(j)
			guarded, _, ok2 := cmd(j + 1)
			if !ok1 || !ok2 || opOf[ifOp] == "" || strings.HasPrefix(guarded, "If") {
				break
			}
			if op == "CompareAnd" && ifOp != "IfEq" && ifOp != "IfNe" {
				break
			}
			if len(runs) > 0 && runs[len(runs)-1].ifOp == ifOp {
				runs[len(runs)-1].cmds = append(runs[len(runs)-1].cmds, strings.TrimPrefix(lines[j+1], "\t"))
			} else {
				runs = append(runs, run{ifOp, []string{strings.TrimPrefix(lines[j+1], "\t")}})
			}
			j += 2
			if slices.Contains(flagSetters, guarded) {
				break
			}
		}
		if len(runs) == 0 {
			out = append(out, lines[i])
			continue
		}
		for k, r := range runs {
			var cond string
			if op == "Compare" {
				cond = args[0] + " " + opOf[r.ifOp] + " " + args[1]
			} else if r.ifOp == "IfEq" {
				cond = fmt.Sprintf("%s == %s && %s == %s", args[0], args[1], args[2], args[3])
			} else {
				cond = fmt.Sprintf("%s != %s || %s != %s", args[0], args[1], args[2], args[3])
			}
			kw := "if"
			if k > 0 {
				kw = "also if"
			}
			if len(r.cmds) == 1 {
				out = append(out, "\t"+kw+" "+cond+": "+r.cmds[0])
				continue
			}
			out = append(out, "\t"+kw+" "+cond+" {")
			for _, c := range r.cmds {
				out = append(out, "\t\t"+c)
			}
			out = append(out, "\t}")
		}
		i = j - 1
	}
	return strings.Join(foldMacros(out, areas), "\n") + "\n"
}
