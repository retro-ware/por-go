package engine

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// A variable's values and flag bits have names (names/variables.txt, the
// variable's name as the kind): a number beside the variable reads as its
// name, SelSide = PartyControlled, if Facing == South. A mask with & or |
// is its bits' names joined by + (Disguised+HallSeen), with &= the bits
// cleared as ~Name. A bit test, D = X & N and if 0 != D on it, reads as
// if X has N (lacks: if 0 == D) when D is hasScratch.

// hasScratch is the variable the bit tests write the mask into.
const hasScratch = "Scratch10"

// valueOps are the operators between a variable and its value.
var valueOps = []string{"=", "==", "!=", "<", ">", "<=", ">=", "&", "|", "&=", "|=", "has", "lacks"}

// bitOps are those of a mask.
var bitOps = []string{"&", "|", "&=", "|=", "has", "lacks"}

// varValue is the name of variable v's number n beside op, or "".
func varValue(names *ScriptNames, v, op string, n int) string {
	v = strings.TrimLeft(v, "&")
	if x := valueName(names.Values, v, names.Set, n); x != "" || !slices.Contains(bitOps, op) || n <= 0 || n > 0xFF {
		return x
	}
	bits := func(n int) string {
		var s []string
		for b := 1; b <= 0x80; b <<= 1 {
			if n&b == 0 {
				continue
			}
			x := valueName(names.Values, v, names.Set, b)
			if x == "" {
				return ""
			}
			s = append(s, x)
		}
		return strings.Join(s, "+")
	}
	if op == "&=" {
		if x := bits(0xFF ^ n); x != "" {
			return "~" + x
		}
		return ""
	}
	return bits(n)
}

// varNumber is the number a value name of variable v stands for.
func varNumber(names *ScriptNames, v, name string) (int, bool) {
	v = strings.TrimLeft(v, "&")
	if n, ok := valueOf(names.Values, v, names.Set, name); ok {
		return n, true
	}
	not := strings.HasPrefix(name, "~")
	n := 0
	for _, p := range strings.Split(strings.TrimPrefix(name, "~"), "+") {
		b, ok := valueOf(names.Values, v, names.Set, p)
		if !ok {
			return 0, false
		}
		n |= b
	}
	if not {
		n ^= 0xFF
	}
	return n, true
}

// beside calls f with each operand of toks beside a variable: the
// variable, the operator and the operand's index.
func beside(toks []string, f func(v, op string, k int)) {
	for k, t := range toks {
		switch {
		case k >= 2 && slices.Contains(valueOps, toks[k-1]) && !strings.HasPrefix(t, `"`):
			f(toks[k-2], toks[k-1], k)
		case k+2 < len(toks) && slices.Contains(valueOps[1:7], toks[k+1]):
			f(toks[k+2], toks[k+1], k) // N == V
		}
	}
}

// nameVarValues writes the numbers beside variables by their names.
func nameVarValues(lines []string, names *ScriptNames) []string {
	vars := map[string]bool{}
	for _, n := range names.Vars {
		vars[n] = true
	}
	for i, l := range lines {
		if !strings.HasPrefix(l, "\t") || strings.HasPrefix(l, "\tData ") {
			continue
		}
		code, comment := l, ""
		if k := commentAt(l); k >= 0 {
			code, comment = strings.TrimRight(l[:k], " \t"), " "+l[k:]
		}
		toks, err := scriptTokens(code)
		if err != nil {
			continue
		}
		changed := false
		beside(toks, func(v, op string, k int) {
			n, err := strconv.Atoi(toks[k])
			if err != nil {
				return
			}
			if x := varValue(names, v, op, n); x != "" && !vars[x] {
				toks[k], changed = x, true
			}
		})
		if changed {
			lines[i] = l[:len(l)-len(strings.TrimLeft(l, "\t"))] + joinTokens(toks) + comment
		}
	}
	return lines
}

// unnameVarValues puts the numbers of the value names beside variables
// back.
func unnameVarValues(toks []string, names *ScriptNames) {
	vars := map[string]bool{}
	for _, n := range names.Vars {
		vars[n] = true
	}
	beside(toks, func(v, op string, k int) {
		if t := toks[k]; t != "" && (t[0] < '0' || t[0] > '9') && !vars[t] {
			if n, ok := varNumber(names, v, t); ok {
				toks[k] = strconv.Itoa(n)
			}
		}
	})
}

