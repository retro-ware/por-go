package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"math/rand"
	"slices"
	"sort"
	"strconv"
	"strings"

	"blobber"
	"blobber/party"
	"blobber/sprites"
	"blobber/ui"

	"github.com/hajimehoshi/ebiten/v2"
)

// kind is one row of maps/monsters.txt.
type kind struct {
	Sprite           string
	HP, AC, Dmg, Exp int
	Spd              int
	Size             float64 // tiles tall; 0 = the engine's 0.6
}

var kinds = map[string]kind{}

// loadMonsters reads "id sprite hp ac dmg exp [spd [size]]" lines (spd defaults to 10); # starts a comment.
func loadMonsters(r io.Reader) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		f := strings.Fields(strings.SplitN(sc.Text(), "#", 2)[0])
		if len(f) == 0 {
			continue
		}
		if len(f) < 6 || len(f) > 8 {
			return fmt.Errorf("monsters.txt: %q: want id sprite hp ac dmg exp [spd [size]]", sc.Text())
		}
		n := [5]int{4: 10}
		for i := range f[2:min(len(f), 7)] {
			n[i], _ = strconv.Atoi(f[2+i])
		}
		k := kind{f[1], n[0], n[1], n[2], n[3], n[4], 0}
		if len(f) == 8 {
			k.Size, _ = strconv.ParseFloat(f[7], 64)
		}
		kinds[f[0]] = k
	}
	return sc.Err()
}

// kindOf looks a monster up; unknown ids are drawn with a sprite of their own name and get stock stats.
func kindOf(id string) kind {
	if k, ok := kinds[id]; ok {
		return k
	}
	return kind{id, 10, 3, 3, 5, 10, 0}
}

// spawn places monster id at (x, y) with hp hit points (hp < 0 = full).
func (c *console) spawn(id string, x, y, hp int) error {
	k := kindOf(id)
	if hp < 0 {
		hp = k.HP
	}
	s, ok := sheets[k.Sprite]
	if !ok {
		if img := sprites.Get(k.Sprite); img != nil {
			s = &blobber.Sheet{Idle: ebiten.NewImageFromImage(img)}
		} else if s, _ = loadSheet(k.Sprite); s == nil {
			img, err := loadMonster(k.Sprite)
			if err != nil {
				fmt.Println("  spawn failed:", err)
				return err
			}
			s = &blobber.Sheet{Idle: ebiten.NewImageFromImage(img)}
		}
		sheets[k.Sprite] = s
	}
	key := fmt.Sprintf("%s@%g", k.Sprite, k.Size) // ids sharing a sprite may differ in size
	sized, ok := scaled[key]
	if !ok {
		sized = s.Scaled(k.Size)
		scaled[key] = sized
	}
	blobber.SpawnSheet(id, sized, x, y, hp)
	return nil
}

// sheets caches loaded monster pictures by sprite name, scaled the resampled ones by sprite@size.
var sheets, scaled = map[string]*blobber.Sheet{}, map[string]*blobber.Sheet{}

// ---------- turn-based rules ----------
// ponytail: d20 rolls against AC, melee only (monsters sharing the party's cell), everyone strikes the
// first monster in the cell, no criticals, no spells in a round. Target choice when groups get mixed.

var roll = rand.Intn

// god is the debug mode (G): monster hits do nothing and the party is invisible; damage dealt is normal.
var god bool

func toggleGod() {
	god = !god
	blobber.Invisible = god
	if god {
		blobber.Message("God mode on: invisible and unhurt.")
	} else {
		blobber.Message("God mode off.")
	}
}

// melee plays one combat round: everyone in the fight acts once, fastest first; each character's
// action is asked for on their turn. When the round is over a turn passes (EndTurn), which starts the
// next round while monsters remain.
func (c *console) melee() {
	type turn struct {
		ch  *party.Character
		mon string
		spd int
	}
	var order []turn
	for _, ch := range c.Chars {
		order = append(order, turn{ch: ch, spd: ch.Num["spd"] + ch.Num["spdtmp"]})
	}
	for _, m := range blobber.Melee() {
		order = append(order, turn{mon: m.Name, spd: kindOf(m.Name).Spd})
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i].spd > order[j].spd })
	clear(blocking)
	var next func(i int)
	next = func(i int) {
		ui.Active = nil // monsters' turns: nobody's frame is lit
		for ; i < len(order) && !c.Wiped(); i++ {
			ms := blobber.Melee()
			if len(ms) == 0 {
				break
			}
			if t := order[i]; t.ch == nil {
				if slices.ContainsFunc(ms, func(m blobber.Monster) bool { return m.Name == t.mon }) {
					c.defend(blobber.Monster{Name: t.mon})
				}
				continue
			}
			if ch := order[i].ch; ch.CanAct() {
				c.choose(ch, func() { next(i + 1) })
				return
			}
		}
		if len(blobber.Melee()) == 0 || c.Wiped() {
			autoFight = false
		}
		if !c.Wiped() {
			blobber.EndTurn()
		}
	}
	next(0)
}

