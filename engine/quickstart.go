package engine

import "slices"

// Quickstart (not in the original): the empty party menu offers a ready-made
// party of six. Each member goes through CreateCharacter with its kind
// from a template, rerolled until its prime abilities are high, saved to
// the roster and then added like the Add command does.

// quickPick is CreateCharacter's template for one character.
type quickPick struct {
	answers []string     // race, gender, class, alignment (as the sheet names them)
	need    map[int]byte // ability index (0 str, 1 int, 2 wis, 3 dex, 4 con) -> minimum
	tries   int
	name    string
	head    byte
	body    byte
}

// kind is the template's race, gender, class and alignment.
func (q *quickPick) kind() kind {
	return kind{
		race:  byte(slices.Index(raceNames[:], q.answers[0])),
		sex:   byte(slices.Index(genderNames[:], q.answers[1])),
		class: byte(slices.Index(classNames[:], q.answers[2])),
		align: byte(slices.Index(alignmentNames[:], q.answers[3])),
	}
}

// good keeps the roll when every wanted ability is reached; after 2000
// rolls the next one is kept whatever it is.
func (q *quickPick) good(c *Character) bool {
	q.tries++
	if q.tries > 2000 {
		return true
	}
	for i, min := range q.need {
		if c.Stats[i] < min {
			return false
		}
	}
	return true
}

// quickParty: two fighters in front, two clerics, two magic-users (Sleep is
// the spell that wins the early fights) and a thief for locks and traps.
var quickParty = []quickPick{
	{answers: []string{"Dwarf", "Male", "Fighter", "Lawful Good"}, need: map[int]byte{0: 18, 4: 17}, name: "Thorgrim", head: 3, body: 2},
	{answers: []string{"Human", "Female", "Cleric", "Lawful Good"}, need: map[int]byte{2: 17, 4: 15, 0: 15}, name: "Mirelle", head: 6, body: 4},
	{answers: []string{"Half-Elf", "Male", "Cleric/Fighter", "Neutral Good"}, need: map[int]byte{0: 17, 2: 16, 4: 15}, name: "Aldric", head: 9, body: 7},
	{answers: []string{"Elf", "Female", "Fighter/Magic-User", "Chaotic Good"}, need: map[int]byte{0: 16, 1: 17, 3: 16}, name: "Sylvaine", head: 12, body: 9},
	{answers: []string{"Human", "Male", "Magic-User", "Neutral Good"}, need: map[int]byte{1: 18, 3: 16, 4: 14}, name: "Corwin", head: 1, body: 11},
	{answers: []string{"Halfling", "Male", "Fighter/Thief", "Neutral Good"}, need: map[int]byte{0: 16, 3: 17, 4: 15}, name: "Pip", head: 14, body: 5},
}

// QuickStart creates the quickParty members and puts them in the party.
func (e *Engine) QuickStart() {
	e.game.PartyCount = 0
	for i := range quickParty {
		q := quickParty[i] // a fresh copy each time
		e.ext.quick = &q
		e.CreateCharacter()
		e.ext.quick = nil
		c := &Character{}
		if e.LoadCharacter(q.name, c, false) { // ovr18:0AEC
			e.AddToParty(c)          // ovr18:1446
			e.BuildCharIcon(c, true) // ovr18:0482
		}
	}
	e.partySel = 0
}
