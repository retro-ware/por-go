# Roadmap: from demo to a complete Might & Magic 3-5 style engine

Goal: a content creator writes `.map` + `.evt` files, data tables and assets, and ships a full game
with a story, quests and an ending. Content tooling stays at the level of the originals (tile events
with opcodes, fixed monster placement, data tables); the engine side (graphics, movement, UI) may be better.

## Decisions (2026-10-06)

1. **Combat is turn-based, always.** Roguelike rounds: the party takes a turn (step, strafe, use, cast, wait), then every monster acts. Turning in place is free. A monster entering the party's cell (or the party entering its) starts melee, Might & Magic style: no stepping, turning allowed; each round everyone acts once in speed order and every character picks Attack or Flee. Flee kills the unconscious and drops the party on the nearest random cell no monster can see. There is no real-time mode.
2. **Dungeons and towns first.** Overworld, terrain and sky only if good assets turn up.
3. **Our own classes and races**, not the MM3 list. Data tables, curated later.
4. **Our own spell list**, curated.
5. **Default party plus character creation at the inn.** Hirelings are a later nice-to-have.
6. **Desktop and web** are both targets. Saves: file on desktop, browser storage on web.
7. **Monsters have three poses, not animations.** Idle, plus attack and hurt shown briefly; death is the hurt pose, then gone.
8. **Chiptune synth stays** until better sound sources are found.
9. **The editor must be able to build a complete campaign** (maps, events, world file, data tables).

## Follow-ups (open items from the work so far)

- [x] Floor and ceiling textures in `assets/walls/floor.png` and `ceiling.png`.
- [ ] Licence lines in `CREDITS.md` for the stone wall, the floor, the ceiling and the monster poses.
- [x] Monster size: optional `size` column in `monsters.txt` (tiles tall, default 0.6).
- [ ] Monster pathfinding: greedy chase stalls behind corners (`monstersTurn` in `blobber.go`).
- [ ] Bolt spells fly in real time after the turn passes, so a monster can step out of the way.
- [ ] Save slots and autosave; currently one slot, `blobber.sav` / localStorage.
- [ ] Item loot on kills (gold only today) and a loot column in `monsters.txt`.
- [ ] Rest has no random encounters.
- [ ] Inn creation is a prompt chain: no portrait choice, no re-rolls.
- [x] Default party is compiled in (`newParty` in `cmd/blobber/game.go`); move it into `maps/world.txt`.
- [ ] Opcode lab (`maps/lab`) does not yet show `win`, `gameover`, doors or the inn.
- [ ] Editor cannot place objects, items or decorations, or edit `world.txt`.
- [ ] Revert the test monsters: `maps/demo.map` and the cave guardian all use the ice giant.

## What exists today

- Grid renderer: five generated wall materials, floor, ceiling, billboards (objects, monsters, decorations), world signs, torch light, dither, minimap.
- Movement: step, strafe, turn, tween, mouse/touch, held-key walking.
- Event VM: the Xeen opcode set (`op-codes.md`), labels, prompts, self-modifying events (in memory only).
- Party: characters with stats, resistances, awards/skills/conditions/spells flags, slots, shared stacked inventory, weapon catalog with five material tiers, four spells, six shops, NPC dialogs, cutscenes, help screen, HUD portraits, hit markers.
- Monsters: billboards with a random walk and fixed HP, killed by the debug bolt. No combat.
- Audio: synthesized effects and loops. Editor: paint walls, edit tile scripts. Desktop and web builds.

## A. Must have for a shippable game

### A1. Save / load (nothing exists)

- [x] Serialize party (characters, inventory, equipped, gold/gems/food/bank, flags, time).
- [x] Serialize world: position, facing, current map, VM patches, per-map cell changes, moved objects, taken floor items, monster state.
- [~] One slot (F5 / F9); file on desktop, browser storage on web. Slots and autosave pending.
- [x] Load from the title menu; F5 / F9 quick save and load.

### A2. Combat (nothing exists beyond the bolt)

- [~] Monster data table (`maps/monsters.txt`): id, sprite, HP, AC, damage, exp, speed, size. Pending: attack kinds, resistances, special attacks, loot class, ranged/caster, boss flag.
- [~] Monster AI: chase within sight, step into the party's cell for melee, wander otherwise. Pending: pathfinding, ranged, flee.
- [~] Turn-based play everywhere (decision 1): step / wait / melee round = one turn, then monsters act. Casting costs a turn. Melee rounds in speed order with Attack / Flee. Pending: block, spells and target choice in a round.
- [~] d20 to-hit vs AC with level and accuracy, weapon damage plus might. Pending: criticals, ranged row.
- [~] Conditions: poisoned, asleep, paralyzed, dead (`party.Conditions`); poison bites per turn, asleep/paralyzed cannot act; cured by the cure spell, temple and rest. Pending: diseased, stone, eradicated, cursed, weak, drunk, monster attacks that inflict them.
- [~] Experience, gold drop and message strip done. Pending: item loot, scrolling log.
- [ ] Respawn rule: non-boss monsters return after N days.
- [x] Party wipe → message, then the title menu.

