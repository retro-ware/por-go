package engine

// Combat setup (ovr11): the battle map, the combat blocks and where every
// combatant starts.

// combatSetup = ovr11:1E31.
func (e *Engine) combatSetup() {
	e.FreeAnimPic()
	e.port = portrait{headID: 0xFF, bodyID: 0xFF} // 0464:2595 x2
	e.ClearRow24()
	e.MsgDelay()
	e.DisplayString(0, 0x18, 0xA, "A battle begins...")
	e.Music(2) // Amiga 0x364E2
	e.magicOn, e.combatRound, e.rv.hitRoll = 0, 0, 0
	e.nbodies = 0
	e.clouds = nil
	e.thrownCopy = nil
	e.bodies = [10]body{}
	e.game.AttackedAlly = 0
	if e.game.Morale > 0x64 {
		e.game.Morale = 0x64
	}
	e.combatMapSetup()
	e.flushKey()
	e.combatBlocks()
	e.flushKey()
	e.placeCombatants()
	e.flushKey()
	e.spr = nil // 0464:248E (1, 4, 3, 0x18): iconToSprite sizes it
	if len(e.party) > 0 {
		e.cmap.viewX = e.combatX(e.party[0]) - 3
		e.cmap.viewY = e.combatY(e.party[0]) - 3
	}
	e.combatUseHook()
	for _, c := range e.party {
		e.affectGroup(c, 8)
	}
	e.foesHealthPercent()
	e.beginFlag = 5
}

// flushKey = 0899:0122: a waiting key is read and dropped.
func (e *Engine) flushKey() {
	if e.keyWaiting() {
		e.ReadKeyByte()
	}
}

// combatMapSetup = ovr11:1244: the tile sets and the map.
func (e *Engine) combatMapSetup() {
	e.snap.dungeon = e.viewMode == 1
	if e.viewMode == 1 {
		e.LoadBackpacIcons("DungCom", 0, 0x18)
	} else {
		e.LoadBackpacIcons("WildCom", 0, 0x21)
	}
	e.LoadBackpacIcons("RandCom", 0x22, 5)
	e.cmap = &combatMap{cursorSize: 1}
	if e.viewMode == 1 {
		e.dungeonMap()
	} else {
		e.wildMap()
	}
}

// combatBlocks = ovr11:12E1: a fresh combat block for every combatant;
// those past the party count are not party members.
func (e *Engine) combatBlocks() {
	for i, c := range e.party {
		e.ApplyItems(c) // 00BA:0BB8
		c.Combat = &CombatInfo{}
		if i+1 > int(e.game.PartyCount) {
			c.Combat.Extra = 1
		}
		c.Combat.Facing = facingDir[e.facing/2]
		if c.Side == sideMonsters {
			c.Combat.Facing = (c.Combat.Facing + 4) % 8
		}
		m := c.Control & 0x7F
		if c.Side == sideParty && c.Combat.Delay == 1 && (m == 0 || m > 0x66) {
			c.Control = byte(e.game.Morale + 0x80)
		}
	}
}

// --- the dungeon map (ovr11:0810) -------------------------------------

// cellSide = ovr11:012B (x, y, dir): 1 wall, 0 open, 3 door. Off the
// 16x16 map only the party's row is open east and west.
func (e *Engine) cellSide(x, y, dir int) byte {
	if x >= 0 && x <= 0xF && y >= 0 && y <= 0xF {
		if e.DoorAt(x, y, dir) == 0 {
			return 1
		}
		if e.WallAt(x, y, dir) == 0 {
			return 0
		}
		return 3
	}
	if y == e.posY && (dir == 2 || dir == 6) {
		return 0
	}
	return 1
}

// cellEdge = ovr11:01AD (x, y, dir): the side seen from both cells.
func (e *Engine) cellEdge(x, y, dir int) byte {
	dx, dy := dirDX[dir], dirDY[dir]
	return e.cellSide(x, y, dir) | e.cellSide(x+dx, y+dy, (dir+4)%8)
}

