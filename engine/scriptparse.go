package engine

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"text/scanner"
)

// ParseScript parses the script text (README.md beside the scripts) into
// a Script. The text's labels, sections and blocks lay the commands out
// one after the other; an if makes its comparison and guards each command
// of its block with an If (scriptform.go on the forms that test flags set
// elsewhere).
func ParseScript(text string, names *ScriptNames) (*Script, error) {
	p := &parser{names: names, s: &Script{Labels: map[string]int{}, At: map[int]string{}, Tables: map[string]int{}},
		vars: map[string]int{}, labels: map[string]bool{}, tables: map[string]bool{}}
	for a, n := range names.Vars {
		p.vars[n] = a
	}
	if err := p.lex(text); err != nil {
		return nil, err
	}
	for i, t := range p.toks { // the labels and tables, for the names' meaning
		switch {
		case t.k == tIdent && i+1 < len(p.toks) && p.toks[i+1].is(":") && (i == 0 || p.toks[i-1].k == tNL || p.toks[i-1].is("{")):
			p.labels[t.s] = true
		case t.k == tIdent && (t.s == "def" || t.s == "on") && i+1 < len(p.toks) && p.toks[i+1].k == tIdent:
			if t.s == "on" {
				if e := slices.Index(eventNames[:], p.toks[i+1].s); e >= 0 {
					p.labels[eventLabel(e)] = true
				}
			} else {
				p.labels[p.toks[i+1].s] = true
			}
		case t.k == tIdent && i+2 < len(p.toks) && p.toks[i+1].is("=") && p.toks[i+2].is("[") && (i == 0 || p.toks[i-1].k == tNL):
			p.tables[t.s] = true
		}
	}
	for l := range p.labels { // a label's name over a variable's (the labels file names addresses too)
		delete(p.vars, l)
	}
	for err := p.file(); err != nil; {
		return nil, fmt.Errorf("%d: %v", p.line(), err)
	}
	if err := p.finish(); err != nil {
		return nil, err
	}
	return p.s, nil
}

type tokKind int

const (
	tEOF tokKind = iota
	tNL
	tIdent
	tInt
	tString
	tOp
)

type tok struct {
	k    tokKind
	s    string
	line int
}

func (t tok) is(op string) bool { return t.k == tOp && t.s == op }

type parser struct {
	names  *ScriptNames
	s      *Script
	vars   map[string]int
	labels map[string]bool
	tables map[string]bool
	toks   []tok
	pos    int
	fix    []fixup // jump operands to resolve to their labels
	depth  int     // in an if block: nothing but statements
	// the returns' comparisons and the tests on called sections, matched
	// by finish
	section string  // the section a return belongs to: the def's or event's, else the last label's
	inDef   bool    // in a def's or event's block
	retOp   []retOp // the operator each return A op B shows (the if's for if A op B { return })
	rets    []valueRet
	calls   []*callTest
}

// fixup is a jump operand waiting for its label's command number.
type fixup struct {
	instr, arg int
	label      string
}

// retOp is the operator the text shows for the Compare at cmp before a
// return: if Foo() tests with it.
type retOp struct {
	cmp int
	op  byte
}

// valueRet is a return X of a section: a Compare X N and a Return, N set
// by the callers (if Foo() op N), or the Return after D = A & B (cmp -1;
// the callers test if Foo() op 0).
type valueRet struct {
	cmp     int
	section string
}

// callTest is an if on a called section's result; its Ifs get their
// operator once the section's returns are known.
type callTest struct {
	label string
	line  int
	op    string // "" for Foo() and not Foo()
	not   bool
	n     opnd   // the operand compared with (if Foo() op N)
	ans   string // or the menu answer (if Foo() == "YES")
	isAns bool
	ifs   []ifRef
}

// ifRef is an If of a call test, the else's negated.
type ifRef struct {
	instr int
	neg   bool
}

// lex splits text into tokens: idents, ints, Go strings, operators,
// newlines (not inside brackets); # starts a comment.
func (p *parser) lex(text string) error {
	var sc scanner.Scanner
	sc.Init(strings.NewReader(text))
	sc.Mode = scanner.ScanIdents | scanner.ScanInts | scanner.ScanStrings
	sc.Whitespace = 1<<' ' | 1<<'\t' | 1<<'\r'
	var errs []string
	sc.Error = func(_ *scanner.Scanner, msg string) { errs = append(errs, fmt.Sprintf("%d: %s", sc.Line, msg)) }
	nest := 0
	for {
		r := sc.Scan()
		line := sc.Position.Line
		switch r {
		case scanner.EOF:
			p.toks = append(p.toks, tok{tNL, "", line}, tok{tEOF, "", line})
			if errs != nil {
				return fmt.Errorf("%s", strings.Join(errs, "; "))
			}
			return nil
		case scanner.Ident:
			p.toks = append(p.toks, tok{tIdent, sc.TokenText(), line})
		case scanner.Int:
			p.toks = append(p.toks, tok{tInt, sc.TokenText(), line})
		case scanner.String:
			s, err := strconv.Unquote(sc.TokenText())
			if err != nil {
				return fmt.Errorf("%d: string %s", line, sc.TokenText())
			}
			p.toks = append(p.toks, tok{tString, s, line})
		case '#':
			for c := sc.Peek(); c != '\n' && c != scanner.EOF; c = sc.Peek() {
				sc.Next()
			}
		case '\n':
			if nest == 0 {
				p.toks = append(p.toks, tok{tNL, "", line})
			}
		default:
			op := string(r)
			if strings.ContainsRune("=!<>+-|&", r) && sc.Peek() == '=' {
				sc.Next()
				op += "="
			}
			switch op {
			case "(", "[":
				nest++
			case ")", "]":
				nest--
			}
			p.toks = append(p.toks, tok{tOp, op, line})
		}
	}
}

func (p *parser) peek() tok { return p.toks[p.pos] }
func (p *parser) next() tok { t := p.toks[p.pos]; p.pos++; return t }
func (p *parser) line() int { return p.toks[min(p.pos, len(p.toks)-1)].line }

// accept takes operator op if it is next.
func (p *parser) accept(op string) bool {
	if p.peek().is(op) {
		p.pos++
		return true
	}
	return false
}

// expect takes operator op.
func (p *parser) expect(op string) error {
	if !p.accept(op) {
		return fmt.Errorf("want %s, have %s", op, p.peek().text())
	}
	return nil
}

func (t tok) text() string {
	switch t.k {
	case tEOF:
		return "the end"
	case tNL:
		return "the end of the line"
	case tString:
		return strconv.Quote(t.s)
	}
	return t.s
}

