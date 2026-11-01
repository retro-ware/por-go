# BlobberRPG

A grid-based first-person renderer in the style of Might & Magic 3-5 / Xeen, written in Go on the
`gpu/` package. Pool of Radiance (`por/cmd/por`) draws its remastered 3D view with it.

```
go test ./...
```

## Tools

- `tools/new-texture.py`: generate floor/wall/ceiling textures with the OpenAI or Gemini image API.
- `tools/flux.sh`, `tools/flux-web.py`: local FLUX.2 image generation (Apple silicon, mflux) and a small web UI for it.

## Layout

- Root package: grid renderer, movement, walls, billboards.
- `maps`: the `.map` files, embedded for the remaster.
- `assets`: textures.

See `ROADMAP.md` for design decisions and status.

## Credits

Asset credits and licences are in [CREDITS.md](CREDITS.md).