### A3. Characters

- [~] Class and race tables (`maps/classes.txt`, `maps/races.txt`): HP/SP per level, level cost, stat modifiers. Pending: allowed classes, spell schools.
- [x] Level-up at the trainer from the class table.
- [ ] Skills table and effects (direction sense, cartographer, spot secret doors, danger sense, swimmer, mountaineer, pathfinder, linguist, merchant, thievery, …).
- [ ] Stats actually used in rules (might → damage, speed → initiative, endurance → HP, luck → saves, personality/intellect → SP).
- [ ] Character sheet screen (stats, resistances, skills, awards, conditions) replacing the debug `P` panel.
- [~] Inn (`town inn`): create a character by prompts, leave members at the inn, take them along, up to 6. Pending: portrait choice, re-rolls.
- [ ] Aging, and `age` effects if wanted.

### A4. Spells

- [~] Spell table `maps/spells.txt`: id, kind, SP, price, power. Pending: school, level gating, gem cost, targets, power by level, duration.
- [ ] Target picker for single-target spells.
- [ ] Timed buffs (bless, heroism, protection from X, power shield, holy bonus, …) and the `*tmp` stats they set.
- [ ] Utility spells with world effects: light (done), wizard eye, levitate, water walk, jump, teleport, town portal, Lloyd's beacon, time distortion, etherealize, super shelter, day of protection, day of sorcery, awaken, cure *.
- [ ] Guild membership per town; spells bought by level.
- [ ] Spellbook with the right lists per class.

### A5. Items

- [~] Armor: leather, chainmail, plate, cloak, helmet, boots, shields by material. Pending: material tiers for body armor, gauntlets.
- [ ] Enchantment model: material prefix + base + attribute suffix (`of Might`, `of Fire`, elemental damage, stat bonuses) with effects applied when worn.
- [ ] Item generation by level for loot and chests; treasure tables per monster loot class.
- [ ] Cursed, unidentified and broken items; identify and repair at the smith.
- [ ] Usable items in the world (wands with charges, scrolls that cast, quest items).
- [ ] Carry limit (weight or slots) and gems as a currency in shops.

### A6. World and time

- [~] Game clock: one minute per turn, rest adds eight hours, day rollover. Pending: shop time, day/night.
- [~] Rest (R): full heal for one food each, refused near monsters. Pending: random encounters.
- [~] Doors: `D` is an open doorway drawn as a door, `L` a locked one that scripts open with `altercell X Y D`. Pending: bash, traps, secret doors.
- [ ] Wall decorations: torches, grates, signs, stairs, cave mouths, town entrances.
- [ ] Terrain types with rules: water (swimmer / water walk), swamp (poison), lava (damage), desert, snow, road, mountains (mountaineer), forest; movement cost or block.
- [ ] Outdoor maps: sky backdrop, horizon, day/night tint, distant trees/mountains as blocks or billboards.
- [~] Map screen (M) with explored cells and coordinates, saved per map. Pending: map name, cartographer skill gate.
- [~] World file (`maps/world.txt`): title, start, default party (heroes, gold, food, items). Pending: map names, music, indoor/outdoor.

### A7. Story, quests, win state

- [x] Journal (J) listing awards with text from `maps/awards.txt`.
- [~] Opcodes `win` and `gameover` (last word, then title); awards text table. Pending: score, credits.
- [~] Title menu: new game, load, quit. Pending: options, credits, art.
- [ ] Intro / ending cutscenes with full-screen art (text-only pages exist).
- [x] Party wipe → message, then the title menu.

### A8. Shops, completed

- [ ] Temple: cure each condition, uncurse items, donate for a blessing.
- [ ] Smith: repair, identify.
- [ ] Bank: gems, interest per day.
- [~] Tavern rumors from `maps/rumors.txt`. Pending: per-town files, prices.
- [ ] Guild: membership + level-gated spells.

### A9. Audio

- [ ] Load real tracks and effects (OGG) by name; keep the synth as fallback.
- [ ] Per-map music from the world file; combat and boss music; victory, game over, title.
- [ ] Monster attack / hit / die sounds from the monster table.
- [ ] Volume options.