func (p *parser) skipNL() {
	for p.peek().k == tNL {
		p.pos++
	}
}

// ident takes a name.
func (p *parser) ident() (string, error) {
	if t := p.peek(); t.k == tIdent {
		p.pos++
		return t.s, nil
	}
	return "", fmt.Errorf("want a name, have %s", p.peek().text())
}

// endStmt takes the end of a statement: a newline, ; or before }.
func (p *parser) endStmt() error {
	if p.accept(";") || p.peek().k == tNL || p.peek().k == tEOF || p.peek().is("}") {
		return nil
	}
	return fmt.Errorf("unexpected %s", p.peek().text())
}

// file parses the whole text.
func (p *parser) file() error {
	for {
		p.skipNL()
		t := p.peek()
		switch {
		case t.k == tEOF:
			return nil
		case t.k == tIdent && t.s == "on" && p.toks[p.pos+1].k == tIdent:
			p.pos++
			name, _ := p.ident()
			e := slices.Index(eventNames[:], name)
			if e < 0 {
				return fmt.Errorf("no event %s (load, move, step, camp, rest)", name)
			}
			p.label(eventLabel(e))
			p.inDef = true
			if err := p.braces(); err != nil {
				return err
			}
			p.inDef = false
		case t.k == tIdent && t.s == "def" && p.toks[p.pos+1].k == tIdent:
			p.pos++
			name, _ := p.ident()
			p.label(name)
			if err := p.expect("("); err != nil {
				return err
			}
			if err := p.expect(")"); err != nil {
				return err
			}
			p.inDef = true
			if err := p.braces(); err != nil {
				return err
			}
			p.inDef = false
		case t.k == tIdent && p.toks[p.pos+1].is("=") && p.toks[p.pos+2].is("["):
			p.pos += 3
			if err := p.table(t.s); err != nil {
				return err
			}
		default:
			if err := p.stmt(); err != nil {
				return err
			}
		}
	}
}

// braces parses { statements }.
func (p *parser) braces() error {
	if err := p.expect("{"); err != nil {
		return err
	}
	for {
		p.skipNL()
		if p.accept("}") {
			return nil
		}
		if p.peek().k == tEOF {
			return fmt.Errorf("missing }")
		}
		if err := p.stmt(); err != nil {
			return err
		}
	}
}

// label names the next command.
func (p *parser) label(name string) error {
	if _, dup := p.s.Labels[name]; dup {
		return fmt.Errorf("label %s twice", name)
	}
	if _, isArea := p.names.kindValue("Area", name); p.tables[name] || p.isVar(name) || isArea {
		return fmt.Errorf("%s is a label and a table, variable or area", name)
	}
	p.s.Labels[name] = len(p.s.Code)
	if _, ok := p.s.At[len(p.s.Code)]; !ok {
		p.s.At[len(p.s.Code)] = name
	}
	if !p.inDef { // a def's or event's own label comes before its block
		p.section = name
	}
	return nil
}

// emit adds a command.
func (p *parser) emit(op byte, args ...Arg) int {
	p.s.Code = append(p.s.Code, Instr{Op: op, Args: args, Line: p.line()})
	return len(p.s.Code) - 1
}

// table parses a table's entries, after Name = [.
func (p *parser) table(name string) error {
	if _, dup := p.s.Tables[name]; dup {
		return fmt.Errorf("table %s twice", name)
	}
	p.s.Tables[name] = eclBase + len(p.s.Data)
	kind := tableKind(name)
	for !p.accept("]") {
		o, err := p.operand()
		if err != nil {
			return err
		}
		a, err := p.resolve(o, kind, "", "")
		if err != nil {
			return err
		}
		if a.Type != 0 {
			return fmt.Errorf("a table holds bytes")
		}
		p.s.Data = append(p.s.Data, byte(a.N))
		if !p.accept(",") && !p.peek().is("]") {
			return fmt.Errorf("want , or ], have %s", p.peek().text())
		}
	}
	return p.endStmt()
}

// stmt parses one statement.
func (p *parser) stmt() error {
	t := p.peek()
	if t.k == tIdent && p.toks[p.pos+1].is(":") {
		if p.depth > 0 {
			return fmt.Errorf("a label in an if")
		}
		p.pos += 2
		return p.label(t.s)
	}
	if t.k != tIdent && !t.is("$") && !t.is("&") {
		return fmt.Errorf("unexpected %s", t.text())
	}
	switch t.s {
	case "if":
		p.pos++
		return p.ifStmt()
	case "for":
		if p.depth > 0 {
			return fmt.Errorf("a loop in an if")
		}
		p.pos++
		return p.forStmt()
	case "goto", "gosub":
		p.pos++
		if p.peek().is("[") {
			return p.onGoto(t.s)
		}
		if t.s == "gosub" {
			return fmt.Errorf("gosub takes [labels][choice]; a section is called as Name()")
		}
		name, err := p.ident()
		if err != nil {
			return err
		}
		p.jump(p.emit(opGoto, Arg{Type: 1, Label: true}), 0, name)
		return p.endStmt()
	case "return":
		p.pos++
		return p.ret()
	case "exit":
		p.pos++
		p.emit(opExit)
		return p.endStmt()
	case "else":
		return fmt.Errorf("else without if")
	}
	if t.k == tIdent && p.toks[p.pos+1].is("(") {
		p.pos += 2
		return p.call(t.s)
	}
	return p.assign()
}

func (p *parser) isVar(name string) bool { _, ok := p.vars[name]; return ok }

// ret parses what follows return: nothing; A op B (Compare A B, the
// callers test if Foo() with that operator); X (Compare X N, N from the
// callers' if Foo() op N; after D = A & B with X its D, the And's own
// flags).
func (p *parser) ret() error {
	if p.endStmt() == nil {
		p.emit(opReturn)
		return nil
	}
	a, err := p.operand()
	if err != nil {
		return err
	}
	if o := p.peek(); o.k == tOp && isCmp(o.s) {
		p.pos++
		op, _ := ifOf(o.s)
		x, b, err := p.compared(a, o.s)
		if err != nil {
			return err
		}
		p.retOp = append(p.retOp, retOp{p.emit(opCompare, x, b), op})
		p.emit(opReturn)
		return p.endStmt()
	}
	x, err := p.resolve(a, "", "", "")
	if err != nil {
		return err
	}
	if x.Type != 1 && x.Type != 3 {
		return fmt.Errorf("return %s: not a variable", a.text())
	}
	if l := len(p.s.Code) - 1; l >= 0 && (p.s.Code[l].Op == opAnd || p.s.Code[l].Op == opOr) && len(p.s.Code[l].Args) == 3 && p.s.Code[l].Args[2] == x {
		p.rets = append(p.rets, valueRet{-1, p.section})
	} else {
		p.rets = append(p.rets, valueRet{p.emit(opCompare, x, Arg{}), p.section})
	}
	p.emit(opReturn)
	return p.endStmt()
}

