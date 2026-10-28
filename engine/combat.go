package engine

// The combat main loop and the player's command menus (ovr09).

import (
	"image"
	"slices"
	"strconv"
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
			e.turnSetup(c)
		}
		e.game.Surprise = 0
		for c := e.nextActor(); c != nil; c = e.nextActor() {
			e.takeTurn(c)
			if e.countSides(); e.rv.side[0] == 0 || e.rv.side[1] == 0 { // extra: the last one down ends the battle at once
				break
			}
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
			e.computerTurn(c)
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
	why := "" // extra: a refusal, on the status row once the panel is back (no flashing)
	for !done {
		switch e.combatMenu(c) {
		case 'M':
			done = e.moveAttack(c)
		case 'V':
			done = e.ViewCharacter() // ovr20:0AB3 (var done): ItemsMenu sets it
			e.setSpeed(c)
			if !done {
				e.combatUseHook()
			}
			if e.ext.throwing != nil {
				done = e.throwItem(c)
			}
		case 'A':
			if !e.foeInReach(c) { // extra: no aiming with nothing to attack (with no foe in sight Next/Prev would cycle the party)
				e.ext.autoAim = nil
				why = "No foe in reach"
				break
			}
			var aim aimTarget
			var ok bool
			e.ext.enemiesOnly = true // extra: Next/Prev pass over allies
			e.playerAim(c, 0xFF, 1, 0, &ok, &aim)
			e.ext.enemiesOnly = false
			e.ext.autoAim = nil
			done = ok
		case 'I':
			done = e.ItemsMenu() // ovr20:0EF8 (var done)
			e.setSpeed(c)
			if !done {
				e.combatUseHook()
			}
			if e.ext.throwing != nil {
				done = e.throwItem(c)
			}
		case 'C':
			var ok bool
			e.castSpell(0, 0, &ok)
			done = ok
		case 'L':
			e.showBattleLog()
		case 'T':
			e.clericTurnUndead(c)
			done = e.clearCombatInfo(c)
		case 'Q':
			e.quickOne(c)
			e.ClearRow24()
			e.ClearKeyboard()
			e.Delay(200)
			done = true
			e.computerTurn(c)
		case 'D':
			done = e.otherMenu(c)
		case 'G': // extra: the Done menu's, from the main menu (combatMenu offers them only when they apply)
			done = e.guard(c)
		case 'B':
			e.bandage(true)
			done = e.clearCombatInfo(c)
		case '2':
			e.magicOn ^= 1
			if e.magicOn != 0 {
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
			if e.stingCheat() {
				e.combatRedrawChar(c, 3, 0)
				done = true
			} else {
				e.refuse("That doesn't work")
			}
		}
		if !done {
			e.combatRedrawChar(c, 2, 1)
			e.combatPanel(c)
			if why != "" {
				e.showWhy(why)
				why = ""
			}
		}
	}
}

// combatMenu = ovr09:0666 (char, var key): the main combat menu.
func (e *Engine) combatMenu(c *Character) byte {
	e.affectGroup(c, 0xF)
	ci := c.Combat
	m := "View aiM " // extra: no Move (a click on the map walks), Aim on M: W A S D stay free
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
	if int8(c.Levels[clsCleric]) > 0 && ci.TurnUsed == 0 && e.weakestUnturned(c, &undead) { // extra: only with undead to turn
		m += "Turn "
	}
	m += "Quick dOne" // extra: L still opens the Log; Done on O
	// extra: click a foe to aim at it (and attack it if it can be), a square in reach to walk there
	reach := map[[2]int][]int{}
	if ci.Moves > 0 {
		reach = e.moveReach(c)
		delete(reach, [2]int{e.combatX(c), e.combatY(c)})
	}
	e.snap.paths = map[image.Point][]Step{}
	for sq, p := range reach {
		e.snap.reach = append(e.snap.reach, image.Pt(sq[0], sq[1]))
		e.snap.paths[image.Pt(sq[0], sq[1])] = e.walkSteps(c, p)
	}
	defer func() { e.snap.reach, e.snap.paths = nil, nil }()
	mark := e.regionMark()
	defer e.regionCut(mark)
	e.addRegion(region{x0: pageCol, y0: 1, x1: pageCol + viewW*3 - 1, y1: 21, click: func(cx, cy int) uint16 {
		sx, sy, in := e.combatSquare(cx, cy)
		if !in {
			return 0
		}
		if who := e.occ[sy][sx]; who > 0 {
			t := e.combatant(who)
			if t == c {
				return 'V' // one's own figure: the sheet
			}
			if t == nil || t.Side == c.Side {
				return 0
			}
			if p := e.pathNextTo(c, t, reach); p != nil { // a melee weapon out of reach: walk up, then attack
				e.ext.autoPath, e.ext.aimAfterWalk = p, t
				return 'M'
			}
			e.ext.autoAim, e.ext.autoJumped = t, false
			return 'M' // aiM, with the foe set
		}
		if p, ok := reach[[2]int{sx, sy}]; ok {
			e.ext.autoPath = append([]int(nil), p...)
			return 'M'
		}
		return 0
	}, square: func(x, y int) uint16 { // extra (remaster): the hovered unit's panel on the status row, a foe's name in red
		t := c
		if who := e.occ[y][x]; who > 0 {
			if u := e.combatant(who); u != nil {
				t = u
			}
		}
		if t != e.ext.panel {
			e.combatPanel(t)
			if t.Side != c.Side {
				e.DisplayString(0, messageRow, colLightRed, t.Name())
			}
		}
		return keyNone
	}})
	defer func() {
		if e.ext.panel != c {
			e.combatPanel(c)
		}
	}()
	// extra: the Done menu's turn enders straight from here (keys off the bar), and
	// the foes a click attacks now (in reach of the weapon, or a walk away from it)
	hidden := "L"
	e.snap.actions = nil // fresh: published views keep the old ones
	if !e.rangedWeapon(c) || e.throwable(c) {
		hidden += "G"
		e.snap.actions = append(e.snap.actions, MenuItem{"Guard", 'G'})
	}
	if e.bandage(false) {
		hidden += "B"
		e.snap.actions = append(e.snap.actions, MenuItem{"Bandage", 'B'})
	}
	e.snap.actions = append(e.snap.actions, MenuItem{"Wait", 'D'}) // Done's Delay: views send Done first
	e.snap.targets, e.snap.hits = nil, map[int]int{}
	rng := 1
	if w := c.Slots[slotWeapon]; w != nil && e.base(w.Type).Range > 1 {
		rng = int(e.base(w.Type).Range) - 1
	}
	for i := 1; i <= e.ncomb; i++ {
		t := e.combatant(byte(i))
		if t == nil || t.Side == c.Side || t.Okay == 0 || e.cpos[i].size == 0 || !e.mayActAgainst(c, t) {
			continue
		}
		walk := e.pathNextTo(c, t, reach)
		if !e.canTarget(c, t, rng, 1) && walk == nil {
			continue
		}
		e.snap.targets = append(e.snap.targets, i)
		e.snap.hits[i] = e.hitChance(c, t)
		if walk != nil { // hovering the foe shows the walk up to it
			steps, n := e.walkSteps(c, walk), e.combatSize(t)
			for d := range n * n {
				e.snap.paths[image.Pt(e.combatX(t)+d%n, e.combatY(t)+d/n)] = steps
			}
		}
	}
	defer func() { e.snap.actions, e.snap.targets, e.snap.hits = nil, nil, nil }()
	if t := e.ext.aimAfterWalk; t != nil { // extra: walked up to a clicked foe: now the attack
		e.ext.aimAfterWalk = nil
		if t.Okay != 0 && e.rangeTo(c, t) <= 1 {
			e.ext.autoAim, e.ext.autoJumped = t, false
			return 'A'
		}
	}
	for {
		e.ext.hiddenKeys = hidden // extra: the Log, Guard, Wait, Bandage: off the bar
		key, extra := e.DisplayInput("", m, colLightMagenta, colLightGreen, colWhite, true, false)
		e.ext.hiddenKeys = ""
		if extra && !(key == 0x10 || key == 0x13 || key == '-' || key == '2') { // set cs:0626
			key = 0
		}
		if key == 'M' && e.ext.autoPath == nil { // extra: aiM (a click on the map walks with autoPath set)
			key = 'A'
		} else if key == 'O' { // extra: dOne
			key = 'D'
		}
		switch key { // set cs:0646
		case 0x10, 0x13, ' ', '-', '2', 'A', 'C', 'D', 'L', 'I', 'M', 'Q', 'T', 'V', 'G', 'B':
			return key
		}
	}
}

// endRound = ovr09:0880 (var done): the clock, the dying, the end check.
func (e *Engine) endRound() (done bool) {
	e.AdvanceClock(1, 1)
	e.combatRound++
	e.foesHealthPercent()
	for _, c := range e.party {
		e.affectGroup(c, 0x13)
		e.stinkingCloud(c)
		if c.Status == statusDying {
			c.Combat.Dying++
			if c.Combat.Dying > 9 {
				c.Status = statusDead
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
	// extra: no "Continue Battle:" once the foes are gone (nothing to gain by it)
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
			e.ClearCells(24, 24, lastCol, 24)
			key, _ = e.DisplayInput("Move/Attack, Move Left = "+strconv.Itoa(int(ci.Moves>>1))+" ", "", colLightGreen, colLightGreen, colWhite, true, false)
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
			switch e.YesNo(colLightMagenta, colLightGreen, colWhite, "Flee:") {
			case 'Y':
				done = e.tryFlee(c)
			case 'N':
				done = false
			}
		case e.groundInfo(g)[0] > ci.Moves:
			e.refuse("Blocked")
		default:
			e.freeAttacksOnLeave(c, d)
			if c.Okay == 0 {
				done = e.clearCombatInfo(c)
				continue
			}
			e.stepSquare(c, d)
			if c.Okay == 0 {
				done = e.clearCombatInfo(c)
			}
			e.stinkingCloud(c)
			if e.isHelpless(c) {
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

// walkSteps (extra) are the squares path walks c through, its own first,
// each marked when leaving it lets an enemy take a free attack.
func (e *Engine) walkSteps(c *Character, path []int) []Step {
	at := [2]int{e.combatX(c), e.combatY(c)}
	out := []Step{{Point: image.Pt(at[0], at[1])}}
	for _, d := range path {
		out[len(out)-1].Free = e.provokes(c, at, d)
		at = [2]int{at[0] + dirDX[d], at[1] + dirDY[d]}
		out = append(out, Step{Point: image.Pt(at[0], at[1])})
	}
	return out
}

// provokes (extra) is freeAttacksOnLeave's test without the attacks: does
// c stepping from square at in direction d leave an enemy that may strike
// it? Position, near list and targets are kept.
func (e *Engine) provokes(c *Character, at [2]int, d int) bool {
	idx := e.combatIndex(c)
	ox, oy, near, targets := e.cpos[idx].x, e.cpos[idx].y, slices.Clone(e.near), e.targets
	defer func() { e.cpos[idx].x, e.cpos[idx].y, e.near, e.targets = ox, oy, near, targets }()
	e.cpos[idx].x, e.cpos[idx].y = byte(at[0]), byte(at[1])
	before := slices.Clone(e.targets[1 : e.nearEnemies(c, 1)+1])
	e.cpos[idx].x, e.cpos[idx].y = byte(at[0]+dirDX[d]), byte(at[1]+dirDY[d])
	after := e.targets[1 : e.nearEnemies(c, 1)+1]
	for _, i := range before {
		t := e.combatant(i)
		if !slices.Contains(after, i) && t != nil && !e.isHelpless(t) && e.mayActAgainst(t, c) &&
			e.FindAffect(t, affBloodDrain) == nil && e.FindAffect(t, affStrikeTied) == nil {
			return true
		}
	}
	return false
}

// hitChance (extra) is a's chance in percent to hit b with one blow of
// the readied attack: attackSlots' number to reach and AttackHits' d20.
// ponytail: the affects that change the roll (bless, prayer, ...) are left
// out; a dry run of affect groups 0xB and the attacker/defender ones would add them.
func (e *Engine) hitChance(a, b *Character) int {
	if e.isHelpless(b) {
		return 100
	}
	need := b.AC[0]
	if e.rangedFacing(a, b) {
		need = b.AC[1] - 2
	}
	bonus := int(need)
	e.rangePenalty(a, b, &bonus)
	need = byte(bonus)
	adj := int(int8(byte(e.game.PartyHit)))
	if a.Side != sideParty {
		adj = int(int8(byte(e.game.MonsterHit)))
	}
	n := 0
	for r := 2; r <= 20; r++ {
		roll := r
		if r == 20 {
			roll = 100
		}
		if roll+int(a.THAC0)+adj >= int(need) {
			n++
		}
	}
	return n * 5
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
	w := c.Slots[slotWeapon]
	return w != nil && e.base(w.Type).Range > 1
}

// throwable = 00BA:2E46 (char): a ranged weapon that also works in melee
// (base item flags 0x14).
func (e *Engine) throwable(c *Character) bool {
	return e.rangedWeapon(c) && e.base(c.Slots[slotWeapon].Type).Usage&0x14 == 0x14
}

// throwableItem (extra): the item would be a throwable weapon if readied.
func (e *Engine) throwableItem(it *Item) bool {
	return e.base(it.Type).Range > 1 && e.base(it.Type).Usage&0x10 != 0 // thrown: also darts and javelins, used up
}

// throwItem (extra): Throw from the inventory readies the item in place of
// the weapon, aims as Aim does, then readies the weapon again. Nothing
// changes when the swap fails or the aim is given up.
func (e *Engine) throwItem(c *Character) (done bool) {
	it, w := e.ext.throwing, c.Slots[slotWeapon]
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
		e.playerAim(c, 0xFF, 1, 0, &done, &aim)
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
	w := c.Slots[slotWeapon]
	*it = nil
	var f byte
	if w != nil {
		f = e.base(w.Type).Usage
		if f&0x10 != 0 {
			*it = w
		}
		if f&8 != 0 {
			if f&1 != 0 {
				*it = c.Slots[slotArrows]
			}
			if f&0x80 != 0 {
				*it = c.Slots[slotBolts]
			}
		}
	}
	return *it != nil || f == 0xA
}

// attackTarget = ovr09:0D45 (char, target, var done).
// done passes through when no attack is made.
func (e *Engine) attackTarget(c, t *Character, done bool) bool {
	if e.rangedWeapon(c) && !e.throwable(c) {
		e.refuse("Not with that weapon")
		return done
	}
	if !e.mayAttack(c, t) {
		return done
	}
	c.Combat.Target = t
	if e.sweep(c, t) {
		return true
	}
	e.countAttackTurn(c, t)
	return e.fullAttack(c, t, 0, nil)
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
		key, _ = e.DisplayInput("", m, colLightMagenta, colLightGreen, colWhite, false, false)
		switch key {
		case 'G':
			done = e.guard(c)
		case 'D':
			c.Combat.Delay = 1
			done = true
		case 'Q':
			done = e.clearCombatInfo(c)
		case 'B':
			e.bandage(true)
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
		if c.Combat.Extra == 0 && c.Side == sideParty && c.Status == statusDying {
			found = true
			if doIt {
				c.Status = statusUnconscious
				c.Combat.Dying = 0
				e.CharMessage(c, "is bandaged", true)
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
		key, _ = e.DisplayInput("GameSpeed ("+strconv.Itoa(int(e.msgDelay))+") :", m, colLightMagenta, colLightGreen, colWhite, false, false)
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
	e.ClearCells(0, messageRow, lastCol, messageRow)
	x := 0
	e.displayName(c, x, messageRow, false)
	x += len(c.Name()) + 2
	e.DisplayString(x, messageRow, colWhite, "HP")
	e.displayHP(c, x+3, 0x17, false)
	x += 3 + len(strconv.Itoa(int(c.HP))) + 2
	e.DisplayString(x, messageRow, colWhite, "AC")
	e.displayAC(c, x+3, 0x17)
	x += 3 + len(strconv.Itoa(abs(int(c.AC[0])-60))) + b2i(c.AC[0] > 60) + 2 // as displayAC writes it
	switch {
	case c.Okay == 0:
		e.DisplayString(x, messageRow, colWhite, cut(e.statusName(c), 0x27-x))
	case e.isHelpless(c):
		e.DisplayString(x, messageRow, colWhite, "(Helpless)")
		x += len("(Helpless)") + 2
	default:
		e.DisplayString(x, messageRow, colWhite, "Dmg")
		e.DisplayString(x+4, messageRow, colLightGreen, cut(damageText(c), 0x27-x-4))
		x += 4 + len(damageText(c)) + 2
	}
	// extra (remaster): a foe a click attacks now, with the chance to hit it
	if h, ok := e.snap.hits[e.combatIndex(c)]; ok {
		cth := strconv.Itoa(h)
		if x+4+len(cth) > lastCol+1 { // a long name: one space before it
			x--
		}
		if x+4+len(cth) <= lastCol+1 {
			e.DisplayString(x, messageRow, colWhite, "CtH")
			e.DisplayString(x+4, messageRow, colLightGreen, cth)
		}
	}
}

// damageText is the damage of the readied attack, as the View page shows it.
func damageText(c *Character) string {
	d := strconv.Itoa(int(c.Dice[0])) + "d" + strconv.Itoa(int(c.Sides[0]))
	if b := int8(c.Bonus[0]); b > 0 {
		d += "+" + strconv.Itoa(int(b))
	} else if b < 0 {
		d += "-" + strconv.Itoa(-int(b)) // abs in 16 bits: -128 shows 128
	}
	return d
}

// combatSay (extra) shows combat text on rows 23-24 under the view and
// keeps it for the Log: fresh starts a new message, otherwise it goes on
// the current one; c (if any) is named first.
func (e *Engine) combatSay(c *Character, msg string, fresh bool) {
	if fresh || len(e.ext.battleLog) == 0 {
		e.ClearCells(0, messageRow, lastCol, promptRow)
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
		if e.msgCol+len(c.Name()) > lastCol {
			e.msgCol, e.msgRow = 0, e.msgRow+1
		}
		e.displayName(c, e.msgCol, e.msgRow, false)
		e.msgCol += len(c.Name()) + 1
		add(c.Name())
	}
	if msg = strings.TrimSpace(msg); msg != "" {
		e.DisplayTextBlock(0, messageRow, lastCol, promptRow, colLightGreen, false, msg)
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
		e.DrawBoxTitled(1, 1, winRight, 23, 0, colWhite, "Battle Log")
		e.ClearCells(2, 2, 37, 23)
		for i := 0; i < rows && top+i < len(lines); i++ {
			e.DisplayString(2, 2+i, colLightGreen, lines[top+i])
		}
		hint := "press a key"
		if len(lines) > rows {
			hint = "up/down: scroll, " + hint
		}
		e.DisplayString(winRight-1-len(hint), messageRow, colLightGreen, hint)
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
	e.DrawBoxTitled(1, 1, winRight, 21, 0, colBlack, "")
	e.ClearCells(0, messageRow, lastCol, promptRow)
	e.combatCentre(e.cmap.viewX+viewW/2, e.cmap.viewY+3, 0xFF, 8)
}

// pathNextTo (extra) is the safest walk in reach that brings c, armed for
// melee, next to foe t: fewest free attacks on the way, then fewest foes
// next to where it ends, then fewest steps (nil: t is in reach of its
// weapon already, c shoots, or no square next to t can be reached).
func (e *Engine) pathNextTo(c, t *Character, reach map[[2]int][]int) []int {
	if e.rangedWeapon(c) || e.rangeTo(c, t) <= 1 {
		return nil
	}
	tx, ty, n := e.combatX(t), e.combatY(t), e.combatSize(t)
	idx := e.combatIndex(c)
	ox, oy, near, targets := e.cpos[idx].x, e.cpos[idx].y, slices.Clone(e.near), e.targets
	defer func() { e.cpos[idx].x, e.cpos[idx].y, e.near, e.targets = ox, oy, near, targets }()
	var best []int
	var bestKey [5]int
	found := false
	for sq, p := range reach {
		if sq[0] < tx-1 || sq[0] > tx+n || sq[1] < ty-1 || sq[1] > ty+n {
			continue
		}
		free := 0
		for _, s := range e.walkSteps(c, p) {
			free += b2i(s.Free)
		}
		e.cpos[idx].x, e.cpos[idx].y = byte(sq[0]), byte(sq[1])
		key := [5]int{free, e.nearEnemies(c, 1), len(p), sq[1], sq[0]} // the square last: map order is random
		e.cpos[idx].x, e.cpos[idx].y = ox, oy
		if !found || slices.Compare(key[:], bestKey[:]) < 0 {
			best, bestKey, found = p, key, true
		}
	}
	if !found {
		return nil
	}
	return append([]int(nil), best...)
}

// foeInSight (extra): is there a foe among the ones c's Aim cycles
// through (playerAim's list) that c may act against? Without one the
// cycle runs over the party, the aimer too.
func (e *Engine) foeInSight(c *Character) bool {
	saved := e.near
	defer func() { e.near = saved }()
	e.buildNear(e.combatX(c), e.combatY(c), 0x7F, 0xFF, e.combatSize(c))
	for _, n := range e.near {
		if f := e.combatant(n.index); f != nil && f.Side != c.Side && e.mayActAgainst(c, f) {
			return true
		}
	}
	return false
}

// throwTargets (extra) are the foes in sight c could throw it at, as Aim's
// cycle would offer them with it readied.
func (e *Engine) throwTargets(c *Character, it *Item) (out []int) {
	w, saved := c.Slots[slotWeapon], e.near
	defer func() { c.Slots[slotWeapon], e.near = w, saved }()
	c.Slots[slotWeapon] = it
	rng := max(int(e.base(it.Type).Range)-1, 1)
	e.buildNear(e.combatX(c), e.combatY(c), 0x7F, 0xFF, e.combatSize(c))
	for _, n := range e.near {
		if f := e.combatant(n.index); f != nil && f.Side != c.Side && f.Okay != 0 && e.mayActAgainst(c, f) && e.canTarget(c, f, rng, 1) {
			out = append(out, int(n.index))
		}
	}
	return out
}

// foeInReach (extra): is there a foe c could attack from where it stands
// with the readied weapon (the ones Aim's Attack takes)?
func (e *Engine) foeInReach(c *Character) bool {
	rng := 1 // as combatMenu's targets
	if w := c.Slots[slotWeapon]; w != nil && e.base(w.Type).Range > 1 {
		rng = int(e.base(w.Type).Range) - 1
	}
	saved := e.near
	defer func() { e.near = saved }()
	e.buildNear(e.combatX(c), e.combatY(c), 0x7F, 0xFF, e.combatSize(c))
	for _, n := range e.near {
		if f := e.combatant(n.index); f != nil && f.Side != c.Side && f.Okay != 0 && e.mayActAgainst(c, f) && e.canTarget(c, f, rng, 1) {
			return true
		}
	}
	return false
}
