# por-go

A Go port of *Pool of Radiance* (SSI, 1988), with a modernized interface.
`MODERNIZED.md` describes everything the port changes from the original
(the 3D views, the voice-over, the interface) and how to do it again for
another Gold Box game; `PORT_CHECKLIST.md` lists each change with its code.

No game data is included. The engine reads the original game files (the
`.DAX` files, `ITEMS`) from your own copy of the game at run time. The rule
tables of `START.EXE` (races, classes, experience, spells ...) live in
`engine/tables.go`.

To set up a release: run `gameimport` in the folder beside `por` and pick
your copy of the game (its folder, a zip, a CD image or the GOG installer).
It copies the files `por` reads into `data/`, and optionally converts the
Amiga disks' graphics. `por` plays from `data/` in its working directory.

To build and play from the repo: put your game files in `data/dos` and run
`./build_n_run.sh` (it builds `./por`). The data folder:

| Folder | What | In git |
|---|---|---|
| `data/dos` | the DOS game files the engine reads (`*.DAX`, `ITEMS`, `POOL.CFG`) | no |
| `data/amiga` | the Amiga graphics and sound, converted from its disks (`por -import-disk2`) | no |
| `data/assets/scripts` | the area event scripts as text, with their names files | yes |
| `data/assets/fonts`, `data/assets/speech` | extra fonts (`*.f8`), the voice-over (`*.mp3`) | no |
| `data/save` | the saves, `FONT.CFG`, `MAPNOTES.TXT` (written by the game) | no |

`original/` (not in git) keeps the rest of the DOS install, which the
engine never reads: `START.EXE`, `GAME.OVR`, the DOS graphics the Amiga
files replace in full, ... The decompilation tools (`decomp/`, `exe`,
`cmd/sounddata`) use it.

The 8x8 fonts in `engine/fonts` are from The Ultimate Oldschool PC Font Pack
by VileR; see `engine/fonts/LICENSE.txt`.