// compared resolves A op B's operands, each named beside the other.
func (p *parser) compared(a opnd, op string) (Arg, Arg, error) {
	an := ""
	if a.isVar() {
		an = a.name
	}
	b, err := p.value(an, op)
	if err != nil {
		return Arg{}, Arg{}, err
	}
	x, err := p.resolve(a, "", other2(b, p), op)
	return x, b, err
}

// jump notes a jump operand to resolve.
func (p *parser) jump(instr, arg int, label string) {
	p.fix = append(p.fix, fixup{instr, arg, label})
}

// onGoto parses goto [A, B][V] after goto or gosub.
func (p *parser) onGoto(word string) error {
	ls, err := p.labelList()
	if err != nil {
		return err
	}
	if err := p.expect("["); err != nil {
		return err
	}
	op := opOnGoto
	if word == "gosub" {
		op = opOnGosub
	}
	if t := p.peek(); t.k == tIdent && p.labels[t.s] && p.toks[p.pos+1].is("(") { // a menu section's call
		p.pos += 3
		if err := p.expect("]"); err != nil {
			return err
		}
		g := p.emit(opGosub, Arg{Type: 1, Label: true})
		p.jump(g, 0, t.s)
		i := p.emit(op, Arg{}, Arg{N: len(ls)})
		p.fix = append(p.fix, fixup{i, -1, t.s}) // the menu's variable, once the section is known
		return p.onLabels(i, ls)
	}
	o, err := p.operand()
	if err != nil {
		return err
	}
	v, err := p.resolve(o, "", "", "")
	if err != nil {
		return err
	}
	if err := p.expect("]"); err != nil {
		return err
	}
	i := p.emit(op, v, Arg{N: len(ls)})
	return p.onLabels(i, ls)
}

func (p *parser) onLabels(i int, ls []string) error {
	for k, l := range ls {
		p.s.Code[i].Args = append(p.s.Code[i].Args, Arg{Type: 1, Label: true})
		p.jump(i, k+2, l)
	}
	return p.endStmt()
}

// labelList parses [A, B, C].
func (p *parser) labelList() ([]string, error) {
	if err := p.expect("["); err != nil {
		return nil, err
	}
	var ls []string
	for !p.accept("]") {
		l, err := p.ident()
		if err != nil {
			return nil, err
		}
		if !p.labels[l] {
			return nil, fmt.Errorf("no label %s", l)
		}
		ls = append(ls, l)
		if !p.accept(",") && !p.peek().is("]") {
			return nil, fmt.Errorf("want , or ], have %s", p.peek().text())
		}
	}
	return ls, nil
}

// call parses a call after Name(: a section (Gosub), an engine routine
// (Call) or a command with its operands.
func (p *parser) call(name string) error {
	if p.labels[name] {
		if err := p.expect(")"); err != nil {
			return err
		}
		p.jump(p.emit(opGosub, Arg{Type: 1, Label: true}), 0, name)
		return p.endStmt()
	}
	switch name {
	case "ShowPortrait", "EnterArea", "OpenTemple", "OpenShop", "Treasure", "Call", "LoadMap", "Compare":
		return p.macro(name)
	}
	op, ok := commandOp(name)
	if !ok {
		a, isVar := p.vars[name]
		if !isVar {
			return fmt.Errorf("no command, section or routine %s", name)
		}
		if err := p.expect(")"); err != nil {
			return err
		}
		p.emit(opCall, Arg{Type: 1, N: a})
		return p.endStmt()
	}
	args, err := p.args(eclOpName(op))
	if err != nil {
		return err
	}
	p.emit(op, args...)
	return p.endStmt()
}

// parsedArg is an operand of a call, with its label when it had one.
type parsedArg struct {
	label  string
	o      opnd
	list   []opnd // [a, b] (an item list)
	isList bool
}

// callArgs parses a call's operands up to ).
func (p *parser) callArgs() ([]parsedArg, error) {
	var out []parsedArg
	for !p.accept(")") {
		var a parsedArg
		if t := p.peek(); t.k == tIdent && p.toks[p.pos+1].is("=") {
			a.label = t.s
			p.pos += 2
		}
		if p.accept("[") {
			a.isList = true
			for !p.accept("]") {
				o, err := p.operand()
				if err != nil {
					return nil, err
				}
				if p.accept("*") {
					n := p.next()
					if n.k != tInt {
						return nil, fmt.Errorf("want a count after *")
					}
					o.name += "*" + n.s
				}
				a.list = append(a.list, o)
				if !p.accept(",") && !p.peek().is("]") {
					return nil, fmt.Errorf("want , or ], have %s", p.peek().text())
				}
			}
		} else {
			o, err := p.operand()
			if err != nil {
				return nil, err
			}
			if o.name != "" && p.accept("*") { // Item*20, a stock's count
				n := p.next()
				if n.k != tInt {
					return nil, fmt.Errorf("want a count after *")
				}
				o.name += "*" + n.s
			}
			if o.name != "" && p.accept("(") { // Random(N)
				n, err := p.operand()
				if err != nil {
					return nil, err
				}
				if err := p.expect(")"); err != nil {
					return nil, err
				}
				o.call, o.callArg = o.name, &n
			}
			a.o = o
		}
		out = append(out, a)
		if !p.accept(",") && !p.peek().is(")") {
			return nil, fmt.Errorf("want , or ), have %s", p.peek().text())
		}
	}
	return out, nil
}

// args parses command name's operands up to ), labelled ones by their
// label, the left-out ones with defaults.
func (p *parser) args(name string) ([]Arg, error) {
	pa, err := p.callArgs()
	if err != nil {
		return nil, err
	}
	kinds, labels, defs := operandKinds[name], operandLabels[name], operandDefaults[name]
	var out []Arg
	j := 0
	for _, a := range pa {
		if a.label != "" {
			for ; j < len(labels) && !slices.Contains(strings.Split(labels[j], "|"), a.label); j++ {
				d, ok := defs[j]
				if !ok {
					return nil, fmt.Errorf("%s is not operand %d of %s", a.label, j+1, name)
				}
				s, _ := strconv.Unquote(d)
				out = append(out, Arg{Type: 0x80, N: (len(s)*6 + 7) / 8, S: s})
			}
			if j >= len(labels) {
				return nil, fmt.Errorf("%s is not an operand of %s", a.label, name)
			}
		}
		if a.isList {
			return nil, fmt.Errorf("%s takes no list", name)
		}
		kind := ""
		if j < len(kinds) {
			kind = kinds[j]
		}
		x, err := p.resolve(a.o, kind, "", "")
		if err != nil {
			return nil, err
		}
		out = append(out, x)
		j++
	}
	for ; j < len(labels); j++ {
		if d, ok := defs[j]; ok {
			s, _ := strconv.Unquote(d)
			out = append(out, Arg{Type: 0x80, N: (len(s)*6 + 7) / 8, S: s})
		}
	}
	return out, nil
}

