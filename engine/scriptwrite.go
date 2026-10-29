package engine

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// WriteScript writes s as the script text (cmd/eclscript). ParseScript
// reads it back to the same Script.
func WriteScript(s *Script, names *ScriptNames) (string, error) {
	w := &scriptWriter{s: s, names: names, taken: map[string]bool{}, table: map[int]string{},
		labelsAt: map[int][]string{}, refs: map[int][]ref{}, events: map[int]int{}, defs: map[int]bool{}}
	for _, n := range names.Vars {
		w.taken[n] = true
	}
	for n, at := range s.Tables {
		w.table[at] = n
		w.taken[n] = true
	}
	for n := range s.Labels {
		w.taken[n] = true
	}
	for op := range 0x40 {
		w.taken[commandName(byte(op))] = true
	}
	for n, i := range s.Labels {
		w.labelsAt[i] = append(w.labelsAt[i], n)
	}
	for i, ls := range w.labelsAt {
		sort.Slice(ls, func(a, b int) bool { // the first label first, events last
			ea, eb := strings.HasPrefix(ls[a], "event"), strings.HasPrefix(ls[b], "event")
			if ea != eb {
				return eb
			}
			return (ls[a] == s.At[i]) || ls[a] < ls[b] && ls[b] != s.At[i]
		})
	}
	for i, e := range s.Events {
		w.events[e] = i
	}
	for i, in := range s.Code {
		for _, a := range in.Args {
			if a.Label {
				w.refs[a.N] = append(w.refs[a.N], ref{i, in.Op})
			}
		}
	}
	for i := range s.Code {
		if _, ok := w.events[i]; ok || len(w.labelsAt[i]) == 0 || len(w.refs[i]) == 0 {
			continue
		}
		if slices.ContainsFunc(w.refs[i], func(r ref) bool { return r.op != opGosub && r.op != opOnGosub }) {
			continue
		}
		if i == 0 || isTerminator(s.Code[i-1].Op) && !(i > 1 && isIf(s.Code[i-2].Op)) {
			w.defs[i] = true
		}
	}
	w.before = scriptFlags(s)
	w.head = s.headSet()
	w.returnForms()
	for i := 0; i < len(s.Code); {
		end := w.blockEnd(i)
		var idx []int
		for j := i; j < end; j++ {
			idx = append(idx, j)
		}
		body, err := w.stmts(idx, false)
		if err != nil {
			return "", err
		}
		if w.b.Len() > 0 {
			w.b.WriteString("\n")
		}
		labels := w.labelsAt[i]
		switch {
		case w.events[i] != 0 || w.events[i] == 0 && slices.Contains(labels, eventLabel(0)):
			fmt.Fprintf(&w.b, "on %s {\n", eventNames[w.events[i]])
			labels = slices.DeleteFunc(slices.Clone(labels), func(l string) bool { return strings.HasPrefix(l, "event") })
		case w.defs[i]:
			fmt.Fprintf(&w.b, "def %s() {\n", s.At[i])
			labels = slices.DeleteFunc(slices.Clone(labels), func(l string) bool { return l == s.At[i] })
		default:
			for _, l := range labels {
				w.b.WriteString(l + ":\n")
			}
			labels = nil
		}
		for _, l := range labels {
			w.b.WriteString(indent + l + ":\n")
		}
		for _, l := range body {
			if l == "" {
				w.b.WriteString("\n")
			} else {
				w.b.WriteString(indent + l + "\n")
			}
		}
		if _, ok := w.events[i]; ok || w.defs[i] {
			if last := s.Code[end-1]; end < len(s.Code) && !(isTerminator(last.Op) && !(end > 1 && isIf(s.Code[end-2].Op))) {
				fmt.Fprintf(&w.b, "%sgoto %s\n", indent, s.At[end]) // the byte code falls through; the parser leaves this jump out
			}
			w.b.WriteString("}\n")
		}
		i = end
	}
	if len(s.Tables) > 0 {
		w.b.WriteString("\n# tables\n")
		var ats []int
		for _, at := range s.Tables {
			ats = append(ats, at)
		}
		sort.Ints(ats)
		for k, at := range ats {
			end := len(s.Data) + eclBase
			if k+1 < len(ats) {
				end = ats[k+1]
			}
			name := w.table[at]
			kind := tableKind(name)
			var vs []string
			for _, b := range s.Data[at-eclBase : end-eclBase] {
				vs = append(vs, w.number(int(b), kind, Arg{}, ""))
			}
			fmt.Fprintf(&w.b, "%s = [%s]\n", name, strings.Join(vs, ", "))
		}
	}
	return w.b.String(), nil
}

const indent = "    "

// ref is a jump to a label: the command and its opcode.
type ref struct {
	from int
	op   byte
}

