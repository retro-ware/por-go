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
	n := 0
	for disk := 1; disk <= 8; disk++ {
		for block := range 40 {
			b := l.Block(fmt.Sprintf("ECL%d", disk), block)
			if len(b) < 2 {
				continue
			}
			names, err := ReadAreaNames("../data/assets/scripts", block, true)
			if err != nil {
				t.Fatal(err)
			}
			var plain strings.Builder
			if err := scriptPlain(b[2:], &plain, names.Vars); err != nil {
				t.Fatal(err)
			}
			folded := foldScript(plain.String(), names)
			want, err := ParseScript(plain.String(), names)
			if err != nil {
				t.Fatalf("ECL%d %d: %v\n%.300s", disk, block, err, plain.String())
			}
			got, err := ParseScript(folded, names)
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
	names := &ScriptNames{Vars: map[int]string{0x4A00: "A", 0x4A01: "B"}}
	one := "event0:\nevent1:\nevent2:\nevent3:\nevent4:\n\tif A == 3 { Save 1 B; Exit }\n\talso if A < 3: Goto event0\n\tExit\n"
	many := "event0:\nevent1:\nevent2:\nevent3:\nevent4:\n\tif A == 3 {\n\t\tSave 1 B\n\t\tExit\n\t}\n\talso if A < 3:Goto event0\n\tExit\n"
	plain := "event0:\nevent1:\nevent2:\nevent3:\nevent4:\n\tCompare A 3\n\tIfEq\n\tSave 1 B\n\tIfEq\n\tExit\n\tIfLt\n\tGoto event0\n\tExit\n"
	want, err := ParseScript(plain, names)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{one, many} {
		got, err := ParseScript(s, names)
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
		if _, err := ParseScript("event0:\nevent1:\nevent2:\nevent3:\nevent4:\n"+bad+"\tExit\n", names); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

// The tests on an And's result and on a carried item.
func TestUnfoldAndCarried(t *testing.T) {
	names := &ScriptNames{Vars: map[int]string{0x4A00: "A", 0x4A01: "B"},
		Values: []ScriptValue{{"Item", 0, 91, "Pass"}}}
	head := "event0:\nevent1:\nevent2:\nevent3:\nevent4:\n"
	got, err := ParseScript(head+"\tAnd 127 A B\n\tif 0 < B: Exit\n\talso if 0 == B { Save 1 A; Exit }\n\tif Carried Pass: Exit\n\tif 0 == B: Exit\n", names)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ParseScript(head+"\tAnd 127 A B\n\tIfLt\n\tExit\n\tIfEq\n\tSave 1 A\n\tIfEq\n\tExit\n\tFindItem 91\n\tIfEq\n\tExit\n\tCompare 0 B\n\tIfEq\n\tExit\n", names)
	if err != nil {
		t.Fatal(err)
	}
	for i := range want.Code {
		want.Code[i].Line, got.Code[i].Line = 0, 0
	}
	if !reflect.DeepEqual(want.Code, got.Code) {
		t.Errorf("got %v\nwant %v", got.Code, want.Code)
	}
}

func TestUnfoldAssign(t *testing.T) {
	names := &ScriptNames{Vars: map[int]string{0x4A00: "A", 0x4A01: "B", 0x6E79: "Scratch1"}}
	head := "event0:\nevent1:\nevent2:\nevent3:\nevent4:\n"
	got, err := ParseScript(head+"\talias Choice = Scratch1\n\tA = 3\n\tA += 1\n\tB = A - 2\n\tB = A & 7\n\tif 0 != B: Exit\n"+
		"\tA = Tab[B]\n\tTab[B] = A\n\tA = Random 5\n\tChoice = Menu \"GO\" -> L1 \"NO\" -> L2\n\tOnGosub Choice L1 L2\n\tAsk YN \"Y\" -> L1 \"N\"\n\tif Ask YQ == \"N\": Exit\n\talso if Ask YQ == \"Y\": Exit\n\tEncounterMenu 1 distance=2 closeup=3 result=A combat=0 wait=1 flee=2 advance=3 parlay=4 mid=\"HI\" escape=5 strength=6\nL1:\nL2:\n\tExit\nYN:\n\tScratch1 = Menu \"Y\" \"N\"\n\tReturn\nYQ:\n\tScratch1 = Menu \"Y\" \"N\"\n\tCompare Scratch1 0\n\tReturn\nTab:\n\tData 1 2\n", names)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ParseScript(head+"\tSave 3 A\n\tAdd 1 A A\n\tSub 2 A B\n\tAnd A 7 B\n\tIfNe\n\tExit\n"+
		"\tGetTable Tab B A\n\tSaveTable A Tab B\n\tRandom 5 A\n\tHMenu Scratch1 2 \"GO\" \"NO\"\n\tOnGoto Scratch1 2 L1 L2\n\tOnGosub Scratch1 2 L1 L2\n\tGosub YN\n\tOnGoto Scratch1 1 L1\n\tGosub YQ\n\tIfNe\n\tExit\n\tIfEq\n\tExit\n\tEncounterMenu 1 2 3 A 0 1 2 3 4 \"\" \"HI\" \"\" 5 6\nL1:\nL2:\n\tExit\nYN:\n\tHMenu Scratch1 2 \"Y\" \"N\"\n\tReturn\nYQ:\n\tHMenu Scratch1 2 \"Y\" \"N\"\n\tCompare Scratch1 0\n\tReturn\nTab:\n\tData 1 2\n", names)
	if err != nil {
		t.Fatal(err)
	}
	for i := range want.Code {
		want.Code[i].Line, got.Code[i].Line = 0, 0
	}
	if !reflect.DeepEqual(want.Code, got.Code) {
		t.Errorf("got %v\nwant %v", got.Code, want.Code)
	}
}

func TestUnfoldNamed(t *testing.T) {
	names := &ScriptNames{Vars: map[int]string{0x4A00: "A", 0x4A01: "B", 0x6E79: "Scratch1", 0x6E82: "Scratch10", 0x4A02: "Temple"},
		Values: []ScriptValue{{"A", 0, 1, "Seen"}, {"A", 0, 4, "Done"}, {"B", 0, 2, "South"}}}
	head := "event0:\nevent1:\nevent2:\nevent3:\nevent4:\n"
	got, err := ParseScript(head+"\tA |= Seen+Done\n\tA &= ~Seen\n\tif A has Done: Exit\n\talso if A lacks Done: Exit\n\tif B == South: Exit\n"+
		"\tB = PartyStrength\n\tOpenTemple\nL:\n\tfor Scratch1 = 1 to 7\n\t\tA += 1\n\tnext\n\tif Gosub T then B > South: Exit\n\tExit\nT:\n\tCompare B 2\n\tReturn\n", names)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ParseScript(head+"\tOr A 5 A\n\tAnd A 254 A\n\tAnd A 4 Scratch10\n\tIfNe\n\tExit\n\tIfEq\n\tExit\n\tCompare B 2\n\tIfEq\n\tExit\n"+
		"\tPartyStrength B\n\tSave 1 Temple\n\tCombat\n\tSave 1 Scratch1\nL:\n\tAdd 1 A A\n\tAdd 1 Scratch1 Scratch1\n\tCompare Scratch1 8\n\tIfLt\n\tGoto L\n\tGosub T\n\tIfGt\n\tExit\n\tExit\nT:\n\tCompare B 2\n\tReturn\n", names)
	if err != nil {
		t.Fatal(err)
	}
	for i := range want.Code {
		want.Code[i].Line, got.Code[i].Line = 0, 0
	}
	if !reflect.DeepEqual(want.Code, got.Code) {
		t.Errorf("got %v\nwant %v", got.Code, want.Code)
	}
}
