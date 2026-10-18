// Package amiga plays the Amiga Pool of Radiance sound: the effects player
// (hunk 33, 0x90172) and Wally Beben's music player (hunk 27, 0x638a0),
// translated instruction by instruction over the program's own memory
// (hunks 27-33, so the songs, instruments, samples and tables are the
// game's), driving a model of Paula.
package amiga

import (
	"encoding/binary"
	"math"
)

// Base and End bound the memory image Image cuts out of the flattened
// program (hunks at 4K-aligned bases from 0x10000).
const Base, End = 0x63000, 0x91000

const pal = 3546895 // PAL Paula clock

// Player is the two players plus Paula. Registers are the 68000's, kept as
// 32-bit values because the code moves bytes and words into registers whose
// upper bits it later uses.
type Player struct {
	m     []byte
	d     [8]uint32
	a     [8]uint32
	ch    [4]voice
	dma   uint16
	Trace func(off, size int, v uint32) // every custom-chip write, for tests
	frac  float64
	lp    [2]float64 // output filter state
}

type voice struct {
	lc, ptr, end uint32
	len, per     uint16
	vol          uint16
	pos          float64
}

// New wants the bytes from Base to End.
func New(img []byte) *Player {
	return &Player{m: append([]byte(nil), img...)}
}

func (p *Player) rb(a uint32) byte {
	if a-Base < uint32(len(p.m)) {
		return p.m[a-Base]
	}
	return 0
}
func (p *Player) rw(a uint32) uint16 { return uint16(p.rb(a))<<8 | uint16(p.rb(a+1)) }
func (p *Player) rl(a uint32) uint32 { return uint32(p.rw(a))<<16 | uint32(p.rw(a+2)) }
func (p *Player) wb(a uint32, v byte) {
	if a-Base < uint32(len(p.m)) {
		p.m[a-Base] = v
	}
}
func (p *Player) ww(a uint32, v uint16) { p.wb(a, byte(v>>8)); p.wb(a+1, byte(v)) }
func (p *Player) wl(a uint32, v uint32) {
	if a>>12 == 0xdff {
		p.hw(a, 4, v)
		return
	}
	p.ww(a, uint16(v>>16))
	p.ww(a+2, uint16(v))
}
func (p *Player) hww(a uint32, v uint16) { p.hw(a, 2, uint32(v)) }

func setB(r *uint32, v byte)               { *r = *r&^0xff | uint32(v) }
func setW(r *uint32, v uint16)             { *r = *r&^0xffff | uint32(v) }
func bset(p *Player, a uint32, bit uint32) { p.wb(a, p.rb(a)|1<<(bit&7)) }
func bclr(p *Player, a uint32, bit uint32) { p.wb(a, p.rb(a)&^(1<<(bit&7))) }
func sx(v uint32) uint32                   { return uint32(int32(int16(v))) } // (An, Dn.w) index

func (p *Player) movem(at uint32) {
	for i := range p.d {
		p.d[i] = p.rl(at + 4*uint32(i))
	}
}

// Song starts a song (1-7; 0 stops) = playMusic 0x62e54: 0x900cc = 1, then
// 0x638a0 stores it at 0x638c0 for the next tick and stops the voices.
func (p *Player) Song(n int) {
	p.wl(0x900cc, 1)
	p.wb(0x638c0, byte(n))
	p.stopAll()
}

// Effect starts an effect (0-14) = 0x9014e: music off, request on channel 0.
func (p *Player) Effect(n int) {
	p.wl(0x900cc, 0)
	p.wb(0x90176, byte(n))
}

// Playing tells whether the music player has the channels (else the effects).
func (p *Player) Playing() bool { return p.rl(0x900cc) != 0 }

// Tick is the vertical-blank server at 0x90082.
func (p *Player) Tick() {
	if p.rl(0x900cc) != 0 {
		p.musTick()
	} else {
		p.sfxTick()
	}
}

// ---- effects player, hunk 33 ----

func (p *Player) sfxTick() {
	d := &p.d
	p.movem(0x90526)
	d[1] = 3
	for {
		setB(&d[0], p.rb(0x90176+sx(d[1])))
		if byte(d[0]) != 0xff {
			p.sfxStart()
		} else if p.rb(0x90624+sx(d[1])) != 0 {
			p.sfxStep()
		}
		setW(&d[1], uint16(d[1])-1)
		if uint16(d[1]) == 0xffff {
			break
		}
	}
	save := p.d
	p.sfxOut()
	p.d = save
	p.ww(0x901e0, p.rw(0x901e0)-1)
	if p.rw(0x901e0) == 0 {
		p.sfxLoop()
		p.ww(0x901e0, 0x14)
	}
}

