#!/usr/bin/env python3
"""New Phlan PoC: remastered wall textures for the BlobberRPG copy in blobber/.
New Phlan is GEO3 block 0 with WALLDEF3 block 0 (15 walls in 3 chunks: wall
value v = chunk (v-1)//5+1, wall (v-1)%5+1). Doors become Blobber doorways,
so only the walls drawn as solid faces get a texture (WALLS below). Uses
tools/remaster-walls.py (reference with key colour, OpenAI edit, key-out),
then crossfades the left and right edges so a wall tiles seamlessly along a
street, and writes blobber/assets/walls/wall_phlan_vNN.png (512x512 RGBA)."""
import importlib.util, io, subprocess, sys
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
import numpy as np
from PIL import Image

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
spec = importlib.util.spec_from_file_location("rw", ROOT / "tools/remaster-walls.py")
rw = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rw)

OUT = HERE / "blobber/assets/walls"
RAW = HERE / "raw"
SIZE = 512
BAND = 0.12  # share of the width crossfaded into the opposite edge

TILING = (" No trim, pilaster or border strip at the left or right edge. IMPORTANT: this wall segment is repeated side by side along a street, so it must tile "
          "seamlessly horizontally: the left edge continues exactly where the right edge ends (same "
          "brick courses, mortar lines, stone rows, plant growth and heights), no border, pillar or "
          "frame closing off either side unless the reference shows one there.")

# wall value -> (viewdump name, description)
WALLS = {
    1: ("WALLDEF3-000-1-wall1", "a grey stone wall with a wide dark square tunnel opening in the middle "
        "leading into darkness, a small lit blue doorway far away at the end of the tunnel; the tunnel "
        "floor is empty."),
    3: ("WALLDEF3-000-1-wall3", "a plain smooth grey plastered stone town wall with a sturdy arched "
        "wooden door of vertical brown planks and a metal handle at the bottom centre."),
    5: ("WALLDEF3-000-1-wall5", rw.WALLS["ivy_wall_door"][1]),
    7: ("WALLDEF3-000-2-wall2", "a town house wall of white irregular fieldstones set in grey mortar "
        "with a sturdy arched wooden door of vertical brown planks and a metal handle at the bottom "
        "centre."),
    9: ("WALLDEF3-000-2-wall4", "a shop front in a town house wall of white irregular fieldstones: a "
        "red and white striped cloth awning across the whole width just above the middle, below it a "
        "wooden plank door on the left and a large shop window of blue glass panes (two rows of four) to "
        "its right."),
    10: ("WALLDEF3-000-2-wall5", "a town house wall of white irregular fieldstones with a large window "
         "of blue glass panes (two rows of four) on the left and an arched wooden plank door on the "
         "right, both in the lower half."),
    2: ("WALLDEF3-000-1-wall2", "a plain smooth grey plastered stone town wall, even and unadorned, the top row empty."),
    4: ("WALLDEF3-000-1-wall4", "a grey stone town wall overgrown with climbing ivy and green vines, "
        "dense near the bottom, thinner tendrils reaching up towards the top."),
    6: ("WALLDEF3-000-2-wall1", "a town house wall of white irregular fieldstones set in grey mortar, "
        "rounded flat stones in a dense even pattern over the whole face."),
    8: ("WALLDEF3-000-2-wall3", "a town house wall of white irregular fieldstones set in grey mortar, "
        "rounded flat stones in a dense even pattern over the whole face."),
    11: ("WALLDEF3-000-3-wall1", "the front of a grand civic hall: red brick wall with four tall "
         "pale cyan marble pilasters with capitals at the top, tall dark blue glass windows between "
         "them in the lower half, and broad grey stone steps along the bottom."),
    12: ("WALLDEF3-000-3-wall2", rw.WALLS["brick_building_stairs"][1]),
    13: ("WALLDEF3-000-3-wall3", rw.WALLS["temple_gate"][1]),
    14: ("WALLDEF3-000-3-wall4", "the edge of a harbour pier: a row of four tall, thick round wooden "
         "mooring stakes (weathered dark brown and reddish timber, dark shadow on their right side) "
         "standing in a row, a short stake stump between each pair at the bottom, and white hemp ropes "
         "hanging in shallow sagging curves from stake to stake at mid height. The water behind the "
         "stakes is not drawn: everything around and between the stakes and ropes is empty."),
    15: ("WALLDEF3-000-3-wall5", "a low harbour quay wall: white irregular fieldstones set in grey "
         "mortar across the bottom quarter only, its top a straight edge; above it nothing is drawn, the "
         "water behind it is not part of the wall."),
}


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
NOTRIM = {1, 2, 3, 6, 7, 8, 9, 10}


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


TILE = {15}  # continuous along a street (water): crossfaded, no seams
NOSEAM = {14, 15}  # pillars and water: a seam would cut through them
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


def finish(v):
    """raw/vNN_openai.png -> the engine texture (no API call)."""
    ref = np.asarray(Image.open(RAW / f"v{v:02}_reference.png").convert("RGB"))
    k = max(rw.KEYS_RGB, key=lambda n: (ref == rw.KEYS_RGB[n][1]).all(axis=2).sum())
    im = rw.key_out((RAW / f"v{v:02}_openai.png").read_bytes(), k)
    im = seamless(im) if v in TILE else im.resize((SIZE, SIZE), Image.LANCZOS)
    if v not in NOSEAM:
        im = seams(im)
    im = outline(im, WALLS[v][0])
    im.save(OUT / f"wall_phlan_v{v:02}.png")


WATER = {14, 15}  # the sea behind them is drawn by the engine as a water surface now
SEA = [(82, 82, 255), (0, 0, 173), (0, 173, 173), (82, 82, 82)]  # EGA water, speckle and its far edge


def unwater(dump, src):
    """The dump with the painted sea cleared, in dump/nowater."""
    a = np.asarray(Image.open(dump / f"{src}.png").convert("RGBA")).copy()
    for c in SEA:
        a[(a[..., :3] == c).all(axis=2), 3] = 0
    (dump / "nowater").mkdir(exist_ok=True)
    Image.fromarray(a, "RGBA").save(dump / "nowater" / f"{src}.png")
    return dump / "nowater"


def job(dump, v, src, desc):
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
    if sys.argv[1:2] == ["finish"]:  # re-cut every texture from raw/ only
        for v in WALLS:
            finish(v)
        return
    RAW.mkdir(exist_ok=True)
    dump = HERE / "viewdump"
    subprocess.run(["go", "run", "./cmd/viewdump", "-transparent", ".", str(dump)], cwd=ROOT, check=True)
    only = {int(a) for a in sys.argv[1:]}
    with ThreadPoolExecutor(9) as ex:
        jobs = [ex.submit(job, dump, v, s, d) for v, (s, d) in WALLS.items() if not only or v in only]
        for j in jobs:
            print(j.result(), flush=True)


if __name__ == "__main__":
    main()
