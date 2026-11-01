package engine

// spellRules tell what each spell does under the game's rules (the camp's
// spell lists' tooltips), indexed like spellNames.
var spellRules = [57]string{
	"",
	"Bless: allies in a 5x5 area not yet in melee get +1 to hit. Lasts 6 rounds.",
	"Curse: enemies in a 5x5 area get -1 to hit. Lasts 6 rounds; a saving throw negates.",
	"Cure Light Wounds: touch heals 1d8 hit points.",
	"Cause Light Wounds: touch attack dealing 1d8 damage.",
	"Detect Magic: shows which items the party carries are magical.",
	"Protection From Evil: the target gets +2 on saves and evil attackers -2 to hit it. Lasts 3 rounds per level.",
	"Protection From Good: the target gets +2 on saves and good attackers -2 to hit it. Lasts 3 rounds per level.",
	"Resist Cold: the target takes half damage from cold and +3 on saves against it. Lasts 1 turn per level.",
	"Burning Hands: a fan of flame dealing 1 point per level to the targets in front. No save.",
	"Charm Person: a humanoid that fails its save fights on the party's side.",
	"Detect Magic: shows which items the party carries are magical.",
	"Enlarge: the target grows, gaining strength (to hit and damage). Lasts 1 turn per level; an unwilling target saves.",
	"Reduce: undoes an Enlarge.",
	"Friends: the caster's Charisma rises by 2d4 for 1 round per level, helping in encounters.",
	"Magic Missile: one 1d4+1 missile per 2 levels, never missing. No save.",
	"Protection From Evil: the target gets +2 on saves and evil attackers -2 to hit it. Lasts 2 rounds per level.",
	"Protection From Good: the target gets +2 on saves and good attackers -2 to hit it. Lasts 2 rounds per level.",
	"Read Magic: reads magic scrolls so their spells can be scribed or cast.",
	"Shield: the caster gets AC 4 against missiles, AC 2 against other attacks and is immune to Magic Missile. Lasts 5 rounds per level.",
	"Shocking Grasp: touch attack dealing 1d8 + 1 per level.",
	"Sleep: puts 2-16 hit dice of creatures to sleep, the weakest first; creatures of 5 hit dice or more are immune. No save.",
	"Find Traps: reveals traps ahead. Lasts 3 turns.",
	"Hold Person: up to 3 humanoids are paralysed unless they save. Lasts 4 rounds + 1 per level.",
	"Resist Fire: the target takes half damage from fire and +3 on saves against it. Lasts 1 turn per level.",
	"Silence, 15' Radius: no one at the target spot can cast spells. A target that saves is not affected.",
	"Slow Poison: keeps a poisoned character alive until the poison is cured. Lasts 1 hour per level.",
	"Snake Charm: charms snakes, as many hit points of them as the caster has.",
	"Spiritual Hammer: a magic hammer attacks for the caster, dealing 1d4+1 (1d4 against large). Lasts 1 round per level.",
	"Detect Invisibility: the caster sees invisible creatures.",
	"Invisibility: attackers have -4 to hit the target until it attacks or casts.",
	"Knock: opens locked or barred doors.",
	"Mirror Image: 1d4 images of the caster absorb attacks, one each hit. Lasts 2 rounds per level.",
	"Ray of Enfeeblement: the target loses a quarter of its strength unless it saves. Lasts 1 round per level.",
	"Stinking Cloud: creatures in a 2x2 area that fail their save are helpless for 1d4+1 rounds.",
	"Strength: the target gains strength by its class (1d8 for fighters, 1d6 for clerics, 1d4 for magic-users). Lasts 6 turns per level.",
	"Animate Dead: raises a dead body as a zombie or skeleton fighting for the party.",
	"Cure Blindness: ends blindness.",
	"Cause Blindness: blinds the target unless it saves.",
	"Cure Disease: ends disease.",
	"Cause Disease: diseases the target unless it saves.",
	"Dispel Magic: ends the spells on the creatures in the area, by chance against the caster's level.",
	"Prayer: allies get +1 to hit and on saves, enemies -1. Lasts 1 round per level.",
	"Remove Curse: ends a curse on a character, freeing them to drop a cursed item.",
	"Bestow Curse: the target gets -4 to hit and on saves unless it saves. Lasts 1 turn per level.",
	"Blink: the caster blinks away after acting, hard to hit. Lasts 1 round per level.",
	"Dispel Magic: ends the spells on the creatures in the area, by chance against the caster's level.",
	"Fireball: a ball of fire dealing 1d6 per level to all in its area; a save halves it.",
	"Haste: the targets act twice as often for 3 rounds + 1 per level, and age a year.",
	"Hold Person: up to 4 humanoids are paralysed unless they save. Lasts 2 rounds per level.",
	"Invisibility, 10' Radius: makes the caster and those beside them invisible.",
	"Lightning Bolt: a bolt dealing 1d6 per level to all in its line, bouncing off walls; a save halves it.",
	"Protection From Evil, 10' Radius: Protection From Evil for the target and those beside it. Lasts 2 rounds per level.",
	"Protection From Good, 10' Radius: Protection From Good for the target and those beside it. Lasts 2 rounds per level.",
	"Protection From Normal Missiles: the target cannot be hurt by non-magical arrows and bolts. Lasts 1 turn per level.",
	"Slow: the targets in the area act half as often unless they save. Lasts 3 rounds + 1 per level.",
	"Restoration: restores a level drained by an undead.",
}

// SpellRules tells what spell id does, wrapped to lines of w characters.
func SpellRules(id, w int) []string {
	if id <= 0 || id >= len(spellRules) {
		return nil
	}
	return WrapWords(spellRules[id], w)
}