// sfxStart = 0x901e2: copy the 8-byte definition into the channel and point
// Paula at the sample.
func (p *Player) sfxStart() {
	d := &p.d
	d[0] <<= 3
	a0 := 0x90662 + d[0]
	a1 := 0x905fc + d[1]
	for i, off := range []uint32{0, 4, 8, 0xc, 0x10, 0x14} {
		p.wb(a1+off, p.rb(a0+uint32(i)))
	}
	p.wb(a1+0x20, p.rb(a1+0x14))
	p.wb(a1+0x18, p.rb(a0+6))
	p.wb(a1+0x1c, p.rb(a0+7))
	d[0] = uint32(p.rb(a0 + 7))
	if byte(d[0]) == 0xff {
		p.wb(a1+0x24, 0xff)
	} else {
		p.wb(a1+0x24, 0)
	}
	p.wb(a1+0x40, 2)
	setB(&d[0], p.rb(a1))
	push := d[0]
	d[0] <<= 2
	setB(&d[4], p.rb(a1+0x3c))
	p.wl(0xdff0a0+sx(d[4]), p.rl(0x906da+sx(d[0])))
	d[2] = d[1]
	setB(&d[2], byte(d[2])<<2)
	p.wl(0x90646+d[2], p.rl(0x906da+sx(d[0])))
	d[0] = push & 0xff << 3
	a6 := 0x90716 + d[0]
	d[0] = uint32(p.rw(a6) >> 1)
	p.hww(0xdff0a4+sx(d[4]), uint16(d[0]))
	setW(&d[0], p.rw(a6+6))
	p.wb(a1+0x44, byte(d[0]))
	d[3] = d[1] << 1
	p.ww(0x905ec+d[3], p.rw(a6+2))
	p.ww(0x905ec+d[3]+8, p.rw(a6+4))
	d[0] = uint32(p.rb(a1+4)) << 1
	p.ww(0x90656+sx(d[3]), p.rw(0x90574+sx(d[0])))
	p.wb(0x90176+sx(d[1]), 0xff)
	p.wb(0x90644, 0xf)
	bset(p, 0x90644, d[1])
	setB(&d[4], p.rb(a1+0x38))
	d[4] &= 0xff
	p.hww(0xdff096, uint16(d[4]))
	p.wb(a1+0x28, 1)
}

// sfxStep = 0x902f4: count the duration down, applying the mode (bit 2
// second note, bit 1 slide down in pitch, bit 0 up); then repeat, decay,
// stop.
func (p *Player) sfxStep() {
	d := &p.d
	bclr(p, 0x90644, d[1])
	a1 := 0x905fc + d[1]
	d[3] = d[1]
	setW(&d[3], uint16(d[3])<<1)
	setW(&d[5], p.rw(0x90656+sx(d[3])))
	d[0] = uint32(p.rb(a1 + 8))
	setW(&d[0], uint16(d[0])<<1)
	setW(&d[4], p.rw(0x90574+sx(d[0])))
	per := 0x90656 + sx(d[3])
	if p.rb(a1+0x24) == 0 {
		if p.rb(a1+0x14) == 0 {
			if int8(p.rb(a1+0x1c)) >= 0 {
				p.wb(a1+0x1c, p.rb(a1+0x1c)-1)
				p.wb(a1+0x14, p.rb(a1+0x20))
				return
			}
			if p.rb(a1+0x18) != 0 {
				p.wb(a1+0xc, p.rb(a1+0xc)-p.rb(a1+0x18)) // via a pushed d6
				if int8(p.rb(a1+0xc)) >= 0 {
					return
				}
			}
			bset(p, 0x90644, d[1])
			p.wb(a1+0xc, 0)
			p.wb(a1+0x28, 0)
			return
		}
		p.wb(a1+0x14, p.rb(a1+0x14)-1)
	}
	mode := p.rb(a1 + 0x10)
	switch {
	case mode&4 != 0:
		if p.rb(a1+0x2c) != 0 {
			p.wb(a1+0x2c, 0)
			p.ww(per, uint16(d[5]))
		} else {
			p.wb(a1+0x2c, 1)
			p.ww(per, uint16(d[4]))
		}
	case mode&2 != 0:
		setW(&d[7], p.rw(per))
		if uint16(d[7]) != 0xd60 {
			d[4] = uint32(p.rb(a1 + 8))
			setW(&d[7], uint16(d[7])+uint16(d[4]))
			d[7] &= 0xfff
			p.ww(per, uint16(d[7]))
		}
	case mode&1 != 0:
		setW(&d[7], p.rw(per))
		if uint16(d[7]) != 0x71 {
			d[4] = uint32(p.rb(a1 + 8))
			setW(&d[7], uint16(d[7])-uint16(d[4]))
			d[7] &= 0xfff
			p.ww(per, uint16(d[7]))
		}
	default:
		p.ww(per, uint16(d[5]))
	}
}

// sfxOut = 0x90442: periods, DMA (on for every channel not masked off, then
// off for the rest), volumes.
func (p *Player) sfxOut() {
	d := &p.d
	for i := uint32(0); i < 4; i++ {
		p.hww(0xdff0a6+0x10*i, p.rw(0x90656+2*i))
	}
	setB(&d[1], ^p.rb(0x90644))
	d[1] = d[1]&0xf | 0x8200 // DMACONR & 0x5f0 holds no audio bits
	p.hww(0xdff096, uint16(d[1]))
	setW(&d[1], uint16(d[1])^0xffff)
	p.hww(0xdff096, uint16(d[1]))
	for i := uint32(0); i < 4; i++ {
		setB(&d[0], p.rb(0x90608+i))
		p.hww(0xdff0a8+0x10*i, uint16(d[0]))
	}
}

