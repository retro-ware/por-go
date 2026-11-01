package engine

import (
	"por/dax"
	"slices"
	"strings"
)

// Bestiary (extra): the monster kinds the party has met (e.ext.met, kept
// with the save), published in camp (View.Camp.Bestiary).

// beast is a bestiary entry: name, danger level (I..VIII) and description
// (por-bestiary.tsv); also are the other MON*CHA names of the kind (the
// entry's name in capitals matches by itself).
type beast struct {
	Name   string
	Danger int
	Desc   string
	also   []string
}

var bestiary = []beast{
	{"Anhkheg", 6, "Large burrowing insects with great mandibles. These creatures have been known to spit a powerful acid.", []string{"AHNKHEG"}},
	{"Basilisk", 7, "A giant eight-legged lizard. One of the most dangerous creatures in the realms because their gaze can turn creatures to stone!", nil},
	{"Bugbear", 4, "Hideous giant sized goblins who stand over seven feet in height. Bugbears look clumsy but are strong, quick fighters with great stealth.", nil},
	{"Centaur", 4, "These good creatures are half men and half horse. They are capable fighters and can be valuable allies.", nil},
	{"Displacer Beast", 6, "These creatures are large, black puma-like creatures with two tentacles sprouting from their backs. These creatures can appear several feet from their actual location.", nil},
	{"Drider", 6, "These creatures resemble a cross between a drow elf and a giant spider. They are powerful spell casters.", nil},
	{"Efreeti", 7, "These large powerful jinn are from the elemental plane of fire. They are very arrogant and will only serve a powerful master.", nil},
	{"Ettin", 7, "These creatures look like giant two-headed orcs. They have great strength and usually wield two spiked clubs that inflict terrible damage in combat.", nil},
	{"Fire Giant", 8, "These evil giants have flaming red hair and are immune to all fire. They usually attack with giant two-handed swords.", nil},
	{"Giant Frog", 3, "These are giant carnivorous frogs. They are fast, dangerous predators who may be poisonous.", []string{"POISONOUS FROG"}},
	{"Giant Lizard", 4, "These are the giant cousins to the command lizard.", nil},
	{"Giant Mantis", 7, "These are the giant version of the common mantis. These creatures are fast, strong, and have good armor.", nil},
	{"Giant Scorpion", 6, "These are the giant version of the common scorpion. Its poisonous tail can kill a man.", []string{"HUGE SCORPION", "LARGE SCORPION"}},
	{"Giant Snake", 5, "These are giant poisonous snakes.", nil},
	{"Ghoul", 3, "These are evil undead whose touch may paralyze a man in combat. They feed on corpses and attack all living creatures on sight.", nil},
	{"Gnoll", 2, "These creatures are hyena-headed humanoids who stand over seven feet tall.", nil},
	{"Goblin", 1, "These are small humanoids common in the Realms.", []string{"GOBLIN GUARD", "GOBLIN LEADER"}},
	{"Hill Giant", 7, "These are one of the smaller, more stupid giants, but they are still tough opponents. They usually carry large clubs.", nil},
	{"Hippogriff", 3, "These magnificent creatures have the forelimbs and head of an eagle and the body and hind legs of a horse.", nil},
	{"Hobgoblin", 2, "These are human-sized, intelligent relatives of the goblin.", []string{"HOBGOBLIN LDR"}},
	{"Kobold", 1, "These are small, cowardly humanoids who delight in killing and torture.", []string{"KOBOLD LEADER"}},
	{"Lizardman", 3, "These are lizard-like humanoids. They are omnivorous but they have a particular fancy for human flesh.", []string{"MUTANT LIZ-MAN"}},
	{"Medusa", 6, "These are hideous women with snakes for hair. They can turn a man to stone with their gaze.", nil},
	{"Minotaur", 6, "These are strong bull-headed humanoids. They are cruel man eaters, commonly found in mazes.", nil},
	{"Mummy", 7, "These are powerful undead with great strength. The mere sight of one has been known to paralyze a man in combat. The touch of the mummy causes a strange rotting disease.", nil},
	{"Nymph", 5, "These are extremely beautiful creatures that appear as ever-young females. They usually inhabit wild lakes and streams.", nil},
	{"Ogre", 4, "These are large, foul-tempered, ugly humanoids. They are strong fighters.", []string{"OGRE LEADER"}},
	{"Orc", 1, "These are evil, pig-faced humanoids.", []string{"ORC LEADER"}},
	{"Phase Spider", 6, "\"These are giant poisonous spiders with the ability to phase in and out of this dimension. Usually they only \"\"phase in\"\" to attack, then \"\"phase out\"\" again.\"", nil},
	{"Quickling", 4, "These are small, fast-moving creatures. Because of their great speed they are invisible when they move.", []string{"QUICKLINGS"}},
	{"Skeleton", 1, "These are the least of the undead. These animated skeletons are usually controlled by some evil force.", []string{"GIANT SKELETON"}},
	{"Spectre", 7, "These are one of the most powerful of the undead. Their touch can drain the life out of men.", nil},
	{"Stirge", 2, "These are small, blood-sucking birds.", nil},
	{"Thri-Kreen", 6, "These are intelligent, carnivorous insect-men who live in burrows. They have four arms and a poisonous bite that paralyzes their foes.", nil},
	{"Tiger", 5, "\"These are noble beasts who are both strong and silent. Though their normal prey are animals, they have been known to become \"\"man-eaters.\"\"\"", nil},
	{"Troll", 6, "These are large, strong, ugly humanoids. They know no fear and can regenerate wounds.", nil},
	{"Vampire", 8, "These are one of the most dreaded undead in the Realms. They can drain life levels, are strong fighters, and are sometimes powerful magic users.", nil},
	{"Wardog", 3, "These are large, strong dogs, trained to kill. Orcs, goblins, and other evil humanoids are known to use them.", nil},
	{"Wight", 6, "Evil, undead humans whose touch can drain the life out of a man.", nil},
	{"Wild Boar", 4, "These creatures are the wild relatives of the pig.", nil},
	{"Wraith", 6, "These creatures are non-corporeal undead. Their touch can drain the life out of a man.", nil},
	{"Wyvern", 7, "These creatures are distant relatives of dragons. They attack by biting and using the poisonous sting in their tail.", nil},
	{"Zombie", 4, "Magically animated corpses controlled by an evil force. Zombies always fight back until destroyed or turned.", []string{"JUJU ZOMBIE"}},
}

