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

- `PaneKind` comes from the state the engine already keeps (`beginFlag`, `viewMode`, `gs.overhead`, `e.pic`). It is not inferred from pixels.
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
| 2 | `engine.View` and `PortraitHead`, plus a test that walks a fixture and asserts View fields | `go test ./engine` |
| 3 | Blobber instance renderer, global state removed | PoC compare shots byte-identical |
| 4 | `present/remaster` 3D view, area table, F10 switch, classic fallback | toggle mid-walk in Phlan; the state is identical either way |
| 5 | Picture and text pop-ups cropped with their red borders | full-size shots of an encounter and a text event |
| 6 | Portrait cards from paper-doll heads | shot with a 6-character party |
| 7 | Later Gold Box games: `AreaID` → map table per game; the seam stays the same | — |

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