// blocking holds the characters that chose Block this round: monsters hit them less often.
var blocking = map[*party.Character]bool{}

// autoFight is set by Auto: every character attacks the first monster without asking until the
// fight is over.
var autoFight bool

// autoPace is the seconds between auto combat turns; tests set 0.
var autoPace = 0.7

// choose asks ch for a round action and calls done once it is taken (flee ends the round itself).
func (c *console) choose(ch *party.Character, done func()) {
	again := func() { c.choose(ch, done) }
	ui.Active = ch
	if autoFight { // paced, so each swing and the monsters' answers play out on screen
		blobber.After(autoPace, func() {
			if ms := blobber.Melee(); len(ms) > 0 {
				c.swing(ch, 0, ms[0])
			}
			done()
		})
		return
	}
	blobber.ShowPrompt([]string{ch.Name + "'s turn:", "  1  Attack", "  2  Block", "  3  Cast", "  4  Flee", "  5  Auto"}, false, func(a string) {
		switch a {
		case "5":
			autoFight = true
			blobber.Message("Auto combat: everyone attacks until the fight is over")
			again()
		case "1":
			c.target(again, func(i int, m blobber.Monster) { c.swing(ch, i, m); done() })
		case "2":
			blocking[ch] = true
			blobber.Message(ch.Name + " blocks")
			done()
		case "3":
			c.castIn(ch, again, done)
		case "4":
			c.flee()
		default:
			again()
		}
	})
}

// target picks a monster in the party's cell (asking only when there are several); Esc calls back.
func (c *console) target(back func(), on func(int, blobber.Monster)) {
	ms := blobber.Melee()
	if len(ms) == 1 {
		on(0, ms[0])
		return
	}
	lines := []string{"Which one?"}
	for i, m := range ms[:min(9, len(ms))] {
		lines = append(lines, fmt.Sprintf("  %d  %s (%d HP)", i+1, m.Name, m.HP))
	}
	blobber.ShowPrompt(lines, false, func(a string) {
		if i := digit(a) - 1; i >= 0 && i < len(lines)-1 {
			on(i, ms[i])
			return
		}
		back()
	})
}

// castIn lets ch cast a known spell in melee; bolts strike a chosen monster in the cell.
func (c *console) castIn(ch *party.Character, back, done func()) {
	var known []party.Spell
	lines := []string{ch.Name + " casts:"}
	for _, s := range party.Spells {
		if ch.Knows(s.ID) && len(known) < 9 {
			known = append(known, s)
			lines = append(lines, fmt.Sprintf("  %d  %s (%d SP)", len(known), s.Name, s.SP))
		}
	}
	if len(known) == 0 {
		blobber.Message(ch.Name + " knows no spells")
		back()
		return
	}
	blobber.ShowPrompt(lines, false, func(a string) {
		i := digit(a) - 1
		if i < 0 || i >= len(known) {
			back()
			return
		}
		s := known[i]
		cast := func(t int) {
			msg, err := c.Cast(ch, s.ID)
			if err != nil {
				blobber.Message(err.Error())
				back()
				return
			}
			blobber.Message(msg)
			if t >= 0 {
				blobber.HitMelee(t, s.Power)
			}
			done()
		}
		if s.Kind == "bolt" || s.Kind == "fire" {
			c.target(back, func(t int, _ blobber.Monster) { cast(t) })
			return
		}
		cast(-1)
	})
}

// flee leaves the fight for a spot no monster can see; the unconscious are left behind, dead.
func (c *console) flee() {
	if !blobber.Flee() {
		blobber.Message("There is nowhere to run!")
		blobber.EndTurn()
		return
	}
	for _, ch := range c.Chars {
		if ch.HP <= 0 {
			ch.Has["condition:dead"] = true
		}
	}
	blobber.Message("You flee!")
	blobber.EndTurn()
}

// swing is ch striking m, the i-th monster in melee, once.
func (c *console) swing(ch *party.Character, i int, m blobber.Monster) {
	if roll(20)+1+ch.Level+ch.Num["acc"]/5 <= kindOf(m.Name).AC+10 {
		blobber.Message(ch.Name + " misses the " + m.Name)
		return
	}
	dmg := 2 // fists
	if w := ch.Equipped[party.RightHand]; w != nil {
		dmg = w.Damage + w.Bonus
	}
	dmg += ch.Num["might"] / 10
	blobber.Message(fmt.Sprintf("%s hits the %s for %d", ch.Name, m.Name, dmg))
	c.PlaySound("hit")
	ui.Bleed(-1, dmg)
	blobber.HitMelee(i, dmg)
}