## B. Should have (the "better than the originals" part)

- [ ] Gamepad support.
- [ ] Message log (scrollable history of event and combat text).
- [ ] Options screen: volume, key bindings, movement speed, dither, fullscreen (partly in the Esc menu).
- [x] Monster poses (idle / attack / hurt); see op-codes.md.
- [x] Painted textures from `assets/walls/`: walls by map character (`walls.txt`), floor and ceiling per map (`floor NAME` / `ceiling NAME`), generated with `create_textures.sh`.
- [ ] Smooth day/night lighting outdoors; weather is optional.
- [ ] Quest markers on the map screen for known quest tiles (optional, not in the originals).
- [ ] Portrait expressions by condition (poisoned, asleep, dead) or at least condition icons on the HUD.

## C. Content tooling

- [~] Editor: monster palette from the asset folder, door cells in the wall palette. Pending: objects, items, decorations.
- [ ] Editor: world file editing (map list, start, music), NPC registry, cutscene preview.
- [~] Data tables in `maps/`: monsters, classes, races, awards, rumors, world. Pending: spells, items, armor, skills (still Go code).
- [ ] Opcode lab kept in sync with new opcodes (`win`, doors, terrain).
- [ ] A `blobber check` command that lints a campaign: missing sprites, unknown items/monsters/spells, bad jumps, unreachable maps.

## D. Known loose ends in the current code

- Properties `levitate light wizardeye walkonwater` are documented but not stored by the party.
- Bolt spells fly in real time after the turn passes, so a monster may step out of the way.
- Spells and the item catalog are compiled in (rumors, awards, classes, races, monsters are data files).
- NPC portraits are recoloured party portraits (see `maps/npcs.txt`).

## E. What you need to provide

All sizes are suggestions matching the current code (128 px textures, 45 px portraits, 16 px icons).
Everything is PNG with transparency unless noted; audio is OGG.

### Art

| Asset                    | Count                                                                                                           | Spec                                                                         | Notes                                   |
| ------------------------ | --------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | --------------------------------------- |
| Wall texture sets        | 1 per map theme (cave, castle, crypt, sewer, town, temple, …), 3-6 sets                                         | 128×128, each set: wall front + 1-2 variants, floor, ceiling                 | Currently generated; sets replace them  |
| Wall features            | doors (closed, open, locked), secret door, grate, torch sconce, sign, stairs up/down, cave mouth, town entrance | 128×128 overlays on a wall                                                   |                                         |
| Outdoor ground           | grass, dirt, road, sand, snow, swamp, water, lava                                                               | 128×128 tileable                                                             | Only if the game has an overworld       |
| Outdoor blocks           | trees, mountains, rocks, town buildings                                                                         | 128×128 wall-style or billboards                                             |                                         |
| Sky                      | per biome × day/dusk/night                                                                                      | wide panorama, e.g. 1024×256                                                 |                                         |
| Monster sprites          | 40-100                                                                                                          | 128-256 px tall billboards, 1 frame minimum, 3 (idle/attack/hit) if animated | Six placeholders exist                  |
| Boss / cutscene art      | per story beat                                                                                                  | full screen 640×480                                                          | Intro, ending, chapters                 |
| Party portraits          | 20-30                                                                                                           | 45×45                                                                        | Enough for character creation choices   |
| NPC portraits            | 1 per named NPC                                                                                                 | 45×45                                                                        | Replaces the recoloured ones            |
| Condition / status icons | ~12                                                                                                             | 16×16                                                                        | Poisoned, asleep, dead, …               |
| Item icons               | armor families × tiers, accessories, wands, quest items                                                         | 16×16                                                                        | Weapons and basics exist (CC BY pack)   |
| UI                       | title screen, game over, victory, map screen frame, combat panel, cursor                                        | 640×480 and skin pieces                                                      | Dark Ages UI covers windows and buttons |

### Audio

- Music: title, town, dungeon ×2-3, cave, outdoor day, outdoor night, combat, boss, victory, game over, cutscene.
- Effects: footsteps (stone, grass, water), door open/locked, chest, trap, coins, buy, error, UI click, level up, rest,
  melee hit/miss by weapon class, arrow, spell per element, heal, monster attack/hit/die (per monster family), party member dies.

### Data and writing

- World file: map list with names, indoor/outdoor, music, start position, starting party or inn roster.
- Tables: monsters, armor, item enchantments, spells, classes, races, skills, awards.
- Maps and events for every area, NPC lines, signs, rumors per tavern, quest text, awards text.
- Story: intro, chapter cutscenes, ending, credits.
- Licences for every third-party asset in `CREDITS.md`.
