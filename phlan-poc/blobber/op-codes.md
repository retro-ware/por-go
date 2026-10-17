# Event script opcodes

Scripts live in `maps/<map>.evt`, one event per tile and facing, one opcode per line.
Parser and VM: `events/events.go`. Xeen originals: `reference.md`.

## File format

```
# comment
event X,Y FACING [use]
  opcode args...
label:
  opcode args...
```

- `FACING` is `n`, `e`, `s`, `w` or `any`. An exact-facing event beats an `any` event on the same tile.
- Without `use` the event fires automatically after every step or turn onto the tile with that facing (Xeen behaviour).
  With `use` it fires only when the player interacts (F key), i.e. searches the tile.
- Keywords are case-insensitive. Strings are `"double quoted"`. Names are bare words. Numbers are decimal or `0x..`.
- A jump target `L` is a label or a 0-based line number within the event. Unknown labels fail at load.
- Execution starts at line 0 and runs until `exit`, `remove`, `teleport`, a failed `take`, or the end of the event.

## Properties

`give`, `take`, `set` and `if` take a *property* and pass it to the host as a string. The VM does not enumerate them;
the host decides what exists. The Xeen vocabulary (see reference.md, opcode 0x08) is the intended set:

`sex race class hp sp ac age level exp gold gems food minutes day week year award skill condition spell item
might int pers end spd acc luck` (base/tmp variants), resistances `fire elec cold poison energy magic`,
party-wide `partyfire partyelec partycold partypoison`, `levitate light wizardeye walkonwater`, `inparty`.

Two properties are special:

| property | meaning |
|---|---|
| `flag NAME` | global game flag, passed to the host as `flag`, `NAME` |
| `mapflag NAME` | per-map flag, passed to the host as `flag`, `<map>:NAME` |

Most properties act on the **selected character** (see `setchar`, `whowill`, `selrndchar`). Slot 1 is the default,
0 means the whole party.

## Opcodes

### Flow control