var bitTest = regexp.MustCompile(`^\t` + hasScratch + ` = (&?[A-Za-z][A-Za-z0-9]*) & (\S+)$`)
var bitIf = regexp.MustCompile(`^\t(if|also if) 0 (==|!=) ` + hasScratch + `([ :{].*)$`)

// foldHas writes D = X & N and the ifs on 0 op D after it as if X has N
// (0 != D) or lacks N (0 == D).
func foldHas(lines []string) []string {
	var out []string
	for i := 0; i < len(lines); i++ {
		m := bitTest.FindStringSubmatch(lines[i])
		if m == nil || i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], "\tif ") || bitIf.FindStringSubmatch(lines[i+1]) == nil {
			out = append(out, lines[i])
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			l := lines[j]
			if strings.HasPrefix(l, "\t\t") || l == "\t}" {
				out = append(out, l)
				continue
			}
			c := bitIf.FindStringSubmatch(l)
			if c == nil || j > i+1 && c[1] == "if" {
				i = j - 1
				break
			}
			test := " has "
			if c[2] == "==" {
				test = " lacks "
			}
			out = append(out, "\t"+c[1]+" "+m[1]+test+m[2]+c[3])
			i = j
		}
	}
	return out
}

// A loop, X = A then at L: ... X += 1 and if X < B+1: Goto L, nothing
// else jumping to L, reads as
//
//	L:
//		for X = A to B
//			...
//		next
//
// (for X = A while X op B for another test). The A goes before L.

// foldLoops writes the loops as for ... next.
func foldLoops(lines []string) []string {
	refs := map[string]int{}
	for _, l := range lines {
		if strings.HasPrefix(l, "\t") {
			toks, _ := scriptTokens(l)
			for _, t := range toks {
				refs[t]++
			}
		}
	}
	for li := 1; li+1 < len(lines); li++ {
		label := strings.TrimSuffix(lines[li], ":")
		if strings.HasPrefix(lines[li], "\t") || label == lines[li] || strings.HasPrefix(label, "event") || refs[label] != 1 ||
			!strings.HasPrefix(lines[li+1], "\t") {
			continue
		}
		pi := li - 1
		for pi >= 0 && lines[pi] == "" {
			pi--
		}
		init, _ := scriptTokens(lines[pi])
		if pi < 0 || strings.HasPrefix(lines[pi], "\t\t") || !strings.HasPrefix(lines[pi], "\t") || commentAt(lines[pi]) >= 0 ||
			len(init) != 3 || init[1] != "=" {
			continue
		}
		x := init[0]
		j := li + 1
		for j+1 < len(lines) && strings.HasPrefix(lines[j], "\t") && lines[j] != "\t"+x+" += 1" {
			j++
		}
		if j+1 >= len(lines) || lines[j] != "\t"+x+" += 1" || commentAt(lines[j+1]) >= 0 {
			continue
		}
		t, _ := scriptTokens(lines[j+1])
		if len(t) != 7 || t[0] != "if" || t[1] != x || !slices.Contains(valueOps[1:7], t[2]) || t[4] != ":" || t[5] != "Goto" || t[6] != label {
			continue
		}
		head := "\tfor " + x + " = " + init[2] + " while " + x + " " + t[2] + " " + t[3]
		if b, err := strconv.Atoi(t[3]); err == nil && t[2] == "<" {
			head = "\tfor " + x + " = " + init[2] + " to " + strconv.Itoa(b-1)
		}
		var body []string
		for _, l := range lines[li+1 : j] {
			body = append(body, "\t"+l)
		}
		out := slices.Concat(lines[:pi], lines[pi+1:li+1], []string{head}, body, []string{"\tnext"}, lines[j+2:])
		lines = out
		li = j // past the loop (one line shorter)
	}
	return lines
}

// loopLines are the plain lines of a for line (its X = A is put before
// the label) or a next line, or nil.
type loop struct{ x, op, b, label string }

