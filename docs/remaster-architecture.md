# Remaster integration plan: Blobber view in PoR

Status: proposal, awaiting review. No code changed yet.

## 1. Principles

- **The classic engine stays authoritative and unaware.**
  - `engine/` keeps running the game exactly as now: rules, ECL, movement, menus, text, and drawing into the 320x200 `ega.Screen`.
  - The remaster is presentation only. It never mutates game state.
  - This is what makes live switching safe: switching changes how a frame is shown, never what the game does.
- **One narrow read-only seam out of the engine.**
  - The engine publishes a small immutable `engine.View` snapshot after each settled step.
  - Everything the remaster knows about the game comes through it.
- **Blobber becomes a library, not a game.**
  - Its renderer gets an instance API that draws into a caller's image.
  - Its own loop, movement, party, combat and event VM stay in `cmd/blobber` and are not used by PoR.
- **The classic pixels are reused, not re-implemented.**
  - Picture and text pop-ups are crops of the classic framebuffer, so the original red borders, fonts, pictures and animations come for free and always match.

## 2. Modules and dependencies

```
cmd/por ──► present/classic   (today's Draw: palette → scaled 4:3)
   │   └──► present/remaster ──► blobber (renderer lib)
   │                └──────────► engine.View  (read-only snapshot)
   └──► engine ──► ega
```

| Module | Owns | Must not |
|---|---|---|
| `engine` | game state, rules, classic drawing into `ega.Screen`, publishing `View` | import ebiten, blobber or present |
| `blobber` (lib) | 3D scene: maps, walls, props, water, camera tween, `DrawView(dst, Camera)` | know PoR, read env vars, own the game loop |
| `present/classic` | today's frame scaling, moved out of `cmd/por/main.go` | — |
| `present/remaster` | layout: 3D view, pop-ups, portrait cards; the area → map table | write engine state |
| `cmd/por` | window, input, the mode hotkey, choosing a presenter | game logic |

`cmd/por` holds one interface with exactly the two implementations that exist:

```go
type Presenter interface {
    Draw(dst *ebiten.Image, scr *ega.Screen, v engine.View)
}
```

## 3. The seam: `engine.View`

All fields are typed, with no offsets (project rule). The engine fills it under the screen mutex when a step settles. `cmd/por` reads a copy.

```go
type View struct {
    Area    AreaID     // geo disk/block currently loaded
    X, Y    int
    Facing  Dir        // N E S W
    Pane    PaneKind   // ThreeD, Picture, AreaMap, Overhead, FullScreen (title, sheets, combat, shops)
    PicRect CellRect   // inner window incl. red border when Pane == Picture
    Text    TextState  // bottom pane: in use?, its CellRect incl. border
    Party   []Member   // Name, HP, MaxHP, AC, Status, Selected, Head PortraitID
}
```

- `PaneKind` comes from the state the engine already keeps (`beginFlag`, `viewMode`, `e.pic`). It is not inferred from pixels.
- Portrait heads: the engine adds `PortraitHead(id) image.Image`. It decodes the HEAD block, the same data `LoadPortrait` uses, and caches the result.

## 4. Mode switching (requirement 2)

- **Hotkey: F10**, handled in `cmd/por` Update and never forwarded to the engine. It swaps the active `Presenter`. The engine and its goroutine never notice, so nothing about the game state can break.
- The remaster draws a frame only when `View.Pane` is `ThreeD` or `Picture`. For `FullScreen` screens (combat, character sheets, shops, title) it falls back to the classic presenter for that frame. Those screens get remastered later, one `PaneKind` at a time.
- Input stays classic in both modes: arrow keys go to the engine, and the camera follows `View`. The blobber tween animates each step and turn, so movement still feels smooth.

## 5. Remaster layout (requirements 3, 4, 5)

- **3D view:** fills the window. `blobber.DrawView(dst, Camera{Map: areaMap[v.Area], X, Y, Facing})`.
- **Picture pop-up (req. 3):** when `Pane == Picture`, crop `PicRect` from the classic frame and draw it centred in the top half over the 3D view.
  - The crop includes the original red border, and the picture keeps animating because the engine keeps drawing it.
  - It is scaled by an integer factor only, so the aspect ratio is kept.
