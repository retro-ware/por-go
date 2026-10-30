package engine

import (
	"strings"

	"por/dax"
)

// Story and housekeeping routines of ovr18, ovr19 and ovr34, and the duel
// set-up of ovr08.

// ProgramEight = ovr19:02A4: the Tyranthraxus finale. The FINAL pictures
// are drawn on the page, then animated; the dragon dies, Tyranthraxus
// speaks, and the screen is rebuilt.
func (e *Engine) ProgramEight() {
	final := e.set.file("FINAL") // Str(DS:5376) -> DS:4713
	savedFlag := e.beginFlag
	e.beginFlag = 7
	var im *dax.Image // DS:470E, never freed

	// finalAnim = the nested routine at ovr19:0006 (x, y, block, loops):
	// cycle the frames of FINAL<block> at (x,y) until `loops` passes are done.
	finalAnim := func(x, y, block, loops int) {
		pass := 0
		last := e.Ticks()
		e.LoadAnimPic("FINAL", block, false)
		cur, count := e.pic.cur, len(e.pic.frames) // rec[1] and rec[0]
		if count > 0 {                             // ovr19:004C: frame 1, unmasked, then 1B52
			e.drawPage(x-1, y-1, e.pic.frames[0].Image, 0, false)
		}
		for {
			var delay int64
			if cur >= 1 && cur <= count {
				e.drawPicAt(x, y, true, e.pic.frames[cur-1].Image) // ovr29:0000 flips
				delay = int64(e.pic.frames[cur-1].Delay)
			}
			now := e.Ticks()
			if now-last > int64(e.msgDelay+3)*delay {
				cur++
				if cur > count {
					cur = 1
					pass++
				}
				last = now
			}
			if pass == loops {
				break
			}
			e.idle() // the original spins; this keeps the host responsive
		}
		e.FreeAnimPic()
	}
	slow := func() int { return (10 - int(e.msgDelay)) * 5 >> 1 }
	// page draws FINAL<set> block at page cell (x,y) (0464:0111, 0464:0D71 mode 0).
	page := func(block, x, y int) {
		im = e.LoadPic(final, block)
		e.drawPage(x, y, im, 0, false)
	}
	say := func(y int, s string) { e.DisplayString(1, y, colLightGreen, s) }
	// speakAll (extra, voice.go) reads the lines just said aloud as one text.
	speakAll := func(lines ...string) {
		for i, l := range lines {
			say(0x11+i, l)
		}
		e.speak(voiceKey(strings.Join(lines, " ")))
	}

	e.ClearBox(1, msgTop, winRight, winBottom)
	speakAll("Mortally wounded, the dragon roars!", "The spirit of Tyranthraxus flares up", "from the dragon's body.")
	e.DrawMainScreen()
	page(1, 0, 0)
	finalAnim(3, 5, 2, slow())
	page(3, 0, 0)
	if e.game.PartyCount > 1 {
		page(4, 0, 0xB)
	}
	if e.game.PartyCount > 2 {
		page(5, 0xD, 9)
	}
	if e.game.PartyCount > 3 {
		page(6, 0, 9)
	}
	e.ClearBox(1, msgTop, winRight, winBottom)
	speakAll("\"Fools, you have but slain the body", "I possessed.  I cannot be defeated!\"", "With the power of the pool of radiance", "which I moved here, to my lair,", "I will still rule, by possessing one", "of you!\"")
	finalAnim(3, 1, 7, slow())
	e.ClearBox(1, msgTop, winRight, winBottom)
	speakAll("\"No Lord Bane!  I can still rule here!", "I have not failed.  Do not call me back", "through the pool!\"", "Noooo...")
	finalAnim(3, 1, 8, 1)
	e.MsgDelay()
	e.MsgDelay()
	e.MsgDelay()
	e.beginFlag = savedFlag
	e.DrawMainScreen()
}

