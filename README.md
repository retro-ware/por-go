# por-go

A Go port of *Pool of Radiance* (SSI, 1988), with a modernized interface.

No game data is included. The engine reads the original game files (the
`.DAX` files, `ITEMS`) from your own copy of the game at run time. The rule
tables of `START.EXE` (races, classes, experience, spells ...) live in
`engine/tables.go`.

To set up a release: run `gameimport` in the folder beside `por` and pick
your copy of the game (its folder, a zip, a CD image or the GOG installer).
It copies the files `por` reads into `data/`, and optionally converts the
Amiga disks' graphics. `por` plays from `data/` in its working directory.

The 8x8 fonts in `engine/fonts` are from The Ultimate Oldschool PC Font Pack
by VileR; see `engine/fonts/LICENSE.txt`.
