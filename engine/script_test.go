package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"por/dax"
)

// TestScriptRoundTrip: every area's byte code, written as a script and
// parsed back, is the same Script (cmd/eclscript's output parses to the
// byte code's commands).
func TestScriptRoundTrip(t *testing.T) {
	l, err := dax.NewLoader("../data/dos")
	if err != nil {
		t.Skip(err)
	}
	dir := "../data/assets/scripts"
	out := os.Getenv("POR_SCRIPTS_OUT") // write the scripts there too
	for set := 1; set <= 8; set++ {
		for block := 0; block < 40; block++ {
			b := l.Block(fmt.Sprintf("ECL%d", set), block)
			if len(b) < 2 {
				continue
			}
			names, err := ReadAreaNames(dir, block, true)
			if err != nil {
				t.Fatal(err)
			}
			if names.Set != set {
				continue
			}
			want, err := DecodeScript(b[2:], names)
			if err != nil {
				t.Fatalf("area %d: decode: %v", block, err)
			}
			text, err := WriteScript(want, names)
			if err != nil {
				t.Errorf("area %d: write: %v", block, err)
				continue
			}
			if out != "" {
				os.WriteFile(filepath.Join(out, fmt.Sprintf("area%d.txt", block)), []byte(text), 0o644)
			}
			got, err := ParseScript(text, names)
			if err != nil {
				t.Errorf("area %d: parse: %v", block, err)
				continue
			}
			for i := range got.Code {
				got.Code[i].Line, got.Code[i].Comment = 0, ""
			}
			for i := range want.Code {
				want.Code[i].Comment = ""
			}
			if out != "" && len(got.Code) != len(want.Code) {
				back, _ := WriteScript(got, names)
				os.WriteFile(filepath.Join(out, fmt.Sprintf("area%d.got.txt", block)), []byte(back), 0o644)
			}
			for i := range want.Code {
				if i >= len(got.Code) || !reflect.DeepEqual(want.Code[i], got.Code[i]) {
					g := "none"
					if i < len(got.Code) {
						g = fmt.Sprint(got.Code[i])
					}
					t.Errorf("area %d: command %d (%s): want %v, got %s", block, i, where(want, i), want.Code[i], g)
					break
				}
			}
			if len(got.Code) != len(want.Code) {
				t.Errorf("area %d: %d commands, want %d", block, len(got.Code), len(want.Code))
			}
			if got.Events != want.Events {
				t.Errorf("area %d: events %v, want %v", block, got.Events, want.Events)
			}
			if !reflect.DeepEqual(namedLabels(got), namedLabels(want)) {
				t.Errorf("area %d: labels differ", block)
			}
			if !reflect.DeepEqual(got.Tables, want.Tables) || string(got.Data) != string(want.Data) {
				t.Errorf("area %d: tables differ", block)
			}
		}
	}
}

// namedLabels are s's labels but the loops' (for X = A to B {} names
// its own).
func namedLabels(s *Script) map[string]int {
	out := map[string]int{}
	for l, i := range s.Labels {
		loop := false
		for g, in := range s.Code {
			if in.Op == opGoto && in.Args[0].N == i && g >= 3 && isIf(s.Code[g-1].Op) && s.Code[g-2].Op == opCompare && s.Code[g-3].Op == opAdd {
				loop = true
			}
		}
		if !loop {
			out[l] = i
		}
	}
	return out
}

func where(s *Script, i int) string {
	for j := i; j >= 0; j-- {
		if l := s.At[j]; l != "" {
			return fmt.Sprintf("%s+%d", l, i-j)
		}
	}
	return "?"
}

