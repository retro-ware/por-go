# Area scripts

`area<N>.txt` is the event script of area N (the area numbers and names
are in `names/areas.txt`). They were written once from the game's ECL byte
code by `cmd/eclscript` (`go run ./cmd/eclscript data/assets/scripts`) and
parse back to exactly that byte code (`engine/script_test.go` checks every
area). The engine parses them into Go structs at area load and runs
those. `names/` holds the variable, label and value names
(`names/README.md`).

## Layout

```
# a comment runs to the end of the line

on step {                        an entry point
    Sky = 11                     one statement a line (or ; between)
    if Facing == South { exit }  a block in braces
    goto Done                    indentation means nothing
}

def PressToContinue() {          a section only called (Name())
    ...
    return
}

Done:                            a label: a jump target
    exit
```

Every script has five entry points:

| Block     | Runs when |
|-----------|-----------|
| `on load` | the area loads (map, walls, sky; also after `NewECL`) |
| `on move` | before each move, after the party's command (and after an area change) |
| `on step` | after each step, and when the party looks or searches: the square's events |
| `on camp` | the party camps, before the camp menu |
| `on rest` | after the camp menu (rest interruptions) |

The script is one sequence of commands; the blocks and labels only name
places in it. Execution runs on from a block's last line into whatever
follows it unless that line is `exit`, `goto`, `return` or `NewECL`
(`} # falls into Name` says so where it happens). A label inside a block
is one only jumps from inside the block reach.

## Statements

