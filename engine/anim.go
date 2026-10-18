package engine

import "por/dax"

// Pictures shown in the view window, ported from ovr29: animated PIC /
// SPRIT / FINAL blocks and the HEAD+BODY portraits.

// animPic = the DS:6ABE record: up to 0x42 bytes holding a frame count and
// per frame a delay and a picture pointer.
type animPic struct {
	name   string // DS:6B0A cache key
	block  int    // DS:6B1C (-1 = nothing loaded)
	frames []dax.SpriteFrame
	cur    int   // DS:6ABF frame shown while waiting for input (1-based)
	last   int64 // ovr26:03BD [bp-57]: tick of the last frame change in the current DisplayInput
}

// portrait = DS:6B00..6B09: the cached HEAD and BODY pictures.
type portrait struct {
	headID, bodyID   int // DS:6B00 / 6B05 (-1 = none)
	headSet, bodySet Set // the sets they came from
	head, body       *dax.Image
}

// LoadAnimPic = ovr29:00A2 (name, block, masked): load <name><gfx>.DAX
// block into the view-window picture, unless it is already there. PIC and
// FINAL frames after the first are stored XORed against the first; the
// first PIC block is cut to 4 frames. Sprites are masked on colour 0.
func (e *Engine) LoadAnimPic(name string, block int, masked bool) {
	a := &e.pic
	if a.name == name && a.block == block {
		return
	}
	e.FreeAnimPic()
	a.name, a.block = name, block
	b := e.L.Block(e.set.file(name), block)
	if b == nil {
		return
	}
	xor := name == "PIC" || name == "FINAL"
	fr := dax.ParseSprite(b, masked, 0, xor)
	if name == "PIC" && block == 1 && len(fr) > 4 {
		fr = fr[:4]
	}
	if masked && (e.Cfg.Graphics == 'E' || e.Cfg.Graphics == 'T') { // DS:4A1E != 0
		for _, f := range fr { // 0464:1E5C(DS:0C6C, DS:0C7C, 0, 0): colour 13 -> opaque black
			e.recolourFrame(maskFrom[:], maskTo[:], 0, false, f.Image)
		}
	}
	a.frames = fr
	a.cur = 1
	e.ClearKeyboard()
}

var maskFrom = [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15} // DS:0C6C
var maskTo = [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 0, 14, 15}    // DS:0C7C

// FreeAnimPic = ovr29:048F.
func (e *Engine) FreeAnimPic() {
	e.pic = animPic{block: -1}
}

// drawPicAt = ovr29:0000 (x, y, overlay, pic): an overlay picture goes
// through 0464:0D71 at page cell (x-1,y-1), which is screen cell (x,y)
// (the page starts one cell in); otherwise 0464:1951.
func (e *Engine) drawPicAt(x, y int, overlay bool, im *dax.Image) {
	if im == nil {
		return
	}
	e.snap.win = WinPicture
	if overlay {
		e.drawPage(x-1, y-1, im, 0, true, false)
		e.Flip() // 0034
	} else {
		e.DrawPicture(x, y, im, 0)
	}
}

// DrawAnimFrame = ovr29:0708 (pic, frame): frame 1..3 of a sprite at its
// own cell offset inside the view window.
func (e *Engine) DrawAnimFrame(frame int) {
	if frame < 1 || frame > 3 {
		e.MessageRow24b(0xF, "Illegal range in Show3DSprite.")
		e.Exit()
		return
	}
	if frame > len(e.pic.frames) {
		return
	}
	im := e.pic.frames[frame-1].Image
	e.snap.win = WinPicture
	e.drawPage(im.X+2, im.Y+2, im, 0, true, false) // 0464:0D71 at (x+3-1, y+3-1)
	e.Flip()
}

// DrawAnimFirst draws frame 1 of the loaded picture the way the PICTURE
// and APPROACH code does: ovr29:0000(3, 3, overlay, frame 1).
func (e *Engine) DrawAnimFirst() {
	if len(e.pic.frames) > 0 {
		e.drawPicAt(3, 3, true, e.pic.frames[0].Image)
	}
}