// macro parses the calls that stand for several commands.
func (p *parser) macro(name string) error {
	pa, err := p.callArgs()
	if err != nil {
		return err
	}
	v := func(k int, kind string) (Arg, error) {
		if k >= len(pa) || pa[k].isList {
			return Arg{}, fmt.Errorf("%s wants operand %d", name, k+1)
		}
		return p.resolve(pa[k].o, kind, "", "")
	}
	variable := func(n string) Arg { return Arg{Type: 1, N: p.vars[n]} }
	switch name {
	case "ShowPortrait":
		h, err := v(0, "Head")
		if err != nil {
			return err
		}
		b, err := v(1, "Body")
		if err != nil || len(pa) != 2 {
			return fmt.Errorf("ShowPortrait(Head, Body)")
		}
		p.emit(opSave, h, variable(varPortrait))
		p.emit(opPicture, b)
	case "EnterArea":
		a, err := v(0, "Area")
		if err != nil || len(pa) != 1 || a.Type != 0 {
			return fmt.Errorf("EnterArea(Area)")
		}
		for _, ar := range p.names.Areas {
			if ar.ID == a.N {
				p.emit(opSave, Arg{N: ar.Set}, variable(varSymbolSet))
			}
		}
		p.emit(opNewECL, a)
	case "OpenTemple":
		if len(pa) != 0 {
			return fmt.Errorf("OpenTemple()")
		}
		p.emit(opSave, Arg{N: 1}, variable(varTemple))
		p.emit(opCombat)
	case "OpenShop":
		var items []string
		for _, a := range pa {
			items = append(items, a.o.name)
		}
		block, ok := valueOf(p.names.Values, "Stock", p.names.Set, strings.Join(items, ","))
		if !ok {
			return fmt.Errorf("no ITEM block holds the stock %s", strings.Join(items, ", "))
		}
		p.emit(opTreasure, Arg{}, Arg{}, Arg{}, Arg{}, Arg{}, Arg{}, Arg{}, Arg{N: block})
		p.emit(opSave, Arg{N: 1}, variable(varShop))
		p.emit(opSave, Arg{N: 1}, variable(varShopFlag))
		p.emit(opSave, Arg{N: 16}, variable(varShopKind))
		p.emit(opCombat)
	case "Call":
		a, err := v(0, "")
		if err != nil || len(pa) != 1 || a.Type != 2 {
			return fmt.Errorf("Call(0xADDR)")
		}
		p.emit(opCall, Arg{Type: 1, N: a.N})
	case "Compare": // each operand named beside the other
		if len(pa) != 2 || pa[0].isList || pa[1].isList {
			return fmt.Errorf("Compare(A, B)")
		}
		an := ""
		if pa[0].o.isVar() {
			an = pa[0].o.name
		}
		b, err := p.resolve(pa[1].o, "", an, "==")
		if err != nil {
			return err
		}
		x, err := p.resolve(pa[0].o, "", other2(b, p), "==")
		if err != nil {
			return err
		}
		p.emit(opCompare, x, b)
	case "LoadMap": // the byte code's second operand is unused
		m, err := v(0, "Map")
		if err != nil || len(pa) > 2 || len(pa) == 2 && pa[1].label != "icons" {
			return fmt.Errorf("LoadMap(Map, icons=Icons)")
		}
		icons := Arg{N: 0xFF}
		if len(pa) == 2 {
			if icons, err = v(1, "MapIcons"); err != nil {
				return err
			}
		}
		p.emit(opLoadMap, m, Arg{N: 0xFF}, icons)
	case "Treasure":
		args := make([]Arg, 8)
		args[7] = Arg{N: 0xFF}
		for _, a := range pa {
			k := slices.Index(treasureLabels, a.label)
			if k < 0 {
				return fmt.Errorf("Treasure(copper=, silver=, electrum=, gold=, platinum=, gems=, jewelry=, items=)")
			}
			switch {
			case k == 7 && a.isList:
				var items []string
				for _, o := range a.list {
					items = append(items, o.name)
				}
				block, ok := valueOf(p.names.Values, "Stock", p.names.Set, strings.Join(items, ","))
				if !ok {
					return fmt.Errorf("no ITEM block holds %s", strings.Join(items, ", "))
				}
				args[7] = Arg{N: block}
			case k == 7 && a.o.call == "Random":
				n, err := p.resolve(*a.o.callArg, "", "", "")
				if err != nil || n.Type != 0 {
					return fmt.Errorf("items=Random(N)")
				}
				args[7] = Arg{N: 0x80 + n.N}
			default:
				x, err := p.resolve(a.o, "", "", "")
				if err != nil {
					return err
				}
				args[k] = x
			}
		}
		p.emit(opTreasure, args...)
	}
	return p.endStmt()
}

