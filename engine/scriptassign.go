package engine

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The scripts' assignments, for the commands that compute a variable:
//
//	D = V                 Save V D (a number, variable or string)
//	D = A + B             Add A B D; also * (Mul), / (Div), & (And), | (Or)
//	D = A - B             Sub B A D (Sub subtracts its first operand)
//	D += A, D -= A        Add A D D, Sub A D D
//	D = T[I]              GetTable T I D
//	T[I] = V              SaveTable V T I
//	D = Random N          Random N D (0..N)
//	D = Menu "A" "B"      HMenu D 2 "A" "B"
//	D = Menu "A" -> L1 "B" -> L2   HMenu D 2 "A" "B", OnGoto D 2 L1 L2
//	D = VMenu "P" "A" "B"  VMenu D "P" 2 "A" "B"
//	OnGoto D L1 L2        OnGoto D 2 L1 L2 (the count from the labels), OnGosub too
//	Ask M "YES" -> L1 "NO" -> L2   Gosub M, OnGoto D 2 L1 L2: M is a section
//	                      that only asks D = Menu "YES" "NO" and returns (an
//	                      answer without -> goes on with the next line)
//
// and the section aliases: "alias Choice = Scratch1" under a label names
// the variable Choice up to the next label.

var arithOps = map[string]string{"+": "Add", "-": "Sub", "*": "Mul", "/": "Div", "&": "And", "|": "Or"}

// tableRef splits T[I] into T and I.
func tableRef(t string) (string, string, bool) {
	i := strings.IndexByte(t, '[')
	if i <= 0 || !strings.HasSuffix(t, "]") || strings.HasPrefix(t, `"`) {
		return "", "", false
	}
	return t[:i], t[i+1 : len(t)-1], true
}

// destLast are the commands that put their result in their last
// operand, written D = Command operands.
var destLast = []string{"PartyStrength", "InputNumber", "InputString", "Parlay"}

// expandAssign is the plain commands of an assignment or count-less
// OnGoto, or nil.
func expandAssign(c []string) [][]string {
	n := strconv.Itoa
	if (c[0] == "OnGoto" || c[0] == "OnGosub") && len(c) >= 3 {
		if k, err := strconv.Atoi(c[2]); err == nil && k == len(c)-3 {
			return nil // the plain form
		}
		return [][]string{append([]string{c[0], c[1], n(len(c) - 2)}, c[2:]...)}
	}
	if c[0] == "Ask" && len(c) >= 2 {
		var labels []string
		for k := 2; k+2 < len(c); k++ {
			if c[k+1] == "->" {
				labels = append(labels, c[k+2])
				k += 2
			}
		}
		// askMenu: the answer's variable, from M's menu (resolveAsks)
		return [][]string{{"Gosub", c[1]}, append([]string{"OnGoto", askMenu + c[1] + " " + strings.Join(c[2:], " "), n(len(labels))}, labels...)}
	}
	if len(c) < 3 || !slices.Contains([]string{"=", "+=", "-=", "|=", "&="}, c[1]) {
		return nil
	}
	d, r := c[0], c[2:]
	if t, i, ok := tableRef(d); ok && len(r) == 1 {
		return [][]string{{"SaveTable", r[0], t, i}}
	}
	switch {
	case slices.Contains(destLast, r[0]):
		return [][]string{append(slices.Clone(r), d)}
	case len(c) == 3 && (c[1] == "+=" || c[1] == "-="):
		return [][]string{{arithOps[c[1][:1]], c[2], d, d}}
	case len(c) == 3 && (c[1] == "|=" || c[1] == "&="):
		return [][]string{{arithOps[c[1][:1]], d, c[2], d}}
	case c[1] != "=":
		return nil
	case len(r) == 1:
		if t, i, ok := tableRef(r[0]); ok {
			return [][]string{{"GetTable", t, i, d}}
		}
		return [][]string{{"Save", r[0], d}}
	case len(r) == 3 && r[1] == "-":
		return [][]string{{"Sub", r[2], r[0], d}}
	case len(r) == 3 && arithOps[r[1]] != "":
		return [][]string{{arithOps[r[1]], r[0], r[2], d}}
	case len(r) == 2 && r[0] == "Random":
		return [][]string{{"Random", r[1], d}}
	case r[0] == "VMenu" && len(r) >= 2:
		return [][]string{append([]string{"VMenu", d, r[1], n(len(r) - 2)}, r[2:]...)}
	case r[0] == "Menu":
		var items, labels []string
		for k := 1; k < len(r); k++ {
			items = append(items, r[k])
			if k+2 < len(r) && r[k+1] == "->" {
				labels = append(labels, r[k+2])
				k += 2
			}
		}
		out := [][]string{append([]string{"HMenu", d, n(len(items))}, items...)}
		if len(labels) > 0 {
			out = append(out, append([]string{"OnGoto", d, n(len(labels))}, labels...))
		}
		return out
	}
	return nil
}