- **Text pop-up (req. 4):** when the text pane is in use, crop rows 0x11–0x16 including the border (exact shape) and draw them centred in the lower half.
  - Row 24 (the menu and prompt line) is shown under it, so menus stay usable.
- **Portraits (req. 5):** Blobber's `ui` portrait cards (`ui/portrait.go`, `ui/hud.go`) get a source interface instead of `*party.Character`:
  - Name, HP, MaxHP, Selected, Portrait image.
  - Cards are fed from `View.Party`, with each portrait being the character's chosen paper-doll head.
  - The 8-bit EGA head is upscaled by an integer factor (nearest neighbour) into the 45 px card art, with the aspect ratio preserved.

## 6. Blobber refactor (inside the copy at phlan-poc/blobber, never ~/Projects/BlobberRPG)

1. **Instance renderer:**
   - Move the global level, faces, edges, water, pond and props state into `type Scene` (built by `LoadScene(fs.FS, name)`, which parses both the grid and the render-relevant `extra` lines now in `cmd/blobber/host.go`).
   - Move the drawing state into `type Renderer` (shaders, texture caches, tween), with `Draw(dst, *Scene, Camera)`.
2. **Thin wrapper:** `Run` and the `Game` loop stay as a wrapper in `cmd/blobber`, so the PoC keeps working and remains the place to tune visuals.
3. **No hidden inputs:** env flags (`BLOBBER_SHOT`, `NIGHT`, `CEILING`, `OVERLAY`) move into a `Options` struct passed by the host.
4. **Embedded assets:** maps and textures are embedded via `fs.FS`, with no CWD-relative paths.
5. **Module wiring:** the module path becomes `por/blobber`, inside the por module, so there is no `replace`.
6. **Regression check:** `cull_test.go` and `main_test.go` keep passing, and compare.py shots must match before and after the refactor (byte-identical frames).

The area → map table is generated by `make_map.py` from `areas.py` (`geo` disk/block → map name) as a small Go file in `present/remaster`. A GEO block with no remastered map falls back to the classic presenter.

## 7. Milestones (each one committed, checklist and PROCESS.md updated)

| # | Milestone | Check |
|---|---|---|
| 1 ✅ | Move today's scaling into `present/classic` and add the `Presenter` seam | game unchanged, screenshots identical |
| 2 ✅ | `engine.View` and `PortraitHead`, plus a test that walks a fixture and asserts View fields | `go test ./engine` |
| 3 ✅ | Blobber instance renderer, global state removed | PoC compare shots byte-identical |
| 4 ✅ | `present/remaster` 3D view, area table, F10 switch, classic fallback | toggle mid-walk in Phlan; the state is identical either way |
| 5 ✅ | Picture and text pop-ups cropped with their red borders | full-size shots of an encounter and a text event |
| 6 ✅ | Portrait cards from paper-doll heads | shot with a 6-character party |
| 7 ⏸ | Later Gold Box games: `AreaID` → map table per game; the seam stays the same | — |

## 8. Decisions I made that you may want to change

- **F10 as the hotkey:** F1–F8 pick a character's inventory, and Tab and the letter keys are PoR commands.
- **Input stays classic in both modes:** the original's discrete steps. Blobber's own free movement is not used.
- **Classic fallback for full-screen screens:** in remaster mode they show the original until they are remastered.
- **Blobber moves into the por module** as `por/blobber`. The phlan-poc copy becomes that package, and `cmd/blobber` stays as the tuning tool.

## 9. Done before milestone 1

- The blobber copy is trimmed:
  - 55 wall, floor and ceiling textures that no PoR map uses are removed, with their walls.txt rows.
  - The painted monster sprites are removed (art/monsters, cmd/blobber/assets/monsters).
- The PoC host's debug combat now shows a stand-in sprite.
- `TestWorldRestAndEnd` was already failing before this cleanup (world.txt starts in Phlan) and is untouched.

## 10. Milestone 1 (done)

