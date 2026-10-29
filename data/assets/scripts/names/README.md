# ECL variable names

`cmd/eclscript` reads these when it writes `../area<N>.txt`
(`go run ./cmd/eclscript data/assets/scripts` from the repo root):
`global.txt` names words for every area, `area<N>.txt` names one area's
words (over global) and `labels/area<N>.txt` its jump labels. Lines are `XXXX Name  # evidence`: the script
address in hex, a PascalCase name, an optional comment.

The engine reads the same files (without the labels) when it parses the
scripts, and also:

- `areas.txt`: the areas, `ID SET Name` (NewECL's number, the data set
  holding it). `EnterArea Name` and `NewECL Name` use these.
- `values.txt`: named numbers made from the game data by `cmd/eclscript`
  (monsters, item types), `Kind Set Number Name`. Do not edit; it is
  rewritten.
- `pictures.txt`: named numbers by hand (pictures, sprites, combat icons,
  portrait heads and bodies, encounter reactions),
  same format; set 0 is every set.
- `variables.txt`: by hand, the values and flag bits of variables, the
  variable's name as the kind (`CastleFlags 0 1 Disguised`).
- `values.txt` also has the shops' stocks, `Stock Set Block Item,Item,...`.

A command's operand of a kind (`LoadMonster`'s monster and icon,
`Picture`, `SetupMonster`'s sprite and picture, `FindItem`'s item,
`ShowPortrait`'s head and body, `LoadMap`'s map, `LoadWalls`' wall sets)
reads as the name of its value. `../README.md` explains the commands.
