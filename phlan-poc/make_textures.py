#!/usr/bin/env python3
"""Remaster pipeline: wall textures of one area for the BlobberRPG copy in blobber/.
  python3 make_textures.py [AREA] [finish | VALUE...]   (areas.py, default phlan)
Wall value v is chunk (v-1)//5+1, wall (v-1)%5+1 (areas.py "chunks"). Uses
tools/remaster-walls.py (reference with key colour, OpenAI edit, key-out),
then crossfades or seams the edges, and writes
blobber/assets/walls/wall_AREA_vNN.png (512x512 RGBA). Raw model output and
references go to raw/AREA/."""
import importlib.util, io, subprocess, sys
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
import numpy as np
from PIL import Image, ImageFilter
from areas import AREAS, origin

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
spec = importlib.util.spec_from_file_location("rw", ROOT / "tools/remaster-walls.py")
rw = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rw)

OUT = HERE / "blobber/assets/walls"
SIZE = 512
BAND = 0.12  # share of the width crossfaded into the opposite edge

TILING = (" No trim, pilaster or border strip at the left or right edge. IMPORTANT: this wall segment is repeated side by side along a street, so it must tile "
          "seamlessly horizontally: the left edge continues exactly where the right edge ends (same "
          "brick courses, mortar lines, stone rows, plant growth and heights), no border, pillar or "
          "frame closing off either side unless the reference shows one there.")

ARGS = sys.argv[1:]
NAME = ARGS.pop(0) if ARGS and ARGS[0] in AREAS else "phlan"
A = AREAS[NAME]
RAW = HERE / "raw" / NAME
# wall value -> (viewdump name, description)
WALLS = {v: (f"{A['chunks'][(v - 1) // 5]}-wall{(v - 1) % 5 + 1}", d) for v, d in A["walls"].items() if v not in A.get("props", {})}  # props: make_props.py


def seamless(im):
    """Crossfade the outer bands so the right edge runs into the left one."""
    a = np.asarray(im, dtype=np.float32)
    w = a.shape[1]
    b = int(w * BAND)
    t = np.linspace(0, 1, b)[None, :, None]
    # The last b columns fade into the first b; then those first b are dropped.
    a[:, w - b:] = a[:, w - b:] * (1 - t) + a[:, :b] * t
    return Image.fromarray(a[:, b:].astype(np.uint8), "RGBA").resize((SIZE, SIZE), Image.LANCZOS)


TRIM = (0, 170, 170)  # EGA teal: the end trim of plain walls, repeated at every block when tiled
NOTRIM = A["notrim"]


def untrim(dump, src):
    """The dump without teal in its outer columns (filled from further in), in dump/notrim."""
    im = Image.open(dump / f"{src}.png").convert("RGBA")
    a = np.asarray(im).copy()
    w = a.shape[1]
    edge = w // 14  # the outer half tile
    for x in list(range(edge)) + list(range(w - edge, w)):
        inner = edge + 2 if x < edge else w - edge - 3
        teal = (a[:, x, :3] == TRIM).all(axis=1)
        a[teal, x] = a[teal, inner]
    (dump / "notrim").mkdir(exist_ok=True)
    Image.fromarray(a, "RGBA").save(dump / "notrim" / f"{src}.png")
    return dump / "notrim"


TILE = A["tile"]  # continuous along a street: crossfaded, no seams
NOSEAM = A["noseam"]  # objects, posts, water: a seam would cut through them
SEAM = 0.022  # seam width, share of the texture width


def seams(im):
    """The original's trick (teal trim, building edges): a vertical joint at both edges, so faces side
    by side read as separate segments and a pattern that doesn't line up across the edge isn't seen."""
    a = np.asarray(im, dtype=np.float32).copy()
    w = a.shape[1]
    s = max(2, int(w * SEAM))
    for x in range(s):
        f = 0.45 + 0.4 * x / s  # darkest at the edge
        for col in (x, w - 1 - x):
            a[:, col, :3] *= f
    a[:, s, :3] = np.minimum(a[:, s, :3] * 1.35, 255)  # a lit edge on the inner side
    a[:, w - 1 - s, :3] = np.minimum(a[:, w - 1 - s, :3] * 1.35, 255)
    return Image.fromarray(a.astype(np.uint8), "RGBA")


