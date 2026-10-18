package engine

// The combat main loop and the player's command menus (ovr09).

import (
	"image"
	"slices"
	"strings"
)

// Combat = ovr09:007A: set up the battle, then rounds until one side is
// down (or the party stops fighting).
func (e *Engine) Combat() {
	e.beginFlag = 5
	e.ext.battleLog, e.ext.panel = nil, nil
	e.combatTargets = true // DS:6B84 = 0068:0066 (ovr14:20C3)
	e.combatSetup()
	done := e.rv.side[0] == 0 || e.rv.side[1] == 0
	for !done {
		e.countSides()
		for _, c := range e.party {
			e.o14_0000(c)
		}
		e.game.Surprise = 0
		for c := e.nextActor(); c != nil; c = e.nextActor() {
			e.takeTurn(c)
		}
		done = e.endRound()
	}
	e.combatEnd()
}

// combatEnd = ovr09:0000: free the clouds, the map and the sprite.
func (e *Engine) combatEnd() {
	e.combatTargets = false // DS:6B84 = 0096:0034 (ovr23:0A91)
	e.clouds = nil
	e.cmap = nil
	e.spr = nil
	for _, p := range e.party {
		if p.Control < 0x80 {
			p.Auto = 0 // extra: Quick lasts one battle (the original kept it until space)
		}
	}
	e.combatViewEnter()
}

// nextActor = ovr09:0131 (var char): the highest initiative left (combat
// block +3); ties go to the higher d100. nil when nobody is left.
func (e *Engine) nextActor() *Character {
	var best, bestRoll byte
	var who *Character
	for _, c := range e.party {
		r := byte(e.Dice(1, 100))
		ini := c.Combat.Delay
		if ini > best {
			bestRoll = r
		}
		if ini >= best && r >= bestRoll {
			bestRoll, best, who = r, ini, c
		}
	}
	if best == 0 {
		return nil
	}
	return who
}

// takeTurn = ovr09:01F6 (char).
func (e *Engine) takeTurn(c *Character) {
	ci := c.Combat
	ci.Attackers, ci.Flank, ci.Guarding = 0, 0, 0
	e.affectGroup(c, 7)
	if int8(ci.Delay) <= 0 {
		return // no cursor erase when the turn is skipped
	}
	if ci.Delay == 0x14 {
		ci.Delay = 0x13
	}
	e.selectChar(c)
	e.combatRedrawChar(c, 2, 1)
	e.ApplyItems(c)
	e.combatPanel(c)
	e.affectGroup(c, 0xF)
	if int8(ci.Delay) > 0 {
		if c.Auto != 0 {
			e.o10_000F(c)
		} else {
			e.playerTurn(c)
		}
	}
	e.eraseCursor(e.combatX(c), e.combatY(c))
}