// TestParseForms: the forms the writer makes, by hand.
func TestParseForms(t *testing.T) {
	names := &ScriptNames{Vars: ScriptVarNames(), Values: []ScriptValue{
		{"Facing", 0, 2, "South"}, {"CastleFlags", 0, 1, "Disguised"}, {"CastleFlags", 0, 2, "HallSeen"},
		{"Monster", 0, 3, "Kobold"}, {"Icon", 0, 5, "KoboldIcon"}, {"Item", 0, 7, "LongSword"},
	}}
	names.Vars[0x4A65] = "CastleFlags"
	for i := 1; i <= 10; i++ {
		names.Vars[0x6E78+i] = fmt.Sprintf("Scratch%d", i)
	}
	vars := map[string]int{}
	for a, n := range names.Vars {
		vars[n] = a
	}
	v := func(n string) Arg { return Arg{Type: 1, N: vars[n]} }
	src := `
on load { exit }
on move { exit }
on camp { exit }
on rest { exit }
on step {
    if Facing == South { goto Ahead }
    if Facing < South {
        Sky = 3
        Sky += 1
    } else {
        CastleFlags |= Disguised|HallSeen
    }
    if CastleFlags has Disguised { Done() }
    if Carried(LongSword) { Text("YES") }
    Scratch1 = Scratch2 & 7
    if Scratch1 != 0 { Print(Scratch1) }
    for Scratch3 = 0 to 4 { Scratch4 = Kinds[Scratch3] }
    LoadMonster(Kobold, count=8, icon=KoboldIcon)
    goto [Ahead, Later][Scratch1]
}
Ahead:
    Treasure(gold=5, items=Random(2))
    exit
Later:
    exit
def Done() {
    return
}
# tables
Kinds = [1, 2, 3]
`
	s, err := ParseScript(src, names)
	if err != nil {
		t.Fatal(err)
	}
	ahead, done, later, kinds := s.Labels["Ahead"], s.Labels["Done"], s.Labels["Later"], Arg{Type: 1, N: eclBase}
	want := []Instr{
		{Op: opExit}, {Op: opExit}, {Op: opExit}, {Op: opExit},
		{Op: opCompare, Args: []Arg{v("Facing"), {N: 2}}}, {Op: opIfEq}, {Op: opGoto, Args: []Arg{{Type: 1, N: ahead, Label: true}}},
		{Op: opIfLt}, {Op: opSave, Args: []Arg{{N: 3}, v("Sky")}}, {Op: opIfLt}, {Op: opAdd, Args: []Arg{{N: 1}, v("Sky"), v("Sky")}},
		{Op: opIfGe}, {Op: opOr, Args: []Arg{v("CastleFlags"), {N: 3}, v("CastleFlags")}},
		{Op: opAnd, Args: []Arg{v("CastleFlags"), {N: 1}, v("Scratch10")}}, {Op: opIfNe}, {Op: opGosub, Args: []Arg{{Type: 1, N: done, Label: true}}},
		{Op: opFindItem, Args: []Arg{{N: 7}}}, {Op: opIfEq}, {Op: opPrintClear, Args: []Arg{{Type: 0x80, N: 3, S: "YES"}}},
		{Op: opAnd, Args: []Arg{v("Scratch2"), {N: 7}, v("Scratch1")}}, {Op: opIfNe}, {Op: opPrint, Args: []Arg{v("Scratch1")}},
		{Op: opSave, Args: []Arg{{N: 0}, v("Scratch3")}},
		{Op: opGetTable, Args: []Arg{kinds, v("Scratch3"), v("Scratch4")}},
		{Op: opAdd, Args: []Arg{{N: 1}, v("Scratch3"), v("Scratch3")}}, {Op: opCompare, Args: []Arg{v("Scratch3"), {N: 4}}}, {Op: opIfLe},
		{Op: opGoto, Args: []Arg{{Type: 1, N: 23, Label: true}}},
		{Op: 0x0B, Args: []Arg{{N: 3}, {N: 8}, {N: 5}}},
		{Op: opOnGoto, Args: []Arg{v("Scratch1"), {N: 2}, {Type: 1, N: ahead, Label: true}, {Type: 1, N: later, Label: true}}},
		{Op: opTreasure, Args: []Arg{{}, {}, {}, {N: 5}, {}, {}, {}, {N: 0x82}}}, {Op: opExit},
		{Op: opExit}, {Op: opReturn},
	}
	for i := range s.Code {
		s.Code[i].Line = 0
	}
	for i, w := range want {
		if i >= len(s.Code) || !reflect.DeepEqual(w, s.Code[i]) {
			g := "none"
			if i < len(s.Code) {
				g = fmt.Sprint(s.Code[i])
			}
			t.Fatalf("command %d: want %v, got %s", i, w, g)
		}
	}
	if len(s.Code) != len(want) || string(s.Data) != "\x01\x02\x03" {
		t.Fatalf("%d commands, want %d; data %v", len(s.Code), len(want), s.Data)
	}
	text, err := WriteScript(s, names)
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range []string{"if Facing == South { goto Ahead }", "} else {", "CastleFlags |= Disguised|HallSeen",
		"if CastleFlags has Disguised { Done() }", "if Carried(LongSword) { Text(\"YES\") }", "if Scratch1 != 0 { Print(Scratch1) }",
		"for Scratch3 = 0 to 4 {", "LoadMonster(Kobold, count=8, icon=KoboldIcon)", "goto [Ahead, Later][Scratch1]",
		"Treasure(gold=5, items=Random(2))", "def Done() {", "Kinds = [1, 2, 3]"} {
		if !strings.Contains(text, form) {
			t.Errorf("the writer lost %q:\n%s", form, text)
		}
	}
}
