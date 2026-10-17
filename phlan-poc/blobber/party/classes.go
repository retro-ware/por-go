package party

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Class is one row of classes.txt: HP and SP gained per level, and the base level cost.
type Class struct{ HP, SP, Exp int }

var (
	Classes = map[string]Class{}
	Races   = map[string]map[string]int{} // stat modifiers at creation
)

func rows(r io.Reader, n int, what string, f func(id string, v []int)) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fs := strings.Fields(strings.SplitN(sc.Text(), "#", 2)[0])
		if len(fs) == 0 {
			continue
		}
		if len(fs) != n+1 {
			return fmt.Errorf("%s: %q: want id and %d numbers", what, sc.Text(), n)
		}
		v := make([]int, n)
		for i := range v {
			v[i], _ = strconv.Atoi(fs[i+1])
		}
		f(strings.ToLower(fs[0]), v)
	}
	return sc.Err()
}

// LoadClasses reads "id hp sp exp" lines.
func LoadClasses(r io.Reader) error {
	return rows(r, 3, "classes.txt", func(id string, v []int) { Classes[id] = Class{v[0], v[1], v[2]} })
}

// LoadRaces reads "id might int pers end spd acc luck" lines.
func LoadRaces(r io.Reader) error {
	return rows(r, len(Stats), "races.txt", func(id string, v []int) {
		m := map[string]int{}
		for i, s := range Stats {
			m[s] = v[i]
		}
		Races[id] = m
	})
}

// Names lists a table's ids in order, for menus.
func Names[T any](m map[string]T) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ClassOf is a character's class row; unknown classes get a plain fighter row.
func ClassOf(c *Character) Class {
	if k, ok := Classes[strings.ToLower(c.Class)]; ok {
		return k
	}
	return Class{8, 0, 100}
}

// Create rolls a level-1 character: stats 8 + roll(8) plus the race's modifiers,
// HP and SP from the class, with endurance adding to HP.
func Create(name, sex, race, class string, roll func(int) int) *Character {
	c := NewCharacter(name, class, 0, 0)
	c.Sex, c.Race = sex, race
	for _, s := range Stats {
		c.Num[s] = 8 + roll(8) + Races[race][s]
	}
	k := ClassOf(c)
	c.MaxHP, c.MaxSP = k.HP+hpBonus(c), k.SP
	c.HP, c.SP = c.MaxHP, c.MaxSP
	return c
}

func hpBonus(c *Character) int { return (c.Num["end"] - 10) / 4 }

// ExpToLevel is the experience needed for the next level.
func (c *Character) ExpToLevel() int { return ClassOf(c).Exp * c.Level }

// LevelUp raises c one level and refills HP and SP.
func (c *Character) LevelUp() {
	k := ClassOf(c)
	c.Level++
	c.MaxHP += max(1, k.HP+hpBonus(c))
	c.MaxSP += k.SP
	c.HP, c.SP = c.MaxHP, c.MaxSP
}

// ---------- inn ----------

// Dismiss moves party member i (0-based) to the roster, taking their gear off. The last member stays.
func (p *Party) Dismiss(i int) bool {
	if len(p.Chars) <= 1 || i < 0 || i >= len(p.Chars) {
		return false
	}
	c := p.Chars[i]
	for _, it := range c.Equipped {
		p.Unequip(it)
	}
	p.Chars = append(p.Chars[:i], p.Chars[i+1:]...)
	p.Roster = append(p.Roster, c)
	return true
}

// Hire moves roster member i into the party, if there is room.
func (p *Party) Hire(i int) bool {
	if len(p.Chars) >= MaxParty || i < 0 || i >= len(p.Roster) {
		return false
	}
	p.Chars = append(p.Chars, p.Roster[i])
	p.Roster = append(p.Roster[:i], p.Roster[i+1:]...)
	return true
}

// MaxParty is how many characters adventure at once.
const MaxParty = 6