// selectChar makes c the selected member (DS:5D92).
func (e *Engine) selectChar(c *Character) {
	for i, p := range e.party {
		if p == c {
			e.partySel = i
		}
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// playerTurn = ovr09:031A (char): a spell being cast goes off, otherwise
// the combat menu until the turn is used.
func (e *Engine) playerTurn(c *Character) {
	if c.Okay == 0 {
		e.clearCombatInfo(c)
		return
	}
	ci := c.Combat
	if ci.Spell > 0 {
		s := ci.Spell
		ci.Spell = 0
		var quit bool
		e.CastSpell(s, 0, 1, &quit)
		e.clearCombatInfo(c)
		return
	}
	done := false
	for !done {
		switch e.combatMenu(c) {
		case 'M':
			done = e.moveAttack(c)
		case 'V':
			done = e.ViewCharacter() // ovr20:0AB3 (var done): ItemsMenu sets it
			e.o14_0D23(c)
			if !done {
				e.combatUseHook()
			}
			if e.ext.throwing != nil {
				done = e.throwItem(c)
			}
		case 'A':
			var aim aimTarget
			var ok bool
			e.ext.enemiesOnly = true // extra: Next/Prev pass over allies
			e.o14_3558(c, 0xFF, 1, 0, &ok, &aim)
			e.ext.enemiesOnly = false
			e.ext.autoAim = nil
			done = ok
		case 'I':
			done = e.ItemsMenu() // ovr20:0EF8 (var done)
			e.o14_0D23(c)
			if !done {
				e.combatUseHook()
			}
			if e.ext.throwing != nil {
				done = e.throwItem(c)
			}
		case 'C':
			var ok bool
			e.o14_2414(0, 0, &ok)
			done = ok
		case 'L':
			e.showBattleLog()
		case 'T':
			e.o14_116A(c)
			done = e.clearCombatInfo(c)
		case 'Q':
			e.quickOne(c)
			e.ClearRow24()
			e.ClearKeyboard()
			e.Delay(200)
			done = true
			e.o10_000F(c)
		case 'D':
			done = e.otherMenu(c)
		case '2':
			e.v6E30 ^= 1
			if e.v6E30 != 0 {
				e.BottomMessage("Magic On")
			} else {
				e.BottomMessage("Magic Off")
			}
		case ' ':
			for _, p := range e.party {
				if p.Control < 0x80 {
					p.Auto = 0
				}
			}
		case 0x10:
			ci.Delay = 0x14
			for _, p := range e.party {
				e.quickOne(p)
			}
			e.ClearRow24()
			e.Delay(200)
			done = true
		case '-':
			if e.o14_3B93() {
				e.combatRedrawChar(c, 3, 0)
				done = true
			} else {
				e.BottomMessage("That doesn't work")
			}
		}
		if !done {
			e.combatRedrawChar(c, 2, 1)
			e.combatPanel(c)
		}
	}
}

// combatMenu = ovr09:0666 (char, var key): the main combat menu.
func (e *Engine) combatMenu(c *Character) byte {
	e.affectGroup(c, 0xF)
	ci := c.Combat
	m := ""
	if ci.Moves > 0 {
		m += "Move "
	}
	m += "View Aim "
	if e.combatItems(c) {
		m += "Items " // extra: Use renamed, and only with something to use or throw
	}
	spells := false
	for i := 0; i <= 0x14; i++ {
		if c.Memorized[i] > 0 {
			spells = true
		}
	}
	if spells && ci.CanCast != 0 && e.area.NoMagic == 0 {
		m += "Cast "
	}
	var undead *Character
	if int8(c.Levels[clsCleric]) > 0 && ci.TurnUsed == 0 && e.o14_1351(c, &undead) { // extra: only with undead to turn
		m += "Turn "
	}
	m += "Quick Done" // extra: L still opens the Log
	// extra: click a foe to aim at it (and attack it if it can be), a square in reach to walk there
	reach := map[[2]int][]int{}
	if ci.Moves > 0 {
		reach = e.moveReach(c)
		delete(reach, [2]int{e.combatX(c), e.combatY(c)})
	}
	for sq := range reach {
		e.snap.reach = append(e.snap.reach, image.Pt(sq[0], sq[1]))
	}
	defer func() { e.snap.reach = nil }()
	mark := e.regionMark()
	defer e.regionCut(mark)
	e.addRegion(region{x0: pageCol, y0: 1, x1: pageCol + viewW*3 - 1, y1: 21, click: func(cx, cy int) uint16 {
		sx, sy, in := e.combatSquare(cx, cy)
		if !in {
			return 0
		}
		if who := e.occ[sy][sx]; who > 0 {
			t := e.combatant(who)
			if t == nil || t.Side == c.Side {
				return 0
			}
			if p := e.pathNextTo(c, t, reach); p != nil { // a melee weapon out of reach: walk up, then attack
				e.ext.autoPath, e.ext.aimAfterWalk = p, t
				return 'M'
			}
			e.ext.autoAim, e.ext.autoJumped = t, false
			return 'A'
		}
		if p, ok := reach[[2]int{sx, sy}]; ok {
			e.ext.autoPath = append([]int(nil), p...)
			return 'M'
		}
		return 0
	}})
	if t := e.ext.aimAfterWalk; t != nil { // extra: walked up to a clicked foe: now the attack
		e.ext.aimAfterWalk = nil
		if t.Okay != 0 && e.rangeTo(c, t) <= 1 {
			e.ext.autoAim, e.ext.autoJumped = t, false
			return 'A'
		}
	}
	for {
		e.ext.hiddenKeys = "L" // extra: the Log, off the bar
		key, extra := e.DisplayInput("", m, 0xD, 0xA, 0xF, true, false)
		e.ext.hiddenKeys = ""
		if extra && !(key == 0x10 || key == 0x13 || key == '-' || key == '2') { // set cs:0626
			key = 0
		}
		switch key { // set cs:0646
		case 0x10, 0x13, ' ', '-', '2', 'A', 'C', 'D', 'L', 'I', 'M', 'Q', 'T', 'V':
			return key
		}
	}
}

// endRound = ovr09:0880 (var done): the clock, the dying, the end check.
func (e *Engine) endRound() (done bool) {
	e.AdvanceClock(1, 1)
	e.v6DE3++
	e.o14_2879()
	for _, c := range e.party {
		e.affectGroup(c, 0x13)
		e.stinkingCloud(c)
		if c.Status == 5 {
			c.Combat.Dying++
			if c.Combat.Dying > 9 {
				c.Status = 6
			}
		}
	}
	if e.bandage(false) {
		e.BottomMessage("Your Teammate is Dying")
	}
	e.countSides()
	e.combatCentre(e.cmap.viewX+viewW/2, e.cmap.viewY+3, 0xFF, 8)
	if e.rv.side[0] == 0 || e.rv.side[1] == 0 {
		done = true
	}
	if e.rv.side[0] > 0 && e.rv.side[1] == 0 && e.v49F4 == 0 {
		if e.YesNo(0xD, 0xA, 0xF, "Continue Battle:") == 'Y' {
			done = false
		}
	}
	return
}

// moveAttack = ovr09:09DF (char, var done): step around with the keypad;
// stepping into someone attacks, off the map flees. Escape undoes the
// moves of this menu.
func (e *Engine) moveAttack(c *Character) (done bool) {
	ci := c.Combat
	moves, face := ci.Moves, ci.Facing
	x, y := e.combatX(c), e.combatY(c)
	key := byte(0x20)
	done = false
	defer e.rawArrows("keypad, arrows, click a square=step, attack, walk to a framed one|Enter, click self=done|Esc=undo the moves")()
	mark := e.regionMark()
	defer e.regionCut(mark)
	var reach map[[2]int][]int
	var path []int                // extra: the steps to a clicked square, taken one by one
	auto := e.ext.autoPath != nil // extra: sent from the main menu: walk there and hand back
	path, e.ext.autoPath = e.ext.autoPath, nil
	{ // extra (nav.go): a click on a square next to the character steps there, on a farther reachable one walks there
		e.addRegion(region{x0: pageCol, y0: 1, x1: pageCol + viewW*3 - 1, y1: 21, click: func(cx, cy int) uint16 {
			sx, sy, in := e.combatSquare(cx, cy)
			dx, dy := sx-e.combatX(c), sy-e.combatY(c)
			if !in {
				return 0
			}
			if dx == 0 && dy == 0 {
				return 13
			}
			if dx < -1 || dx > 1 || dy < -1 || dy > 1 {
				if p, ok := reach[[2]int{sx, sy}]; ok {
					path = append([]int(nil), p...)
					return keyNone
				}
				return 0
			}
			for d := 0; d < 8; d++ {
				if dirDX[d] == dx && dirDY[d] == dy {
					return stepKeys[d]
				}
			}
			return 0
		}})
	}
	for ci.Moves > 1 && key != 0 && key != 0xD {
		if len(path) > 0 { // walking to a clicked square: the next step, while it is still free
			d := path[0]
			path = path[1:]
			if who, g := e.groundAt(c, d); who > 0 || g == 0 || e.groundInfo(g)[0] > ci.Moves {
				path = nil
				continue
			}
			key = stepLetters[d]
			e.Delay(0x32)
		} else if auto {
			break
		} else {
			reach = e.moveReach(c)
			e.markReach(reach, true)
			e.ClearCells(0x18, 0x18, 0x27, 0x18)
			key, _ = e.DisplayInput("Move/Attack, Move Left = "+itoa(int(ci.Moves>>1))+" ", "", 0xA, 0xA, 0xF, true, false)
			e.markReach(reach, false)
		}
		d := 8
		switch key {
		case 0:
			ci.Moves = moves
			e.combatClearIcon(0, 0, e.combatIndex(c))
			done = !e.combatSquareFree(c, x, y, false)
			e.combatCentre(e.combatX(c), e.combatY(c), 0, 8)
			ci.Facing = face
		case 'H':
			d = 0
		case 'I':
			d = 1
		case 'M':
			d = 2
		case 'Q':
			d = 3
		case 'P':
			d = 4
		case 'O':
			d = 5
		case 'K':
			d = 6
		case 'G':
			d = 7
		}
		if d >= 8 {
			continue
		}
		e.combatTurn(c, byte(d), 0, false)
		who, g := e.groundAt(c, d)
		switch {
		case who > 0:
			done = e.attackTarget(c, e.combatant(who), done)
		case g == 0:
			switch e.YesNo(0xD, 0xA, 0xF, "Flee:") {
			case 'Y':
				done = e.o14_0C6A(c)
			case 'N':
				done = false
			}
		case e.groundInfo(g)[0] > ci.Moves:
			e.BottomMessage("Blocked")
		default:
			e.o14_08B2(c, d)
			if c.Okay == 0 {
				done = e.clearCombatInfo(c)
				continue
			}
			e.o14_0712(c, d)
			if c.Okay == 0 {
				done = e.clearCombatInfo(c)
			}
			e.stinkingCloud(c)
			if e.hasAffect286B(c) {
				done = e.clearCombatInfo(c)
			}
		}
	}
	if ci.Moves < 2 {
		ci.Moves = 0
	}
	return
}

// stepLetters are the keypad letters moveAttack reads for the 8 directions.
var stepLetters = [8]byte{'H', 'I', 'M', 'Q', 'P', 'O', 'K', 'G'}

// moveReach (extra) finds the squares c can walk to with the moves it has
// left, as moveAttack's steps allow them (2 a step, 3 diagonally, free
// ground no dearer than the moves left), each with the steps that keep
// the most moves.
func (e *Engine) moveReach(c *Character) map[[2]int][]int {
	idx := e.combatIndex(c)
	ox, oy := e.cpos[idx].x, e.cpos[idx].y
	defer func() { e.cpos[idx].x, e.cpos[idx].y = ox, oy }()
	type node struct {
		left int
		path []int
	}
	best := map[[2]int]node{{int(ox), int(oy)}: {left: int(c.Combat.Moves)}}
	queue := [][2]int{{int(ox), int(oy)}}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		n := best[at]
		if n.left <= 1 {
			continue
		}
		e.cpos[idx].x, e.cpos[idx].y = byte(at[0]), byte(at[1])
		for d := 0; d < 8; d++ {
			cost := 2 + d&1
			who, g := e.groundAt(c, d)
			if cost > n.left || who > 0 || g == 0 || int(e.groundInfo(g)[0]) > n.left {
				continue
			}
			to := [2]int{at[0] + dirDX[d], at[1] + dirDY[d]}
			if b, ok := best[to]; ok && b.left >= n.left-cost {
				continue
			}
			best[to] = node{n.left - cost, append(append([]int(nil), n.path...), d)}
			queue = append(queue, to)
		}
	}
	out := map[[2]int][]int{}
	for k, n := range best {
		if len(n.path) > 0 {
			out[k] = n.path
		}
	}
	return out
}

// markReach (extra) shows the reachable squares in view with a dotted
// frame (on) or draws them plain again (off).
func (e *Engine) markReach(reach map[[2]int][]int, on bool) {
	e.snap.marks = nil
	for k := range reach {
		if on {
			e.snap.marks = append(e.snap.marks, image.Pt(k[0], k[1]))
		}
	}
	for k := range reach {
		sx, sy := k[0]-e.cmap.viewX, k[1]-e.cmap.viewY
		if !combatInView(sx, sy) {
			continue
		}
		if on {
			e.markSquare(sx, sy, 0xB)
		} else {
			e.drawTile(sx*3, sy*3, e.tileOf(k[0], k[1]))
			e.drawOccupant(k[0], k[1])
		}
	}
	e.Flip()
}

// rangedWeapon = 00BA:2E03 (char): the readied weapon shoots (base item
// +0xC above 1).
func (e *Engine) rangedWeapon(c *Character) bool {
	w := c.Slots[0]
	return w != nil && e.baseItem(w.Type, 0xC) > 1
}

// throwable = 00BA:2E46 (char): a ranged weapon that also works in melee
// (base item flags 0x14).
func (e *Engine) throwable(c *Character) bool {
	return e.rangedWeapon(c) && e.baseItem(c.Slots[0].Type, 0xE)&0x14 == 0x14
}

// throwableItem (extra): the item would be a throwable weapon if readied.
func (e *Engine) throwableItem(it *Item) bool {
	return e.baseItem(it.Type, 0xC) > 1 && e.baseItem(it.Type, 0xE)&0x10 != 0 // thrown: also darts and javelins, used up
}

// throwItem (extra): Throw from the inventory readies the item in place of
// the weapon, aims as Aim does, then readies the weapon again. Nothing
// changes when the swap fails or the aim is given up.
func (e *Engine) throwItem(c *Character) (done bool) {
	it, w := e.ext.throwing, c.Slots[0]
	e.ext.throwing = nil
	saved := e.partySel
	for i, p := range e.party {
		if p == c {
			e.partySel = i // readyItem works on the selected member
		}
	}
	defer func() { e.partySel = saved }()
	if w != nil {
		e.readyItem(w)
		if w.Readied != 0 { // cursed
			return false
		}
		e.ApplyItems(c) // free the weapon's hands: readyItem counts c.Hands
	}
	e.readyItem(it)
	e.ApplyItems(c)
	if it.Readied != 0 {
		var aim aimTarget
		e.ext.enemiesOnly = true
		e.o14_3558(c, 0xFF, 1, 0, &done, &aim)
		e.ext.enemiesOnly = false
		if slices.Contains(c.Items, it) && it.Readied != 0 {
			e.readyItem(it)
		}
	}
	if w != nil && slices.Contains(c.Items, w) && w.Readied == 0 {
		e.readyItem(w)
	}
	e.ApplyItems(c)
	return done
}

// ammoFor = 00BA:2E8B (char, var item): the item a ranged attack uses up
// (the weapon itself, the arrows in slot 11 or the bolts in slot 12); true
// when there is one or the weapon needs none (flags 0x0A).
func (e *Engine) ammoFor(c *Character, it **Item) bool {
	w := c.Slots[0]
	*it = nil
	var f byte
	if w != nil {
		f = e.baseItem(w.Type, 0xE)
		if f&0x10 != 0 {
			*it = w
		}
		if f&8 != 0 {
			if f&1 != 0 {
				*it = c.Slots[11]
			}
			if f&0x80 != 0 {
				*it = c.Slots[12]
			}
		}
	}
	return *it != nil || f == 0xA
}

// attackTarget = ovr09:0D45 (char, target, var done).
// done passes through when no attack is made.
func (e *Engine) attackTarget(c, t *Character, done bool) bool {
	if e.rangedWeapon(c) && !e.throwable(c) {
		e.BottomMessage("Not with that weapon")
		return done
	}
	if !e.o14_2998(c, t) {
		return done
	}
	c.Combat.Target = t
	if e.o14_0E82(c, t) {
		return true
	}
	e.o14_17F6(c, t)
	return e.o14_1884(c, t, 0, nil)
}

// otherMenu = ovr09:0E45 (char, var done): Done's submenu.
func (e *Engine) otherMenu(c *Character) (done bool) {
	done = false
	m := ""
	if !e.rangedWeapon(c) || e.throwable(c) {
		m += "Guard "
	}
	m += "Delay Quit "
	if e.bandage(false) {
		m += "Bandage "
	}
	m += "Speed Exit"
	for key := byte(0x20); key != 0 && key != 'E' && !done; {
		key, _ = e.DisplayInput("", m, 0xD, 0xA, 0xF, false, false)
		switch key {
		case 'G':
			done = e.guard(c)
		case 'D':
			c.Combat.Delay = 1
			done = true
		case 'Q':
			done = e.clearCombatInfo(c)
		case 'B':
			done = e.bandage(true)
			done = e.clearCombatInfo(c)
		case 'S':
			e.speedMenu()
		}
	}
	return
}

// guard = 00BA:2686 (char): end the turn on guard.
func (e *Engine) guard(c *Character) bool {
	r := e.clearCombatInfo(c)
	c.Combat.Guarding = 1
	e.BottomMessage("Guarding")
	return r
}

// bandage = ovr09:0FFF (doIt): is a party member dying? With doIt the
// first one is bandaged.
func (e *Engine) bandage(doIt bool) bool {
	found := false
	for _, c := range e.party {
		if c.Combat.Extra == 0 && c.Side == 0 && c.Status == 5 {
			found = true
			if doIt {
				c.Status = 4
				c.Combat.Dying = 0
				e.CharMessage(c, "is bandaged", 0xA, true)
				doIt = false
			}
		}
	}
	return found
}

// speedMenu = ovr09:10F6: the message delay.
func (e *Engine) speedMenu() {
	for key := byte(0x20); key != 0 && key != 'E'; {
		m := " "
		if e.msgDelay < 9 {
			m += "Slower "
		}
		if e.msgDelay > 0 {
			m += "Faster "
		}
		m += "Exit"
		key, _ = e.DisplayInput("GameSpeed ("+itoa(int(e.msgDelay))+") :", m, 0xD, 0xA, 0xF, false, false)
		switch key {
		case 'S':
			e.msgDelay++
		case 'F':
			e.msgDelay--
		}
	}
}

// quickOne = ovr09:122B (char): the computer fights for it; a target on
// its own side is dropped.
func (e *Engine) quickOne(c *Character) {
	c.Auto = 1
	if t := c.Combat.Target; t != nil && t.Side == c.Side {
		c.Combat.Target = nil
	}
}

// combatItems (extra): c carries an item its items menu could Use (when
// it may use items this round) or Throw.
func (e *Engine) combatItems(c *Character) bool {
	for _, it := range c.Items {
		if it.Readied == 0 && e.throwableItem(it) ||
			c.Combat.CanUse != 0 && (e.isScroll(it) || it.Spells[1] > 0 && it.Spells[2] < 0x80) {
			return true
		}
	}
	return false
}

// combatPanel = 00BA:09E3 (char): the combatant's name, hit points, AC and
// damage (or its status) on one row under the view (extra: the original's
// side pane, gone for the full-width view).
func (e *Engine) combatPanel(c *Character) {
	e.ext.panel = c
	e.ClearCells(0, 0x17, 0x27, 0x17)
	x := 1
	e.displayName(c, x, 0x17, false)
	x += len(c.Name()) + 2
	e.DisplayString(x, 0x17, 0xF, "HP")
	e.displayHP(c, x+3, 0x17, false)
	x += 3 + len(itoa(int(c.HP))) + 2
	e.DisplayString(x, 0x17, 0xF, "AC")
	e.displayAC(c, x+3, 0x17)
	x += 8
	switch {
	case c.Okay == 0:
		e.DisplayString(x, 0x17, 0xF, cut(e.statusName(c), 0x27-x))
	case e.hasAffect286B(c):
		e.DisplayString(x, 0x17, 0xF, "(Helpless)")
	default:
		e.DisplayString(x, 0x17, 0xF, "Damage")
		e.DisplayString(x+7, 0x17, 0xA, cut(damageText(c), 0x27-x-7))
	}
}

// damageText is the damage of the readied attack, as the View page shows it.
func damageText(c *Character) string {
	d := itoa(int(c.Dice[0])) + "d" + itoa(int(c.Sides[0]))
	if b := int8(c.Bonus[0]); b > 0 {
		d += "+" + itoa(int(b))
	} else if b < 0 {
		d += "-" + itoa(-int(b)) // abs in 16 bits: -128 shows 128
	}
	return d
}

// combatSay (extra) shows combat text on rows 23-24 under the view and
// keeps it for the Log: fresh starts a new message, otherwise it goes on
// the current one; c (if any) is named first.
func (e *Engine) combatSay(c *Character, msg string, fresh bool) {
	if fresh || len(e.ext.battleLog) == 0 {
		e.ClearCells(0, 0x17, 0x27, 0x18)
		e.msgCol, e.msgRow = 0, 0x17
		e.ext.battleLog = append(e.ext.battleLog, "")
	} else {
		e.msgCol++
	}
	l := &e.ext.battleLog[len(e.ext.battleLog)-1]
	add := func(s string) {
		if *l != "" {
			*l += " "
		}
		*l += s
	}
	if c != nil {
		if e.msgCol+len(c.Name()) > 0x27 {
			e.msgCol, e.msgRow = 0, e.msgRow+1
		}
		e.displayName(c, e.msgCol, e.msgRow, false)
		e.msgCol += len(c.Name()) + 1
		add(c.Name())
	}
	if msg = strings.TrimSpace(msg); msg != "" {
		e.DisplayTextBlock(0, 0x17, 0x27, 0x18, 0xA, false, msg)
		add(msg)
	}
}

// showBattleLog (extra, Log) lists this battle's messages, newest at the
// bottom; Up/Down and the wheel scroll, any other key goes back.
func (e *Engine) showBattleLog() {
	off := e.modalOn()
	e.ext.paging = true
	defer func() { off(); e.ext.paging = false }()
	restore := e.saveScreen()
	defer restore()
	var lines []string
	for _, m := range e.ext.battleLog {
		lines = append(lines, wrapText(m, 0x24)...)
	}
	if len(lines) == 0 {
		lines = []string{"Nothing has happened yet."}
	}
	const rows = 0x15
	top := max(len(lines)-rows, 0)
	for {
		e.DrawBoxTitled(1, 1, 0x26, 0x17, 0, 0xF, "Battle Log")
		e.ClearCells(2, 2, 0x25, 0x17)
		for i := 0; i < rows && top+i < len(lines); i++ {
			e.DisplayString(2, 2+i, 0xA, lines[top+i])
		}
		hint := "press a key"
		if len(lines) > rows {
			hint = "up/down: scroll, " + hint
		}
		e.DisplayString(0x25-len(hint), 0x17, 0xA, hint)
		switch k := e.ReadKey(); k {
		case 0x4800:
			top = max(top-1, 0)
		case 0x5000:
			top = min(top+1, max(len(lines)-rows, 0))
		case 0x4900:
			top = max(top-rows, 0)
		case 0x5100:
			top = min(top+rows, max(len(lines)-rows, 0))
		default:
			return
		}
	}
}

// combatUseHook = 00BA:2B88: redraw the combat screen: the map frame
// (extra: full width, no panel frame), rows 23-24 cleared, the map centred on the view.
func (e *Engine) combatUseHook() {
	e.combatViewLeave() // ovr32:00E3: the combat palette
	e.DrawBoxTitled(1, 1, 0x26, 0x15, 0, 0, "")
	e.ClearCells(0, 0x17, 0x27, 0x18)
	e.combatCentre(e.cmap.viewX+viewW/2, e.cmap.viewY+3, 0xFF, 8)
}

// pathNextTo (extra) is the shortest walk in reach that brings c, armed
// for melee, next to foe t (nil: t is in reach of its weapon already, c
// shoots, or no square next to t can be reached).
func (e *Engine) pathNextTo(c, t *Character, reach map[[2]int][]int) []int {
	if e.rangedWeapon(c) || e.rangeTo(c, t) <= 1 {
		return nil
	}
	tx, ty, n := e.combatX(t), e.combatY(t), e.combatSize(t)
	var best []int
	found := false
	for sq, p := range reach {
		if sq[0] >= tx-1 && sq[0] <= tx+n && sq[1] >= ty-1 && sq[1] <= ty+n && (!found || len(p) < len(best)) {
			best, found = p, true
		}
	}
	if !found {
		return nil
	}
	return append([]int(nil), best...)
}
