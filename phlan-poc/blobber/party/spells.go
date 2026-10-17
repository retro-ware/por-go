package party

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Spell is one entry in the spell list. Knowing it is c.Has["spell:<ID>"], as scripts already address it.
type Spell struct {
	ID, Kind, Name, Desc string // Kind: heal, healall, cure, light, bolt, fire, sleep
	SP, Price            int
	Power                int // HP healed or damage dealt
}

// Spells is the full list, in the order the guild sells them; maps/spells.txt replaces it.
var Spells = []Spell{
	{"light", "light", "Light", "brighter torch for a minute", 1, 25, 0},
	{"heal", "heal", "Heal", "heals the most wounded 15 HP", 3, 60, 15},
	{"lightning", "bolt", "Lightning", "fast bolt, 8 damage", 4, 120, 8},
	{"fireball", "fire", "Fireball", "slow bolt, 14 damage", 6, 200, 14},
}

// LoadSpells reads "id kind sp price power | Name | description" lines.
func LoadSpells(r io.Reader) error {
	var out []Spell
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.SplitN(sc.Text(), "#", 2)[0]
		if strings.TrimSpace(line) == "" {
			continue
		}
		cols := strings.Split(line, "|")
		fs := strings.Fields(cols[0])
		if len(cols) != 3 || len(fs) != 5 {
			return fmt.Errorf("spells.txt: %q: want id kind sp price power | name | desc", sc.Text())
		}
		sp, _ := strconv.Atoi(fs[2])
		price, _ := strconv.Atoi(fs[3])
		power, _ := strconv.Atoi(fs[4])
		out = append(out, Spell{fs[0], fs[1], strings.TrimSpace(cols[1]), strings.TrimSpace(cols[2]), sp, price, power})
	}
	if len(out) > 0 {
		Spells = out
	}
	return sc.Err()
}

func SpellByID(id string) (Spell, bool) {
	for _, s := range Spells {
		if s.ID == id {
			return s, true
		}
	}
	return Spell{}, false
}

func (c *Character) Knows(id string) bool { return c.Has["spell:"+id] }

func (c *Character) CanLearn() bool { return c.MaxSP > 0 || ClassOf(c).SP > 0 }

// Learn teaches c spell id (the guild has already been paid).
func (c *Character) Learn(id string) error {
	switch _, ok := SpellByID(id); {
	case !ok:
		return fmt.Errorf("no spell %q", id)
	case !c.CanLearn():
		return fmt.Errorf("%s cannot learn spells", c.Name)
	case c.Knows(id):
		return fmt.Errorf("%s already knows it", c.Name)
	}
	c.Has["spell:"+id] = true
	return nil
}

// Cast spends c's SP on spell id and applies what the party itself can: heal.
// Light and the bolts are left to OnCast, which the game wires to the engine.
func (p *Party) Cast(c *Character, id string) (string, error) {
	s, ok := SpellByID(id)
	switch {
	case !ok || !c.Knows(id):
		return "", fmt.Errorf("%s does not know that spell", c.Name)
	case !c.CanAct():
		return "", fmt.Errorf("%s cannot cast", c.Name)
	case c.SP < s.SP:
		return "", fmt.Errorf("not enough spell points")
	}
	c.SP -= s.SP
	msg := fmt.Sprintf("%s casts %s", c.Name, s.Name)
	switch s.Kind {
	case "heal": // ponytail: auto-targets the most wounded; add a target picker when it matters
		var t *Character
		for _, x := range p.Chars {
			if x.HP > 0 && (t == nil || x.MaxHP-x.HP > t.MaxHP-t.HP) {
				t = x
			}
		}
		before := t.HP
		t.HP = min(t.MaxHP, t.HP+s.Power)
		msg += fmt.Sprintf(": %s +%d HP", t.Name, t.HP-before)
	case "healall":
		for _, x := range p.Chars {
			if x.HP > 0 {
				x.HP = min(x.MaxHP, x.HP+s.Power)
			}
		}
	case "cure":
		for _, x := range p.Chars {
			x.Cure()
		}
	}
	if p.OnCast != nil {
		p.OnCast(s)
	}
	return msg, nil
}
