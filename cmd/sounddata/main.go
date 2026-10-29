// Command sounddata writes engine/sounddata.go: the sound effects of START.EXE's sound
// driver (the PC speaker's and the Tandy's songs, their scripts and the vibrato
// waveforms) as Go data, so the engine plays them without the exe.
//
//	go run ./cmd/sounddata original/START.EXE > engine/sounddata.go
//
// It checks that the scripts use only what the engine's driver implements: no notes, no
// channel resets, and none of the fields that drive the notes' envelope.
package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"por/exe"
)

// The driver's segment (08AE) in the load image, and its data.
const (
	segment      = 35552 // 08AE:0000
	jumpTable    = 545   // the six script commands' handlers, from command 250 on
	channelSize  = 48
	speakerSet   = 557 // the PC speaker's four channels
	tandySet     = 967 // the Tandy's: three tones, then the noise
	speakerSongs = 1375
	tandySongs   = 1543
	songs        = 21
	channels     = 4
	enabledFlag  = 525
	firstCommand = 250
)

// The handlers of the commands, by their address in the segment.
var commands = map[int]string{
	5710: "reset", 5804: "return", 5811: "call", 5701: "switch", 5678: "loop", 5759: "set",
}

// The channel fields a script may set, by their offset in a channel, and what the
// engine calls them; the fields only notes and their envelope read are dropped.
var fields = map[int]string{
	0: "sndTimer", 4: "sndPeriod", 6: "sndSlide", 10: "sndLevel", 12: "sndLevelSlide",
	28: "sndVibrato", 32: "sndVibratoSpeed", 34: "sndVibratoDepth", 36: "sndVibratoLength",
	38: "sndCounter", 40: "sndCounter2",
}
var unread = map[int]bool{14: true, 16: true, 18: true, 22: true} // tempo, -, transpose, envelope table
var noteOnly = map[int]bool{20: true, 24: true, 26: true}         // the release timer and the envelope position