// recolourFrame = 0464:1E5C (from, to, frame, random, image), the EGA
// path (DS:4A1E = 1) on chunky pixels: for each of the 16 colours whose
// from and to differ, pixels that had that colour in the picture as passed
// in become `to` in a copy, which replaces the picture at the end (so one
// call never chains A->B->C). With random set, each 8-pixel cell of a row
// changes only the pixels selected by Random(256) & Random(256), leftmost
// pixel in bit 7. A negative frame means every frame.
func (e *Engine) recolourFrame(from, to []byte, frame int, random bool, im *dax.Image) {
	if im == nil {
		return
	}
	count := 1
	if frame < 0 {
		frame, count = 0, im.Count
	}
	if frame+count > im.Count {
		count = im.Count - frame
	}
	if count <= 0 {
		return
	}
	lo, hi := frame*im.BPP, (frame+count)*im.BPP
	src := append([]byte(nil), im.Data[lo:hi]...)
	dst := im.Data[lo:hi]
	for i := 0; i < 16; i++ {
		if from[i] == to[i] {
			continue
		}
		for cell := 0; cell < count*im.Height*im.Width; cell++ {
			mask := byte(0xFF)
			if random {
				a := byte(e.Random(0x100))
				mask = a & byte(e.Random(0x100))
			}
			for k := 0; k < 8; k++ {
				if mask&(0x80>>k) != 0 && src[cell*8+k] == from[i] {
					dst[cell*8+k] = to[i]
				}
			}
		}
	}
}

// EndgameHook = ovr19:06B3 (n): the pool of radiance fades. With n set the
// area map is drawn first and the squares change in nine steps (randomised
// recolouring of the SQRPACI tiles); a final pass recolours them fully.
func (e *Engine) EndgameHook(n int) {
	tiles := func(to1, to2 *[16]byte, random bool) {
		for j := 0; j <= 0xA; j++ {
			e.recolourSquare(endFrom[:], to1[:], endTiles1[j], random)
		}
		for j := 0; j <= 0x11; j++ {
			e.recolourSquare(endFrom[:], to2[:], endTiles2[j], random)
		}
	}
	if n != 0 {
		e.DrawAreaMap(0x11, 0xF)
		for k := 0; k <= 8; k++ {
			tiles(&endTo1, &endTo2, true)
			e.DrawAreaMap(int(endMapX[k]), int(endMapY[k]))
		}
	}
	tiles(&endTo1, &endTo2, false)
	if n != 0 {
		e.DrawAreaMap(0x17, 0x19)
		e.MsgDelay()
		e.MsgDelay()
	}
}

// recolourSquare recolours one SQRPACI frame: values above 0x7F select the
// second picture (DS:5E56), the others the first (DS:5E52).
func (e *Engine) recolourSquare(from, to []byte, v byte, random bool) {
	if v > 0x7F {
		e.recolourFrame(from, to, int(v&0x7F), random, e.sqrpaci[1])
	} else {
		e.recolourFrame(from, to, int(v), random, e.sqrpaci[0])
	}
}

// DuelSetup = ovr08:1A6F (n): only the selected member fights (the others,
// by name, are made inactive). With n set a clone of the selected member,
// "ROLF" with the CPIC picture, is appended as the opponent and gets
// copies of the member's items, in reverse order.
func (e *Engine) DuelSetup(n int) {
	e.run.duelMode = 1
	e.game.Duel = uint16(int16(int8(n)))
	sel := e.selected()
	if sel == nil {
		return
	}
	for _, c := range e.party {
		if c.Name() != sel.Name() { // 0AF8:0C10 compares the name strings
			c.Okay = 0
		}
	}
	if n == 0 {
		return
	}
	e.LoadIconPic("CPIC", 0xB, int(e.es.cloneIcon)) // ovr33:01C8
	cl := &Character{charRecord: sel.charRecord}    // no affects, no items yet
	cl.Okay = 1
	cl.SetName("ROLF")
	cl.Auto = 1
	cl.Side = sideMonsters
	cl.Control = 0xB2
	cl.Order = e.es.cloneIcon
	e.party = append(e.party, cl)
	for _, it := range sel.Items {
		c := *it
		cl.Items = append([]*Item{&c}, cl.Items...)
	}
}
