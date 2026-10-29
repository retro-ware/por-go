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
// one after the other; an if guards each command of its block with an If
// (and makes its comparison first unless the flags hold it, scriptform.go).
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
	fix    []fixup   // jump operands to resolve to their labels
	elide  []elision // the comparisons an if made, left out where the flags held them
	depth  int       // in an if block: nothing but statements
}

// fixup is a jump operand waiting for its label's command number.
type fixup struct {
	instr, arg int
	label      string
}

// elision is a comparison an if made at command at, guarding the Ifs.
type elision struct {
	at  int
	ifs []int
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
			if err := p.braces(); err != nil {
				return err
			}
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
			if err := p.braces(); err != nil {
				return err
			}
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
		p.emit(opReturn)
		return p.endStmt()
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
	case "ShowPortrait", "EnterArea", "OpenTemple", "OpenShop", "Treasure", "Call":
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

// ifStmt parses if test { ... } [else { ... }] after if.
func (p *parser) ifStmt() error {
	op, setter, err := p.test()
	if err != nil {
		return err
	}
	var el *elision
	if setter >= 0 {
		p.elide = append(p.elide, elision{at: setter})
		el = &p.elide[len(p.elide)-1]
	}
	guard := func(ifop byte) error {
		start := len(p.s.Code)
		p.depth++
		err := p.braces()
		p.depth--
		if err != nil {
			return err
		}
		var body []Instr
		body, p.s.Code = append(body, p.s.Code[start:]...), p.s.Code[:start]
		for f := range p.fix { // the jumps in the body move behind their Ifs
			if p.fix[f].instr >= start {
				p.fix[f].instr = start + 2*(p.fix[f].instr-start) + 1
			}
		}
		for _, in := range body {
			i := p.emit(ifop)
			if el != nil {
				el.ifs = append(el.ifs, i)
			}
			p.s.Code = append(p.s.Code, in)
		}
		return nil
	}
	if err := guard(op); err != nil {
		return err
	}
	if p.peek().k == tIdent && p.peek().s == "else" {
		p.pos++
		if err := guard(elseIf(op)); err != nil {
			return err
		}
	}
	return p.endStmt()
}

// test parses an if's test: the If opcode and the comparing command it
// emitted (-1 for none: the flags as they are, or a bit test's And).
func (p *parser) test() (byte, int, error) {
	t := p.peek()
	switch {
	case t.is("("): // (==): the flags as they are
		p.pos++
		o := p.next()
		op, ok := ifOf(o.s)
		if !ok || !p.accept(")") {
			return 0, 0, fmt.Errorf("want (==) ... (>=)")
		}
		return op, -1, nil
	case t.k == tIdent && (t.s == "not" || t.s == "Carried"):
		op := opIfEq
		if t.s == "not" {
			p.pos++
			op = opIfNe
		}
		if n, _ := p.ident(); n != "Carried" || !p.accept("(") {
			return 0, 0, fmt.Errorf("want Carried(Item)")
		}
		pa, err := p.callArgs()
		if err != nil {
			return 0, 0, err
		}
		if len(pa) != 1 || pa[0].isList {
			return 0, 0, fmt.Errorf("Carried(Item)")
		}
		item, err := p.resolve(pa[0].o, "Item", "", "")
		if err != nil {
			return 0, 0, err
		}
		return op, p.emit(opFindItem, item), nil
	case t.k == tIdent && p.labels[t.s] && p.toks[p.pos+1].is("("): // Menu() == "YES"
		p.pos += 3
		o := p.next()
		ans := p.next()
		if !o.is("==") && !o.is("!=") || ans.k != tString {
			return 0, 0, fmt.Errorf("want %s() == \"ANSWER\"", t.s)
		}
		g := p.emit(opGosub, Arg{Type: 1, Label: true})
		p.jump(g, 0, t.s)
		c := p.emit(opCompare)
		p.fix = append(p.fix, fixup{c, -2, t.s + "\x00" + o.s + "\x00" + ans.s}) // the menu's test, once the section is known
		return opIfEq, c, nil
	}
	a, err := p.operand()
	if err != nil {
		return 0, 0, err
	}
	o := p.next()
	if o.k != tOp && !(o.k == tIdent && (o.s == "has" || o.s == "lacks")) {
		return 0, 0, fmt.Errorf("want a comparison after %s, have %s", a.text(), o.text())
	}
	an := ""
	if a.isVar() {
		an = a.name
	}
	b, err := p.value(an, o.s)
	if err != nil {
		return 0, 0, err
	}
	x, err := p.resolve(a, "", other2(b, p), o.s)
	if err != nil {
		return 0, 0, err
	}
	if o.k == tIdent { // X has Bits: Scratch10 = X & Bits, 0 != Scratch10
		op := opIfNe
		if o.s == "lacks" {
			op = opIfEq
		}
		s, ok := p.vars[hasScratch]
		if !ok {
			return 0, 0, fmt.Errorf("no variable %s for the bit test", hasScratch)
		}
		p.emit(opAnd, x, b, Arg{Type: 1, N: s})
		return op, -1, nil
	}
	op, ok := ifOf(o.s)
	if !ok {
		return 0, 0, fmt.Errorf("no comparison %s", o.s)
	}
	if w := p.peek(); w.k == tIdent && (w.s == "and" || w.s == "or") {
		p.pos++
		if w.s == "and" && op != opIfEq || w.s == "or" && op != opIfNe {
			return 0, 0, fmt.Errorf("want A == B and C == D, or A != B or C != D")
		}
		c, err := p.operand()
		if err != nil {
			return 0, 0, err
		}
		if !p.accept(o.s) {
			return 0, 0, fmt.Errorf("want %s", o.s)
		}
		cn := ""
		if c.isVar() {
			cn = c.name
		}
		d, err := p.value(cn, o.s)
		if err != nil {
			return 0, 0, err
		}
		y, err := p.resolve(c, "", other2(d, p), o.s)
		if err != nil {
			return 0, 0, err
		}
		return op, p.emit(opCompareAnd, x, b, y, d), nil
	}
	return op, p.emit(opCompare, x, b), nil
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
	for _, f := range p.fix {
		l, ok := p.s.Labels[f.label]
		if f.arg == -2 { // a menu section's test: its answer
			name, rest, _ := strings.Cut(f.label, "\x00")
			op, ans, _ := strings.Cut(rest, "\x00")
			l, ok = p.s.Labels[name]
			if !ok {
				return fmt.Errorf("%d: no label %s", p.s.Code[f.instr].Line, name)
			}
			d, items, k, isMenu := p.s.menuOf(l)
			j := slices.Index(items, ans)
			if !isMenu || j < 0 {
				return fmt.Errorf("%d: %s is not a menu section asking %q", p.s.Code[f.instr].Line, name, ans)
			}
			p.s.Code[f.instr].Args = []Arg{d, {N: k}}
			ifop := opIfEq
			switch {
			case op == "==" && j == k:
			case op == "==" && len(items) == 2, op == "!=" && j == k:
				ifop = opIfNe
			default:
				return fmt.Errorf("%d: %s compares its answer with %q", p.s.Code[f.instr].Line, name, items[k])
			}
			for _, e := range p.elide {
				if e.at == f.instr {
					for _, i := range e.ifs {
						p.s.Code[i].Op = ifop
					}
				}
			}
			continue
		}
		if !ok {
			return fmt.Errorf("%d: no label %s", p.s.Code[f.instr].Line, f.label)
		}
		if f.arg == -1 { // a menu section's variable
			d, _, _, isMenu := p.s.menuOf(l)
			if !isMenu {
				return fmt.Errorf("%d: %s is not a menu section", p.s.Code[f.instr].Line, f.label)
			}
			p.s.Code[f.instr].Args[0] = d
			continue
		}
		p.s.Code[f.instr].Args[f.arg].N = l
	}
	for i := range p.s.Events {
		at, ok := p.s.Labels[eventLabel(i)]
		if !ok {
			return fmt.Errorf("no on %s", eventNames[i])
		}
		p.s.Events[i] = at
	}
	// the comparisons the flags held already
	before := scriptFlags(p.s)
	drop := map[int]bool{}
	for _, e := range p.elide {
		in := p.s.Code[e.at]
		if ok, flipped := before[e.at].holds(in.Op, in.Args); ok {
			drop[e.at] = true
			if flipped {
				for _, i := range e.ifs {
					p.s.Code[i].Op = flipIf(p.s.Code[i].Op)
				}
			}
		}
	}
	if len(drop) == 0 {
		return nil
	}
	index := make([]int, len(p.s.Code)+1)
	var code []Instr
	for i, in := range p.s.Code {
		index[i] = len(code)
		if !drop[i] {
			code = append(code, in)
		}
	}
	index[len(p.s.Code)] = len(code)
	for i := range code {
		for k, a := range code[i].Args {
			if a.Label {
				code[i].Args[k].N = index[a.N]
			}
		}
	}
	for l, i := range p.s.Labels {
		p.s.Labels[l] = index[i]
	}
	at := map[int]string{}
	for i, l := range p.s.At {
		if _, ok := at[index[i]]; !ok || p.s.Labels[l] == index[i] && strings.HasPrefix(at[index[i]], "event") {
			at[index[i]] = l
		}
	}
	for i := range p.s.Events {
		p.s.Events[i] = index[p.s.Events[i]]
	}
	p.s.Code, p.s.At = code, at
	return nil
}
