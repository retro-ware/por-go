# MODERNIZED.md: what the port changes from the original, and how to do it again

This is the write-up of everything the Go port of *Pool of Radiance* changes
from the DOS game to streamline and modernize it, with the method, the tools,
the files and the constants, so that another agent can repeat the work on
another Gold Box game (Curse of the Azure Bonds, Secret of the Silver Blades,
Pools of Darkness, the Krynn and Savage Frontier games, which share the same
DAX / ECL / GEO / WALLDEF data formats and the same overlay engine).

It is written from the repository's docs, the git history (448 commits since
2026-10-10) and the code. Where a companion document already holds the
detail, this file says what it is for and points at it:

| Document | What it holds |
|---|---|
| `README.md` | the data layout, how to build and play |
| `PORT_CHECKLIST.md` | the routine-by-routine port status, and under "Extras (not in the original)" and "Idiomatic Go" one line per modernization with its code pointers; the most complete inventory, kept up to date with every commit |
| `PARITY_CHECKLIST.md` | the parity audit of each ported routine against the DOS listing |
| `phlan-poc/PROCESS.md` | the step-by-step recipe for remastering one 3D map (the primary source for section 3) |
| `phlan-poc/MAPS.md` | every 3D map of the game with its ECL, GEO and wall sets, all ticked |
| `docs/remaster-architecture.md` | the design and the milestones of the 3D view integration (the View seam, presenters, the Blobber refactor) |
| `docs/battle-remaster.md` | the design and the milestones of the 3D combat view and the move from Ebiten to `gpu/` |
| `data/assets/scripts/README.md`, `names/README.md` | the text form of the area scripts |
| `phlan-poc/blobber/{README,ROADMAP,CREDITS,reference,op-codes}.md` | the BlobberRPG copy: its map format, its assets and their licences |
| `decomp/emu/README.md`, `.claude/skills/ghidra/SKILL.md` | the reverse-engineering tools |

Nothing in this file replaces the originals' detail; it is the map of what
was done and why, with enough of the how to start from.

---

## 1. Rules that held the whole way

These are the project's standing rules (`CLAUDE.md` in the repo and above
it). Every modernization below was made under them, and a port of another
game should keep them:

- **The classic engine stays authoritative.** `engine/` runs the game's rules,
  scripts, movement, menus and text as the DOS overlays did. The modern
  presentation only reads a published snapshot and sends typed actions back.
  It never mutates game state.
- **No raw memory blocks or offsets.** Every value the engine keeps is a typed
  Go variable or struct field (the DS globals became named fields; records
  such as the character, item, affect, script-variable and save-file layouts
  are structs read with `encoding/binary`). See "Idiomatic Go" in
  `PORT_CHECKLIST.md`.
- **Never scale an asset against its aspect ratio.** Pop-ups, pictures,
  portraits, journal scans and textures are scaled by whole factors or fitted
  with the ratio kept.
- **Game maps never wrap.** The original repeats a town map past its edge for
  most area kinds; the port treats that as a bug and cuts it everywhere.
