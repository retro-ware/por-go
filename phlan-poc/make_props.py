#!/usr/bin/env python3
"""Remaster pipeline: free-standing props drawn as billboards instead of wall faces (see make_map.py "props").
  python3 make_props.py [NAME...] [finish]   (NAME: tree, ladder, rim, grass1, grass2; default all)
Nomad Camp trees: PoR paints them onto walls (a tree at a face's left, or halves at its edges that join across
faces); the PoC stands one tree sprite wherever a tree's trunk is. The reference is the original's one whole tree
(WALLDEF7-007-2-wall1, nomad wall 6), cropped and centred; writes blobber/assets/walls/prop_nomad_tree.png.
Kuto's Well: the well hole and its ladder, which PoR paints on every wall of the well room, stand once in the
room (areas.py "cellprops"): the hole is drawn by the engine (an oval opening with the rim's stones laid around
it), the ladder stands down in it at the far side; references are parts of kuto wall 8.
Garden beds (library wall 13, reused in the pyramid): PoR paints weeds on mounds on every side of a bed; the PoC
plants grass clumps (two variants) over the bed's squares (areas.py "beds")."""
import importlib.util, io, sys
from pathlib import Path
import numpy as np
from PIL import Image

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("rw", HERE.parent / "tools/remaster-walls.py")
rw = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rw)
from areas import src
RAW = HERE / "raw/props"
OUT = HERE / "blobber/assets/walls"
TREE = ("Game sprite on a flat solid magenta (#FF00FF) background: one single tall leafy deciduous tree standing "
          "upright, seen from the side at eye level, a full rounded crown of dense green leaves with a few small red "
          "berries, a gnarled dark brown trunk with roots at the bottom touching the lower edge of the image. The tree "
          "is centred and fills the image height; nothing else, no ground, no shadow, no grass, the magenta "
          "background is perfectly flat and fills everything around the tree. Painterly high-fantasy game art with "
          "modern detail, faithful to the reference's shape and colours.")
LADDER = ("Game sprite on a flat solid magenta (#FF00FF) background: one straight rusty iron ladder standing "
          "perfectly upright, seen from the front, two round rails and seven evenly spaced rungs, the bottom ends of "
          "the rails at the lower edge. Nothing else, no ground, no wall, no shadow, a small magenta margin at the "
          "top and the sides. Painterly high-fantasy game art with modern detail, colours like the reference.")
RIM = ("Game texture on a flat solid magenta (#FF00FF) background: one single straight horizontal row of rounded grey "
       "fieldstone blocks set side by side with dark mortar joints and a little moss, seen straight down from above, "
       "like the stone rim of a well laid out straight. The row runs the full width of the image from the left edge "
       "to the right edge and must tile seamlessly left to right; it is about a sixth of the image tall, centred "
       "vertically, flat magenta above and below. Painterly high-fantasy game art with modern detail.")
GRASS = ("Game sprite on a flat solid magenta (#FF00FF) background: one small low tuft of short wild grass seen from "
         "the side at eye level, wider than tall: a fan of short bright green blades spreading outward, two or three "
         "tiny blue flower specks, a couple of small grey pebbles at its base, the base touching the lower edge of the "
         "image. Centred, nothing else, no soil mound, no ground, no shadow, a small magenta margin at the top and "
         "sides. Painterly high-fantasy game art with modern detail, colours like the reference's weeds.")
# name -> (texture, area, wall value, share of the dump's rows the reference starts at, prompt)
PROPS = {"tree": ("prop_nomad_tree", "nomad", 6, 0.0, TREE), "ladder": ("prop_kuto_ladder", "kuto", 8, 0.5, LADDER),
         "rim": ("floor_kuto_rim", "kuto", 8, 0.9, RIM),
         "grass1": ("prop_grass1", "library", 13, 0.6, GRASS), "grass2": ("prop_grass2", "library", 13, 0.6, GRASS)}


def reference(area, v, top):
    a = np.asarray(Image.open(HERE / "viewdump" / f"{src(area, v)}.png").convert("RGBA")).copy()
    a[:int(a.shape[0] * top), :, 3] = 0
    ys, xs = np.nonzero(a[..., 3])
    tree = Image.fromarray(a[ys.min():ys.max() + 1, xs.min():xs.max() + 1])
    side = max(tree.size)
    ref = Image.new("RGBA", (side, side), (255, 0, 255, 255))
    ref.paste(tree, ((side - tree.width) // 2, side - tree.height), tree)
    if top:  # Kuto: a margin all round, so the model doesn't run it off the edges
        big = Image.new("RGBA", (side * 5 // 4, side * 5 // 4), (255, 0, 255, 255))
        big.paste(ref, (side // 8, side // 8))
        ref = big
    b = io.BytesIO()
    ref.resize((1024, 1024), Image.NEAREST).convert("RGB").save(b, "PNG")
    return b.getvalue()


def finish(name):
    im = rw.key_out((RAW / f"{name}_openai.png").read_bytes(), "magenta")
    if name == "rim":  # the well's rim, laid around the hole by the engine: the row, cut at two mortar joints
        a = np.asarray(im)
        rows = np.nonzero((a[..., 3] > 0).mean(axis=1) > 0.9)[0]
        a = a[rows.min():rows.max() + 1]
        lum = a[..., :3].mean(axis=(0, 2))
        w = a.shape[1]
        x0 = int(np.argmin(lum[: w // 4]))
        x1 = w * 3 // 4 + int(np.argmin(lum[w * 3 // 4:]))
        Image.fromarray(a[:, x0:x1]).save(OUT / f"{PROPS[name][0]}.png")
        return
    a = np.asarray(im)
    ys, xs = np.nonzero(a[..., 3] > 30)  # key-out leaves a faint fringe that would pad the sprite
    im = im.crop((xs.min(), ys.min(), xs.max() + 1, ys.max() + 1))  # trunk on the bottom edge
    k = 512 / max(im.size)
    im.resize((round(im.width * k), round(im.height * k)), Image.LANCZOS).save(OUT / f"{PROPS[name][0]}.png")


if __name__ == "__main__":
    args = sys.argv[1:]
    names = [a for a in args if a in PROPS] or list(PROPS)
    for n in names:
        if "finish" not in args:
            RAW.mkdir(parents=True, exist_ok=True)
            ref = reference(*PROPS[n][1:4])
            (RAW / f"{n}_reference.png").write_bytes(ref)
            (RAW / f"{n}_openai.png").write_bytes(rw.openai(PROPS[n][4], ref))
        finish(n)
