#!/usr/bin/env python3
"""Remaster pipeline: free-standing props drawn as billboards instead of wall faces (see make_map.py "props").
  python3 make_props.py [finish]
Nomad Camp trees: PoR paints them onto walls (a tree at a face's left, or halves at its edges that join across
faces); the PoC stands one tree sprite wherever a tree's trunk is. The reference is the original's one whole tree
(WALLDEF7-007-2-wall1, nomad wall 6), cropped and centred; writes blobber/assets/walls/prop_nomad_tree.png."""
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
OUT = HERE / "blobber/assets/walls/prop_nomad_tree.png"
PROMPT = ("Game sprite on a flat solid magenta (#FF00FF) background: one single tall leafy deciduous tree standing "
          "upright, seen from the side at eye level, a full rounded crown of dense green leaves with a few small red "
          "berries, a gnarled dark brown trunk with roots at the bottom touching the lower edge of the image. The tree "
          "is centred and fills the image height; nothing else, no ground, no shadow, no grass, the magenta "
          "background is perfectly flat and fills everything around the tree. Painterly high-fantasy game art with "
          "modern detail, faithful to the reference's shape and colours.")


def reference():
    a = np.asarray(Image.open(HERE / "viewdump" / f"{src('nomad', 6)}.png").convert("RGBA"))
    ys, xs = np.nonzero(a[..., 3])
    tree = Image.fromarray(a[ys.min():ys.max() + 1, xs.min():xs.max() + 1])
    side = max(tree.size)
    ref = Image.new("RGBA", (side, side), (255, 0, 255, 255))
    ref.paste(tree, ((side - tree.width) // 2, side - tree.height), tree)
    b = io.BytesIO()
    ref.resize((1024, 1024), Image.NEAREST).convert("RGB").save(b, "PNG")
    return b.getvalue()


def finish():
    im = rw.key_out((RAW / "tree_openai.png").read_bytes(), "magenta")
    a = np.asarray(im)
    ys, xs = np.nonzero(a[..., 3] > 0)
    im = im.crop((xs.min(), ys.min(), xs.max() + 1, ys.max() + 1))  # trunk on the bottom edge
    k = 512 / max(im.size)
    im.resize((round(im.width * k), round(im.height * k)), Image.LANCZOS).save(OUT)


if __name__ == "__main__":
    if sys.argv[1:] != ["finish"]:
        RAW.mkdir(parents=True, exist_ok=True)
        ref = reference()
        (RAW / "tree_reference.png").write_bytes(ref)
        (RAW / "tree_openai.png").write_bytes(rw.openai(PROMPT, ref))
    finish()
