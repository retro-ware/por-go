#!/usr/bin/env python3
"""Sky and ceiling flats for the BlobberRPG copy in blobber/:
  python3 make_sky.py          night sky gradient + star layer (procedural, no API)
  python3 make_sky.py ceilings indoor ceilings (OpenAI edit of the default ceiling; raw output in raw/ceilings/)
The night pair stands in for ceiling_sky_phlan / ceiling_phlan_clouds when the engine's night is on (N)."""
import importlib.util, io, sys
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
import numpy as np
from PIL import Image

HERE = Path(__file__).resolve().parent
OUT = HERE / "blobber/assets/walls"
RAW = HERE / "raw/ceilings"

CEILINGS = {  # name -> what it is
    "cave": "a rough natural cave ceiling of dark grey-brown rock seen from below: uneven bulges, cracks and a few "
            "small stalactite stubs, damp and dimly lit",
    "dungeon": "a vaulted dungeon ceiling of dark grey cut stone blocks in even courses with dark mortar, seen from "
               "below, old and slightly soot-stained",
    "wood": "an interior ceiling of dark brown wooden planks crossed by heavy square wooden beams, seen from below, "
            "old and worn",
}


def night():
    h = 256
    t = np.linspace(0, 1, h)[:, None]  # top .. horizon
    top, low = np.array([4, 6, 22]), np.array([28, 40, 78])
    sky = (top + (low - top) * t ** 1.6)[:, None, :].repeat(4, axis=1)
    Image.fromarray(sky.astype(np.uint8), "RGB").save(OUT / "ceiling_sky_night.png")
    r = np.random.default_rng(7)
    a = np.zeros((512, 512, 4), np.uint8)
    for _ in range(420):  # stars, a few brighter and larger; wrap at the tile edge so the layer tiles
        x, y, b = r.integers(512), r.integers(512), r.uniform(0.35, 1) ** 2
        s = 2 if b > 0.75 else 1
        c = (np.array([255, 250, 235]) if r.random() < 0.7 else np.array([200, 215, 255])).astype(np.uint8)
        for dy in range(s):
            for dx in range(s):
                a[(y + dy) % 512, (x + dx) % 512] = [*c, int(255 * b)]
    Image.fromarray(a, "RGBA").save(OUT / "ceiling_night_stars.png")


def tile(im):
    """Crossfade the outer bands in both axes so the texture repeats without seams."""
    a = np.asarray(im.convert("RGB"), np.float32)
    for axis in (1, 0):
        n = a.shape[axis]
        b = n // 8
        t = np.linspace(0, 1, b).reshape((1, b, 1) if axis == 1 else (b, 1, 1))
        lo = np.take(a, range(b), axis=axis)
        hi = np.take(a, range(n - b, n), axis=axis)
        a = np.concatenate([np.take(a, range(b, n - b), axis=axis), hi * (1 - t) + lo * t], axis=axis)
    return Image.fromarray(a.astype(np.uint8), "RGB").resize((256, 256), Image.LANCZOS)  # square in, square out


def ceiling(name, desc):
    spec = importlib.util.spec_from_file_location("rw", HERE.parent / "tools/remaster-walls.py")
    rw = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(rw)
    buf = io.BytesIO()
    Image.open(OUT / "ceiling.png").convert("RGB").resize((1024, 1024), Image.NEAREST).save(buf, "PNG")
    prompt = (f"A seamless square texture for a fantasy dungeon crawler game: {desc}. Viewed straight on, flat, "
              "evenly lit, no perspective, no vignette, no text, no frame. It must tile seamlessly in both directions.")
    png = rw.openai(prompt, buf.getvalue())
    (RAW / f"{name}_openai.png").write_bytes(png)
    tile(Image.open(io.BytesIO(png))).save(OUT / f"ceiling_{name}.png")
    return f"ceiling_{name}: ok"


if __name__ == "__main__":
    if sys.argv[1:] == ["ceilings"]:
        RAW.mkdir(parents=True, exist_ok=True)
        with ThreadPoolExecutor(3) as ex:
            for r in ex.map(lambda kv: ceiling(*kv), CEILINGS.items()):
                print(r)
    else:
        night()