// forLoop reads a for line: for X = A to B, for X = A while X op B.
func forLoop(toks []string) (loop, string, bool) {
	switch {
	case len(toks) == 6 && toks[2] == "=" && toks[4] == "to":
		b, err := strconv.Atoi(toks[5])
		if err != nil {
			return loop{}, "", false
		}
		return loop{toks[1], "<", strconv.Itoa(b + 1), ""}, toks[3], true
	case len(toks) == 8 && toks[2] == "=" && toks[4] == "while" && toks[5] == toks[1] && slices.Contains(valueOps[1:7], toks[6]):
		return loop{toks[1], toks[6], toks[7], ""}, toks[3], true
	}
	return loop{}, "", false
}

// foldShops writes the shop, its stock (Treasure's ITEM block) by item
// names, then Shop = 1, ShopFlag = 1, ShopKind = 16 and Combat, as
// OpenShop and the items on stock lines; Temple = 1 and Combat as
// OpenTemple.
func foldShops(lines []string, names *ScriptNames) []string {
	var out []string
	for i := 0; i < len(lines); i++ {
		if i+1 < len(lines) && lines[i] == "\tTemple = 1" && lines[i+1] == "\tCombat" {
			out = append(out, "\tOpenTemple")
			i++
			continue
		}
		code := lines[i]
		if k := commentAt(code); k >= 0 {
			code = strings.TrimRight(code[:k], " ")
		}
		b, ok := strings.CutPrefix(code, "\tTreasure 0 0 0 0 0 0 0 ")
		v, err := strconv.Atoi(b)
		stock := valueName(names.Values, "Stock", names.Set, v)
		if !ok || err != nil || stock == "" || i+4 >= len(lines) ||
			!slices.Equal(lines[i+1:i+5], []string{"\tShop = 1", "\tShopFlag = 1", "\tShopKind = 16", "\tCombat"}) {
			out = append(out, lines[i])
			continue
		}
		out = append(out, "\tOpenShop")
		items := strings.Split(stock, ",")
		for len(items) > 0 {
			n := min(6, len(items))
			out = append(out, "\t\tstock "+strings.Join(items[:n], " "))
			items = items[n:]
		}
		i += 4
	}
	return out
}

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

// nameData writes the entries of the tables of a kind by name.
func nameData(lines []string, names *ScriptNames) []string {
	kind := ""
	for i, l := range lines {
		if !strings.HasPrefix(l, "\t") {
			if l != "" {
				kind = tableKind(strings.TrimSuffix(l, ":"))
			}
			continue
		}
		if kind == "" || !strings.HasPrefix(l, "\tData ") {
			continue
		}
		f := strings.Fields(l)
		for k, t := range f[1:] {
			if v, err := strconv.Atoi(t); err == nil {
				if n := valueName(names.Values, kind, names.Set, v); n != "" {
					f[k+1] = n
				}
			}
		}
		lines[i] = "\t" + strings.Join(f, " ")
	}
	return lines
}

// compareSections are the plain script's sections that end in Compare A
// B and their one Return, without a jump out: a Gosub of one tests A op
// B after it, written if Gosub M then A op B. Their A and B by name.
func compareSections(lines []string) map[string][]string {
	out := map[string][]string{}
	for i := 0; i < len(lines); {
		var names []string
		for i < len(lines) && !strings.HasPrefix(lines[i], "\t") {
			if lines[i] != "" {
				names = append(names, strings.TrimSuffix(lines[i], ":"))
			}
			i++
		}
		j := i
		ok := true
		for j < len(lines) && strings.HasPrefix(lines[j], "\t") {
			t, _ := scriptTokens(lines[j])
			if slices.ContainsFunc(t, func(s string) bool { return s == "Goto" || s == "OnGoto" || s == "Return" || s == "Exit" }) &&
				!(lines[j] == "\tReturn" && j+1 < len(lines) && !strings.HasPrefix(lines[j+1], "\t")) {
				ok = false
			}
			j++
		}
		if ok && j-i >= 2 && lines[j-1] == "\tReturn" {
			if t, _ := scriptTokens(lines[j-2]); len(t) == 3 && t[0] == "Compare" && commentAt(lines[j-2]) < 0 {
				for _, n := range names {
					out[n] = t[1:]
				}
			}
		}
		if j == i {
			j++
		}
		i = j
	}
	return out
}