- `present/classic.Presenter` holds the frame conversion, the 4:3 scaling, the overlays and the toast that were in `cmd/por/main.go`. Its code is unchanged apart from renames, checked by diff.
- `cmd/por` keeps one `classic.Presenter` and draws through it. The `Presenter` interface is added in milestone 4, when the second implementation exists.
- New capture mode for full-size checks: `POR_SHOT=FILE.png [POR_KEYS='keys, \r Enter, \e Escape'] por -data .`
  - The window opens unfocused and ignores the user's input.
  - It sends the scripted keys, one every quarter second, writes the window to FILE, then exits.

## 11. Milestone 2 (done)

The seam is `engine/snapshot.go`:

- **When it is published:** `(*Engine).View()` returns the last snapshot. The engine publishes it each time it polls or waits for a key, on its own goroutine, through an atomic pointer.
- **`MainFrame`:** `DrawMainFrame` records the border pixels of the view window and the text pane. The frame counts as up while those pixels are unchanged. Combat, sheets, shops and the title all draw over them.
  - Fixed in milestone 4: the `CellRect`s are box insides, as `DrawBorder` takes them, and the border is `Framed()`. The check now samples the real border cells.
- **`Window`:** set by whatever drew into the view window last: `Draw3DView`, `DrawAreaMap`, or the picture paths (`drawPicAt`, `DrawAnimFrame`).
- **`TextShown`:** any lit pixel inside the text pane's border.
- **`Party`:** name, HP, AC as shown, status, selection, and `Head`. `Head` is the chosen paper-doll head (`portraitHead[PortHead]` from the HEAD files), 88x40 in EGA colours, decoded once per id.
- **Fixed rectangles:** `ViewWindow`, `InnerWindow`, `TextPane` and `PromptRows`, as `CellRect`s.
- **Test:** `TestView` loads GAME A headless and checks the 3D view in the main frame, the party heads, turning right, and that the character sheet hides the frame and Esc brings it back. It is skipped without the game files.
- **Known gap:** when the view window is only cleared, `Window` keeps the last picture. Add a case for it in milestone 5 if a pop-up shows up empty.

## 12. Milestone 3 (done)

What PoR gets from Blobber is now three types:

- **`blobber.Scene`** (`scene.go`): `ParseScene(r)` reads a .map file's grid and its drawing lines (faces, edges, water, ponds, props, wells, floor and ceiling) and returns the other lines to the host.
  - `Show()` makes it the drawn scene.
  - The PoC host now uses it too, so the parsing lives in one place.
- **`blobber.View`** (`view.go`): `NewView()`, `Show(scene, Camera)`, `Follow(Camera)`, `Update()` and `Draw(dst)`.
  - It draws the 3D view alone into the caller's image, at that image's size.
  - `Follow` animates a step, sidestep or turn as the player's own move would, and jumps anything further.
  - It never reads the keyboard.
- **`blobber.Camera`**: a cell and a facing.

`Game.Draw` is split so the 3D part (`draw3D`) is shared. `Run` and the PoC keep working on top of the same code.

Deviations from section 6, kept deliberately:

- **Package state stays, behind the View API.** Only one 3D view draws at a time, and making every global per-instance would touch most of the 2300 lines for no visible gain. This is marked in `view.go` with the upgrade path.
- **The BLOBBER_* env flags stay.** They only act when set, which only the PoC tools do.
- **Module wiring moves to milestone 4**, where PoR first imports the package.

Checks:

- `TestViewFollow` covers step, sidestep and jump.
- `go test ./...` in phlan-poc/blobber passes. Two tests left broken by the asset cleanup and the world start change are fixed: TestSheetAnimation now uses in-memory poses, and TestWorldRestAndEnd expects the Phlan start.
- Frames are not byte-identical from run to run (animated water and clouds). The six-area compare (82 frames) differs from the baseline only in those places.

## 13. Milestone 4 (done)

- **Module wiring:** the por module requires `blobber` with `replace blobber => ./phlan-poc/blobber`. `blobber/maps.Levels` embeds the .map files.
- **Area table:** `present/remaster/areas.go` maps a GEO disk and block to a map name. `make_map.py` generates it from areas.py on every run, and runs gofmt on it.
- **`present/remaster.Presenter`:**
  - It draws the Blobber view full-window when the main frame is up, the view window shows the 3D view, and the area has a remastered map.
  - Otherwise it draws through the shared `classic.Presenter`: combat, sheets, pictures (until milestone 5), and unremastered areas.
  - It parses each map once. A step or turn tweens, anything else jumps, and the animation runs at 60 ticks/s whatever the frame rate.
  - The menu and prompt line (row 0x18) is laid along the bottom, cropped from the classic frame and scaled by a whole factor.