type scriptWriter struct {
	s        *Script
	names    *ScriptNames
	taken    map[string]bool // the names a value name must not be (variables, labels ...)
	table    map[int]string  // a table's name by address
	labelsAt map[int][]string
	refs     map[int][]ref
	events   map[int]int  // an event's number by its command
	defs     map[int]bool // the commands that start a def
	before   []flagState
	head     []bool            // whether a portrait head is up at each command
	callOp   map[int]byte      // by a called label: the If of its first tested call
	retState map[int]flagState // by a called label: the flags it returns with
	valueDef map[int]bool      // the called labels whose returns are return X
	retOp    map[int]byte      // by a return's Compare: the operator its callers test
	b        strings.Builder
}

// returnForms finds how the called sections return their result: return X
// (valueDef: the callers compare X, if Foo() op N) when every return
// compares the same variable with the same number, or else return A op B
// (if Foo()) with the operator the first tested call uses (retOp, on
// every return a walk from the called label reaches).
func (w *scriptWriter) returnForms() {
	w.callOp, w.retState, w.valueDef, w.retOp = map[int]byte{}, map[int]flagState{}, map[int]bool{}, map[int]byte{}
	s := w.s
	var called []int
	for i, in := range s.Code {
		if in.Op == opGosub && len(in.Args) == 1 && in.Args[0].Label && i+1 < len(s.Code) && isIf(s.Code[i+1].Op) {
			if _, ok := w.callOp[in.Args[0].N]; !ok {
				w.callOp[in.Args[0].N] = s.Code[i+1].Op
				called = append(called, in.Args[0].N)
			}
		}
	}
	sort.Ints(called)
	for _, l := range called {
		op := w.callOp[l]
		s.returnsFrom(l, func(_, c int) bool {
			if c >= 0 && s.Code[c].Op == opCompare {
				if _, ok := w.retOp[c]; !ok {
					w.retOp[c] = op
				}
			}
			return false
		})
		st, changed := s.returnFlags(l, map[int]bool{})
		_, c := s.firstReturn(l)
		if !changed || c < 0 {
			continue
		}
		isVar := func(a Arg) bool { return (a.Type == 1 || a.Type == 3) && !a.Label }
		in := s.Code[c] // the first return's comparison is the one
		switch {
		case st.kind == opCompare && isVar(st.args[0]) && st.args[1].Type != 0x80 && st.args[1].Type != 0x81 && !isVar(st.args[1]) &&
			in.Op == opCompare && slices.Equal(in.Args, st.args) && w.sectionOf(c) == l, // a return of its own: return X, else return X op N reads better
			st.kind == opAnd && in.Op != opCompare && in.Args[2] == st.args[1]:
			w.retState[l], w.valueDef[l] = st, true
		}
	}
}

// textOp is the operator the text shows for the Compare at c before a
// return: the if's when it is if A op B { return }, else its return A op
// B's (retOp, == when no call tests it).
func (w *scriptWriter) textOp(c int) byte {
	if c+1 < len(w.s.Code) && isIf(w.s.Code[c+1].Op) && !(c > 0 && isIf(w.s.Code[c-1].Op)) {
		return w.s.Code[c+1].Op
	}
	if op, ok := w.retOp[c]; ok {
		return op
	}
	return opIfEq
}

// sectionEnd is where the section at label l ends for its returns: a
// def's or event's block, else the next label.
func (w *scriptWriter) sectionEnd(l int) int {
	if _, ok := w.events[l]; ok || w.defs[l] {
		return w.blockEnd(l)
	}
	for j := l + 1; j < len(w.s.Code); j++ {
		if len(w.labelsAt[j]) > 0 {
			return j
		}
	}
	return len(w.s.Code)
}

// sectionOf is the label command c's returns belong to: the def's or
// event's whose block holds c, else the nearest label above.
func (w *scriptWriter) sectionOf(c int) int {
	sec := -1
	for j := c; j >= 0; j-- {
		if len(w.labelsAt[j]) == 0 {
			continue
		}
		if _, ok := w.events[j]; ok || w.defs[j] {
			if w.blockEnd(j) > c {
				return j
			}
			return sec
		}
		if sec < 0 {
			sec = j
		}
	}
	return sec
}

// valueRet: the return after command c (which leaves the flags st) is
// its section's return X.
func (w *scriptWriter) valueRet(c int, st flagState) bool {
	l := w.sectionOf(c)
	return w.valueDef[l] && w.retState[l].same(st)
}

// ret writes the return after the Compare at c: return X, or return A op B.
func (w *scriptWriter) ret(c int) string {
	in := w.s.Code[c]
	st := flagState{opCompare, in.Args, c}
	if w.valueRet(c, st) {
		return "return " + w.arg(in.Args[0], "", Arg{}, "")
	}
	op, ok := w.retOp[c]
	if !ok {
		op = opIfEq
	}
	return "return " + w.test(st, op)
}