// dungeonMap = ovr11:0810: every cell within 6 east-west and 2 north-south
// of the party becomes a 6x5 block of tiles, rows skewed 5 squares.
func (e *Engine) dungeonMap() {
	for r := -2; r <= 2; r++ {
		for c := -6; c <= 6; c++ {
			x, y := e.posX+c, e.posY+r
			w, n, ea := e.cellEdge(x, y, 6), e.cellEdge(x, y, 0), e.cellEdge(x, y, 2)
			put := func(col, row int, t byte) { // ovr11:0221
				px, py := col+c*6+r*5+0x15, row+r*5+0xA
				if px >= 0 && px <= 0x31 && py >= 0 && py <= 0x18 {
					e.cmap.ground[py][px] = t + 1
				}
			}
			// nested ovr11:02BA: floor, west wall or door
			for row := 2; row <= 4; row++ {
				for col := 0; col <= 5; col++ {
					put(col, row, 0x16)
				}
			}
			if w == 1 {
				for i := 2; i <= 4; i++ {
					put(i-1, i, 4)
					put(i, i, 3)
					put(i+1, i, 0xD)
				}
			} else if w == 3 {
				put(1, 2, 8)
				put(5, 4, 0)
			}
			// nested ovr11:0377: north wall
			if n == 1 {
				put(3, 0, 5)
				put(4, 0, 5)
				put(3, 1, 0xA)
				put(4, 1, 0xA)
			} else {
				put(3, 0, 0x16)
				put(4, 0, 0x16)
				put(3, 1, 0x16)
				put(4, 1, 0x16)
			}
			// nested ovr11:040F: the north-west corner
			nw := e.cellEdge(x, y-1, 6) == 0 && e.cellEdge(x-1, y, 0) == 0
			pick := func(ok bool, a, b byte) byte {
				if ok {
					return a
				}
				return b
			}
			var a, b, cc, d byte
			if n == 0 {
				switch w {
				case 0:
					a = 0x16
				case 3:
					a = 0xD
				case 1:
					a = pick(nw, 0, 0xD)
				}
			} else if w == 0 {
				a = pick(nw, 0xF, 5)
			} else {
				a = pick(nw, 0x12, 2)
			}
			b = map[byte]byte{0: 0x16, 3: 0x11, 1: 5}[n]
			switch w {
			case 0:
				cc = pick(n == 0, 0x16, pick(nw, 0x10, 0xA))
			case 3:
				cc = pick(nw, 0x14, 7)
			case 1:
				cc = pick(nw, 1, 3)
			}
			if w == 0 || w == 3 {
				d = map[byte]byte{0: 0x16, 3: 0x17, 1: 0xA}[n]
			} else {
				d = map[byte]byte{0: 0xD, 3: 0x15, 1: 6}[n]
			}
			put(1, 0, a)
			put(2, 0, b)
			put(1, 1, cc)
			put(2, 1, d)
			// nested ovr11:0615: the north-east corner
			ne7, ne8 := e.cellEdge(x, y-1, 2), e.cellEdge(x+1, y, 0)
			ne := ne7 == 0 && ne8 == 0
			a = map[byte]byte{0: pick(ne7 == 1, 4, 0x16), 3: 0xF, 1: 5}[n]
			if n == 0 {
				switch ne7 {
				case 0:
					b = 0x16
				case 3:
					b = pick(ea == 0 && ne8 != 0, 0x18, 1)
				case 1:
					b = pick(ea == 0 && ne8 != 0, 0xB, 3)
				}
			} else {
				switch {
				case ea != 0:
					b = 9
				case ne8 != 0:
					b = 5
				default:
					b = pick(ne, 0x11, 0x13)
				}
			}
			cc = map[byte]byte{0: 0x16, 3: 0x10, 1: 0xA}[n]
			if n == 0 {
				switch {
				case ne7 == 0:
					d = 0x16
				case ea != 0:
					d = 4
				default:
					d = pick(ne8 == 0, 8, 0xC)
				}
			} else {
				switch {
				case ea != 0:
					d = 0xE
				default:
					d = pick(ne8 == 0, 0x17, 0xA)
				}
			}
			put(5, 0, a)
			put(6, 0, b)
			put(5, 1, cc)
			put(6, 1, d)
		}
	}
	e.floorClutter()
}