// LoadPortrait = ovr29:05D9 (head, body): set's HEAD/BODY blocks, kept
// while they do not change; 0xFF leaves that half alone.
func (e *Engine) LoadPortrait(set Set, head, body int) {
	p := &e.port
	if head != 0xFF && (p.headID != head || p.headSet != set) {
		p.head = e.LoadPic(set.file("HEAD"), head)
		p.headID, p.headSet = head, set
	}
	if body != 0xFF && (p.bodyID != body || p.bodySet != set) {
		p.body = e.LoadPic(set.file("BODY"), body)
		p.bodyID, p.bodySet = body, set
	}
	e.ClearKeyboard()
}

// DrawPortrait = ovr29:0697 (x, y, both): the head at (x,y) and, when both
// is set, the body five rows below (nested drawers ovr29:0050 / ovr29:0070).
func (e *Engine) DrawPortrait(x, y int, both bool) {
	e.drawPicAt(x, y, false, e.port.head)
	if both {
		e.drawPicAt(x, y+5, false, e.port.body)
	}
}

// ShowPortrait = ovr08:0511 (head, body).
func (e *Engine) ShowPortrait(head, body int) {
	e.es.f84E1 = 0
	e.es.f49FD, e.es.f49FE = byte(head), byte(body)
	e.LoadPortrait(e.set, head, body)
	e.DrawPortrait(3, 3, true)
}

// ShowEncounter = ovr08:054E (sprite, pic, dist, flags): draw the approaching
// monster sprite for its distance, then at distance 0 its close-up picture
// (or the portrait game[0x5C2] when one is set). flags is DS:84C8: [0] the
// sprite is loaded, [1] the picture is shown.
func (e *Engine) ShowEncounter(sprite, pic, dist int, flags *[2]byte) {
	if flags[1] == 0 && flags[0] != 0 {
		e.Redraw()
	}
	frame := dist + 1
	if flags[1] == 0 {
		if flags[0] == 0 {
			e.LoadAnimPic("SPRIT", sprite, true)
			flags[0] = 1
			e.es.f84E3 = 1
		}
		if e.viewMode == 1 {
			e.DrawAnimFrame(frame)
		} else {
			e.DrawAnimFrame(2)
		}
	}
	face := e.game.Portrait
	if flags[1] != 0 && uint16(e.es.f84EA) == face {
		return
	}
	if dist != 0 || e.viewMode != 1 || (e.es.f84E9 != 0 && e.flag49F6 == 0) {
		return
	}
	e.es.f84EA = byte(face)
	e.es.f84E0 = 1
	if face == 0xFF {
		e.LoadAnimPic("PIC", pic, false)
		flags[1] = 1
		e.DrawAnimFirst()
	} else {
		e.ShowPortrait(int(face), pic)
		flags[1] = 1
		e.es.f84E1 = 0
	}
}

// animateWhileWaiting = the wait loop of ovr26:03BD: until a key arrives,
// cycle the view picture's frames at (3,3), each shown for its delay in
// tenths of a second (area+0x3FE forces a change every pass). Runs when
// the caller asked for animation or area+0x3FE is set.
func (e *Engine) animateWhileWaiting() {
	a := &e.pic
	force := e.viewAnim != 0
	if !force && !e.animInput || a.cur == 0 || a.cur > len(a.frames) || e.ext.fullMap {
		return
	}
	// ovr26:04CC: the current frame is redrawn on every poll
	e.drawPicAt(3, 3, e.animInput, a.frames[a.cur-1].Image)
	if e.Ticks()-a.last >= int64(a.frames[a.cur-1].Delay)*10 || force {
		a.cur++
		if a.cur > len(a.frames) {
			a.cur = 1
		}
		a.last = e.Ticks()
	}
}
