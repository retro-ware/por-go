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
// with == != < > <= >= between two operands (Compare), A == B && C == D
// and A != B || C != D (CompareAnd), Carried Item and !Carried Item
// (FindItem). Right after an And or Or into D, 0 op D tests its result
// without a Compare (the flags those set compare 0 with it). It stands for the byte code's
// Compare A B, then IfEq (or the comparison's If) before each command:
// every command in a block is guarded by the one test, as a Gosub or Call
// in it may change what later ones see. "also" guards with the flags of
// the test before without a new Compare. The plain Compare and If lines
// still work.

var ifOps = map[string]string{"==": "IfEq", "!=": "IfNe", "<": "IfLt", ">": "IfGt", "<=": "IfLe", ">=": "IfGe"}

// flagSetters are the commands that change the test flags (a Gosub's
// commands may).
var flagSetters = []string{"Compare", "CompareAnd", "And", "Or", "FindItem", "Gosub"}

// srcLine is one command or label of a script with its line number.
type srcLine struct {
	n     int
	toks  []string
	label bool
}

// unfoldScript reads a script into its labels and plain commands, the
// conditions written out as Compare and If commands.
func unfoldScript(text string, names *ScriptNames) ([]srcLine, error) {
	var out []srcLine
	var last []string // the last test's operands and its kind ("Compare" or "CompareAnd")
	inBlock, guard := false, ""
	alias := map[string]string{} // the section's
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
			alias = map[string]string{}
			out = append(out, srcLine{n: n, toks: toks[:1], label: true})
			continue
		}
		if toks[0] == "alias" {
			if len(toks) != 4 || toks[2] != "=" {
				return nil, fmt.Errorf("%d: want alias Name = Variable", n)
			}
			if _, ok := alias[toks[3]]; ok {
				return nil, fmt.Errorf("%d: alias of an alias", n)
			}
			alias[toks[1]] = toks[3]
			continue
		}
		unalias(toks, alias)
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
				// 0 op D right after an And or Or into D tests its flags
				andOr := len(cond) == 3 && cond[0] == "0" && (also && len(last) == 3 && last[2] == "AndOr" ||
					!also && len(out) > 0 && !out[len(out)-1].label && slices.Contains([]string{"And", "Or"}, out[len(out)-1].toks[0]) &&
						len(out[len(out)-1].toks) == 4 && out[len(out)-1].toks[3] == cond[2])
				if andOr {
					test[2] = "AndOr"
				}
				switch {
				case also && !slices.Equal(test, last):
					return nil, fmt.Errorf("%d: also if tests other operands than the if before", n)
				case !also && !andOr:
					emit(append([]string{test[len(test)-1]}, test[:len(test)-1]...)...)
				}
				last = test
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
				for _, c := range expandMacro(toks[:i], names) {
					emit(guard)
					emit(c...)
				}
				if slices.ContainsFunc(expandMacro(toks[:i], names), func(c []string) bool { return slices.Contains(flagSetters, c[0]) }) {
					last = nil
				}
				toks = toks[i:]
			default:
				i := slices.IndexFunc(toks, func(s string) bool { return s == ";" || s == "{" || s == "}" })
				if i < 0 {
					i = len(toks)
				}
				if slices.ContainsFunc(expandMacro(toks[:i], names), func(c []string) bool {
					return slices.Contains(flagSetters, c[0]) || strings.HasPrefix(c[0], "If")
				}) {
					last = nil // the flags are no longer the last if's
				}
				for _, c := range expandMacro(toks[:i], names) {
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
	return out, resolveAsks(out)
}