// assignForm writes a plain command as its assignment, or returns it.
func assignForm(c []string) []string {
	j := func(s ...string) []string { return s }
	cnt := func(i int) bool { k, err := strconv.Atoi(c[i]); return err == nil && k == len(c)-i-1 }
	switch {
	case c[0] == "Save" && len(c) == 3:
		return j(c[2], "=", c[1])
	case (c[0] == "Add" || c[0] == "Sub") && len(c) == 4 && c[2] == c[3]:
		return j(c[3], map[string]string{"Add": "+=", "Sub": "-="}[c[0]], c[1])
	case (c[0] == "And" || c[0] == "Or") && len(c) == 4 && c[1] == c[3]:
		return j(c[3], map[string]string{"And": "&=", "Or": "|="}[c[0]], c[2])
	case c[0] == "Sub" && len(c) == 4:
		return j(c[3], "=", c[2], "-", c[1])
	case len(c) == 4 && slices.Contains([]string{"Add", "Mul", "Div", "And", "Or"}, c[0]):
		for s, op := range arithOps {
			if op == c[0] {
				return j(c[3], "=", c[1], s, c[2])
			}
		}
	case c[0] == "GetTable" && len(c) == 4:
		return j(c[3], "=", c[1]+"["+c[2]+"]")
	case c[0] == "SaveTable" && len(c) == 4:
		return j(c[2]+"["+c[3]+"]", "=", c[1])
	case slices.Contains(destLast, c[0]) && len(c) >= 2:
		return append(j(c[len(c)-1], "=", c[0]), c[1:len(c)-1]...)
	case c[0] == "Random" && len(c) == 3:
		return j(c[2], "=", "Random", c[1])
	case c[0] == "HMenu" && len(c) >= 3 && cnt(2):
		return append(j(c[1], "=", "Menu"), c[3:]...)
	case c[0] == "VMenu" && len(c) >= 4 && cnt(3):
		return append(j(c[1], "=", "VMenu", c[2]), c[4:]...)
	case (c[0] == "OnGoto" || c[0] == "OnGosub") && len(c) >= 3 && cnt(2):
		return append(j(c[0], c[1]), c[3:]...)
	}
	return c
}

// foldMenus writes an unguarded HMenu D N and OnGoto D N on its answer
// as one Menu with the labels.
func foldMenus(lines []string) []string {
	var out []string
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if i+1 < len(lines) && strings.HasPrefix(l, "\t") && !strings.HasPrefix(l, "\t\t") && commentAt(l) < 0 && commentAt(lines[i+1]) < 0 {
			a, _ := scriptTokens(l)
			b, _ := scriptTokens(lines[i+1])
			if len(a) >= 3 && a[0] == "HMenu" && len(b) >= 3 && b[0] == "OnGoto" && a[1] == b[1] && a[2] == b[2] &&
				len(a) == len(b) && !strings.HasPrefix(lines[i+1], "\t\t") && !slices.ContainsFunc(append(a, b...), func(s string) bool { return s == ":" || s == ";" || s == "{" || s == "}" }) {
				s := "\t" + a[1] + " = Menu"
				for k := 3; k < len(a); k++ {
					s += " " + a[k] + " -> " + b[k]
				}
				out = append(out, s)
				i++
				continue
			}
		}
		out = append(out, l)
	}
	return out
}