- **Verify against the reference, not by eye.** Textures, colours, blocks,
  models and animations are checked against the original data (the compare
  sheets of section 3.8, the engine's own tile decoders), not by visual
  impression. Extra features (fog, ambient occlusion, lighting) are fine.
- **Never teleport NPCs**; they walk.
- **One game instance at a time** when benchmarking; tests after the work,
  not continuously.
- **One commit per milestone**, with the checklist updated.
- **Desktop apps are native** (no Electron, Tauri or web shells); the port is
  a Go binary on wgpu, and the browser build is the same code compiled to
  WebAssembly.

---

## 2. Architecture that makes the modernization possible

### 2.1 The engine: a routine-by-routine port of the DOS overlays

`engine/` is a Go translation of `START.EXE` and the overlays of `GAME.OVR`,
one Go function per original routine, named after the original
(`ovr04:0A9F` style references in comments). The game data (`*.DAX`,
`ITEMS`, `POOL.CFG`) is read at run time from `data/dos`; nothing of the
game is in the repo. The rule tables of `START.EXE` (races, classes,
experience, spells) are Go data in `engine/tables.go`.

How the original was read, so another game can be read the same way:

- `decomp/` holds the disassembly and the tools that produced it, all
  from `original/` (`decomp/out/` is not in git): `tools/unexepack.py`
  unpacks `START.EXE` (EXEPACK) to `out/START.bin` with relocations; the
  Borland overlays of `GAME.OVR` are split into `out/ovr/ovr01..35.bin`
  (`out/overlays.json`); `disasm.sh` disassembles them with radare2
  (`r2 -a x86 -b 16`) into `out/asm/*.asm`; `view.py ovrNN LO HI` prints an
  annotated listing (far-call names from `names.json`, string refs, Pascal
  strings); `xref.py` writes `out/xref.tsv` (far-call targets);
  `pstrings.py` lists the Pascal strings.
- **Ghidra** (12.1 from brew) with the **GhidraMCP** 6.0 headless server
  (`~/tools/ghidra-mcp/run-headless.sh`, port 8090 for this game) decompiles
  an overlay blob to pseudo-C as a reading aid. The whole procedure (start,
  `POST /load_program` with `x86:LE:16:Real Mode`, `/run_analysis`,
  `/decompile_function?address=0000:OFS`) is in
  `.claude/skills/ghidra/SKILL.md`, with its caveats: far calls and segments
  are wrong in the unrelocated blobs (take targets from `view.py` or
  `xref.tsv`), Pascal argument order is reversed (`param_1` is the last
  argument), `unaff_BP` means a wrong function start; use the C for control
  flow and port 1:1 from the asm.
- **The Unicorn emulator harness** `decomp/emu/` (unicorn 2.1.4, capstone,
  pillow; a venv from `requirements.txt`) runs the real `START.bin` plus
  every overlay resident at fixed segments, serves DOS/BIOS in Python,
  intercepts the hardware primitives (`prims.py`) and writes a trace
  `[addr, name, args...]` in the record format the Go engine's trace mode
  wrote, so `compare.py` finds the first divergence. It is deterministic
  (`RandSeed` 0x12345678, Turbo Pascal's LCG `seed*0x08088405+1`, virtual
  time from Delays plus 10 ms per Ticks, the copy protection at ovr03:00FD
  skipped). `run.py call ovr25:0DE3 ...` calls one routine from a memory
  snapshot with Pascal-order arguments; `all.sh` runs every scenario of
  `scenarios.txt`, `fuzz.py` random key streams (seeds 1–1000 at 1000 keys
  passed on 2026-10-09). The trace harness on the Go side (`cmd/trace`,
  `cmd/snap`) was retired on 2026-10-10 when classic mode was removed
  (`PARITY_CHECKLIST.md`; the last commit with all 35 scenarios passing is
  651acd2), but the emulator still answers "what does the original do
  here".
- **Original bugs found this way and fixed on purpose** (port them the same
  way): Haste/Slow swapped on the sheet (ovr20:086A); the area+0x3FE
  overlap that cycled the view picture after every save or load; the broken
  "javelin of" record; Appraise losing the gem when the item list was empty;
  the rod's record read past the item table; sky colours 10..15 read past
  the table.
- `cmd/fixture` builds a save fixture that stands the party in any area at
  any square with any script variables (`-slot NAME -block N -walls A,B,C
  -x X -y Y -dir D -set VAR=VAL`), through `engine.SaveEditor` (typed, no
  offsets). Used for traces, tests and screenshots.
- `cmd/ecl DISK BLOCK` lists one ECL script block as opcodes;
  `cmd/geodump DISK BLOCK OUT` extracts a GEO block; `cmd/viewdump` renders
  every wall of every WALLDEF set to PNG (section 3.2); `cmd/sounddata`
  extracts the PC speaker / Tandy sound driver's data from `START.EXE` into
  `engine/sounddata.go`.
- `cmd/por` is the game. Its flags: `-data DIR`, `-scale N` (default 3:
  a 960x720 4:3 window, `pal.W*scale` by `pal.H*scale*6/5`, resizable,
  laid out in device pixels so Retina renders at native resolution),
  `-import DIR`, `-import-disk2 DIR`, `-setup`, `-explore` (debug: no intro, a Quick Start party, no tour, straight into the 3D view). Keys are converted to
  BIOS-style codes (`biosKey`) and then to actions by the presenter; Ctrl-S
  toggles sound, Ctrl-F / Ctrl-Shift-F cycle the font, Ctrl-C quits. The
  gamepad (standard layout): d-pad and left stick walk, A selects, B is
  Esc, X the party overview, Y the minimap, LB/RB move the highlight,
  LT/RT previous/next member, Start Enter. Its env vars for headless work:
  `POR_DEV` (a dev scene from `engine/devviews.go`: shop, temple, appraise,
  combat, wildcombat, castfight, missile, autocombat (the `-battle` flag:
  the computer fights both sides, for measuring the battle view), parlay, cure, portrait,
  sheetcast, cast, castko, who, effects, journal, runes, inventory,
  lockeddoor, loot), `POR_SEED` (fixed dice), `POR_SHOT=file.png` (write
  the window and exit), `POR_KEYS` (scripted keys one every quarter second:
  `\r` Enter, `\e` Esc, `\U \D \L \R` arrows, `\1..\8` F-keys, `\t` Tab,
  `\M \C` right/left click at `POR_POINTER=x,y;x,y`, `\P` next pointer
  position), `POR_PNG` (a dropped portrait), `POR_SCRIPTS_OUT`, `POR_FPS=1` (a frame-rate line a second: where a frame's time goes, and the frame's draws, passes, clears and buffer writes), `POR_NOVSYNC=1` (frames as fast as they come, for measuring), `POR_QUIT=1` (the run ends after the keys: measuring without a shot), `POR_PASSES=1` (one frame's render passes with each fps line), `POR_SKIP=a,b` (parts of the 3D frame left out to time the rest: fill, sky, planes, floors, walls, bills, dither, hud, in a battle table, board, shadows, frames, figures, sparks; clear and back undo two savings for a comparison), `POR_OFFSCREEN=1` (the frame drawn into a texture and copied to the window, as a shot does, instead of straight into the window), `POR_FREEZE=1` (after the keys time stands still, for a grab of the window from outside), `POR_GLITCH=dir` (every frame is read back after its present and the pair is saved as PNGs when more than half of the pixels changed: a glitch of one frame), `POR_CPUPROFILE=file` (a CPU profile of the first 20 seconds), `POR_STACKS=n` (every goroutine to stderr every n seconds). Shot
  windows open unfocused and ignore the user's input while they run (a
  global rule: captures must not take stray keys).

### 2.2 The seam: `engine.View` and typed actions

The whole modernization hangs on one read-only seam out of the engine
(`engine/snapshot.go`, designed in `docs/remaster-architecture.md`):

- The engine publishes an immutable `View` snapshot through an atomic pointer
  every time it polls or waits for input (`publish()`). `View` carries,
  typed: the area (`AreaID` = GEO block), position and facing, what the view
  window shows (`Window`: 3D, picture, area map), whether the main frame is
  up (`MainFrame`), the text pane (`TextShown`, the text cells), the party
  (`Party []Member`: name, HP, AC, status, selection, head, portrait,
  powers), the menu the game waits on (`Menu`: prompt, items, hotkeys,
  highlight; `Command` marks the adventure bar), the day flag (`Day`,
  6:00–20:00), `Inside` (cell byte above 0x7F: the original's red indoor
  sky), the map image (`Map`), the inventory screen (`Inventory`), the
  battle (`Battle`), the spell pick, the member pick (`Choosing`), typed
  input (`Typing`), the clickable regions (`Spots`), the page layers
  (`Page`, `Text`, `Overlays`), and `Working` (an action is still being
  carried out).
- There is **no EGA frame buffer any more**. The engine records what it
  draws as typed layers (pictures, symbols, fills, maps, text cells,
  overlays) and the presenter draws every screen from the `View`. The first
  milestones cropped pop-ups out of the classic 320x200 frame; that frame,
  `present/classic`, `package ega` and the trace hooks were all removed
  later (PORT_CHECKLIST "No EGA frame buffer", "No key channel").
- Input goes the other way as **typed actions** (`engine/act.go`:
  `Action{Kind, N, X, Y, Text}`), never keys. `ActPick`/`ActSelect` pick a
  menu item, `ActContinue` is Enter, `ActClose` Esc, `ActMove` the
  arrows/pages/wheel, `ActMember`/`ActInventory`, `ActMap`/`ActMinimap`/
  `ActOverview`/`ActHelp`/`ActSlots`/`ActFont`/`ActSound`, `ActSpot`/
  `ActHover` for clickable regions, `ActText` for typed lines, and the
  combat and inventory kinds (`ActWalk`, `ActAttack`, `ActCast`, `ActBuy`,
  `ActTakeAll` ...). The ported routines read actions themselves
  (`Engine.input`, `menuBar.decide` in `engine/nav.go`); the presenter
  (`present/remaster/input.go`) turns keyboard, mouse and gamepad into
  them, and `POR_KEYS` goes the same way. Menus hand back typed picks
  (`engine.Pick`, `Choice`), not letters.
- `engine/nav.go` gives every screen one set of navigation keys: arrows move
  the highlight, Enter confirms the highlighted item, Esc picks the menu's
  Exit, F1–F8 pick a member, Up/Down step where the keypad is used, and the
  mouse hovers, clicks and wheels everywhere (section 7.1).
- Pacing: the window polls at 120 Hz and, before drawing, waits up to 8 ms
  for the engine to settle (`cmd/por/main.go settle()`, `Engine.Busy /
  Settled / Idle`), so a key's result shows in the same frame. The
  original's slow text typing (DS:4A00) is gone; text appears at once.

### 2.3 Rendering: `gpu/` on wgpu, no Ebiten

The game, the Blobber copy and the editor all draw through `gpu/`, a package
with the slice of Ebiten's API they use (images, `DrawImage`,
`DrawTriangles`, WGSL shaders in the role of Kage, input, the game loop) plus
`DrawMesh` for depth-tested 3D triangles. It is pure Go on **gogpu/wgpu**
(`github.com/gogpu/wgpu`), with the device code copied from VoxelLib's wgpu
backend. `go.mod` replaces `github.com/gogpu/wgpu` with the local fork
`../gogpu-wgpu` (github.com/memmaker/wgpu) and `github.com/gogpu/gogpu` with
`../goplay/third_party/gogpu` (macOS fullscreen fixes). The window is glfw.
Audio is `github.com/ebitengine/oto/v3` (two players at 44100 Hz s16
stereo: the game's sound and the voice-over); MP3 decoding is
`github.com/hajimehoshi/go-mp3`. The browser build runs on goplay
(https://ruzzoli.de/goplay/); `GOPLAY_DATA` names its data dir and
`GOPLAY_FULLSCREEN=1` starts fullscreen. The palette is `pal/` (the EGA set
and the Amiga palettes).

### 2.4 Data layout and import

`data/dos` (the DOS files the engine reads), `data/amiga` (converted Amiga
graphics and sound), `data/assets/scripts` (the area scripts as text, in
git), `data/assets/fonts` (`*.f8`), `data/assets/speech` (`*.mp3`),
`data/save` (the named save slots, `FONT.CFG`, `MAPNOTES.TXT`,
`SETTINGS.JSON` and the marker files). `original/` keeps the rest of the
DOS install for the decompilation tools only.

- `por -import DIR` finds the game anywhere under DIR and copies what the
  engine reads (`cmd/por/import.go`: the folder holding `TITLE.DAX` and
  `8X8D1.DAX` but not `SKY.DAX`/`BIGPIC1.DAX`, which are Curse of the Azure
  Bonds'; `ITEMS` must come too, or every item has the wrong class; a loose
  `SAVE` and `FONTS` folder beside the game files are taken).
- `por -import-disk2 DIR` finds the Amiga data disk among `.adf` images
  (loose or zipped) and writes its graphics, converted to DOS-layout DAX
  archives, to `data/amiga`, plus `TITLE.PNG` from the PowerPacker-crunched
  intro (section 8).
- The release setup uses **gameimport** (`~/Projects/gameimport`, module
  `github.com/retro-ware/gameimport`, replaced to `../gameimport` in
  `go.mod`; it came from the mm3-go port): it finds a game's files in
  whatever the player hands over (folders, zip/7z/RAR/ARC, ISO/BIN-CUE/GOG
  `game.gog`, FAT and HFS+ images, Inno Setup installers), pure Go, for
  every goplay engine's `-import`. It ships beside `por` and is driven by
  `import.toml` at the repo root: the DOS source is marked by `TITLE.DAX`
  and `8X8D1.DAX` (excluding `SKY.DAX`/`BIGPIC1.DAX`), `*.DAX`, `POOL.CFG`
  and `ITEMS` are copied to `data/dos` upper-cased, then `./por -setup`
  runs; an optional Amiga source copies `*.ADF` to `data/ADF` and runs
  `./por -import-disk2 data/ADF -data data`. Releases come from
  `.github/workflows/release.yml` on `v*` tags (`CGO_ENABLED=0`, macOS
  arm64 and amd64, a Windows 7 build with go-legacy-win7). The Windows exe
  carries the Amiga title picture as its icon: `cmd/por/icon.ico` linked in
  through `cmd/por/rsrc_windows_amd64.syso`, a COFF resource object that
  `tools/winicon` writes from the .ico (`go generate ./cmd/por`); no
  third-party resource tool.
- **No disks**: the DOS game spread one set of DAX blocks over eight disks
  (GEO1..8, ECL1..8, HEAD1..8 ...). `dax.Loader.Block` looks a missing block
  up in the same archive on the other disks, so every block is always
  available and `engine.AreaID` is just the GEO block. Only WALLDEF 23 and
  CPIC 24/152 differ between disks, and the disk asked for still wins.

---

## 3. The 3D exploration view

The original draws the first-person view from 8x8 EGA symbol tiles
(`8X8D1.DAX`) laid out by WALLDEF "walls": each wall value is a 56x64 pixel
front face plus nine other projected parts. The port keeps that data as the
*reference* and renders the same squares in a real-time textured 3D engine
with remastered textures, one texture per original wall art.

### 3.1 Where the renderer came from

The renderer is a **copy of BlobberRPG** (`~/Projects/BlobberRPG`, Felix's own
first-person dungeon crawler), placed at `phlan-poc/blobber/` and modified
there. **The original BlobberRPG is never touched.** The copy was trimmed
(55 unused textures and all painted monster sprites removed; its Dark Ages
UI skin, ThickOne font, `ui/` and `font/` packages and the `cmd/blobber`
demo were later removed too, so the remaster uses only the game's own art)
and then refactored into a library (`docs/remaster-architecture.md` §6,
milestone 3):

- `blobber.Scene` (`scene.go`): `ParseScene(r)` reads a `.map` file (the
  grid and the drawing lines: faces, edges, water, ponds, props, wells,
  floor, ceiling) and hands the other lines to the host.
- `blobber.View` (`view.go`): `NewView()`, `Show(scene, Camera)`,
  `Follow(Camera)`, `Update()`, `Draw(dst)`. It draws the 3D view alone
  into the host's image at that image's size, animates each step and turn
  (`Follow`: a step, sidestep or turn tweens, anything further jumps; moves
  queue up to 8 and the tween carries its speed into them), and never reads
  the keyboard except for its Esc options page (`UpdateOptions`).
- `blobber.Camera`: a cell and a facing (0 N, 1 E, 2 S, 3 W).
- `blobber.Settings` / `OnSettings` / `Voice` hooks for the Esc menu.
- Package state stays behind the View API (one 3D view per process, marked
  `ponytail:` in `view.go` with the upgrade path).
- The module is `blobber v0.0.0`, wired by `replace blobber =>
  ./phlan-poc/blobber` in `go.mod`; `blobber/maps.Levels` embeds the maps
  (`maps/embed.go`), `walls.go` embeds `assets/walls/*.png` and `walls.txt`
  (`FlatTexture(kind, name)` loads `<kind>_<name>.png`, `WallTexture(name)`
  by name). It is a separate module: vet and test the main module and it
  separately.
- **What is no longer in the tree:** the tuning host `cmd/blobber`
  (`host.go` with `BLOBBER_OPEN`, the `T` night key, `main.go`, `game.go`)
  was deleted in commit 187d27f (2026-10-29) with the foreign UI skin.
  `compare/compare.py` still shoots a prebuilt `blobber-poc` binary
  (`$BLOBBER_POC`), so to run the compare loop again restore `cmd/blobber`
  from `187d27f^` and build it. The F10 classic/remaster toggle and
  `present/classic` are gone too (98a39ae, d80a12c). BlobberRPG's look
  follows Deepnight's "Labyrinth" (purple fog, dithered dark bands,
  `look.go`); the copy declares no licence (`CREDITS.md`), and its README
  still says Ebitengine although the copy runs on `gpu` (7679e29).

### 3.2 Reading the original's map data

Everything about a map is in its ECL script and two DAX archives:

1. **Which GEO and wall sets an area uses:** in its ECL
   (`go run ./cmd/ecl DISK BLOCK`): `21 opLoadFiles GEO 2 255` names the GEO
   block, `37 opLoadFiles A B C` the WALLDEF blocks for wall slots 1–3. The
   ECL block number is the area kind (DS:84DC). The entry square is the
   `opSave` into `[C04B]` (x) and `[C04C]` (y). In the text scripts
   (section 6) these are the `LoadMap`/`LoadWalls` lines of `on load`.
2. **The map:** `go run ./cmd/geodump DISK BLOCK phlan-poc/geo<disk>_<block>.bin`
   (repo root). A GEO block is 0x400 bytes after a 2-byte header, four
   planes of 16x16 bytes (`engine/geo.go WallAt/DoorAt/MapCell`):

   | plane | content |
   |---|---|
   | 0x000 | north wall value << 4 \| east wall value |
   | 0x100 | south << 4 \| west |
   | 0x200 | the cell byte (above 0x7F: indoors, red sky) |
   | 0x300 | door bits, 2 per side, N E S W from bit 0 |

   Walls are **thin edges** between squares and **two-sided**: each square
   keeps its own value per side, and the original draws only the viewed
   square's own sides. A wall stored on one side only is invisible from the
   other. Wall value v (1–15) maps to WALLDEF chunk `(v-1)//5+1`, wall
   `(v-1)%5+1`; a WALLDEF block holds chunks of five walls and a block of two
   chunks fills its slot and the next.
3. **The wall art:** `go run ./cmd/viewdump [-scale N] -transparent . viewdump`
   (repo root) writes every wall's front face (part 6 of the ten view parts,
   `engine/geo.go partBase..partH`, 7x8 symbols of 8x8 = 56x64 EGA pixels)
   as `WALLDEF<disk>-<block:03>-<chunk>-wall<n>.png`, NRGBA, 224x256 at the
   default scale 4, assembled from the symbol sets the engine itself uses
   (set 0 is 8X8D1 block 203; chunk k of block id uses `8X8D<n>` block
   `id*10+k`, or `id` itself for a single chunk; `symFix` =
   {0x001,0x02E,0x074,0x0BA,0x100}; symbols masked on colour 13 and drawn
   with `pal.Palette`), sky and ground transparent (without `-transparent`
   the default ground bands are drawn: horizon colour 0, band 7, ground 6).
   210 walls for Pool of Radiance. `phlan-poc/viewdump/` is the contact
   sheet to pick textures from (gitignored, regenerated; the cleaned
   references land in `viewdump/notrim`, `nowater`, `clear`).
4. **The list of maps:** `phlan-poc/MAPS.md`, found by scanning every ECL
   block for `opLoadFiles GEO`. Pool of Radiance has 20 areas on 26 GEO
   blocks (Sokal Keep, Valjevo Castle, the pyramid and Kuto's Well have
   several). The wilderness scripts load the overland tile map instead
   (`DrawAreaMap`), which the port keeps as the original's 2D tile map.

### 3.3 One entry per area: `phlan-poc/areas.py`

Every area is one dict in `AREAS` (the docstring at the top of `areas.py`
lists every key). The important ones:

| key | meaning |
|---|---|
| `geo` | (disk, block) of the GEO; the map file is `geo<disk>_<block>.bin` |
| `walldefs` | (block, slot) loads in the ECL's order |
| `area_kind` | the ECL block number (0 and 10 leave off-map squares empty, the others wrap in the original) |
| `chunks` | the viewdump names of wall values 1–5, 6–10, 11–15 |
| `start` | (x, y, facing) where the party enters, or `None` |
| `water_walls`, `free_walls` | values with the sea behind them; free-standing values (stakes, railings) drawn from the far side too |
| `floor`, `ceiling` | the painted flats (`phlan_dirt`, `sky_phlan`) |
| `walls` | wall value → a **specific description for the image model**: what the wall *is*, not what its pixels look like |
| `notrim`, `water`, `tile`, `noseam`, `arch`, `clear`, `cut` | per-value texture flags (section 3.5) |
| `beds`, `pond`, `props`, `cellprops` | walls that become plants, water, billboards or one prop per room instead of faces |
| `doors`, `nomap` | fixes to the original's data (one-way doors made passable), values left off the minimap (graves) |
| `views` | the comparison shots (x, y, facing), reachable squares only |

`src(name, v)` gives a value's viewdump name; `origin(name, v)` finds the
first area whose wall has pixel-identical original art (SHA-1 of the dump's
RGBA bytes) so its texture is **reused** instead of generated again;
`used(name)` lists the wall values the GEO actually uses. An area with an
empty `walls` dict is filled automatically: `walls = {v: NEW[area].get(v,
"")}` (an empty description means a reuse), `notrim` = all, and the per-area
`FLAGS` at the end of the file are merged in (a FLAGS example:
`kuto=dict(arch={8}, clear={8: [(72,128,150,256),(30,232,194,256)]},
cellprops={(7,7): [("well","floor_kuto_rim",0.3,0.12,0.07,0.37),
("prop","prop_kuto_ladder",0.65,0.42,0.08)]})`). Across the 26 maps the
game uses 236 wall values; only 47 needed new art after the first four
areas.

### 3.4 Building the map: `make_map.py AREA`

Writes `phlan-poc/blobber/maps/AREA.map` and regenerates
`present/remaster/areas.go` (the GEO block → map name table, gofmt'd). The
map is 16 rows of `.` (all floor, one engine cell per PoR square, same
coordinates, so the minimap matches the original's automap), plus:

- `floor NAME` / `ceiling NAME`.
- For every non-zero side: `face X Y SIDE wall_<area>_vNN` (drawn only from
  inside that square, like the original). For every non-door side:
  `edge X Y SIDE` (blocks movement; doors pass). Faces name their texture
  directly; a reused wall names its origin area's file.
- **Free-standing walls** (`free_walls`) get a face on the far side too, when
  the neighbour has none: the original hid what was behind by painting it
  onto the wall, see-through textures need the far row visible.
- **Land** is flood-filled from `start` (or the largest walkable region)
  through open sides and doors; **water** is flood-filled from beyond the
  `water_walls` into squares no party can reach (`water X Y`); off the map
  is water too, but only on maps that have water. A `sea edge` line is
  written when a water wall faces off the map.
- `pond X Y` for values in `pond` (a superellipse pool per square, joined to
  neighbours by a pinched neck, grass tufts on the bank); garden `beds` get
  sixteen low grass billboards on a jittered 4x4 grid; `props` become
  `prop TEX X Y SIZE` billboards where the trunk stands (Nomad Camp trees);
  `cellprops` place one `prop`/`well` per room (Kuto's well: `well RIMTEX X
  Y HALFW HALFD RIMW`, an oval hole with a tiled rim and the ladder down in
  it).
- Phlan sets the `start` line of `world.txt`; other areas are opened with
  `BLOBBER_OPEN=map:NAME:X:Y:DIR`.
- Props from walls (`props`): `trunks(v)` scans the bottom 10 % of the
  dump's columns for trunk runs (a run touching an edge stands at 0 or 1,
  else at its centre / w), height in squares `opaque_rows/h * 64/56`; beds
  get 16 tufts at `x+0.125+0.25*i ± uniform(0.09)`, size `uniform(0.26,
  0.36)`, seeded `Random(y*16+x)`; pond fringes 14 tufts on the
  superellipse, seeded `Random(1000+y*16+x)`.
- **No wrapping, anywhere.** Rays run past the map edge; the town is not
  repeated behind the sea.

The map format `blobber.ParseScene` reads (lines a lowercase word and a
space; grid rows first, `.` is floor; anything else goes back to the host):

```
face X Y SIDE CHAR|TEXNAME [T0 T1 U0 U1]     SIDE 0 N 1 E 2 S 3 W
edge X Y SIDE
nomap X Y SIDE
water X Y   /   pond X Y   /   sea edge      (two words on purpose: one word would parse as a grid row)
prop TEX X Y SIZE [FRONT [SINK]]
well RIMTEX X Y HALFW HALFD RIMW [FRONT]
floor NAME   /   ceiling NAME                (assets/walls/floor_NAME.png, ceiling_NAME.png)
```

`phlan.map` is 934 lines (361 edge, 524 face, 31 water); `kuto.map` has
`well floor_kuto_rim 7.5 7.5 0.3 0.12 0.07 0.37` and `prop prop_kuto_ladder
7.5 7.5 0.65 0.42 0.08`; `nomad.map` `prop prop_nomad_tree 1.0 5.0 1.089`;
`library.map` `pond 4 11`, `prop prop_grass2 3.041 11.871 0.271`, `sea edge`.

### 3.5 Wall textures: `make_textures.py AREA` and `tools/remaster-walls.py`

This is the heart of the graphics remaster. One 512x512 RGBA texture per
original wall art, generated by an image model from the original pixels as a
*reference*, cleaned and finished by script. Run from `phlan-poc/`:

```bash
python3 make_textures.py AREA          # every wall of the area with new art (API calls)
python3 make_textures.py AREA finish   # re-cut everything from raw/AREA/ without API calls
python3 make_textures.py AREA 8 14     # redo single wall values
```

The pipeline, per wall value:

1. **Dump** the original walls (`cmd/viewdump -transparent`; the script runs
   it). The reference is the 56x64 front face, empty parts transparent.
2. **Clean the reference** before it goes to the model
   (`make_textures.py`):
   - `untrim()` for values in `notrim`: removes the EGA teal end trim
     (`(0, 170, 170)`, the outer `w // 14` columns filled from further in),
     which would otherwise repeat on every tile of a long wall.
   - `unwater()` for values in `water`: clears the painted sea (the EGA
     water colours `(82,82,255) (0,0,173) (0,173,173) (82,82,82)`); the
     engine draws real water behind the wall instead.
   - `unclear()` for `clear` boxes: paints over things the engine draws
     elsewhere (Kuto's well hole and ladder become a prop) with the wall's
     commonest colour.
   - `cut()` for `cut` boxes: makes boxes and the river seen between posts
     transparent (Stojanow's railing keeps the railing, drops the river).
3. **Key colour and prompt** (`tools/remaster-walls.py`, imported as a
   module by `make_textures.py` and `make_props.py`):
   - `pick_key(im)` picks magenta `#FF00FF`, green `#00FF00` or blue
     `#0000FF`, whichever the wall's own opaque pixels lean towards least
     (the lowest maximum `keyness`; `keyness(a, key)` = min over the key's
     "on" channels minus max over its "off" channels, for magenta
     `min(R,B) - G`). In practice every Pool of Radiance wall used magenta.
     `keyness` and `generate` come from BlobberRPG's `tools/new-monster.py`,
     which the script loads by path at import time
     (`BLOBBER = ~/Projects/BlobberRPG/tools/new-monster.py`; a
     byte-identical copy is at `phlan-poc/blobber/tools/new-monster.py`) and
     sets `CONFIG["imageConfig"]["aspectRatio"] = "1:1"`. Every script that
     imports `remaster-walls.py` fails without that file, even the
     OpenAI-only ones.
   - `reference()` composites the transparent dump on the flat key colour
     and stretches it to **1024x1024 nearest-neighbour** (the model wants a
     square; the prompt says the reference is stretched).
   - The prompt is `CONTEXT.format(kname, khex) + description`, where
     `CONTEXT` (verbatim in `tools/remaster-walls.py`) says: this is a wall
     graphic from the first-person view of Pool of Radiance (SSI, 1988), the
     reference is the original 16-colour EGA art 56x64 upscaled, the wall
     face seen straight on, one map square wide; recreate it as a modern
     high-resolution remaster, same subject, layout, composition, placement
     and proportions of every element, same colour scheme, painted in rich
     detail with believable materials, subtle weathering and soft even
     lighting, "in a painterly high-fantasy style like late-1980s TSR book
     covers rendered with modern quality"; straight-on frontal orthographic
     view, no perspective, tilt, vignette, text, frame or characters; the
     wall fills the image edge to edge; every area that is flat *key colour*
     in the reference is empty and must stay flat pure *key* with crisp
     edges, nothing drawn there (no sky, clouds, ground, horizon, nothing
     behind or in front of the wall) and the key colour used nowhere on the
     wall itself; then "What the reference shows: " and the area's
     description of that value. Walls in `tile` get the `TILING` sentence
     appended (no trim or pilaster at the edges; the segment repeats side by
     side along a street, so the left edge must continue exactly where the
     right edge ends).
   - **Model:** OpenAI image edits, `POST https://api.openai.com/v1/images/edits`,
     a hand-built multipart request with `model=gpt-image-2`,
     `size=1024x1024`, `n=1`, the reference PNG as `image[]` (`ref.png`), no
     `quality` or other parameter (API defaults), `Authorization: Bearer` with
     the key read from the file `~/.config/blobber/openai_key` (no env var;
     the same key file BlobberRPG's tools use), timeout 600 s; the response's
     `data[0].b64_json` is the image. `gemini_key` beside it drives the
     Gemini path of the proof of concept (`new-monster.generate`:
     `gemini-3.1-flash-image` by default, `$GEMINI_MODEL` overrides,
     `generationConfig` with `responseModalities ["TEXT","IMAGE"]`,
     `temperature 0.5`, `aspectRatio 1:1`, 300 s, three retries); the PoC
     ran six sample walls through both models and the production scripts
     use OpenAI only. `make_textures.py` runs 9 jobs in parallel
     (`remaster-walls.py` 12, `make_sky.py` 3). No cost figure is recorded.
   - Everything the model returned is kept untouched under `raw/AREA/`
     (`vNN_openai.png`, `vNN_reference.png` and `vNN_prompt.txt` beside it),
     so the finishing steps can be re-run with no API cost.
4. **Key-out** (`key_out()`): the key colour becomes transparency. The
   keyness `t` of each pixel against the pure key gives
   `alpha = clip((0.8 - t) / 0.5, 0, 1)` and the colour is un-premultiplied
   from the key (`rgb = (a - (1 - alpha) * key) / alpha`), so edges keep no
   magenta fringe.
5. **Finish** (`finish(v)` in `make_textures.py`, no API), in this order:
   1. Re-detect the key from `vNN_reference.png` (the `KEYS_RGB` colour
      with the most exactly matching pixels) and `key_out`.
   2. `tile` walls (continuous: quays, long streets) are crossfaded at the
      edges (`seamless()`, `b = int(w * BAND)`, `BAND = 0.12`: the last b
      columns fade linearly into the first b, then the first b are dropped)
      so the texture tiles horizontally; every other wall is resized to
      `SIZE = 512` with Lanczos.
   3. `outline()` clears the rows the original leaves empty above the wall
      top (rows where the dump, or its `nowater` version, is entirely
      transparent, mapped by `arange(H)*len(empty)//H`), so walls side by
      side are one height (the models paint into them).
   4. `arch()` for see-through openings (before the seams, so the copied
      stone has none): the model draws them thinner, so the opening is cut
      exactly where the original has it (the dump's transparent area below
      the first opaque row, from `viewdump/clear/` when the value is in
      `clear`, resized nearest); anything the model drew inside the
      opening's box (Kuto's bucket) is kept in a padded box (pad =
      extent//3 + 4); the stone above and any holes are refilled column by
      column from a strip beside the opening (`x0 = 2 %` of the width, band
      `max(8, left - 4 %)`, mirrored back and forth so there is no join);
      the opening's alpha is set to 0; a dark rim `r = int(n * RIM)`,
      `RIM = 0.045`, found by repeated `MaxFilter(3)` dilation, is darkened
      by `0.3 + 0.7 * (dist/r)**0.7`.
   5. Unless the value is in `noseam`, **seams** (`seams()`: `s = max(2,
      int(w * SEAM))`, `SEAM = 0.022`; columns `x < s` at both edges are
      multiplied by `0.45 + 0.4 * x/s`, darkest at the edge, and the column
      just inside is brightened x1.35 as a lit inner edge), the original's
      trick with its teal trim and building edges, so neighbours read as
      separate segments and a pattern that does not line up across the edge
      is not seen. `noseam` is for values a joint would cut through (stakes,
      water, graves).
   6. Write **512x512 RGBA** to
      `phlan-poc/blobber/assets/walls/wall_AREA_vNN.png`.
6. **Contact-sheet** original vs texture with the aspect ratio kept, then
   compare in the engine (section 3.8).

The texture names are referenced directly by the map's `face` lines, so a
wall reused across areas (`origin()`) needs no copy. `assets/walls` holds
only what some PoR map uses (99 files today).

### 3.6 Floors, sky, night, ceilings, water, props

- **Ground:** `floor_phlan_dirt.png` (256x256), a warm brown iterated until
  it matches the original's brown ground; the default `floor.png` and
  `ceiling.png` (256) are BlobberRPG's. No script in this repo makes the
  dirt floor or the day sky pair `ceiling_sky_phlan.png` (a 4x256 RGB
  gradient) and `ceiling_phlan_clouds.png` (512x512 RGBA); they predate the
  2026-10-18 history squash. Flats like them are made with the copy's
  `create_textures.sh` / `tools/new-texture.py` (OpenAI `gpt-image-2`,
  256x256, writes `assets/walls/<kind>_<name>.png`; it also honours
  `$OPENAI_API_KEY` and `$OPENAI_IMAGE_MODEL`).
- **Water:** `make_water.py` (`tile` re-grades `raw/water_openai.png` only)
  generates one image with `POST /v1/images/generations` (`gpt-image-2`,
  1024x1024, no reference; the prompt in the file: seamless harbour water of
  the Moonsea from straight above, painterly high-fantasy texture). `tile()`
  takes the luma (0.299/0.587/0.114, normalised between the 1st and 99.5th
  percentile) and maps it onto the ramp `[16,52,110] [34,92,160]
  [70,140,200] [215,235,250]` (the model's water came out near black; this
  grades it onto the original's harbour blues), then blends it with itself
  rolled by half (weight `(1 - max(|x|,|y|))**0.5`: the original in the
  middle, the shifted copy at the edges) and writes `floor_phlan_water.png`
  512x512. The engine animates it on water cells (uv scale x/2, drifting at
  `time*0.02`), off the map, and as a sea fill to the horizon on maps that
  have water; under open sky the lower half is first filled with RGB
  (40,98,160). Pond water is the same texture with ColorScale (1.25, 1.35,
  1.7) and a mud bank (46,30,16).
- **Sky:** `openSky` is true when the ceiling name starts with `sky`; the
  gradient `ceiling_sky_*` is stretched over the top half and a cloud quad
  (height -6, half-width 90, tile 14 cells, wind `time*0.03`, subdivided
  24, unlit) scrolls with steps, turns and time, never mirrored (the
  original's mirrored stretch was one of the first things replaced).
  **Night:** `make_sky.py` (no arguments, procedural, no API) writes
  `ceiling_sky_night.png` (4x256, top `[4,6,22]` to horizon `[28,40,78]`,
  `t**1.6`) and `ceiling_night_stars.png` (512x512 RGBA, `default_rng(7)`,
  420 stars, brightness `uniform(0.35,1)**2`, 2 px when brighter than
  0.75, warm `(255,250,235)` 70 % of the time else `(200,215,255)`,
  positions wrapped so it tiles), which stand in for the day pair when
  `blobber.Night` is on. The engine's `View.Day` (6:00–20:00, the same test
  the scripts use; the scripts' own check of hour < 14 is patched in
  `opCompare`) sets `blobber.Night = !v.Day` every frame; night lights the
  world with the moonlight tint `nightTint = {0.42, 0.48, 0.72}` on the
  shader's `Warm`.
- **Ceilings:** `make_sky.py ceilings` makes `cave`, `dungeon` and `wood`
  ceilings by an OpenAI edit of `assets/walls/ceiling.png` resized to 1024
  nearest (prompt: "A seamless square texture for a fantasy dungeon crawler
  game: {desc}. Viewed straight on, flat, evenly lit, no perspective, no
  vignette, no text, no frame. It must tile seamlessly in both
  directions."), crossfaded on both axes with a band of n//8 and written at
  256x256 to `raw/ceilings/` and `assets/walls/ceiling_{name}.png`. Those
  three files were removed again in the "only textures a map uses" cleanup:
  the inside ceiling in play is the default `ceiling.png`. `View.Inside`
  (cell byte > 0x7F) swaps the open sky for the ceiling (`SetFlats(floor,
  "")` vs `SetFlats(floor, ceiling)`): a ceiling plane at y = -0.5 with
  torch darkness and fog. `BLOBBER_CEILING=cave|dungeon|wood` closes any
  map for tuning.
- **Props:** `make_props.py [tree|ladder|rim|grass1|grass2] [finish]`
  generates billboards on a magenta key (the prompts hard-code magenta, e.g.
  TREE: "Game sprite on a flat solid magenta (#FF00FF) background: one
  single tall leafy deciduous tree ... faithful to the reference's shape and
  colours"; LADDER, RIM (a tileable row of fieldstones about 1/6 of the
  image tall) and GRASS follow the pattern). `reference(area, v, top)`
  clears the dump's rows above `top*H`, crops to the opaque box, pastes it
  bottom-centred on a magenta square (a 1/8 margin when `top` is set),
  resizes to 1024 nearest and calls `rw.openai`; `finish` keys out, crops
  to alpha > 30 and scales the long side to 512 (`rim`: keeps the rows more
  than 90 % opaque and cuts on the darkest columns so the cut falls on
  mortar). The table in the file:

  | name | texture | reference | top share |
  |---|---|---|---|
  | tree | `prop_nomad_tree` (225x512) | nomad wall 6 (`WALLDEF7-007-2-wall1`) | 0 |
  | ladder | `prop_kuto_ladder` (224x512) | kuto wall 8 | 0.5 |
  | rim | `floor_kuto_rim` (636x231) | kuto wall 8 | 0.9 |
  | grass1, grass2 | `prop_grass1` (512x201), `prop_grass2` (512x157) | library wall 13 | 0.6 |
- **Distance fog** (on by default, open skies only): the texture shader fades
  every pixel toward the sky's horizon colour by distance squared, full at
  128 cells. **Floor seams** were fixed by drawing the floor as one quad over
  the visible cells, cut per cell for the lighting, so pieces share edge
  vertices exactly. **Horizon:** a ground plane reaches 4000 cells round the
  camera under the floor quad (water on maps with water, else the floor), so
  sky meets sea.

### 3.7 Engine features in the Blobber copy (reuse, do not rewrite)

All in `phlan-poc/blobber/blobber.go`, `scene.go`, `view.go` and
`cmd/blobber/host.go` (the tuning host):

- **The PoR projection** (top of `blobber.go`, every change marked
  `phlan-poc` in comments): `focalW = 42.0/88 * 0.75` (0.358; `focal =
  W * focalW`), `wallTop = 0.5 - 64.0/56` (-0.643: walls 64/56 as tall as
  wide, the floor at y = +0.5, the eye at 0, y down), `eyeBack = 0.25`
  (the eye 0.75 squares from the wall ahead, inside its tile, so side
  exits stay visible and turns orbit the centre), `near = 0.05`, `radius =
  8` (ray depth when not under open sky), `tex = 128`. Measured from the
  original's 88 px wide view, where the wall ahead is 56 px and the next one
  24 px. With these the wall tops line up with the original's at every
  depth, which the compare sheets check.
- **Thin walls**: `Faces map[[3]int][]FacePart` (x, y, side), `FacePart{Mat;
  T0,T1,U0,U1; Tex}`, `Edges` for `blocked()`, `NoMap` for the minimap,
  back-face culling (one-sided like the original), a deterministic (sorted)
  draw order so there is no flicker or z-fighting; per-map faces, edges and
  water swapped in on `enter`; an optional `NAME_light.png` light map per
  painted face. The 3D view is a painter's sort of perspective-correct quads
  through `DrawTrianglesShader` (the depth meshes are for the battle view).
- **Fog, torch, haze** (`look.go`): fog colour `{0x16,0x10,0x23}`; torch
  colour `{1.08, 0.94, 0.78}`, indoors power `1.6 * flicker` with
  `flicker = 1 + 0.06 sin(7.3t) + 0.04 sin(13.1t+1) + 0.03 sin(23.7t+2)`,
  `range = 2.4 / sqrt(darkMul)`; under open sky power and range are 1e3 (no
  darkness); dithering is 7 hard bands with a checker of `W/500`; distance
  haze only under open sky, `hazeCells = 128`, `mix(c, HazeCol,
  (d/128)²)`, day `(165,195,220)`, night `(10,32,70)`. The WGSL shader
  `texSrc` takes Mode (shaded / unlit / glow), Pow, Range, Torch, Warm, Inc,
  HasLit, Dither, Center, Focal, Px, Haze and HazeCol.
- **Ponds** (`pondH, pondNeck, pondMudW = 0.46, 0.36, 0.12`, 48 segments),
  **sea edge, beds, cell props** (`Object.At` places a billboard at an
  exact floor position; `Size` the long side in squares, `Sink` lowers it,
  `Front` stands it that far ahead and turns it with the view while the
  party is on its square), **wells** (`Ring`: an oval black hole with the
  rim texture tiled round it in 36 segments) as in 3.4.
- **Tween** (`walkSpeed, turnSpeed, walkAccel = 5.0, 7.0, 70.0` steps or
  quarter turns per second and acceleration; acceleration scales with
  `Speed²`; brakes when `v² ≥ 2a(1-t)`, minimum speed 0.5; with queued moves
  `top *= 1 + len(next)/2`; a sidestep sways; `snapRepeat = 0.15` s when
  smooth movement is off): a turn takes about 13 frames, a step about 16.
- **Shots:** `BLOBBER_SHOT=file.png BLOBBER_OPEN=map:NAME:X:Y:DIR` saves
  frame 30 with no overlay (`BLOBBER_OVERLAY=1` keeps the position text);
  `BLOBBER_NIGHT=1` starts at night. The tuning host was
  `cd phlan-poc/blobber && go build -o $SCRATCH/blobber-poc ./cmd/blobber`
  (restore it from `187d27f^`, see 3.1).
- **Fluid movement** (option, on): while a move key is held the view asks the
  host for the next move halfway through the current one (`View.Flow(held,
  ready)`, true once per move at `t ≥ 0.5` with Fluid and Smooth on and the
  queue empty), so steps and turns flow into each other; `cmd/por` (120
  ticks/s) counts a move key as held only after `holdTicks = 24` (200 ms),
  so a tap is one step or turn, and skips the arrows' typematic repeat
  while the view paces them.
- **Options** (`blobber.Options`, the Esc menu, saved as `SETTINGS.JSON`):

  | field | default | menu |
  |---|---|---|
  | `Smooth`, `Fluid` | true, true | Gameplay: Smooth / Fluid movement |
  | `Speed` | 1 | Walk speed 0.5–2.0, step 0.25 |
  | `MapOff`, `MapSmall`, `MapAlpha` | false, false, 1 | Minimap, size Normal/Small, opacity 0.2–1 |
  | `Deskew` | true | Deskew combat maps |
  | `Zoom` | 0.5 | Zoom speed 0.25–2.0 |
  | `Fullscreen`, `Dither` | true, true | Graphics: Fullscreen (the screen at its own resolution; `-window` overrides it for a run, `-scale` the window's size), Dithered darkness |
  | `Torch` | 1 | Torch intensity 0.5–1.5 |
  | `PicDouble` | false | Picture box Normal/Double |

  Removed from BlobberRPG's page: Smooth monsters, Distance fog (always on),
  normal mapping, light maps. Hotkeys are the first free letters
  (`blobber.Hotkeys`); Left/Right step a value, Enter steps up and wraps;
  the game is dimmed under the menu; it takes the mouse and the gamepad.
  Other hooks the host sets: `blobber.Voice`, `blobber.OnSaveLoad` (acts
  like F9), `blobber.OnMainMenu`.

### 3.8 The compare loop: `compare/compare.py AREA`

The discipline that kept the remaster exact: every change is judged on a
side-by-side sheet, original left, remaster right, numbered views.

- `views` in `areas.py` lists (x, y, facing) in PoR squares. First pass:
  15–20 random reachable squares (`random.Random(seed).sample(land, n)`),
  then targeted views covering every wall value, wall-to-wall joins, doors,
  corridors from the side, map edges, water. Only reachable squares: views
  from water or sea differ by design (the original paints brown ground
  everywhere and draws only ~3 squares deep).
- The script renders the original through a temporary engine test (written,
  run, deleted; it loads the area's GEO, wall sets and area kind from
  `areas.py`), shoots the PoC (`BLOBBER_POC` names the built host, 60 s
  timeout per shot), crops symmetrically and writes `compare/AREA.jpg`.
  Views on the command line write `compare/AREA-check.jpg`.
- After each fix, drop the views that match, keep the rest, add new ones for
  anything suspicious, until the sheet is clean. Then tick the map in
  `MAPS.md`.

### 3.9 Host integration: `present/remaster`

The remaster presenter (`present/remaster/remaster.go`, `Draw(dst, note,
v) Rect`) draws the game with the Blobber view when a scene exists and the
main frame is up with the window showing the 3D view, a picture or text, or
while a member is being chosen. The live path: `scene(v)` looks `v.Area`
up in `areaMaps` (the generated `areas.go`, keyed by GEO block only), calls
`blobber.NewView()` on first use (a failure falls back to drawing the page),
opens `maps.Levels.Open(name + ".map")`, `ParseScene`s it and caches the
scene per name; `cam := blobber.Camera{X: v.X, Y: v.Y, Dir:
int(v.Facing)}`; a new map name calls `view.Show(sc, cam)` (a jump), else
`view.Follow(cam)` (a step or turn tweens, anything further jumps);
`v.Inside` and `!v.Day` set the flats and `blobber.Night`; `animate()` runs
`view.Update()` at 60 ticks/s whatever the frame rate, capped at 6 ticks of
catch-up; then `view.Draw(dst)`, the minimap, the cards and `view.DrawMenu`.
The rest of the screen is the port's own UI (section 7), drawn with the
game's own art: the window border tiles, the 8x8 font, the EGA colours. On
the 3D view:

- The **picture box** (close-ups, the camp fire, shop keepers) is the
  original's pixel art in its own red border, drawn at a whole scale centred
  in the top half (`Esc > Graphics > Picture box Normal / Double`), the text
  box under it, a question's menu under that. The art keeps animating because
  the engine keeps drawing it.
- **Text pop-ups and menus** (`menu.go drawMenu`): a box of the text pane's
  red border cells, the current font green on black, each item's hotkey
  white, the selection on blue. Enter opens the command bar as a vertical
  menu; a question menu opens by itself under the text. Messages that need
  no answer flash on a black band along the bottom edge (one second plus
  80 ms a character, cut to 36 columns); ones to confirm are a bordered box
  in the middle.
- **Party cards** (`cards.go`): the member's face in the game's border (grey
  for unselected), down both edges (first half left, rest right, up to 4+4),
  the hit point bar under it (green, red under a quarter), the experience bar
  (gold) while Shift is held, the status over the face; hover shows a tip
  (name, class, experience), left click selects the member and opens their
  context menu (View, Items, Spells, Magic, Heal, Cure), right click opens
  their inventory. A dropped PNG portrait is drawn at the window's resolution
  (whole-pixel nearest when that fills 90 % of the box, else Catmull-Rom).
- The **minimap** top right (`View.Map`: the engine's one map renderer
  `engine/fullmap.go drawMap`, seen squares, walls, doors, labels, the
  party), with size and opacity options.
- **Options** (Esc): the Blobber options page restyled as the game's menu:
  Resume, Graphics (Fullscreen, Dithered darkness, Light maps, Torch
  intensity, Picture box), Audio (Voice-over), Gameplay (Smooth / Fluid
  movement, Walk speed, Minimap size/opacity, Deskew combat maps, Zoom speed),
  Save / Load (the F9 slots), Back to Main Menu, Quit; kept in
  `data/save/SETTINGS.JSON` (`blobber.OnSettings`).
- **Mode switching:** F10 swapped the classic and remastered presenters in
  the early milestones; the classic frame was removed later, so the remaster
  is the only presenter now and every screen (title, menus, sheets, shops,
  journal, inventory, combat) is drawn from the `View`.

### 3.10 Problems met on the way (check each on a new map)

From `PROCESS.md` §7, the table every new map should be checked against:
walls missing or visible from the wrong side (thick cells → thin one-sided
faces); stretched textures (one texture over two cells → never); wall tops at
different heights (`outline()`, the projection constants); repeated pattern
or teal stripes (`untrim()` + `seams()`, crossfade for `tile`); cut facades
(the "no border at the edges" prompt); flicker on an edge (sorted faces);
mirrored static sky (gradient + clouds); flat water painted on walls
(`unwater()` + engine water); the town repeated behind the sea (wrap
removed); far pier's stakes invisible (far-side faces); all shots identical
(the HUD runs on a scratch image in shot mode); "no monitor found"
(`caffeinate -u -t 1` before shooting); the comparison test hanging (a wall
set the ECL names is missing: leave it out of `walldefs`); the second map
showing the first map's walls (per-world faces); sea around a land-locked
area (off-map water only on maps with water).

### 3.11 Doing it for another Gold Box game

1. Port or reuse the DAX loader, `engine/geo.go` (GEO planes), `WallAt`,
   `LoadWalldef` and `cmd/viewdump` (the WALLDEF format and the 8x8 symbol
   sets are shared by the series; later games add more walls per set and
   bigger symbol blocks, check the part tables).
2. Scan every ECL block for `opLoadFiles GEO` and write the game's `MAPS.md`.
3. Copy `phlan-poc/` (`areas.py`, `make_map.py`, `make_textures.py`,
   `make_sky.py`, `make_water.py`, `make_props.py`, `compare/`) and
   `tools/remaster-walls.py` into the new repo; point `BLOBBER` at
   BlobberRPG's `new-monster.py` (for `keyness`) and keep the keys in
   `~/.config/blobber/`. Start a fresh `AREAS` dict; keep `origin()` so
   repeated walls are generated once. **Rewrite the prompts:** `CONTEXT`,
   `TILING`, the water and prop prompts hard-code "Pool of Radiance",
   Phlan and the Moonsea; the new game's name, setting and art era go
   there, and the key-colour sentences stay as they are. Restore
   `cmd/blobber` from `187d27f^` for the compare loop (or teach
   `compare.py` to shoot the game itself with `POR_SHOT`).
4. Per map: the checklist at the end of `PROCESS.md` (read the ECL, geodump,
   write the `areas.py` entry with a description per wall value, `make_map`,
   `make_textures`, contact-sheet, build, 15+ random reachable views,
   `compare`, fix, recompare until clean).
5. Reuse `phlan-poc/blobber` as the renderer (it is already a library; the
   per-game part is only `areaMaps` in `present/remaster/areas.go`, which
   `make_map.py` writes, and `docs/remaster-architecture.md` milestone 7
   says how to key it per game).
6. Budget: a wall costs one `gpt-image-2` edit at 1024x1024; Pool of
   Radiance needed roughly 100 new arts for 26 maps (plus a few re-rolls
   kept under `raw/`).

---

## 4. The 3D combat view

The combat screen is a 3D tabletop (`present/battle`, design in
`docs/battle-remaster.md`): the map a board on a wooden table, wall blocks
built from the original's tiles, the party and the monsters as **figurines**
cut from their combat icons.

- **Engine seam:** `View.Battle` publishes the ground grid, tiles, units
  (position, facing, side, icon, HP, AC, hit chance), the cursor, the active
  turn (`Turn`: actions, spells, aiming, the weapon in hand), the ask, the
  reach and paths, the area of effect, strikes and effects, and the battle
  log. **Combat is action-driven**: the player's turn (`engine
  playerTurn/turnChoices`) and every aim (`playerAim`) wait for typed actions
  (`ActWalk, ActAttack, ActGuard, ActBandage, ActWait, ActEnd, ActQuick,
  ActTurn, ActCast, ActItems, ActAim/ActPoint/ActClose, ActFlee, ActRanged`);
  the classic combat menu, the Move/Aim key loops and the combat rows of the
  frame are gone. The classic 2D combat page is gone too: the engine no
  longer blits tiles, icons or hit sprites; it notes the snaps.
- **Meshes** (`scene.go`, pure): the map mesh (blocks for the wall tiles,
  the floor), boxes, matrices; `extrude.go` (copied from VoxelLib
  `items/extrude`) turns a sprite into a Minecraft-style slab (half a texel
  of the icon thick, so a PNG icon is as thick as the original 24-pixel
  sprite); `look.go` derives the stone texture at run time from wall tile 3
  (nearly plain stone in both the DOS and the Amiga art, its bevel mirrored
  away), the oak table, the round bases, blob shadows, the board's sides.
  Lit meshes (sun, sky, specular), floor grain and square edges, corner
  occlusion, blue pedestals for the party and red for the enemies.
- **Camera:** XCOM style: turning in eighths with Q/E (`[ ]` at first),
  W A S D or the arrows pan relative to the heading, the wheel zooms
  (`minDist 5`, `maxDist 24`, 15 % times the Zoom option per step, pitch
  0.82 rad), Home recentres, Tab/Shift-Tab cycle targets and jump the
  camera there; it chases only what is out of sight (an eased move toward
  the active unit, the aim or an effect counts in full when the thing is off
  screen or the player pans, not at all inside a comfort box of side 0.6,
  top 0.65, bottom -0.3), so auto-battles do not swing the view.
- **Action bar** (`bar.go`): one button per menu item along the bottom in
  the order Attack, Guard, Items, Cast, Turn, Bandage, Wait, Quick, End
  (= Done+Quit; aiming: Free, Attack, Cancel), 64 px PNG icons in
  `present/battle/icons/` drawn 1:1 with the hotkey letter bottom right
  (the commit that added them, 51d814a, does not record how they were
  made), Attack showing the weapon in hand and Ranged (M) the missile from
  the inventory atlas, buttons 64 px apart with their words on translucent
  black; 1..9 and 0 pick buttons, Enter attacks the aimed one, Backspace
  ends the turn; the arrows stay with the game (manual aim, keypad steps).
  **Weapon tooltips:** the pointer on Attack or Ranged opens the game's menu
  box above the button (`remaster/fight.go` `drawTipOver`, the box shared
  with the card tip as `tipMenu` in `cards.go`) with the readied weapon's
  name, its Look lines and its combat rules, which the engine derives from
  the base item table so they match what the aim code refuses
  (`engine/combat.go` `weaponRules`, published as `InvItem.Rules` on
  `Turn.Weapon` and `Turn.Missile`): "Melee: a foe next to you"; a thrown
  weapon that is no melee weapon (oil, holy water, darts, javelins:
  usage flag 0x10 without 0x04) "Thrown up to N squares, not with a foe
  next to you; one is used up each throw"; a hand axe or spear (0x14)
  "Melee, or thrown up to N squares; a thrown one lands on the ground"; a
  bow or crossbow (0x08 with 0x01/0x80) "Shoots up to N squares with its
  ammunition readied, not with a foe next to you"; a sling (0x0A) the same
  without the ammunition line. N is the base item's range minus one, the
  original's `+C` field. The bar's hover comes from the host's pointer
  (`battle.Presenter.Pointer`), not the window's cursor, so `POR_POINTER`
  captures can script it.
  Mark colours: reach blue, path `{0.1,0.35,0.8}`, free-attack squares,
  targets and a blocked aim red, spell areas orange; pedestals blue
  `{0.22,0.42,0.95}` for the party, red `{0.88,0.2,0.14}` for enemies,
  figures with ambient 0.85 and pedestal specular 0.25; the lunge lasts
  0.35 s.
- **XCOM 2 flow:** a click on a square in reach walks there (the reach drawn
  in blue with a dotted frame, 2 moves a step, 3 diagonally; the walk
  previewed on hover, red where leaving a square gives a foe a free attack),
  a click on a foe aims and attacks (a melee fighter walks up first; the hover
  text gives the chance to hit with one blow); spell areas are washed orange
  round the aimed square by the cast's own test (`engine/aoe.go`, bolts
  walk their own line with bounces); pointing moves the aim at once; one's
  own figure clicked opens the sheet; the active member's card bottom left,
  a left click on it opens the inventory, a right click the sheet.
- **Feedback:** each weapon attack is reported (`Battle.Strike`): the
  attacker lunges, a hit flashes the target red with red sparks, a miss sheds
  grey ones; point lights and sparks for missiles and spells, deaths, the
  weapon-hit sound. Health bars while Shift is held (`health.go`).
- **Deskew** (Gameplay option, on): the original's dungeon combat map is
  skewed (walls drawn as diagonal bands); the port lays it on a square grid,
  each cell a 6x5 block with its west wall a column and its north wall a row,
  doors two-square gaps, formation and party placed accordingly.
- **Custom icons:** a PNG dropped on the icon editor is the member's combat
  icon (`Character.Icon`, saved as `NAME.icon.png`): a square picture is both
  poses, one twice as wide as high is cut in the middle, left ready, right
  action; a figure's size follows the built icon's pixel size, so a sharper
  icon stands on the same pedestal. The icon sheets for Felix's own party
  were generated with **PixelLab** (the `pixellab` skill: `/generate-image-v2`
  Pro with `style_image` = an accepted sheet and `reference_images` = the
  portrait, both poses as one 128x64 two-frame sheet in one call; the
  animation and template endpoints did not work for this; see the memory
  note `pixellab-combat-icons.md` for the dead ends).
- **Combat screen extras** in the engine: the full-width 12x7 view in one box
  (the side pane gone), the current combatant's name/HP/AC/damage on row 23,
  a battle log (Log command, scrollable), Move showing every reachable
  square, fleeing by stepping off the map, Tab-aiming that passes over allies
  for hostile spells, the loot screen after combat as the inventory screen
  with Take / All / Exit.

---

## 5. The voice-over

Story texts, first visits, NPC dialogue, journal entries, tavern tales,
proclamations and the finale are read aloud by a local TTS voice. Everything
is offline, free and reproducible.

### 5.1 What is voiced and how it is keyed

- **Script texts:** `voice/lines.txt` (in git), a `#` header then one text
  per line, exactly the string the engine prints (trimmed), no quoting. 767
  lines: 764 ECL `Text(...)` / `Print(...)` operands (PRINTCLEAR 0x12 /
  PRINT 0x11), in sorted byte order, plus the three finale texts of
  `engine/story.go` appended by hand in the form `speakAll` joins them
  (the screen lines joined with single spaces).
- **Curation:** the scripts hold 1613 print statements with 1415 distinct
  texts; 54 % are voiced. Kept: one-off story events, dialogue, area
  descriptions, first-visit texts. Dropped: prompts and questions ("WHAT
  PASSWORD DO YOU GIVE?"), repeated and generic messages ("THE WAY IS
  BLOCKED.", "YOU FIND SOME TREASURE."), fragments of concatenated prints
  (" SURPRISES YOU.", "YOU SEE A"), empty strings, and all of New Phlan's
  own prints (area 0: Rolf's tour, the harbour master; its tales and
  proclamations are voiced through the journal). The list was made once from
  a dump of every print literal (today: `go run ./cmd/eclscript
  data/assets/scripts` writes them all as `Text("...")` in
  `data/assets/scripts/area*.txt`), trimmed, de-duplicated, sorted and
  filtered by judgement. The extractor itself was not kept; for a new game
  write it as a small cmd in the repo.
- **Keys:** a script text's file is the **FNV-1a 64-bit hash of the trimmed
  text, lower-case hex, no padding** (`engine/voice.go voiceKey`;
  `render.py key()` computes the same with the constants
  `0xcbf29ce484222325` and `0x100000001b3`; `engine/voice_test.go` pins
  `"  YOU ARE IN THE MERCHANTS' CAMP AREA. "` → `5913b4acb1eb5288`). The
  hash is taken on the text **after** the engine's own rewrites
  (`platinumPrices`: "1 PLATINUM PIECE" → "5 GOLD PIECES", because of the
  one-purse change), so `lines.txt` must hold the rewritten text; editing a
  script string silently orphans its recording.
- **Journal texts** are keyed by title with spaces replaced by `_`
  (`Journal_Entry_26.mp3`, `Proclamation_LIX.mp3`, `Tavern_Tale_5.mp3`),
  not hashed. The source is `engine/journal/JOURNAL.TXT` (embedded in the
  binary: `[Title]` starts a text, a blank line separates paragraphs,
  `<FILE.PNG>` is a picture, `#` a comment); what is spoken is
  `title + ".\n" + the paragraphs`. 99 texts (58 entries, the proclamations,
  23 tales).
- **Text normalisation for speech** (`render.py spoken()`, script texts
  only): an all-capitals text is lower-cased, then a capital restored at the
  start, after `. ! ?` plus whitespace and after an opening quote, and a
  lone `i` becomes `I`. Proper names stay lower case (Kokoro pronounces
  them acceptably). No number, abbreviation or Roman-numeral expansion; the
  hash is always taken on the un-normalised text.

### 5.2 Rendering: `voice/render.py`

```bash
python3.12 -m venv ~/.mlx-audio-env
~/.mlx-audio-env/bin/pip install mlx-audio     # mlx 0.32, mlx-audio 0.4.7, misaki 0.9.4 ...
brew install ffmpeg                              # needs libmp3lame
~/.mlx-audio-env/bin/python voice/render.py data/assets   # writes data/assets/SPEECH/*.mp3
```

- **Model:** Kokoro-82M through MLX on Apple Silicon
  (`mlx_audio.tts.utils.load_model('mlx-community/Kokoro-82M-bf16')`).
  On other hardware the `kokoro` PyTorch package has the same voices.
- **Voice:** `am_michael` (American male) for every text; `lang_code='a'`,
  `speed=1.0`; Kokoro takes no style prompt. There is no per-speaker
  casting.
- **Output:** the 24 kHz float audio goes through a temp WAV to ffmpeg:
  `-ar 44100 -ac 1 -b:a 48k`, so `<key>.mp3` is 44.1 kHz mono 48 kbps.
  **44100 matters:** the engine does not resample; `oto` runs at 44100 Hz
  and go-mp3 decodes at the file's rate (mono duplicated to stereo).
- **Incremental:** a file that exists is skipped; delete one to re-render it.
  866 files, about 50 MB, roughly 2.4 hours of speech. Not in git
  (`/data/assets/*` is ignored); render on each machine.
- **Case:** `render.py` writes `GAMEDIR/SPEECH`, the engine reads
  `data/assets/speech`; this works on macOS's case-insensitive APFS. On a
  case-sensitive filesystem rename the folder or fix the script.

### 5.3 Playback in the engine (`engine/voice.go`)

- `speak(key)` is called from the **one** shared text routine
  (`engine/eclops.go scriptText`, every script `Print`/`Text`), from the
  journal pager per page shown (`engine/journal.go showJournal`, so the
  Enter that turns a page silences only the last page's text), from the
  native tavern tale (`engine/native.go tavernTale`) and from the finale
  (`engine/story.go speakAll`). A text with no file is silently not voiced:
  `lines.txt` alone decides what is spoken.
- `speak` returns if the save has heard the key (`e.ext.voiced[key]`) or the
  voice is off; else it reads `data/assets/speech/<key>.mp3`, decodes it with
  go-mp3 to s16 stereo PCM and appends it to a queue; texts play back to
  back. `VoiceReader()` is the audio source, mixed by oto with the game's
  sound player (`cmd/por/main.go`: two players, 50 ms buffers; no ducking;
  Ctrl-S sound off does not touch the voice).
- **Silence:** Enter or Esc (`ActContinue`/`ActClose` in `engine/nav.go`)
  clear the queue; so does Back to Main Menu and switching the voice off.
- **Once per save:** the heard set is gob-encoded to `VOICED.GOB` in the
  save slot folder beside `MAPS.GOB` (`engine/slots.go`); a load restores it.
- **Switch:** on by default; the marker file `data/save/VOICE.OFF` turns it
  off (`engine mark/marked`, shared with `SOUND.OFF` and `MINIMAP.ON`);
  the Esc menu's Audio page has the Voice-over line
  (`blobber.Voice = eng.ToggleVoice` in `cmd/por/main.go`).
- Commits: 244f0ad (the feature, 2026-10-21), 9095161 (switch moved to the
  Esc menu), 06c86bf (journal speaks per page), 58ab530 (on by default),
  8389b7b (tavern tales native and voiced at once).

### 5.4 Reproducing it for another game

1. Dump every print literal of the game's scripts (eclscript), trim,
   de-duplicate, sort, curate to narrative texts; apply the engine's own text
   rewrites first; append any hard-coded engine texts exactly as they are
   joined. Keep the extractor in the repo.
2. Put the game's journal in `engine/journal/JOURNAL.TXT` in the format
   above (section 7.6 says how the port shows it).
3. Hook `speak(voiceKey(s))` into the one shared print routine, the journal
   pager per page, and `stopVoice()` into Enter/Esc; a heard-set per slot;
   a second oto player; the marker file and the options line.
4. Install the MLX environment, render, check `go test -run TestVoiceKey
   ./engine` and that every line has a file.

---

## 6. The scripts as text

The area event scripts (ECL byte code, `ECL<n>.DAX`) were translated once
into readable text by `go run ./cmd/eclscript data/assets/scripts`
(`data/assets/scripts/area<N>.txt`, in git) and parse back to exactly that
byte code (`engine/script_test.go` round-trips every area). The engine parses
them into Go structs at area load and runs those. The format
(`data/assets/scripts/README.md`): brace blocks `on load/move/step/camp/rest
{ }`, `def Name() { }` for called sections, `Name:` labels, `if A op B { }
else { }` with `and/or/has/lacks/Carried/Menu() == "YES"`, `for X = A to B
{ }`, `Text()`, `EnterArea(Area)`. Names come from `names/` (`global.txt`,
`area<N>.txt`, `labels/`, `variables.txt` for value and flag-bit names,
`areas.txt` for "ID SET Name"); every run of statements ends in
exit/goto/return/NewECL.

`names/values.txt` (generated monster, item and shop stock names) and
`names/pictures.txt` (pictures, sprites, icons, heads, bodies, maps, wall
sets, by hand) complete the set.

Why it matters for modernization: the text scripts are where the game's
flow was edited. They are read from disk at every area load (`LoadArea` →
`loadScript` → `ParseScript`, `engine/scriptparse.go`), also for a loaded
game, so re-entering the area or loading a save picks a change up (no
hot-reload key; a broken script is reported on stderr and runs as an empty
one). Changes made in them so far: New Phlan's four shops and three temples
open their menus directly (no Yes/No question first; Exit leaves as 'no'
did), and the tavern-tale block is a comment plus exit. **Native scripts**
(`engine/native.go`): `nativeScripts map[scriptAt{area, label}]func(*Engine)`;
when `eclPC` reaches that label the Go function runs instead and sets
`eclPC` to where the script continues. The hooks so far: `{0,
"TavernGamble"}` (a gold wager on the party purse, house limit 5000 gold in
place of 1000 platinum; a d10: 8–9 green wins, 3–7 blue returns the wager,
0–2 red loses it) and `{0, "TavernTale"}` (the tale is shown and voiced at
once, the "YOU OVERHEAR TAVERN TALE n" line and its wait are gone).

---

## 7. Interface and flow modernizations

Everything below is listed with its code pointer in `PORT_CHECKLIST.md`
("Extras (not in the original)"). Grouped here by what the player meets.

### 7.1 Input and navigation

- One navigation model on every screen (`engine/nav.go`): arrows move the
  highlight (each menu keeps its own, the original shared one; Yes/No keeps
  its preselected No), Enter confirms the highlighted item, Esc picks Exit,
  Up/Down work like Left/Right on bars, Home/End step the member.
- Mouse everywhere (`present/remaster/input.go`, `View.Spots`): hover
  highlights, click picks (a list line twice = Enter), party cards, combat
  squares, the wheel scrolls, the right button is Esc; pickers with no menu
  words (race, class ...) pick on one click; a click ends a typed line.
- One key answers one menu (`remaster.go` `menuFor`): the vertical popup
  remembers the menu it opened for and closes when another arrives, so the
  Enter that picked Bash on a locked door never opens the command bar's
  popup; and `gpu` releases every key and button when the window loses the
  focus (`OnFocus` in `gpu/input.go`), because the releases never arrive
  then and a stuck Shift turned the next click into a head change (`ActHead`
  and `ActPortrait` now act on the sheet alone, `ext.viewing`).
- Gamepad: d-pad walks and moves selections, Y minimap, X overview.
- F1–F8 open member n's inventory anywhere, switch members on the inventory,
  View page, camp, memorize, loot and shop bars; P toggles inventory/View.
- Keyboard help (`?`) at any prompt, grouped by where keys work.
- Down steps backwards in the 3D view with the facing kept (the original
  turned round); a tap of a move key is one move, a hold flows.
- Context-sensitive command bar (`engine/cmdbar.go`): Cast only when the
  selected member can cast; Area is gone (the minimap shows every wall);
  Search is "Sweep ahead" (W) and Look is "Search here" (S), both 3 minutes
  instead of 10; a hotkey inside a word lights in place.
- Resizable window, the picture kept 4:3; fullscreen option.

### 7.2 Maps

- Minimap over the roster (Tab), top right on the 3D view, with every wall
  and unseen squares dim; doors light brown (Amiga 0xA84) instead of cyan.
- Full-screen map (M, or a click on the minimap, `engine/fullmap.go`):
  cursor, legend, Label / Remove per square kept in `MAPNOTES.TXT`; New
  Phlan's shops, temples, inns, taverns and offices come labelled from the
  block's step-event ONGOTO; labelled squares are dots on the minimap.
- Walk on the full map (`engine/walk.go`): the party walks to the cursor
  square, stopping at text, pictures, menus, monsters, a blocked step or any
  key.
- Explored squares are kept per save slot and **per map** (`MAPS.GOB`,
  keyed by data set and GEO block, so New Phlan's three script areas pool on
  one map; an old per-script-area `SEEN.GOB` is converted on load).
- The original's overhead map and Blobber's map screen are gone;
  `DrawAreaMap` (the overland tile map) stays.

### 7.3 Party, inventory, money, characters

- **One party purse** (`engine/money.go`): gold, gems and jewelry belong to
  the party; every coin kind counts as gold and copper (200 copper = 1 gp,
  exact); found money goes straight in; shops, temples, training and
  identifying pay from it; Pool/Share/Take money/Trade/Drop money are gone;
  the scripts see it as the selected member's platinum, gold and copper, and
  platinum prices read as gold (5 gp); "WHO WILL PAY?" never asks.
- **Party inventory** (`engine/pool.go`): no weight or encumbrance (armour
  still slows); members keep only readied items; everything else is one
  shared list of 60 slots (+10 with a bag of holding); slot-less items are
  used straight from it; Give/Trade are gone; the save format is kept (the
  pool rides with the first member's `.itm`).
- **Inventory screen** (`present/remaster/inventory.go`): the member's
  **Eye of the Beholder paper doll** (`present/remaster/doll/doll.png`,
  backpack cut, a bracers box added, the Bolts box replaced by one Missiles
  slot for arrows, quarrels or a thrown weapon) under name, portrait, HP;
  the party inventory with **item icons** from an atlas
  (`present/remaster/items/atlas.png` 20x20 cells, 24 a row, 1273 icons from
  the "OldRPG" icon set plus the "16x16 RPG icons" iron weapons from cell
  912, mapped by `items.rec`: `internal_name` = base item type in hex or
  `type:word` → cell, edited with the external tool `remapper`
  (`~/Projects/remapper`) through `./remap_items.sh` at the repo root
  (`remapper 20 20 atlas.png items.rec`, `s` saves, then rebuild: the files
  are embedded)), tooltips with the Look text and the damage / AC difference
  to what is readied (green/red, one decimal), a scroll bar, items the
  class cannot ready in red, All / Weapons / Armor / Misc tabs, the pool
  sorted by kind and strength;
  everything drawn 1:1 on one canvas of at least 400x250 and scaled by a
  whole factor; the original's menu row and Look/Drop boxes over it. Shop Browse is the same screen in two panes (goods left,
  party items right with sell prices, gold and copper on the border; a second
  click or a drag buys/sells). The loot after a fight is the same screen with
  the treasure as wares (Take, All, Exit).
- **Stacks on the cursor** (`inventory.go` `carried`, `pickUp`, `put`): the
  mouse model of the default inventory screen is a cursor stack, as in
  Eye of the Beholder or Minecraft. A left click on a stack (doll or party
  inventory) picks the whole of it up; its icon and count follow the
  pointer; the next left click drops it where the pointer is: onto the
  doll it is readied, into the party inventory it is unreadied, back on
  its own side it is put back. A right click picks up one piece,
  Shift-right-click half; with a stack on the cursor a right click drops
  one, Shift-right-click half. Shift-left-click readies or unreadies the
  whole stack at once with no pickup; a drag onto or off the doll moves the
  whole stack; Esc puts a carried stack back; a right click on empty space
  closes the screen as before. The carried stack is identified by item type
  and name words, not by `Index`, because the pool is re-sorted and the
  count in the name changes after every drop. The engine side is one
  field: `ActReady.X` is the count (0: the whole stack) and
  `engine/view.go` `readySome` splits that many off into an item of its
  own (a copy of the `Item` with `Count` = n) and readies or unreadies it
  through `swapReady`; a part readied beside a like readied stack joins it
  (`sameStack`: arrows, flasks), an unreadied part joins its like in the
  pool (`addToPool`), and a refused move puts the split back. Shop and
  loot screens keep their own clicks (a second click or a drag buys or
  sells, Shift-click trades without the question). Scripted captures reach
  this screen: `POR_KEYS` `\C` and `\M` are a left and a right click at
  `POR_POINTER`, `\P` moves the pointer on to its next position, e.g.
  `POR_POINTER='900,135;140,820' POR_KEYS='\rL\r\r\r\1iéé\Mé\Pé\Mé'`
  picks one flask of oil off a stack and readies it (1920x1440 window).
- Item names end in Angband-style base stats ("(1d8)", "[7]"); shop Look
  (`engine/itemlook.go`) explains what an item does; goods a buyer cannot
  ready are grey and skipped.
- **Portraits:** Shift-click on the sheet's portrait box takes the next
  paper-doll head; a PNG dropped on the window while the sheet is up becomes
  the member's portrait everywhere (`Character.Portrait`, saved as
  `NAME.png` beside the record and `CHRDATA<n>.png` in a slot).
- **Character sheet** and loot list fill a 4:3 window (`View.TallRow`); a
  Next line shows the experience the next level takes; the roster names a
  member ready for a level in purple.
- Character creation: names keep their case; minimal stats are rerolled
  automatically with a count, or refused when the race/class/age cannot reach
  them; **Quickstart** (`engine/quickstart.go`) offers six template
  members (Thorgrim dwarf fighter, Mirelle human cleric, Aldric half-elf
  cleric/fighter, Sylvaine elf fighter/magic-user, Corwin human magic-user,
  Pip halfling fighter/thief, each with a fixed head and body) run through
  the real `CreateCharacter` with its menus answered automatically; Forget
  Saved Character on the party menu.
- **Paladins and rangers** playable with Curse of the Azure Bonds' rules
  (`cotab-re/ref/coab`): tables capped at paladin 8 / ranger 7, paladin
  saves and lasting Protection From Evil, View Heal and Cure as port-only
  affects, ranger giant damage, join rules.
- A card's context menu (right click on the 3D view): View, Items, Spells,
  Magic, Heal, Cure for that member; the spell pick is a compact "Cast:" list
  on the current screen (`View.SpellPick`) instead of a full page; the
  inventory's and View page's Spells actually cast (the original only
  listed).

### 7.4 Camp, rest, memorize

- Camp: the roster replaces the minimap; F1–F8 pick the member the menus
  act for; the bar lists only what that member can do (Cast, Memorize,
  Scribe, a paladin's Heal and Cure sit directly on it, no Magic sub-menu);
  Alter is Order, Drop, Icon, Exit; Party Order drags a card onto another's
  place.
- Rest menu "Heal Memorize Wait Night|Day Exit" replaces the
  days/hours/minutes editor: Heal works like the later games' Fix (clerics
  cast cures, relearn, natural rest), Wait one hour, Night/Day until
  20:00/6:00; the camp fire animates while waiting.
- **Memorize is one screen**: the spell book with how often each spell is
  memorized on every rest (`Character.Plan`, saved as `NAME.mem`); the plan
  never resets, resting fills it up again; the review list and its question
  are gone; a cast or memorized spell flashes as a toast.
- Day sky from 6:00 to 20:00.

### 7.5 Saves and options

- **Named save slots** (`engine/slots.go`, F9 at any prompt): save new /
  overwrite / load / delete, newest first; each slot is a folder
  `data/save/saves/NAME/` in the original file format (`SAVGAMA.DAT`,
  `CHRDATA1..6.*`, plus `MAPS.GOB`, `VOICED.GOB`, the portraits and icons);
  a load unwinds whatever runs (a menu, a script, a battle) back into the
  game loop with `panic(slotLoaded{})`; saves are atomic (written to
  `.NAME.saving`, renamed once `SAVGAMA.DAT` exists); names are filtered to
  `[A-Za-z0-9 -_!']` and keep their case; no saving in combat; losing the
  whole party returns to the party menu instead of exiting to DOS; the
  original's lettered saves A–J are gone (quitting no longer forces a
  save). The slot records the running area, not the stale one the area block
  names after a script's area change.
- Options kept across runs in `data/save/SETTINGS.JSON`; marker files for
  the Tab minimap, sound and voice; the font choice in `FONT.CFG`.

### 7.6 Text, journal, fonts, messages

- **Journal** (`engine/journal.go`): when the text names a journal entry,
  tavern tale or proclamation (`journalRef` regex), the text itself shows in
  a pager, built in from `engine/journal/JOURNAL.TXT`; the printed journal's
  maps and sketches (`engine/journal/*.PNG`, 300 dpi scans) show inside their
  entries at the window's resolution (`ega.Overlay` → `View.Overlays`), never
  split across pages; the pager fills the 4:3 window with 19 rows a page.
- **Fonts** (`engine/fonts.go`): Ctrl-F / Ctrl-Shift-F cycle the game's font
  and 56 8x8 PC fonts (The Ultimate Oldschool PC Font Pack by VileR, CC
  BY-SA 4.0, `engine/fonts/*.f8`) in real upper/lower case; fonts in
  `data/assets/fonts/*.f8` join the cycle (Dark Queen of Krynn's 8x12
  `GAME.FON` converts with `decomp/tools/dqfon2f8.py`).
- Messages: ones that need no answer flash (a yellow note on the page's
  bottom row, or the black band under the 3D view); ones to confirm are a
  bordered box; refusals ("Nothing to appraise", "cannot cast now") are
  notes, so the menu reopens after them; a failed Pick or Bash on a door says
  so.
- Text appears at once; the prompt row is never drawn on the 3D view.

### 7.7 Title, main menu, shops, temples

- The remaster's main menu shows the Amiga painted title (section 8) with
  the lettering and functions fading in over it; the credits page.
- Shops and temples open their menus straight after the greeting (script
  edits); the greeting stays on the pane; the Buy list opens on the first
  usable item, names the buyer, F1–F8 change them.
- One member-pick screen for every "whom?" question (`View.Choosing`): the
  3D view with the cards and the question in a pop-up; a card click, F1–F8
  or the cursor keys answer, Esc cancels.

---

## 8. Amiga assets and sound

The DOS game's graphics are 16-colour EGA; the Amiga version has 32-colour
art for the same blocks. The port can play with the Amiga art and music:

- `por -import-disk2 DIR` (`cmd/por/import.go importDisk2`, packages
  `adf` and `amiga`): scans for `.adf` files, loose or zipped (`adf/adf.go`
  reads OFS/FFS); the POOLDATA disk is the one with at least 8 known
  archives; disk 1's `program` becomes `SOUND.BIN` (the memory image
  `amiga.Base..End` the music player runs on) and its `intro` becomes
  `TITLE.PNG` (`amiga/title.go` decrunches PowerPacker; hunk 5 holds 4
  bitplanes of 320x200 plus 16 0x0RGB colours); each Amiga archive
  (`dax/amiga.go`: a u16 BE index size, 10-byte BE entries, ByteKiller
  crunched blocks, the decruncher at 0x63346) is converted to a DOS-layout
  DAX by `dax.AmigaImage` / `AmigaSprite` + `WriteDOS` with the palette it
  is drawn in (`pal.AmigaView` 0x54080 for PIC and FINAL; `AmigaPortrait`
  0x54040 for HEAD, BODY, SPRIT; `AmigaTerrain` 0x54020 for ICON, COMSPR,
  CPIC, CHEAD, CBODY and the split two-tile ground sheets BACPAC, SQRPACI,
  WILDCOM, DUNGCOM, RANDCOM; the part table at 0x8B10E gives each icon
  part's light/dark slots). The 8x8 3D-view symbols stay DOS: the Amiga set
  is laid out for its own wall renderer. The loader prefers `data/amiga`
  (`dax.Loader.UseAmiga`); `pal.AmigaSaturation = 1.4` pushes the colours
  toward the vivid EGA set; Amiga heads are centred (7 cells in the 11-cell
  slot). `build_n_run.sh` runs the import from `$ADF_DIR` (default
  `~/Downloads`).
- Icon colours recolour Amiga icons too (EGA colour numbers map one-to-one
  onto the Amiga icon palette slots); the Amiga has one combat head per
  picture used for both poses.
- **Music:** `amiga/sound.go` is the Amiga effects player (hunk 33,
  0x90172) and Wally Beben's music player (hunk 27, 0x638a0) translated
  instruction by instruction over the program's own memory (hunks 27–33),
  with the gotos unrolled into named steps, driving a Paula model
  (loop-wrap timing, per-sample averaging of held bytes, the A500 RC
  low-pass); `golden_test.go` hashes the chip writes and audio of every song
  and effect before and after. It is used when `data/amiga/SOUND.BIN`
  exists; `amigaFx` maps the DOS PlaySound numbers 2..12 to the Amiga
  effects `{5,9,8,10,12,11,6,11,4,7,0}`; `Music(n)`: 1 title/camp, 2
  battle, 3 party menu, 5 party killed. `cmd/amigasongs SOUND.BIN OUTDIR
  [SECONDS]` renders songs 1–7 to WAV for listening. Extra music: song 6
  loops on the overland map (`areaMusic`) and song 4 plays on first
  entering Phlan (the original never started them: nine `playMusic`
  callers were audited, none starts 4 or 6). A goplay volume fader puts the
  Amiga music on the music fader and the rest on sfx.
- **DOS sound:** `engine/sound.go` is the PC speaker and Tandy SN76489
  driver (START segment 08AE, a 236.7 Hz tick), its data extracted by
  `cmd/sounddata original/START.EXE > engine/sounddata.go`. Ctrl-S toggles
  sound (`SOUND.OFF`).

---

## 9. Tool index

| Tool | Where | What |
|---|---|---|
| `cmd/por` | repo | the game; `-import`, `-import-disk2`, `-setup` (writes the embedded text scripts and the cover); `POR_*` env for headless shots and dev scenes (`engine/devviews.go`) |
| `build_n_run.sh` | repo root | build and play |
| `cmd/eclscript` | repo | ECL → text scripts (`data/assets/scripts`) |
| `cmd/ecl` | repo | list one ECL block as opcodes |
| `cmd/geodump` | repo | extract a GEO block |
| `cmd/viewdump` | repo | render every WALLDEF wall to PNG (`-transparent`, `-scale`) |
| `cmd/fixture` | repo | build a save that stands the party anywhere (`-slot`, `-block`, `-walls`, `-x -y -dir`, `-set`) |
| `cmd/sounddata` | repo | the DOS sound driver's data → Go |
| `cmd/amigasongs` | repo | Amiga songs → WAV |
| `phlan-poc/areas.py` | repo | one entry per 3D map |
| `phlan-poc/make_map.py` | repo | GEO → `.map` + `present/remaster/areas.go` |
| `phlan-poc/make_textures.py` | repo | wall textures (OpenAI edit, finish) |
| `tools/remaster-walls.py` | repo | reference + key colour + prompt + OpenAI/Gemini call + key-out (module of the above) |
| `phlan-poc/make_sky.py`, `make_water.py`, `make_props.py` | repo | night sky, ceilings, water, billboards |
| `phlan-poc/compare/compare.py` | repo | original vs remaster sheets |
| `phlan-poc/blobber/cmd/blobber` | repo, at `187d27f^` only | the renderer's tuning host (`BLOBBER_*` env), which `compare.py` shoots |
| `phlan-poc/blobber/create_textures.sh`, `tools/new-texture.py` | repo | flat textures (floors, skies) with `gpt-image-2` at 256x256 |
| `phlan-poc/blobber/tools/new-monster.py` | repo (a copy of BlobberRPG's) | `keyness`, the Gemini call |
| `import.toml` | repo root | what gameimport copies and runs |
| `remap_items.sh` | repo root | opens the item atlas in `remapper` |
| `.github/workflows/release.yml` | repo | the release builds |
| `decomp/tools/{unexepack,view,xref,pstrings,dqfon2f8,fon2f8}.py`, `disasm.sh` | repo | unpack, disassemble, list, convert fonts |
| `voice/render.py`, `voice/lines.txt` | repo | the voice-over |
| `decomp/tools/view.py`, `decomp/out` | repo (out not in git) | disassembly listings |
| `decomp/emu` | repo | the original under Unicorn, traces, single-routine calls |
| Ghidra + GhidraMCP | `~/tools/ghidra-mcp`, `.claude/skills/ghidra/SKILL.md` | decompile overlays to pseudo-C |
| BlobberRPG | `~/Projects/BlobberRPG` (never modified) | the renderer's origin; `tools/new-monster.py` (Gemini, `keyness`), `new-texture.py`, `monster-sprites.sh`, `flux.sh` |
| gogpu-wgpu fork | `~/Projects/gogpu-wgpu` (github.com/memmaker/wgpu) | the wgpu backend `gpu/` builds on |
| goplay | `~/Projects/goplay` | the browser host; `third_party/gogpu` with the macOS fixes |
| gameimport | `~/Projects/gameimport` | finds a game's files in any container |
| goldbox-editor, por-re | `~/Projects/goldbox-editor`, `~/Projects/por-re` | the editor on `gpu/`; the earlier reverse-engineering notes |
| API keys | `~/.config/blobber/openai_key`, `gemini_key` | image generation |
| MLX TTS | `~/.mlx-audio-env` | Kokoro rendering |
| PixelLab | the `pixellab` skill (bundled CLI) | custom combat icon sheets |
| remapper | `~/Projects/remapper` | edit the item atlas mapping (`remapper 20 20 atlas.png items.rec`) |
| cotab-re | `~/Projects/cotab-re` (`ref/coab`) | Curse of the Azure Bonds' rules for paladins and rangers |

---

## 10. Order of work for another Gold Box game

1. **Read the original** with the decomp tools (listings, Ghidra, the
   emulator) and port the overlays routine by routine into `engine/`, typed
   records from the start. Keep a `PORT_CHECKLIST.md` and tick per routine.
2. **Publish `View` and read actions** from the first screen on
   (`engine/snapshot.go`, `engine/act.go`, `engine/nav.go`): no frame
   buffer, no key channel. Every modernization sits on this seam.
3. **Scripts to text** (`cmd/eclscript`, names files) so the flow can be
   edited and the texts listed.
4. **The 3D view** per section 3: MAPS.md, areas.py, make_map,
   make_textures, compare, one map at a time, each one committed and ticked.
5. **The combat tabletop** per section 4 on `View.Battle`.
6. **The voice-over** per section 5 (needs the text scripts and the journal).
7. **The UI extras** of section 7 in the order a player meets them: slots,
   navigation and mouse, maps, inventory and purse, camp and memorize,
   journal and fonts, title and options.
8. **Amiga assets** if the game has an Amiga release (section 8).
9. Keep the rules of section 1, the compare sheets and the tests
   (`go test ./...` in the main module and in `phlan-poc/blobber`), and
   commit per milestone with the checklist line that points at the code.