def outline(im, src):
    """Clear the rows the original leaves empty (above the wall top), so walls side by side are one
    height as in the original; the models sometimes paint into them."""
    d = HERE / "viewdump" / "nowater" / f"{src}.png"
    dump = np.asarray(Image.open(d if d.exists() else HERE / "viewdump" / f"{src}.png").convert("RGBA"))[..., 3]
    empty = (dump == 0).all(axis=1)  # per dump row
    a = np.asarray(im).copy()
    rows = np.arange(a.shape[0]) * len(empty) // a.shape[0]
    a[empty[rows], :, 3] = 0
    return Image.fromarray(a, "RGBA")


ARCH = A.get("arch", set())  # see-through openings: cut to the original's shape, edge darkened
RIM = 0.045  # darkened rim around an opening, share of the texture width


def arch(im, src):
    """The opening of an arch exactly where the original has it (the image model draws it thinner), the stone
    it fills taken from the same rows of the solid wall left of it, and a dark rim around the cutout so it
    stands out against walls seen through it."""
    p = HERE / "viewdump" / ("clear" if src in {WALLS[v][0] for v in CLEAR} else "") / f"{src}.png"
    d = np.asarray(Image.open(p).convert("RGBA"))[..., 3] > 0
    top = np.argmax(d.any(axis=1))
    hole = ~d
    hole[:top] = False  # the empty rows above the wall top are not the opening
    hole = np.asarray(Image.fromarray(hole.astype(np.uint8) * 255).resize(im.size, Image.NEAREST)) > 0
    a = np.asarray(im).copy()
    n = a.shape[1]
    solid = a[..., 3] > 0
    # an object hanging in the opening (Kuto's bucket): the model draws it in its own shape, kept as drawn
    ys, xs = np.nonzero(hole)
    inside = np.zeros_like(hole)
    inside[ys.min():ys.max() + 1, xs.min():xs.max() + 1] = True
    oy, ox = np.nonzero(inside & ~hole)
    keep = np.zeros_like(hole)
    if len(oy):
        py, px = (oy.max() - oy.min()) // 3 + 4, (ox.max() - ox.min()) // 3 + 4
        keep[max(oy.min() - py, 0):oy.max() + 3, max(ox.min() - px, 0):ox.max() + px] = True
    box = keep & inside  # around the object: as the model drew it, no stone filled in where the original's was
    keep &= inside & solid
    whole = hole.copy()  # the opening with the object, for where the stone above it ends
    hole &= ~keep
    left = int(np.argmax(whole.any(axis=0)))  # first opening column; the wall left of it is solid
    x0, band = int(n * 0.02), max(8, left - int(n * 0.04))
    # the stone above the opening, column by column from the wall top down to the opening:
    # the model's own (thinner) arch there is painted over
    y0 = int(top * n / d.shape[0])
    fill = np.zeros_like(hole)
    for x in np.nonzero(whole.any(axis=0))[0]:
        fill[y0:np.argmax(whole[:, x]), x] = True
    fill |= ~whole & ~solid
    fill[:y0] = False
    fill &= ~box
    ys, xs = np.nonzero(fill)
    k = (xs - x0) % (2 * band)  # mirrored back and forth: no joins where the strip repeats
    a[ys, xs] = a[ys, x0 + np.where(k < band, k, 2 * band - 1 - k)]
    a[hole, 3] = 0
    # distance to the opening in pixels, by growing it one pixel at a time
    r = int(n * RIM)
    m = Image.fromarray(hole.astype(np.uint8) * 255)
    dist = np.full(hole.shape, r, dtype=np.float32)
    for i in range(r):
        m = m.filter(ImageFilter.MaxFilter(3))
        ring = (np.asarray(m) > 0) & (dist == r) & ~hole
        dist[ring] = i
    f = 0.3 + 0.7 * (dist / r) ** 0.7
    a[..., :3] = (a[..., :3] * f[..., None]).astype(np.uint8)
    return Image.fromarray(a, "RGBA")