- **`classic.Presenter.Frame`** returns the converted 320x200 frame for those crops.
- **F10** in `cmd/por` switches presenters and never reaches the engine. `POR_REMASTER=1` starts in remaster mode.
- **POR_KEYS** gained `\U \D \L \R` (the arrows) and `\T` (F10). Later `\1`..`\8` (F1..F8).
- **Checks:**
  - Remastered Phlan from GAME A (7,0 E, and 6,0 S after walking) matches the classic view's geometry.
  - Switching three times mid-walk (`L\r\U\T\U\R\T\U\T`) ends on a frame byte-identical to the same walk without switching.
- **Mouse:** in a remastered frame the mouse points at nothing for now; the classic layout's cells don't apply.

## 14. Milestone 5 (done)

- **The 3D view stays behind pop-ups.** `present/remaster` also keeps it when the view window shows a picture (`WinPicture`).
- **Picture pop-up:** the classic frame's `ViewWindow.Framed()` box, red border included, is drawn over the view, centred in the top half.
- **Text pop-up:** when `TextShown` is set, `TextPane.Framed()` is drawn centred in the bottom half.
- **Scale:** both pop-ups use the same whole factor, the largest at which the picture box fits half the window, so pixels stay square. The prompt row keeps the full classic factor.
- **Checks:** at 1920x1440, the trainer picture with its question, and the Phlan sign text.
- **Known quirk:** the text box's top border keeps the frame's T joint, where the view window's border meets it in the original.

## 15. Milestone 6 (done)

- **Cards:** `present/remaster` draws Blobber's own party cards (`ui.DrawParty`) on the 3D view, from a `party.Party` it rebuilds each frame from `View.Party` (name, HP, MaxHP).
- **Card art:** the 45x45 picture is the middle 45 columns of the 88x40 paper-doll head, where the face is, set on the card's bottom edge without scaling.
- **Blobber changes:** `party.Character.Face` (an `image.Image`) overrides the portrait pool, and `ui.PartyRect(n)` gives the cards' area.
- **Pop-ups:** they now centre in the space left of the cards, and their whole-number scale also fits that width.
- **Check:** the 6-character GAME A party at the trainer and at the sign, at 1920x1440.

## 16. Milestone 7 (waiting)

Waiting until the engine runs a second Gold Box game. For now it loads only Pool of Radiance, so a per-game area table would have nothing to map.

When a second game runs:
- add its identity to `engine.View`;
- key `areaMaps` by game and `AreaID`;
- have `make_map.py` write each game's table.

The rest of the seam stays as it is.

## 17. Remaster polish (after milestone 6)

- **Day and night:** `View.Day` is the engine's `isDay` (6:00–20:00, the same test that picks the day sky in the scripts). The remaster sets `blobber.Night` from it every frame.
- **Ceiling:**
  - `View.Inside` is set when the party's map cell is above 0x7F, which is when the original paints its red inside sky.
  - Inside, the remaster swaps the open sky for Blobber's default ceiling.
  - The Phlan start corridor is such an inside area in the original too.
- **Menu:**
  - `View.Menu` holds the prompt, items, hotkeys and highlight of the `DisplayInput` waiting for a choice. `View.Command` marks the adventuring command bar.
  - In the remaster the bottom bar is gone. Enter opens the menu as a vertical Blobber window: arrows move, Enter sends the item's hotkey, Esc closes, and the hotkeys still work.
  - The original prompt line is still drawn when no menu waits, for typed input and "press a key".
- **Options:**
  - Esc on the command bar opens Blobber's technical options page (`View.OpenOptions`). It covers smooth movement, walk speed, dithering, normal and light maps, torch and fullscreen.
  - While the page is open it reads the keyboard itself and the game gets no keys.
  - Settings aren't saved between runs yet.
