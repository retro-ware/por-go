# BlobberRPG

A grid-based, turn-based first-person dungeon crawler engine in the style of Might & Magic 3-5 / Xeen, written in Go with [Ebitengine](https://ebitengine.org/). Content (maps, tile event scripts, data tables) is plain text, and a map editor is included.

## Build and run

Requires Go 1.26+.

```
go run ./cmd/blobber            # the game
go run ./cmd/editor [mapname]   # map editor (default map: demo)
go test ./...
```

Saves go to `blobber.sav` on desktop (browser storage in the web build, which has `js` build-tagged code).

## Controls

| Key | Action |
|---|---|
| W / Up, S / Down | step forward, back |
| A, D | strafe left, right |
| Q, E | turn left, right |
| F | search, take, use |
| I, C, J, M, R | inventory, cast, journal, map, rest |
| Space | wait a turn |
| F1-F6 | pick a character in menus and shops |
| F5 / F9 | quick save / load |
| ? | in-game help (all hotkeys) |
| Esc | menu, quit |

The mouse works in every window. Debug keys (G god mode, P party sheet, K kill ahead, N spawn monster) are listed in the help screen.

The editor: left-click paints the selected wall material, right-click selects a tile for events, click a monster thumbnail then a tile to place it. Ctrl+S saves, N new event, F cycle facing, U toggle use, Delete removes an event, Tab next event, Shift+Arrows grow the map, Ctrl+Shift+Arrows shrink it. See the header of `cmd/editor/main.go`.

## Tools

- `go run ./tools/items-html > docs/items.html`: regenerate the weapon/armor catalog page.
- `tools/new-monster.py`: generate monster poses with Gemini (API key needed).
- `tools/new-texture.py`: generate floor/wall/ceiling textures with the OpenAI or Gemini image API.
- `tools/monster-sprites.sh`: turn monster stills into animated sheets with sprite-gen (OpenAI key needed).
- `tools/flux.sh`, `tools/flux-web.py`: local FLUX.2 image generation (Apple silicon, mflux) and a small web UI for it.

## Layout

- `cmd/blobber`: the game (main loop, combat, saves). `cmd/editor`: map editor.
- Root package: grid renderer, movement, walls, billboards.
- `events`: event VM (Xeen-style opcodes, see `reference.md`). `party`: characters, items, spells, conditions.
- `maps`: `.map` / `.evt` files and data tables (`monsters.txt`, `classes.txt`, `spells.txt`, `world.txt`, ...).
- `ui`: HUD, inventory, shops, dialogs, help. `sfx`: synthesized sound. `font`, `sprites`: helpers.
- `assets`, `art`: textures and source art. `docs`: generated docs. `tools`: scripts.

See `ROADMAP.md` for design decisions and status.

## Credits

Asset credits and licences are in [CREDITS.md](CREDITS.md).