def finish(v):
    """raw/vNN_openai.png -> the engine texture (no API call)."""
    ref = np.asarray(Image.open(RAW / f"v{v:02}_reference.png").convert("RGB"))
    k = max(rw.KEYS_RGB, key=lambda n: (ref == rw.KEYS_RGB[n][1]).all(axis=2).sum())
    im = rw.key_out((RAW / f"v{v:02}_openai.png").read_bytes(), k)
    im = seamless(im) if v in TILE else im.resize((SIZE, SIZE), Image.LANCZOS)
    im = outline(im, WALLS[v][0])
    if v in ARCH:  # before the seams, so the stone it copies has no seam in it
        im = arch(im, WALLS[v][0])
    if v not in NOSEAM:
        im = seams(im)
    im.save(OUT / f"wall_{NAME}_v{v:02}.png")


WATER = A["water"]  # the sea behind them is drawn by the engine as a water surface
SEA = [(82, 82, 255), (0, 0, 173), (0, 173, 173), (82, 82, 82)]  # EGA water, speckle and its far edge


def unwater(dump, src):
    """The dump with the painted sea cleared, in dump/nowater."""
    a = np.asarray(Image.open(dump / f"{src}.png").convert("RGBA")).copy()
    for c in SEA:
        a[(a[..., :3] == c).all(axis=2), 3] = 0
    (dump / "nowater").mkdir(exist_ok=True)
    Image.fromarray(a, "RGBA").save(dump / "nowater" / f"{src}.png")
    return dump / "nowater"


CLEAR = A.get("clear", {})


def unclear(dump, src, boxes):
    """The dump with boxes painted over in its commonest colour (the plain wall), in dump/clear."""
    a = np.asarray(Image.open(dump / f"{src}.png").convert("RGBA")).copy()
    px = a[a[..., 3] > 0][:, :3]
    cols, n = np.unique(px, axis=0, return_counts=True)
    for x0, y0, x1, y1 in boxes:
        a[y0:y1, x0:x1] = [*cols[n.argmax()], 255]
    (dump / "clear").mkdir(exist_ok=True)
    Image.fromarray(a, "RGBA").save(dump / "clear" / f"{src}.png")
    return dump / "clear"


def job(dump, v, src, desc):
    if (o := origin(NAME, v)) != (NAME, v):  # make_map points the faces at that texture
        return f"wall {v}: reused {o[0]} {o[1]}"
    if v in CLEAR:
        dump = unclear(dump, src, CLEAR[v])
    if v in WATER:
        dump = unwater(dump, src)
    if v in NOTRIM:
        dump = untrim(dump, src)
    ref, k = rw.reference(dump, src)
    prompt = rw.CONTEXT.format(kname=k, khex=rw.KEYS_RGB[k][0]) + desc + TILING
    (RAW / f"v{v:02}_reference.png").write_bytes(ref)
    (RAW / f"v{v:02}_prompt.txt").write_text(prompt + "\n")
    try:
        png = rw.openai(prompt, ref)
    except Exception as e:
        return f"wall {v}: FAILED {e}"
    (RAW / f"v{v:02}_openai.png").write_bytes(png)
    finish(v)
    return f"wall {v}: ok (key {k})"


def main():
    if ARGS[:1] == ["finish"]:  # re-cut every texture from raw/ only
        for v in WALLS:
            if origin(NAME, v) == (NAME, v):
                finish(v)
        return
    RAW.mkdir(parents=True, exist_ok=True)
    dump = HERE / "viewdump"
    subprocess.run(["go", "run", "./cmd/viewdump", "-transparent", ".", str(dump)], cwd=ROOT, check=True)
    only = {int(a) for a in ARGS}
    with ThreadPoolExecutor(9) as ex:
        jobs = [ex.submit(job, dump, v, s, d) for v, (s, d) in WALLS.items() if not only or v in only]
        for j in jobs:
            print(j.result(), flush=True)


if __name__ == "__main__":
    main()