- **Portraits:**
  - Cards run down both edges: the first half of the party on the left, the rest on the right (4 + 4 with two NPCs). They use the largest whole scale at which a column of four fits.
  - Each head's lit pixels are centred in the card art.
  - Pop-ups centre between the two columns.
- **Picture pop-up:** only the inner picture with its own border (`InnerWindow.Framed()`), without the view window's outer frame.
- **Movement:**
  - The copy's tween is the same as BlobberRPG's.
  - What looked jerky was `Follow`: every engine step braked to a stop, and a step arriving mid-tween restarted from the target.
  - Moves that come while one runs now queue (up to 8). The tween carries its speed into them and runs faster while they pile up (`TestViewFollowQueue`).
- **No disks:** the game split one set of DAX blocks over eight disks (GEO1..8, ECL1..8, HEAD1..8 and so on). Across all of them a block id is the same data wherever it appears; only WALLDEF 23 and CPIC 24 and 152 differ between disks.
  - `dax.Loader.Block` now looks for a block missing from the named file in the same archive on the other disks. Every read of the game's data goes through it, so all data is always available.
  - The disk asked for still wins, which keeps the three differing blocks as the original had them.
  - The engine keeps its disk digit (DS:5376) only as original game state; it is in the save files.
  - `engine.AreaID` is now just the GEO block, and the generated area table has no disk.
  - This also fixed the wrong portraits after the boat to Sokol Keep: that disk's HEAD file lacks most player heads.
- **Floor seams:** the floor (and the ceiling indoors) is one quad over the visible cells, cut into pieces per cell for the lighting and skipping cells no ray reaches. Pieces of one quad share their edge vertices exactly, so no sky shows between them; per-cell tiles computed their shared edges apart and showed thin blue lines (Phlan 3,3 N: 290 blue floor pixels before, 0 after). Water cells are laid over it (pass 1).
- **Horizon:** where the sky is open, a ground plane reaches 4000 cells round the camera under the floor quad, so its far edge sits under a pixel below the horizon. It is water on any map with water (Phlan has water cells but no `sea` line), else the floor. At 90 cells and floor-textured it showed a brown strip with a light line above it (Phlan 4,0,N); now sky meets sea, day and night.
- **Map and cards:** the engine has one map renderer, `drawMap` in engine/fullmap.go (seen squares, walls, doors, labels, the party). The full map (M), the Tab panel's minimap and the remaster all use it: for the remaster, the engine draws it into an image off screen and publishes it as `View.Map` (16 squares of `MapCell` px), which the presenter shows top right on black in a 1 px grey border.
- **Messages:** text with no choice (no menu, the command bar, or a one-item "press enter" menu) goes along the bottom edge with no menu bar or pop-up; Enter passes straight to the game.
- **Menus:** the vertical menus (questions and the command bar on Enter) are drawn in the original's look: the text pane's red border cells, the game's current font green on black with each item's hotkey white, the selection on blue. The font toast (Ctrl-F) shows top centre too, cropped from the strip the classic frame writes it into. The original's overhead map (DrawOverheadMap) and Blobber's map screen and minimap are gone; DrawAreaMap (the overland tile map) stays. The portrait cards sit as low as they go; the map takes the largest whole scale that still leaves room under it for columns of four cards (two NPCs included).
- **Question pop-ups:** a menu that is not the command bar (an answer the game waits for) opens as a vertical menu by itself, right under the text pane; the picture sits above the text, the text in the middle. Enter on the game's own highlighted item passes Enter through (prompts like "Press enter to continue" take nothing else); another item sends its hotkey. Esc closes the pop-up, Enter opens it again.
- **Distance fog** (Blobber option, on by default; open skies only): the texture shader fades every pixel toward the sky's horizon colour (day or night) by its distance, squared, reaching full at 128 cells. Close by it is barely there; far out the sea melts into the sky.
- **Fluid movement** (Blobber option, on by default): while a move key is held, the view asks the host for the next move (`View.Flow`) halfway through the current one, and if the game is slower it waits at the cell at full speed. Steps and turns flow into each other without slowing, as in BlobberRPG. cmd/por skips its typematic repeat for the arrows while the view paces them; with the option off they repeat as typed.
