package engine

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"por/dax"
)

// Named numbers in the scripts: a command's operand of some kind (a
// monster, a picture ...) reads as the name of its value in the area's
// data set, e.g. LoadMonster Kobold. The names are in names/values.txt
// (made from the game data by cmd/eclscript: monsters and items) and
// names/pictures.txt (by hand: pictures, sprites, portraits), lines of
// "Kind Set Number Name"; set 0 is every set.

// ScriptValue is one named number.
type ScriptValue struct {
	Kind   string
	Set, N int
	Name   string
}

// operandKinds are the kinds of the commands' operands, "" a plain number.
var operandKinds = map[string][]string{
	"LoadMonster":   {"Monster", "", "Icon"},
	"AddNPC":        {"Monster"},
	"Picture":       {"Picture|Body"},
	"SetupMonster":  {"Sprite", "", "Picture|Body"},
	"EncounterMenu": {"Sprite", "", "Picture|Body", "", "Reaction", "Reaction", "Reaction", "Reaction", "Reaction"},
	"FindItem":      {"Item"},
	"ShowPortrait":  {"Head", "Body"},
	"LoadMap":       {"Map"},
	"LoadWalls":     {"Walls", "Walls", "Walls"},
	"NewECL":        {"Area"},
}

// operandLabels name the commands' operands in the script, as label=value
// (LoadMonster Orc count=8 icon=Orc); "" is an operand without one, a|b
// two labels (a SetupMonster's close-up is a body picture while a
// portrait is up).
var operandLabels = map[string][]string{
	"LoadMonster":  {"", "count", "icon"},
	"SetupMonster": {"", "distance", "closeup|body"},
	"LoadMap":      {"", "unused", "icons"},
	"EncounterMenu": {"", "distance", "closeup|body", "result", "combat", "wait", "flee", "advance", "parlay",
		"near", "mid", "far", "escape", "strength"},
}

// operandDefaults are the values of labelled operands a command may leave
// out (EncounterMenu's empty texts).
var operandDefaults = map[string]map[int]string{
	"EncounterMenu": {9: `""`, 10: `""`, 11: `""`},
}

// withDefaults puts the left-out labelled operands of op back in.
func withDefaults(op string, args []string) []string {
	labels, defs := operandLabels[op], operandDefaults[op]
	if defs == nil {
		return args
	}
	var out []string
	for _, a := range args {
		l, _, ok := strings.Cut(a, "=")
		if ok && !strings.HasPrefix(a, `"`) {
			for j := len(out); j < len(labels) && !slices.Contains(strings.Split(labels[j], "|"), l); j++ {
				d, ok := defs[j]
				if !ok {
					break // a wrong label, reported by the parser
				}
				out = append(out, labels[j]+"="+d)
			}
		}
		out = append(out, a)
	}
	for j := len(out); j < len(labels); j++ {
		if d, ok := defs[j]; ok {
			out = append(out, labels[j]+"="+d)
		}
	}
	return out
}

// ReadScriptValues adds the values of a values file, if there is one.
func ReadScriptValues(path string, vals []ScriptValue) ([]ScriptValue, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return vals, nil
	} else if err != nil {
		return nil, err
	}
	for n, line := range strings.Split(string(b), "\n") {
		line, _, _ = strings.Cut(line, "#")
		fs := strings.Fields(line)
		if len(fs) == 0 {
			continue
		}
		if len(fs) != 4 {
			return nil, fmt.Errorf("%s:%d: want \"Kind Set Number Name\"", path, n+1)
		}
		set, err1 := strconv.Atoi(fs[1])
		v, err2 := strconv.Atoi(fs[2])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("%s:%d: want \"Kind Set Number Name\"", path, n+1)
		}
		vals = append(vals, ScriptValue{fs[0], set, v, fs[3]})
	}
	return vals, nil
}

