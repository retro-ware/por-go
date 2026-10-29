# Area scripts

`area<N>.txt` is the event script of area N (the area numbers and names
are in `names/areas.txt`). They were written once from the game's ECL byte
code by `cmd/eclscript` (`go run ./cmd/eclscript data/assets/scripts`).
The engine parses them into Go structs at area load and runs those; it
never turns them back into bytes. `names/` holds the variable, label and
value names (see `names/README.md`).

## Layout

```
event0:                     a label names the next command
	Command operand ...     one command per line, indented
	if A == B: Command      a test (below)
	// comment
Table:
	Data 1 2 3              the bytes of a table
```

Every script has five entry points, `event0` .. `event4`:

| Label    | Runs when                                                     |
|----------|---------------------------------------------------------------|
| `event4` | the area loads (map, walls, sky; also after `NewECL`) |
| `event0` | before each move, after the party's command (and after an area change) |
| `event1` | after each step, and when the party looks or searches: the square's events |
| `event2` | the party camps, before the camp menu |
| `event3` | after the camp menu (rest interruptions) |

Execution runs from the entry label until `Exit`. `Goto` jumps, `Gosub`
calls a section up to its `Return`.

## Operands

| Form          | Meaning |
|---------------|---------|
| `12`          | a number (a byte) |
| `#0807`       | a number in hex (a word) |
| `Name`        | a variable, label, table or area by name |
| `[4A07]`      | a variable by its script address, where it has no name |
| `&Name`       | the variable read as a word rather than a byte (or the other way round) |
| `"TEXT"`      | a string (Go quoting) |
| `$Name`       | a string variable |
| `Kobold`      | a named number: a monster, item, picture, sprite, icon ... (by the operand's kind; `names/values.txt`, `names/pictures.txt`) |
| `count=8`     | an operand with its label; the label is only for reading and must be that operand's |

`Scratch1` .. `Scratch10` are temporary words that every section reuses.
`alias Choice = Scratch1` under a label names one for that section (until
the next label). `cmd/eclscript` adds `Choice` for a menu's answer and
`<Table>` for an entry read from a table.

## Assignments and arithmetic

| Script            | Does |
|-------------------|------|
| `D = V`           | D gets V (a number, variable or string) |
| `D = A + B`       | addition, also `-` `*` `/` (integer, x/0 = 0) |
| `D = A & B`       | bitwise and, `\|` or. Sets the test flags as `0 op D` (see Tests) |
| `D += A`, `D -= A`| add to D, take from D |
| `D = T[I]`        | entry I of table (or variable row) T |
| `T[I] = V`        | set entry I of T |
| `D = Random N`    | a random number 0 .. N |

Words are 16 bits and wrap; many variables are bytes.

## Tests

```
if Gold >= 100: Goto Rich              one command
if Facing == 2 { Goto A; Exit }        several on one line
if Facing == 2 {                       or one per line
	Sky = 8
	Exit
}
also if Facing < 2: Goto B             the same test's result again (no new test)
```

| Test                      | True when |
|---------------------------|-----------|
| `A op B`                  | op is `==` `!=` `<` `>` `<=` `>=`; strings compare as text |
| `A == B && C == D`        | both equal |
| `A != B \|\| C != D`      | either differs |
| `Carried Item`, `!Carried Item` | someone in the party carries it (or nobody does) |
| `Ask M == "YES"` | calls menu section M, which asks a two-answer `D = Menu "YES" "NO"` and ends in `Compare D 0`, and tests the answer |
| `0 op D` right after `D = A & B` / `D = A \| B` | tests that result without a new comparison (`0 != D`: D is not 0) |

Every command inside an `if` block is guarded by the one test: a `Gosub`
inside may change what the commands after it see. The raw form, `Compare
A B` followed by `IfEq`/`IfNe`/`IfLt`/`IfGt`/`IfLe`/`IfGe` (each skips the
next command unless its comparison held), still appears where the test
is made elsewhere: after a `Gosub` of a section that ends in a test
(`CheckGarrisonsBeaten`, `RollSurprise`), or at a label in the middle of
a test.

## Flow

| Command | Does |
|---------|------|
| `Goto L` | continue at label L |
| `Gosub L` / `Return` | call section L, come back at `Return` |
| `OnGoto V L0 L1 ...` | go to the V-th label (0 first); past the list, continue |
| `OnGosub V L0 L1 ...` | the same as a call |
| `Exit` | end the event; redraws the view frame if text was shown |
| `Call Name` | run an engine routine the original called by address (`CallRedraw`, duel setup, sounds; `names/global.txt`) |
| `Program N` | open a game screen: 0 party menu, 9 encamp |
| `NewECL Area` | switch to another area's script (same data set) |
| `EnterArea Area` | switch to an area in another data set (`SymbolSet = set` + `NewECL`) |
| `Delay` | pause for the text speed |
| `Clock N` | advance the clock N minutes |

## Text and menus

| Command | Does |
|---------|------|
| `Print "TEXT"` / `Print V` | write text (or a number) in the text window |
| `PrintClear "TEXT"` | clear the text window first |
| `NewLine` | text cursor to the next row |
| `ClearBox` | blank the view window for text |
| `D = Menu "A" "B"` | a one-line menu; D gets the answer, 0 for the first |
| `D = Menu "A" -> L1 "B" -> L2` | the same, then go to the answer's label (`HMenu` + `OnGoto`) |
| `Ask M "YES" -> L1 "NO" -> L2` | call menu section M (it only asks `D = Menu "YES" "NO"` and returns), then go to the answer's label; an answer without `->` goes on with the next line (`Gosub M` + `OnGoto D L1 L2`) |
| `D = VMenu "PROMPT" "A" "B"` | a vertical list menu |
| `Parlay a b c d e D` | the HAUGHTY/SLY/NICE/MEEK/ABUSIVE menu; D gets the operand of the chosen attitude |
| `InputNumber N D`, `InputString N $D` | ask for a number or text into D (N is not used by the port) |
| `Who "PROMPT"` | choose a party member (the selected member after) |
| `LoadCharacter N` | select party member N (+128: also remove them if they died) |

## Pictures and the view

| Command | Does |
|---------|------|
| `Picture Name` | show a picture in the view; `Picture None` puts the 3D view back |
| `ShowPortrait Head Body` | show a two-part portrait (`Portrait = Head` + `Picture Body`); `ShowPortrait None None` clears it |
| `Portrait = Head` | while a head is set (not `None`), a `Picture` or an encounter's close-up is the BODY under that head (written `body=`; the head may have been set in a section that jumps here) |
| `RedrawView` | redraw the 3D view after a picture or sprite |
| `Sky = N`, `SkyInside = N` | the sky colours |
| `LoadMap Map unused=X icons=Y` | load map (GEO block) Map; `Keep`/`Unchanged` keep the current one. `unused` is ignored, by the original too. `icons` not 255 loads the wilderness travel icons outdoors |
| `LoadWalls A B C` | load the wall sets (WALLDEF blocks) into wall slots 1-3; `None` empties a slot, `Reset` in the first resets them |
| `Protection Base` | draw the glyph string stored in the variables from Base on |

## Monsters and fights

| Command | Does |
|---------|------|
| `ClearMonsters` | forget the loaded monsters and treasure |
| `LoadMonster M count=N icon=I` | add N monsters M (a monster record), drawn with combat icon I (the icon's CPIC block) |
| `AddNPC M Morale` | monster M joins the party |
| `SetupMonster S distance=D closeup=P` | show sprite S coming at the party from up to D squares away (fewer if a wall is nearer), P the close-up picture once it is near; `body=P` when a portrait is up |
| `Approach` | the monster comes one square closer |
| `EncounterMenu S distance=D closeup=P result=R combat= wait= flee= advance= parlay= near= mid= far= escape=E strength=N` | the COMBAT/WAIT/FLEE/ADVANCE (PARLAY when near) approach menu for sprite S, coming from D squares like `SetupMonster`. R gets 0 nothing, 1 fight, 2 the party got away, 3 talk. `combat` .. `parlay` are how the monsters react to each choice (below). `near`/`mid`/`far` are the texts shown at distance 0/1/2; a text left out is empty and falls back to the others. `escape`: against `Attack`, FLEE gets away when the party's weakest has at least E. `strength`: `Flee` monsters run even from COMBAT when N is above the party's best, else they fight |

Reactions (`EncounterMenu`):

| Reaction | The monsters ... |
|----------|------------------|
| `Attack` | fight, whatever the party does (FLEE may get away, see `escape`) |
| `Mirror` | answer in kind: COMBAT fights, WAIT waits, FLEE lets the party go, ADVANCE closes in, PARLAY talks |
| `Flee` | run away (a COMBAT makes them fight unless `strength` says otherwise) |
| `Approach` | come one square closer (COMBAT fights, FLEE lets the party go, PARLAY talks) |
| `Talk` | close in and talk at distance 0 (COMBAT fights, FLEE lets the party go) |

| Command | Does |
|---------|------|
| `PartySurprise A B` / `Surprise A B C D` | roll surprise; `Surprise` is 0 none, 1 party surprised, 2 monsters surprised |
| `PartyStrength D` | the party's strength (hit points, AC, THAC0, levels) into D |
| `Combat` | run the fight with the loaded monsters (or the shop, temple or treasure screen when no fight is pending) |
| `Damage flags dice sides bonus save` | roll damage on one member, everyone (flags 64) or `flags` random members; with flags 128 a saving throw avoids it |
| `Treasure g1 .. g7 Items` | treasure for after the fight: 7 money amounts (copper .. platinum, gems, jewelry), Items an ITEM block (contents in the comment), 128+N for N random items, 255 none |
| `Rob Who Pct x` | take Pct% of the money and items, of the selected member (Who 0) or everyone |

## Party checks

| Command | Does |
|---------|------|
| `CheckParty What Special Min Max Avg Found` | a party census (a skill or special ability) |
| `Spell N Slot Count` | look for memorized spell N |
| `Carried Item` | see Tests (the `FindItem` command) |

Unknown or unnamed things keep their numbers; where a name in `names/`
is a guess, its comment there says so.
