#!/usr/bin/env python3
"""Remaster pipeline: free-standing props drawn as billboards instead of wall faces (see make_map.py "props").
  python3 make_props.py [NAME...] [finish]   (NAME: tree, well; default all)
Nomad Camp trees: PoR paints them onto walls (a tree at a face's left, or halves at its edges that join across
faces); the PoC stands one tree sprite wherever a tree's trunk is. The reference is the original's one whole tree
(WALLDEF7-007-2-wall1, nomad wall 6), cropped and centred; writes blobber/assets/walls/prop_nomad_tree.png.
Kuto's Well: the well hole and its ladder, which PoR paints on every wall of the well room, stand once in the
middle of the room (areas.py "cellprops"); the reference is the lower part of kuto wall 8."""
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
WELL = ("Game sprite on a flat solid magenta (#FF00FF) background: an old village well seen from the side at eye "
        "level, slightly from above: a low round rim of grey fieldstones around a black hole, and a rusty iron ladder "
        "standing upright out of the hole, its rails and rungs going down into the darkness. The well is centred, its "
        "rim touching the lower edge of the image; nothing else, no ground, no walls, no shadow, the magenta "
        "background is perfectly flat and fills everything around it. Painterly high-fantasy game art with modern "
        "detail, faithful to the reference's shape and colours.")
# name -> (texture, area, wall value, share of the dump's rows the reference starts at, prompt)
PROPS = {"tree": ("prop_nomad_tree", "nomad", 6, 0.0, TREE), "well": ("prop_kuto_well", "kuto", 8, 0.5, WELL)}


def reference(area, v, top):
    a = np.asarray(Image.open(HERE / "viewdump" / f"{src(area, v)}.png").convert("RGBA")).copy()
    a[:int(a.shape[0] * top), :, 3] = 0
    ys, xs = np.nonzero(a[..., 3])
    tree = Image.fromarray(a[ys.min():ys.max() + 1, xs.min():xs.max() + 1])
    side = max(tree.size)
    ref = Image.new("RGBA", (side, side), (255, 0, 255, 255))
    ref.paste(tree, ((side - tree.width) // 2, side - tree.height), tree)
    b = io.BytesIO()
    ref.resize((1024, 1024), Image.NEAREST).convert("RGB").save(b, "PNG")
    return b.getvalue()


def finish(name):
    im = rw.key_out((RAW / f"{name}_openai.png").read_bytes(), "magenta")
    a = np.asarray(im)
    ys, xs = np.nonzero(a[..., 3] > 0)
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