// callTest writes the test If op makes on what the section at label l
// returns: Foo() == "YES" (a menu section), Foo() op N (return X), Foo()
// or not Foo() (return A op B with that operator); "" when there is none.
func (w *scriptWriter) callTest(l int, op byte) string {
	name := w.s.At[l] + "()"
	if _, items, k, ok := w.s.menuOf(l); ok && (op == opIfEq || op == opIfNe) {
		return fmt.Sprintf("%s %s %q", name, ifOps[op], items[k])
	}
	if w.valueDef[l] {
		st := w.retState[l]
		if st.kind == opCompare {
			return fmt.Sprintf("%s %s %s", name, ifOps[op], w.arg(st.args[1], "", st.args[0], ifOps[op]))
		}
		return fmt.Sprintf("%s %s 0", name, ifOps[flipIf(op)])
	}
	if _, c := w.s.firstReturn(l); c >= 0 && w.s.Code[c].Op == opCompare && !w.valueRet(c, flagState{opCompare, w.s.Code[c].Args, c}) {
		switch w.textOp(c) {
		case op:
			return name
		case elseIf(op):
			return "not " + name
		}
	}
	return ""
}

// flagsComment says where the flags an (op) test reads come from.
func (w *scriptWriter) flagsComment(i int, op byte) string {
	st := w.before[i]
	if st.kind == 0 {
		if i > 0 && w.s.Code[i-1].Op == opGosub {
			return "the flags as " + w.s.At[w.s.Code[i-1].Args[0].N] + "() left them"
		}
		return "the flags of the jump here"
	}
	if in := w.s.Code[st.at]; in.Op == opGosub && len(in.Args) == 1 && in.Args[0].Label { // set by a called section
		if t := w.callTest(in.Args[0].N, op); t != "" {
			return t
		}
	}
	t := w.test(st, op)
	if t == "" {
		return ""
	}
	if st.at > 0 && isIf(w.s.Code[st.at-1].Op) { // set by a guarded command
		if u := w.test(w.before[st.at], op); u != "" {
			return t + " if that ran, else " + u
		}
	}
	return t
}

// blockEnd is where the section starting at command i ends: at the next
// label, taken further over labels only gotos inside the section reach.
func (w *scriptWriter) blockEnd(i int) int {
	next := func(j int) int {
		for j++; j < len(w.s.Code); j++ {
			if len(w.labelsAt[j]) > 0 {
				return j
			}
		}
		return len(w.s.Code)
	}
	end := next(i)
	for end < len(w.s.Code) {
		if _, ok := w.events[end]; ok || w.defs[end] {
			break
		}
		after := next(end)
		if slices.ContainsFunc(w.refs[end], func(r ref) bool { return r.op != opGoto || r.from < i || r.from >= after }) {
			break
		}
		end = after
	}
	return end
}

// stmts writes the commands idx as statements; guarded when they are the
// commands an if guards (each with its If before it in the code).
func (w *scriptWriter) stmts(idx []int, guarded bool) ([]string, error) {
	var out []string
	for k := 0; k < len(idx); {
		i := idx[k]
		if k > 0 {
			for _, l := range w.labelsAt[i] {
				if guarded {
					return nil, fmt.Errorf("label %s on a guarded command", l)
				}
				out = append(out, "", l+":")
			}
		}
		lines, n, err := w.stmt(idx, k, guarded)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", w.where(i), err)
		}
		if c := w.s.Code[idx[k+n-1]].Comment; c != "" {
			lines[len(lines)-1] += " # " + c
		}
		out = append(out, lines...)
		k += n
	}
	return out, nil
}

// where names command i for an error.
func (w *scriptWriter) where(i int) string {
	for j := i; j >= 0; j-- {
		if l := w.s.At[j]; l != "" {
			return fmt.Sprintf("%s+%d", l, i-j)
		}
	}
	return strconv.Itoa(i)
}

// adjacent: idx[k..k+n) are the next commands with no label on the later
// ones (in the guarded list, the next guarded ones).
func (w *scriptWriter) adjacent(idx []int, k, n int, guarded bool) bool {
	if k+n > len(idx) {
		return false
	}
	for j := 1; j < n; j++ {
		if len(w.labelsAt[idx[k+j]]) > 0 || !guarded && idx[k+j] != idx[k]+j {
			return false
		}
	}
	return true
}