// floorClutter = ovr11:0000: now and then something lies on open ground.
func (e *Engine) floorClutter() {
	for x := 0; x <= 0x31; x++ {
		for y := 0; y <= 0x18; y++ {
			g := &e.cmap.ground[y][x]
			if e.groundInfo(*g)[3] != 0x16 {
				continue
			}
			switch e.Dice(1, 100) {
			case 0x62:
				*g = 0x1A
			case 0x63:
				*g = 0x1B
			case 0x64:
				if e.gs.areaKind != 0xA && e.Dice(1, 100) == 1 {
					if r := e.Dice(1, 10); r <= 6 {
						*g = 0x1C
					} else {
						*g = 0x1D
					}
				}
			}
		}
	}
}

// --- the wilderness map (ovr11:11B4) ----------------------------------

// wildMap = ovr11:11B4.
func (e *Engine) wildMap() {
	for y := range e.cmap.ground {
		for x := range e.cmap.ground[y] {
			e.cmap.ground[y][x] = 0x17
		}
	}
	a, b := int(byte(e.area.MapX)), int(byte(e.area.MapY))
	switch e.viewMode {
	case 3:
		a += 0xD
	case 4:
		a += 0x1A
	}
	e.terrain = terrainMap[b][a] // DS:465A
	e.wildRoad()
	e.wildWater()
	e.wildPlants()
	e.floorClutter()
}

// terrainKinds = the ranges of ovr11:08A4, tried in order.
var terrainKinds = []struct{ lo, hi, kind byte }{
	{1, 2, 1}, {0x28, 0x29, 1}, {0x4F, 0x56, 1}, {0x5B, 0x5C, 1}, {0x2B, 0x2B, 1}, {0xF1, 0xF1, 1},
	{0xA8, 0xB1, 0x21},
	{0xB9, 0xB9, 0x11}, {0xBC, 0xC0, 0x11},
	{3, 4, 3}, {6, 7, 3}, {8, 9, 3}, {0x13, 0x15, 3}, {0x17, 0x19, 3},
	{7, 7, 2}, {0xA, 0x12, 2}, {0x16, 0x16, 2}, {0x1A, 0x24, 2}, {0xC9, 0xD0, 2},
	{0xD1, 0xD5, 0x12},
	{0x25, 0x25, 4}, {0x39, 0x39, 4}, {0x40, 0x40, 4}, {0x9A, 0x9E, 4}, {0xA4, 0xA5, 4}, {0xB4, 0xB4, 4},
	{0xF3, 0xF3, 4}, {0xEF, 0xF0, 4}, {0xFA, 0xFA, 4}, {0xFC, 0xFC, 4},
	{0x27, 0x2C, 5}, {0x99, 0x99, 5}, {0x9F, 0x9F, 5},
	{0x2D, 0x2E, 0x44}, {0x33, 0x38, 0x44}, {0x43, 0x4D, 0x44}, {0x5D, 0x61, 0x44},
	{0xB5, 0xB8, 0x14}, {0xBA, 0xBA, 0x14}, {0xC1, 0xC8, 0x14},
	{0x2F, 0x32, 0x40}, {0x62, 0x65, 0x40}, {0xF2, 0xF2, 0x40},
	{0x26, 0x26, 0x41},
	{0xB2, 0xB3, 0x60},
	{0x3A, 0x3C, 0x48}, {0x41, 0x42, 0x48},
	{0x3D, 0x3F, 8}, {0x79, 0x7C, 8}, {0x87, 0x89, 8}, {0x8B, 0x8B, 8}, {0x8D, 0x8D, 8}, {0x90, 0x93, 8},
	{0x95, 0x95, 8}, {0x97, 0x98, 8}, {0xA0, 0xA2, 8}, {0xA6, 0xA7, 8}, {0xD6, 0xD8, 8}, {0xEE, 0xEE, 8},
	{0x57, 0x5A, 0x88}, {0xA3, 0xA3, 0x88}, {0xE0, 0xE0, 0x88},
	{0x7D, 0x7D, 9}, {0x7C, 0x7C, 9}, {0x8A, 0x8A, 9}, {0x8C, 0x8C, 9}, {0x8E, 0x8F, 9}, {0x94, 0x94, 9}, {0x96, 0x96, 9},
	{0x7E, 0x85, 0x28}, {0xE5, 0xE7, 0x28},
	{0xD9, 0xDF, 0x18},
	{0x4E, 0x4E, 0x20}, {0xF5, 0xF5, 0x20}, {0xE8, 0xE9, 0x20},
	{0x6D, 0x6D, 0x80}, {0xF4, 0xF6, 0x80},
	{0x6E, 0x71, 0x81}, {0x73, 0x78, 0x81},
	{0x66, 0x6C, 0x90}, {0x72, 0x72, 0x90},
	{0xE1, 0xE4, 0xA0}, {0xEA, 0xED, 0xA0},
}