| Statement | Does |
|-----------|------|
| `exit` | end the event |
| `goto Name` | continue at the label |
| `goto [A, B, C][V]` | go to the V-th label (0 the first); past the list, continue |
| `Name()` | call section Name (to its `return`); `gosub [A, B][V]` calls the V-th |
| `return` | back to the caller |
| `Text("...")` | clear the text window and write the text (or a variable's number) |
| `Print("...")` | write on after the text shown |
| `NewLine()` | text cursor to the next row |
| `Name(a, b)` | a command (below); a labelled operand reads `count=8` |
| `exit # the original runs on into data at XXXX` | the byte code ran off the end of the code; the port stops |

## Operands

| Form | Meaning |
|------|---------|
| `12` | a number (a byte) |
| `1000`, `0x0010` | a word (a number over 255, or in hex) |
| `-4` | a byte read as signed (252) |
| `Name` | a variable, label, table or area by name |
| `V_4A07` | a variable by its script address, where it has no name |
| `&Name` | the variable read as a word rather than a byte (or the other way round) |
| `"TEXT"` | a string (Go quoting) |
| `$Name` | a string variable |
| `Kobold` | a named number: a monster, item, picture, sprite, icon, map, wall set or area, by the operand's kind (`names/values.txt`, `names/pictures.txt`, `names/areas.txt`) |
| `SelSide = PartyControlled` | a number beside a variable by the name of that variable's value (`names/variables.txt`): after `=`, in a comparison, or in a mask |
| `Seen\|Done`, `~Seen`, `~(Seen\|Done)` | a mask by its bits' names: the bits, or all but them |

`Scratch1` .. `Scratch10` are temporary words every section reuses;
`Aux...` words are the engine's temporaries.

## Assignments

| Statement | Does |
|-----------|------|
| `D = V` | D gets V (a number, variable or string) |
| `D = A + B` | addition, also `-` `*` `/` (integer, x/0 = 0) |
| `D = A & B`, `D = A \| B` | bitwise and, or (they also set the flags, see Tests) |
| `D += A`, `D -= A` | add to D, take from D |
| `D \|= Bits`, `D &= ~Bits` | set bits of D, clear them |
| `D = T[I]` | entry I of table (or variable row) T |
| `T[I] = V` | set entry I of T |
| `D = Random(N)` | a random number 0 .. N |
| `D = PartyStrength()` | the party's strength (hit points, AC, THAC0, levels) |
| `D = InputNumber(N)`, `$D = InputString(N)` | ask for a number or text (N is not used by the port) |
| `D = Parlay(a, b, c, d, e)` | the HAUGHTY/SLY/NICE/MEEK/ABUSIVE menu; D gets the chosen attitude's operand |
| `D = Menu("A", "B")` | a one-line menu; D gets the answer, 0 for the first |
| `D = VMenu("PROMPT", "A", "B")` | a vertical list menu |

Words are 16 bits and wrap; many variables are bytes.

## Tests

```
if Gold >= 100 { goto Rich }        one statement
if Facing == South {                or several
    Sky = 8
    exit
} else {
    Sky = 9
}
```

| Test | True when |
|------|-----------|
| `A op B` | op is `==` `!=` `<` `>` `<=` `>=`; strings compare as text |
| `A == B and C == D` | both equal |
| `A != B or C != D` | either differs |
| `Carried(Item)`, `not Carried(Item)` | someone in the party carries it (or nobody does) |
| `X has Bits`, `X lacks Bits` | some of the bits are set in X, or none (`Scratch10 = X & Bits` and the test on it; Scratch10 keeps the masked bits) |
| `Ask() == "YES"` | calls section Ask, which asks a `Menu` and returns comparing the answer, and tests the answer |
| `(==)` | the flags as the last comparison left them (made elsewhere: in a called section with several outcomes, or before a jump here) |

A test compares and then guards each statement of its block: in the byte
code every command of the block has its own If, so a called section in
the block may change what the statements after it see. The comparison is
left out when the flags already hold it: after `if A == B { ... }`, `if A
< B { ... }` is only the If; after `Name()`, a test names the comparison
the section returned with. `Compare(A, B)` (also `CompareAnd`,
`FindItem`) is a comparison on its own: a section ends in one for its
caller to test, and one stands before an `if` where the byte code redid
it.

## Loops

```
for Scratch1 = 0 to 5 { ... }               X = A, the body, X += 1, back while X <= B
for Scratch1 = 41 while Scratch1 < 54 { ... }  the same, back while X < B
```

The body runs at least once; the loop leaves the flags comparing X and B.

## Flow and the view

| Command | Does |
|---------|------|
| `Call(0x2C90)`, `CallRedraw()` | run an engine routine the original called by address (`names/global.txt` names them) |
| `Program(N)` | open a game screen: 0 party menu, 9 encamp |
| `NewECL(Area)` | switch to another area's script (same data set) |
| `EnterArea(Area)` | switch to an area in another data set (`SymbolSet = set` + `NewECL`) |
| `Delay()` | pause for the text speed |
| `Clock(N)` | advance the clock N minutes |
| `Picture(Name)` | show a picture in the view; `Picture(None)` puts the 3D view back |
| `ShowPortrait(Head, Body)` | show a two-part portrait (`Portrait = Head` + `Picture(Body)`); `ShowPortrait(None, None)` clears it |
| `Portrait = Head` | while a head is set (not `None`), a `Picture` or an encounter's close-up is the BODY under that head (written `body=`) |
| `RedrawView()` | redraw the 3D view after a picture or sprite |
| `ClearBox()` | blank the view window for text |
| `Sky = N`, `SkyInside = N` | the sky colours |
| `LoadMap(Map, unused=X, icons=Y)` | load map (GEO block) Map; `Keep`/`Unchanged` keep the current one. `unused` is ignored, by the original too. `icons` not 255 loads the wilderness travel icons outdoors |
| `LoadWalls(A, B, C)` | load the wall sets (WALLDEF blocks) into wall slots 1-3; `None` empties a slot, `Reset` in the first resets them |
| `Protection(Base)` | draw the glyph string stored in the variables from Base on |
| `Who("PROMPT")` | choose a party member (the selected member after) |
| `SelectMember(N)` | select party member N (+128: also remove them if they died) |

## Monsters and fights

| Command | Does |
|---------|------|
| `ClearMonsters()` | forget the loaded monsters and treasure |
| `LoadMonster(M, count=N, icon=I)` | add N monsters M (a monster record), drawn with combat icon I (the icon's CPIC block) |
| `AddNPC(M, Morale)` | monster M joins the party |
| `SetupMonster(S, distance=D, closeup=P)` | show sprite S coming at the party from up to D squares away (fewer if a wall is nearer), P the close-up picture once it is near; `body=P` when a portrait is up |
| `Approach()` | the monster comes one square closer |
| `EncounterMenu(S, distance=D, closeup=P, result=R, combat=, wait=, flee=, advance=, parlay=, near=, mid=, far=, escape=E, strength=N)` | the COMBAT/WAIT/FLEE/ADVANCE (PARLAY when near) approach menu for sprite S, coming from D squares like `SetupMonster`. R gets 0 nothing, 1 fight, 2 the party got away, 3 talk. `combat` .. `parlay` are how the monsters react to each choice (below). `near`/`mid`/`far` are the texts shown at distance 0/1/2; a text left out is empty and falls back to the others. `escape`: against `Attack`, FLEE gets away when the party's weakest has at least E. `strength`: `Flee` monsters run even from COMBAT when N is above the party's best, else they fight |
| `PartySurprise(A, B)`, `Surprise(A, B, C, D)` | roll surprise; `Surprise` is 0 none, 1 party surprised, 2 monsters surprised |
| `Combat()` | run the fight with the loaded monsters (or the shop, temple or treasure screen when no fight is pending) |
| `OpenShop(Item, Item*20, ...)` | the shop selling those items: `Treasure` with the ITEM block of exactly those items (`*20`: 20 of it), `Shop = 1`, `ShopFlag = 1`, `ShopKind = 16` (the last two are never read) and `Combat()` |
| `OpenTemple()` | the temple (`Temple = 1`, `Combat()`) |
| `Damage(flags, dice, sides, bonus, save)` | roll damage on one member, everyone (flags 64) or `flags` random members; with flags 128 a saving throw avoids it |
| `Treasure(copper=, silver=, electrum=, gold=, platinum=, gems=, jewelry=, items=)` | treasure for after the fight; an amount left out is 0. `items=[A, B*2]` the ITEM block holding exactly those, `items=Random(N)` N random items, left out none |
| `Rob(Who, Pct, x)` | take Pct% of the money and items, of the selected member (Who 0) or everyone |

Reactions (`EncounterMenu`):

| Reaction | The monsters ... |
|----------|------------------|
| `Attack` | fight, whatever the party does (FLEE may get away, see `escape`) |
| `Mirror` | answer in kind: COMBAT fights, WAIT waits, FLEE lets the party go, ADVANCE closes in, PARLAY talks |
| `Flee` | run away (a COMBAT makes them fight unless `strength` says otherwise) |
| `Approach` | come one square closer (COMBAT fights, FLEE lets the party go, PARLAY talks) |
| `Talk` | close in and talk at distance 0 (COMBAT fights, FLEE lets the party go) |

## Party checks

| Command | Does |
|---------|------|
| `CheckParty(What, Special, Min, Max, Avg, Found)` | a party census (a skill or special ability) |
| `Spell(N, Slot, Count)` | look for memorized spell N |
| `Carried(Item)` | see Tests (the `FindItem` command) |

## Tables

```
# tables
DuelMonsters = [Nomad, Buccaneer, 7]
```

The tables come after the code, `Name = [bytes]`; `T[I]` reads them. A
table named `...Monsters` or `...Icons` has its entries by name.

Unknown or unnamed things keep their numbers.
