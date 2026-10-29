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
	e.snap.cmapOn = false
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
	e.ext.panel = c
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
func (e *Engine) selectChar(c *Character) { e.selectMember(e.partyIndex(c)) }

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// playerTurn = ovr09:031A (char): a spell being cast goes off, otherwise
// the combat menu until the turn is used. extra: the menu is the View's
// Turn; the player acts through Act.
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
	defer func() { e.snap.turn, e.snap.reach, e.snap.paths, e.snap.targets, e.snap.hits = nil, nil, nil, nil, nil }()
	for done := false; !done; {
		reach := e.turnChoices(c)
		a := e.act()
		turn := e.snap.turn
		e.snap.turn = nil
		if !slices.Contains(turn.Can, a.Kind) && a.Kind != ActWalk {
			continue
		}
		switch a.Kind {
		case ActWalk:
			if p, ok := reach[[2]int{a.X, a.Y}]; ok {
				done = e.walk(c, p)
			}
		case ActAttack:
			done = e.attackUnit(c, a.N, reach)
		case ActItems:
			done = e.ItemsMenu() // ovr20:0EF8 (var done)
			e.setSpeed(c)
		case ActRanged:
			done = e.throwMissile(c)
		case ActCast:
			if slices.ContainsFunc(turn.Spells, func(s Spell) bool { return s.ID == a.N }) {
				var ok bool
				e.castSpell(a.N, 0, &ok)
				done = ok
			}
		case ActTurn:
			e.clericTurnUndead(c)
			done = e.clearCombatInfo(c)
		case ActQuick:
			e.quickOne(c)
			e.Delay(200)
			done = true
			e.computerTurn(c)
		case ActGuard:
			done = e.guard(c)
		case ActBandage:
			e.bandage(true)
			done = e.clearCombatInfo(c)
		case ActWait: // Done's Delay
			c.Combat.Delay = 1
			done = true
		case ActEnd: // Done's Quit
			done = e.clearCombatInfo(c)
		case ActFlee: // moveAttack (ovr09:09DF): a step off the map, "Flee:" answered Yes
			if d := e.fleeDir(c); d >= 0 && e.ask("Flee?") {
				e.combatTurn(c, byte(d), 0, false)
				done = e.tryFlee(c)
			}
		}
	}
}

// fleeDir is a direction in which c's next step leaves the map (moveAttack
// offers Flee there), or -1.
func (e *Engine) fleeDir(c *Character) int {
	for d := 0; d < 8; d++ {
		if who, g := e.groundAt(c, d); who == 0 && g == 0 {
			return d
		}
	}
	return -1
}

// turnChoices = ovr09:0666 (combatMenu)'s offer, published as the View's
// Turn with the squares a walk reaches (returned, with their steps) and
// the foes ActAttack takes (in reach of the weapon, or a walk away).
func (e *Engine) turnChoices(c *Character) map[[2]int][]int {
	e.affectGroup(c, 0xF)
	ci := c.Combat
	t := &Turn{Unit: e.combatIndex(c), Moves: int(ci.Moves >> 1)}
	can := func(k ActKind) { t.Can = append(t.Can, k) }
	can(ActAttack)
	if m := e.missile(c); m != nil { // extra: the Missiles slot's thrown weapon
		can(ActRanged)
		t.Missile = e.invItem(c, m)
	}
	if !e.rangedWeapon(c) || e.throwable(c) {
		can(ActGuard) // Done's
	}
	if e.combatItems(c) {
		can(ActItems)
	}
	if ci.CanCast != 0 && e.area.NoMagic == 0 {
		seen := map[byte]bool{}
		for _, v := range c.Memorized[:0x15] {
			if v > 0 && v < 0x80 && !seen[v] && e.spellData(int(v), 5) != 0 { // extra: no camp only spells
				seen[v] = true
				t.Spells = append(t.Spells, Spell{int(v), e.spellName(int(v))})
			}
		}
		if len(t.Spells) > 0 {
			can(ActCast)
		}
	}
	var undead *Character
	if int8(c.Levels[clsCleric]) > 0 && ci.TurnUsed == 0 && e.weakestUnturned(c, &undead) { // extra: only with undead to turn
		can(ActTurn)
	}
	if e.bandage(false) {
		can(ActBandage)
	}
	can(ActWait)
	can(ActEnd)
	can(ActQuick)
	if ci.Moves > 1 && e.fleeDir(c) >= 0 {
		can(ActFlee)
	}
	reach := map[[2]int][]int{}
	if ci.Moves > 0 {
		reach = e.moveReach(c)
		delete(reach, [2]int{e.combatX(c), e.combatY(c)})
	}
	e.snap.reach, e.snap.paths = nil, map[image.Point][]Step{}
	for sq, p := range reach {
		e.snap.reach = append(e.snap.reach, image.Pt(sq[0], sq[1]))
		e.snap.paths[image.Pt(sq[0], sq[1])] = e.walkSteps(c, p)
	}
	e.snap.targets, e.snap.hits = nil, map[int]int{}
	for i := 1; i <= e.ncomb; i++ {
		f := e.combatant(byte(i))
		if f == nil || f.Side == c.Side || f.Okay == 0 || e.cpos[i].size == 0 || !e.mayActAgainst(c, f) {
			continue
		}
		walk := e.pathNextTo(c, f, reach)
		if !e.canTarget(c, f, e.weaponRange(c), 1) && walk == nil {
			continue
		}
		e.snap.targets = append(e.snap.targets, i)
		e.snap.hits[i] = e.hitChance(c, f)
		if walk != nil { // hovering the foe shows the walk up to it
			steps, n := e.walkSteps(c, walk), e.combatSize(f)
			for d := range n * n {
				e.snap.paths[image.Pt(e.combatX(f)+d%n, e.combatY(f)+d/n)] = steps
			}
		}
	}
	e.ext.panel = c
	e.snap.turn = t
	return reach
}