// beastOf is the bestiary entry of a MON*CHA name, nil when none.
func beastOf(monster string) *beast {
	monster = strings.ToUpper(strings.TrimSpace(monster))
	for i := range bestiary {
		if b := &bestiary[i]; strings.ToUpper(b.Name) == monster || slices.Contains(b.also, monster) {
			return b
		}
	}
	return nil
}

// metIcon is where a met kind's combat icon is: CPIC file and block.
type metIcon struct {
	File  string
	Block int
}

// meetMonster records the kind of a monster loaded for a fight (addMonsters).
func (e *Engine) meetMonster(name, file string, block int) {
	b := beastOf(name)
	if b == nil {
		return
	}
	if e.ext.met == nil {
		e.ext.met = map[string]metIcon{}
	}
	e.ext.met[b.Name] = metIcon{file, block}
}

// beasts are the bestiary entries of the kinds met, with their icons.
func (e *Engine) beasts() []Beast {
	if b := e.ext.beasts; b != nil && len(b) == len(e.ext.met) { // kinds are only ever added
		return b
	}
	var out []Beast
	for _, b := range bestiary {
		ic, ok := e.ext.met[b.Name]
		if !ok {
			continue
		}
		bt := Beast{Name: b.Name, Danger: b.Danger, Text: b.Desc}
		if im := e.LoadPicMasked(ic.File, ic.Block, 0); im != nil && im.Count > 0 {
			e.recolour(im, &iconPaletteCE2)
			bt.Icon = e.picRGBA(im, 0, false, -1)
		}
		if bp, ok := beastPics[b.Name]; ok { // the scripts' picture of its fights (beastpics.go)
			if b := e.L.Block(e.set.file("PIC"), bp); b != nil {
				if fr := dax.ParseSprite(b, false, 0, true); len(fr) > 0 {
					bt.Pic = e.picRGBA(fr[0].Image, 0, false, -1)
				}
			}
		}
		out = append(out, bt)
	}
	e.ext.beasts = out
	return out
}
