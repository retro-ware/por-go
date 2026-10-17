// Package party holds the adventuring party: characters, shared resources, flags.
// It implements the stat side of events.Host, so scripts can read and change it by property name.
package party

import (
	"fmt"
	"sort"
	"strings"
)

// Stats and resistances addressable by name; each also has a "<name>tmp" bonus.
var Stats = []string{"might", "int", "pers", "end", "spd", "acc", "luck"}
var Resists = []string{"fire", "elec", "cold", "poison", "energy", "magic"}

// Conditions a character can have ("condition:<name>" in Has). poisoned loses 1 HP a turn,
// asleep and paralyzed cannot act; cure, the temple and rest clear them. dead is special.
// ponytail: no diseased/stone/eradicated; add names here and effects in Tick when needed.
var Conditions = []string{"poisoned", "asleep", "paralyzed", "dead"}

// CanAct is false for the dead, sleeping and paralyzed.
func (c *Character) CanAct() bool {
	return c.HP > 0 && !c.Has["condition:dead"] && !c.Has["condition:asleep"] && !c.Has["condition:paralyzed"]
}

// Cure clears every condition but death.
func (c *Character) Cure() {
	for _, k := range Conditions {
		if k != "dead" {
			c.Has["condition:"+k] = false
		}
	}
}

// Tick applies one turn of conditions: poison bites, sleep wears off by chance.
func (p *Party) Tick() {
	for i, c := range p.Chars {
		if c.HP <= 0 {
			continue
		}
		if c.Has["condition:poisoned"] {
			p.Damage(i+1, 1, "poison")
		}
		if c.Has["condition:asleep"] && p.Minutes%10 == 0 { // ponytail: wakes within ten turns; a timer per condition when it matters
			c.Has["condition:asleep"] = false
		}
	}
}

type Character struct {
	Name, Sex, Race, Class string
	Portrait               string // id in the portrait pool; "" picks one by class and sex
	HP, MaxHP, SP, MaxSP   int
	AC, Level, Age, Exp    int
	Num                    map[string]int  // stats, resistances and their tmp bonuses, actmp, leveltmp
	Has                    map[string]bool // "award:x", "skill:x", "condition:x", "spell:x"
	Equipped               map[Slot]*Item  `json:"-"` // rebuilt by Relink after a load
}

func NewCharacter(name, class string, hp, sp int) *Character {
	c := &Character{Name: name, Class: class, Sex: "m", Race: "human", HP: hp, MaxHP: hp, SP: sp, MaxSP: sp,
		AC: 5, Level: 1, Age: 20, Num: map[string]int{}, Has: map[string]bool{}, Equipped: map[Slot]*Item{}}
	for _, s := range Stats {
		c.Num[s] = 10
	}
	return c
}

type Party struct {
	Chars            []*Character
	Roster           []*Character // waiting at the inn
	Inventory        [][]*Item    // shared, stacked; equipped items included
	Gold, Gems, Food int
	Bank             int // gold deposited in town
	Minutes          int // since midnight
	Day, Year        int
	Flags            map[string]bool
	OnHit            func(who, n int, kind string) `json:"-"` // after damage: 1-based character, points taken, kind
	OnCast           func(Spell)                   `json:"-"` // after a spell is cast, for its effect in the world
}

func New(chars ...*Character) *Party {
	return &Party{Chars: chars, Flags: map[string]bool{}, Day: 1, Year: 1, Minutes: 8 * 60}
}

func (p *Party) PartySize() int { return len(p.Chars) }

// char returns slot n (1-based); 0 and out of range give the first character.
func (p *Party) char(n int) *Character {
	if n >= 1 && n <= len(p.Chars) {
		return p.Chars[n-1]
	}
	return p.Chars[0]
}

// targets is the set of characters an action with who applies to (0 = everyone).
func (p *Party) targets(who int) []*Character {
	if who == 0 {
		return p.Chars
	}
	return []*Character{p.char(who)}
}