// sfxLoop = 0x904ba (every 20 frames): looped samples get their loop.
func (p *Player) sfxLoop() {
	p.movem(0x90526)
	for c := int32(3); c >= 0; c-- {
		if p.rb(0x90640+uint32(c)) != 0 {
			p.hww(0xdff0a4+0x10*uint32(c), p.rw(0x905f4+2*uint32(c))>>1)
			p.wl(0xdff0a0+0x10*uint32(c), uint32(p.rw(0x905ec+2*uint32(c)))+p.rl(0x90646+4*uint32(c)))
		}
	}
}

// ---- music player, hunk 27 ----

// stopVol = 0x64052, stopAll = 0x6407e.
func (p *Player) stopVol() {
	for i := uint32(0); i < 4; i++ {
		p.hww(0xdff0a8+0x10*i, 0)
		p.wb(0x667b0+i, 0)
	}
}
func (p *Player) stopAll() {
	p.stopVol()
	p.wb(0x667d7, 0)
	p.wb(0x667d5, 0)
}

// musTick = 0x638c2: a song request at 0x638c0 (0 stop, 1-7 start), else
// one tick of the four voices; 0x64048 clears the request.
func (p *Player) musTick() {
	for p.musStep() {
	}
	p.wb(0x638c0, 0xff)
}

// musStep runs musTick once; true when a song command (0xfe, 0xfc) changed
// the request and the tick starts over (the original's branch to 0x638c2).
func (p *Player) musStep() bool {
	d, a := &p.d, &p.a
	p.movem(0x666c8)
	setB(&d[0], p.rb(0x638c0))
	if byte(d[0]) == 0 {
		p.wb(0x667d5, 0)
		p.stopVol()
		return false
	}
	d[1] = 7
	if byte(d[1]) >= byte(d[0]) { // start song d0
		d[0]--
		setW(&d[0], uint16(d[0])<<4)
		a[0] = 0x6697c + d[0]
		for i := uint32(0); i < 16; i++ {
			p.wb(0x6682c+i, p.rb(a[0]+i))
		}
		d[1] = 3
		for {
			a[0] = 0x66750 + d[1]
			p.wb(a[0]+0x14, 0)
			p.wb(a[0]+0x18, 0)
			p.wb(a[0]+0xc, 0)
			p.seq()
			setW(&d[1], uint16(d[1])-1)
			if uint16(d[1]) == 0xffff {
				break
			}
		}
		p.stopAll()
		p.wb(0x667d5, 1)
		p.wb(0x667e0, 0xf)
		return false
	}
	if p.rb(0x667d5) == 0 { // 0x639ac
		return false
	}
	p.wb(0x667d4, 3)
	for { // voices 3..0
		if p.voiceTick() {
			return true
		}
		p.volEnv()
		v := p.rb(0x667d4) - 1
		p.wb(0x667d4, v)
		if int8(v) < 0 {
			break
		}
	}
	p.musOut()
	p.movem(0x666c8)
	for c := uint32(0); c < 4; c++ {
		p.hww(0xdff0a4+0x10*(3-c), p.rw(0x6680c+2*(3-c))>>1)
		base := uint32(0x668b8)
		if p.rb(0x667c8+3-c) != 0 {
			base = 0x667fc
		}
		p.wl(0xdff0a0+0x10*(3-c), uint32(p.rw(0x66814+2*(3-c)))+p.rl(base+4*(3-c)))
	}
	v := p.rb(0x667d7) - 1
	p.wb(0x667d7, v)
	if int8(v) < 0 {
		p.wb(0x667d7, p.rb(0x667d6))
	}
	return false
}

// voiceTick = 0x639be: voice 0x667d4 counts its note down, reads the next
// one when it ends, and runs its slide or effect; true as musStep.
func (p *Player) voiceTick() bool {
	d, a := &p.d, &p.a
	a[0] = 0x66750
	setB(&d[1], p.rb(0x667d4))
	d[2] = d[1]
	setB(&d[2], byte(d[2])<<2)
	p.wl(0x6684c, d[2])
	d[2] = d[1]
	setB(&d[2], byte(d[2])<<1)
	p.wl(0x66850, d[2])
	a[0] += d[1]
	p.wb(0x667dc, p.rb(a[0]))
	if p.rb(0x667d7) != 0 {
		if p.rb(a[0]+0x34) == 0 {
			p.noteFx()
			return false
		}
		p.wb(a[0]+0x34, 0)
		p.wb(a[0]+8, p.rb(a[0]+8)-2)
		p.setPer()
		return false
	}
	p.wb(a[0]+0x14, p.rb(a[0]+0x14)-1)
	if p.rb(a[0]+0x70) == 0 && p.rb(a[0]+0x14) == 0 {
		bset(p, 0x667e0, d[1])
	} else if int8(p.rb(a[0]+0x14)) < 0 {
		return p.nextNote()
	}
	switch {
	case p.rb(a[0]+0x2c) != 0:
		p.wb(a[0]+8, p.rb(a[0]+8)-1)
		p.setPer()
	case p.rb(a[0]+0x30) != 0:
		p.wb(a[0]+8, p.rb(a[0]+8)+1)
		p.setPer()
	default:
		p.noteFx()
	}
	return false
}

