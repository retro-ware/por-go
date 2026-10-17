# Remastering a Pool of Radiance map in the Blobber engine

How New Phlan (GEO3 block 0 + WALLDEF3 block 0) was brought into the
BlobberRPG copy in `phlan-poc/blobber/`, written as a recipe for every other
map. Run everything from `phlan-poc/`. `~/Projects/BlobberRPG` is never
touched; all engine changes live in the copy.

## 0. Rules that held the whole way

- **Exact layout.** One engine cell per PoR square, 16x16, same coordinates.
  The minimap must match the original's automap.
- **Never stretch.** No texture is ever scaled against its aspect ratio, and
  no texture spans two cells. If something looks wrong, explain the cause
  first, then change it.
- **Compare, don't guess.** Every change is judged on the side-by-side sheet
  (step 6), original left, PoC right, numbered views.
- **Screenshots block the keyboard.** Shot windows open unfocused and the
  game ignores input while `BLOBBER_SHOT` is set.

## 1. Read the map data

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

## 2. Build the map (`make_map.py`)

Adapt the GEO file, `CHARS` (wall value -> map char) and `START`.

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
- Writes `walls.txt` (`CHAR wall_<map>_vNN`) and the `start` line in
  `world.txt`.

## 3. Engine features in the copy (already in place, reuse)

All in `blobber/blobber.go` and `cmd/blobber/host.go`:

- **PoR projection**: eye 0.75 squares from the wall ahead, focal 42/88 of
  the view, walls 64/56 tall, eye 0.5 above the floor (`focalW`, `wallTop`,
  `eyeBack`). Wall tops line up with the original.
- **Thin walls**: `face` / `edge` map lines, `Faces`, `Edges`, `blocked()`
  for movement, back-face culling, deterministic (sorted) draw order, so no
  flicker or z-fighting.
- **No wrapping.** PoR wraps off-map coordinates for towns; the PoC doesn't,
  so the town is not repeated behind the sea. Rays run past the map edge.
- **Water**: animated `floor_<map>_water` on water cells and off the map,
  plus a sea fill to the horizon.
- **Sky**: gradient plus a scrolling procedural cloud layer (moves with
  steps, turns and time, never mirrored).
- **Minimap** draws edges as lines.
- **Shots**: `BLOBBER_SHOT=file.png BLOBBER_OPEN=map:NAME:X:Y:DIR` saves
  frame 30 with no overlay (`BLOBBER_OVERLAY=1` keeps the position text,
  useful to confirm the camera).

## 4. Wall textures (`make_textures.py`)

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
6. Re-cut everything from `raw/` without API calls:
   `python3 make_textures.py finish`. Redo single walls with
   `python3 make_textures.py 14`.

Ground: `floor_<map>_dirt` (+ `_normal`) in warm brown, iterated until it
matches the original's brown. Water: `make_water.py` (generate, grade onto
the harbour blues, tile with a half-shift blend; `tile` regrades from raw/).

## 5. Build

```bash
cd blobber && go build -o $SCRATCH/blobber-poc ./cmd/blobber
```

## 6. Compare (`compare/compare.py`)

- `VIEWS` = list of `(x, y, facing)` in PoR squares.
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

## Checklist for a new map

1. Copy `geoN_M.bin`; set `CHARS`, `START`, harbour/water values in
   `make_map.py`; run it.
2. Map the WALLDEF block in `make_textures.py`; write a description per wall
   value; set `NOTRIM`, `WATER`, `TILE`, `NOSEAM`.
3. Generate textures, then `finish`. Floor/water if the area needs them.
4. Build, pick 10-30 reachable views, run `compare/compare.py`.
5. Fix, recompare, drop matching views, repeat until the sheet is clean.
