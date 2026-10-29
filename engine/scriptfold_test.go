package engine

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"por/dax"
)

// Every area's folded script reads back to its plain script's commands.
func TestFoldScript(t *testing.T) {
	l, err := dax.NewLoader("../data/dos")
	if err != nil || !l.Exists("ECL1.DAX") {
		t.Skip("no game files")
	}
	areas, err := ReadScriptAreas("../data/assets/scripts/names/areas.txt")
	if err != nil {
		t.Fatal(err)
	}
	names := ScriptVarNames()
	vars := map[string]int{}
	for a, n := range names {
		vars[n] = a
	}
	n := 0
	for disk := 1; disk <= 8; disk++ {
		for block := range 40 {
			b := l.Block(fmt.Sprintf("ECL%d", disk), block)
			if len(b) < 2 {
				continue
			}
			var plain strings.Builder
			if err := scriptPlain(b[2:], &plain, names); err != nil {
				t.Fatal(err)
			}
			folded := foldScript(plain.String(), areas)
			want, err := ParseScript(plain.String(), vars, areas)
			if err != nil {
				t.Fatalf("ECL%d %d: %v\n%.300s", disk, block, err, plain.String())
			}
			got, err := ParseScript(folded, vars, areas)
			if err != nil {
				os.WriteFile(t.TempDir()+"/f.txt", []byte(folded), 0o644)
				t.Fatalf("ECL%d %d: %v", disk, block, err)
			}
			for i := range want.Code {
				want.Code[i].Line, got.Code[i].Line = 0, 0
			}
			if !reflect.DeepEqual(want.Code, got.Code) || want.Events != got.Events {
				t.Fatalf("ECL%d %d: the folded script differs", disk, block)
			}
			n += strings.Count(folded, "if ")
		}
	}
	t.Logf("%d ifs", n)
}

func TestUnfoldForms(t *testing.T) {
	vars := map[string]int{"A": 0x4A00, "B": 0x4A01}
	one := "event0:\nevent1:\nevent2:\nevent3:\nevent4:\n\tif A == 3 { Save 1 B; Exit }\n\talso if A < 3: Goto event0\n\tExit\n"
	many := "event0:\nevent1:\nevent2:\nevent3:\nevent4:\n\tif A == 3 {\n\t\tSave 1 B\n\t\tExit\n\t}\n\talso if A < 3:Goto event0\n\tExit\n"
	plain := "event0:\nevent1:\nevent2:\nevent3:\nevent4:\n\tCompare A 3\n\tIfEq\n\tSave 1 B\n\tIfEq\n\tExit\n\tIfLt\n\tGoto event0\n\tExit\n"
	want, err := ParseScript(plain, vars, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{one, many} {
		got, err := ParseScript(s, vars, nil)
		if err != nil {
			t.Fatal(err)
		}
		for i := range want.Code {
			want.Code[i].Line, got.Code[i].Line = 0, 0
		}
		if !reflect.DeepEqual(want.Code, got.Code) {
			t.Errorf("%q: %v, want %v", s, got.Code, want.Code)
		}
	}
	for _, bad := range []string{"\tif A == 3 {\n", "\t}\n", "\tif A = 3: Exit\n", "\tCompare B 1\n\talso if A == 3: Exit\n"} {
		if _, err := ParseScript("event0:\nevent1:\nevent2:\nevent3:\nevent4:\n"+bad+"\tExit\n", vars, nil); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}