// assign parses D = ..., D op= ..., T[I] = V.
func (p *parser) assign() error {
	d, err := p.operand()
	if err != nil {
		return err
	}
	if p.accept("[") { // T[I] = V
		i, err := p.operand()
		if err != nil {
			return err
		}
		if err := p.expect("]"); err != nil {
			return err
		}
		if err := p.expect("="); err != nil {
			return err
		}
		v, err := p.operand()
		if err != nil {
			return err
		}
		t, err := p.resolve(d, "", "", "")
		if err != nil {
			return err
		}
		ix, err := p.resolve(i, "", "", "")
		if err != nil {
			return err
		}
		x, err := p.resolve(v, "", "", "")
		if err != nil {
			return err
		}
		p.emit(opSaveTable, x, t, ix)
		return p.endStmt()
	}
	dest, err := p.resolve(d, "", "", "")
	if err != nil {
		return err
	}
	if dest.Type != 1 && dest.Type != 3 && dest.Type != 0x81 {
		return fmt.Errorf("%s is not a variable", d.text())
	}
	dn := d.name
	t := p.next()
	if t.k != tOp {
		return fmt.Errorf("want = after %s, have %s", d.text(), t.text())
	}
	switch t.s {
	case "+=", "-=":
		v, err := p.value(dn, "=")
		if err != nil {
			return err
		}
		p.emit(map[string]byte{"+=": opAdd, "-=": opSub}[t.s], v, dest, dest)
		return p.endStmt()
	case "|=", "&=":
		v, err := p.value(dn, t.s)
		if err != nil {
			return err
		}
		p.emit(map[string]byte{"|=": opOr, "&=": opAnd}[t.s], dest, v, dest)
		return p.endStmt()
	case "=":
	default:
		return fmt.Errorf("unexpected %s", t.s)
	}
	a, err := p.operand()
	if err != nil {
		return err
	}
	switch {
	case a.name != "" && p.accept("("): // D = Command(...)
		switch a.name {
		case "Random", "InputNumber", "InputString":
			pa, err := p.callArgs()
			if err != nil {
				return err
			}
			if len(pa) != 1 || pa[0].isList {
				return fmt.Errorf("%s(N)", a.name)
			}
			n, err := p.resolve(pa[0].o, "", "", "")
			if err != nil {
				return err
			}
			p.emit(map[string]byte{"Random": opRandom, "InputNumber": opInputNumber, "InputString": opInputString}[a.name], n, dest)
		case "PartyStrength":
			if err := p.expect(")"); err != nil {
				return err
			}
			p.emit(opPartyStr, dest)
		case "Parlay", "Menu", "VMenu":
			pa, err := p.callArgs()
			if err != nil {
				return err
			}
			var args []Arg
			for _, x := range pa {
				if x.isList || x.label != "" {
					return fmt.Errorf("%s takes plain operands", a.name)
				}
				v, err := p.resolve(x.o, "", "", "")
				if err != nil {
					return err
				}
				args = append(args, v)
			}
			switch a.name {
			case "Parlay":
				if len(args) != 5 {
					return fmt.Errorf("Parlay(haughty, sly, nice, meek, abusive)")
				}
				p.emit(opParlay, append(args, dest)...)
			case "Menu":
				p.emit(opHMenu, append([]Arg{dest, {N: len(args)}}, args...)...)
			case "VMenu":
				if len(args) < 1 {
					return fmt.Errorf("VMenu(prompt, items...)")
				}
				p.emit(opVMenu, append([]Arg{dest, args[0], {N: len(args) - 1}}, args[1:]...)...)
			}
		default:
			return fmt.Errorf("%s is not a command with a result", a.name)
		}
		return p.endStmt()
	case a.name != "" && p.accept("["): // D = T[I]
		i, err := p.operand()
		if err != nil {
			return err
		}
		if err := p.expect("]"); err != nil {
			return err
		}
		t, err := p.resolve(a, "", "", "")
		if err != nil {
			return err
		}
		ix, err := p.resolve(i, "", "", "")
		if err != nil {
			return err
		}
		p.emit(opGetTable, t, ix, dest)
		return p.endStmt()
	}
	if t := p.peek(); t.k == tOp && strings.Contains("+-*/&|", t.s) && len(t.s) == 1 {
		p.pos++
		other := ""
		if a.isVar() {
			other = a.name
		}
		b, err := p.value(other, t.s)
		if err != nil {
			return err
		}
		x, err := p.resolve(a, "", other2(b, p), t.s)
		if err != nil {
			return err
		}
		switch t.s {
		case "-":
			p.emit(opSub, b, x, dest)
		default:
			p.emit(map[string]byte{"+": opAdd, "*": opMul, "/": opDiv, "&": opAnd, "|": opOr}[t.s], x, b, dest)
		}
		return p.endStmt()
	}
	kind := ""
	if dn == varPortrait {
		kind = "Head"
	}
	x, err := p.resolve(a, kind, dn, "=")
	if err != nil {
		return err
	}
	p.emit(opSave, x, dest)
	return p.endStmt()
}

// other2 is the variable name of b, for naming a number beside it.
func other2(b Arg, p *parser) string {
	if b.Type == 1 || b.Type == 3 {
		return p.names.Vars[b.N]
	}
	return ""
}

// value parses an operand beside variable beside under op (a value or
// bit name of that variable).
func (p *parser) value(beside, op string) (Arg, error) {
	o, err := p.operand()
	if err != nil {
		return Arg{}, err
	}
	return p.resolve(o, "", beside, op)
}

// ifHead is an if's test: the If's operator, the comparison's operands
// when it made a Compare (else if A op B tests them again), or the call
// test whose operator the called section's returns decide.
type ifHead struct {
	op   byte
	cmp  []Arg
	call *callTest
}

// ifStmt parses if test { ... } [else if ... { ... }] [else { ... }]
// after if. Each statement of a block gets an If before it; an else if's
// test is (op) or the if's comparison with another operator, an else is
// the complement of the block before it.
func (p *parser) ifStmt() error {
	if p.depth > 0 {
		return fmt.Errorf("an if in an if")
	}
	h, err := p.test()
	if err != nil {
		return err
	}
	first := true // the if's own block, right after its comparison
	guard := func(ifop byte, call *callTest, neg bool) error {
		start := len(p.s.Code)
		p.depth++
		err := p.braces()
		p.depth--
		if err != nil {
			return err
		}
		var body []Instr
		body, p.s.Code = append(body, p.s.Code[start:]...), p.s.Code[:start]
		move := func(i *int) { // the body's commands move behind their Ifs
			if *i >= start {
				*i = start + 2*(*i-start) + 1
			}
		}
		for f := range p.fix {
			move(&p.fix[f].instr)
		}
		for r := range p.rets {
			move(&p.rets[r].cmp)
		}
		for r := range p.retOp {
			move(&p.retOp[r].cmp)
		}
		if first && len(body) > 0 && body[0].Op == opReturn && h.cmp != nil {
			p.retOp = append(p.retOp, retOp{start - 1, ifop}) // if A op B { return }: the if's operator is the return's
		}
		first = false
		for _, in := range body {
			i := p.emit(ifop)
			if call != nil {
				call.ifs = append(call.ifs, ifRef{i, neg})
			}
			p.s.Code = append(p.s.Code, in)
		}
		return nil
	}
	if err := guard(h.op, h.call, false); err != nil {
		return err
	}
	prev, prevCall := h.op, h.call // the block before: its operator, or the call deciding it
	for p.peek().k == tIdent && p.peek().s == "else" {
		p.pos++
		if p.peek().k == tIdent && p.peek().s == "if" {
			p.pos++
			op, err := p.elseTest(h)
			if err != nil {
				return err
			}
			if err := guard(op, nil, false); err != nil {
				return err
			}
			prev, prevCall = op, nil
			continue
		}
		if err := guard(elseIf(prev), prevCall, true); err != nil {
			return err
		}
		break
	}
	return p.endStmt()
}