// defend is monster m striking a random living character.
func (c *console) defend(m blobber.Monster) {
	blobber.Strike(m.Name)
	if god {
		return
	}
	var alive []int
	for i, ch := range c.Chars {
		if ch.HP > 0 {
			alive = append(alive, i+1)
		}
	}
	if len(alive) == 0 {
		return
	}
	who := alive[roll(len(alive))]
	ch := c.Chars[who-1]
	ac := ch.AC + 5
	if blocking[ch] {
		ac += 4
	}
	if roll(20)+1 <= ac {
		blobber.Message("The " + m.Name + " misses " + ch.Name)
		return
	}
	dmg := roll(kindOf(m.Name).Dmg) + 1
	blobber.Message(fmt.Sprintf("The %s hits %s for %d", m.Name, ch.Name, dmg))
	c.Damage(who, dmg, "physical")
	if ch.HP <= 0 {
		blobber.Message(ch.Name + " falls!")
	}
}

// killed awards the monster's experience to the living.
func (c *console) killed(m blobber.Monster) {
	exp := kindOf(m.Name).Exp
	for _, ch := range c.Chars {
		if ch.HP > 0 {
			ch.Exp += exp
		}
	}
	gold := roll(exp + 1) // ponytail: gold only, half the exp on average; a loot column when items should drop
	c.Gold += gold
	blobber.Message(fmt.Sprintf("The %s dies. +%d exp, %d gold", m.Name, exp, gold))
}

// tick is one party turn: a minute passes and conditions bite.
func (c *console) tick() {
	if c.Minutes++; c.Minutes >= 24*60 {
		c.Minutes, c.Day = 0, c.Day+1
	}
	c.Tick()
}

// Wiped reports whether nobody is left standing.
func (c *console) Wiped() bool {
	for _, ch := range c.Chars {
		if ch.HP > 0 {
			return false
		}
	}
	return true
}

// installCombat wires the engine's turn hooks to the rules above.
func (c *console) installCombat() {
	blobber.OnMelee = c.melee
	blobber.OnKill = c.killed
	blobber.OnTurn = c.tick
}

// step is the cell offset of each facing (0=N 1=E 2=S 3=W).
var step = [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

// killAhead is a debug key (K): every monster in the party's cell and in the cells straight ahead,
// up to the next wall, dies at once (with the usual experience and loot).
func killAhead() {
	x, y, dir := blobber.Pos()
	n := 0
	for {
		for blobber.HitMonster(x, y, 1<<30) {
			n++
		}
		x, y = x+step[dir][0], y+step[dir][1]
		if c := blobber.Cell(x, y); c != '.' && c != blobber.Door {
			break
		}
	}
	blobber.Message(fmt.Sprintf("Debug: killed %d monster(s).", n))
}

// complete reports whether monster id has usable stats and all three painted poses.
func complete(id string) bool {
	k := kinds[id]
	if k.HP <= 0 || k.AC < 0 || k.Dmg <= 0 || k.Exp < 0 || k.Spd <= 0 || k.Size <= 0 {
		return false
	}
	for _, pose := range []string{"idle", "attack", "hurt"} {
		if _, err := fs.Stat(monsters, "assets/monsters/"+k.Sprite+"/"+pose+".png"); err != nil {
			return false
		}
	}
	return true
}

// spawnMenu is a debug key (N): pick a complete monster of monsters.txt and it appears
// 3 cells ahead, or at the farthest open cell before a wall.
func (c *console) spawnMenu() {
	ids := slices.DeleteFunc(slices.Sorted(maps.Keys(kinds)), func(id string) bool { return !complete(id) })
	lines := []string{"SPAWN MONSTER", ""}
	for i, id := range ids {
		lines = append(lines, fmt.Sprintf("  %2d  %s", i+1, id))
	}
	blobber.ShowPrompt(append(lines, "", "Enter spawns, Esc cancels"), false, func(a string) {
		i, err := strconv.Atoi(strings.TrimSpace(a))
		if err != nil || i < 1 || i > len(ids) {
			return
		}
		x, y, dir := blobber.Pos()
		for range 3 {
			nx, ny := x+step[dir][0], y+step[dir][1]
			if c := blobber.Cell(nx, ny); c != '.' && c != blobber.Door {
				break
			}
			x, y = nx, ny
		}
		if c.spawn(ids[i-1], x, y, -1) == nil {
			blobber.Message(fmt.Sprintf("Debug: spawned %s at %d,%d.", ids[i-1], x, y))
		}
	})
}
