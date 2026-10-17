# Remastering a Pool of Radiance map in the Blobber engine

How New Phlan (GEO3 block 0) and Valhingen Graveyard (GEO4 block 10) were
brought into the BlobberRPG copy in `phlan-poc/blobber/`, written as a recipe
for every other map. Run everything from `phlan-poc/`. `~/Projects/BlobberRPG`
is never touched; all engine changes live in the copy.

Every area is one entry in **`areas.py`** (GEO block, wall sets, area kind,
start, wall descriptions, texture flags, comparison views). The scripts take
the area name as their first argument:

```bash
python3 make_map.py graveyard
python3 make_textures.py graveyard          # or: graveyard finish / graveyard 8 14
python3 compare/compare.py graveyard        # -> compare/graveyard.jpg
```

## 0. Rules that held the whole way

- **Exact layout.** One engine cell per PoR square, 16x16, same coordinates.
  The minimap must match the original's automap.
- **Never stretch.** No texture is ever scaled against its aspect ratio, and
  no texture spans two cells. If something looks wrong, explain the cause
  first, then change it.
- **Compare, don't guess.** Every change is judged on the side-by-side sheet
  (step 6), original left, PoC right, numbered views.
- **No map wrapping, anywhere.** PoR repeats a map past its edge for most
  area kinds; that is nonsense in a remake and is cut for every map. Views
  near an edge differ from the original because of it, by design.
- **Screenshots block the keyboard.** Shot windows open unfocused and the
  game ignores input while `BLOBBER_SHOT` is set.

## 1. Find the area and read the map data

`MAPS.md` lists every 3D map with its ECL, GEO and wall sets; take the next
unticked one and tick it when its comparison sheet is done.

Which GEO and wall sets an area uses is in its ECL script (`go run ./cmd/ecl
DISK BLOCK` from `cmd/`): `21 opLoadFiles GEO 2 255` and `37 opLoadFiles A B
C` (wall set blocks for slots 1-3). The ECL block number is the area kind.
The entry square is in the `opSave` to `[C04B]` (x) / `[C04C]` (y).

- A wall set block can hold several 5-wall chunks that fill consecutive
  slots (graveyard: block 21 fills slots 1-2). A block the ECL asks for
  that doesn't exist (graveyard: 22) is left out of `walldefs`: the
  original engine pops up an error and waits for a key, which hangs the
  comparison test.
- `viewdump` names: `WALLDEF<disk>-<block>-<chunk>-wall<n>`; list the three
  chunks for values 1-5, 6-10, 11-15 in `chunks`.
- Extract the map: `go run ./cmd/geodump DISK BLOCK phlan-poc/geo<disk>_<block>.bin`
  (repo root).
- Contact-sheet the wall sets first (`viewdump/`) and pick textures by
  looking at them.

PoR GEO block (`geoN_M.bin`, the 0x400 bytes after the 2-byte header):

| plane | content |
|-------|---------|
| 0x000 | north<<4 \| east wall value |
| 0x100 | south<<4 \| west wall value |
| 0x200 | cell byte |
| 0x300 | door bits, 2 per side, N E S W from bit 0 |

Walls are **thin edges** between squares and **two-sided**: each square keeps
its own value per side, and PoR draws only the viewed square's own sides. A
wall stored on one side only is invisible from the other.

Wall value v maps to WALLDEF chunk `(v-1)//5+1`, wall `(v-1)%5+1`.

## 2. Build the map (`make_map.py AREA`)

From `areas.py`: `geo`, `chars` (wall value -> map char, unique across areas
because `walls.txt` is shared; Phlan uses capitals, the graveyard a-o,
the slums p-z and B D I L M),
`start`, `water_walls`, `free_walls`.

- 16 rows of `.` (all floor), plus `floor` / `ceiling` lines.
- For every nonzero side: `face X Y SIDE CHAR` (drawn only from inside that
  square, like PoR). For every non-door side: `edge X Y SIDE` (blocks
  movement; doors pass).
- **Free-standing walls (stakes, railings, fences) get a face on the far
  side too**, when the neighbour has no wall of its own there. PoR hid this
  by painting what's behind onto the wall; with see-through textures the far
  row must be visible.
- **Land**: flood fill from START through open sides and doors.
- **Water**: flood fill from beyond harbour walls (here values 14, 15) into
  squares no party can reach. Emits `water X Y`. Off the map is water too.
- Writes `walls.txt` (`CHAR wall_<area>_vNN`); Phlan also sets the `start`
  line in `world.txt` (other areas are opened with `BLOBBER_OPEN`).
- Walls marked as doors are passable faces. In the graveyard every cross and
  headstone is a "door": the party walks through the grave rows.

## 3. Engine features in the copy (already in place, reuse)

All in `blobber/blobber.go` and `cmd/blobber/host.go`:

- **PoR projection**: eye 0.75 squares from the wall ahead, focal 42/88 of
  the view, walls 64/56 tall, eye 0.5 above the floor (`focalW`, `wallTop`,
  `eyeBack`). Wall tops line up with the original.
- **Thin walls**: `face` / `edge` map lines, `Faces`, `Edges`, `blocked()`
  for movement, back-face culling, deterministic (sorted) draw order, so no
  flicker or z-fighting.
- **Per-map thin walls**: each loaded map keeps its own faces, edges and
  water; `enter` swaps them into `blobber.Faces/Edges/Water`.
- **No wrapping.** PoR wraps off-map coordinates for towns (area kinds other
  than 0 and 10); the PoC doesn't, so the town is not repeated behind the
  sea. Rays run past the map edge.
- **Water**: animated `floor_phlan_water` on water cells, plus off the map
  and a sea fill to the horizon **only when the map has water**. Otherwise
  off-map is plain ground, like PoR's empty off-map squares.