// nextNote = 0x63a58: read the voice's pattern up to its next note (0xff
// ends the pattern: seq picks the next) and play it; true as musStep.
func (p *Player) nextNote() bool {
	d, a := &p.d, &p.a
	if p.rb(a[0]+0x18) != 2 {
		p.wb(a[0]+0x18, 0)
		p.wb(a[0]+0x1c, 0)
	}
	p.wb(0x667db, 0)
	p.wb(a[0]+0x24, 0)
	p.wb(a[0]+0x2c, 0)
	p.wb(a[0]+0x30, 0)
	p.wb(a[0]+0x6c, 0)
	p.wl(0x66840, p.rl(0x6681c+p.rl(0x6684c)))
	next := func() { // the byte after a command
		setB(&d[0], p.rb(a[3]))
		a[3]++
		setB(&d[4], byte(d[4])+1)
	}
	for {
		done, restart := p.patternNote(next)
		if restart {
			return true
		}
		if done {
			break
		}
		p.seq() // 0x63ab6: the pattern ended
		p.wb(0x667db, 0)
	}
	p.playNote()
	return false
}

// patternNote = 0x63a92: the commands before a note; done with the note in
// d0, else the pattern ended (d0 0); restart for a song command (0xfe stop,
// 0xfc start the next byte's song), which sets the request.
func (p *Player) patternNote(next func()) (done, restart bool) {
	d, a := &p.d, &p.a
	d[4], d[0] = 0, 0
	setB(&d[4], p.rb(0x667db))
	p.wb(0x667db, p.rb(0x667db)+1)
	a[3] = p.rl(0x66840)
	setB(&d[0], p.rb(a[3]))
	a[3]++
	setB(&d[4], byte(d[4])+1)
	switch byte(d[0]) {
	case 0xff:
		d[0] = 0
		return false, false
	case 0xfe:
		p.wb(0x638c0, 0)
		return false, true
	case 0xfc:
		p.wb(0x638c0, p.rb(a[3]))
		return false, true
	}
	if byte(d[0]) > 0xef { // tempo
		setB(&d[0], byte(d[0])&0xf)
		p.wb(0x667d6, byte(d[0]))
		next()
	}
	if byte(d[0]) > 0xdf { // instrument
		setB(&d[0], (byte(d[0])&0xf)<<3)
		p.wb(a[0]+0x4c, byte(d[0]))
		next()
		if byte(d[0]) == 0xff {
			d[0] = 0
			return false, false
		}
	}
	if byte(d[0]) > 0xbf { // envelope
		setB(&d[0], (byte(d[0])&0x1f)<<3)
		p.wb(a[0]+0x50, byte(d[0]))
		next()
		if byte(d[0]) == 0xff {
			d[0] = 0
			return false, false
		}
	}
	if byte(d[0]) > 0x7f { // duration
		setB(&d[0], byte(d[0])&0x3f)
		p.wb(a[0]+0x10, byte(d[0]))
		next()
	}
	if byte(d[0]) <= 0x5f {
		return true, false
	}
	switch byte(d[0]) {
	case 0x7f, 0x7e: // slide
		p.wb(a[0]+0x2c, b2u(byte(d[0]) == 0x7f))
		p.wb(a[0]+0x30, b2u(byte(d[0]) == 0x7e))
		p.wb(a[0]+0x18, 1)
		next()
	case 0x7d, 0x7c: // arpeggio
		setB(&d[0], 0x7e-byte(d[0]))
		p.wb(a[0]+0x18, byte(d[0]))
		next()
		p.wb(a[0]+0x1c, byte(d[0]))
		p.wb(0x667da, byte(d[0]))
		p.arp()
		next()
	case 0x7b: // portamento
		p.wb(a[0]+0x1c, 0)
		p.wb(a[0]+0x18, 1)
		setB(&d[0], p.rb(a[3]))
		a[3]++
		setB(&d[0], byte(d[0])+p.rb(a[0]+0x20))
		p.wb(a[0]+0x24, byte(d[0]))
		p.wb(a[0]+0x28, p.rb(a[3]))
		a[3]++
		setB(&d[4], byte(d[4])+2)
		next()
	case 0x7a:
		p.wb(a[0]+0x44, p.rb(a[3]))
		a[3]++
		setB(&d[0], p.rb(a[3]))
		d[4] += 2
	}
	return true, false
}