// stmt writes the statement at idx[k]: its lines and how many commands
// it takes.
func (w *scriptWriter) stmt(idx []int, k int, guarded bool) ([]string, int, error) {
	i := idx[k]
	in := w.s.Code[i]
	op := func(j int) byte { return w.s.Code[idx[j]].Op }
	if !guarded {
		if lines, n, err := w.loop(idx, k); n > 0 || err != nil {
			return lines, n, err
		}
		if lines, n, err := w.ifStmt(idx, k); n > 0 || err != nil {
			return lines, n, err
		}
	} else if isIf(in.Op) {
		return nil, 0, fmt.Errorf("an if guards an if")
	}
	saves := func(name string) bool { return in.Op == opSave && len(in.Args) == 2 && w.varName(in.Args[1]) == name }
	switch {
	case in.Op == opCompare && len(in.Args) == 2 && w.adjacent(idx, k, 2, guarded) && op(k+1) == opReturn:
		return []string{w.ret(i)}, 2, nil
	case (in.Op == opAnd || in.Op == opOr) && len(in.Args) == 3 && w.adjacent(idx, k, 2, guarded) && op(k+1) == opReturn &&
		w.valueRet(i, flagState{opAnd, []Arg{{}, in.Args[2]}, i}):
		s, err := w.simple(i)
		return []string{s, "return " + w.arg(in.Args[2], "", Arg{}, "")}, 2, err
	case saves(varPortrait) && w.adjacent(idx, k, 2, guarded) && op(k+1) == opPicture:
		b := w.s.Code[idx[k+1]].Args[0]
		return []string{fmt.Sprintf("ShowPortrait(%s, %s)", w.arg(in.Args[0], "Head", Arg{}, ""), w.arg(b, "Body", Arg{}, ""))}, 2, nil
	case saves(varSymbolSet) && w.adjacent(idx, k, 2, guarded) && op(k+1) == opNewECL:
		a := w.s.Code[idx[k+1]].Args[0]
		for _, ar := range w.names.Areas {
			if a.Type == 0 && ar.ID == a.N && in.Args[0] == (Arg{N: ar.Set}) {
				return []string{fmt.Sprintf("EnterArea(%s)", ar.Name)}, 2, nil
			}
		}
	case saves(varTemple) && in.Args[0] == (Arg{N: 1}) && w.adjacent(idx, k, 2, guarded) && op(k+1) == opCombat:
		return []string{"OpenTemple()"}, 2, nil
	case in.Op == opTreasure && w.adjacent(idx, k, 5, guarded) && op(k+4) == opCombat:
		if stock := w.shop(idx[k : k+5]); stock != "" {
			return []string{"OpenShop(" + stock + ")"}, 5, nil
		}
	case in.Op == opGosub && w.adjacent(idx, k, 2, guarded) && (op(k+1) == opOnGoto || op(k+1) == opOnGosub):
		on := w.s.Code[idx[k+1]]
		if d, _, _, ok := w.s.menuOf(in.Args[0].N); ok && len(on.Args) >= 2 && on.Args[0] == d {
			return []string{w.onGoto(on, w.arg(in.Args[0], "", Arg{}, "")+"()")}, 2, nil
		}
	}
	line, err := w.simple(i)
	return []string{line}, 1, err
}

// shop is an OpenShop's stock when idx are its commands (Treasure of an
// ITEM block, Shop = 1, ShopFlag = 1, ShopKind = 16, Combat), else "".
func (w *scriptWriter) shop(idx []int) string {
	t := w.s.Code[idx[0]].Args
	if len(t) != 8 || t[7].Type != 0 || t[7].N >= 0x80 {
		return ""
	}
	for _, a := range t[:7] {
		if a != (Arg{}) {
			return ""
		}
	}
	for j, want := range []struct {
		v    int
		name string
	}{{1, varShop}, {1, varShopFlag}, {16, varShopKind}} {
		in := w.s.Code[idx[j+1]]
		if in.Op != opSave || len(in.Args) != 2 || in.Args[0] != (Arg{N: want.v}) || w.varName(in.Args[1]) != want.name {
			return ""
		}
	}
	stock := valueName(w.names.Values, "Stock", w.names.Set, t[7].N)
	if v, _ := valueOf(w.names.Values, "Stock", w.names.Set, stock); stock == "" || v != t[7].N {
		return ""
	}
	return strings.ReplaceAll(stock, ",", ", ")
}

// onGoto writes an OnGoto or OnGosub on v (its operand, or a menu
// section's call).
func (w *scriptWriter) onGoto(in Instr, v string) string {
	var ls []string
	for _, a := range in.Args[2:] {
		ls = append(ls, w.arg(a, "", Arg{}, ""))
	}
	word := "goto"
	if in.Op == opOnGosub {
		word = "gosub"
	}
	if v == "" {
		v = w.arg(in.Args[0], "", Arg{}, "")
	}
	return fmt.Sprintf("%s [%s][%s]", word, wrapArgs(ls), v)
}