// assignLines writes each command of the lines in its assignment form.
func assignLines(lines []string) []string {
	for i, l := range lines {
		if !strings.HasPrefix(l, "\t") {
			continue
		}
		code, comment := l, ""
		if k := commentAt(l); k >= 0 {
			code, comment = strings.TrimRight(l[:k], " \t"), " "+l[k:]
		}
		toks, err := scriptTokens(code)
		if err != nil || len(toks) == 0 {
			continue
		}
		var out []string
		for k := 0; k < len(toks); {
			if toks[k] == ":" || toks[k] == "{" || toks[k] == ";" || toks[k] == "}" {
				out = append(out, toks[k])
				k++
				continue
			}
			end := slices.IndexFunc(toks[k:], func(s string) bool { return s == ":" || s == "{" || s == ";" || s == "}" })
			if end < 0 {
				end = len(toks) - k
			}
			seg := toks[k : k+end]
			if seg[0] != "if" && seg[0] != "also" {
				seg = assignForm(seg)
			}
			out = append(out, seg...)
			k += end
		}
		lines[i] = l[:len(l)-len(strings.TrimLeft(l, "\t"))] + joinTokens(out) + comment
	}
	return lines
}

// joinTokens writes tokens back as a line.
func joinTokens(toks []string) string {
	var b strings.Builder
	for k, t := range toks {
		if k > 0 && t != ":" && t != ";" {
			b.WriteByte(' ')
		}
		b.WriteString(t)
	}
	return b.String()
}

var hexTable = regexp.MustCompile(`^Table[0-9A-F]+$`) // an unnamed table

var scratchName = regexp.MustCompile(`^(Scratch\d+|Aux[A-Za-z0-9]*)$`) // the temporary words

// aliasScratch names a section's scratch variable by its one role there:
// Choice for a menu's answer, a table's entry by the table (Kinds[I] into
// it: Kind). The alias is a word the script does not use.
func aliasScratch(lines []string) []string {
	used := map[string]bool{}
	for _, l := range lines {
		toks, _ := scriptTokens(l)
		for _, t := range toks {
			used[strings.TrimSuffix(t, ":")] = true
			if tb, ix, ok := tableRef(t); ok {
				used[tb], used[ix] = true, true
			}
		}
	}
	var out []string
	for i := 0; i < len(lines); {
		j := i
		for j < len(lines) && !strings.HasPrefix(lines[j], "\t") {
			j++ // the section's labels
		}
		k := j
		for k < len(lines) && (strings.HasPrefix(lines[k], "\t") || lines[k] == "") {
			k++
		}
		roles := map[string]map[string]bool{}
		var order []string            // the variables, in order
		uses := map[string][]string{} // the roles of their uses
		for _, l := range lines[j:k] {
			toks, _ := scriptTokens(l)
			for p := 0; p+2 < len(toks); p++ {
				if toks[p+1] != "=" || !scratchName.MatchString(toks[p]) || p > 0 && !slices.Contains([]string{":", "{", ";"}, toks[p-1]) {
					continue
				}
				role := ""
				if toks[p+2] == "Menu" || toks[p+2] == "VMenu" {
					role = "Choice"
				} else if tb, _, ok := tableRef(toks[p+2]); ok && !hexTable.MatchString(tb) && (p+3 == len(toks) || slices.Contains([]string{";", "}"}, toks[p+3])) {
					role = strings.TrimSuffix(strings.TrimSuffix(tb, "Table"), "s")
					if role == tb {
						role += "Entry"
					}
				}
				if roles[toks[p]] == nil {
					roles[toks[p]] = map[string]bool{}
					order = append(order, toks[p])
				}
				roles[toks[p]][role] = true
			}
			// a table entry's role by its use: a LoadMonster's monster, icon or count
			if p := slices.Index(toks, "LoadMonster"); p >= 0 && p+1 < len(toks) {
				for _, t := range toks[p+1:] {
					l, v, ok := strings.Cut(t, "=")
					if !ok {
						l, v = "Monster", t
					}
					if l == "Monster" || l == "icon" || l == "count" {
						uses[v] = append(uses[v], strings.ToUpper(l[:1])+l[1:])
					}
				}
			}
		}
		alias := map[string]string{}
		taken := map[string]bool{}
		var vars []string
		byRole := map[string][]string{}
		for _, v := range order {
			rs := roles[v]
			if len(rs) != 1 || rs[""] {
				continue
			}
			r := ""
			for r = range rs {
			}
			if u := uses[v]; len(u) > 0 && r != "Choice" && !slices.ContainsFunc(u, func(s string) bool { return s != u[0] }) {
				r = u[0]
			}
			byRole[r] = append(byRole[r], v)
		}
		for r, vs := range byRole {
			for n, v := range vs {
				a := r
				if len(vs) > 1 {
					a += strconv.Itoa(n + 1)
				}
				if !used[a] && !taken[a] {
					alias[v], taken[a] = a, true
					vars = append(vars, v)
				}
			}
		}
		slices.SortFunc(vars, func(a, b string) int { return strings.Compare(alias[a], alias[b]) })
		out = append(out, lines[i:j]...)
		if j > i && len(vars) > 0 {
			for _, v := range vars {
				out = append(out, "\talias "+alias[v]+" = "+v)
			}
			for _, l := range lines[j:k] {
				out = append(out, renameWords(l, alias))
			}
		} else {
			out = append(out, lines[j:k]...)
		}
		if k == i {
			k++
		}
		i = k
	}
	return out
}

var wordRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// renameWords renames the words of a line's code outside strings.
func renameWords(l string, m map[string]string) string {
	code, comment := l, ""
	if k := commentAt(l); k >= 0 {
		code, comment = l[:k], l[k:]
	}
	parts := strings.Split(code, `"`) // odd parts are inside strings (no \" in the scripts' texts)
	for p := 0; p < len(parts); p += 2 {
		parts[p] = wordRe.ReplaceAllStringFunc(parts[p], func(w string) string {
			if r, ok := m[w]; ok {
				return r
			}
			return w
		})
	}
	return strings.Join(parts, `"`) + comment
}

// unalias replaces a section's aliases in a line's tokens.
func unalias(toks []string, m map[string]string) {
	for i, t := range toks {
		if l, v, ok := strings.Cut(t, "="); ok && l != "" && !strings.HasPrefix(t, `"`) {
			w := []string{v}
			unalias(w, m)
			toks[i] = l + "=" + w[0] // a labelled operand
			continue
		}
		pre := strings.TrimLeft(t, "&$")
		if r, ok := m[pre]; ok {
			toks[i] = t[:len(t)-len(pre)] + r
		} else if tb, ix, ok := tableRef(t); ok {
			if r, ok := m[tb]; ok {
				tb = r
			}
			if r, ok := m[ix]; ok {
				ix = r
			}
			toks[i] = tb + "[" + ix + "]"
		}
	}
}

// askMenu marks an Ask's OnGoto operand, askIf the If of an "if Ask M ==
// ANSWER" test, until resolveAsks knows M's menu.
const askMenu, askIf = "?ask:", "?askif:"

// askSections are the plain script's sections that ask a two-answer
// menu D = Menu "A" "B", maybe print, then Compare D 0 and return: a
// Gosub of one tests the answer (IfEq: the first). Their answers by name.
func askSections(lines []string) map[string][]string {
	out := map[string][]string{}
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "\t") || !strings.HasSuffix(lines[i], ":") {
			continue
		}
		var names []string
		for i < len(lines) && lines[i] != "" && !strings.HasPrefix(lines[i], "\t") {
			names = append(names, strings.TrimSuffix(lines[i], ":"))
			i++
		}
		if i >= len(lines) {
			break
		}
		m, _ := scriptTokens(lines[i])
		if len(m) != 5 || m[0] != "HMenu" || m[2] != "2" {
			continue
		}
		j := i + 1
		for j < len(lines) && (strings.HasPrefix(lines[j], "\tPrint ") || strings.HasPrefix(lines[j], "\tPrintClear ")) {
			j++
		}
		if j+1 < len(lines) && lines[j] == "\tCompare "+m[1]+" 0" && lines[j+1] == "\tReturn" {
			for _, n := range names {
				out[n] = m[3:]
			}
		}
	}
	return out
}

// resolveAsks puts the answer variable of each Ask's menu section in.
func resolveAsks(out []srcLine) error {
	for i, sl := range out {
		if !sl.label && strings.HasPrefix(sl.toks[0], askIf) {
			if err := resolveAskIf(out, i); err != nil {
				return err
			}
			continue
		}
		if sl.label || len(sl.toks) < 2 || !strings.HasPrefix(sl.toks[1], askMenu) {
			continue
		}
		name, items, _ := strings.Cut(strings.TrimPrefix(sl.toks[1], askMenu), " ")
		at := slices.IndexFunc(out, func(x srcLine) bool { return x.label && x.toks[0] == name })
		for at >= 0 && at < len(out) && out[at].label {
			at++
		}
		if at < 0 || at+1 >= len(out) || out[at].toks[0] != "HMenu" || out[at+1].toks[0] != "Return" {
			return fmt.Errorf("%d: Ask %s: %s is not a menu section (D = Menu ...; Return)", sl.n, name, name)
		}
		var said []string
		toks, _ := scriptTokens(items)
		for k := 0; k < len(toks); k++ {
			said = append(said, toks[k])
			if k+2 < len(toks) && toks[k+1] == "->" {
				k += 2
			}
		}
		if !slices.Equal(said, out[at].toks[3:]) {
			return fmt.Errorf("%d: Ask %s: its menu is %s", sl.n, name, strings.Join(out[at].toks[3:], " "))
		}
		out[i].toks[1] = out[at].toks[1]
	}
	return nil
}