- **Sky**: gradient plus a scrolling procedural cloud layer (moves with
  steps, turns and time, never mirrored).
- **Minimap** draws edges as lines.
- **Shots**: `BLOBBER_SHOT=file.png BLOBBER_OPEN=map:NAME:X:Y:DIR` saves
  frame 30 with no overlay (`BLOBBER_OVERLAY=1` keeps the position text,
  useful to confirm the camera).

## 4. Wall textures (`make_textures.py AREA`)

Flags in `areas.py`: `walls`, `notrim`, `water`, `tile`, `noseam`. Raw model
output goes to `raw/<area>/`.

1. Dump the original walls: `go run ./cmd/viewdump -transparent . viewdump`
   (from the repo root; the script does it).
2. Per wall value, a **specific description** in `WALLS`: what the wall *is*
   (not what the pixels look like). Read the EGA art carefully: on the dock,
   brown columns are wooden stakes, white curves are ropes, and the blue is
   the sea *behind*, not part of the wall.
3. Clean the reference before sending it:
   - `untrim()` removes the teal end trim (`NOTRIM` walls), which otherwise
     repeats on every tile.
   - `unwater()` removes painted sea (`WATER` walls); the engine draws real
     water behind them.
4. `tools/remaster-walls.py`: reference with a key colour -> OpenAI
   gpt-image edit -> key-out. Raw results go to `raw/`.
5. `finish(v)`:
   - `TILE` walls (continuous, like the quay) are crossfaded at the edges.
   - Other walls get **seams** (dark joint + lit edge) at both sides, like
     the original's trim, so neighbours read as separate segments. `NOSEAM`
     for walls a joint would cut through (stakes, water).
   - `outline()` clears the rows the original leaves empty above the wall
     top, so adjacent walls are the same height.
6. Re-cut everything from `raw/<area>/` without API calls:
   `python3 make_textures.py AREA finish`. Redo single walls with
   `python3 make_textures.py AREA 14`.
7. Contact-sheet original vs texture (aspect ratio kept) before comparing.
8. See-through openings (`arch`): the model tends to draw them thinner and
   narrower. `arch()` cuts the opening from the original's own shape, fills
   the stone above it from the same rows of the wall beside it, and darkens
   a rim around the cutout so it reads against walls seen through it.

Ground: `floor_<map>_dirt` (+ `_normal`) in warm brown, iterated until it
matches the original's brown. Water: `make_water.py` (generate, grade onto
the harbour blues, tile with a half-shift blend; `tile` regrades from raw/).

## 5. Build

```bash
cd blobber && go build -o $SCRATCH/blobber-poc ./cmd/blobber
```

## 6. Compare (`compare/compare.py AREA`)

- `views` in `areas.py` = list of `(x, y, facing)` in PoR squares. For a
  first pass take 15-20 random reachable squares and facings
  (`random.Random(seed).sample(land, n)`), then add targeted views.
- The test loads the area's GEO, wall sets and area kind from `areas.py`.
  Each PoC shot has a 60 s timeout, so a hung window can't pile up.
- Renders the original through a temporary engine test (written, run,
  deleted), shoots the PoC, crops symmetrically, writes
  `compare/comparison.jpg` with numbered labels.
- Pick views that cover: every wall value, wall-to-wall joins, doors,
  corridors seen from the side, map edges, water/harbour.
- **Only reachable squares.** Views from squares no party can stand on
  (water, sea) differ by design (PoR paints brown ground everywhere and draws
  only ~3 squares deep) and are dropped.
- After each fix, drop the views that now match, keep the rest, add new ones
  for anything suspicious.

## 7. Problems we hit, and their fixes (check each on a new map)

| symptom | cause | fix |
|---------|-------|-----|
| walls missing or visible from the wrong side | thick-cell walls; PoR's one-sided walls | thin `face`/`edge` walls, one cell per square |
| stretched textures | one texture over two cells | never; one face per square side |
| wall tops at different heights | model painted above the top; wrong projection | `outline()`; PoR projection constants |
| repeated pattern or teal stripes on long walls | trim tiled per segment | `untrim()` + `seams()`; crossfade for `TILE` |
| cut facades | trim/edge columns in the reference | `untrim()` and a "no border at the edges" prompt |
| flicker on a wall edge | random map iteration order | sorted cells, stable face sort |
| mirrored, static sky | sky drawn as a mirrored stretch | gradient + scrolling cloud layer |
| flat water painted on walls | sea baked into the EGA wall | `unwater()` + engine water surface |
| town repeated behind the sea | PoR coordinate wrap | wrap removed |
| far pier's stakes invisible | stakes stored one-sided | far-side face for free-standing walls |
| all shots identical | overlay skip also skipped the teleport hook | HUD runs on a scratch image in shot mode |
| "no monitor found" | display asleep | `caffeinate -u -t 1` before shooting |
| comparison test hangs | a wall set the ECL names is missing; error box waits for a key | leave it out of `walldefs` |
| second map shows the first map's walls | thin walls were global | per-world faces/edges/water |
| sea around a land-locked area | off-map always water | off-map water only on maps with water |

## Checklist for a new map

1. Read the area's ECL (GEO, wall sets, start); `geodump` the GEO block.
2. Add an `areas.py` entry: geo, walldefs, area_kind, chunks, chars, start,
   water/free walls, a description per used wall value, texture flags.
3. `make_map.py AREA`, `make_textures.py AREA`, contact-sheet the textures.
4. Build, pick 15+ random reachable views, `compare/compare.py AREA`.
5. Fix, recompare, drop matching views, repeat until the sheet is clean.