// terrainFlags = ovr11:08A4: what the wilderness square holds. 0x80
// (buildings) turns into 1 where area +0x366 is 0xFF.
func (e *Engine) terrainFlags() byte {
	var k byte
	for _, t := range terrainKinds {
		if e.terrain >= t.lo && e.terrain <= t.hi {
			k = t.kind
			break
		}
	}
	if k&0x80 != 0 && e.area.Boss == 0xFF {
		k = k - 0x80 | 1
	}
	return k
}

// wildRoad = ovr11:0BDE: a diagonal road (0x32/0x33), maybe with stones.
func (e *Engine) wildRoad() {
	var chance, stones int
	f := e.terrainFlags()
	if f&0x20 != 0 {
		chance = 0x23
	}
	if f&0x10 != 0 {
		chance = 0x4B
	}
	if e.Dice(1, 100) > chance {
		return
	}
	col := int(int8(byte(0x22 - e.Dice(5, 4))))
	for int(int8(byte(col+2)))%7 > 0 {
		col--
	}
	start := col
	stone := func(row, col int) { // ovr11:0B97
		e.cmap.ground[row][col] = 0x34
		e.cmap.ground[row][col+1] = 0x35
		stones++
	}
	for row := 0; row <= 0x18; row++ {
		if col > 0x31 {
			continue
		}
		e.cmap.ground[row][col] = 0x32
		e.cmap.ground[row][col+1] = 0x33
		if e.Dice(1, 20) == 1 || stones&1 != 0 {
			stone(row, col)
		}
		col++
	}
	if stones == 0 {
		row := 0xC - e.Dice(1, 9) + 4
		stone(row, row+start)
		stone(row+1, row+start+1)
	}
}

// wildWater = ovr11:0D13: pools and rocks on open ground.
func (e *Engine) wildWater() {
	chance := 1
	f := e.terrainFlags()
	if f&2 != 0 {
		chance = 1
	}
	if f&4 != 0 {
		chance = 3
	}
	if f&0x40 != 0 {
		chance = 4
	}
	if f&8 != 0 {
		chance = 7
	}
	if f&0x80 != 0 {
		chance = 0
	}
	g := &e.cmap.ground
	for x := 0; x <= 0x31; x++ {
		for y := 1; y <= 0x18; y++ {
			if e.groundInfo(g[y][x])[3] != 0x16 || e.groundInfo(g[y-1][x])[3] != 0x16 || e.Dice(1, 100) > chance {
				continue
			}
			v := e.Dice(1, 10)
			if v > 8 {
				v = e.Dice(1, 2) + 4
			} else {
				v = (v + 1) >> 1
			}
			if e.terrainFlags()&0x40 != 0 {
				v = e.Dice(1, 3) + 3
			}
			g[y][x] = byte(v + 0x23)
			if v < 5 {
				g[y-1][x] = byte(v + 0x1F)
			}
		}
	}
}