// foldAsks writes Gosub M and OnGoto D on its answer as an Ask, when M
// only asks D = Menu ... and returns.
func foldAsks(lines []string) []string {
	menus := map[string][]string{} // a menu section's D and items
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "\t") || !strings.HasSuffix(lines[i], ":") {
			continue
		}
		name := strings.TrimSuffix(lines[i], ":")
		j := i + 1
		for j < len(lines) && lines[j] != "" && !strings.HasPrefix(lines[j], "\t") {
			j++ // more labels
		}
		if j+1 < len(lines) && lines[j+1] == "\tReturn" && commentAt(lines[j]) < 0 {
			if t, _ := scriptTokens(lines[j]); len(t) >= 4 && t[1] == "=" && t[2] == "Menu" && !slices.Contains(t, "->") {
				menus[name] = append([]string{t[0]}, t[3:]...)
			}
		}
	}
	var out []string
	for i := 0; i < len(lines); i++ {
		if i+1 < len(lines) && strings.HasPrefix(lines[i], "\tGosub ") && strings.HasPrefix(lines[i+1], "\tOnGoto ") && commentAt(lines[i+1]) < 0 {
			g, _ := scriptTokens(lines[i])
			o, _ := scriptTokens(lines[i+1])
			if m := menus[g[1]]; len(g) == 2 && m != nil && len(o) >= 3 && o[1] == m[0] && len(o)-2 <= len(m)-1 &&
				!slices.ContainsFunc(o, func(s string) bool { return s == ":" || s == ";" || s == "{" || s == "}" }) {
				s := "\tAsk " + g[1]
				for k, item := range m[1:] {
					s += " " + item
					if k+2 < len(o) {
						s += " -> " + o[k+2]
					}
				}
				out = append(out, s)
				i++
				continue
			}
		}
		out = append(out, lines[i])
	}
	return out
}

// resolveAskIf makes the If out[i] of an "if Ask M == ANSWER" IfEq for
// M's first answer, IfNe for its second, and the test before it Gosub M.
func resolveAskIf(out []srcLine, i int) error {
	name, answer, _ := strings.Cut(strings.TrimPrefix(out[i].toks[0], askIf), " ")
	at := slices.IndexFunc(out, func(x srcLine) bool { return x.label && x.toks[0] == name })
	for at >= 0 && at < len(out) && out[at].label {
		at++
	}
	if at < 0 || at >= len(out) || out[at].toks[0] != "HMenu" || len(out[at].toks) != 5 {
		return fmt.Errorf("%d: Ask %s: %s is not a two-answer menu section", out[i].n, name, name)
	}
	j := at + 1
	for j < len(out) && (out[j].toks[0] == "Print" || out[j].toks[0] == "PrintClear") {
		j++
	}
	if j+1 >= len(out) || !slices.Equal(out[j].toks, []string{"Compare", out[at].toks[1], "0"}) || out[j+1].toks[0] != "Return" {
		return fmt.Errorf("%d: Ask %s: %s does not end in Compare %s 0 and Return", out[i].n, name, name, out[at].toks[1])
	}
	switch answer {
	case out[at].toks[3]:
		out[i].toks = []string{"IfEq"}
	case out[at].toks[4]:
		out[i].toks = []string{"IfNe"}
	default:
		return fmt.Errorf("%d: Ask %s: its answers are %s and %s", out[i].n, name, out[at].toks[3], out[at].toks[4])
	}
	for k := i - 1; k >= 0; k-- {
		if slices.Equal(out[k].toks, []string{"Ask", name}) {
			out[k].toks = []string{"Gosub", name}
			break
		}
	}
	return nil
}