// playNote = 0x63bf8: key the voice on with the note in d0.
func (p *Player) playNote() {
	d, a := &p.d, &p.a
	bclr(p, 0x667e0, d[1])
	setB(&d[5], p.rb(a[0]+0x64))
	d[5] &= 0xff
	p.hww(0xdff096, uint16(d[5]))
	p.wb(0x667db, byte(d[4]))
	save, sa := p.d, p.a
	p.instr()
	p.env()
	p.d, p.a = save, sa
	setB(&d[0], byte(d[0])+p.rb(a[0]+0x20))
	p.wb(a[0]+8, byte(d[0]))
	if byte(d[0]) != 0 && p.rb(a[0]+0x1c) == 0 {
		setB(&d[4], p.rb(a[0]+0x50))
		d[4] &= 0xff
		a[1] = 0x6692c + d[4]
		if p.rb(a[0]+0x34) != 0 {
			p.wb(a[0]+8, p.rb(a[0]+8)+2)
		}
	}
	p.setPer()
	a[1] = 0x6681c + p.rl(0x6684c)
	setB(&d[0], p.rb(0x667db))
	d[0] &= 0xff
	p.wl(a[1], p.rl(a[1])+d[0])
	d[0] = 0
	p.wb(a[0]+0x14, p.rb(a[0]+0x10))
	p.wb(a[0]+0x48, p.rb(a[0]+0x44))
	p.wb(a[0]+0x38, p.rb(a[0]+4)+1)
}

// noteFx = 0x63cb8: a sounding note's arpeggio, portamento or vibrato.
func (p *Player) noteFx() {
	a := &p.a
	switch {
	case p.rb(a[0]+8) == 0:
	case p.rb(a[0]+0x18) == 0 && p.rb(a[0]+0x68) != 0:
		p.vibrato()
	default:
		if p.rb(a[0]+0x18) == 0 {
			p.wb(a[0]+0x80, p.rb(a[0]+0x4e))
		}
		if p.rb(a[0]+0x1c) != 0 {
			p.arpStep()
		} else {
			p.portaStep()
		}
	}
}

// vibrato = 0x63cd8.
func (p *Player) vibrato() {
	d, a := &p.d, &p.a
	save, sa := p.d, p.a
	setB(&d[0], p.rb(a[0]+0x68))
	p.wb(0x667d9, byte(d[0])&0xf)
	setB(&d[0], byte(d[0])>>4)
	p.wb(0x667d8, byte(d[0]))
	setB(&d[0], p.rb(a[0]+8))
	d[0] &= 0xff
	setB(&d[0], byte(d[0])<<1)
	a[1] = 0x666e8 + d[0]
	setW(&d[0], p.rw(a[1])-p.rw(a[1]+2))
	p.ww(0x66844, uint16(d[0]))
	for {
		p.wb(0x667d9, p.rb(0x667d9)-1)
		if int8(p.rb(0x667d9)) < 0 {
			break
		}
		p.ww(0x66844, p.rw(0x66844)>>1|1)
	}
	step := true
	if int8(p.rb(a[0]+0x3c)) < 0 {
		p.wb(a[0]+0x40, p.rb(a[0]+0x40)-1)
		if p.rb(a[0]+0x40) == 0 {
			v := p.rb(a[0]+0x3c) + 1
			p.wb(a[0]+0x3c, v)
			step = int8(v) < 0
		} else {
			step = false
		}
	}
	if step {
		p.wb(a[0]+0x40, p.rb(a[0]+0x40)+1)
		setB(&d[0], p.rb(a[0]+0x40))
		d[0] &= 0xff
		if byte(d[0]) > p.rb(0x667d8) {
			p.wb(a[0]+0x3c, p.rb(a[0]+0x3c)-1)
		}
	}
	if p.rb(a[0]+0x48) != 0 { // 0x63d6c
		p.wb(a[0]+0x48, p.rb(a[0]+0x48)-1)
	} else {
		p.ww(0x66848, p.rw(0x667e2+p.rl(0x66850)))
		setB(&d[0], p.rb(0x667d8))
		d[0] &= 0xff
		setB(&d[0], byte(d[0])>>1)
		setB(&d[4], byte(d[0]))
		for d[4]--; int32(d[4]) >= 0; d[4]-- {
			setW(&d[0], p.rw(0x66848)+p.rw(0x66844))
			p.ww(0x66848, uint16(d[0]))
		}
		setB(&d[4], p.rb(a[0]+0x40))
		d[4] &= 0xff
		for d[4]--; int8(d[4]) >= 0; d[4]-- {
			setW(&d[0], p.rw(0x66848)-p.rw(0x66844))
			p.ww(0x66848, uint16(d[0]))
		}
		setW(&d[0], p.rw(0x66848))
		p.ww(0x66748+p.rl(0x66850), uint16(d[0]))
	}
	p.d, p.a = save, sa
}