// test parses an if's test, emitting its comparison: (op), Carried(X),
// not Carried(X), Foo() [op N | == "ANS"], not Foo(), D = A & B; D op 0,
// X has Bits, X lacks Bits, A op B [and C == D | or C != D].
func (p *parser) test() (ifHead, error) {
	t := p.peek()
	switch {
	case t.is("("): // (==): the flags as they are
		p.pos++
		o := p.next()
		op, ok := ifOf(o.s)
		if !ok || !p.accept(")") {
			return ifHead{}, fmt.Errorf("want (==) ... (>=)")
		}
		return ifHead{op: op}, nil
	case t.k == tIdent && t.s == "not":
		p.pos++
		n, _ := p.ident()
		if p.labels[n] && p.accept("(") {
			if err := p.expect(")"); err != nil {
				return ifHead{}, err
			}
			return p.callHead(n, true)
		}
		if n != "Carried" {
			return ifHead{}, fmt.Errorf("want not Carried(Item) or not Foo()")
		}
		op, err := p.carried()
		return ifHead{op: elseIf(op)}, err
	case t.k == tIdent && t.s == "Carried" && p.toks[p.pos+1].is("("):
		p.pos++
		op, err := p.carried()
		return ifHead{op: op}, err
	case t.k == tIdent && p.labels[t.s] && p.toks[p.pos+1].is("("):
		p.pos += 3
		return p.callHead(t.s, false)
	}
	start := p.pos
	a, err := p.operand()
	if err != nil {
		return ifHead{}, err
	}
	if o := p.peek(); o.is("=") || o.is("|=") || o.is("&=") { // D = A & B; D op 0
		p.pos = start
		if err := p.assign(); err != nil {
			return ifHead{}, err
		}
		l := len(p.s.Code) - 1
		if in := p.s.Code[l]; in.Op != opAnd && in.Op != opOr || len(in.Args) != 3 {
			return ifHead{}, fmt.Errorf("want D = A & B (or |) before ; D op 0")
		}
		d, err := p.operand()
		if err != nil {
			return ifHead{}, err
		}
		o := p.next()
		op, ok := ifOf(o.s)
		x, err := p.resolve(d, "", "", "")
		if z := p.next(); !ok || err != nil || x != p.s.Code[l].Args[2] || z.k != tInt || z.s != "0" {
			return ifHead{}, fmt.Errorf("want %s op 0 after the assignment", p.arg(p.s.Code[l].Args[2]))
		}
		return ifHead{op: flipIf(op)}, nil // the And's flags compare 0 with D
	}
	o := p.next()
	if o.k != tOp && !(o.k == tIdent && (o.s == "has" || o.s == "lacks")) {
		return ifHead{}, fmt.Errorf("want a comparison after %s, have %s", a.text(), o.text())
	}
	x, b, err := p.compared(a, o.s)
	if err != nil {
		return ifHead{}, err
	}
	if o.k == tIdent { // X has Bits: Scratch10 = X & Bits, 0 != Scratch10
		op := opIfNe
		if o.s == "lacks" {
			op = opIfEq
		}
		s, ok := p.vars[hasScratch]
		if !ok {
			return ifHead{}, fmt.Errorf("no variable %s for the bit test", hasScratch)
		}
		p.emit(opAnd, x, b, Arg{Type: 1, N: s})
		return ifHead{op: op}, nil
	}
	op, ok := ifOf(o.s)
	if !ok {
		return ifHead{}, fmt.Errorf("no comparison %s", o.s)
	}
	if w := p.peek(); w.k == tIdent && (w.s == "and" || w.s == "or") {
		p.pos++
		if w.s == "and" && op != opIfEq || w.s == "or" && op != opIfNe {
			return ifHead{}, fmt.Errorf("want A == B and C == D, or A != B or C != D")
		}
		c, err := p.operand()
		if err != nil {
			return ifHead{}, err
		}
		if !p.accept(o.s) {
			return ifHead{}, fmt.Errorf("want %s", o.s)
		}
		y, d, err := p.compared(c, o.s)
		if err != nil {
			return ifHead{}, err
		}
		p.emit(opCompareAnd, x, b, y, d)
		return ifHead{op: op}, nil
	}
	p.emit(opCompare, x, b)
	return ifHead{op: op, cmp: []Arg{x, b}}, nil
}

// arg is a's text, for an error.
func (p *parser) arg(a Arg) string {
	if n := p.names.Vars[a.N]; (a.Type == 1 || a.Type == 3) && n != "" {
		return n
	}
	return fmt.Sprint(a)
}

// carried parses (Item) after Carried: FindItem, true with ==.
func (p *parser) carried() (byte, error) {
	if err := p.expect("("); err != nil {
		return 0, err
	}
	pa, err := p.callArgs()
	if err != nil {
		return 0, err
	}
	if len(pa) != 1 || pa[0].isList {
		return 0, fmt.Errorf("Carried(Item)")
	}
	item, err := p.resolve(pa[0].o, "Item", "", "")
	if err != nil {
		return 0, err
	}
	p.emit(opFindItem, item)
	return opIfEq, nil
}

// callHead parses what follows Foo() in a test: nothing (the section's
// return A op B, not: its complement), op N (its return X compared with
// N) or == "ANS" (a menu section's answer); the Ifs' operator is set by
// finish.
func (p *parser) callHead(name string, not bool) (ifHead, error) {
	p.jump(p.emit(opGosub, Arg{Type: 1, Label: true}), 0, name)
	c := &callTest{label: name, line: p.line(), not: not}
	if o := p.peek(); o.k == tOp && isCmp(o.s) {
		if not {
			return ifHead{}, fmt.Errorf("not %s() takes no comparison", name)
		}
		p.pos++
		c.op = o.s
		if t := p.peek(); t.k == tString {
			p.pos++
			c.ans, c.isAns = t.s, true
		} else if n, err := p.operand(); err != nil {
			return ifHead{}, err
		} else {
			c.n = n
		}
	}
	p.calls = append(p.calls, c)
	return ifHead{op: opIfEq, call: c}, nil
}

// isCmp: a comparison operator.
func isCmp(op string) bool { _, ok := ifOf(op); return ok }

// elseTest parses an else if's test: (op), or the if's comparison with
// another operator (A op B, the same operands).
func (p *parser) elseTest(h ifHead) (byte, error) {
	if p.accept("(") {
		o := p.next()
		op, ok := ifOf(o.s)
		if !ok || !p.accept(")") {
			return 0, fmt.Errorf("want else if (==) ... (>=)")
		}
		return op, nil
	}
	a, err := p.operand()
	if err != nil {
		return 0, err
	}
	o := p.next()
	op, ok := ifOf(o.s)
	if !ok {
		return 0, fmt.Errorf("want a comparison after %s, have %s", a.text(), o.text())
	}
	x, b, err := p.compared(a, o.s)
	if err != nil {
		return 0, err
	}
	if !slices.Equal([]Arg{x, b}, h.cmp) {
		return 0, fmt.Errorf("an else if tests the if's comparison; this compares something else")
	}
	return op, nil
}