type op struct {
	addr, next int
	kind       string
	field, arg int
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: sounddata START.EXE")
	}
	x, err := exe.Load(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	seg := x.Mem[segment:]
	word := func(o int) int { return int(seg[o]) | int(seg[o+1])<<8 }

	if seg[enabledFlag] != 1 {
		log.Fatalf("the driver starts with the sound off (%d)", seg[enabledFlag])
	}
	for o := speakerSet; o < speakerSet+channels*channelSize; o++ {
		if seg[o] != 0 || seg[tandySet+o-speakerSet] != 0 {
			log.Fatal("the channels do not start cleared")
		}
	}

	// every script the songs start, and what they call
	var todo []int
	for _, table := range []int{speakerSongs, tandySongs} {
		for s := range songs {
			for c := range channels {
				if p := word(table + s*8 + c*2); p != 0 {
					todo = append(todo, p)
				}
			}
		}
	}
	ops := map[int]op{}
	vibratos := map[int]int{} // table address: its length
	for len(todo) > 0 {
		pc := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		for {
			if _, ok := ops[pc]; ok {
				break
			}
			b := int(seg[pc])
			if b < firstCommand {
				log.Fatalf("a note at %d: the engine plays no notes", pc)
			}
			o := op{addr: pc, kind: commands[word(jumpTable+2*(b-firstCommand))]}
			switch o.kind {
			case "return":
				o.next = pc + 1
			case "loop":
				o.field, o.arg, o.next = int(seg[pc+1]), word(pc+2), pc+4
				if fields[o.field] == "" || o.field < 38 {
					log.Fatalf("a loop at %d counts in field %d", pc, o.field)
				}
				todo = append(todo, o.arg)
			case "call", "switch":
				o.arg, o.next = word(pc+1), pc+3
				if o.kind == "call" {
					todo = append(todo, o.arg)
				} else if o.arg%channelSize != 0 || o.arg/channelSize >= channels {
					log.Fatalf("a switch at %d to channel offset %d", pc, o.arg)
				}
			case "set":
				o.field, o.arg, o.next = int(seg[pc+1]), word(pc+2), pc+4
				if noteOnly[o.field] {
					log.Fatalf("a script at %d sets field %d, which only notes use", pc, o.field)
				}
				if fields[o.field] == "" && !unread[o.field] {
					log.Fatalf("a script at %d sets the unknown field %d", pc, o.field)
				}
			default:
				log.Fatalf("command %d (%s) at %d is not implemented", b, o.kind, pc)
			}
			ops[pc] = o
			if o.kind == "return" || o.kind == "set" && o.field == 0 && o.arg == 0 {
				break // a timer of 0 frees the channel: the script ends
			}
			pc = o.next
		}
	}

	var addrs []int
	for a := range ops {
		addrs = append(addrs, a)
	}
	sort.Ints(addrs)
	// the vibrato tables lie one after the other below the scripts: each ends where the
	// next begins; no script lets its position run past the longest table it may use
	longest := 0
	for _, a := range addrs {
		o := ops[a]
		switch {
		case o.kind == "set" && o.field == 28:
			vibratos[o.arg] = 0
		case o.kind == "set" && o.field == 36:
			longest = max(longest, o.arg/16)
		}
	}
	var vibAddrs []int
	for a := range vibratos {
		vibAddrs = append(vibAddrs, a)
	}
	sort.Ints(vibAddrs)
	for i, a := range vibAddrs {
		end := addrs[0]
		if i+1 < len(vibAddrs) {
			end = vibAddrs[i+1]
		}
		vibratos[a] = end - a
	}
	if longest > vibratos[vibAddrs[len(vibAddrs)-1]] {
		log.Fatalf("a vibrato of %d entries runs past the tables", longest)
	}
	vibIndex := map[int]int{}
	for i, a := range vibAddrs {
		vibIndex[a] = i + 1
	}

	// the program: the kept ops in address order after the empty op 0; a dropped set
	// stands for the op after it
	index := map[int]int{}
	var kept []op
	for _, a := range addrs {
		o := ops[a]
		if o.kind == "set" && unread[o.field] {
			continue
		}
		index[a] = len(kept) + 1
		kept = append(kept, o)
	}
	resolve := func(a int) int {
		for {
			if i, ok := index[a]; ok {
				return i
			}
			o, ok := ops[a]
			if !ok {
				log.Fatalf("a jump to %d, which is no op", a)
			}
			a = o.next
		}
	}
	for i := 1; i < len(kept); i++ {
		if p := kept[i-1]; p.kind != "return" && p.next != kept[i].addr && resolve(p.next) != i+1 {
			log.Fatalf("the op after %d is not the next one kept", p.addr)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by cmd/sounddata from START.EXE's sound driver; DO NOT EDIT.\n\npackage engine\n\n")
	fmt.Fprintf(&b, "// sndProgram is the scripts of every song; op 0, none, ends a script.\nvar sndProgram = []sndOp{\n\t{},\n")
	for _, o := range kept {
		switch o.kind {
		case "return":
			fmt.Fprintf(&b, "\t{kind: sndReturn},\n")
		case "call":
			fmt.Fprintf(&b, "\t{kind: sndCall, arg: %d},\n", resolve(o.arg))
		case "switch":
			fmt.Fprintf(&b, "\t{kind: sndSwitch, arg: %d},\n", o.arg/channelSize)
		case "loop":
			fmt.Fprintf(&b, "\t{kind: sndLoop, field: %s, arg: %d},\n", fields[o.field], resolve(o.arg))
		case "set":
			arg := o.arg
			if o.field == 28 {
				arg = vibIndex[arg]
			}
			fmt.Fprintf(&b, "\t{kind: sndSet, field: %s, arg: %d},\n", fields[o.field], arg)
		}
	}
	fmt.Fprintf(&b, "}\n")
	for _, t := range []struct {
		name, doc string
		addr      int
	}{{"sndSpeakerSongs", "the PC speaker's", speakerSongs}, {"sndTandySongs", "the Tandy's", tandySongs}} {
		fmt.Fprintf(&b, "\n// %s are the first op of %s four channels in each song.\nvar %s = [sndSongs][sndChannels]int{\n", t.name, t.doc, t.name)
		for s := range songs {
			var row []string
			for c := range channels {
				p := word(t.addr + s*8 + c*2)
				if p != 0 {
					p = resolve(p)
				}
				row = append(row, fmt.Sprint(p))
			}
			fmt.Fprintf(&b, "\t{%s},\n", strings.Join(row, ", "))
		}
		fmt.Fprintf(&b, "}\n")
	}
	fmt.Fprintf(&b, "\n// sndVibratos are the vibrato waveforms; table 0 is none.\nvar sndVibratos = [][]int8{\n\tnil,\n")
	for _, a := range vibAddrs {
		var vs []string
		for i := range vibratos[a] {
			vs = append(vs, fmt.Sprint(int8(seg[a+i])))
		}
		fmt.Fprintf(&b, "\t{")
		for i, v := range vs {
			if i%16 == 0 {
				fmt.Fprintf(&b, "\n\t\t")
			} else {
				fmt.Fprintf(&b, " ")
			}
			fmt.Fprintf(&b, "%s,", v)
		}
		fmt.Fprintf(&b, "\n\t},\n")
	}
	fmt.Fprintf(&b, "}\n")
	os.Stdout.WriteString(b.String())
}