// wildPlants = ovr11:0E9E: trees, bushes and the like on open ground.
func (e *Engine) wildPlants() {
	p1, p2, p3, p4, p5 := 0, 6, 0xF, 0x28, 0
	f := e.terrainFlags()
	if f&2 != 0 {
		p3, p4 = 0x2D, 0xA
	}
	if f&4 != 0 {
		p3 = 0x19
	}
	if f&0x40 != 0 {
		p4, p5 = 5, 0x1E
	}
	if f&0x80 != 0 {
		p1, p4, p3 = 0xF, 0x19, 0xA
	}
	g := &e.cmap.ground
	for x := 0; x <= 0x31; x++ {
		for y := 0; y <= 0x18; y++ {
			if e.groundInfo(g[y][x])[3] != 0x16 {
				continue
			}
			r := e.Dice(1, 0xFF)
			switch {
			case r <= p1:
				v := e.Dice(1, 4)
				if v == 4 && y > 0 && e.groundInfo(g[y-1][x])[3] == 0x16 {
					g[y-1][x], g[y][x] = 0x41, 0x40
				}
				if v < 4 {
					g[y][x] = byte(v + 0x3C)
				}
			case r <= p1+p2:
				g[y][x] = byte(e.Dice(1, 3) + 0x36)
			case r <= p3+p1+p2:
				g[y][x] = byte(e.Dice(1, 4) + 0x29)
			case r <= p5+p3+p1+p2:
				g[y][x] = byte((e.Dice(1, 10)-1)/3 + 1 + 0x38)
			case r <= p4+p5+p3+p1+p2:
				v := e.Dice(1, 4)
				if v == 4 && e.terrainFlags()&0x40 == 0 {
					v = e.Dice(1, 3)
				}
				g[y][x] = byte(v + 0x2D)
			}
		}
	}
}

// --- placement (ovr11:19F5) -------------------------------------------

// placement is the DS:4440..4658 state of ovr11:19F5: per side the
// formation templates (4 teams of 6 rows by 11), origin, size and facing.
type placement struct {
	slots  [2][4][6][11]byte // DS:4440
	ox, oy [2]int8           // DS:4650/4652
	rows   [2]byte           // DS:4654
	dir    [2]byte           // DS:4656
	side   byte              // DS:4658
}

// cloud is a 0x13-byte DS:6DBB record (linked at +4): a stinking cloud
// over the four squares around X, Y.
type cloud struct {
	Ptr00   *Character // +0x00 the caster
	Ground  [4]byte    // +0x08 ground under each square
	Covered [4]byte    // +0x0C square is covered
	X, Y    byte       // +0x10
	Num     byte       // +0x12 the caster's cloud number (affect data >> 4)
}

// placeCombatants = ovr11:19F5.
func (e *Engine) placeCombatants() {
	e.countSides()
	e.cpos = [maxCombatants + 1]combatPos{}
	e.combatants = make([]*Character, maxCombatants)
	e.ncomb = 0
	e.combatRedrawMap()
	p := &e.place
	p.ox[0], p.oy[0] = 0, 0
	p.dir[0] = byte(e.facing / 2)
	dist := int(e.game.MonsterDist)
	p.ox[1] = int8(dist*dirDX[e.facing] + int(p.ox[0]))
	p.oy[1] = int8(dist*dirDY[e.facing] + int(p.oy[0]))
	p.dir[1] = byte((e.facing + 4) % 8 / 2)
	for s := 0; s <= 1; s++ {
		p.rows[s] = byte((int(e.rv.side[s]) + 1) >> 1)
	}
	for s := 0; s <= 1; s++ {
		for team := 0; team <= 3; team++ {
			d := int(p.dir[s])
			if team == 1 {
				d = 4
			}
			for y := 0; y <= 5; y++ {
				lr := placeRows[d][y]
				for x := 0; x <= 0xA; x++ {
					v := byte(0)
					if int(int8(lr[0])) <= x && int(int8(lr[1])) >= x {
						v = 1
					}
					p.slots[s][team][y][x] = v
				}
			}
		}
	}
	idx := 1
	e.ncomb = 1
	e.cpos[0].size = 1 // original quirk: the absolute DS:5F2A is record 0's size, counted up per combatant
	for i := 0; i < len(e.party); i++ {
		c := e.party[i]
		e.flushKey()
		e.combatants[idx-1] = c
		p.side = c.Side
		e.cpos[idx].id = byte(idx)
		e.cpos[idx].size = c.Size & 7
		if e.placeOne(idx) {
			if c.Okay == 0 {
				e.cpos[idx].size = 0
				if e.run.duelMode == 0 && c.Combat.Extra == 0 {
					x, y := e.cpos[idx].x, e.cpos[idx].y
					e.nbodies++
					e.bodies[e.nbodies] = body{c: c, x: x, y: y, ground: e.cmap.ground[y][x]}
					e.cmap.ground[y][x] = 0x1F
				}
			}
			e.combatRedrawMap()
			idx++
			e.cpos[0].size++
			e.ncomb++
			continue
		}
		e.cpos[idx].size = 0
		if c.Combat.Extra == 1 { // no room: the extra leaves
			e.combatants[idx-1] = nil
			e.partySel = i
			e.RemoveSelected(true, false)
			i--
			continue
		}
		idx++
		e.cpos[0].size++
		e.ncomb++
	}
}