func known(prop string) bool {
	for _, s := range append(append([]string{}, Stats...), Resists...) {
		if prop == s || prop == s+"tmp" {
			return true
		}
	}
	return prop == "actmp" || prop == "leveltmp"
}

// setProp parses "award:x" style keys for the Has map.
func hasKey(prop, arg string) (string, bool) {
	switch prop {
	case "award", "skill", "condition", "spell":
		return prop + ":" + arg, true
	}
	return "", false
}

// ---------- events.Host stat methods ----------

func (p *Party) Get(who int, prop, arg string) int {
	c := p.char(who)
	switch prop {
	case "flag":
		return b2i(p.Flags[arg])
	case "item":
		return b2i(p.FindItem(arg) != nil)
	case "key":
		return b2i(p.FindItem(arg+" key") != nil)
	case "hpfull":
		return b2i(c.HP >= c.MaxHP)
	case "spfull":
		return b2i(c.SP >= c.MaxSP)
	case "sex", "race", "class":
		return b2i(strings.EqualFold(map[string]string{"sex": c.Sex, "race": c.Race, "class": c.Class}[prop], arg))
	case "inparty":
		for _, t := range p.Chars {
			if strings.EqualFold(t.Name, arg) {
				return 1
			}
		}
		return 0
	}
	if k, ok := hasKey(prop, arg); ok {
		return b2i(c.Has[k])
	}
	if v, ok := p.get(c, prop); ok {
		return v
	}
	return 0
}

func (p *Party) get(c *Character, prop string) (int, bool) {
	if ptr := p.fieldPtr(c, prop); ptr != nil {
		return *ptr, true
	}
	if known(prop) {
		return c.Num[prop], true
	}
	return 0, false
}

func (p *Party) fieldPtr(c *Character, prop string) *int {
	switch prop {
	case "gold":
		return &p.Gold
	case "gems":
		return &p.Gems
	case "food":
		return &p.Food
	case "minutes":
		return &p.Minutes
	case "day":
		return &p.Day
	case "year":
		return &p.Year
	case "hp":
		return &c.HP
	case "sp":
		return &c.SP
	case "maxhp":
		return &c.MaxHP
	case "maxsp":
		return &c.MaxSP
	case "ac":
		return &c.AC
	case "level":
		return &c.Level
	case "age":
		return &c.Age
	case "exp":
		return &c.Exp
	}
	return nil
}

func (p *Party) set(c *Character, prop string, v int) bool {
	if ptr := p.fieldPtr(c, prop); ptr != nil {
		*ptr = v
		return true
	}
	if known(prop) {
		c.Num[prop] = v
		return true
	}
	return false
}

func (p *Party) Set(who int, prop, arg string, n int) {
	for _, c := range p.targets(who) {
		switch prop {
		case "flag":
			p.Flags[arg] = n != 0
		case "hpfull":
			c.HP = c.MaxHP
		case "spfull":
			c.SP = c.MaxSP
		case "sex":
			c.Sex = arg
		case "race":
			c.Race = arg
		case "class":
			c.Class = arg
		case "portrait":
			c.Portrait = arg
		default:
			if k, ok := hasKey(prop, arg); ok {
				c.Has[k] = n != 0
			} else if !p.set(c, prop, n) {
				fmt.Printf("party: unknown property %q\n", prop)
			}
		}
		if partyWide(prop) {
			break
		}
	}
}

func (p *Party) Give(who int, prop, arg string, n int) {
	switch prop {
	case "item":
		for range n {
			p.AddItem(NewItem(arg, 0))
		}
		return
	case "key":
		p.AddItem(NewKey(arg))
		return
	}
	for _, c := range p.targets(who) {
		switch prop {
		case "flag", "hpfull", "spfull", "sex", "race", "class", "portrait":
			p.Set(who, prop, arg, n)
			return
		default:
			if k, ok := hasKey(prop, arg); ok {
				c.Has[k] = true
			} else if v, ok := p.get(c, prop); ok {
				p.set(c, prop, v+n)
			} else {
				fmt.Printf("party: unknown property %q\n", prop)
			}
		}
		if partyWide(prop) {
			break
		}
	}
	p.clamp()
}