// loop writes a for loop at idx[k]: X = A, a label, the body, X += 1,
// Compare X B and if X < B (or <=) goto the label, which nothing else
// jumps to.
func (w *scriptWriter) loop(idx []int, k int) ([]string, int, error) {
	in := w.s.Code[idx[k]]
	if in.Op != opSave || len(in.Args) != 2 || in.Args[1].Type != 1 || k+1 >= len(idx) || idx[k+1] != idx[k]+1 {
		return nil, 0, nil
	}
	x, l := in.Args[1], idx[k+1]
	if _, ok := w.events[l]; ok || w.defs[l] || len(w.labelsAt[l]) == 0 || len(w.refs[l]) != 1 || w.refs[l][0].op != opGoto {
		return nil, 0, nil
	}
	g := w.refs[l][0].from
	m := slices.Index(idx, g)
	if m < k+5 || idx[m] != l+m-k-1 {
		return nil, 0, nil
	}
	add, cmp, jif := w.s.Code[g-3], w.s.Code[g-2], w.s.Code[g-1]
	if add.Op != opAdd || !slices.Equal(add.Args, []Arg{{N: 1}, x, x}) || cmp.Op != opCompare || cmp.Args[0] != x ||
		jif.Op != opIfLt && jif.Op != opIfLe {
		return nil, 0, nil
	}
	for j := m - 3; j <= m; j++ {
		if len(w.labelsAt[idx[j]]) > 0 {
			return nil, 0, nil
		}
	}
	body, err := w.stmts(idx[k+1:m-3], false)
	if err != nil {
		return nil, 0, err
	}
	head := fmt.Sprintf("for %s = %s ", w.arg(x, "", Arg{}, ""), w.arg(in.Args[0], "", x, "="))
	if jif.Op == opIfLe {
		head += "to " + w.arg(cmp.Args[1], "", x, "<=")
	} else {
		head += fmt.Sprintf("while %s < %s", w.arg(x, "", Arg{}, ""), w.arg(cmp.Args[1], "", x, "<"))
	}
	return w.block(head, body, nil), m - k + 1, nil
}

// elseBlock is an else if / else of a block.
type elseBlock struct {
	head string // "else if ..." or "else"
	body []string
}

// block lays out head { body } [else ... { other }].
func (w *scriptWriter) block(head string, body []string, elses []elseBlock) []string {
	if len(body) == 1 && elses == nil && len(head)+len(body[0]) < 90 && !strings.HasPrefix(head, "for") {
		return []string{head + " { " + body[0] + " }"}
	}
	out := []string{head + " {"}
	for _, l := range body {
		out = append(out, indent+l)
	}
	for _, e := range elses {
		out = append(out, "} "+e.head+" {")
		for _, l := range e.body {
			out = append(out, indent+l)
		}
	}
	return append(out, "}")
}

// ifStmt writes an if at idx[k]: a comparing command and the If after it
// (if A == B, if X has Bits, if D = A & B; D != 0, if Carried(X)), a
// section's call and the If on its result (if Foo() == "YES", if Foo() <
// 2, if Foo(), if not Foo()), or an If on the flags as they are, (==),
// with a comment saying what they hold. The commands the same If guards
// after it make the block; further Ifs on the same flags make else if and
// else blocks, up to a guarded command that sets the flags.
func (w *scriptWriter) ifStmt(idx []int, k int) ([]string, int, error) {
	i := idx[k]
	in := w.s.Code[i]
	var lines []string
	test, comment, p := "", "", k
	var head flagState
	next := w.adjacent(idx, k, 2, false) && isIf(w.s.Code[i+1].Op)
	raw := func(op byte) error { // the command, then (op)
		s, err := w.simple(i)
		lines = append(lines, s)
		test = "(" + ifOps[op] + ")"
		return err
	}
	switch {
	case next && (in.Op == opCompare || in.Op == opCompareAnd || in.Op == opFindItem):
		op := w.s.Code[i+1].Op
		head = flagState{in.Op, in.Args, i}
		if test = w.test(head, op); test == "" {
			if err := raw(op); err != nil {
				return nil, 0, err
			}
		}
		p = k + 1
	case next && (in.Op == opAnd || in.Op == opOr) && len(in.Args) == 3:
		op := w.s.Code[i+1].Op
		head = flagState{opAnd, []Arg{{}, in.Args[2]}, i}
		if in.Op == opAnd && w.varName(in.Args[2]) == hasScratch && (op == opIfEq || op == opIfNe) {
			word := "has"
			if op == opIfEq {
				word = "lacks"
			}
			test = fmt.Sprintf("%s %s %s", w.arg(in.Args[0], "", in.Args[1], word), word, w.arg(in.Args[1], "", in.Args[0], word))
		} else {
			s, err := w.simple(i)
			if err != nil {
				return nil, 0, err
			}
			test = s + "; " + w.test(head, op)
		}
		p = k + 1
	case next && in.Op == opGosub && len(in.Args) == 1 && in.Args[0].Label:
		op := w.s.Code[i+1].Op
		if test = w.callTest(in.Args[0].N, op); test == "" {
			if err := raw(op); err != nil {
				return nil, 0, err
			}
			comment = w.flagsComment(i+1, op)
		}
		p = k + 1
	case isIf(in.Op):
		test = "(" + ifOps[in.Op] + ")"
		comment = w.flagsComment(i, in.Op)
	default:
		return nil, 0, nil
	}
	// the runs of Ifs on these flags, each with the commands it guards
	type seg struct {
		op byte
		g  []int
	}
	var segs []seg
	q, broken := p, false
	for q < len(idx) && isIf(w.s.Code[idx[q]].Op) && !broken && (q == p || len(w.labelsAt[idx[q]]) == 0 && idx[q] == idx[q-1]+1) {
		if q+1 >= len(idx) || idx[q+1] != idx[q]+1 || len(w.labelsAt[idx[q+1]]) > 0 {
			return nil, 0, fmt.Errorf("an if without its command")
		}
		op := w.s.Code[idx[q]].Op
		if len(segs) == 0 || segs[len(segs)-1].op != op {
			segs = append(segs, seg{op, nil})
		}
		segs[len(segs)-1].g = append(segs[len(segs)-1].g, idx[q+1])
		g := idx[q+1] // a guarded command that sets the flags ends the chain
		broken = g+1 < len(w.before) && w.before[g+1].at != w.before[g].at
		q += 2
	}
	body, err := w.stmts(segs[0].g, true)
	if err != nil {
		return nil, 0, err
	}
	var elses []elseBlock
	for n, sg := range segs[1:] {
		h := "else if (" + ifOps[sg.op] + ")"
		if n == len(segs)-2 && sg.op == elseIf(segs[n].op) {
			h = "else"
		} else if head.kind == opCompare {
			h = "else if " + w.test(head, sg.op)
		}
		b, err := w.stmts(sg.g, true)
		if err != nil {
			return nil, 0, err
		}
		elses = append(elses, elseBlock{h, b})
	}
	out := w.block("if "+test, body, elses)
	if comment != "" {
		out[0] += " # " + comment
	}
	return append(lines, out...), q - k, nil
}