// weaponRange is the squares the readied weapon reaches (1: melee).
func (e *Engine) weaponRange(c *Character) int {
	if w := c.Slots[slotWeapon]; w != nil && e.base(w.Type).Range > 1 {
		return int(e.base(w.Type).Range) - 1
	}
	return 1
}

// attackUnit (extra): ActAttack. Unit 0 aims first (combatMenu's Aim);
// a foe of the turn's targets is attacked, a melee fighter walking up to
// it first (pathNextTo) when it stands out of reach.
func (e *Engine) attackUnit(c *Character, n int, reach map[[2]int][]int) bool {
	var aim aimTarget
	var ok bool
	if n == 0 {
		if !e.foeInReach(c) { // extra: no aiming with nothing to attack
			e.refuse("No foe in reach")
			return false
		}
		e.ext.enemiesOnly = true
		e.playerAim(c, 0xFF, 1, 0, &ok, &aim)
		e.ext.enemiesOnly = false
		return ok
	}
	if !slices.Contains(e.snap.targets, n) {
		return false
	}
	t := e.combatant(byte(n))
	if p := e.pathNextTo(c, t, reach); p != nil {
		if e.walk(c, p) {
			return true
		}
		if t.Okay == 0 || e.rangeTo(c, t) > 1 { // stopped short: the turn goes on
			return false
		}
	}
	e.aimConfirm(c, t, 1, &ok, &aim)
	return ok
}

// walk (extra) takes c along path as moveAttack's keypad steps would, one
// step a message delay apart, while the next square is free. done: c is
// out (fallen, helpless).
func (e *Engine) walk(c *Character, path []int) (done bool) {
	ci := c.Combat
	for _, d := range path {
		if ci.Moves <= 1 || done {
			break
		}
		if who, g := e.groundAt(c, d); who > 0 || g == 0 || e.groundInfo(g)[0] > ci.Moves {
			break
		}
		e.Delay(0x32)
		e.combatTurn(c, byte(d), 0, false)
		e.freeAttacksOnLeave(c, d)
		if c.Okay == 0 {
			return e.clearCombatInfo(c)
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
	if ci.Moves < 2 {
		ci.Moves = 0
	}
	return
}

// combatSay (extra) keeps combat text for the View's Log: fresh starts a
// new message, otherwise it goes on the current one; c (if any) is named first.
func (e *Engine) combatSay(c *Character, msg string, fresh bool) {
	if fresh || len(e.ext.battleLog) == 0 {
		e.ext.battleLog = append(e.ext.battleLog, "")
	}
	l := &e.ext.battleLog[len(e.ext.battleLog)-1]
	for _, s := range []string{name(c), strings.TrimSpace(msg)} {
		if s != "" && *l != "" {
			*l += " "
		}
		*l += s
	}
}

func name(c *Character) string {
	if c == nil {
		return ""
	}
	return c.Name()
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

// throwMissile (extra): ActRanged takes the Missiles slot's weapon in hand
// in place of the weapon, aims as Aim does, then readies the weapon again.
// Nothing changes when the swap fails or the aim is given up.
func (e *Engine) throwMissile(c *Character) (done bool) {
	it, w := e.missile(c), c.Slots[slotWeapon]
	if it == nil {
		return false
	}
	if len(e.throwTargets(c, it)) == 0 { // as Attack's "No foe in reach"
		e.refuse("No target in range")
		return false
	}
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
	it.Readied = 1 // in hand (the weapon's hands are free)
	e.ApplyItems(c)
	var aim aimTarget
	e.ext.enemiesOnly = true
	e.playerAim(c, 0xFF, 1, 0, &done, &aim)
	e.ext.enemiesOnly = false
	if slices.Contains(c.Items, it) { // not the last one thrown
		it.Readied = 2
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
			if f&1 != 0 && c.Slots[slotArrows] != nil && c.Slots[slotArrows].Type == itemArrow { // not a missile in the slot
				*it = c.Slots[slotArrows]
			}
			if f&0x80 != 0 {
				*it = c.Slots[slotBolts]
			}
		}
	}
	return *it != nil || f == 0xA
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

// quickOne = ovr09:122B (char): the computer fights for it; a target on
// its own side is dropped.
func (e *Engine) quickOne(c *Character) {
	c.Auto = 1
	if t := c.Combat.Target; t != nil && t.Side == c.Side {
		c.Combat.Target = nil
	}
}

// combatItems (extra): c carries an item its items menu could Use (when
// it may use items this round).
func (e *Engine) combatItems(c *Character) bool {
	for _, it := range e.carried(c) {
		if c.Combat.CanUse != 0 && (e.isScroll(it) || it.Spells[1] > 0 && it.Spells[2] < 0x80) {
			return true
		}
	}
	return false
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

// combatUseHook = 00BA:2B88: back to the combat map: its palette, the map
// centred on the view (extra: no frame drawn: views draw their own).
func (e *Engine) combatUseHook() {
	e.combatViewLeave() // ovr32:00E3: the combat palette
	e.combatCentre(e.cmap.viewX+viewW/2, e.cmap.viewY+3, 0xFF, 8)
}