// forStmt parses for X = A to B { } / for X = A while X < B { } after for.
func (p *parser) forStmt() error {
	xo, err := p.operand()
	if err != nil {
		return err
	}
	x, err := p.resolve(xo, "", "", "")
	if err != nil {
		return err
	}
	if x.Type != 1 {
		return fmt.Errorf("%s is not a variable", xo.text())
	}
	if err := p.expect("="); err != nil {
		return err
	}
	a, err := p.value(xo.name, "=")
	if err != nil {
		return err
	}
	w, _ := p.ident()
	var b Arg
	ifop := opIfLe
	switch w {
	case "to":
		if b, err = p.value(xo.name, "<="); err != nil {
			return err
		}
	case "while":
		if x2, err := p.operand(); err != nil || x2 != xo {
			return fmt.Errorf("want while %s < B", xo.text())
		}
		if !p.accept("<") {
			return fmt.Errorf("want while %s < B", xo.text())
		}
		ifop = opIfLt
		if b, err = p.value(xo.name, "<"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("want to or while")
	}
	p.emit(opSave, a, x)
	l := len(p.s.Code)
	if err := p.label(fmt.Sprintf("loop%d", p.line())); err != nil { // the loop's own label
		return err
	}
	if err := p.braces(); err != nil {
		return err
	}
	p.emit(opAdd, Arg{N: 1}, x, x)
	p.emit(opCompare, x, b)
	p.emit(ifop)
	p.emit(opGoto, Arg{Type: 1, Label: true, N: l})
	return p.endStmt()
}

// opnd is an operand as written.
type opnd struct {
	num     int
	isNum   bool
	hex     bool
	neg     bool
	str     string
	isStr   bool
	sigil   string // $ & ~
	name    string // a name, or A|B
	call    string // Random in Random(N)
	callArg *opnd
}

func (o opnd) text() string {
	switch {
	case o.isStr:
		return strconv.Quote(o.str)
	case o.isNum:
		return strconv.Itoa(o.num)
	}
	return o.sigil + o.name
}

// isVar: a plain name, maybe a variable's.
func (o opnd) isVar() bool { return !o.isNum && !o.isStr && (o.sigil == "" || o.sigil == "&") }

// operand parses one operand.
func (p *parser) operand() (opnd, error) {
	var o opnd
	t := p.next()
	switch {
	case t.k == tString:
		return opnd{str: t.s, isStr: true}, nil
	case t.is("-"):
		t = p.next()
		if t.k != tInt {
			return o, fmt.Errorf("want a number after -")
		}
		o.neg = true
		fallthrough
	case t.k == tInt:
		v, err := strconv.ParseInt(t.s, 0, 32)
		if err != nil {
			return o, fmt.Errorf("number %s", t.s)
		}
		o.num, o.isNum, o.hex = int(v), true, strings.HasPrefix(t.s, "0x")
		return o, nil
	case t.is("$") || t.is("&") || t.is("~"):
		o.sigil = t.s
		t = p.next()
		if o.sigil == "~" && t.is("(") {
			for {
				n, err := p.ident()
				if err != nil {
					return o, err
				}
				o.name += n
				if p.accept(")") {
					break
				}
				if err := p.expect("|"); err != nil {
					return o, err
				}
				o.name += "|"
			}
			return o, nil
		}
	}
	if t.k != tIdent {
		return o, fmt.Errorf("want an operand, have %s", t.text())
	}
	o.name = t.s
	for o.sigil == "" && !p.isVar(o.name) && p.peek().is("|") && p.toks[p.pos+1].k == tIdent && !p.isVar(p.toks[p.pos+1].s) { // a union of bit names
		p.pos++
		n, _ := p.ident()
		o.name += "|" + n
	}
	return o, nil
}

// resolve is operand o as an Arg: a number, a string, a variable, label,
// table or area by name, a value of kind, or a value or bits of the
// variable beside under op.
func (p *parser) resolve(o opnd, kind, beside, op string) (Arg, error) {
	switch {
	case o.isStr:
		return Arg{Type: 0x80, N: (len(o.str)*6 + 7) / 8, S: o.str}, nil
	case o.isNum && (o.hex || o.num > 0xFF && !o.neg): // a word
		if o.num < 0 || o.num > 0xFFFF {
			return Arg{}, fmt.Errorf("the word 0x%X is out of range", o.num)
		}
		return Arg{Type: 2, N: o.num}, nil
	case o.isNum:
		v := o.num
		if o.neg {
			v = 256 - v
		}
		if v < 0 || v > 255 {
			return Arg{}, fmt.Errorf("the byte %d is out of range", o.num)
		}
		return Arg{N: v}, nil
	case o.sigil == "$" || o.sigil == "&":
		a, ok := p.vars[o.name]
		if !ok {
			return Arg{}, fmt.Errorf("no variable %s", o.name)
		}
		if o.sigil == "$" {
			return Arg{Type: 0x81, N: a}, nil
		}
		return Arg{Type: 3, N: a}, nil
	case o.sigil == "~":
		if beside == "" {
			return Arg{}, fmt.Errorf("~%s: no variable to name the bits of", o.name)
		}
		v, ok := p.names.varNumber(beside, "~"+o.name)
		if !ok {
			return Arg{}, fmt.Errorf("%s has no bits %s", beside, o.name)
		}
		return Arg{N: v}, nil
	}
	if p.labels[o.name] {
		return Arg{Type: 1, Label: true, N: -1, S: o.name}, nil // resolved by finish
	}
	if p.tables[o.name] {
		return Arg{Type: 1, N: -1, S: "[" + o.name}, nil // resolved by finish
	}
	if a, ok := p.vars[o.name]; ok {
		return Arg{Type: 1, N: a}, nil
	}
	if v, ok := p.names.kindValue(kind, o.name); ok {
		return Arg{N: v}, nil
	}
	if beside != "" {
		if v, ok := p.names.varNumber(beside, o.name); ok {
			return Arg{N: v}, nil
		}
	}
	if strings.HasPrefix(o.name, "V_") {
		if v, err := strconv.ParseUint(o.name[2:], 16, 16); err == nil {
			return Arg{Type: 1, N: int(v)}, nil
		}
	}
	if kind != "" {
		return Arg{}, fmt.Errorf("no %s %s", strings.ToLower(kind), o.name)
	}
	if beside != "" {
		return Arg{}, fmt.Errorf("%s is no variable, and no value of %s", o.name, beside)
	}
	return Arg{}, fmt.Errorf("no variable %s", o.name)
}

// finish resolves the labels, leaves out the comparisons the flags held
// and sets the events.
func (p *parser) finish() error {
	for i := range p.s.Code {
		for k, a := range p.s.Code[i].Args {
			switch {
			case a.Label && a.N == -1:
				p.jump(i, k, a.S)
				p.s.Code[i].Args[k].S = ""
			case a.Type == 1 && a.N == -1 && strings.HasPrefix(a.S, "["): // a table
				at, ok := p.s.Tables[a.S[1:]]
				if !ok {
					return fmt.Errorf("%d: no table %s", p.s.Code[i].Line, a.S[1:])
				}
				p.s.Code[i].Args[k] = Arg{Type: 1, N: at}
			}
		}
	}
	var menus []fixup
	for _, f := range p.fix {
		l, ok := p.s.Labels[f.label]
		if !ok {
			return fmt.Errorf("%d: no label %s", p.s.Code[f.instr].Line, f.label)
		}
		if f.arg == -1 { // a menu section's variable, once its return is known
			menus = append(menus, f)
			continue
		}
		p.s.Code[f.instr].Args[f.arg].N = l
	}
	if err := p.callTests(); err != nil {
		return err
	}
	for _, f := range menus {
		d, _, _, isMenu := p.s.menuOf(p.s.Labels[f.label])
		if !isMenu {
			return fmt.Errorf("%d: %s is not a menu section", p.s.Code[f.instr].Line, f.label)
		}
		p.s.Code[f.instr].Args[0] = d
	}
	for i := range p.s.Events {
		at, ok := p.s.Labels[eventLabel(i)]
		if !ok {
			return fmt.Errorf("no on %s", eventNames[i])
		}
		p.s.Events[i] = at
	}
	p.s.dropNextGotos()
	return nil
}

// dropNextGotos leaves out the gotos to the label right after them: the
// text ends every run of commands that goes on into a label with one,
// and the byte code jumps nowhere there (a jump to the next command, as
// a few originals have, does nothing and goes the same way).
func (s *Script) dropNextGotos() {
	index := make([]int, len(s.Code)+1)
	var code []Instr
	for i, in := range s.Code {
		index[i] = len(code)
		if in.Op == opGoto && len(in.Args) == 1 && in.Args[0].Label && in.Args[0].N == i+1 && !(i > 0 && isIf(s.Code[i-1].Op)) {
			continue
		}
		code = append(code, in)
	}
	index[len(s.Code)] = len(code)
	if len(code) == len(s.Code) {
		return
	}
	for i := range code {
		for k, a := range code[i].Args {
			if a.Label {
				code[i].Args[k].N = index[a.N]
			}
		}
	}
	for l, i := range s.Labels {
		s.Labels[l] = index[i]
	}
	at := map[int]string{}
	for i, l := range s.At {
		if _, ok := at[index[i]]; !ok || strings.HasPrefix(at[index[i]], "event") && !strings.HasPrefix(l, "event") {
			at[index[i]] = l
		}
	}
	for i := range s.Events {
		s.Events[i] = index[s.Events[i]]
	}
	s.Code, s.At = code, at
}

// callTests sets the operator of the Ifs on called sections' results,
// and the number the sections' return X compare with: if Foo() tests
// with Foo's return A op B; if Foo() op N with op, N going into Foo's
// Compare X N (a menu answer "ANS" is its number on the menu); after D =
// A & B; return D, if Foo() op 0 tests 0 op D.
func (p *parser) callTests() error {
	set := map[int]string{} // a return's number, by the call that set it
	for _, c := range p.calls {
		op, _ := ifOf(c.op)
		_, rc := p.s.firstReturn(p.s.Labels[c.label])
		if c.op == "" {
			k := slices.IndexFunc(p.retOp, func(r retOp) bool { return r.cmp == rc })
			if rc < 0 || k < 0 {
				return fmt.Errorf("%d: %s() returns no comparison (return A op B) to test", c.line, c.label)
			}
			op = p.retOp[k].op
			if c.not {
				op = elseIf(op)
			}
		} else {
			andRet := rc >= 0 && p.s.Code[rc].Op != opCompare
			for _, r := range p.rets {
				if r.section != c.label || r.cmp < 0 {
					continue
				}
				x := p.s.Code[r.cmp].Args[0]
				var n Arg
				if c.isAns {
					d, items := p.menuItems(p.s.Labels[c.label])
					j := slices.Index(items, c.ans)
					if j < 0 || d != x {
						return fmt.Errorf("%d: %s() is not a menu section answering %q", c.line, c.label, c.ans)
					}
					n = Arg{N: j}
				} else {
					var err error
					if n, err = p.resolve(c.n, "", p.names.Vars[x.N], c.op); err != nil {
						return fmt.Errorf("%d: %v", c.line, err)
					}
				}
				if by, ok := set[r.cmp]; ok && p.s.Code[r.cmp].Args[1] != n {
					return fmt.Errorf("%d: %s() compares %s with %s here and %s at %s", c.line, c.label, p.arg(x), c.n.text(), p.arg(p.s.Code[r.cmp].Args[1]), by)
				}
				p.s.Code[r.cmp].Args[1] = n
				set[r.cmp] = strconv.Itoa(c.line)
			}
			if andRet {
				if c.isAns || !c.n.isNum || c.n.num != 0 {
					return fmt.Errorf("%d: %s() returns D = A & B: test it against 0", c.line, c.label)
				}
				op = flipIf(op)
			}
		}
		for _, f := range c.ifs {
			if f.neg {
				p.s.Code[f.instr].Op = elseIf(op)
			} else {
				p.s.Code[f.instr].Op = op
			}
		}
	}
	for _, r := range p.rets {
		if _, ok := set[r.cmp]; r.cmp >= 0 && !ok {
			return fmt.Errorf("%d: return %s: no if %s() op N says what it compares with", p.s.Code[r.cmp].Line, p.arg(p.s.Code[r.cmp].Args[0]), r.section)
		}
	}
	return nil
}

// menuItems is the variable and items of the one-line menu the section
// at l asks.
func (p *parser) menuItems(l int) (d Arg, items []string) {
	for i := l; i < len(p.s.Code); i++ {
		in := p.s.Code[i]
		if in.Op == opHMenu && len(in.Args) >= 2 {
			for _, a := range in.Args[2:] {
				items = append(items, a.S)
			}
			return in.Args[0], items
		}
		if isTerminator(in.Op) {
			break
		}
	}
	return d, nil
}