// test writes the test If op makes on the flags st, or "" when the
// text has no form for it.
func (w *scriptWriter) test(st flagState, op byte) string {
	switch st.kind {
	case opCompare:
		return fmt.Sprintf("%s %s %s", w.arg(st.args[0], "", st.args[1], ifOps[op]), ifOps[op], w.arg(st.args[1], "", st.args[0], ifOps[op]))
	case opAnd: // 0 op D, written D op 0
		f := ifOps[flipIf(op)]
		return fmt.Sprintf("%s %s 0", w.arg(st.args[1], "", Arg{}, ""), f)
	case opCompareAnd:
		a := st.args
		switch op {
		case opIfEq:
			return fmt.Sprintf("%s == %s and %s == %s", w.arg(a[0], "", a[1], "=="), w.arg(a[1], "", a[0], "=="), w.arg(a[2], "", a[3], "=="), w.arg(a[3], "", a[2], "=="))
		case opIfNe:
			return fmt.Sprintf("%s != %s or %s != %s", w.arg(a[0], "", a[1], "!="), w.arg(a[1], "", a[0], "!="), w.arg(a[2], "", a[3], "!="), w.arg(a[3], "", a[2], "!="))
		}
	case opFindItem:
		switch op {
		case opIfEq:
			return fmt.Sprintf("Carried(%s)", w.arg(st.args[0], "Item", Arg{}, ""))
		case opIfNe:
			return fmt.Sprintf("not Carried(%s)", w.arg(st.args[0], "Item", Arg{}, ""))
		}
	}
	return ""
}