// formationEdge = ovr11:1403 (x, y): outside the 11x6 formation both ways.
func formationEdge(x, y int) bool {
	return (x < 0 || x >= 0xB) && (y < 0 || y >= 6)
}

// placeAt = ovr11:1434 (idx, x, y, ox, oy, team): put combatant idx on
// formation slot (x,y) of the team block at (ox,oy) if the ground lets it.
func (e *Engine) placeAt(idx, x, y, ox, oy, team int) bool {
	p := &e.place
	if x < 0 || y < 0 || x > 0xA || y > 5 || p.slots[p.side][team][y][x] == 0 {
		return false
	}
	e.cpos[idx].x = byte(x + ox*6 + oy*5 + 0x16)
	e.cpos[idx].y = byte(oy*5 + y + 0xA)
	who, g := e.groundAt(e.combatants[idx-1], 8)
	if who == 0 && g > 0 && e.groundInfo(g)[0] < 0xFF {
		p.slots[p.side][team][y][x] = 0
		return true
	}
	return false
}

// placeOne = ovr11:1577 (idx): walk the formation of the combatant's side,
// row by row outwards from the middle, falling back on the other teams
// (blocks beside the first) until a free slot takes it.
func (e *Engine) placeOne(idx int) bool {
	p := &e.place
	s := p.side
	first, fail, state := true, false, 1
	var row, team, k, n int
	var bx, by, x, y int
	var dx, dy int
	ox, oy := int(p.ox[s]), int(p.oy[s])
	for {
		dir := int(placeTeamDirs[p.dir[s]][team] >> 1)
		turn := func(q int) {
			d := int(facingDir[(dir+q)%4])
			dx, dy = dirDX[d], dirDY[d]
		}
		switch state {
		case 1:
			turn(2)
			t := 0
			if team > 0 {
				t = 1
			}
			bx = int(placeStartX[t][dir]) + dx*row
			by = int(placeStartY[t][dir]) + dy*row
			x, y = bx, by
			k, state, n = 1, 2, 1
		case 2:
			turn(1)
			x, y = bx+dx*k, by+dy*k
			state = 3
			n++
		case 3:
			turn(3)
			x, y = bx+dx*k, by+dy*k
			state = 2
			k++
			n++
		}
		out := x < 0 || y < 0 || x > 0xA || y > 5
		if state > 1 {
			full := out && !formationEdge(x, y)
			if !full {
				if first {
					full = n >= int(p.rows[s])
				} else {
					full = n > 0xB
				}
			}
			if full {
				row++
				if s == 0 && p.dir[0]&1 != 0 && team == 0 && row == 1 {
					cx, cy := e.posX+int(p.ox[0]), e.posY+int(p.oy[0])
					open := false
					for a := 1; a <= 3; a++ {
						d := int(placeDirs[p.dir[s]][a])
						if e.viewMode > 1 || e.cellEdge(cx, cy, d) != 1 {
							open = true
						}
					}
					if open {
						row++
					}
				}
				state, first = 1, false
			}
		}
		ok := false
		if out && formationEdge(x, y) {
			state = 0
			for team < 3 && state != 1 {
				team++
				cx, cy := e.posX+int(p.ox[s]), e.posY+int(p.oy[s])
				d := int(placeDirs[p.dir[s]][team])
				if e.viewMode <= 1 && e.cellEdge(cx, cy, d) == 1 {
					continue
				}
				ox, oy = dirDX[d]+int(p.ox[s]), dirDY[d]+int(p.oy[s])
				row, state = 0, 1
			}
			if state != 1 {
				fail = true
			}
		}
		if !out {
			ok = e.placeAt(idx, x, y, ox, oy, team)
		}
		if ok || fail {
			return !fail
		}
	}
}