| keyword | Xeen | behaviour |
|---|---|---|
| `nop` | 0x00 NOP, 0x0D/0x2F NoAction | does nothing |
| `exit` | 0x12 Exit | stops the event |
| `goto L` | 0x36 Goto | unconditional jump (Xeen's surface check dropped) |
| `gotorandom L L ...` | 0x38 GotoRandom | jumps to one of the labels at random |
| `jumprnd MAX N goto L` | 0x17 JumpRnd | rolls 1..MAX; jumps if the roll equals N |
| `if PROP [ARG] [OP N] goto L` | 0x08 / 0x09 / 0x0A If, 0x2A IfMapFlag | compares `Get(who, PROP, ARG)` with N using OP (`< <= == != >= >`); without OP, true if non-zero |
| `choose L L ...` | 0x30 ChooseNumeric | asks for option 1..n+1; option i jumps to the i-th label, the last option continues |
| `confirm "PROMPT" "WORD" ["WORD2"] goto L` | 0x15 / 0x37 ConfirmWord | asks for a word; jumps if it matches either word, case-insensitively, else continues |
| `call X,Y FACING` | 0x19 CallEvent | runs the automatic event at X,Y as a subroutine |
| `return` | 0x1A Return | returns from `call` (falling off the end also returns) |

### Text and NPCs

| keyword | Xeen | behaviour |
|---|---|---|
| `text [STYLE] "LINE" ["LINE2"]` | 0x01 Display, 0x02/0x03 DoorText, 0x04/0x27 SignText, 0x25 DisplayStat, 0x29 DisplayBottom, 0x31 DisplayBottomTwoLines, 0x32 DisplayLarge, 0x35 DisplayMain | shows text; STYLE is `main` (default) `door` `doorlarge` `sign` `signlarge` `stat` `bottom` `large`. Returns a Message wait |
| `npc ID "TEXT" [confirm L]` | 0x05 NPC | NPC dialog window: portrait, name, wrapped text (`|` breaks a line). The script waits for Continue; with `confirm`, Yes jumps to L, No continues. ID comes from `maps/npcs.txt` (`id portrait Display Name`); portraits come from the same pool as the party's (`ui/assets/portraits/<id>.png`, 45x45), and `set N portrait ID` changes a character's. |

### Characters, stats, inventory

| keyword | Xeen | behaviour |
|---|---|---|
| `give PROP [ARG] [N]` | 0x0C / 0x1C / 0x1D / 0x26 TakeOrGive | `Give(who, PROP, ARG, N)`; N defaults to 1 |
| `take PROP [ARG] [N]` | same | `Take(...)`; if the host reports failure the event exits |
| `set PROP [ARG] [N]` | 0x1B SetVar | `Set(...)` |
| `setchar N` | 0x0F SetChar | selects slot N; 0 = whole party |
| `whowill WHAT "TEXT"` | 0x20 WhoWill | asks which character; WHAT is e.g. `search open drink mine touch`. Escape (slot 0) exits. A party of one skips the prompt |
| `selrndchar` | 0x2B SelRndChar | selects a random slot |
| `damage N KIND` | 0x16 Damage | KIND: `physical magic fire elec cold poison energy dead` |
| `rnddamage KIND MAX` | 0x21 RndDamage | 1..MAX damage |
| `giveitem KIND [ENCHANT [EXTRA]]` | 0x2C GiveEnchanted | `GiveItem(who, KIND, ENCHANT, EXTRA)` |
| `itemtype KIND` | 0x2D ItemType | `GiveItem(who, KIND, 0, 0)` |

### Map and world

| keyword | Xeen | behaviour |
|---|---|---|
| `teleport MAP X Y [continue]` | 0x07 Teleport / 0x1F TeleportAndContinue | moves the party; exits the event unless `continue` |
| `fall MAP X Y DAMAGE` | 0x34 FallToMap | moves the party with fall damage; exits |
| `spawn MONSTER X Y` | 0x10 Spawn | places a monster; MONSTER is an id from `maps/monsters.txt` |
| `alterwall X Y SIDE KIND` | 0x13 AlterMap | changes the wall on SIDE (n e s w) of a cell. Walls are whole cells, so this sets the neighbouring cell. KIND is a material letter or name (stone brick moss wood iron), `wall`, or `open` |
| `altercell X Y KIND` | 0x23 AlterCellFlag | changes a cell's floor type |
| `moveobj NAME X Y` | 0x0B MoveObj | moves a map object |
| `movewall NAME X Y` | 0x22 MoveWallObj | moves a wall object (same host call) |
| `exchobj A B` | 0x33 ExchObj | swaps two objects |
| `sound NAME` | 0x06 PlayFX, 0x28 PlayEventVoc | plays a synthesized effect: click coins door trap hit magic bell lever splash fanfare |
| `music NAME` | 0x3C PlayCD | loops a synthesized track: dungeon cave town lab victory. `music stop` ends it |
| `cutscene NAME` | 0x1E / 0x39 / 0x3A CutsceneEnd* | shows `maps/cutscenes/NAME.txt` full screen. The script continues underneath |
| `town KIND` | 0x11 DoTownEvent | opens a shop: smith, guild, temple, tavern, trainer, bank |

### Story

| keyword | Xeen | behaviour |
|---|---|---|
| `win` | 0x39 / 0x3A CutsceneEnd* | the game is won: a last word, then the title menu. Play `cutscene ending` first for a finale |
| `gameover` | — | the quest has failed; same flow |

Awards are the quest log: `give award ID` records one, `if award ID goto L` tests it, J shows them with
the text from `maps/awards.txt` (`id text...`).

### Self-modifying

These persist through `VM.Patches()` / `ApplyPatches()`; the `.evt` file is never changed.

| keyword | Xeen | behaviour |
|---|---|---|
| `remove` | 0x0E Remove | deletes this event for good and exits |
| `nothing` | 0x2E MakeNothingHere | replaces this event with `text "Nothing here."` and exits |
| `alter L OP` | 0x18 AlterEvent | rewrites line L's opcode to OP (arguments kept; use an argument-less OP like `nop` or `exit`) |

### Dropped from Xeen

0x14 GiveExtended (undocumented), 0x24 AlterHed (map header byte), 0x3B FlipWorld (two-world switch).

## Example

```
event 3,5 any use
  if mapflag searched goto done
  whowill search "Who will search the rubble?"
  give gold 10
  set mapflag searched
  sound coins
  exit
done:
  text "Nothing here."
```

## Items, keys and map objects

- `item NAME` checks for, gives or takes inventory items from the catalog: dagger, sword, bow, shield, helmet, chainmail, boots, robe, ring, amulet, potion, scroll, gem. Unknown names become misc items.
- `key NAME` is the item "NAME key". `give key brass`, `if key brass goto L`, `take key brass`.
- `giveitem BASE [BONUS]` and `itemtype BASE` add one item to the shared party inventory.
- Weapons are named MATERIAL_FAMILY, e.g. `steel_longsword`, `oak_staff`, `bloodforged_katana`. Materials, worst to best: bronze (oak for wooden weapons), iron, steel, orichalcum, bloodforged. Families and stats are in `party/weapons.go`. The smith sells up to steel.

Extra lines in a `.map` file, after the grid:

| Line | Meaning |
|------|---------|
| `object NAME SPRITE X Y` | A named billboard. `moveobj`, `movewall` and `exchobj` act on it. Put it at -1 -1 to hide it. |
| `item BASE X Y [BONUS]` | An item hidden on the tile, never drawn (as in World of Xeen). F finds and takes it. |
| `monster ID X Y` | A monster from `maps/monsters.txt` (`id sprite hp ac dmg exp [spd [size]]`). Unknown ids are drawn with a sprite of their own name and get stock stats. |

Monster pictures live in `cmd/blobber/assets/monsters/`: either `<sprite>.png` (a still) or a folder
`<sprite>/` holding `idle.png` and optionally `attack.png` and `hurt.png` (transparent PNGs, one pose
each). Attack shows when the monster strikes and hurt when it is wounded, each for a moment before idle
returns; a killed monster shows hurt, then vanishes. Drop a folder there and the editor's monster row shows it.
| `deco grass\|lantern\|firefly X Y` | Decoration. |

Spells: the Mage Guild (`town guild`) teaches the spells in `maps/spells.txt` (`id kind sp price
power | Name | description`; kinds heal, healall, cure, light, bolt, fire) to classes with SP;
C opens the spellbook to cast. Conditions (`poisoned`, `asleep`, `paralyzed`, `dead`) are set by
scripts (`give condition poisoned`): poison costs 1 HP a turn, asleep and paralyzed cannot act;
cure, the temple and rest clear them. Knowing a spell is the property
the `spell` property with the spell id as its argument, so scripts can test, give and take spells.

In game, F takes floor items first and otherwise runs the tile's `use` event. I opens the inventory.

## World file

`maps/world.txt` names the campaign and where a new game starts:

```
title Blobber Demo
start demo 1 1 e
hero Ann f human knight   # up to six, rolled like at the inn
gold 500
food 6
item potion               # one line per copy
```

The title menu (new game, load, quit) shows at startup and after `win`, `gameover` or a party wipe.
R rests for eight hours when no monster is in sight: one food per character, full HP and SP.
G toggles god mode for testing: the party is invisible to monsters and takes no damage.

## Turns, combat, saving

Play is turn-based: a step, strafe, attack or wait (Space) is one turn, after which every monster acts.
Turning in place is free. A monster next to the party attacks a random living character; one within
sight steps towards the party; others wander. Walking into a monster attacks it with every living
character (right-hand weapon, fists for 2). A kill gives its `exp` to the living. One turn is one game minute.

F5 saves, F9 loads: one slot, `blobber.sav` next to the binary (browser storage on the web). A save holds
the party, position, script patches and every visited map's grid, props, floor items and monsters.
When the whole party is down, the last save is loaded; without one the game quits.

## The opcode lab

`maps/lab` has one alcove per wired opcode. Press F on the demo start tile to get there. Walking into an alcove shows a sign. F runs the demo. The lab's first alcove leads back.

## Cutscene files

`maps/cutscenes/NAME.txt` holds pages separated by blank lines. A line `@sprite` shows that sprite large above the text. Lines starting with `#` are comments. Space, Enter, F or a click turns the page, and Esc skips the rest.