// simple writes command i as one statement.
func (w *scriptWriter) simple(i int) (string, error) {
	in := w.s.Code[i]
	a := in.Args
	plain := func(k int) string { return w.arg(a[k], "", Arg{}, "") }
	beside := func(k, v int, op string) string { return w.arg(a[k], "", a[v], op) }
	n := len(a)
	switch {
	case in.Op == opSave && n == 2:
		if w.varName(a[1]) == varPortrait {
			return plain(1) + " = " + w.arg(a[0], "Head", Arg{}, ""), nil
		}
		return plain(1) + " = " + beside(0, 1, "="), nil
	case (in.Op == opAdd || in.Op == opSub) && n == 3 && a[1] == a[2]:
		return plain(2) + " " + map[byte]string{opAdd: "+=", opSub: "-="}[in.Op] + " " + beside(0, 2, "="), nil
	case (in.Op == opAnd || in.Op == opOr) && n == 3 && a[0] == a[2]:
		op := map[byte]string{opAnd: "&=", opOr: "|="}[in.Op]
		return plain(2) + " " + op + " " + beside(1, 0, op), nil
	case in.Op == opSub && n == 3:
		return plain(2) + " = " + beside(1, 0, "-") + " - " + beside(0, 1, "-"), nil
	case (in.Op == opAdd || in.Op == opMul || in.Op == opDiv || in.Op == opAnd || in.Op == opOr) && n == 3:
		op := map[byte]string{opAdd: "+", opMul: "*", opDiv: "/", opAnd: "&", opOr: "|"}[in.Op]
		return plain(2) + " = " + beside(0, 1, op) + " " + op + " " + beside(1, 0, op), nil
	case in.Op == opGetTable && n == 3:
		return fmt.Sprintf("%s = %s[%s]", plain(2), plain(0), plain(1)), nil
	case in.Op == opSaveTable && n == 3:
		return fmt.Sprintf("%s[%s] = %s", plain(1), plain(2), plain(0)), nil
	case in.Op == opRandom && n == 2:
		return fmt.Sprintf("%s = Random(%s)", plain(1), plain(0)), nil
	case in.Op == opPartyStr && n == 1:
		return plain(0) + " = PartyStrength()", nil
	case (in.Op == opInputNumber || in.Op == opInputString) && n == 2:
		return fmt.Sprintf("%s = %s(%s)", plain(1), eclOpName(in.Op), plain(0)), nil
	case in.Op == opParlay && n == 6:
		return fmt.Sprintf("%s = Parlay(%s)", plain(5), w.list(a[:5], nil)), nil
	case in.Op == opHMenu && n >= 2 && a[1] == (Arg{N: n - 2}):
		return fmt.Sprintf("%s = Menu(%s)", plain(0), w.list(a[2:], nil)), nil
	case in.Op == opVMenu && n >= 3 && a[2] == (Arg{N: n - 3}):
		return fmt.Sprintf("%s = VMenu(%s)", plain(0), w.list(append([]Arg{a[1]}, a[3:]...), nil)), nil
	case in.Op == opGoto && n == 1:
		return "goto " + plain(0), nil
	case in.Op == opGosub && n == 1:
		return plain(0) + "()", nil
	case in.Op == opReturn:
		return "return", nil
	case in.Op == opExit:
		return "exit", nil
	case (in.Op == opOnGoto || in.Op == opOnGosub) && n >= 2 && a[1] == (Arg{N: n - 2}):
		return w.onGoto(in, ""), nil
	case in.Op == opCall && n == 1:
		if a[0].Type == 1 && w.names.Vars[a[0].N] != "" {
			return plain(0) + "()", nil
		}
		return fmt.Sprintf("Call(0x%04X)", a[0].N), nil
	case in.Op == opNewECL && n == 1:
		return "NewECL(" + w.arg(a[0], "Area", Arg{}, "") + ")", nil
	case in.Op == opCompare && n == 2:
		return fmt.Sprintf("Compare(%s, %s)", beside(0, 1, "=="), beside(1, 0, "==")), nil
	case in.Op == opLoadMap && n == 3: // the second operand is unused, the third is 255 or loads the icons
		s := "LoadMap(" + w.arg(a[0], "Map", Arg{}, "")
		if a[2] != (Arg{N: 0xFF}) {
			s += ", icons=" + w.arg(a[2], "MapIcons", Arg{}, "")
		}
		return s + ")", nil
	case in.Op == opTreasure && n == 8:
		var parts []string
		for k, l := range treasureLabels {
			switch {
			case k == 7 && a[k].Type == 0 && a[k].N == 0xFF:
			case k == 7 && a[k].Type == 0 && a[k].N >= 0x80:
				parts = append(parts, fmt.Sprintf("items=Random(%d)", a[k].N-0x80))
			case k == 7 && a[k].Type == 0:
				stock := valueName(w.names.Values, "Stock", w.names.Set, a[k].N)
				if v, _ := valueOf(w.names.Values, "Stock", w.names.Set, stock); stock != "" && v == a[k].N {
					parts = append(parts, "items=["+strings.ReplaceAll(stock, ",", ", ")+"]")
				} else {
					parts = append(parts, "items="+plain(k))
				}
			case a[k] != (Arg{}):
				parts = append(parts, l+"="+plain(k))
			}
		}
		return "Treasure(" + strings.Join(parts, ", ") + ")", nil
	case isIf(in.Op):
		return "", fmt.Errorf("an if out of place")
	}
	name := eclOpName(in.Op)
	kinds, labels := operandKinds[name], operandLabels[name]
	var parts []string
	for k, x := range a {
		kind, label := "", ""
		if k < len(kinds) {
			kind = kinds[k]
		}
		if k < len(labels) {
			label = labels[k]
		}
		if l, b, ok := strings.Cut(label, "|"); ok {
			label = l
			if w.head[i] {
				label = b
			}
		}
		if kind == "Picture|Body" && !w.head[i] {
			kind = "Picture"
		}
		if d, ok := operandDefaults[name][k]; ok && x.Type == 0x80 && strconv.Quote(x.S) == d {
			continue
		}
		v := w.arg(x, kind, Arg{}, "")
		if label != "" {
			v = label + "=" + v
		}
		parts = append(parts, v)
	}
	return commandName(in.Op) + "(" + wrapArgs(parts) + ")", nil
}