// arpStep = 0x63dfa: the next note of the arpeggio list.
func (p *Player) arpStep() {
	d, a := &p.d, &p.a
	save, sa := p.d, p.a
	for {
		setB(&d[4], p.rb(a[0]+0x38))
		d[4] &= 0xff
		d[0] = uint32(p.rb(0x66854 + d[4]))
		if byte(d[0]) != 0xff {
			break
		}
		p.wb(a[0]+0x38, p.rb(a[0]+4))
	}
	setB(&d[0], byte(d[0])+p.rb(a[0]+8))
	setB(&d[0], byte(d[0])<<1)
	setW(&d[0], p.rw(0x666e8+d[0]))
	p.ww(0x66748+p.rl(0x66850), uint16(d[0]))
	p.wb(a[0]+0x38, p.rb(a[0]+0x38)+1)
	p.d, p.a = save, sa
}

// portaStep = 0x63e5a: slide toward the target note inside its time
// window, then vibrato when the voice has one (0x63f02).
func (p *Player) portaStep() {
	d, a := &p.d, &p.a
	if p.rb(a[0]+0x24) == 0 {
		return
	}
	p.wb(0x640f5, p.rb(a[0]+0x24)) // self-modified immediates
	setB(&d[0], p.rb(a[0]+0x28)&0xf)
	p.wb(0x63e93, byte(d[0]))
	p.wb(0x6417f, byte(d[0]))
	setB(&d[0], p.rb(a[0]+0x28)>>4)
	setB(&d[0], byte(d[0])+p.rb(a[0]+0x14))
	if int8(d[0]) <= int8(p.rb(a[0]+0x10)) {
		setB(&d[0], byte(d[0])+p.rb(0x63e93))
		if byte(d[0]) <= p.rb(a[0]+0x10) {
			p.ww(0x667e2+p.rl(0x66850), p.rw(0x667ea+p.rl(0x66850)))
		} else {
			setB(&d[5], p.rb(a[0]+8))
			save, sa := p.d, p.a
			p.porta()
			p.d, p.a = save, sa
			a[1] = 0x667e2 + p.rl(0x66850)
			setW(&d[0], p.rw(a[1]))
			if p.rb(0x667de) != 0 {
				setW(&d[0], uint16(d[0])-p.rw(0x667f2))
			} else {
				setW(&d[0], uint16(d[0])+p.rw(0x667f2))
			}
			p.ww(a[1], uint16(d[0]))
			p.ww(0x66748+p.rl(0x66850), p.rw(a[1]))
		}
	}
	if p.rb(a[0]+0x80) != 0 {
		p.vibrato()
	}
}

// volEnv = 0x63f0e: the voice's volume envelope (attack, then release
// when the note ends).
func (p *Player) volEnv() {
	d, a := &p.d, &p.a
	d[4], d[5], d[6] = 0, 0, 0
	if p.rb(a[0]+0x14) == 0 {
		return
	}
	release := p.rb(a[0]+0x6c) != 0
	if !release {
		setB(&d[6], p.rb(a[0]+0x14))
		if p.rb(a[0]+0x70) == 0 {
			p.wb(a[0]+0x6c, 1)
			return
		}
		setB(&d[4], p.rb(a[0]+0x70))
		if byte(d[4]) == 0xff {
			return
		}
		setB(&d[5], p.rb(a[0]+0x10))
		divu(&d[5], uint16(d[4]))
		setB(&d[6], byte(d[6])-byte(d[5]))
		release = byte(d[6]) == 0
	}
	if !release {
		setB(&d[0], p.rb(a[0]+0x60))
		setB(&d[4], p.rb(a[0]+0x54))
		if byte(d[4]) != byte(d[0]) {
			setB(&d[0], byte(d[0])+p.rb(a[0]+0x58))
			p.wb(a[0]+0x60, byte(d[0]))
		}
		return
	}
	p.wb(a[0]+0x6c, 1) // 0x63f6e
	d[0] = uint32(p.rb(a[0] + 0x60))
	setB(&d[4], p.rb(a[0]+0x5c))
	setB(&d[0], byte(d[0])-byte(d[4]))
	if int8(d[0]) > 0 {
		p.wb(a[0]+0x60, byte(d[0]))
	} else {
		p.wb(a[0]+0x60, 0)
		bset(p, 0x667e0, d[1])
	}
}