// valueName is the name of kind's value v in set, or "".
func valueName(vals []ScriptValue, kind string, set, v int) string {
	for _, x := range vals {
		if x.Kind == kind && x.N == v && (x.Set == set || x.Set == 0) {
			return x.Name
		}
	}
	return ""
}

// valueOf is the number kind's name stands for in set; kind may be a|b,
// either kind's.
func valueOf(vals []ScriptValue, kind string, set int, name string) (int, bool) {
	if a, b, ok := strings.Cut(kind, "|"); ok {
		if v, ok := valueOf(vals, a, set, name); ok {
			return v, true
		}
		kind = b
	}
	for _, x := range vals {
		if x.Kind == kind && x.Name == name && (x.Set == set || x.Set == 0) {
			return x.N, true
		}
	}
	return 0, false
}

// DataValues names set's monsters (MON<set>CHA records) and item types
// (the items of ITEM<set>), for names/values.txt. A name two numbers
// share gets the number on.
func DataValues(l *dax.Loader, set int) []ScriptValue {
	var out []ScriptValue
	add := func(kind string, names map[int]string) {
		count := map[string]int{}
		for _, n := range names {
			count[n]++
		}
		var ids []int
		for id := range names {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			n := names[id]
			if count[n] > 1 {
				n += strconv.Itoa(id)
			}
			out = append(out, ScriptValue{kind, set, id, n})
		}
	}
	mons := map[int]string{}
	for id := range 256 {
		if r := l.Block(fmt.Sprintf("MON%dCHA", set), id); len(r) > 0 {
			c := &Character{}
			c.setRecord(r)
			if n := camelName(c.Name()); n != "" {
				mons[id] = n
			}
		}
	}
	add("Monster", mons)
	items := map[int]string{} // the shortest name of an item of the type
	for id := range 0x80 {
		for _, it := range readRecords[Item](l.Block(fmt.Sprintf("ITEM%d", set), id), itemRecSize) {
			n := camelName(itemFullName(it))
			if old, ok := items[int(it.Type)]; n != "" && (!ok || len(n) < len(old)) {
				items[int(it.Type)] = n
			}
		}
	}
	add("Item", items)
	for id := range 0x80 { // the stocks, an ITEM block's items by name
		var s []string
		for _, it := range readRecords[Item](l.Block(fmt.Sprintf("ITEM%d", set), id), itemRecSize) {
			n := camelName(itemFullName(it))
			if it.Count > 1 {
				n += "*" + strconv.Itoa(int(it.Count))
			}
			s = append(s, n)
		}
		if len(s) > 0 && !slices.Contains(s, "") {
			out = append(out, ScriptValue{"Stock", set, id, strings.Join(s, ",")})
		}
	}
	return out
}

// TreasureItems lists the items of set's ITEM block, for a comment.
func TreasureItems(l *dax.Loader, set, block int) string {
	var s []string
	for _, it := range readRecords[Item](l.Block(fmt.Sprintf("ITEM%d", set), block), itemRecSize) {
		n := itemFullName(it)
		if it.Count > 1 {
			n = strconv.Itoa(int(it.Count)) + " " + n
		}
		s = append(s, n)
	}
	if len(s) > 6 {
		s = append(s[:6], fmt.Sprintf("and %d more", len(s)-6))
	}
	return strings.Join(s, ", ")
}

// itemFullName is an item's words, all known.
func itemFullName(it *Item) string {
	var w []string
	for i := 2; i >= 0; i-- {
		if it.Words[i] != 0 {
			w = append(w, strings.TrimSpace(itemWords[it.Words[i]]))
		}
	}
	return strings.Join(w, " ")
}

// camelName is a game name as one script word: "KOBOLD LEADER" ->
// KoboldLeader.
func camelName(s string) string {
	var b strings.Builder
	up := true
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) && b.Len() > 0:
			if up {
				r = unicode.ToUpper(r)
			}
			b.WriteRune(r)
			up = false
		default:
			up = true
		}
	}
	return b.String()
}