// list writes operands apart by commas.
func (w *scriptWriter) list(a []Arg, kinds []string) string {
	var parts []string
	for k, x := range a {
		kind := ""
		if k < len(kinds) {
			kind = kinds[k]
		}
		parts = append(parts, w.arg(x, kind, Arg{}, ""))
	}
	return wrapArgs(parts)
}

// wrap joins a call's operands, on more lines when long.
func wrapArgs(parts []string) string {
	s := strings.Join(parts, ", ")
	if len(s) < 100 {
		return s
	}
	var lines []string
	cur := ""
	for _, p := range parts {
		if cur != "" && len(cur)+len(p) > 90 {
			lines = append(lines, cur+",")
			cur = ""
		}
		if cur != "" {
			cur += ", "
		}
		cur += p
	}
	return strings.Join(append(lines, cur), "\n"+indent+indent)
}

// varName is the name of a variable operand (or "" when it is not one).
func (w *scriptWriter) varName(a Arg) string {
	if a.Type != 1 && a.Type != 3 && a.Type != 0x81 || a.Label {
		return ""
	}
	if n := w.names.Vars[a.N]; n != "" {
		return n
	}
	return fmt.Sprintf("V_%04X", a.N)
}

// arg writes operand a: a number of kind, or beside the variable other
// under op (the name of that variable's value).
func (w *scriptWriter) arg(a Arg, kind string, other Arg, op string) string {
	switch a.Type {
	case 0x80:
		return strconv.Quote(a.S)
	case 0x81:
		return "$" + w.varName(a)
	case 3:
		return "&" + w.varName(a)
	case 2: // a word: in hex when a byte would hold it, to tell it from one
		if a.N > 0xFF {
			return strconv.Itoa(a.N)
		}
		return fmt.Sprintf("0x%04X", a.N)
	case 1:
		if a.Label {
			return w.s.At[a.N]
		}
		if n := w.table[a.N]; n != "" {
			return n
		}
		return w.varName(a)
	}
	return w.number(a.N, kind, other, op)
}

// number writes the byte v as the name of kind's value, or of the value
// of the variable other under op, else as the number.
func (w *scriptWriter) number(v int, kind string, other Arg, op string) string {
	name := ""
	if kind == "Picture|Body" {
		kind = "Body"
	}
	if kind != "" {
		name = w.names.kindName(kind, v)
		if kind == "Body" { // the parser reads a picture by a Picture name first
			if x, _ := w.names.kindValue("Picture|Body", name); x != v {
				name = ""
			}
		}
	} else if vn := w.varName(other); vn != "" && other.Type != 0x81 {
		name = w.names.varValue(vn, op, v)
		if name != "" {
			if x, ok := w.names.varNumber(vn, name); !ok || x != v {
				name = ""
			}
		}
	}
	if name == "" || w.taken[strings.Trim(name, "~()")] || strings.ContainsAny(name, "|~") && w.anyTaken(name) {
		return strconv.Itoa(v)
	}
	if _, isArea := w.names.kindValue("Area", name); isArea && kind == "" && varKinds[w.varName(other)] == "" {
		return strconv.Itoa(v) // an area's name beside a plain variable would read as the area
	}
	return name
}

// anyTaken: a bit of the mask text is a name in use.
func (w *scriptWriter) anyTaken(mask string) bool {
	for _, p := range strings.Split(strings.Trim(mask, "~()"), "|") {
		if w.taken[p] {
			return true
		}
	}
	return false
}

// headSet is, for each command, whether a portrait head may be up: after
// Portrait = H (not None) on a way to it, falling through or by a jump.
// An event starts without one.
func (s *Script) headSet() []bool {
	entry := map[int]bool{}
	out := make([]bool, len(s.Code))
	events := map[int]bool{}
	for _, e := range s.Events {
		events[e] = true
	}
	portrait := -1
	for a, n := range ScriptVarNames() {
		if n == varPortrait {
			portrait = a
		}
	}
	for changed := true; changed; {
		changed = false
		state := false
		for i, in := range s.Code {
			if events[i] {
				state = false
			}
			if _, ok := s.At[i]; ok {
				state = state || entry[i]
			}
			out[i] = state
			if in.Op == opSave && len(in.Args) == 2 && in.Args[1].Type == 1 && in.Args[1].N == portrait {
				state = in.Args[0] != (Arg{N: 255})
			}
			if state {
				for _, a := range in.Args {
					if a.Label && !entry[a.N] {
						entry[a.N], changed = true, true
					}
				}
			}
			if (in.Op == opGoto || in.Op == opExit || in.Op == opReturn) && !(i > 0 && isIf(s.Code[i-1].Op)) {
				state = false
			}
		}
	}
	return out
}