func b2u(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// divu.w: no change on overflow; the original would trap on zero.
func divu(r *uint32, s uint16) {
	if s == 0 {
		return
	}
	q, rem := *r/uint32(s), *r%uint32(s)
	if q <= 0xffff {
		*r = rem<<16 | q
	}
}

// seq = 0x6393c: the channel's next sequence entry (0xff restart, >0xc0
// transpose) selects its pattern.
func (p *Player) seq() {
	d, a := &p.d, &p.a
	d[2] = d[1] << 2
	p.wl(0x6683c, p.rl(0x6682c+d[2]))
	for {
		a[1] = p.rl(0x6683c)
		d[0] = uint32(p.rb(a[0] + 0xc))
		p.wb(a[0]+0xc, p.rb(a[0]+0xc)+1)
		a[1] += d[0]
		setB(&d[0], p.rb(a[1]))
		if byte(d[0]) <= 0xc0 {
			break
		}
		if byte(d[0]) == 0xff {
			p.wb(a[0]+0xc, 0)
			continue
		}
		d[0] = (d[0] + 0x20) & 0xff
		p.wb(a[0]+0x20, byte(d[0]))
	}
	d[0] <<= 2
	a[1] = 0x669dc + d[0]
	p.wl(0x6681c+sx(d[2]), p.rl(a[1]))
	p.wl(0x66840, p.rl(a[1]))
	d[0] = 0
}

// setPer = 0x64290.
func (p *Player) setPer() {
	d := &p.d
	setB(&d[0], p.rb(p.a[0]+8))
	setB(&d[0], byte(d[0])<<1)
	setW(&d[0], p.rw(0x666e8+sx(d[0])))
	p.ww(0x667e2+p.rl(0x66850), uint16(d[0]))
	p.ww(0x66748+p.rl(0x66850), uint16(d[0]))
}

// instr = 0x64204: the instrument's sample start and length to Paula, its
// loop for later.
func (p *Player) instr() {
	d, a := &p.d, &p.a
	d[0] = uint32(p.rb(a[0] + 0x4c))
	a[1] = 0x668c4 + d[0]
	setB(&d[4], p.rb(a[1]))
	p.wb(a[0]+0x78, p.rb(a[1]+1))
	if p.rb(a[0]+0x78) == 0 {
		setW(&d[0], p.rw(a[1]+2))
	} else {
		setW(&d[0], p.rw(a[1]+4)+p.rw(a[1]+6))
	}
	p.ww(0x6680c+p.rl(0x66850), p.rw(a[1]+6))
	p.ww(0x66814+p.rl(0x66850), p.rw(a[1]+4))
	setB(&d[6], p.rb(0x667dc))
	d[6] &= 0xff
	setW(&d[0], uint16(d[0])>>1)
	p.hww(0xdff0a4+sx(d[6]), uint16(d[0]))
	a[1] = 0x6687c
	d[4] <<= 2
	p.wl(0xdff0a0+sx(d[6]), p.rl(a[1]+sx(d[4])))
	p.wl(0x667fc+p.rl(0x6684c), p.rl(a[1]+sx(d[4])))
}

// env = 0x64198: the envelope's 8 bytes into the channel.
func (p *Player) env() {
	a0 := p.a[0]
	p.d[0] = uint32(p.rb(a0 + 0x50))
	a2 := 0x6692c + p.d[0]
	for i, off := range []uint32{0x60, 0x54, 0x58, 0x5c, 0x70, 0x68} {
		p.wb(a0+off, p.rb(a2+uint32(i)))
	}
	if p.rb(a2+6) != 0 {
		p.wb(a0+0x1c, p.rb(a2+6))
		p.wb(a0+0x18, 0x7c)
		p.wb(0x667da, p.rb(a2+6))
		p.arp()
	}
	if p.rb(a2+7)&8 != 0 {
		setB(&p.d[0], p.rb(a2+7))
		p.wb(a0+0x27, 1)
	}
}

// arp = 0x6408e: the set bits of 0x667da pick offsets from 0x66924 into the
// channel's arpeggio list, ended by 0xff.
func (p *Player) arp() {
	a0 := p.a[0]
	d4 := uint32(p.rb(a0+4) + 1)
	for d5 := uint32(0); d5 < 8; d5++ {
		b := p.rb(0x667da)
		if b&1 != 0 {
			p.wb(0x66854+d4, p.rb(0x66924+d5))
			d4++
		}
		p.wb(0x667da, b>>1)
	}
	p.wb(0x66854+d4, 0xff)
	p.wb(a0+0x24, 0)
}

// porta = 0x640f2: the slide's start period and per-tick step toward the
// target note (self-modified immediate at 0x640f5).
func (p *Player) porta() {
	d := &p.d
	setB(&d[4], p.rb(0x640f5))
	up := byte(d[5]) < byte(d[4])
	p.wb(0x667de, b2u(up))
	setB(&d[4], byte(d[4])<<1)
	setW(&d[4], p.rw(0x666e8+d[4]))
	p.ww(0x667ea+p.rl(0x66850), uint16(d[4]))
	setB(&d[5], byte(d[5])<<1)
	setW(&d[5], p.rw(0x666e8+d[5]))
	if up {
		setW(&d[5], uint16(d[5])-uint16(d[4]))
		p.ww(0x667f2, uint16(d[5]))
	} else {
		setW(&d[4], uint16(d[4])-uint16(d[5]))
		p.ww(0x667f2, uint16(d[4]))
	}
	d[4] = uint32(p.rb(0x667d6))
	d[0] = 0
	for {
		setB(&d[0], byte(d[0])+p.rb(0x6417f))
		setW(&d[4], uint16(d[4])-1)
		if uint16(d[4]) == 0xffff {
			break
		}
	}
	d[4], d[5] = 0, uint32(p.rw(0x667f2))
	divu(&d[5], uint16(d[0]))
	p.ww(0x667f2, uint16(d[5]))
}

// musOut = 0x642be.
func (p *Player) musOut() {
	for i := uint32(0); i < 4; i++ {
		p.hww(0xdff0a6+0x10*i, p.rw(0x66748+2*i))
	}
	d1 := uint16(^p.rb(0x667e0))&0xf | 0x8200
	p.hww(0xdff096, d1)
	p.hww(0xdff096, d1^0x820f)
	for i := uint32(0); i < 4; i++ {
		p.hww(0xdff0a8+0x10*i, uint16(p.rb(0x667b0+i)))
	}
}

// ---- Paula ----

func (p *Player) hw(a uint32, size int, v uint32) {
	off := int(a - 0xdff000)
	if p.Trace != nil {
		p.Trace(off, size, v)
	}
	if off == 0x96 {
		old := p.dma
		if v&0x8000 != 0 {
			p.dma |= uint16(v) & 0xf
		} else {
			p.dma &^= uint16(v) & 0xf
		}
		for c := range p.ch {
			if p.dma&^old&(1<<c) != 0 {
				p.ch[c].restart()
			}
		}
		return
	}
	c := (off - 0xa0) >> 4
	if c < 0 || c > 3 {
		return
	}
	ch := &p.ch[c]
	switch off & 0xf {
	case 0:
		ch.lc = v
	case 4:
		ch.len = uint16(v)
	case 6:
		ch.per = uint16(v)
	case 8:
		ch.vol = uint16(v) & 0x7f
	}
}

func (v *voice) restart() {
	v.reload()
	v.pos = 0
}

// reload = Paula at the end of a sample: start again from the registers.
func (v *voice) reload() {
	n := uint32(v.len)
	if n == 0 {
		n = 0x10000
	}
	v.ptr, v.end = v.lc&^1, v.lc&^1+2*n
}

// Render mixes the four voices into s16 stereo at rate Hz (0 and 3 left,
// 1 and 2 right), ticking the players at 50 Hz. Paula holds each byte for
// its period, so each output sample is that held signal averaged over the
// sample's time, then the A500's fixed RC low-pass (about 4.4 kHz).
func (p *Player) Render(out []byte, rate int) {
	k := 1 - math.Exp(-2*math.Pi*4420/float64(rate))
	for i := 0; i+3 < len(out); i += 4 {
		if p.frac -= 50; p.frac < 0 {
			p.frac += float64(rate)
			p.Tick()
		}
		var lr [2]float64
		for c := range p.ch {
			ch := &p.ch[c]
			if p.dma&(1<<c) == 0 || ch.per == 0 {
				continue
			}
			step := pal / float64(ch.per) / float64(rate) // bytes per output sample
			sum := 0.0
			for t := step; t > 0; {
				v := float64(int8(p.rb(ch.ptr)))
				if left := 1 - ch.pos; left > t {
					sum += v * t
					ch.pos += t
					break
				} else {
					sum += v * left
					t -= left
				}
				ch.pos = 0
				if ch.ptr++; ch.ptr >= ch.end {
					ch.reload()
				}
			}
			lr[(c+1)>>1&1] += sum / step * float64(min(ch.vol, 64))
		}
		for s := range lr {
			p.lp[s] += k * (lr[s]*2 - p.lp[s])
			binary.LittleEndian.PutUint16(out[i+2*s:], uint16(int16(p.lp[s])))
		}
	}
}

// Image flattens the Amiga "program" hunk executable the way it runs here
// (hunks at 4K-aligned bases from 0x10000, relocated) and returns Base..End,
// or nil if it isn't Pool of Radiance.
func Image(exe []byte) (img []byte) {
	defer func() {
		if recover() != nil {
			img = nil
		}
	}()
	p := 0
	u := func() uint32 { v := binary.BigEndian.Uint32(exe[p:]); p += 4; return v }
	if u() != 0x3f3 {
		return nil
	}
	for u() != 0 {
	}
	n := int(u())
	first, last := u(), u()
	base := make([]uint32, n)
	addr := uint32(0x10000)
	for i := 0; i <= int(last-first); i++ {
		base[i] = addr
		addr += ((u()&0x3fffffff)*4 + 0xfff) &^ 0xfff
	}
	flat := make([]byte, max(addr, End)-0x10000)
	for h := 0; p < len(exe) && h < n; {
		switch u() & 0x3fffffff {
		case 0x3e9, 0x3ea:
			l := int(u()) * 4
			copy(flat[base[h]-0x10000:], exe[p:p+l])
			p += l
		case 0x3eb:
			u()
		case 0x3ec:
			for c := u(); c != 0; c = u() {
				th := u()
				for ; c > 0; c-- {
					q := base[h] - 0x10000 + u()
					binary.BigEndian.PutUint32(flat[q:], binary.BigEndian.Uint32(flat[q:])+base[th])
				}
			}
		case 0x3f2:
			h++
		default:
			return nil
		}
	}
	img = flat[Base-0x10000 : End-0x10000]
	if string(img[0x63c92-Base:0x63c92-Base+16]) != "(c)Wally Beben 1" {
		return nil
	}
	return img
}