// The commands that stand for several (a guard guards each):
//
//	SetMapXY X Y      Save X MapX, Save Y MapY
//	EnterArea Name    Save <its set> SymbolSet, NewECL Name
//	ShowPortrait H B  Save H Portrait, Picture B (a HEAD and BODY picture)
//
// and the assignments (scriptassign.go).
func expandMacro(c []string, names *ScriptNames) [][]string {
	if a := expandAssign(c); a != nil {
		return a
	}
	switch {
	case c[0] == "SetMapXY" && len(c) == 3:
		return [][]string{{"Save", c[1], "MapX"}, {"Save", c[2], "MapY"}}
	case c[0] == "ShowPortrait" && len(c) == 3:
		h, b := c[1], c[2]
		if v, ok := valueOf(names.Values, "Head", names.Set, h); ok {
			h = strconv.Itoa(v)
		}
		if v, ok := valueOf(names.Values, "Body", names.Set, b); ok {
			b = strconv.Itoa(v)
		}
		return [][]string{{"Save", h, "Portrait"}, {"Picture", b}}
	case c[0] == "EnterArea" && len(c) == 2:
		for _, a := range names.Areas {
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
		case pair && f[2] == "Portrait" && len(g) == 2 && g[0] == "Picture":
			out = append(out, ind+"ShowPortrait "+f[1]+" "+g[1])
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
	case len(c) == 2 && c[0] == "Carried":
		return []string{c[1], "FindItem"}, "IfEq", nil
	case len(c) == 2 && c[0] == "!Carried":
		return []string{c[1], "FindItem"}, "IfNe", nil
	}
	return nil, "", fmt.Errorf("a test is A op B, A == B && C == D, A != B || C != D or [!]Carried Item: %s", strings.Join(c, " "))
}

// foldScript writes a plain script's Compare and If commands as if lines
// (cmd/eclscript); unfoldScript reads them back to the same commands.
func foldScript(text string, names *ScriptNames) string {
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
		test := op == "Compare" && len(args) == 2 || op == "CompareAnd" && len(args) == 4 ||
			op == "FindItem" && len(args) == 1 || (op == "And" || op == "Or") && len(args) == 3
		if op == "Compare" && len(args) == 2 && args[0] == "0" {
			// after an And or Or into the operand, if 0 op D would not redo the Compare
			if p, pa, _ := cmd(i - 1); (p == "And" || p == "Or") && len(pa) == 3 && pa[2] == args[1] {
				test = false
			}
		}
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
			if (op == "CompareAnd" || op == "FindItem") && ifOp != "IfEq" && ifOp != "IfNe" {
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
		if op == "And" || op == "Or" {
			out = append(out, lines[i]) // the And stays, the ifs test its result
		}
		if len(runs) == 0 {
			if op != "And" && op != "Or" {
				out = append(out, lines[i])
			}
			continue
		}
		for k, r := range runs {
			var cond string
			switch {
			case op == "Compare":
				cond = args[0] + " " + opOf[r.ifOp] + " " + args[1]
			case op == "And" || op == "Or":
				cond = "0 " + opOf[r.ifOp] + " " + args[2]
			case op == "FindItem" && r.ifOp == "IfEq":
				cond = "Carried " + args[0]
			case op == "FindItem":
				cond = "!Carried " + args[0]
			case r.ifOp == "IfEq":
				cond = fmt.Sprintf("%s == %s && %s == %s", args[0], args[1], args[2], args[3])
			default:
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
	return strings.Join(aliasScratch(foldAsks(assignLines(foldMenus(nameValues(foldMacros(out, names.Areas), names))))), "\n") + "\n"
}

// nameValues writes the numbers of the commands' operands of a kind
// (operandKinds) as their names, and a Treasure's items as a comment. A
// Picture or close-up while a portrait head may be set (headSet) is a
// BODY picture.
func nameValues(lines []string, names *ScriptNames) []string {
	heads := headSet(lines)
	portrait, certain := false, false // a head may be set; it is, set in this section
	for i, l := range lines {
		if !strings.HasPrefix(l, "\t") {
			portrait, certain = heads[i], false // a label
			continue
		}
		code, comment := l, ""
		if k := commentAt(l); k >= 0 {
			code, comment = strings.TrimRight(l[:k], " \t"), l[k:]
		}
		toks, err := scriptTokens(code)
		if err != nil {
			continue
		}
		changed := false
		picLabel := map[int]bool{} // close-ups named as pictures under a head
		for k := 0; k < len(toks); k++ {
			if k > 0 && !slices.Contains([]string{":", "{", ";"}, toks[k-1]) {
				continue
			}
			c := toks[k]
			if c == "Save" && k+2 < len(toks) && toks[k+2] == "Portrait" {
				portrait, certain = toks[k+1] != "255", true
			}
			if c == "ShowPortrait" && k+1 < len(toks) {
				portrait, certain = toks[k+1] != "255", true // it leaves the head set
			}
			if c == "Treasure" && k+8 < len(toks) && names.Treasure != nil && comment == "" {
				switch v, err := strconv.Atoi(toks[k+8]); {
				case err != nil || v == 0xFF:
				case v < 0x80:
					comment = "// " + names.Treasure(v)
				default:
					comment = fmt.Sprintf("// %d random items", v-0x80)
				}
			}
			kinds := operandKinds[c]
			if c == "Save" && k+2 < len(toks) && toks[k+2] == "Portrait" {
				kinds = []string{"Head"}
			}
			for j, kind := range kinds {
				if a, b, ok := strings.Cut(kind, "|"); ok {
					kind = a
					if portrait {
						kind = b // a BODY picture under the head
					}
				}
				if kind == "" || k+1+j >= len(toks) {
					continue
				}
				v, err := strconv.Atoi(toks[k+1+j])
				if err != nil {
					continue
				}
				n := valueName(names.Values, kind, names.Set, v)
				if kind == "Body" {
					// the sprite's own creature, or no such body: a picture after all
					if p := valueName(names.Values, "Picture", names.Set, v); p != "" && (p == toks[k+1] || n == "" && !certain) {
						n = p
						if l := operandLabels[c]; j < len(l) && strings.Contains(l[j], "|") {
							picLabel[k+1+j] = true
						}
					}
				}
				if n != "" {
					toks[k+1+j], changed = n, true
				}
			}
			for j, l := range operandLabels[c] {
				if a, b, ok := strings.Cut(l, "|"); ok {
					l = a
					if portrait && !picLabel[k+1+j] {
						l = b
					}
				}
				switch {
				case l != "" && k+1+j < len(toks) && operandDefaults[c][j] == toks[k+1+j] && toks[k+1+j] != "":
					toks[k+1+j], changed = "\x00", true // left out, the default
				case l != "" && k+1+j < len(toks):
					toks[k+1+j], changed = l+"="+toks[k+1+j], true
				}
			}
		}
		if !changed && comment == "" || !changed && strings.HasSuffix(l, comment) {
			continue
		}
		toks = slices.DeleteFunc(toks, func(t string) bool { return t == "\x00" })
		var b strings.Builder
		b.WriteString(l[:len(l)-len(strings.TrimLeft(l, "\t"))])
		for k, t := range toks {
			if k > 0 && t != ":" && t != ";" {
				b.WriteByte(' ')
			}
			b.WriteString(t)
		}
		if comment != "" {
			b.WriteString(" " + comment)
		}
		lines[i] = b.String()
	}
	return lines
}

// headSet is, for each label line, whether a portrait head may be set
// there: after Portrait = H (or ShowPortrait H) not 255 on a way to it,
// falling through or by any jump naming it (Goto, Gosub, OnGoto ...).
// An event starts without one.
func headSet(lines []string) []bool {
	entry := map[string]bool{}
	out := make([]bool, len(lines))
	for changed := true; changed; {
		changed = false
		state := false
		for i, l := range lines {
			toks, _ := scriptTokens(l)
			if len(toks) == 0 {
				continue
			}
			if !strings.HasPrefix(l, "\t") {
				name := strings.TrimSuffix(toks[0], ":")
				if strings.HasPrefix(name, "event") {
					state = false
				}
				state = state || entry[name]
				out[i] = state
				continue
			}
			for k, t := range toks {
				switch {
				case t == "Save" && k+2 < len(toks) && toks[k+2] == "Portrait",
					t == "ShowPortrait" && k+1 < len(toks):
					state = toks[k+1] != "255"
				case state && k > 0 && !entry[t]:
					entry[t], changed = true, true // a jump target, maybe
				}
			}
			if toks[0] == "Goto" || toks[0] == "Exit" || toks[0] == "Return" {
				state = false // no falling through
			}
		}
	}
	return out
}

// commentAt is where a line's // comment starts outside strings, or -1.
func commentAt(l string) int {
	quoted := false
	for i := 0; i < len(l); i++ {
		switch {
		case l[i] == '"' && (i == 0 || l[i-1] != '\\'):
			quoted = !quoted
		case !quoted && strings.HasPrefix(l[i:], "//"):
			return i
		}
	}
	return -1
}
