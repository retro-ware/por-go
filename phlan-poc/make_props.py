#!/usr/bin/env python3
"""Remaster pipeline: free-standing props drawn as billboards instead of wall faces (see make_map.py "props").
  python3 make_props.py [NAME...] [finish]   (NAME: tree, well; default all)
Nomad Camp trees: PoR paints them onto walls (a tree at a face's left, or halves at its edges that join across
faces); the PoC stands one tree sprite wherever a tree's trunk is. The reference is the original's one whole tree
(WALLDEF7-007-2-wall1, nomad wall 6), cropped and centred; writes blobber/assets/walls/prop_nomad_tree.png.
Kuto's Well: the well hole and its ladder, which PoR paints on every wall of the well room, stand once in the
room (areas.py "cellprops"): the hole as a flat oval on the floor, the ladder upright at its far rim; references
are parts of kuto wall 8."""
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
HOLE = ("Game texture on a flat solid magenta (#FF00FF) background, seen straight down from directly above: the "
        "opening of an old village well shaft in the ground, a very flat wide oval, about two and a half times as wide as it is tall, exactly the oval of the reference, a flat "
        "ring of grey fieldstones set level with the ground around a pitch-black bottomless hole, and the top rungs "
        "and rails of a rusty iron ladder at the top (far) side of the hole going down into the darkness. Strictly "
        "top-down, no perspective, no walls, no shadow outside the ring; the oval fills the image width with a small "
        "magenta margin all round. Painterly high-fantasy game art with modern detail, colours like the reference.")
LADDER = ("Game sprite on a flat solid magenta (#FF00FF) background: one straight rusty iron ladder standing "
          "perfectly upright, seen from the front, two round rails and seven evenly spaced rungs, the bottom ends of "
          "the rails at the lower edge. Nothing else, no ground, no wall, no shadow, a small magenta margin at the "
          "top and the sides. Painterly high-fantasy game art with modern detail, colours like the reference.")
# name -> (texture, area, wall value, share of the dump's rows the reference starts at, prompt)
PROPS = {"tree": ("prop_nomad_tree", "nomad", 6, 0.0, TREE), "hole": ("floor_kuto_well", "kuto", 8, 0.9, HOLE),
         "ladder": ("prop_kuto_ladder", "kuto", 8, 0.5, LADDER)}


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


def oval():
    """The hole's reference, drawn: the 2:1 oval it must have (the model draws a round one from the original's art)."""
    from PIL import ImageDraw
    ref = Image.new("RGB", (1536, 1024), (255, 0, 255))
    d = ImageDraw.Draw(ref)
    d.ellipse((88, 332, 1448, 692), fill=(140, 140, 140))  # 2.4:1, what fits the floor in front of the wall (the model still draws ~1.6:1)
    d.ellipse((168, 372, 1368, 652), fill=(0, 0, 0))
    for x in (700, 836):  # the ladder's rails at the far side
        d.line((x, 366, x, 470), fill=(120, 70, 40), width=14)
    for y in (392, 430, 466):
        d.line((700, y, 836, y), fill=(120, 70, 40), width=10)
    b = io.BytesIO()
    ref.save(b, "PNG")
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
            ref = oval() if n == "hole" else reference(*PROPS[n][1:4])
            (RAW / f"{n}_reference.png").write_bytes(ref)
            (RAW / f"{n}_openai.png").write_bytes(rw.openai(PROPS[n][4], ref, "1536x1024" if n == "hole" else "1024x1024"))
        finish(n)