// Take removes; it reports false if the party does not have enough / the item / the award.
func (p *Party) Take(who int, prop, arg string, n int) bool {
	ts := p.targets(who)
	switch prop {
	case "flag":
		ok := p.Flags[arg]
		p.Flags[arg] = false
		return ok
	case "item", "key":
		if prop == "key" {
			arg += " key"
		}
		for range n {
			it := p.FindItem(arg)
			if it == nil {
				return false
			}
			p.RemoveItem(it)
		}
		return true
	}
	if k, ok := hasKey(prop, arg); ok {
		had := false
		for _, c := range ts {
			had = had || c.Has[k]
			c.Has[k] = false
		}
		return had
	}
	ok := true
	for _, c := range ts {
		v, found := p.get(c, prop)
		if !found {
			fmt.Printf("party: unknown property %q\n", prop)
			return false
		}
		if v < n {
			ok = false
		}
		p.set(c, prop, max(0, v-n))
		if partyWide(prop) {
			break
		}
	}
	return ok
}

// Damage applies n points of kind to who; elemental kinds are reduced by the matching resistance,
// "dead" kills outright. HP may go negative; at or below 0 the character is dead.
func (p *Party) Damage(who, n int, kind string) {
	for _, c := range p.targets(who) {
		if kind == "dead" {
			c.HP = 0
			c.Has["condition:dead"] = true
			p.hit(c, 0, kind)
			continue
		}
		dmg := n
		for _, r := range Resists {
			if kind == r {
				dmg = max(0, n-c.Num[r]-c.Num[r+"tmp"])
			}
		}
		c.HP -= dmg
		if c.HP <= 0 {
			c.Has["condition:dead"] = true
		}
		p.hit(c, dmg, kind)
	}
}

func (p *Party) hit(c *Character, n int, kind string) {
	if p.OnHit == nil {
		return
	}
	for i, t := range p.Chars {
		if t == c {
			p.OnHit(i+1, n, kind)
		}
	}
}

func partyWide(prop string) bool {
	switch prop {
	case "gold", "gems", "food", "minutes", "day", "year", "flag":
		return true
	}
	return false
}

func (p *Party) clamp() {
	for _, c := range p.Chars {
		c.HP = min(c.HP, c.MaxHP)
		c.SP = min(c.SP, c.MaxSP)
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Summary renders the party for a debug view.
func (p *Party) Summary() []string {
	out := []string{fmt.Sprintf("gold %d  gems %d  food %d  day %d  %02d:%02d", p.Gold, p.Gems, p.Food, p.Day, p.Minutes/60, p.Minutes%60)}
	for i, c := range p.Chars {
		out = append(out, fmt.Sprintf("%d %-8s %-8s L%d  HP %d/%d  SP %d/%d  AC %d", i+1, c.Name, c.Class, c.Level, c.HP, c.MaxHP, c.SP, c.MaxSP, c.AC))
		s := "   "
		for _, st := range Stats {
			s += fmt.Sprintf("%s %d ", st, c.Num[st]+c.Num[st+"tmp"])
		}
		out = append(out, s)
		var has []string
		for k, v := range c.Has {
			if v {
				has = append(has, k)
			}
		}
		sort.Strings(has)
		for _, sl := range Slots {
			if it := c.Equipped[sl]; it != nil {
				has = append(has, string(sl)+"="+it.Name())
			}
		}
		if len(has) > 0 {
			out = append(out, "   "+strings.Join(has, " "))
		}
	}
	var inv []string
	for _, st := range p.Inventory {
		inv = append(inv, fmt.Sprintf("%s x%d", st[0].Name(), len(st)))
	}
	if len(inv) > 0 {
		out = append(out, "inventory: "+strings.Join(inv, ", "))
	}
	var flags []string
	for k, v := range p.Flags {
		if v {
			flags = append(flags, k)
		}
	}
	sort.Strings(flags)
	if len(flags) > 0 {
		out = append(out, "flags: "+strings.Join(flags, " "))
	}
	return out
}
